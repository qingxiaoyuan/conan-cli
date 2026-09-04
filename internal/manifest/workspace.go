package manifest

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// recipeVersionLine reads the static version assignment from a conanfile.py
// without invoking the conan process.
var recipeVersionLine = regexp.MustCompile(`(?m)^[ \t]+version\s*=\s*['"]([^'"]+)['"]`)

// Workspace is a publishable component discovered npm-workspaces style: a
// directory (by default packages/* and src/*) with its own conanfile.py or a
// dist/lib artifact layout.
type Workspace struct {
	Dir          string   `json:"dir"` // 相对项目根，如 packages/luckylabel
	Name         string   `json:"name"`
	Version      string   `json:"version,omitempty"`
	HasRecipe    bool     `json:"has_recipe"`
	LibDirs      []string `json:"lib_dirs,omitempty"` // 相对项目根
	IncludeDirs  []string `json:"include_dirs,omitempty"`
	HasArtifacts bool     `json:"has_artifacts"`
}

// DefaultWorkspaceGlobs is used when the project does not configure
// workspaces: every direct child of packages/ and src/ is a candidate
// component.
func DefaultWorkspaceGlobs() []string {
	return []string{"packages/*", "src/*"}
}

// DiscoverWorkspaces expands the workspace globs (relative to projectDir) and
// inspects each matched directory. A directory becomes a workspace when it has
// a conanfile.py, a dist/ directory, or a lib/ directory. Pure lookup: no
// writes, no conan process.
func DiscoverWorkspaces(projectDir string, globs []string) []Workspace {
	if len(globs) == 0 {
		globs = DefaultWorkspaceGlobs()
	}
	seen := map[string]bool{}
	var out []Workspace
	for _, glob := range globs {
		glob = strings.TrimSpace(glob)
		if glob == "" {
			continue
		}
		matches, err := filepath.Glob(filepath.Join(projectDir, filepath.FromSlash(glob)))
		if err != nil {
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}
			rel, err := filepath.Rel(projectDir, match)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			if seen[rel] {
				continue
			}
			workspace, ok := inspectWorkspace(projectDir, rel)
			if !ok {
				continue
			}
			seen[rel] = true
			out = append(out, workspace)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func inspectWorkspace(projectDir, rel string) (Workspace, bool) {
	abs := filepath.Join(projectDir, filepath.FromSlash(rel))
	hasRecipe := fileExists(abs, "conanfile.py")
	if !hasRecipe && !isSubDir(abs, "dist") && !isSubDir(abs, "lib") {
		return Workspace{}, false
	}
	workspace := Workspace{Dir: rel, HasRecipe: hasRecipe}
	workspace.Name = DetectPackageName(abs).Name
	if workspace.Name == "" {
		workspace.Name = filepath.Base(rel)
	}
	if hasRecipe {
		workspace.Version = workspaceRecipeVersion(abs)
	}
	if isSubDir(abs, filepath.Join("dist", "lib")) {
		workspace.LibDirs = []string{rel + "/dist/lib"}
	} else if isSubDir(abs, "lib") {
		workspace.LibDirs = []string{rel + "/lib"}
	}
	if isSubDir(abs, filepath.Join("dist", "include")) {
		workspace.IncludeDirs = []string{rel + "/dist/include"}
	} else if isSubDir(abs, "include") {
		workspace.IncludeDirs = []string{rel + "/include"}
	}
	if len(workspace.LibDirs) > 0 {
		workspace.HasArtifacts = HasPrebuiltLibraries(projectDir, workspace.LibDirs)
	} else {
		workspace.HasArtifacts = HasPrebuiltLibraries(abs)
	}
	return workspace, true
}

func workspaceRecipeVersion(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "conanfile.py"))
	if err != nil {
		return ""
	}
	m := recipeVersionLine.FindSubmatch(data)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(string(m[1]))
}

func fileExists(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name))
	return err == nil && !info.IsDir()
}

func isSubDir(dir, rel string) bool {
	info, err := os.Stat(filepath.Join(dir, rel))
	return err == nil && info.IsDir()
}
