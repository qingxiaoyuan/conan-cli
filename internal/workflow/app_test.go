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

func TestReferenceCoordinates(t *testing.T) {
	user, channel := referenceCoordinates("demo/1.0@alice/staging", "dev")
	if user != "alice" || channel != "staging" {
		t.Fatalf("coordinates = %q/%q", user, channel)
	}
	user, channel = referenceCoordinates("demo/1.0", "dev")
	if user != "" || channel != "dev" {
		t.Fatalf("fallback coordinates = %q/%q", user, channel)
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
		Name: "demo", Version: "2.0", OS: "linux", Arch: "x64",
		Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8", DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := report.Data.(map[string]any)
	if data["recipe_action"] != "patch" || data["version"] != "2.0" {
		t.Fatalf("preview data = %#v", data)
	}
	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Fatal("dry-run rewrote conanfile.py")
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
	text := string(got)
	if !strings.Contains(text, `version = "2.0"`) || !strings.Contains(text, `self.run("custom-build")`) {
		t.Fatalf("expected version bump without rewriting build(), got %s", text)
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
	configured := filepath.Join(t.TempDir(), "conan")
	if err := config.SaveGlobal(&config.Global{ConanBin: configured}); err != nil {
		t.Fatal(err)
	}
	if got := New(t.TempDir()).Client.Binary; got != configured {
		t.Fatalf("client binary = %q, want %q", got, configured)
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
