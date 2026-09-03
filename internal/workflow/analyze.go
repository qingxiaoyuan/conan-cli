package workflow

import (
	"context"
	"errors"
	"strings"

	"conan-cli/internal/conan"
	"conan-cli/internal/config"
	"conan-cli/internal/manifest"
	"conan-cli/internal/platform"
)

type DependencyRow struct {
	Reference string            `json:"reference"`
	InProject bool              `json:"in_project"`
	InRecipe  bool              `json:"in_recipe"`
	Aligned   bool              `json:"aligned"`
	Status    string            `json:"status"`
	Detail    string            `json:"detail"`
	Platform  string            `json:"platform"`
	Settings  map[string]string `json:"settings,omitempty"`
}

func (a *App) Analyze(ctx context.Context, osName, arch, buildType string) (Report, error) {
	project, err := a.Project()
	if err != nil {
		return Report{OK: true, Action: "analyze", Message: "尚未初始化", Data: map[string]any{"dependencies": []DependencyRow{}}}, nil
	}
	spec := project.Platform.Consume
	if osName != "" {
		spec.OS = osName
	}
	if arch != "" {
		spec.Arch = arch
	}
	if buildType != "" {
		spec.BuildType = buildType
	}
	targetMissing := missingTarget(spec)
	settings := platform.Resolve(spec, project.Compiler, project.QtVersion)
	remote := resolveRemote("", project)

	recipeDeps := []string{}
	recipeErr := ""
	dynamic := false
	deps, depErr := manifest.Dependencies(a.Dir)
	if errors.Is(depErr, manifest.ErrDynamicRequirements) {
		dynamic = true
		recipeErr = "动态 requirements()，跳过与配方的逐项对比"
	} else if depErr != nil {
		recipeErr = depErr.Error()
	} else {
		recipeDeps = deps
	}

	projectSet := toSet(project.Dependencies)
	recipeSet := toSet(recipeDeps)
	all := orderedUnion(project.Dependencies, recipeDeps)

	rows := make([]DependencyRow, 0, len(all))
	ok := true
	for _, reference := range all {
		row := DependencyRow{
			Reference: reference,
			InProject: hasKey(projectSet, reference),
			InRecipe:  hasKey(recipeSet, reference),
			Platform:  spec.Display(),
			Settings:  settings.Map(),
		}
		row.Aligned = row.InProject && (dynamic || row.InRecipe || len(recipeDeps) == 0)
		if !row.Aligned && !dynamic {
			row.Status = "mismatch"
			row.Detail = "project.yaml 与 conanfile 不一致"
			ok = false
		} else if targetMissing {
			row.Status = "unknown"
			row.Detail = "尚未选择目标平台，跳过制品查询"
		} else if remote == "" {
			row.Status = "unknown"
			row.Detail = "未配置远程仓库，跳过制品查询"
		} else {
			status, detail := a.lookupBinary(ctx, reference, remote, settings)
			row.Status = status
			row.Detail = detail
			if status != "found" && status != "unknown" {
				ok = false
			}
		}
		rows = append(rows, row)
	}

	message := "依赖分析完成"
	if targetMissing {
		message = "尚未选择目标平台，只对比了配方，未查询制品"
	} else if !ok {
		message = "部分依赖在当前平台没有匹配制品"
	}
	return Report{OK: ok, Action: "analyze", Message: message, Data: map[string]any{
		"os":             settings.OS,
		"arch":           settings.Arch,
		"platform":       spec.Display(),
		"conan_settings": settings.Map(),
		"remote":         remote,
		"dynamic_recipe": dynamic,
		"recipe_error":   recipeErr,
		"dependencies":   rows,
	}}, nil
}

func (a *App) lookupBinary(ctx context.Context, reference, remote string, settings platform.Settings) (string, string) {
	query := reference
	if !strings.Contains(reference, ":") {
		query = reference + ":*"
	}
	data, _, err := a.Client.List(ctx, query, remote)
	if err != nil {
		// RunJSON never populates data when the command or JSON parsing
		// fails, so every error means the remote could not be queried.
		return "unknown", "查询仓库失败: " + err.Error()
	}
	if data == nil || !conan.ListHasReference(data, reference) {
		nameOnly := strings.SplitN(reference, "/", 2)[0]
		nameData, _, nameErr := a.Client.List(ctx, nameOnly, remote)
		if nameErr == nil && conan.ListHasReference(nameData, nameOnly) {
			return "missing_version", "仓库有该包，但没有这个版本"
		}
		return "missing_package", "仓库中没有该包"
	}
	binaries := conan.ExtractBinaries(data)
	if len(binaries) == 0 {
		return "missing_binary", "有配方记录，但没有预编译二进制"
	}
	osArch := 0
	full := 0
	kylinHint := false
	for _, binary := range binaries {
		if !matchSetting(binary.Settings["os"], settings.ConanOS) || !matchSetting(binary.Settings["arch"], settings.ConanArch) {
			continue
		}
		osArch++
		if blobContains(binary, "kylin") || blobContains(binary, "麒麟") {
			kylinHint = true
		}
		if settings.Compiler != "" && binary.Settings["compiler"] != "" && !strings.EqualFold(binary.Settings["compiler"], settings.Compiler) {
			continue
		}
		if settings.CompilerVersion != "" && binary.Settings["compiler.version"] != "" && !strings.HasPrefix(binary.Settings["compiler.version"], settings.CompilerVersion) {
			continue
		}
		if settings.QtVersion != "" && binary.Options["qt_version"] != "" && binary.Options["qt_version"] != settings.QtVersion {
			continue
		}
		full++
	}
	if full > 0 {
		detail := "已找到匹配二进制"
		if settings.OS == config.OSKylin && !kylinHint {
			detail = "已找到 Linux/" + settings.Arch + " 二进制（未单独标注麒麟）"
		}
		return "found", detail
	}
	if osArch > 0 {
		return "missing_binary", "有该操作系统/架构的包，但编译器或 Qt 组合不匹配"
	}
	return "missing_binary", "没有 " + settings.ConanOS + "/" + settings.ConanArch + " 的预编译二进制"
}

func matchSetting(got, want string) bool {
	if want == "" || got == "" {
		return want == "" || got == ""
	}
	return strings.EqualFold(got, want)
}

func blobContains(binary conan.Binary, needle string) bool {
	needle = strings.ToLower(needle)
	for _, value := range binary.Settings {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	for _, value := range binary.Options {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
}

func toSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func hasKey(set map[string]struct{}, key string) bool {
	_, ok := set[key]
	return ok
}

func orderedUnion(left, right []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range append(append([]string{}, left...), right...) {
		if _, ok := seen[value]; ok || value == "" {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
