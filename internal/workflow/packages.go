package workflow

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"conan-cli/internal/config"
	"conan-cli/internal/manifest"
)

// component is the merged publish view of one package: discovered workspace
// (npm-workspaces style) or a declared packages[] entry. Declared entries win
// on identity and artifact dirs; workspace fields (dir/has_recipe) survive the
// merge so publish can still target the workspace recipe.
type component struct {
	Name         string
	Version      string
	Source       string // "workspace" | "declared"
	Dir          string // 相对项目根；仅 workspace 组件非空
	LibDirs      []string
	IncludeDirs  []string
	QtVersion    string
	NoQt         bool
	HasArtifacts bool
	HasRecipe    bool
}

// IsWorkspace reports whether the component carries its own conanfile.py and
// can be export-pkg'd directly from its directory.
func (c component) IsWorkspace() bool {
	return c.Dir != "" && c.HasRecipe
}

// resolveComponents merges discovered workspaces with declared packages[].
// 同名时 declared 优先（version/dirs 以 packages[] 为准），workspace 的
// dir/has_recipe 信息保留。两者都为空时回退到以项目名合成的单组件。
func resolveComponents(projectDir string, project *config.Project) []component {
	byName := map[string]int{}
	var out []component
	for _, workspace := range manifest.DiscoverWorkspaces(projectDir, project.Workspaces) {
		if workspace.Name == "" {
			continue
		}
		byName[workspace.Name] = len(out)
		out = append(out, component{
			Name:         workspace.Name,
			Version:      workspace.Version,
			Source:       "workspace",
			Dir:          workspace.Dir,
			LibDirs:      workspace.LibDirs,
			IncludeDirs:  workspace.IncludeDirs,
			HasArtifacts: workspace.HasArtifacts,
			HasRecipe:    workspace.HasRecipe,
		})
	}
	for _, spec := range project.ListPackages() {
		if spec.Name == "" {
			continue
		}
		if index, ok := byName[spec.Name]; ok {
			merged := out[index]
			merged.Source = "declared"
			if spec.Version != "" {
				merged.Version = spec.Version
			}
			if len(spec.LibDirs) > 0 {
				merged.LibDirs = spec.LibDirs
			}
			if len(spec.IncludeDirs) > 0 {
				merged.IncludeDirs = spec.IncludeDirs
			}
			merged.QtVersion = spec.QtVersion
			merged.NoQt = spec.NoQt
			out[index] = merged
			continue
		}
		// 没有 workspace 时 packages[] 为空会合成项目名单组件；二者皆空才走到这。
		if len(project.Packages) == 0 && len(out) > 0 {
			continue
		}
		byName[spec.Name] = len(out)
		out = append(out, component{
			Name:         spec.Name,
			Version:      spec.Version,
			Source:       "declared",
			LibDirs:      spec.LibDirs,
			IncludeDirs:  spec.IncludeDirs,
			QtVersion:    spec.QtVersion,
			NoQt:         spec.NoQt,
			HasArtifacts: manifest.HasPrebuiltLibraries(projectDir, spec.LibDirs),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// PackageInfo is the JSON shape of one component in packages-list / status.
type PackageInfo struct {
	Name         string   `json:"name"`
	Version      string   `json:"version,omitempty"`
	Source       string   `json:"source"`
	Dir          string   `json:"dir,omitempty"`
	LibDirs      []string `json:"lib_dirs,omitempty"`
	IncludeDirs  []string `json:"include_dirs,omitempty"`
	NoQt         bool     `json:"no_qt,omitempty"`
	HasArtifacts bool     `json:"has_artifacts"`
	HasRecipe    bool     `json:"has_recipe"`
}

func (c component) info() PackageInfo {
	return PackageInfo{
		Name:         c.Name,
		Version:      c.Version,
		Source:       c.Source,
		Dir:          c.Dir,
		LibDirs:      c.LibDirs,
		IncludeDirs:  c.IncludeDirs,
		NoQt:         c.NoQt,
		HasArtifacts: c.HasArtifacts,
		HasRecipe:    c.HasRecipe,
	}
}

func componentInfos(components []component) []PackageInfo {
	infos := make([]PackageInfo, 0, len(components))
	for _, c := range components {
		infos = append(infos, c.info())
	}
	return infos
}

// PackagesList reports the merged component view (workspaces ∪ packages[]).
func (a *App) PackagesList(ctx context.Context) (Report, error) {
	_ = ctx
	project, err := a.Project()
	if err != nil {
		return Report{}, err
	}
	components := resolveComponents(a.Dir, project)
	infos := componentInfos(components)
	message := fmt.Sprintf("共 %d 个组件", len(infos))
	if len(infos) == 0 {
		message = "没有发现可发布的组件"
	}
	return Report{
		OK:      true,
		Action:  "packages-list",
		Message: message,
		Output:  packagesTable(infos),
		Data:    map[string]any{"packages": infos},
	}, nil
}

func packagesTable(infos []PackageInfo) string {
	if len(infos) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-24s %-10s %-10s %s", "名称", "版本", "来源", "产物")
	for _, info := range infos {
		artifacts := "缺失"
		if info.HasArtifacts {
			artifacts = "就绪"
		}
		version := info.Version
		if version == "" {
			version = "-"
		}
		fmt.Fprintf(&b, "\n%-24s %-10s %-10s %s", info.Name, version, info.Source, artifacts)
	}
	return b.String()
}
