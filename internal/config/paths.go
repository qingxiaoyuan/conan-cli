package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SplitPathList splits a user-facing comma / semicolon / newline list of
// relative directories.
func SplitPathList(value string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	}) {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func JoinPathList(values []string) string {
	return strings.Join(values, ", ")
}

// NormalizeRelPath requires a path relative to the project root that cannot
// escape it with "..".
func NormalizeRelPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("path is empty")
	}
	cleaned := filepath.ToSlash(filepath.Clean(value))
	cleaned = strings.TrimPrefix(cleaned, "./")
	if cleaned == "" || cleaned == "." {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(value) || strings.HasPrefix(cleaned, "/") || (len(cleaned) >= 2 && cleaned[1] == ':') {
		return "", fmt.Errorf("path must be relative to the project root: %s", value)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path escapes the project root: %s", value)
	}
	return cleaned, nil
}

func NormalizeRelPaths(values []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		path, err := NormalizeRelPath(value)
		if err != nil {
			return nil, err
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	return out, nil
}

// NormalizeWorkspaceGlob validates a workspace glob: relative to the project
// root, never absolute and never escaping with "..". Unlike NormalizeRelPath
// it allows "*" wildcard segments.
func NormalizeWorkspaceGlob(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(value) {
		return "", fmt.Errorf("path must be relative to the project root: %s", value)
	}
	cleaned := filepath.ToSlash(filepath.Clean(value))
	cleaned = strings.TrimPrefix(cleaned, "./")
	if cleaned == "" || cleaned == "." {
		return "", fmt.Errorf("path is empty")
	}
	if strings.HasPrefix(cleaned, "/") || (len(cleaned) >= 2 && cleaned[1] == ':') {
		return "", fmt.Errorf("path must be relative to the project root: %s", value)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path escapes the project root: %s", value)
	}
	return cleaned, nil
}

func NormalizeWorkspaceGlobs(values []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		glob, err := NormalizeWorkspaceGlob(value)
		if err != nil {
			return nil, err
		}
		if seen[glob] {
			continue
		}
		seen[glob] = true
		out = append(out, glob)
	}
	return out, nil
}

func normalizePackageSpec(spec *PackageSpec) error {
	if spec == nil {
		return nil
	}
	spec.Name = strings.TrimSpace(spec.Name)
	spec.Version = strings.TrimSpace(spec.Version)
	libDirs, err := NormalizeRelPaths(spec.LibDirs)
	if err != nil {
		return fmt.Errorf("lib_dirs: %w", err)
	}
	includeDirs, err := NormalizeRelPaths(spec.IncludeDirs)
	if err != nil {
		return fmt.Errorf("include_dirs: %w", err)
	}
	spec.LibDirs = libDirs
	spec.IncludeDirs = includeDirs
	return nil
}

// PrimaryPackage is the component used for publish in this phase (the first
// packages[] entry, or a synthetic spec from the top-level name).
func (p *Project) PrimaryPackage() PackageSpec {
	if p == nil {
		return PackageSpec{}
	}
	if len(p.Packages) > 0 {
		spec := p.Packages[0]
		if spec.Name == "" {
			spec.Name = p.Name
		}
		return spec
	}
	return PackageSpec{Name: p.Name}
}

// SetPrimaryArtifactDirs writes lib/include dirs onto packages[0], creating
// the slice if needed. Empty lists omit the fields (defaults apply at publish).
func (p *Project) SetPrimaryArtifactDirs(libDirs, includeDirs []string) error {
	if p == nil {
		return fmt.Errorf("project config is nil")
	}
	var err error
	if libDirs, err = NormalizeRelPaths(libDirs); err != nil {
		return fmt.Errorf("lib_dirs: %w", err)
	}
	if includeDirs, err = NormalizeRelPaths(includeDirs); err != nil {
		return fmt.Errorf("include_dirs: %w", err)
	}
	spec := PackageSpec{}
	if len(p.Packages) > 0 {
		spec = p.Packages[0]
	}
	spec.LibDirs = libDirs
	spec.IncludeDirs = includeDirs
	if spec.Name == "" && spec.Version == "" && len(spec.LibDirs) == 0 && len(spec.IncludeDirs) == 0 {
		p.Packages = nil
		return nil
	}
	if len(p.Packages) == 0 {
		p.Packages = []PackageSpec{spec}
		return nil
	}
	p.Packages[0] = spec
	return nil
}

// ListPackages returns configured components. With an empty packages list the
// top-level project name is presented as a single synthetic component so old
// projects keep working.
func (p *Project) ListPackages() []PackageSpec {
	if p == nil {
		return nil
	}
	if len(p.Packages) > 0 {
		out := make([]PackageSpec, len(p.Packages))
		copy(out, p.Packages)
		for i := range out {
			if out[i].Name == "" {
				out[i].Name = p.Name
			}
		}
		return out
	}
	if strings.TrimSpace(p.Name) == "" {
		return nil
	}
	return []PackageSpec{{Name: p.Name}}
}

// FindPackage looks up a component by Conan package name. An empty id matches
// only when there is exactly one component.
func (p *Project) FindPackage(id string) (PackageSpec, int, bool) {
	if p == nil {
		return PackageSpec{}, -1, false
	}
	id = strings.TrimSpace(id)
	listed := p.ListPackages()
	if id == "" {
		if len(listed) == 1 {
			index := 0
			if len(p.Packages) == 0 {
				index = -1
			}
			return listed[0], index, true
		}
		return PackageSpec{}, -1, false
	}
	if len(p.Packages) == 0 && p.Name == id {
		return PackageSpec{Name: p.Name}, -1, true
	}
	for index, spec := range p.Packages {
		name := spec.Name
		if name == "" {
			name = p.Name
		}
		if name == id {
			if spec.Name == "" {
				spec.Name = p.Name
			}
			return spec, index, true
		}
	}
	return PackageSpec{}, -1, false
}

// UpsertPackage inserts or replaces a component by name.
func (p *Project) UpsertPackage(spec PackageSpec) error {
	if p == nil {
		return fmt.Errorf("project config is nil")
	}
	if err := normalizePackageSpec(&spec); err != nil {
		return err
	}
	if spec.Name == "" {
		return fmt.Errorf("package name is empty")
	}
	if len(p.Packages) == 0 {
		p.Packages = []PackageSpec{spec}
		return nil
	}
	for index, existing := range p.Packages {
		name := existing.Name
		if name == "" {
			name = p.Name
		}
		if name == spec.Name {
			p.Packages[index] = spec
			return nil
		}
	}
	p.Packages = append(p.Packages, spec)
	return nil
}
