package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDoesNotGuessQtFromPro(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "demo.pro"), []byte("QT += core widgets\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := Project(dir)
	if result.Qt.Value == "6" {
		t.Fatalf("should not guess Qt 6 from QT +=, got %#v", result.Qt)
	}
	if result.Qt.OK {
		t.Fatalf("Qt should not auto-select, got %#v", result.Qt)
	}
}

func TestScanDoesNotGuessQtFromCMake(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CMakeLists.txt"), []byte("find_package(Qt6 REQUIRED COMPONENTS Core)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := Project(dir)
	if result.Qt.Value == "6" {
		t.Fatalf("should not guess Qt 6 from find_package(Qt6), got %#v", result.Qt)
	}
}

func TestGlobQMakesFindsInstallerLayout(t *testing.T) {
	root := t.TempDir()
	kit := filepath.Join(root, "5.14.2", "gcc_64", "bin")
	tools := filepath.Join(root, "Tools", "QtCreator", "bin")
	if err := os.MkdirAll(kit, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tools, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kit, "qmake"), []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tools, "qmake"), []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}
	found := globQMakes([]string{root})
	if len(found) != 1 || filepath.Base(filepath.Dir(filepath.Dir(found[0]))) != "gcc_64" {
		t.Fatalf("found = %#v", found)
	}
}

func TestShortVersion(t *testing.T) {
	if got := shortVersion("5.15.13"); got != "5.15" {
		t.Fatalf("short = %q", got)
	}
	if got := shortVersion("6.8"); got != "6.8" {
		t.Fatalf("short = %q", got)
	}
}

func TestUniqueInstallsKeepsDifferentPrefix(t *testing.T) {
	items := uniqueInstalls([]Install{
		{Version: "5.15.13", Prefix: "/usr"},
		{Version: "5.15.13", Prefix: "/usr"},
		{Version: "5.14.2", Prefix: "/home/u/Qt/5.14.2/gcc_64"},
	})
	if len(items) != 2 {
		t.Fatalf("items = %#v", items)
	}
	if items[0].Version != "5.15.13" {
		t.Fatalf("expected newer first, got %#v", items)
	}
}

func TestChooserToolDir(t *testing.T) {
	env := "QT_SELECT=\"5\"\nQTTOOLDIR=\"/usr/lib/qt5/bin\"\nQTLIBDIR=\"/usr/lib\"\n"
	if got := chooserToolDir(env); got != "/usr/lib/qt5/bin" {
		t.Fatalf("dir = %q", got)
	}
}

func TestVersionGreater(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{a: "6.8", b: "5.15.13", want: true},
		{a: "5.15.13", b: "5.15.2", want: true},
		{a: "5.15", b: "5.15.0", want: false},
		{a: "5.15", b: "5.15", want: false},
		{a: "6", b: "6.0.1", want: false},
		{a: "6.0.1", b: "6", want: true},
	}
	for _, test := range tests {
		if got := versionGreater(test.a, test.b); got != test.want {
			t.Errorf("versionGreater(%q, %q) = %v, want %v", test.a, test.b, got, test.want)
		}
	}
}

func TestSkipQtPathFiltersNonRuntimeDirs(t *testing.T) {
	for _, path := range []string{
		"/opt/Qt/Tools/QtCreator/bin/qmake",
		"/opt/Qt/6.8.0/Docs/bin/qmake",
		"/opt/Qt/Examples/6.8.0/bin/qmake",
	} {
		if !skipQtPath(path) {
			t.Errorf("skipQtPath(%q) = false, want true", path)
		}
	}
	if skipQtPath("/opt/Qt/6.8.0/gcc_64/bin/qmake") {
		t.Error("skipQtPath() skipped a runtime install")
	}
}
