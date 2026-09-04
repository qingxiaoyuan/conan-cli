package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"conan-cli/internal/conan"
	"conan-cli/internal/config"
)

func TestRemoteListContainsMatchesExactName(t *testing.T) {
	output := "conancenter: https://center.conan.io [Verify SSL: True]\nproduction: https://example.test\n"
	if !remoteListContains(output, "production") {
		t.Fatal("expected exact remote name to match")
	}
	if remoteListContains(output, "prod") {
		t.Fatal("remote name matched a substring")
	}
}

func TestReportFromResultPreservesExitCode(t *testing.T) {
	report := reportFromResult("install", conan.Result{Stderr: "missing binary", Code: 6}, assertError{})
	if report.OK || report.ExitCode != 6 || report.Action != "install" {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestDependencyCheck(t *testing.T) {
	check := dependencyCheck([]string{"fmt/10.2.1", "zlib/1.3"}, []string{"zlib/1.3", "fmt/10.2.1"})
	if !check.OK {
		t.Fatalf("matching dependencies failed: %#v", check)
	}
	check = dependencyCheck([]string{"fmt/10.2.1"}, []string{"zlib/1.3"})
	if check.OK {
		t.Fatalf("different dependencies passed: %#v", check)
	}
}

func TestPublishDryRunDoesNotRewriteRecipe(t *testing.T) {
	dir := t.TempDir()
	project := config.NewProject(dir)
	project.Compiler = config.Compiler{ID: "gcc", Version: "13"}
	project.QtVersion = "6.8"
	project.Platform.Publish = config.PlatformSpec{OS: "linux", Arch: "x64"}
	if err := config.SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
	original := "from conan import ConanFile\nclass DemoConan(ConanFile):\n    name = \"demo\"\n    version = \"1.0\"\n    def build(self):\n        self.run(\"custom-build\")\n"
	path := filepath.Join(dir, "conanfile.py")
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := New(dir).PublishPackage(context.Background(), PublishRequest{
		Name: "demo", Version: "2.0", Channel: "dev", OS: "linux", Arch: "x64",
		Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8", DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := report.Data.(map[string]any)
	if data["recipe_action"] != "generate" || data["version"] != "2.0" || data["reference"] != "demo/2.0" {
		t.Fatalf("preview data = %#v", data)
	}
	if data["channel"] != "dev" {
		t.Fatalf("channel = %#v, want dev", data["channel"])
	}
	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Fatal("dry-run rewrote conanfile.py")
	}
	dirs, _ := data["lib_dirs"].([]string)
	if len(dirs) == 0 {
		t.Fatalf("lib_dirs missing: %#v", data["lib_dirs"])
	}
}

func TestPublishDryRunReportsCustomLibDirs(t *testing.T) {
	dir := t.TempDir()
	project := config.NewProject(dir)
	project.Compiler = config.Compiler{ID: "gcc", Version: "13"}
	project.QtVersion = "6.8"
	project.Platform.Publish = config.PlatformSpec{OS: "linux", Arch: "x64"}
	if err := config.SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
	report, err := New(dir).PublishPackage(context.Background(), PublishRequest{
		Name: "demo", Version: "1.0", OS: "linux", Arch: "x64",
		Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8",
		LibDirs: []string{"build/Release"}, DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := report.Data.(map[string]any)
	dirs, _ := data["lib_dirs"].([]string)
	if len(dirs) != 1 || dirs[0] != "build/Release" {
		t.Fatalf("lib_dirs = %#v", data["lib_dirs"])
	}
}

func TestPublishRejectsMissingCustomLibDir(t *testing.T) {
	dir := t.TempDir()
	project := config.NewProject(dir)
	project.Compiler = config.Compiler{ID: "gcc", Version: "13"}
	project.QtVersion = "6.8"
	project.Platform.Publish = config.PlatformSpec{OS: "linux", Arch: "x64"}
	project.Remote = "nexus"
	if err := config.SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
	_, err := New(dir).PublishPackage(context.Background(), PublishRequest{
		Name: "demo", Version: "1.0", OS: "linux", Arch: "x64",
		Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8",
		LibDirs: []string{"build/Release"}, Remote: "nexus",
	})
	if err == nil || !strings.Contains(err.Error(), "build/Release") {
		t.Fatalf("err = %v", err)
	}
}

func TestSaveProjectSettingsPersistsLibDirs(t *testing.T) {
	t.Setenv("CONAN_CLI_HOME", t.TempDir())
	dir := t.TempDir()
	app := New(dir)
	if _, err := app.SaveProjectSettings(ProjectSettingsInput{
		LibDirs: []string{"out/lib"}, IncludeDirs: []string{"src/foo"}, HasLibDirs: true, HasIncludeDirs: true,
	}); err != nil {
		t.Fatal(err)
	}
	project, err := app.Project()
	if err != nil {
		t.Fatal(err)
	}
	spec := project.PrimaryPackage()
	if len(spec.LibDirs) != 1 || spec.LibDirs[0] != "out/lib" || spec.IncludeDirs[0] != "src/foo" {
		t.Fatalf("spec = %#v", spec)
	}
}

func TestSaveProjectSettingsWorkspaces(t *testing.T) {
	t.Setenv("CONAN_CLI_HOME", t.TempDir())
	dir := t.TempDir()
	app := New(dir)
	if _, err := app.SaveProjectSettings(ProjectSettingsInput{
		Workspaces: []string{"pkgs/*", "src/libs/*", "pkgs/*"}, HasWorkspaces: true,
	}); err != nil {
		t.Fatal(err)
	}
	project, err := app.Project()
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Workspaces) != 2 || project.Workspaces[0] != "pkgs/*" || project.Workspaces[1] != "src/libs/*" {
		t.Fatalf("workspaces = %#v", project.Workspaces)
	}
	// 未带 HasWorkspaces 的保存不应改动已有值
	if _, err := app.SaveProjectSettings(ProjectSettingsInput{QtVersion: "6.8"}); err != nil {
		t.Fatal(err)
	}
	if project, err = app.Project(); err != nil {
		t.Fatal(err)
	}
	if len(project.Workspaces) != 2 {
		t.Fatalf("workspaces after unrelated save = %#v", project.Workspaces)
	}
	// 空列表恢复默认（清空字段）
	if _, err := app.SaveProjectSettings(ProjectSettingsInput{Workspaces: []string{""}, HasWorkspaces: true}); err != nil {
		t.Fatal(err)
	}
	if project, err = app.Project(); err != nil {
		t.Fatal(err)
	}
	if len(project.Workspaces) != 0 {
		t.Fatalf("workspaces after reset = %#v", project.Workspaces)
	}
	// 非法 glob 报错
	if _, err := app.SaveProjectSettings(ProjectSettingsInput{Workspaces: []string{"../escape/*"}, HasWorkspaces: true}); err == nil {
		t.Fatal("expected error for escaping glob")
	}
}

func TestPublishWritesRecipeThenPackages(t *testing.T) {
	dir := t.TempDir()
	project := config.NewProject(dir)
	project.Compiler = config.Compiler{ID: "gcc", Version: "13"}
	project.QtVersion = "6.8"
	project.Platform.Publish = config.PlatformSpec{OS: "linux", Arch: "x64"}
	project.Remote = "nexus"
	if err := config.SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "conanfile.py")
	if err := os.WriteFile(path, []byte("from conan import ConanFile\nclass DemoConan(ConanFile):\n    name = \"demo\"\n    version = \"1.0\"\n    def build(self):\n        self.run(\"custom-build\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lib", "libdemo.a"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(dir, "fake-conan")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	app := New(dir)
	app.Client.Binary = fake
	report, err := app.PublishPackage(context.Background(), PublishRequest{
		Name: "demo", Version: "2.0", OS: "linux", Arch: "x64",
		Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8", Remote: "nexus",
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !report.OK || report.Action != "publish" {
		t.Fatalf("report = %#v", report)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), `version = "1.0"`) || !strings.Contains(string(got), `self.run("custom-build")`) {
		t.Fatalf("root consume recipe was rewritten: %s", got)
	}
	isolated, err := os.ReadFile(filepath.Join(dir, ".conan-cli", "recipes", "demo", "conanfile.py"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(isolated), `version = "2.0"`) || !strings.Contains(string(isolated), "kind: publish") {
		t.Fatalf("isolated recipe = %s", isolated)
	}
	saved, err := config.LoadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if spec := saved.PrimaryPackage(); spec.Name != "demo" || spec.Version != "2.0" {
		t.Fatalf("saved package = %#v", spec)
	}
}

func TestPublishRequiresPackageWhenMultiple(t *testing.T) {
	dir := t.TempDir()
	project := config.NewProject(dir)
	project.Compiler = config.Compiler{ID: "gcc", Version: "13"}
	project.QtVersion = "6.8"
	project.Platform.Publish = config.PlatformSpec{OS: "linux", Arch: "x64"}
	project.Packages = []config.PackageSpec{{Name: "alpha"}, {Name: "beta"}}
	if err := config.SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
	_, err := New(dir).PublishPackage(context.Background(), PublishRequest{
		Version: "1.0", OS: "linux", Arch: "x64",
		Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8", DryRun: true,
	})
	if err == nil || !strings.Contains(err.Error(), "--package") {
		t.Fatalf("err = %v", err)
	}
	report, err := New(dir).PublishPackage(context.Background(), PublishRequest{
		Package: "beta", Version: "1.0", OS: "linux", Arch: "x64",
		Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8", DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := report.Data.(map[string]any)
	if data["name"] != "beta" || data["package"] != "beta" {
		t.Fatalf("data = %#v", data)
	}
}

func TestPublishAllowsMissingQt(t *testing.T) {
	dir := t.TempDir()
	project := config.NewProject(dir)
	project.Compiler = config.Compiler{ID: "gcc", Version: "13"}
	project.QtVersion = "6.8"
	project.Platform.Publish = config.PlatformSpec{OS: "linux", Arch: "x64"}
	project.Packages = []config.PackageSpec{{Name: "plainlib", NoQt: true}}
	if err := config.SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
	report, err := New(dir).PublishPackage(context.Background(), PublishRequest{
		Package: "plainlib", Version: "1.0", OS: "linux", Arch: "x64",
		Compiler: "gcc", CompilerVersion: "13", NoQt: true, DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := report.Data.(map[string]any)
	if data["qt_version"] != "" {
		t.Fatalf("qt_version = %#v, want empty", data["qt_version"])
	}
	settings, _ := data["conan_settings"].(map[string]string)
	if settings["qt_version"] != "" {
		t.Fatalf("settings qt = %#v", settings)
	}
}

func TestApplyPackageIdentityUsesQmakeTarget(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "goodluckbutton.pro"), []byte("TEMPLATE = lib\nTARGET = goodluckbutton\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := config.NewProject(dir)
	if filepath.Base(dir) == "goodluckbutton" {
		t.Skip("temp dir already named like the library")
	}
	if project.Name == "goodluckbutton" {
		t.Fatal("NewProject should not yet lock onto TARGET")
	}
	fillProjectDefaults(dir, project)
	if project.Name != "goodluckbutton" {
		t.Fatalf("name = %q, want goodluckbutton", project.Name)
	}
	if project.NameLocked {
		t.Fatal("auto detect should not lock")
	}
}

func TestLockedPackageNameIsKept(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "goodluckbutton.pro"), []byte("TEMPLATE = lib\nTARGET = goodluckbutton\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := config.NewProject(dir)
	project.Name = "qt-test-1"
	project.NameLocked = true
	fillProjectDefaults(dir, project)
	if project.Name != "qt-test-1" {
		t.Fatalf("locked name overwritten: %q", project.Name)
	}
}

func TestPublishUsesDetectedNameWhenOmitted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "goodluckbutton.pro"), []byte("TEMPLATE = lib\nTARGET = goodluckbutton\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := config.NewProject(dir)
	project.Compiler = config.Compiler{ID: "gcc", Version: "13"}
	project.QtVersion = "6.8"
	project.Platform.Publish = config.PlatformSpec{OS: "linux", Arch: "x64"}
	project.Remote = "nexus"
	if err := config.SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lib", "libgoodluckbutton.a"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(dir, "fake-conan")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	app := New(dir)
	app.Client.Binary = fake
	report, err := app.PublishPackage(context.Background(), PublishRequest{
		Version: "1.0.1", OS: "linux", Arch: "x64",
		Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8", Remote: "nexus", DryRun: true,
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	data, _ := report.Data.(map[string]any)
	if data["name"] != "goodluckbutton" || data["reference"] != "goodluckbutton/1.0.1" {
		t.Fatalf("data = %#v", data)
	}
}

func TestSplitNameVersion(t *testing.T) {
	name, version := splitNameVersion("qtutils/1.1@alice/dev")
	if name != "qtutils" || version != "1.1" {
		t.Fatalf("got %q/%q", name, version)
	}
}

func TestFillProjectDefaultsDoesNotUseHost(t *testing.T) {
	dir := t.TempDir()
	project := config.NewProject(dir)
	fillProjectDefaults(dir, project)
	if !project.Platform.Consume.Empty() {
		t.Fatalf("consume should stay empty, got %#v", project.Platform.Consume)
	}
	if project.Compiler.ID != "" || project.QtVersion != "" {
		t.Fatalf("toolchain should stay empty, compiler=%#v qt=%q", project.Compiler, project.QtVersion)
	}
}

func TestInitCreatesMissingProjectDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new-project")
	report, err := New(dir).Init(context.Background())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if !report.OK {
		t.Fatalf("Init() report = %#v", report)
	}
	for _, path := range []string{config.ProjectPath(dir), filepath.Join(dir, "conanfile.txt")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}
}

func TestNewUsesConfiguredConanBinary(t *testing.T) {
	t.Setenv("CONAN_CLI_HOME", t.TempDir())
	t.Setenv("CONAN_BIN", "")
	t.Setenv("CONAN_CLI_BUNDLED_PYTHON", "/opt/bundled/python3")
	configured := filepath.Join(t.TempDir(), "conan")
	if err := config.SaveGlobal(&config.Global{ConanBin: configured}); err != nil {
		t.Fatal(err)
	}
	client := New(t.TempDir()).Client
	if client.Binary != configured {
		t.Fatalf("client binary = %q, want %q", client.Binary, configured)
	}
	if len(client.BaseArgs) != 0 {
		t.Fatalf("base args = %#v", client.BaseArgs)
	}
}

func TestNewUsesBundledPythonWhenUnconfigured(t *testing.T) {
	t.Setenv("CONAN_CLI_HOME", t.TempDir())
	t.Setenv("CONAN_BIN", "")
	t.Setenv("CONAN_CLI_BUNDLED_PYTHON", "/opt/bundled/python3")
	client := New(t.TempDir()).Client
	if client.Binary != "/opt/bundled/python3" {
		t.Fatalf("client binary = %q", client.Binary)
	}
	if len(client.BaseArgs) != 3 || client.BaseArgs[2] != "conans.conan" {
		t.Fatalf("base args = %#v", client.BaseArgs)
	}
}

func TestInstallRequiresTargetPlatform(t *testing.T) {
	t.Setenv("CONAN_CLI_HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "conanfile.txt"), []byte("[requires]\nfmt/10.2.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := config.NewProject(dir)
	if err := config.SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
	_, err := New(dir).InstallPlatform(context.Background(), InstallRequest{})
	if err == nil || !strings.Contains(err.Error(), "目标操作系统") {
		t.Fatalf("err = %v", err)
	}
}

func TestCatalogWithoutRemote(t *testing.T) {
	t.Setenv("CONAN_CLI_HOME", t.TempDir())
	_, err := New(t.TempDir()).Catalog(context.Background(), "")
	if err == nil {
		t.Fatal("expected catalog to fail without a repository")
	}
}

func TestAnalyzeWithoutTargetSkipsBinaryLookup(t *testing.T) {
	t.Setenv("CONAN_CLI_HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "conanfile.txt"), []byte("[requires]\nfmt/10.2.1\n\n[generators]\nCMakeDeps\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := config.NewProject(dir)
	project.Dependencies = []string{"fmt/10.2.1"}
	if err := config.SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
	report, err := New(dir).Analyze(context.Background(), "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := report.Data.(map[string]any)
	rows, _ := data["dependencies"].([]DependencyRow)
	if len(rows) != 1 || rows[0].Status != "unknown" {
		t.Fatalf("rows = %#v data=%#v", rows, data)
	}
}

func TestAnalyzeMarksAlignmentWithoutRemote(t *testing.T) {
	t.Setenv("CONAN_CLI_HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "conanfile.txt"), []byte("[requires]\nfmt/10.2.1\n\n[generators]\nCMakeDeps\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := config.NewProject(dir)
	project.Dependencies = []string{"fmt/10.2.1"}
	project.Platform.Consume = config.PlatformSpec{OS: "kylin", Arch: "x64"}
	if err := config.SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
	report, err := New(dir).Analyze(context.Background(), "kylin", "x64", "Release")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := report.Data.(map[string]any)
	rows, _ := data["dependencies"].([]DependencyRow)
	if len(rows) != 1 || rows[0].Status != "unknown" || !rows[0].Aligned {
		t.Fatalf("rows = %#v data=%#v", rows, data)
	}
}

type assertError struct{}

func (assertError) Error() string { return "command failed" }
