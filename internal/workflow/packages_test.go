package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"conan-cli/internal/config"
)

// writeWorkspaceComponent creates packages/<name>/ with a hand-written
// conanfile.py and a dist/lib + dist/include prebuilt layout.
func writeWorkspaceComponent(t *testing.T, root, name, version string) {
	t.Helper()
	dir := filepath.Join(root, "packages", name)
	for _, sub := range []string{filepath.Join("dist", "lib"), filepath.Join("dist", "include")} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	className := strings.ToUpper(name[:1]) + name[1:]
	recipe := fmt.Sprintf(`from conan import ConanFile


class %sConan(ConanFile):
    name = %q
    version = %q
    settings = "os", "compiler", "build_type", "arch"
`, className, name, version)
	if err := os.WriteFile(filepath.Join(dir, "conanfile.py"), []byte(recipe), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dist", "lib", "lib"+name+".a"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func savePublishProject(t *testing.T, dir string) {
	t.Helper()
	project := config.NewProject(dir)
	project.Name = "testproj"
	project.NameLocked = true
	project.Compiler = config.Compiler{ID: "gcc", Version: "13"}
	project.QtVersion = "6.8"
	project.Platform.Publish = config.PlatformSpec{OS: "linux", Arch: "x64"}
	project.Remote = "nexus"
	if err := config.SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
}

func isolateEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CONAN_CLI_HOME", t.TempDir())
	t.Setenv("CONAN_BIN", "")
	t.Setenv("CONAN_CLI_BUNDLED_PYTHON", "")
}

func publishAllRequest() PublishRequest {
	return PublishRequest{
		All: true, OS: "linux", Arch: "x64", Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8",
	}
}

func TestPackagesListMergesWorkspaceAndDeclared(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	writeWorkspaceComponent(t, dir, "wslib", "1.0.0")
	// otherlib：只有产物，无配方。
	otherDir := filepath.Join(dir, "packages", "otherlib")
	if err := os.MkdirAll(filepath.Join(otherDir, "dist", "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "dist", "lib", "libotherlib.a"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := config.NewProject(dir)
	project.Name = "testproj"
	project.NameLocked = true
	project.Packages = []config.PackageSpec{{Name: "wslib", Version: "9.9"}, {Name: "declaredonly"}}
	if err := config.SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}

	report, err := New(dir).PackagesList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Action != "packages-list" {
		t.Fatalf("action = %q", report.Action)
	}
	data, _ := report.Data.(map[string]any)
	infos, _ := data["packages"].([]PackageInfo)
	if len(infos) != 3 {
		t.Fatalf("packages = %#v", infos)
	}
	if infos[0].Name != "declaredonly" || infos[1].Name != "otherlib" || infos[2].Name != "wslib" {
		t.Fatalf("not sorted: %#v", infos)
	}
	if infos[0].Source != "declared" {
		t.Fatalf("declaredonly = %#v", infos[0])
	}
	if infos[1].Source != "workspace" || !infos[1].HasArtifacts || infos[1].HasRecipe {
		t.Fatalf("otherlib = %#v", infos[1])
	}
	// 同名时 declared 优先，但 workspace 的 dir/has_recipe 保留。
	ws := infos[2]
	if ws.Source != "declared" || ws.Version != "9.9" || ws.Dir != "packages/wslib" || !ws.HasRecipe || !ws.HasArtifacts {
		t.Fatalf("wslib = %#v", ws)
	}
}

func TestPackagesListFallsBackToSyntheticComponent(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	project := config.NewProject(dir)
	project.Name = "lonely"
	project.NameLocked = true
	if err := config.SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
	report, err := New(dir).PackagesList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	infos, _ := report.Data.(map[string]any)["packages"].([]PackageInfo)
	if len(infos) != 1 || infos[0].Name != "lonely" || infos[0].Source != "declared" {
		t.Fatalf("packages = %#v", infos)
	}
}

func TestStatusIncludesPackages(t *testing.T) {
	app := newFakeApp(t, nil)
	savePublishProject(t, app.Dir)
	writeWorkspaceComponent(t, app.Dir, "statuslib", "1.0.0")
	report, err := app.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	infos, _ := report.Data.(map[string]any)["packages"].([]PackageInfo)
	if len(infos) != 1 || infos[0].Name != "statuslib" || infos[0].Source != "workspace" {
		t.Fatalf("packages = %#v", infos)
	}
}

func TestPublishAllConflictsWithPackage(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	savePublishProject(t, dir)
	_, err := New(dir).PublishPackage(context.Background(), PublishRequest{All: true, Package: "x"})
	if err == nil || !strings.Contains(err.Error(), "--all 与 --package") {
		t.Fatalf("err = %v", err)
	}
}

func TestPublishAllDryRunAggregates(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	savePublishProject(t, dir)
	writeWorkspaceComponent(t, dir, "alphalib", "1.0.0")
	writeWorkspaceComponent(t, dir, "betalib", "2.0.0")
	betaBefore, _ := os.ReadFile(filepath.Join(dir, "packages", "betalib", "conanfile.py"))

	report, err := New(dir).PublishPackage(context.Background(), PublishRequest{
		All: true, DryRun: true, OS: "linux", Arch: "x64", Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Action != "publish-preview" {
		t.Fatalf("report = %#v", report)
	}
	results, _ := report.Data.(map[string]any)["results"].([]map[string]any)
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	if results[0]["package"] != "alphalib" || results[0]["reference"] != "alphalib/1.0.0" || results[0]["ok"] != true {
		t.Fatalf("results[0] = %#v", results[0])
	}
	if results[1]["reference"] != "betalib/2.0.0" || results[1]["recipe_action"] != "patch" {
		t.Fatalf("results[1] = %#v", results[1])
	}
	// dry-run 不写任何东西：无 recipes 目录，workspace 配方原样。
	if _, statErr := os.Stat(filepath.Join(dir, ".conan-cli", "recipes")); !os.IsNotExist(statErr) {
		t.Fatal("dry-run created .conan-cli/recipes")
	}
	betaAfter, _ := os.ReadFile(filepath.Join(dir, "packages", "betalib", "conanfile.py"))
	if string(betaBefore) != string(betaAfter) {
		t.Fatal("dry-run rewrote workspace recipe")
	}
}

func TestPublishAllPartialFailureDoesNotStop(t *testing.T) {
	app := newFakeApp(t, map[string]string{
		"upload badlib/1.0.0": "FAIL remote exploded",
	})
	savePublishProject(t, app.Dir)
	writeWorkspaceComponent(t, app.Dir, "badlib", "1.0.0")
	writeWorkspaceComponent(t, app.Dir, "goodlib", "1.0.0")

	report, err := app.PublishPackage(context.Background(), publishAllRequest())
	if err == nil {
		t.Fatal("expected aggregated failure")
	}
	if report.OK || report.ExitCode == 0 {
		t.Fatalf("report = %#v", report)
	}
	if !strings.Contains(report.Message, "1 成功 1 失败") {
		t.Fatalf("message = %q", report.Message)
	}
	results, _ := report.Data.(map[string]any)["results"].([]map[string]any)
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	// 按名称排序：badlib 先失败，goodlib 仍然发布成功（不中断）。
	if results[0]["package"] != "badlib" || results[0]["ok"] != false {
		t.Fatalf("results[0] = %#v", results[0])
	}
	if !strings.Contains(results[0]["error"].(string), "remote exploded") {
		t.Fatalf("results[0] error = %#v", results[0]["error"])
	}
	if results[1]["package"] != "goodlib" || results[1]["ok"] != true || results[1]["reference"] != "goodlib/1.0.0" {
		t.Fatalf("results[1] = %#v", results[1])
	}
}

func TestPublishReplaceDeletesPreviousVersion(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	savePublishProject(t, dir)
	writeWorkspaceComponent(t, dir, "wslib", "1.0.0")
	fake, log := argsLoggingConan(t)
	app := New(dir)
	app.Client.Binary = fake

	// dry-run 预览暴露 previous_version，供界面提示。
	preview, err := app.PublishPackage(context.Background(), PublishRequest{
		Package: "wslib", Version: "2.0", OS: "linux", Arch: "x64",
		Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8", Remote: "nexus",
		DryRun: true, Replace: true,
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if data, _ := preview.Data.(map[string]any); data["previous_version"] != "1.0.0" {
		t.Fatalf("preview previous_version = %#v", data["previous_version"])
	}

	report, err := app.PublishPackage(context.Background(), PublishRequest{
		Package: "wslib", Version: "2.0", OS: "linux", Arch: "x64",
		Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8", Remote: "nexus",
		Replace: true,
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	data, _ := report.Data.(map[string]any)
	if data["replaced_reference"] != "wslib/1.0.0" {
		t.Fatalf("replaced_reference = %#v", data["replaced_reference"])
	}
	logData, _ := os.ReadFile(log)
	args := strings.Split(string(logData), "\n")
	foundRemove := false
	for i, arg := range args {
		if arg == "remove" && i+1 < len(args) && args[i+1] == "wslib/1.0.0" {
			rest := strings.Join(args[i+2:i+4], " ")
			if strings.Contains(rest, "--force") && strings.Contains(rest, "--remote=nexus") {
				foundRemove = true
			}
		}
	}
	if !foundRemove {
		t.Fatalf("remove not called with force/remote:\n%s", logData)
	}
}

func TestPublishReplaceWithoutVersionChangeIsNoop(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	savePublishProject(t, dir)
	writeWorkspaceComponent(t, dir, "wslib", "1.0.0")
	fake, log := argsLoggingConan(t)
	app := New(dir)
	app.Client.Binary = fake

	if _, err := app.PublishPackage(context.Background(), PublishRequest{
		Package: "wslib", Version: "1.0.0", OS: "linux", Arch: "x64",
		Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8", Remote: "nexus",
		Replace: true,
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	logData, _ := os.ReadFile(log)
	if strings.Contains(string(logData), "remove") {
		t.Fatalf("unexpected remove call:\n%s", logData)
	}
}

func TestPublishReplaceFailureIsWarningNotError(t *testing.T) {
	app := newFakeApp(t, map[string]string{
		"remove wslib/1.0.0": "FAIL nexus refused",
	})
	savePublishProject(t, app.Dir)
	writeWorkspaceComponent(t, app.Dir, "wslib", "1.0.0")

	report, err := app.PublishPackage(context.Background(), PublishRequest{
		Package: "wslib", Version: "2.0", OS: "linux", Arch: "x64",
		Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8", Remote: "nexus",
		Replace: true,
	})
	if err != nil || !report.OK {
		t.Fatalf("publish should still succeed: err=%v report=%#v", err, report)
	}
	data, _ := report.Data.(map[string]any)
	if warning, _ := data["replace_warning"].(string); !strings.Contains(warning, "wslib/1.0.0") {
		t.Fatalf("replace_warning = %#v", data["replace_warning"])
	}
}

func TestPublishWorkspaceExportPkgTargetsWorkspaceDir(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	savePublishProject(t, dir)
	writeWorkspaceComponent(t, dir, "wslib", "1.0.0")
	fake, log := argsLoggingConan(t)
	app := New(dir)
	app.Client.Binary = fake

	report, err := app.PublishPackage(context.Background(), PublishRequest{
		Package: "wslib", Version: "2.0", OS: "linux", Arch: "x64",
		Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8", Remote: "nexus",
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !report.OK {
		t.Fatalf("report = %#v", report)
	}
	logData, _ := os.ReadFile(log)
	lines := strings.Split(string(logData), "\n")
	found := false
	for i, line := range lines {
		if line == "export-pkg" && i+1 < len(lines) && lines[i+1] == "packages/wslib" {
			found = true
		}
	}
	if !found {
		t.Fatalf("export-pkg did not target workspace dir:\n%s", logData)
	}
	// 版本与配方不同：就地补丁 workspace 配方。
	recipe, _ := os.ReadFile(filepath.Join(dir, "packages", "wslib", "conanfile.py"))
	if !strings.Contains(string(recipe), `version = "2.0"`) {
		t.Fatalf("workspace recipe not patched:\n%s", recipe)
	}
	// workspace 组件不登记进 packages[]。
	saved, err := config.LoadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Packages) != 0 {
		t.Fatalf("workspace component leaked into packages[]: %#v", saved.Packages)
	}
	// 不应生成隔离配方。
	if _, statErr := os.Stat(filepath.Join(dir, ".conan-cli", "recipes")); !os.IsNotExist(statErr) {
		t.Fatal("workspace publish created .conan-cli/recipes")
	}
}

func TestPublishPackageMatchesWorkspace(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	savePublishProject(t, dir)
	writeWorkspaceComponent(t, dir, "alphalib", "1.0.0")
	writeWorkspaceComponent(t, dir, "betalib", "2.0.0")

	report, err := New(dir).PublishPackage(context.Background(), PublishRequest{
		Package: "betalib", DryRun: true, OS: "linux", Arch: "x64",
		Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8",
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := report.Data.(map[string]any)
	if data["name"] != "betalib" || data["version"] != "2.0.0" || data["recipe_dir"] != "packages/betalib" {
		t.Fatalf("data = %#v", data)
	}
	if data["recipe_action"] != "patch" {
		t.Fatalf("recipe_action = %#v", data["recipe_action"])
	}
}

func TestPublishMultiComponentErrorListsWorkspaceNames(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	savePublishProject(t, dir)
	writeWorkspaceComponent(t, dir, "alphalib", "1.0.0")
	writeWorkspaceComponent(t, dir, "betalib", "2.0.0")

	_, err := New(dir).PublishPackage(context.Background(), PublishRequest{
		DryRun: true, OS: "linux", Arch: "x64", Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8",
	})
	if err == nil || !strings.Contains(err.Error(), "--package") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "alphalib") || !strings.Contains(err.Error(), "betalib") {
		t.Fatalf("error should list components: %v", err)
	}
}
