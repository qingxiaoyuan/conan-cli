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
	path, err = Generate(dir, GenerateInput{Kind: RecipePublish, Name: "qtutils", Version: "1.0", QtVersion: "6.8"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, filepath.Join(".conan-cli", "recipes", "qtutils")) {
		t.Fatalf("publish recipe path = %s", path)
	}
	root, _ := os.ReadFile(filepath.Join(dir, "conanfile.py"))
	if !strings.Contains(string(root), "kind: consume") {
		t.Fatal("root consume recipe was overwritten by publish generate")
	}
	data, _ = os.ReadFile(path)
	text = string(data)
	if !strings.Contains(text, "kind: publish") || !strings.Contains(text, `"qt_version"`) || !strings.Contains(text, pythonClass("qtutils")+"Conan") || !strings.Contains(text, "_project_root") || !strings.Contains(text, "未找到已编译的库") || !strings.Contains(text, "def build(self):\n        pass") || !strings.Contains(text, "_lib_dirs = []") {
		t.Fatalf("publish recipe = %s", text)
	}
}

func TestGeneratePublishUsesConfiguredDirs(t *testing.T) {
	dir := t.TempDir()
	path, err := Generate(dir, GenerateInput{
		Kind: RecipePublish, Name: "qtutils", Version: "1.0", QtVersion: "6.8",
		LibDirs: []string{"build/Release", "src/mylib/lib"}, IncludeDirs: []string{"src/mylib"},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if !strings.Contains(text, `"build/Release"`) || !strings.Contains(text, `"src/mylib/lib"`) || !strings.Contains(text, `"src/mylib"`) || !strings.Contains(text, "if self._lib_dirs") {
		t.Fatalf("publish recipe missing configured dirs: %s", text)
	}
}

func TestGeneratePublishWithoutQt(t *testing.T) {
	dir := t.TempDir()
	path, err := Generate(dir, GenerateInput{Kind: RecipePublish, Name: "plainlib", Version: "1.0"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if strings.Contains(text, "qt_version") {
		t.Fatalf("non-Qt recipe should not mention qt_version: %s", text)
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
