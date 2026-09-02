package platform

import (
	"os"
	"runtime"
	"strings"

	"conan-cli/internal/config"
)

type Settings struct {
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	ConanOS         string `json:"conan_os"`
	ConanArch       string `json:"conan_arch"`
	Compiler        string `json:"compiler,omitempty"`
	CompilerVersion string `json:"compiler_version,omitempty"`
	BuildType       string `json:"build_type,omitempty"`
	QtVersion       string `json:"qt_version,omitempty"`
	Distro          string `json:"distro,omitempty"`
	Note            string `json:"note,omitempty"`
}

func DetectHost() config.PlatformSpec {
	spec := config.PlatformSpec{OS: config.OSLinux, Arch: config.ArchX64}
	switch runtime.GOOS {
	case "windows":
		spec.OS = config.OSWindows
	case "darwin":
		spec.OS = config.OSLinux
	default:
		spec.OS = config.OSLinux
		if isKylin() {
			spec.OS = config.OSKylin
		}
	}
	switch runtime.GOARCH {
	case "386":
		spec.Arch = config.ArchX86
	case "arm":
		spec.Arch = config.ArchARM
	case "arm64":
		spec.Arch = config.ArchARM64
	default:
		spec.Arch = config.ArchX64
	}
	return spec
}

func isKylin() bool {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return false
	}
	text := strings.ToLower(string(data))
	return strings.Contains(text, "kylin") || strings.Contains(text, "麒麟") || strings.Contains(text, "neokylin")
}

func Resolve(spec config.PlatformSpec, compiler config.Compiler, qtVersion string) Settings {
	spec.OS = config.NormalizeOS(spec.OS)
	spec.Arch = config.NormalizeArch(spec.Arch)
	settings := Settings{
		OS:              spec.OS,
		Arch:            spec.Arch,
		Compiler:        strings.TrimSpace(compiler.ID),
		CompilerVersion: strings.TrimSpace(compiler.Version),
		BuildType:       config.NormalizeBuildType(spec.BuildType),
		QtVersion:       strings.TrimSpace(qtVersion),
	}
	if settings.BuildType == "" {
		settings.BuildType = config.BuildTypeRelease
	}
	switch spec.OS {
	case config.OSWindows:
		settings.ConanOS = "Windows"
	case config.OSKylin:
		settings.ConanOS = "Linux"
		settings.Distro = "kylin"
		settings.Note = "麒麟按 Linux ABI 查询，并优先匹配发行版信息"
	default:
		if spec.OS != "" {
			settings.ConanOS = "Linux"
		}
	}
	switch spec.Arch {
	case config.ArchX86:
		settings.ConanArch = "x86"
	case config.ArchX64:
		settings.ConanArch = "x86_64"
	case config.ArchARM:
		settings.ConanArch = "armv7"
	case config.ArchARM64:
		settings.ConanArch = "armv8"
	}
	return settings
}

func (s Settings) Args() []string {
	var args []string
	if s.ConanOS != "" {
		args = append(args, "-s", "os="+s.ConanOS)
	}
	if s.ConanArch != "" {
		args = append(args, "-s", "arch="+s.ConanArch)
	}
	if s.Compiler != "" {
		args = append(args, "-s", "compiler="+s.Compiler)
	}
	if s.CompilerVersion != "" {
		args = append(args, "-s", "compiler.version="+s.CompilerVersion)
	}
	if s.BuildType != "" {
		args = append(args, "-s", "build_type="+s.BuildType)
	}
	if s.QtVersion != "" {
		args = append(args, "-o", "*:qt_version="+s.QtVersion)
	}
	return args
}

func (s Settings) Map() map[string]string {
	out := map[string]string{}
	if s.ConanOS != "" {
		out["os"] = s.ConanOS
	}
	if s.ConanArch != "" {
		out["arch"] = s.ConanArch
	}
	if s.Compiler != "" {
		out["compiler"] = s.Compiler
	}
	if s.CompilerVersion != "" {
		out["compiler.version"] = s.CompilerVersion
	}
	if s.BuildType != "" {
		out["build_type"] = s.BuildType
	}
	if s.Distro != "" {
		out["distro"] = s.Distro
	}
	if s.QtVersion != "" {
		out["qt_version"] = s.QtVersion
	}
	return out
}
