package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPackageNameQmakeLibNotFolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qt-test-1.pro"), []byte("TEMPLATE = subdirs\nSUBDIRS += goodluckbutton example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "goodluckbutton.pro"), []byte("QT += widgets\nTEMPLATE = lib\nTARGET = goodluckbutton\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "include", "goodluckbutton"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "example"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "example", "example.pro"), []byte("TEMPLATE = app\nTARGET = example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := DetectPackageName(dir)
	if got.Name != "goodluckbutton" || got.Source != "qmake" {
		t.Fatalf("got %#v, want goodluckbutton/qmake", got)
	}
}

func TestDetectPackageNameSkipsConsumeRecipe(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(dir, GenerateInput{Kind: RecipeConsume, Name: "qt-test-1", Version: "1.0"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "goodluckbutton.pro"), []byte("TEMPLATE = lib\nTARGET = goodluckbutton\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := DetectPackageName(dir)
	if got.Name != "goodluckbutton" {
		t.Fatalf("got %#v", got)
	}
}

func TestDetectPackageNameIgnoresGeneratedPublishName(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(dir, GenerateInput{Kind: RecipePublish, Name: "qt-test-1", Version: "1.0", QtVersion: "5.14.2"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "goodluckbutton.pro"), []byte("TEMPLATE = lib\nTARGET = goodluckbutton\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := DetectPackageName(dir)
	if got.Name != "goodluckbutton" || got.Source != "qmake" {
		t.Fatalf("got %#v", got)
	}
}

func TestDetectPackageNameUsesHandwrittenRecipe(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "conanfile.py"), []byte("from conan import ConanFile\nclass X(ConanFile):\n    name = \"mylib\"\n    version = \"1.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := DetectPackageName(dir)
	if got.Name != "mylib" || got.Source != "recipe" {
		t.Fatalf("got %#v", got)
	}
}

func TestDetectPackageNameIncludeFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "include", "mylib"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := DetectPackageName(dir)
	if got.Name != "mylib" || got.Source != "include" {
		t.Fatalf("got %#v", got)
	}
}

func TestSanitizePkgName(t *testing.T) {
	if got := sanitizePkgName("GoodLuckButton"); got != "goodluckbutton" {
		t.Fatalf("got %q", got)
	}
	if got := sanitizePkgName("1abc"); got != "" {
		t.Fatalf("digit start %q", got)
	}
}
