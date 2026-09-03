package workflow

import (
	"context"
	"fmt"
	"strings"

	"conan-cli/internal/conan"
	"conan-cli/internal/config"
	"conan-cli/internal/nexus"
)

func (a *App) Catalog(ctx context.Context, query string) (Report, error) {
	query = strings.TrimSpace(query)
	global, _ := config.LoadGlobal()
	if global == nil {
		global = &config.Global{}
	}
	var project *config.Project
	if loaded, err := a.Project(); err == nil {
		project = loaded
	}
	remote := resolveRemote("", project)

	packages, source, err := a.loadCatalog(ctx, global, remote, query)
	if err != nil {
		return Report{OK: false, Action: "catalog", Error: err.Error(), Message: "查询仓库失败"}, err
	}
	packages = conan.FilterPackages(packages, query)

	var lines []string
	for _, pkg := range packages {
		lines = append(lines, pkg.Name+"  "+strings.Join(pkg.Versions, ", "))
	}
	message := "仓库里没有组件"
	if len(packages) > 0 {
		message = fmt.Sprintf("找到 %d 个组件", len(packages))
	}
	return Report{OK: true, Action: "catalog", Message: message, Output: strings.Join(lines, "\n"), Data: map[string]any{
		"remote":   remote,
		"query":    query,
		"source":   source,
		"packages": packages,
	}}, nil
}

func (a *App) loadCatalog(ctx context.Context, global *config.Global, remote, query string) ([]conan.Package, string, error) {
	if global.Nexus.URL != "" {
		password, _ := config.LoadPassword()
		packages, err := nexus.ListPackages(ctx, global.Nexus.URL, global.Nexus.Username, password, "")
		if err == nil {
			return packages, "nexus", nil
		}
		if query == "" || query == "*" {
			return nil, "", fmt.Errorf("无法列出全部组件（%v）。请输入包名再查，例如 qtutils", err)
		}
	}

	pattern := query
	if pattern == "" || pattern == "*" {
		return nil, "", fmt.Errorf("该仓库不支持列出全部组件，请输入包名查询")
	}
	if !strings.ContainsAny(pattern, "/*") {
		pattern = query + "*"
	}
	data, _, err := a.Client.List(ctx, pattern, remote)
	packages := conan.GroupPackages(conan.ParseRecipes(data))
	if len(packages) > 0 {
		return packages, "conan-list", nil
	}
	if msg := conan.RemoteListError(data); msg != "" {
		return nil, "", fmt.Errorf("%s", msg)
	}
	if err != nil {
		return nil, "", err
	}
	return packages, "conan-list", nil
}
