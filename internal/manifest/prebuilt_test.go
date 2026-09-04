package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasPrebuiltLibraries(t *testing.T) {
	dir := t.TempDir()
	if HasPrebuiltLibraries(dir) {
		t.Fatal("empty dir should have no libs")
	}
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lib", "libgoodluckbutton.a"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasPrebuiltLibraries(dir) {
		t.Fatal("expected lib/*.a")
	}
}

func TestHasPrebuiltLibrariesCustomDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "build", "Release"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "build", "Release", "libfoo.so"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if HasPrebuiltLibraries(dir) {
		t.Fatal("default scan should not see build/Release")
	}
	if !HasPrebuiltLibraries(dir, []string{"build/Release"}) {
		t.Fatal("expected build/Release/*.so")
	}
}
