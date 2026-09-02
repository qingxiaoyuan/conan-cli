package config

import "strings"

const (
	OSWindows = "windows"
	OSLinux   = "linux"
	OSKylin   = "kylin"
	ArchX86   = "x86"
	ArchX64   = "x64"
	ArchARM   = "arm"
	ArchARM64 = "arm64"

	BuildTypeDebug   = "Debug"
	BuildTypeRelease = "Release"
)

func ValidOS(value string) bool {
	switch NormalizeOS(value) {
	case OSWindows, OSLinux, OSKylin:
		return true
	default:
		return false
	}
}

func ValidArch(value string) bool {
	switch NormalizeArch(value) {
	case ArchX86, ArchX64, ArchARM, ArchARM64:
		return true
	default:
		return false
	}
}

func NormalizeOS(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "windows", "win", "win32", "win64":
		return OSWindows
	case "linux":
		return OSLinux
	case "kylin", "麒麟", "neokylin":
		return OSKylin
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func NormalizeArch(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "x86", "i386", "i686", "386":
		return ArchX86
	case "x64", "x86_64", "amd64", "x86-64":
		return ArchX64
	case "arm", "arm32", "armv7", "armv7hf", "armhf", "armeabi", "armel":
		return ArchARM
	case "arm64", "aarch64", "armv8", "armv8.0":
		return ArchARM64
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func DisplayOS(value string) string {
	switch NormalizeOS(value) {
	case OSWindows:
		return "Windows"
	case OSLinux:
		return "Linux"
	case OSKylin:
		return "麒麟"
	default:
		return strings.TrimSpace(value)
	}
}

func ValidBuildType(value string) bool {
	switch NormalizeBuildType(value) {
	case BuildTypeDebug, BuildTypeRelease:
		return true
	default:
		return false
	}
}

func NormalizeBuildType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug", "dbg":
		return BuildTypeDebug
	case "release", "rel", "release-mode":
		return BuildTypeRelease
	default:
		return strings.TrimSpace(value)
	}
}

func DisplayBuildType(value string) string {
	switch NormalizeBuildType(value) {
	case BuildTypeDebug:
		return "Debug"
	case BuildTypeRelease:
		return "Release"
	default:
		return strings.TrimSpace(value)
	}
}

func DisplayArch(value string) string {
	switch NormalizeArch(value) {
	case ArchX86:
		return "x86 32 位"
	case ArchX64:
		return "x64 64 位"
	case ArchARM:
		return "ARM 32 位"
	case ArchARM64:
		return "ARM 64 位"
	default:
		return strings.TrimSpace(value)
	}
}
