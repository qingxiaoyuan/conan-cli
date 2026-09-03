package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateConsumeAndPublish(t *testing.T) {
	dir := t.TempDir()
	path, err := Generate(dir, GenerateInput{
		Kind: RecipeConsume, Name: "conan-demo", Version: "1.0",
		BuildSystem: "qmake", QtVersion: "6.8", Requires: []string{"qtutils/1.0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if !strings.Contains(text, "kind: consume") || !strings.Contains(text, `"qtutils/1.0"`) || !strings.Contains(text, `*:qt_version`) {
		t.Fatalf("consume recipe = %s", text)
	}
	if _, err := Generate(dir, GenerateInput{Kind: RecipePublish, Name: "qtutils", Version: "1.0", QtVersion: "6.8"}); err == nil {
		t.Fatal("expected refuse overwrite of generated consume recipe without force")
	}
	path, err = Generate(dir, GenerateInput{Kind: RecipePublish, Name: "qtutils", Version: "1.0", QtVersion: "6.8", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	text = string(data)
	if !strings.Contains(text, "kind: publish") || !strings.Contains(text, `"qt_version"`) || !strings.Contains(text, pythonClass("qtutils")+"Conan") || !strings.Contains(text, "def export_sources") || !strings.Contains(text, "未找到已编译的库") || !strings.Contains(text, "def build(self):\n        pass") {
		t.Fatalf("publish recipe = %s", text)
	}
	if _, err := os.Stat(filepath.Join(dir, "conanfile.txt")); !os.IsNotExist(err) {
		t.Fatal("expected conanfile.txt to be removed")
	}
}

func TestGenerateRefusesHandWritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "conanfile.py")
	if err := os.WriteFile(path, []byte("from conan import ConanFile\nclass X(ConanFile):\n    requires = ()\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(dir, GenerateInput{Kind: RecipeConsume, Name: "x"}); err == nil {
		t.Fatal("expected refuse handwritten recipe")
	}
}

func TestPythonClass(t *testing.T) {
	if got := pythonClass("conan-demo"); got != "ConanDemo" {
		t.Fatalf("got %q", got)
	}
}
