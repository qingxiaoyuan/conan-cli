package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyDoesNotTouchRootRecipe(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "conanfile.py")
	original := `from conan import ConanFile

class QtutilsConan(ConanFile):
    name = "app"
    version = "0.1"
    def build(self):
        self.run("custom-build")
`
	if err := os.WriteFile(root, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyPublishIdentity(dir, PublishIdentity{Name: "qtutils", Version: "1.1", QtVersion: "6.8"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "generate" {
		t.Fatalf("action = %q", result.Action)
	}
	if !strings.Contains(result.Path, filepath.Join(".conan-cli", "recipes", "qtutils")) {
		t.Fatalf("path = %s", result.Path)
	}
	got, _ := os.ReadFile(root)
	if string(got) != original {
		t.Fatal("root conanfile.py was modified")
	}
	data, _ := os.ReadFile(result.Path)
	text := string(data)
	if !strings.Contains(text, `name = "qtutils"`) || !strings.Contains(text, `version = "1.1"`) || !strings.Contains(text, "_project_root") {
		t.Fatalf("isolated recipe = %s", text)
	}
}

func TestApplyPatchesHandwrittenIsolatedRecipe(t *testing.T) {
	dir := t.TempDir()
	path := PublishRecipePath(dir, "fresh")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("from conan import ConanFile\nclass X(ConanFile):\n    name = \"old\"\n    def build(self):\n        self.run(\"keep\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyPublishIdentity(dir, PublishIdentity{Name: "fresh", Version: "2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "patch" {
		t.Fatalf("action = %q", result.Action)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if !strings.Contains(text, `name = "fresh"`) || !strings.Contains(text, `version = "2.0"`) || !strings.Contains(text, `self.run("keep")`) {
		t.Fatalf("patched = %s", text)
	}
}

func TestApplyGeneratesWhenMissing(t *testing.T) {
	dir := t.TempDir()
	result, err := ApplyPublishIdentity(dir, PublishIdentity{Name: "qtutils", Version: "1.0", QtVersion: "6.8"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "generate" {
		t.Fatalf("action = %q", result.Action)
	}
	data, _ := os.ReadFile(result.Path)
	text := string(data)
	if !strings.Contains(text, "kind: publish") || !strings.Contains(text, `name = "qtutils"`) || !strings.Contains(text, `version = "1.0"`) {
		t.Fatalf("generated = %s", text)
	}
	if _, err := os.Stat(filepath.Join(dir, "conanfile.py")); !os.IsNotExist(err) {
		t.Fatal("did not expect a root conanfile.py")
	}
}

func TestApplyLeavesConsumeRecipe(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(dir, GenerateInput{Kind: RecipeConsume, Name: "app", Version: "0.1"}); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyPublishIdentity(dir, PublishIdentity{Name: "qtutils", Version: "1.2", QtVersion: "6.8"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "generate" {
		t.Fatalf("action = %q", result.Action)
	}
	root, _ := os.ReadFile(filepath.Join(dir, "conanfile.py"))
	if !strings.Contains(string(root), "kind: consume") {
		t.Fatal("consume recipe replaced")
	}
	data, _ := os.ReadFile(result.Path)
	if !strings.Contains(string(data), "kind: publish") || !strings.Contains(string(data), `version = "1.2"`) {
		t.Fatalf("publish recipe = %s", data)
	}
}

func TestPatchQtDefaultAndOptions(t *testing.T) {
	text := `class X(ConanFile):
    name = "lib"
    version = "1.0"
    options = {"qt_version": ["6.5", "6.8"]}
    default_options = {"qt_version": "6.5"}
`
	got, err := patchIdentityText(text, PublishIdentity{Name: "lib", Version: "1.0", QtVersion: "6.9"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"qt_version": "6.9"`) {
		t.Fatalf("default not updated: %s", got)
	}
	if !strings.Contains(got, `"6.9", "6.5", "6.8"`) && !strings.Contains(got, `"qt_version": ["6.9"`) {
		t.Fatalf("option list not updated: %s", got)
	}
}

func TestApplyRefreshesGeneratedPublish(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(dir, GenerateInput{Kind: RecipePublish, Name: "qt-test-1", Version: "0.1", QtVersion: "6.5"}); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyPublishIdentity(dir, PublishIdentity{Name: "qt-test-1", Version: "1.0.0", QtVersion: "5.14.2"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "update" {
		t.Fatalf("action = %q", result.Action)
	}
	data, _ := os.ReadFile(result.Path)
	text := string(data)
	if !strings.Contains(text, `name = "qt-test-1"`) || !strings.Contains(text, `version = "1.0.0"`) || !strings.Contains(text, "_project_root") {
		t.Fatalf("refreshed recipe = %s", text)
	}
}

func TestValidateIdentity(t *testing.T) {
	if _, err := ApplyPublishIdentity(t.TempDir(), PublishIdentity{Name: "bad/name", Version: "1"}); err == nil {
		t.Fatal("expected invalid name")
	}
	if _, err := ApplyPublishIdentity(t.TempDir(), PublishIdentity{Name: "ok", Version: ""}); err == nil {
		t.Fatal("expected missing version")
	}
}

func TestDryRunPlanDoesNotNeedFile(t *testing.T) {
	dir := t.TempDir()
	plan := PlanPublishIdentity(dir, "demo")
	if plan.Action != "generate" {
		t.Fatalf("missing file should generate, got %q", plan.Action)
	}
}
