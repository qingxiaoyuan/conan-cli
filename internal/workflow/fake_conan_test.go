package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeConan writes a shell script that stands in for the conan binary. Calls
// are keyed by "$1 $2" (for example "list fmt/1.0:*", "remote list"); bodies
// prefixed with "FAIL " exit non-zero so error paths can be exercised. Any
// unmatched call prints "{}" and succeeds.
func fakeConan(t *testing.T, responses map[string]string) string {
	t.Helper()
	var script strings.Builder
	script.WriteString("#!/bin/sh\nkey=\"$1 $2\"\n")
	for key, body := range responses {
		if reason, ok := strings.CutPrefix(body, "FAIL "); ok {
			fmt.Fprintf(&script, "if [ \"$key\" = '%s' ]; then echo '%s' >&2; exit 3; fi\n", key, reason)
			continue
		}
		fmt.Fprintf(&script, "if [ \"$key\" = '%s' ]; then cat <<'JSON'\n%s\nJSON\nexit 0\nfi\n", key, body)
	}
	script.WriteString("echo '{}'\n")
	path := filepath.Join(t.TempDir(), "fake-conan")
	if err := os.WriteFile(path, []byte(script.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// newFakeApp builds an App backed by the fake conan script, with an isolated
// CONAN_CLI_HOME so machine-wide settings cannot leak in.
func newFakeApp(t *testing.T, responses map[string]string) *App {
	t.Helper()
	t.Setenv("CONAN_CLI_HOME", t.TempDir())
	t.Setenv("CONAN_BIN", "")
	t.Setenv("CONAN_CLI_BUNDLED_PYTHON", "")
	app := New(t.TempDir())
	app.Client.Binary = fakeConan(t, responses)
	return app
}

// argsLoggingConan creates a fake conan that records every invocation to a
// log file and succeeds; it returns the binary path and the log path.
func argsLoggingConan(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "args.log")
	script := "#!/bin/sh\nfor arg in \"$@\"; do echo \"$arg\" >> " + log + "; done\necho '{}'\n"
	path := filepath.Join(dir, "fake-conan")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path, log
}
