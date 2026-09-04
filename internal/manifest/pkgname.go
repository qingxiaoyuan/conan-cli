package manifest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

type PackageName struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

var (
	qmakeAssign     = regexp.MustCompile(`(?i)^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)$`)
	cmakeProject    = regexp.MustCompile(`(?im)^\s*project\s*\(\s*([A-Za-z0-9_.+-]+)`)
	cmakeAddLibrary = regexp.MustCompile(`(?im)^\s*add_library\s*\(\s*([A-Za-z0-9_.+-]+)`)
	recipeNameLine  = regexp.MustCompile(`(?m)^[ \t]+name\s*=\s*['"]([^'"]+)['"]`)
)

var skipDirNames = map[string]bool{
	"example": true, "examples": true, "test": true, "tests": true,
	"test_package": true, "build": true, ".git": true, ".conan-cli": true,
}

func DetectPackageNames(dir string) []PackageName {
	var out []PackageName
	seen := map[string]bool{}
	add := func(name, source string) {
		name = sanitizePkgName(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, PackageName{Name: name, Source: source})
	}
	for _, name := range qmakeLibTargets(dir) {
		add(name, "qmake")
	}
	for _, name := range cmakeLibNames(dir) {
		add(name, "cmake")
	}
	if len(out) == 0 {
		if guess := DetectPackageName(dir); guess.Name != "" {
			out = append(out, guess)
		}
	}
	return out
}

func DetectPackageName(dir string) PackageName {
	if name := recipePackageName(dir); name != "" {
		return PackageName{Name: name, Source: "recipe"}
	}
	if names := qmakeLibTargets(dir); len(names) == 1 {
		return PackageName{Name: names[0], Source: "qmake"}
	} else if len(names) > 1 {
		if inc := includeDirName(dir); inc != "" {
			for _, name := range names {
				if name == inc {
					return PackageName{Name: name, Source: "qmake"}
				}
			}
		}
	}
	if names := cmakeLibNames(dir); len(names) == 1 {
		return PackageName{Name: names[0], Source: "cmake"}
	} else if len(names) > 1 {
		if inc := includeDirName(dir); inc != "" {
			for _, name := range names {
				if name == inc {
					return PackageName{Name: name, Source: "cmake"}
				}
			}
		}
	}
	if name := cmakeProjectName(dir); name != "" {
		return PackageName{Name: name, Source: "cmake"}
	}
	if name := includeDirName(dir); name != "" {
		return PackageName{Name: name, Source: "include"}
	}
	return PackageName{Name: sanitizePkgName(filepath.Base(dir)), Source: "directory"}
}

func recipePackageName(dir string) string {
	if GeneratedKind(dir) != "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, "conanfile.py"))
	if err != nil {
		return ""
	}
	m := recipeNameLine.FindSubmatch(data)
	if m == nil {
		return ""
	}
	return sanitizePkgName(string(m[1]))
}

func qmakeLibTargets(dir string) []string {
	var names []string
	seen := map[string]bool{}
	add := func(name string) {
		name = sanitizePkgName(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	for _, path := range proFiles(dir) {
		tmpl, target := parseProIdentity(path)
		if tmpl == "lib" && target != "" {
			add(target)
		}
	}
	return names
}

func proFiles(dir string) []string {
	var paths []string
	matches, _ := filepath.Glob(filepath.Join(dir, "*.pro"))
	paths = append(paths, matches...)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return paths
	}
	for _, entry := range entries {
		if !entry.IsDir() || skipDirNames[strings.ToLower(entry.Name())] {
			continue
		}
		nested, _ := filepath.Glob(filepath.Join(dir, entry.Name(), "*.pro"))
		paths = append(paths, nested...)
	}
	return paths
}

func parseProIdentity(path string) (template, target string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		m := qmakeAssign.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key, value := strings.ToUpper(m[1]), strings.TrimSpace(m[2])
		if strings.ContainsAny(value, "$") {
			continue
		}
		switch key {
		case "TEMPLATE":
			template = strings.ToLower(fieldsFirst(value))
		case "TARGET":
			target = fieldsFirst(value)
		}
	}
	return template, target
}

func cmakeLibNames(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, "CMakeLists.txt"))
	if err != nil {
		return nil
	}
	var names []string
	seen := map[string]bool{}
	for _, m := range cmakeAddLibrary.FindAllSubmatch(data, -1) {
		name := sanitizePkgName(string(m[1]))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func cmakeProjectName(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "CMakeLists.txt"))
	if err != nil {
		return ""
	}
	m := cmakeProject.FindSubmatch(data)
	if m == nil {
		return ""
	}
	return sanitizePkgName(string(m[1]))
}

func includeDirName(dir string) string {
	entries, err := os.ReadDir(filepath.Join(dir, "include"))
	if err != nil {
		return ""
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if name := sanitizePkgName(entry.Name()); name != "" {
			names = append(names, name)
		}
	}
	if len(names) != 1 {
		return ""
	}
	return names[0]
}

func fieldsFirst(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func sanitizePkgName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '+' || r == '.' || r == '-':
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), ".-+")
	if len(out) < 2 {
		return ""
	}
	if unicode.IsDigit([]rune(out)[0]) {
		return ""
	}
	return out
}
