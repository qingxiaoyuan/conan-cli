package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"conan-cli/internal/config"
)

func TestResolveRemoteFallbackOrder(t *testing.T) {
	t.Setenv("CONAN_CLI_HOME", t.TempDir())

	project := &config.Project{Remote: "project-remote"}
	if got := resolveRemote("explicit", project); got != "explicit" {
		t.Fatalf("explicit remote = %q", got)
	}
	if got := resolveRemote("", project); got != "project-remote" {
		t.Fatalf("project remote = %q", got)
	}
	if got := resolveRemote("  ", project); got != "project-remote" {
		t.Fatalf("blank explicit remote should fall through, got %q", got)
	}

	if err := config.SaveGlobal(&config.Global{Nexus: config.Nexus{Name: "global-nexus", URL: "https://nexus.test/repo"}}); err != nil {
		t.Fatal(err)
	}
	if got := resolveRemote("", &config.Project{}); got != "global-nexus" {
		t.Fatalf("global remote = %q", got)
	}

	// 全局仓库只有名字、没有 URL 时不能作为兜底，避免把未注册的 remote
	// 传给 Conan。
	if err := config.SaveGlobal(&config.Global{Nexus: config.Nexus{Name: "name-only"}}); err != nil {
		t.Fatal(err)
	}
	if got := resolveRemote("", &config.Project{}); got != "" {
		t.Fatalf("global remote without URL = %q, want empty", got)
	}
}

func TestDoctorWithFakeConan(t *testing.T) {
	app := newFakeApp(t, map[string]string{
		"--version --version": "Conan version 2.32.0",
		"remote list":         "nexus: https://nexus.test/repo [Verify SSL: True]",
	})
	project := config.NewProject(app.Dir)
	project.Dependencies = []string{"fmt/10.2.1"}
	project.Compiler = config.Compiler{ID: "gcc", Version: "13"}
	project.QtVersion = "6.8"
	project.Platform.Consume = config.PlatformSpec{OS: "linux", Arch: "x64"}
	project.Remote = "nexus"
	if err := config.SaveProject(app.Dir, project); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app.Dir, "conanfile.txt"), []byte("[requires]\nfmt/10.2.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := app.Doctor(context.Background())
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if !report.OK {
		for _, check := range report.Checks {
			if !check.OK {
				t.Logf("failed check: %s: %s", check.Name, check.Detail)
			}
		}
		t.Fatal("Doctor() report not OK")
	}
	for _, name := range []string{"conan", "project_config", "conanfile", "manifest_dependencies", "profiles", "remotes", "configured_remote", "platform"} {
		found := false
		for _, check := range report.Checks {
			if check.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Doctor() missing check %q in %#v", name, report.Checks)
		}
	}
}

func TestInstallPlatformSuccessUsesDownloadOnlyAndSettings(t *testing.T) {
	binary, log := argsLoggingConan(t)
	t.Setenv("CONAN_CLI_HOME", t.TempDir())
	dir := t.TempDir()
	project := config.NewProject(dir)
	project.Dependencies = []string{"fmt/10.2.1"}
	project.Compiler = config.Compiler{ID: "gcc", Version: "13"}
	project.QtVersion = "6.8"
	project.Platform.Consume = config.PlatformSpec{OS: "kylin", Arch: "x64"}
	project.Remote = "nexus"
	if err := config.SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "conanfile.txt"), []byte("[requires]\nfmt/10.2.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := New(dir)
	app.Client.Binary = binary

	report, err := app.InstallPlatform(context.Background(), InstallRequest{})
	if err != nil {
		t.Fatalf("InstallPlatform() error = %v", err)
	}
	if !report.OK {
		t.Fatalf("report = %#v", report)
	}
	data, _ := report.Data.(map[string]any)
	if data["remote"] != "nexus" || data["warning"] != nil {
		t.Fatalf("data = %#v", data)
	}
	logged, readErr := os.ReadFile(log)
	if readErr != nil {
		t.Fatal(readErr)
	}
	args := strings.Split(strings.TrimSpace(string(logged)), "\n")
	wantArgs := []string{"install", ".", "--output-folder=conan", "--build=never", "--profile:host=default", "--remote=nexus",
		"-s", "os=Linux", "-s", "arch=x86_64", "-s", "compiler=gcc", "-s", "compiler.version=13", "-s", "build_type=Release", "-o", "*:qt_version=6.8"}
	if strings.Join(args, " ") != strings.Join(wantArgs, " ") {
		t.Fatalf("conan install args = %v, want %v", args, wantArgs)
	}
}

func TestConfigTestReportsMissingRemote(t *testing.T) {
	app := newFakeApp(t, map[string]string{
		"remote list": "conancenter: https://center.conan.io [Verify SSL: True]",
	})
	if err := config.SaveGlobal(&config.Global{Nexus: config.Nexus{Name: "nexus", URL: "https://nexus.test/repo", Username: "alice"}}); err != nil {
		t.Fatal(err)
	}
	report, err := app.ConfigTest(context.Background())
	if err == nil || report.OK {
		t.Fatalf("expected failure, report = %#v err = %v", report, err)
	}
	if !strings.Contains(report.Error, "nexus") {
		t.Fatalf("error = %q", report.Error)
	}
}

func TestConfigTestPassesWhenRemoteListed(t *testing.T) {
	app := newFakeApp(t, map[string]string{
		"remote list": "nexus: https://nexus.test/repo [Verify SSL: True]",
	})
	if err := config.SaveGlobal(&config.Global{Nexus: config.Nexus{Name: "nexus", URL: "https://nexus.test/repo", Username: "alice"}}); err != nil {
		t.Fatal(err)
	}
	report, err := app.ConfigTest(context.Background())
	if err != nil {
		t.Fatalf("ConfigTest() error = %v", err)
	}
	if !report.OK || report.Message != "仓库可达: nexus" {
		t.Fatalf("report = %#v", report)
	}
}

func TestSaveGlobalSettingsSurfacesRemoteAddFailure(t *testing.T) {
	app := newFakeApp(t, map[string]string{
		"remote add": "FAIL cannot reach server",
	})
	report, err := app.SaveGlobalSettings(context.Background(), GlobalSettingsInput{
		Name: "nexus", URL: "https://nexus.test/repo", Username: "alice",
	})
	if err == nil || report.OK {
		t.Fatalf("expected failure, report = %#v err = %v", report, err)
	}
	if report.Message != "全局配置已保存，但添加 Conan remote 失败" {
		t.Fatalf("message = %q", report.Message)
	}
	// 全局配置本身应已落盘。
	saved, loadErr := config.LoadGlobal()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if saved.Nexus.Name != "nexus" || saved.Nexus.URL != "https://nexus.test/repo" {
		t.Fatalf("saved global = %#v", saved.Nexus)
	}
}
