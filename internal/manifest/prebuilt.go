package manifest

import (
	"os"
	"path/filepath"
	"strings"
)

var prebuiltExt = []string{".so", ".a", ".lib", ".dll", ".dylib"}

func HasPrebuiltLibraries(dir string) bool {
	var roots []string
	lib := filepath.Join(dir, "lib")
	bin := filepath.Join(dir, "bin")
	for _, root := range []string{lib, bin} {
		roots = append(roots, root)
		for _, sub := range []string{"Release", "Debug", "release", "debug"} {
			roots = append(roots, filepath.Join(root, sub))
		}
	}
	for _, root := range roots {
		if containsPrebuilt(root) {
			return true
		}
	}
	return false
}

func containsPrebuilt(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		lower := strings.ToLower(name)
		for _, ext := range prebuiltExt {
			if strings.HasSuffix(lower, ext) || strings.Contains(lower, ".so.") {
				return true
			}
		}
	}
	return false
}
