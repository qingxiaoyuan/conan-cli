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
	"time"

	"conan-cli/internal/conan"
)

type componentPage struct {
	Items             []componentItem `json:"items"`
	ContinuationToken string          `json:"continuationToken"`
}

type componentItem struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Format  string `json:"format"`
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
	baseURL, repo, ok := ParseRepositoryURL(repositoryURL)
	if !ok {
		return nil, fmt.Errorf("not a Nexus repository URL")
	}
	client := &http.Client{Timeout: 20 * time.Second}
	seen := map[string]map[string]bool{}
	token := ""
	for page := 0; page < 50; page++ {
		endpoint := baseURL + "/service/rest/v1/components?repository=" + url.QueryEscape(repo)
		if token != "" {
			endpoint += "&continuationToken=" + url.QueryEscape(token)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		if username != "" {
			req.SetBasicAuth(username, password)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("Nexus %s: %s", resp.Status, strings.TrimSpace(string(body)))
		}
		var pageData componentPage
		if err := json.Unmarshal(body, &pageData); err != nil {
			return nil, fmt.Errorf("parse Nexus catalog: %w", err)
		}
		for _, item := range pageData.Items {
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
		if pageData.ContinuationToken == "" {
			break
		}
		token = pageData.ContinuationToken
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
	return conan.FilterPackages(packages, query), nil
}
