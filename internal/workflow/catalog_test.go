package workflow

import (
	"context"
	"testing"

	"conan-cli/internal/conan"
	"conan-cli/internal/config"
)

func TestCatalogEnrichesBinaries(t *testing.T) {
	app := newLookupApp(t, map[string]string{
		"qtutils*": listJSON("qtutils/1.0"),
		"qtutils/*:*": listJSON("qtutils/1.0", binaryInfo(
			map[string]string{"os": "Linux", "arch": "x86_64", "compiler": "gcc", "compiler.version": "11", "build_type": "Release"},
			map[string]string{"qt_version": "6.8"},
		), binaryInfo(
			map[string]string{"os": "Linux", "arch": "x86_64", "compiler": "gcc", "compiler.version": "11", "build_type": "Debug"},
			map[string]string{"qt_version": "6.8"},
		)),
	})
	report, err := app.Catalog(context.Background(), "qtutils")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := report.Data.(map[string]any)
	packages, _ := data["packages"].([]conan.Package)
	if len(packages) != 1 || packages[0].Name != "qtutils" || len(packages[0].Binaries) != 2 {
		t.Fatalf("packages = %#v data=%#v", packages, data)
	}
	release := packages[0].Binaries[1]
	if packages[0].Binaries[0].BuildType != config.BuildTypeDebug {
		release = packages[0].Binaries[0]
	}
	if release.OS != config.OSLinux || release.Arch != config.ArchX64 || release.Compiler != "gcc" || release.CompilerVersion != "11" || release.QtVersion != "6.8" || release.NoQt || release.BuildType != config.BuildTypeRelease {
		t.Fatalf("binary = %#v", packages[0].Binaries)
	}
}

func TestCatalogFilterBuildTypeAndOS(t *testing.T) {
	app := newLookupApp(t, map[string]string{
		"qtutils*": listJSON("qtutils/1.0"),
		"qtutils/*:*": listJSON("qtutils/1.0", binaryInfo(
			map[string]string{"os": "Linux", "arch": "x86_64", "compiler": "gcc", "compiler.version": "11", "build_type": "Release"},
			map[string]string{"qt_version": "6.8"},
		), binaryInfo(
			map[string]string{"os": "Windows", "arch": "x86_64", "compiler": "msvc", "compiler.version": "193", "build_type": "Debug"},
			map[string]string{},
		)),
	})
	report, err := app.CatalogFilter(context.Background(), CatalogFilter{Query: "qtutils", OS: "linux", BuildType: "Release"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := report.Data.(map[string]any)
	packages, _ := data["packages"].([]conan.Package)
	if len(packages) != 1 || len(packages[0].Binaries) != 1 {
		t.Fatalf("packages = %#v", packages)
	}
	bin := packages[0].Binaries[0]
	if bin.OS != config.OSLinux || bin.BuildType != config.BuildTypeRelease || bin.NoQt {
		t.Fatalf("binary = %#v", bin)
	}
}

func TestCatalogMarksNoQtAndKylin(t *testing.T) {
	app := newLookupApp(t, map[string]string{
		"plainlib*": listJSON("plainlib/2.0"),
		"plainlib/*:*": listJSON("plainlib/2.0", binaryInfo(
			map[string]string{"os": "Linux", "arch": "armv8", "compiler": "gcc", "compiler.version": "13", "build_type": "Release"},
			map[string]string{"distro": "kylin"},
		)),
	})
	report, err := app.Catalog(context.Background(), "plainlib")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := report.Data.(map[string]any)
	packages, _ := data["packages"].([]conan.Package)
	if len(packages) != 1 || len(packages[0].Binaries) != 1 {
		t.Fatalf("packages = %#v", packages)
	}
	bin := packages[0].Binaries[0]
	if bin.OS != config.OSKylin || bin.Arch != config.ArchARM64 || !bin.NoQt || bin.QtVersion != "" {
		t.Fatalf("binary = %#v", bin)
	}
}

func TestAttachBinaryRefsFromNexus(t *testing.T) {
	packages := []conan.Package{{Name: "qtutils", Versions: []string{"1.0"}}}
	packages = attachBinaryRefs(packages, []conan.BinaryRef{{
		Name: "qtutils", Version: "1.0", Reference: "qtutils/1.0",
		Settings: map[string]string{"os": "Linux", "arch": "x86_64", "compiler": "gcc", "compiler.version": "11", "build_type": "Release"},
		Options:  map[string]string{"qt_version": "6.8"},
	}})
	if len(packages[0].Binaries) != 1 || packages[0].Binaries[0].OS != config.OSLinux || packages[0].Binaries[0].QtVersion != "6.8" {
		t.Fatalf("packages = %#v", packages)
	}
}

func TestMatchCatalogBinary(t *testing.T) {
	bin := conan.PackageBinary{OS: "linux", Arch: "x64", Compiler: "gcc", CompilerVersion: "11", BuildType: "Release", QtVersion: "6.8"}
	if !matchCatalogBinary(bin, CatalogFilter{Compiler: "gcc 11"}) {
		t.Fatal("expected gcc 11 label to match")
	}
	if matchCatalogBinary(bin, CatalogFilter{NoQt: true}) {
		t.Fatal("expected qt binary to miss no-qt filter")
	}
	if matchCatalogBinary(bin, CatalogFilter{Qt: "6.5"}) {
		t.Fatal("expected qt 6.5 not to match")
	}
}
