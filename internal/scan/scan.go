package scan

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"conan-cli/internal/config"
	"conan-cli/internal/platform"
)

type Finding struct {
	Value  string `json:"value"`
	Source string `json:"source"`
	OK     bool   `json:"ok"`
}

type Install struct {
	Version string `json:"version"`
	Short   string `json:"short"`
	Prefix  string `json:"prefix,omitempty"`
	QMake   string `json:"qmake,omitempty"`
	Label   string `json:"label"`
}

type Result struct {
	Qt              Finding             `json:"qt"`
	QtInstalls      []Install           `json:"qt_installs"`
	Compiler        config.Compiler     `json:"compiler"`
	CompilerFinding Finding             `json:"compiler_finding"`
	Host            config.PlatformSpec `json:"host"`
}

func Project(dir string) Result {
	_ = dir
	result := Result{Host: platform.DetectHost()}
	result.QtInstalls = listInstalledQt()
	if len(result.QtInstalls) == 0 {
		result.Qt = Finding{Source: "未探测到本机 Qt。请按目标制品手填，不必装在这台开发机上"}
	} else {
		result.Qt = Finding{Source: "本机看到 " + strconv.Itoa(len(result.QtInstalls)) + " 套 Qt，仅供参考"}
	}
	result.Compiler, result.CompilerFinding = scanCompiler()
	return result
}

func listInstalledQt() []Install {
	seenPath := map[string]bool{}
	var installs []Install
	for _, qmake := range qtCandidates() {
		resolved := resolvePath(qmake)
		if seenPath[resolved] {
			continue
		}
		seenPath[resolved] = true
		install, ok := queryInstall(qmake)
		if !ok {
			continue
		}
		installs = append(installs, install)
	}
	return uniqueInstalls(installs)
}

func qtCandidates() []string {
	var candidates []string
	candidates = append(candidates, lookPathQMakes()...)
	candidates = append(candidates, envQMakes()...)
	candidates = append(candidates, qtchooserQMakes()...)
	candidates = append(candidates, globQMakes(qtSearchRoots())...)
	return candidates
}

func lookPathQMakes() []string {
	names := []string{"qmake6", "qmake-qt6", "qmake5", "qmake-qt5", "qmake"}
	if runtime.GOOS == "windows" {
		names = []string{"qmake6.exe", "qmake-qt6.exe", "qmake5.exe", "qmake-qt5.exe", "qmake.exe", "qmake"}
	}
	var out []string
	for _, name := range names {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		out = append(out, path)
	}
	return out
}

func envQMakes() []string {
	var out []string
	for _, key := range []string{"QTDIR", "QT_ROOT_DIR"} {
		root := strings.TrimSpace(os.Getenv(key))
		if root == "" {
			continue
		}
		out = append(out, qmakeInBin(root)...)
		out = append(out, qmakeInBin(filepath.Join(root, "bin"))...)
	}
	return out
}

func qmakeInBin(dir string) []string {
	var out []string
	for _, name := range []string{"qmake", "qmake.exe", "qmake6", "qmake6.exe"} {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			out = append(out, path)
		}
	}
	return out
}

func qtSearchRoots() []string {
	home, _ := os.UserHomeDir()
	roots := []string{
		filepath.Join(home, "Qt"),
		"/opt/Qt",
		"/usr/lib",
		"/usr/local/Qt",
		"/usr/local/opt",
		"/opt/homebrew/opt",
	}
	if runtime.GOOS == "windows" {
		roots = append(roots, `C:\Qt`, `D:\Qt`)
		if home != "" {
			roots = append(roots, filepath.Join(home, "Qt"))
		}
	}
	return roots
}

func globQMakes(roots []string) []string {
	patterns := []string{
		filepath.Join("*", "*", "bin", "qmake"),
		filepath.Join("*", "*", "bin", "qmake.exe"),
		filepath.Join("qt*", "bin", "qmake"),
		filepath.Join("qt*", "bin", "qmake.exe"),
		filepath.Join("*", "qt*", "bin", "qmake"),
		filepath.Join("*", "qt*", "bin", "qmake.exe"),
		filepath.Join("qt@*", "bin", "qmake"),
	}
	var out []string
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		if _, err := os.Stat(root); err != nil {
			continue
		}
		for _, rel := range patterns {
			matches, _ := filepath.Glob(filepath.Join(root, rel))
			for _, match := range matches {
				if skipQtPath(match) {
					continue
				}
				out = append(out, match)
			}
		}
	}
	return out
}

func skipQtPath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		switch strings.ToLower(part) {
		case "tools", "docs", "examples", "installer":
			return true
		}
	}
	return false
}

func qtchooserQMakes() []string {
	bin, err := exec.LookPath("qtchooser")
	if err != nil {
		return nil
	}
	output, err := runTimed(bin, "-list-versions")
	if err != nil {
		return nil
	}
	var out []string
	for _, version := range strings.Split(strings.TrimSpace(output), "\n") {
		version = strings.TrimSpace(version)
		if version == "" {
			continue
		}
		env, err := runTimed(bin, "-qt="+version, "-print-env")
		if err != nil {
			continue
		}
		dir := chooserToolDir(env)
		if dir == "" {
			continue
		}
		out = append(out, qmakeInBin(dir)...)
	}
	return out
}

func chooserToolDir(env string) string {
	for _, line := range strings.Split(env, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "QTTOOLDIR=") {
			continue
		}
		value := strings.TrimPrefix(line, "QTTOOLDIR=")
		return strings.Trim(value, `"'`)
	}
	return ""
}

func queryInstall(qmake string) (Install, bool) {
	version := strings.TrimSpace(qmakeQuery(qmake, "QT_VERSION"))
	if version == "" || !versionPattern.MatchString(version) {
		return Install{}, false
	}
	prefix := strings.TrimSpace(qmakeQuery(qmake, "QT_INSTALL_PREFIX"))
	short := shortVersion(version)
	label := version
	if prefix != "" {
		label = version + "（" + displayPrefix(prefix) + "）"
	}
	return Install{
		Version: version,
		Short:   short,
		Prefix:  prefix,
		QMake:   qmake,
		Label:   label,
	}, true
}

var versionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)

func qmakeQuery(qmake, key string) string {
	output, err := runTimed(qmake, "-query", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(output)
}

func runTimed(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func shortVersion(version string) string {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return version
}

func displayPrefix(prefix string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" && (prefix == home || strings.HasPrefix(prefix, home+string(os.PathSeparator))) {
		return "~" + prefix[len(home):]
	}
	return prefix
}

func resolvePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return resolved
}

func uniqueInstalls(installs []Install) []Install {
	type key struct{ version, prefix string }
	seen := map[key]bool{}
	var out []Install
	sort.SliceStable(installs, func(i, j int) bool {
		return versionGreater(installs[i].Version, installs[j].Version)
	})
	for _, install := range installs {
		item := key{install.Version, filepath.Clean(install.Prefix)}
		if seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, install)
	}
	return out
}

func versionGreater(a, b string) bool {
	as, bs := versionParts(a), versionParts(b)
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		if av != bv {
			return av > bv
		}
	}
	return false
}

func versionParts(version string) []int {
	var out []int
	for _, part := range strings.Split(version, ".") {
		n, _ := strconv.Atoi(part)
		out = append(out, n)
	}
	return out
}

func scanCompiler() (config.Compiler, Finding) {
	if runtime.GOOS == "windows" {
		if compiler, finding := fromVersionCommand("cl", true); finding.OK {
			return compiler, finding
		}
	}
	if compiler, finding := fromVersionCommand("g++", false); finding.OK {
		return compiler, finding
	}
	if compiler, finding := fromVersionCommand("gcc", false); finding.OK {
		return compiler, finding
	}
	if compiler, finding := fromVersionCommand("clang++", false); finding.OK {
		return compiler, finding
	}
	return config.Compiler{}, Finding{Source: "未探测到，请在设置中手填"}
}

func fromVersionCommand(binary string, msvc bool) (config.Compiler, Finding) {
	args := []string{"--version"}
	if binary == "g++" || binary == "gcc" {
		args = []string{"-dumpversion"}
	}
	if msvc {
		args = []string{}
	}
	command := exec.Command(binary, args...)
	output, err := command.CombinedOutput()
	if err != nil && len(output) == 0 {
		return config.Compiler{}, Finding{}
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return config.Compiler{}, Finding{}
	}
	match := regexp.MustCompile(`([0-9]+)(?:\.[0-9]+)*`).FindStringSubmatch(text)
	if len(match) < 2 {
		return config.Compiler{}, Finding{}
	}
	id := "gcc"
	switch {
	case msvc || strings.Contains(strings.ToLower(text), "microsoft"):
		id = "msvc"
	case strings.Contains(binary, "clang") || strings.Contains(strings.ToLower(text), "clang"):
		id = "clang"
	}
	compiler := config.Compiler{ID: id, Version: match[1]}
	return compiler, Finding{Value: compiler.Display(), Source: binary, OK: true}
}
