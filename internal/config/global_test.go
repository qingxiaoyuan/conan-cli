package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadGlobalAndPassword(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CONAN_CLI_HOME", home)
	global := &Global{Nexus: Nexus{Name: "nexus", URL: "https://example.test/conan", Username: "alice"}}
	if err := SaveGlobal(global); err != nil {
		t.Fatal(err)
	}
	if err := SavePassword("s3cret"); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Nexus.Username != "alice" {
		t.Fatalf("username = %q", loaded.Nexus.Username)
	}
	password, err := LoadPassword()
	if err != nil {
		t.Fatal(err)
	}
	if password != "s3cret" {
		t.Fatalf("password = %q", password)
	}
	info, err := os.Stat(filepath.Join(home, "credentials"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credentials mode = %v", info.Mode().Perm())
	}
	view := loaded.View()
	if !view.HasPassword {
		t.Fatal("expected has_password")
	}
}
