package workflow

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"conan-cli/internal/conan"
	"conan-cli/internal/config"
	"conan-cli/internal/nexus"
)

const catalogListWorkers = 6

type CatalogFilter struct {
	Query     string
	OS        string
	Arch      string
	Compiler  string
	Qt        string
	BuildType string
	NoQt      bool
}

// CatalogPackage / CatalogPackageBinary 是 Report.Data 暴露的仓库目录类型。
// 界面层消费 Report 时只依赖 workflow，不直接 import internal/conan。
type (
	CatalogPackage       = conan.Package
	CatalogPackageBinary = conan.PackageBinary
)

func (a *App) Catalog(ctx context.Context, query string) (Report, error) {
	return a.CatalogFilter(ctx, CatalogFilter{Query: query})
}

func (a *App) CatalogFilter(ctx context.Context, filter CatalogFilter) (Report, error) {
	query := strings.TrimSpace(filter.Query)
	filter.Query = query
	filter.OS = config.NormalizeOS(filter.OS)
	filter.Arch = config.NormalizeArch(filter.Arch)
	filter.BuildType = config.NormalizeBuildType(filter.BuildType)
	global, _ := config.LoadGlobal()
	if global == nil {
		global = &config.Global{}
	}
	var project *config.Project
	if loaded, err := a.Project(); err == nil {
		project = loaded
	}
	remote := resolveRemote("", project)

	packages, refs, source, err := a.loadCatalog(ctx, global, remote, query)
	if err != nil {
		return Report{OK: false, Action: "catalog", Error: err.Error(), Message: "查询仓库失败"}, err
	}
	packages = conan.FilterPackages(packages, query)
	packages = attachBinaryRefs(packages, refs)
	packages = a.enrichCatalogBinaries(ctx, packages, remote)
	packages = filterCatalogPackages(packages, filter)

	var lines []string
	binaryCount := 0
	for _, pkg := range packages {
		if len(pkg.Binaries) == 0 {
			lines = append(lines, pkg.Name+"  "+strings.Join(pkg.Versions, ", "))
			continue
		}
		for _, bin := range pkg.Binaries {
			binaryCount++
			lines = append(lines, formatCatalogBinary(pkg.Name, bin))
		}
	}
	message := "仓库里没有组件"
	if len(packages) > 0 {
		if binaryCount > 0 {
			message = fmt.Sprintf("找到 %d 个组件，%d 条制品", len(packages), binaryCount)
		} else {
			message = fmt.Sprintf("找到 %d 个组件", len(packages))
		}
	}
	return Report{OK: true, Action: "catalog", Message: message, Output: strings.Join(lines, "\n"), Data: map[string]any{
		"remote":       remote,
		"query":        query,
		"source":       source,
		"os":           filter.OS,
		"arch":         filter.Arch,
		"compiler":     strings.TrimSpace(filter.Compiler),
		"qt":           strings.TrimSpace(filter.Qt),
		"build_type":   filter.BuildType,
		"no_qt":        filter.NoQt,
		"packages":     packages,
		"binary_count": binaryCount,
	}}, nil
}

func (a *App) loadCatalog(ctx context.Context, global *config.Global, remote, query string) ([]conan.Package, []conan.BinaryRef, string, error) {
	if global.Nexus.URL != "" {
		password, _ := config.LoadPassword()
		packages, refs, err := nexus.ListPackagesDetailed(ctx, global.Nexus.URL, global.Nexus.Username, password, "")
		if err == nil {
			return packages, refs, "nexus", nil
		}
		if query == "" || query == "*" {
			return nil, nil, "", fmt.Errorf("无法列出全部组件（%v）。请输入包名再查，例如 qtutils", err)
		}
	}

	pattern := query
	if pattern == "" || pattern == "*" {
		return nil, nil, "", fmt.Errorf("该仓库不支持列出全部组件，请输入包名查询")
	}
	if !strings.ContainsAny(pattern, "/*") {
		pattern = query + "*"
	}
	data, _, err := a.Client.List(ctx, pattern, remote)
	packages := conan.GroupPackages(conan.ParseRecipes(data))
	refs := conan.ExtractBinaryRefs(data)
	if len(packages) > 0 {
		return packages, refs, "conan-list", nil
	}
	if msg := conan.RemoteListError(data); msg != "" {
		return nil, nil, "", fmt.Errorf("%s", msg)
	}
	if err != nil {
		return nil, nil, "", err
	}
	return packages, refs, "conan-list", nil
}

func attachBinaryRefs(packages []conan.Package, refs []conan.BinaryRef) []conan.Package {
	if len(refs) == 0 {
		return packages
	}
	grouped := map[string][]conan.PackageBinary{}
	for _, ref := range refs {
		grouped[ref.Name] = append(grouped[ref.Name], toPackageBinary(ref))
	}
	for i := range packages {
		if len(packages[i].Binaries) > 0 {
			continue
		}
		packages[i].Binaries = dedupPackageBinaries(grouped[packages[i].Name])
	}
	return packages
}

func (a *App) enrichCatalogBinaries(ctx context.Context, packages []conan.Package, remote string) []conan.Package {
	if len(packages) == 0 || a.Client == nil {
		return packages
	}
	type result struct {
		index    int
		binaries []conan.PackageBinary
	}
	workers := catalogListWorkers
	if len(packages) < workers {
		workers = len(packages)
	}
	jobs := make(chan int)
	out := make(chan result, len(packages))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				select {
				case <-ctx.Done():
					out <- result{index: index}
					continue
				default:
				}
				name := packages[index].Name
				if len(packages[index].Binaries) > 0 {
					out <- result{index: index, binaries: packages[index].Binaries}
					continue
				}
				binaries := a.listPackageBinaries(ctx, name, packages[index].Versions, remote)
				out <- result{index: index, binaries: binaries}
			}
		}()
	}
	go func() {
		for i := range packages {
			select {
			case <-ctx.Done():
			case jobs <- i:
			}
		}
		close(jobs)
	}()
	go func() {
		wg.Wait()
		close(out)
	}()
	for item := range out {
		if item.index < 0 || item.index >= len(packages) {
			continue
		}
		packages[item.index].Binaries = item.binaries
	}
	return packages
}

func (a *App) listPackageBinaries(ctx context.Context, name string, versions []string, remote string) []conan.PackageBinary {
	patterns := make([]string, 0, len(versions)+2)
	for _, version := range versions {
		version = strings.TrimSpace(version)
		if version != "" {
			patterns = append(patterns, name+"/"+version+":*")
		}
	}
	patterns = append(patterns, name+"/*:*", name+"/*#*:*")
	seen := map[string]bool{}
	var binaries []conan.PackageBinary
	for _, pattern := range patterns {
		data, _, err := a.Client.List(ctx, pattern, remote)
		if err != nil || data == nil {
			continue
		}
		for _, item := range packageBinariesFromList(data, name) {
			key := binaryKey(item)
			if seen[key] {
				continue
			}
			seen[key] = true
			binaries = append(binaries, item)
		}
		if len(binaries) > 0 {
			break
		}
	}
	return dedupPackageBinaries(binaries)
}

func packageBinariesFromList(data map[string]any, name string) []conan.PackageBinary {
	var binaries []conan.PackageBinary
	for _, ref := range conan.ExtractBinaryRefs(data) {
		if name != "" && ref.Name != name {
			continue
		}
		binaries = append(binaries, toPackageBinary(ref))
	}
	return dedupPackageBinaries(binaries)
}

func binaryKey(item conan.PackageBinary) string {
	return strings.Join([]string{item.Version, item.OS, item.Arch, item.Compiler, item.CompilerVersion, item.BuildType, item.QtVersion, fmt.Sprintf("%t", item.NoQt), item.Channel}, "|")
}

func dedupPackageBinaries(binaries []conan.PackageBinary) []conan.PackageBinary {
	seen := map[string]bool{}
	var out []conan.PackageBinary
	for _, item := range binaries {
		key := binaryKey(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch {
		case a.Version != b.Version:
			return a.Version < b.Version
		case a.OS != b.OS:
			return a.OS < b.OS
		case a.Arch != b.Arch:
			return a.Arch < b.Arch
		case a.Compiler != b.Compiler:
			return a.Compiler < b.Compiler
		case a.CompilerVersion != b.CompilerVersion:
			return a.CompilerVersion < b.CompilerVersion
		case a.BuildType != b.BuildType:
			return a.BuildType < b.BuildType
		default:
			return a.QtVersion < b.QtVersion
		}
	})
	return out
}

func toPackageBinary(ref conan.BinaryRef) conan.PackageBinary {
	qt, noQt := catalogQt(ref.Options)
	return conan.PackageBinary{
		Version:         ref.Version,
		OS:              catalogOS(ref.Settings, ref.Options),
		Arch:            config.NormalizeArch(ref.Settings["arch"]),
		Compiler:        catalogCompiler(ref.Settings["compiler"]),
		CompilerVersion: strings.TrimSpace(ref.Settings["compiler.version"]),
		BuildType:       config.NormalizeBuildType(ref.Settings["build_type"]),
		QtVersion:       qt,
		NoQt:            noQt,
		Channel:         ref.Channel,
		Reference:       ref.Reference,
	}
}

func catalogOS(settings, options map[string]string) string {
	if blobContainsKylin(settings, options) {
		return config.OSKylin
	}
	return config.NormalizeOS(settings["os"])
}

func catalogCompiler(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "visual studio", "msvc":
		return "msvc"
	default:
		return strings.TrimSpace(raw)
	}
}

func catalogQt(options map[string]string) (string, bool) {
	for _, key := range []string{"qt_version", "*:qt_version"} {
		value := strings.TrimSpace(options[key])
		if value == "" || strings.EqualFold(value, "none") || strings.EqualFold(value, "null") {
			continue
		}
		return value, false
	}
	return "", true
}

func blobContainsKylin(settings, options map[string]string) bool {
	for _, value := range settings {
		lower := strings.ToLower(value)
		if strings.Contains(lower, "kylin") || strings.Contains(lower, "麒麟") || strings.Contains(lower, "neokylin") {
			return true
		}
	}
	for _, value := range options {
		lower := strings.ToLower(value)
		if strings.Contains(lower, "kylin") || strings.Contains(lower, "麒麟") || strings.Contains(lower, "neokylin") {
			return true
		}
	}
	return false
}

func filterCatalogPackages(packages []conan.Package, filter CatalogFilter) []conan.Package {
	if !hasBinaryFilter(filter) {
		return packages
	}
	var out []conan.Package
	for _, pkg := range packages {
		var matched []conan.PackageBinary
		for _, bin := range pkg.Binaries {
			if matchCatalogBinary(bin, filter) {
				matched = append(matched, bin)
			}
		}
		if len(matched) == 0 {
			continue
		}
		pkg.Binaries = matched
		versions := make([]string, 0, len(matched))
		seen := map[string]bool{}
		for _, bin := range matched {
			if bin.Version == "" || seen[bin.Version] {
				continue
			}
			seen[bin.Version] = true
			versions = append(versions, bin.Version)
		}
		if len(versions) > 0 {
			pkg.Versions = versions
		}
		out = append(out, pkg)
	}
	return out
}

func hasBinaryFilter(filter CatalogFilter) bool {
	return filter.OS != "" || filter.Arch != "" || strings.TrimSpace(filter.Compiler) != "" ||
		strings.TrimSpace(filter.Qt) != "" || filter.BuildType != "" || filter.NoQt
}

func matchCatalogBinary(bin conan.PackageBinary, filter CatalogFilter) bool {
	if filter.OS != "" && bin.OS != filter.OS {
		return false
	}
	if filter.Arch != "" && bin.Arch != filter.Arch {
		return false
	}
	if filter.BuildType != "" && bin.BuildType != filter.BuildType {
		return false
	}
	if filter.NoQt && !bin.NoQt {
		return false
	}
	if qt := strings.TrimSpace(filter.Qt); qt != "" && !filter.NoQt && bin.QtVersion != qt {
		return false
	}
	if compiler := strings.TrimSpace(filter.Compiler); compiler != "" {
		label := strings.TrimSpace(bin.Compiler)
		if bin.CompilerVersion != "" {
			label = strings.TrimSpace(bin.Compiler + " " + bin.CompilerVersion)
		}
		if !strings.EqualFold(bin.Compiler, compiler) && !strings.EqualFold(label, compiler) {
			return false
		}
	}
	return true
}

func formatCatalogBinary(name string, bin conan.PackageBinary) string {
	ref := bin.Reference
	if ref == "" {
		ref = name + "/" + bin.Version
	}
	platform := strings.TrimSpace(bin.OS + "/" + bin.Arch)
	if platform == "/" {
		platform = "-"
	}
	compiler := strings.TrimSpace(bin.Compiler + " " + bin.CompilerVersion)
	if compiler == "" {
		compiler = "-"
	}
	qt := "无 Qt"
	if !bin.NoQt && bin.QtVersion != "" {
		qt = "Qt " + bin.QtVersion
	}
	buildType := bin.BuildType
	if buildType == "" {
		buildType = "-"
	}
	return ref + "  " + platform + "  " + compiler + "  " + qt + "  " + buildType
}
