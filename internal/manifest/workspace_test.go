package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeWorkspaceProject builds a qt-test-1 style layout:
// packages/<name>/{conanfile.py, dist/lib/lib<name>.a, dist/include/<name>/<name>.h}.
func writeWorkspaceProject(t *testing.T, dir string) {
	t.Helper()
	recipe := `from conan import ConanFile


class AlphaConan(ConanFile):
    name = "alphalib"
    version = "1.2.3"
    settings = "os", "compiler", "build_type", "arch"
`
	alpha := filepath.Join(dir, "packages", "alphalib")
	for _, sub := range []string{filepath.Join("dist", "lib"), filepath.Join("dist", "include", "alphalib")} {
		if err := os.MkdirAll(filepath.Join(alpha, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(alpha, "conanfile.py"), []byte(recipe), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alpha, "dist", "lib", "libalphalib.a"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alpha, "dist", "include", "alphalib", "alphalib.h"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// beta：无 dist，用 lib + include 布局，手写配方无 version 行。
	beta := filepath.Join(dir, "packages", "betalib")
	for _, sub := range []string{"lib", "include"} {
		if err := os.MkdirAll(filepath.Join(beta, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	betaRecipe := "from conan import ConanFile\nclass BetaConan(ConanFile):\n    name = \"betalib\"\n"
	if err := os.WriteFile(filepath.Join(beta, "conanfile.py"), []byte(betaRecipe), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beta, "lib", "libbetalib.so"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// gamma：只有 dist/ 产物，无配方，无版本。
	gamma := filepath.Join(dir, "packages", "gammalib")
	if err := os.MkdirAll(filepath.Join(gamma, "dist", "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gamma, "dist", "lib", "gammalib.dll"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// delta：空目录，不满足收录条件。
	if err := os.MkdirAll(filepath.Join(dir, "packages", "delta"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverWorkspacesFindsAndSorts(t *testing.T) {
	dir := t.TempDir()
	writeWorkspaceProject(t, dir)
	workspaces := DiscoverWorkspaces(dir, nil)
	if len(workspaces) != 3 {
		t.Fatalf("workspaces = %#v", workspaces)
	}
	if workspaces[0].Name != "alphalib" || workspaces[1].Name != "betalib" || workspaces[2].Name != "gammalib" {
		t.Fatalf("not sorted by name: %#v", workspaces)
	}
}

func TestDiscoverWorkspacesDefaultGlobsCoverSrc(t *testing.T) {
	dir := t.TempDir()
	writeWorkspaceProject(t, dir)
	srcLib := filepath.Join(dir, "src", "srclib")
	if err := os.MkdirAll(filepath.Join(srcLib, "dist", "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcLib, "dist", "lib", "libsrclib.a"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspaces := DiscoverWorkspaces(dir, nil)
	if len(workspaces) != 4 || workspaces[3].Name != "srclib" || workspaces[3].Dir != "src/srclib" {
		t.Fatalf("workspaces = %#v", workspaces)
	}
	if globs := DefaultWorkspaceGlobs(); len(globs) != 2 || globs[0] != "packages/*" || globs[1] != "src/*" {
		t.Fatalf("default globs = %#v", globs)
	}
}

func TestDiscoverWorkspacesReadsRecipeAndDirs(t *testing.T) {
	dir := t.TempDir()
	writeWorkspaceProject(t, dir)
	workspaces := DiscoverWorkspaces(dir, nil)
	alpha := workspaces[0]
	if alpha.Dir != "packages/alphalib" || !alpha.HasRecipe || alpha.Version != "1.2.3" {
		t.Fatalf("alpha = %#v", alpha)
	}
	if len(alpha.LibDirs) != 1 || alpha.LibDirs[0] != "packages/alphalib/dist/lib" {
		t.Fatalf("alpha lib dirs = %#v", alpha.LibDirs)
	}
	if len(alpha.IncludeDirs) != 1 || alpha.IncludeDirs[0] != "packages/alphalib/dist/include" {
		t.Fatalf("alpha include dirs = %#v", alpha.IncludeDirs)
	}
	if !alpha.HasArtifacts {
		t.Fatal("alpha should have artifacts")
	}

	beta := workspaces[1]
	if beta.Version != "" {
		t.Fatalf("beta version = %q, want empty (recipe has no version line)", beta.Version)
	}
	if len(beta.LibDirs) != 1 || beta.LibDirs[0] != "packages/betalib/lib" {
		t.Fatalf("beta lib dirs = %#v", beta.LibDirs)
	}
	if !beta.HasArtifacts {
		t.Fatal("beta should have artifacts via lib/")
	}

	gamma := workspaces[2]
	if gamma.HasRecipe || gamma.Version != "" {
		t.Fatalf("gamma = %#v", gamma)
	}
	if !gamma.HasArtifacts {
		t.Fatal("gamma should have artifacts via dist/lib")
	}
}

func TestDiscoverWorkspacesWithoutPackagesDir(t *testing.T) {
	dir := t.TempDir()
	if got := DiscoverWorkspaces(dir, nil); len(got) != 0 {
		t.Fatalf("workspaces = %#v, want empty", got)
	}
	if got := DiscoverWorkspaces(dir, []string{"components/*"}); len(got) != 0 {
		t.Fatalf("workspaces = %#v, want empty", got)
	}
}

func TestDiscoverWorkspacesCustomGlob(t *testing.T) {
	dir := t.TempDir()
	writeWorkspaceProject(t, dir)
	if got := DiscoverWorkspaces(dir, []string{"packages/alphalib"}); len(got) != 1 || got[0].Name != "alphalib" {
		t.Fatalf("workspaces = %#v", got)
	}
}

func TestDiscoverWorkspacesSkipsNonDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "packages"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "packages", "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DiscoverWorkspaces(dir, nil); len(got) != 0 {
		t.Fatalf("workspaces = %#v, want empty", got)
	}
}

func TestApplyPublishIdentityInPatchesWorkspaceRecipe(t *testing.T) {
	dir := t.TempDir()
	original := "from conan import ConanFile\nclass DemoConan(ConanFile):\n    name = \"demo\"\n    version = \"1.0\"\n    def build(self):\n        self.run(\"custom-build\")\n"
	if err := os.WriteFile(filepath.Join(dir, "conanfile.py"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyPublishIdentityIn(dir, PublishIdentity{Name: "demo", Version: "2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "patch" {
		t.Fatalf("action = %q, want patch", result.Action)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "conanfile.py"))
	if !strings.Contains(string(got), `version = "2.0"`) || !strings.Contains(string(got), `self.run("custom-build")`) {
		t.Fatalf("patched recipe = %s", got)
	}
}

func TestApplyPublishIdentityInMissingRecipe(t *testing.T) {
	if _, err := ApplyPublishIdentityIn(t.TempDir(), PublishIdentity{Name: "demo", Version: "1.0"}); err == nil {
		t.Fatal("expected error for missing conanfile.py")
	}
}
