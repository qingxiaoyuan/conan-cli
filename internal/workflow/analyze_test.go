package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"conan-cli/internal/config"
	"conan-cli/internal/platform"
)

// fakeConanList writes a shell script that answers `conan list <query>` calls
// with canned JSON payloads. Bodies prefixed with "FAIL " make the fake exit
// non-zero so error paths can be exercised without a real Conan.
func fakeConanList(t *testing.T, responses map[string]string) string {
	t.Helper()
	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	for query, body := range responses {
		if reason, ok := strings.CutPrefix(body, "FAIL "); ok {
			fmt.Fprintf(&script, "if [ \"$2\" = '%s' ]; then echo '%s' >&2; exit 3; fi\n", query, reason)
			continue
		}
		fmt.Fprintf(&script, "if [ \"$2\" = '%s' ]; then cat <<'JSON'\n%s\nJSON\nexit 0\nfi\n", query, body)
	}
	script.WriteString("echo '{}'\n")
	path := filepath.Join(t.TempDir(), "fake-conan")
	if err := os.WriteFile(path, []byte(script.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// newLookupApp builds an App whose Conan client is the fake script, with a
// clean CONAN_CLI_HOME so machine-wide settings cannot override it.
func newLookupApp(t *testing.T, responses map[string]string) *App {
	t.Helper()
	t.Setenv("CONAN_CLI_HOME", t.TempDir())
	t.Setenv("CONAN_BIN", "")
	t.Setenv("CONAN_CLI_BUNDLED_PYTHON", "")
	app := New(t.TempDir())
	app.Client.Binary = fakeConanList(t, responses)
	return app
}

// listJSON renders the shape `conan list <ref>` returns: the reference key
// holds revisions whose packages carry an info block per binary.
func listJSON(reference string, infos ...map[string]any) string {
	packages := map[string]any{}
	for index, info := range infos {
		packages[fmt.Sprintf("pkg%d", index)] = map[string]any{"info": info}
	}
	payload := map[string]any{
		reference: map[string]any{
			"revisions": map[string]any{
				"r1": map[string]any{"packages": packages},
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func binaryInfo(settings, options map[string]string) map[string]any {
	return map[string]any{"settings": settings, "options": options}
}

func TestLookupBinary(t *testing.T) {
	linux := platform.Settings{
		OS: config.OSLinux, Arch: config.ArchX64,
		ConanOS: "Linux", ConanArch: "x86_64",
		Compiler: "gcc", CompilerVersion: "13", QtVersion: "6.8", BuildType: "Release",
	}
	kylin := linux
	kylin.OS, kylin.Distro = config.OSKylin, "kylin"

	matchInfo := binaryInfo(
		map[string]string{"os": "Linux", "arch": "x86_64", "compiler": "gcc", "compiler.version": "13"},
		map[string]string{"qt_version": "6.8"},
	)

	tests := []struct {
		name          string
		settings      platform.Settings
		referenceJSON string
		nameJSON      string
		wantStatus    string
		wantDetail    string
	}{
		{
			name:          "found",
			settings:      linux,
			referenceJSON: listJSON("foo/1.0", matchInfo),
			wantStatus:    "found",
			wantDetail:    "已找到匹配二进制",
		},
		{
			name:     "found with case-insensitive os and compiler version prefix",
			settings: linux,
			referenceJSON: listJSON("foo/1.0", binaryInfo(
				map[string]string{"os": "linux", "arch": "X86_64", "compiler": "GCC", "compiler.version": "13.2"},
				map[string]string{"qt_version": "6.8"},
			)),
			wantStatus: "found",
			wantDetail: "已找到匹配二进制",
		},
		{
			name:     "found on kylin with distro hint",
			settings: kylin,
			referenceJSON: listJSON("foo/1.0", binaryInfo(
				map[string]string{"os": "Linux", "arch": "x86_64", "compiler": "gcc", "compiler.version": "13", "os.distro": "Kylin V10"},
				map[string]string{"qt_version": "6.8"},
			)),
			wantStatus: "found",
			wantDetail: "已找到匹配二进制",
		},
		{
			name:          "found on kylin without distro hint",
			settings:      kylin,
			referenceJSON: listJSON("foo/1.0", matchInfo),
			wantStatus:    "found",
			wantDetail:    "已找到 Linux/x64 二进制（未单独标注麒麟）",
		},
		{
			name:     "compiler mismatch keeps os/arch match",
			settings: linux,
			referenceJSON: listJSON("foo/1.0", binaryInfo(
				map[string]string{"os": "Linux", "arch": "x86_64", "compiler": "clang", "compiler.version": "16"},
				map[string]string{"qt_version": "6.8"},
			)),
			wantStatus: "missing_binary",
			wantDetail: "有该操作系统/架构的包，但编译器或 Qt 组合不匹配",
		},
		{
			name:     "qt option mismatch keeps os/arch match",
			settings: linux,
			referenceJSON: listJSON("foo/1.0", binaryInfo(
				map[string]string{"os": "Linux", "arch": "x86_64", "compiler": "gcc", "compiler.version": "13"},
				map[string]string{"qt_version": "5.15"},
			)),
			wantStatus: "missing_binary",
			wantDetail: "有该操作系统/架构的包，但编译器或 Qt 组合不匹配",
		},
		{
			name:     "no os/arch match",
			settings: linux,
			referenceJSON: listJSON("foo/1.0", binaryInfo(
				map[string]string{"os": "Windows", "arch": "x86_64", "compiler": "msvc", "compiler.version": "193"},
				map[string]string{"qt_version": "6.8"},
			)),
			wantStatus: "missing_binary",
			wantDetail: "没有 Linux/x86_64 的预编译二进制",
		},
		{
			name:          "recipe without binaries",
			settings:      linux,
			referenceJSON: listJSON("foo/1.0"),
			wantStatus:    "missing_binary",
			wantDetail:    "有配方记录，但没有预编译二进制",
		},
		{
			name:          "package exists but version missing",
			settings:      linux,
			referenceJSON: "{}",
			nameJSON:      listJSON("foo/9.0.0"),
			wantStatus:    "missing_version",
			wantDetail:    "仓库有该包，但没有这个版本",
		},
		{
			name:          "package missing entirely",
			settings:      linux,
			referenceJSON: "{}",
			nameJSON:      "{}",
			wantStatus:    "missing_package",
			wantDetail:    "仓库中没有该包",
		},
		{
			name:          "remote query failure",
			settings:      linux,
			referenceJSON: "FAIL remote unreachable",
			wantStatus:    "unknown",
			wantDetail:    "查询仓库失败",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newLookupApp(t, map[string]string{
				"foo/1.0:*": test.referenceJSON,
				"foo":       test.nameJSON,
			})
			status, detail := app.lookupBinary(context.Background(), "foo/1.0", "nexus", test.settings)
			if status != test.wantStatus {
				t.Fatalf("status = %q, want %q", status, test.wantStatus)
			}
			if !strings.Contains(detail, test.wantDetail) {
				t.Fatalf("detail = %q, want it to contain %q", detail, test.wantDetail)
			}
		})
	}
}

func TestLookupBinaryQueriesReferenceWithWildcard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-conan")
	script := "#!/bin/sh\nfor arg in \"$@\"; do echo \"$arg\" >> " + dir + "/args.log\ndone\necho '{}'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	app := newLookupApp(t, nil)
	app.Client.Binary = path
	app.lookupBinary(context.Background(), "foo/1.0", "nexus", platform.Settings{ConanOS: "Linux", ConanArch: "x86_64"})
	logged, err := os.ReadFile(filepath.Join(dir, "args.log"))
	if err != nil {
		t.Fatal(err)
	}
	calls := strings.Split(strings.TrimSpace(string(logged)), "\n")
	want := []string{"list", "foo/1.0:*", "--format=json", "--remote=nexus", "list", "foo", "--format=json", "--remote=nexus"}
	if strings.Join(calls, " ") != strings.Join(want, " ") {
		t.Fatalf("conan args = %v, want %v", calls, want)
	}
}

func TestMatchSetting(t *testing.T) {
	tests := []struct {
		got, want string
		match     bool
	}{
		{got: "Linux", want: "Linux", match: true},
		{got: "linux", want: "Linux", match: true},
		{got: "Windows", want: "Linux", match: false},
		// Empty on either side means "do not filter on this setting".
		{got: "", want: "", match: true},
		{got: "Linux", want: "", match: true},
		{got: "", want: "Linux", match: true},
	}
	for _, test := range tests {
		if got := matchSetting(test.got, test.want); got != test.match {
			t.Errorf("matchSetting(%q, %q) = %v, want %v", test.got, test.want, got, test.match)
		}
	}
}
