package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var ErrDynamicRequirements = errors.New("dynamic requirements() is not statically inspectable")

var dynamicRequirements = regexp.MustCompile(`(?m)^\s*def\s+requirements\s*\(`)
var staticRequires = regexp.MustCompile(`(?ms)^([ \t]+)requires\s*=\s*\((.*?)^([ \t]+)\)`)
var singleLineRequires = regexp.MustCompile(`(?m)^([ \t]+)requires\s*=\s*[\(\[](.*)[\)\]]\s*$`)
var scalarDoubleRequires = regexp.MustCompile(`(?m)^([ \t]+)requires\s*=\s*"([^"]+)"\s*$`)
var scalarSingleRequires = regexp.MustCompile(`(?m)^([ \t]+)requires\s*=\s*'([^']+)'\s*$`)
var quotedValue = regexp.MustCompile(`['"]([^'"]+)['"]`)
var classHeader = regexp.MustCompile(`(?m)^(class[^\n]*ConanFile[^\n]*:\s*\n)`)
var settingsAssign = regexp.MustCompile(`(?m)^([ \t]+)settings\s*=[^\n]*\n`)

// Add updates a simple Conan manifest or a static top-level requires assignment.
// Dynamic requirements() methods are intentionally rejected because text edits
// could silently change conditional dependency behavior.
func Add(dir, dependency string) (string, error) {
	dependency = strings.TrimSpace(dependency)
	if dependency == "" {
		return "", errors.New("package reference cannot be empty")
	}

	pyPath := filepath.Join(dir, "conanfile.py")
	txtPath := filepath.Join(dir, "conanfile.txt")
	if _, err := os.Stat(txtPath); err == nil {
		return addToTextManifest(txtPath, dependency)
	}
	if _, err := os.Stat(pyPath); err == nil {
		return addToPythonRecipe(pyPath, dependency)
	}
	content := defaultTextManifest(dependency, "")
	if err := writeAtomic(txtPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("create conanfile.txt: %w", err)
	}
	return txtPath, nil
}

func HasConanfile(dir string) bool {
	for _, name := range []string{"conanfile.py", "conanfile.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func EnsureText(dir, buildSystem string) (string, bool, error) {
	if HasConanfile(dir) {
		if _, err := os.Stat(filepath.Join(dir, "conanfile.txt")); err == nil {
			return filepath.Join(dir, "conanfile.txt"), false, nil
		}
		return filepath.Join(dir, "conanfile.py"), false, nil
	}
	path := filepath.Join(dir, "conanfile.txt")
	if err := writeAtomic(path, []byte(defaultTextManifest("", buildSystem)), 0o644); err != nil {
		return "", false, fmt.Errorf("create conanfile.txt: %w", err)
	}
	return path, true, nil
}

func defaultTextManifest(dependency, buildSystem string) string {
	requires := ""
	if strings.TrimSpace(dependency) != "" {
		requires = strings.TrimSpace(dependency) + "\n"
	}
	generators := "CMakeDeps\nCMakeToolchain\n"
	if strings.EqualFold(buildSystem, "qmake") {
		generators = "PkgConfigDeps\nVirtualRunEnv\n"
	}
	return "[requires]\n" + requires + "\n[generators]\n" + generators
}

// Dependencies reads the statically declared requirements from a Conan
// manifest. Dynamic Python recipes are valid Conan recipes, but cannot be
// compared safely with project.yaml without executing arbitrary recipe code.
func Dependencies(dir string) ([]string, error) {
	txtPath := filepath.Join(dir, "conanfile.txt")
	if _, err := os.Stat(txtPath); err == nil {
		data, err := os.ReadFile(txtPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", txtPath, err)
		}
		var dependencies []string
		inRequires := false
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.EqualFold(trimmed, "[requires]") {
				inRequires = true
				continue
			}
			if inRequires && strings.HasPrefix(trimmed, "[") {
				break
			}
			if inRequires && trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				dependencies = append(dependencies, trimmed)
			}
		}
		return dependencies, nil
	}

	pyPath := filepath.Join(dir, "conanfile.py")
	data, err := os.ReadFile(pyPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	return pythonDependencies(string(data))
}

func pythonDependencies(text string) ([]string, error) {
	if dynamicRequirements.MatchString(text) {
		return nil, ErrDynamicRequirements
	}
	block, ok := pythonRequiresBlock(text)
	if !ok {
		return []string{}, nil
	}
	return extractQuoted(block), nil
}

func pythonRequiresBlock(text string) (string, bool) {
	if match := staticRequires.FindStringIndex(text); match != nil {
		return text[match[0]:match[1]], true
	}
	if match := singleLineRequires.FindStringIndex(text); match != nil {
		return text[match[0]:match[1]], true
	}
	if match := scalarDoubleRequires.FindStringIndex(text); match != nil {
		return text[match[0]:match[1]], true
	}
	if match := scalarSingleRequires.FindStringIndex(text); match != nil {
		return text[match[0]:match[1]], true
	}
	return "", false
}

func extractQuoted(block string) []string {
	matches := quotedValue.FindAllStringSubmatch(block, -1)
	dependencies := make([]string, 0, len(matches))
	for _, match := range matches {
		dependencies = append(dependencies, match[1])
	}
	return dependencies
}

func addToTextManifest(path, dependency string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	text := string(data)
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == dependency {
			return path, nil
		}
	}

	lines := strings.Split(text, "\n")
	requiresIndex := -1
	insertIndex := len(lines)
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.EqualFold(trimmed, "[requires]") {
			requiresIndex = index
			continue
		}
		if requiresIndex >= 0 && strings.HasPrefix(trimmed, "[") {
			insertIndex = index
			break
		}
	}
	if requiresIndex < 0 {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += "\n[requires]\n" + dependency + "\n"
	} else {
		lines = append(lines, "")
		copy(lines[insertIndex+1:], lines[insertIndex:])
		lines[insertIndex] = dependency
		text = strings.Join(lines, "\n")
	}
	if err := writeAtomic(path, []byte(text), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

func addToPythonRecipe(path, dependency string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	text := string(data)
	if dynamicRequirements.MatchString(text) {
		return "", fmt.Errorf("%w; 请手工改 requirements()，避免破坏条件依赖", ErrDynamicRequirements)
	}
	if strings.Contains(text, `"`+dependency+`"`) || strings.Contains(text, `'`+dependency+`'`) {
		return path, nil
	}
	updated, err := insertPythonRequirement(text, dependency)
	if err != nil {
		return "", err
	}
	if err := writeAtomic(path, []byte(updated), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

func insertPythonRequirement(text, dependency string) (string, error) {
	if match := staticRequires.FindStringSubmatchIndex(text); match != nil {
		indent := text[match[2]:match[3]]
		closing := match[5]
		return text[:closing] + indent + "    \"" + dependency + "\",\n" + text[closing:], nil
	}
	if match := singleLineRequires.FindStringSubmatchIndex(text); match != nil {
		indent := text[match[2]:match[3]]
		existing := extractQuoted(text[match[0]:match[1]])
		return text[:match[0]] + formatRequires(indent, append(existing, dependency)) + text[match[1]:], nil
	}
	if match := scalarDoubleRequires.FindStringSubmatchIndex(text); match != nil {
		return replaceScalarRequires(text, match, dependency), nil
	}
	if match := scalarSingleRequires.FindStringSubmatchIndex(text); match != nil {
		return replaceScalarRequires(text, match, dependency), nil
	}
	inserted, ok := insertRequiresAssignment(text, dependency)
	if !ok {
		return "", errors.New("conanfile.py 没有 ConanFile 类，无法自动加入依赖")
	}
	return inserted, nil
}

func replaceScalarRequires(text string, match []int, dependency string) string {
	indent := text[match[2]:match[3]]
	old := text[match[4]:match[5]]
	return text[:match[0]] + formatRequires(indent, []string{old, dependency}) + text[match[1]:]
}

func formatRequires(indent string, deps []string) string {
	var b strings.Builder
	b.WriteString(indent)
	b.WriteString("requires = (\n")
	for _, dep := range deps {
		b.WriteString(indent)
		b.WriteString("    \"")
		b.WriteString(dep)
		b.WriteString("\",\n")
	}
	b.WriteString(indent)
	b.WriteString(")")
	return b.String()
}

func insertRequiresAssignment(text, dependency string) (string, bool) {
	block := formatRequires("    ", []string{dependency}) + "\n"
	if loc := settingsAssign.FindStringIndex(text); loc != nil {
		return text[:loc[1]] + block + text[loc[1]:], true
	}
	if loc := classHeader.FindStringIndex(text); loc != nil {
		return text[:loc[1]] + block + text[loc[1]:], true
	}
	return text, false
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".conanfile.*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}
