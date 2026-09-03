package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeBuildType(t *testing.T) {
	if NormalizeBuildType("debug") != BuildTypeDebug || DisplayBuildType("dbg") != "Debug" {
		t.Fatalf("debug = %s %s", NormalizeBuildType("debug"), DisplayBuildType("dbg"))
	}
	if NormalizeBuildType("release") != BuildTypeRelease || !ValidBuildType("Release") {
		t.Fatalf("release = %s", NormalizeBuildType("release"))
	}
	if ValidBuildType("RelWithDebInfo") {
		t.Fatal("RelWithDebInfo should not be accepted in the UI convention")
	}
}

func TestNormalizeAndDisplayArch(t *testing.T) {
	if NormalizeArch("aarch64") != ArchARM64 || DisplayArch("arm64") != "ARM 64 位" {
		t.Fatalf("arm64 = %s %s", NormalizeArch("aarch64"), DisplayArch("arm64"))
	}
	if NormalizeArch("armv7") != ArchARM || DisplayArch("arm") != "ARM 32 位" {
		t.Fatalf("arm32 = %s %s", NormalizeArch("armv7"), DisplayArch("arm"))
	}
	if DisplayArch("x86") != "x86 32 位" || DisplayArch("x64") != "x64 64 位" {
		t.Fatalf("x86/x64 display")
	}
}

func TestNewProjectDetectsQmake(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "demo.pro"), []byte("TEMPLATE = app\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	project := NewProject(dir)
	if project.BuildSystem != "qmake" {
		t.Fatalf("BuildSystem = %q, want qmake", project.BuildSystem)
	}
	if project.MissingBinaryPolicy != "download-only" {
		t.Fatalf("MissingBinaryPolicy = %q, want download-only", project.MissingBinaryPolicy)
	}
	if project.OutputFolder != DefaultOutputFolder {
		t.Fatalf("OutputFolder = %q, want %q", project.OutputFolder, DefaultOutputFolder)
	}
}

func TestSaveLoadProjectAndAddDependency(t *testing.T) {
	dir := t.TempDir()
	project := NewProject(dir)
	if err := AddDependency(project, "fmt/10.2.1"); err != nil {
		t.Fatal(err)
	}
	if err := AddDependency(project, "fmt/10.2.1"); err != nil {
		t.Fatal(err)
	}
	if len(project.Dependencies) != 1 {
		t.Fatalf("dependencies = %d, want 1", len(project.Dependencies))
	}
	if err := SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Dependencies) != 1 || loaded.Dependencies[0] != "fmt/10.2.1" {
		t.Fatalf("loaded dependencies = %#v", loaded.Dependencies)
	}
}

func TestSaveLoadPlatformAndCompiler(t *testing.T) {
	dir := t.TempDir()
	project := NewProject(dir)
	project.QtVersion = "6.8"
	project.Compiler = Compiler{ID: "gcc", Version: "11"}
	project.Platform.Consume = PlatformSpec{OS: "kylin", Arch: "x64"}
	if err := SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Platform.Consume.OS != OSKylin || loaded.Platform.Publish.Arch != ArchX64 {
		t.Fatalf("platform = %#v", loaded.Platform)
	}
	if loaded.Compiler.Display() != "gcc 11" {
		t.Fatalf("compiler = %#v", loaded.Compiler)
	}
}

func TestLoadProjectRejectsUnsupportedMissingBinaryPolicy(t *testing.T) {
	dir := t.TempDir()
	path := ProjectPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte("name: demo\nmissing_binary_policy: build-missing\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProject(dir); err == nil {
		t.Fatal("LoadProject accepted unsupported missing_binary_policy")
	}
}
