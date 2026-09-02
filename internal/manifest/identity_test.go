package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchHandwrittenNameVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "conanfile.py")
	original := `from conan import ConanFile

class QtutilsConan(ConanFile):
    name = "qtutils"
    version = "1.0"
    def build(self):
        self.run("custom-build")
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyPublishIdentity(dir, PublishIdentity{Name: "qtutils", Version: "1.1", QtVersion: "6.8"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "patch" {
		t.Fatalf("action = %q", result.Action)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if !strings.Contains(text, `name = "qtutils"`) || !strings.Contains(text, `version = "1.1"`) {
		t.Fatalf("patched recipe = %s", text)
	}
	if !strings.Contains(text, `self.run("custom-build")`) {
		t.Fatal("expected handwritten build() to stay")
	}
}

func TestPatchInsertsMissingVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "conanfile.py")
	if err := os.WriteFile(path, []byte("from conan import ConanFile\nclass X(ConanFile):\n    name = \"old\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyPublishIdentity(dir, PublishIdentity{Name: "fresh", Version: "2.0"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if !strings.Contains(text, `name = "fresh"`) || !strings.Contains(text, `version = "2.0"`) {
		t.Fatalf("inserted recipe = %s", text)
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
	data, _ := os.ReadFile(filepath.Join(dir, "conanfile.py"))
	text := string(data)
	if !strings.Contains(text, "kind: publish") || !strings.Contains(text, `name = "qtutils"`) || !strings.Contains(text, `version = "1.0"`) {
		t.Fatalf("generated = %s", text)
	}
}

func TestApplyReplacesConsumeRecipe(t *testing.T) {
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
	data, _ := os.ReadFile(filepath.Join(dir, "conanfile.py"))
	text := string(data)
	if !strings.Contains(text, "kind: publish") || !strings.Contains(text, `version = "1.2"`) {
		t.Fatalf("replaced = %s", text)
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
	plan := PlanPublishIdentity(dir)
	if plan.Action != "generate" {
		t.Fatalf("missing file should generate, got %q", plan.Action)
	}
}
