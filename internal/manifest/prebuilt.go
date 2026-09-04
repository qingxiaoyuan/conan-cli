package manifest

import (
	"os"
	"path/filepath"
	"strings"
)

var prebuiltExt = []string{".so", ".a", ".lib", ".dll", ".dylib"}

// DefaultLibDirs is the root-relative layout used when the project does not
// configure packages[].lib_dirs. Matches the historical lib/ + bin/ scan.
func DefaultLibDirs() []string {
	return []string{
		"lib",
		"lib/Release", "lib/Debug", "lib/release", "lib/debug",
		"bin",
		"bin/Release", "bin/Debug", "bin/release", "bin/debug",
	}
}

func HasPrebuiltLibraries(dir string, libDirs ...[]string) bool {
	dirs := DefaultLibDirs()
	if len(libDirs) > 0 && len(libDirs[0]) > 0 {
		dirs = libDirs[0]
	}
	for _, rel := range dirs {
		if containsPrebuilt(filepath.Join(dir, filepath.FromSlash(rel))) {
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
