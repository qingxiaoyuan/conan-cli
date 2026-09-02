package platform

import (
	"strings"
	"testing"

	"conan-cli/internal/config"
)

func TestResolveKylin(t *testing.T) {
	settings := Resolve(config.PlatformSpec{OS: "麒麟", Arch: "x64"}, config.Compiler{ID: "gcc", Version: "11"}, "6.8")
	if settings.OS != config.OSKylin || settings.ConanOS != "Linux" || settings.ConanArch != "x86_64" {
		t.Fatalf("settings = %#v", settings)
	}
	args := settings.Args()
	joined := ""
	for _, arg := range args {
		joined += arg + " "
	}
	if !containsAll(joined, "os=Linux", "arch=x86_64", "compiler=gcc", "compiler.version=11", "build_type=Release", "*:qt_version=6.8") {
		t.Fatalf("args = %v", args)
	}
}

func TestResolveDebugBuildType(t *testing.T) {
	settings := Resolve(config.PlatformSpec{OS: "linux", Arch: "x64", BuildType: "debug"}, config.Compiler{}, "")
	if settings.BuildType != config.BuildTypeDebug {
		t.Fatalf("build type = %q", settings.BuildType)
	}
	joined := strings.Join(settings.Args(), " ")
	if !strings.Contains(joined, "build_type=Debug") {
		t.Fatalf("args = %v", settings.Args())
	}
}

func TestNormalizeWindowsARM(t *testing.T) {
	settings := Resolve(config.PlatformSpec{OS: "windows", Arch: "arm64"}, config.Compiler{}, "")
	if settings.ConanOS != "Windows" || settings.ConanArch != "armv8" {
		t.Fatalf("arm64 settings = %#v", settings)
	}
	settings = Resolve(config.PlatformSpec{OS: "linux", Arch: "arm"}, config.Compiler{}, "")
	if settings.ConanOS != "Linux" || settings.ConanArch != "armv7" {
		t.Fatalf("arm32 settings = %#v", settings)
	}
}

func containsAll(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if !contains(haystack, needle) {
			return false
		}
	}
	return true
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(needle) == 0 || (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})())
}
