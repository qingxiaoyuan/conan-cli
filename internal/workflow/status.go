package workflow

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"conan-cli/internal/config"
	"conan-cli/internal/manifest"
	"conan-cli/internal/scan"
)

// ScanResult 是 Report.Data["scan"] 的类型；界面层只依赖 workflow。
type ScanResult = scan.Result

func (a *App) Status(ctx context.Context) (Report, error) {
	scanResult := scan.Project(a.Dir)
	global, _ := config.LoadGlobal()
	if global == nil {
		global = &config.Global{}
	}
	nameGuess := manifest.DetectPackageName(a.Dir)
	// project.yaml 的读-改-写全程持锁：BubbleTea 控制台会并行触发
	// Status 与静默保存，last-writer-wins 会丢字段。
	a.saveMu.Lock()
	project, projectErr := a.Project()
	initialized := projectErr == nil
	if initialized && applyPackageIdentity(a.Dir, project) {
		if saveErr := config.SaveProject(a.Dir, project); saveErr == nil {
			project, projectErr = a.Project()
			initialized = projectErr == nil
		}
	}
	a.saveMu.Unlock()
	conanfile := ""
	if manifest.HasConanfile(a.Dir) {
		if fileExists(a.Dir, "conanfile.py") {
			conanfile = "conanfile.py"
		} else {
			conanfile = "conanfile.txt"
		}
	}

	recipe := map[string]any{}
	if metadata, _, inspectErr := a.Client.Inspect(ctx); inspectErr == nil {
		recipe["name"] = metadata["name"]
		recipe["version"] = metadata["version"]
		recipe["inspectable"] = true
	} else {
		recipe["inspectable"] = false
		recipe["error"] = inspectErr.Error()
	}
	if kind := manifest.GeneratedKind(a.Dir); kind != "" {
		recipe["kind"] = string(kind)
		recipe["generated"] = true
	} else if fileExists(a.Dir, "conanfile.py") {
		recipe["generated"] = false
	}

	data := map[string]any{
		"initialized":        initialized,
		"project":            project,
		"global":             global.View(),
		"scan":               scanResult,
		"recipe":             recipe,
		"conanfile":          conanfile,
		"host":               scanResult.Host,
		"machine":            map[string]string{"os": runtime.GOOS, "arch": runtime.GOARCH},
		"package_name":       nameGuess,
		"package_candidates": manifest.DetectPackageNames(a.Dir),
	}
	if projectErr == nil {
		data["packages"] = componentInfos(resolveComponents(a.Dir, project))
	}
	if projectErr != nil {
		data["project_error"] = projectErr.Error()
	}
	return Report{OK: true, Action: "status", Data: data}, nil
}

func (a *App) Scan(_ context.Context) (Report, error) {
	result := scan.Project(a.Dir)
	data := map[string]any{"scan": result}
	message := "扫描仅供参考，不会写入项目。请按目标制品手填 Qt/编译器和平台"
	if n := len(result.QtInstalls); n > 0 {
		message = fmt.Sprintf("本机看到 %d 套 Qt，仅供参考；目标版本请在设置中手填或选用", n)
	}
	var lines []string
	for _, install := range result.QtInstalls {
		line := "Qt " + install.Version
		if install.Prefix != "" {
			line += "  " + install.Prefix
		}
		lines = append(lines, line)
	}
	if result.CompilerFinding.Value != "" {
		lines = append(lines, "编译器 "+result.CompilerFinding.Value+"（本机 PATH，仅供参考）")
	}
	return Report{OK: true, Action: "scan", Message: message, Output: strings.Join(lines, "\n"), Data: data}, nil
}

func (a *App) ensureProject() (*config.Project, error) {
	project, err := a.Project()
	if err == nil {
		return project, nil
	}
	project = config.NewProject(a.Dir)
	fillProjectDefaults(a.Dir, project)
	if saveErr := config.SaveProject(a.Dir, project); saveErr != nil {
		return nil, saveErr
	}
	return project, nil
}

func fileExists(dir, name string) bool {
	return hasNamedFile(dir, name)
}
