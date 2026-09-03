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
