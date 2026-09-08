package nexus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"conan-cli/internal/conan"
)

const (
	catalogPageLimit = 50
	conanInfoWorkers = 8
	conanInfoTimeout = 20 * time.Second
)

type componentPage struct {
	Items             []componentItem `json:"items"`
	ContinuationToken string          `json:"continuationToken"`
}

type componentItem struct {
	ID      string           `json:"id"`
	Name    string           `json:"name"`
	Version string           `json:"version"`
	Format  string           `json:"format"`
	Group   string           `json:"group"`
	Assets  []componentAsset `json:"assets"`
}

type componentAsset struct {
	DownloadURL string `json:"downloadUrl"`
	Path        string `json:"path"`
}

func ParseRepositoryURL(raw string) (baseURL, repo string, ok bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "repository" || parts[1] == "" {
		return "", "", false
	}
	return parsed.Scheme + "://" + parsed.Host, parts[1], true
}

func CleanConanVersion(raw string) string {
	value := strings.TrimSpace(raw)
	if i := strings.Index(value, "#"); i >= 0 {
		value = value[:i]
	}
	value = strings.TrimSuffix(value, "-_")
	if i := strings.LastIndex(value, "-"); i > 0 {
		suffix := value[i+1:]
		if suffix == "_" || strings.Contains(suffix, "_") {
			value = value[:i]
		}
	}
	return value
}

func ListPackages(ctx context.Context, repositoryURL, username, password, query string) ([]conan.Package, error) {
	packages, _, err := ListPackagesDetailed(ctx, repositoryURL, username, password, query)
	return packages, err
}

func ListPackagesDetailed(ctx context.Context, repositoryURL, username, password, query string) ([]conan.Package, []conan.BinaryRef, error) {
	baseURL, repo, ok := ParseRepositoryURL(repositoryURL)
	if !ok {
		return nil, nil, fmt.Errorf("not a Nexus repository URL")
	}
	client := &http.Client{Timeout: conanInfoTimeout}
	items, err := listComponentItems(ctx, client, baseURL, repo, username, password)
	if err != nil {
		return nil, nil, err
	}
	packages := groupComponents(items, query)
	if !hasConanInfoAsset(items) {
		items = hydrateComponentAssets(ctx, client, baseURL, username, password, items)
	}
	refs := fetchConanInfoRefs(ctx, client, username, password, items)
	return packages, refs, nil
}

func hasConanInfoAsset(items []componentItem) bool {
	for _, item := range items {
		for _, asset := range item.Assets {
			if isConanInfoAsset(asset) {
				return true
			}
		}
	}
	return false
}

func hydrateComponentAssets(ctx context.Context, client *http.Client, baseURL, username, password string, items []componentItem) []componentItem {
	for i, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		body, status, err := doGET(ctx, client, baseURL+"/service/rest/v1/components/"+url.PathEscape(id), username, password)
		if err != nil || status >= 400 {
			continue
		}
		var detailed componentItem
		if json.Unmarshal(body, &detailed) != nil {
			continue
		}
		if len(detailed.Assets) > 0 {
			items[i].Assets = detailed.Assets
		}
		if items[i].Group == "" {
			items[i].Group = detailed.Group
		}
	}
	return items
}

func listComponentItems(ctx context.Context, client *http.Client, baseURL, repo, username, password string) ([]componentItem, error) {
	var items []componentItem
	token := ""
	for page := 0; page < catalogPageLimit; page++ {
		endpoint := baseURL + "/service/rest/v1/components?repository=" + url.QueryEscape(repo)
		if token != "" {
			endpoint += "&continuationToken=" + url.QueryEscape(token)
		}
		body, status, err := doGET(ctx, client, endpoint, username, password)
		if err != nil {
			return nil, err
		}
		if status >= 400 {
			return nil, fmt.Errorf("Nexus %s: %s", http.StatusText(status), strings.TrimSpace(string(body)))
		}
		var pageData componentPage
		if err := json.Unmarshal(body, &pageData); err != nil {
			return nil, fmt.Errorf("parse Nexus catalog: %w", err)
		}
		items = append(items, pageData.Items...)
		if pageData.ContinuationToken == "" {
			break
		}
		token = pageData.ContinuationToken
	}
	return items, nil
}

func groupComponents(items []componentItem, query string) []conan.Package {
	seen := map[string]map[string]bool{}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		version := CleanConanVersion(item.Version)
		if name == "" || version == "" {
			continue
		}
		if seen[name] == nil {
			seen[name] = map[string]bool{}
		}
		seen[name][version] = true
	}
	var packages []conan.Package
	for name, versions := range seen {
		pkg := conan.Package{Name: name}
		for version := range versions {
			pkg.Versions = append(pkg.Versions, version)
		}
		sort.Strings(pkg.Versions)
		packages = append(packages, pkg)
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].Name < packages[j].Name })
	return conan.FilterPackages(packages, query)
}

func fetchConanInfoRefs(ctx context.Context, client *http.Client, username, password string, items []componentItem) []conan.BinaryRef {
	type job struct {
		name    string
		version string
		channel string
		infoURL string
	}
	var jobs []job
	seenURL := map[string]bool{}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		version := CleanConanVersion(item.Version)
		if name == "" || version == "" {
			continue
		}
		channel := recipeChannel(item.Group)
		for _, asset := range item.Assets {
			if !isConanInfoAsset(asset) {
				continue
			}
			infoURL := strings.TrimSpace(asset.DownloadURL)
			if infoURL == "" || seenURL[infoURL] {
				continue
			}
			seenURL[infoURL] = true
			jobs = append(jobs, job{name: name, version: version, channel: channel, infoURL: infoURL})
		}
	}
	if len(jobs) == 0 {
		return nil
	}
	out := make(chan conan.BinaryRef, len(jobs))
	work := make(chan job)
	workers := conanInfoWorkers
	if len(jobs) < workers {
		workers = len(jobs)
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				select {
				case <-ctx.Done():
					return
				default:
				}
				body, status, err := doGET(ctx, client, item.infoURL, username, password)
				if err != nil || status >= 400 {
					continue
				}
				settings, options := conan.ParseConanInfo(string(body))
				if len(settings) == 0 && len(options) == 0 {
					continue
				}
				out <- conan.BinaryRef{
					Name:      item.name,
					Version:   item.version,
					Channel:   item.channel,
					Reference: item.name + "/" + item.version,
					Settings:  settings,
					Options:   options,
				}
			}
		}()
	}
	go func() {
		for _, item := range jobs {
			select {
			case <-ctx.Done():
			case work <- item:
			}
		}
		close(work)
	}()
	go func() {
		wg.Wait()
		close(out)
	}()
	var refs []conan.BinaryRef
	for ref := range out {
		refs = append(refs, ref)
	}
	return refs
}

func isConanInfoAsset(asset componentAsset) bool {
	path := strings.ToLower(strings.TrimSpace(asset.Path) + " " + strings.TrimSpace(asset.DownloadURL))
	return strings.Contains(path, "conaninfo.txt")
}

func recipeChannel(group string) string {
	group = strings.TrimSpace(group)
	if group == "" || group == "_" || group == "_/_" {
		return ""
	}
	parts := strings.Split(group, "/")
	if len(parts) >= 2 {
		channel := strings.TrimSpace(parts[len(parts)-1])
		if channel == "_" {
			return ""
		}
		return channel
	}
	if group == "_" {
		return ""
	}
	return group
}

func doGET(ctx context.Context, client *http.Client, endpoint, username, password string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	if username != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, resp.StatusCode, readErr
	}
	return body, resp.StatusCode, nil
}
