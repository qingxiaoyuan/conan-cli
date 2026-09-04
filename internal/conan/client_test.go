package conan

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func TestNewUsesBundledPython(t *testing.T) {
	t.Setenv("CONAN_BIN", "")
	t.Setenv("CONAN_CLI_BUNDLED_PYTHON", "/opt/bundled/python3")
	client := New(t.TempDir())
	if client.Binary != "/opt/bundled/python3" {
		t.Fatalf("binary = %q", client.Binary)
	}
	if len(client.BaseArgs) != 3 || client.BaseArgs[0] != "-s" || client.BaseArgs[1] != "-m" || client.BaseArgs[2] != "conans.conan" {
		t.Fatalf("base args = %#v", client.BaseArgs)
	}
}

func TestNewPrefersConanBinOverBundledPython(t *testing.T) {
	t.Setenv("CONAN_BIN", "/usr/bin/conan")
	t.Setenv("CONAN_CLI_BUNDLED_PYTHON", "/opt/bundled/python3")
	client := New(t.TempDir())
	if client.Binary != "/usr/bin/conan" {
		t.Fatalf("binary = %q", client.Binary)
	}
	if len(client.BaseArgs) != 0 {
		t.Fatalf("base args = %#v", client.BaseArgs)
	}
}

func TestUseExecutableClearsBundledArgs(t *testing.T) {
	client := &Client{Binary: "/opt/bundled/python3", BaseArgs: []string{"-m", "conans.conan"}}
	client.UseExecutable("/usr/bin/conan")
	if client.Binary != "/usr/bin/conan" || len(client.BaseArgs) != 0 {
		t.Fatalf("client = %#v", client)
	}
}

func TestRemoteLoginDoesNotExposePassword(t *testing.T) {
	if os.Getenv("GO_WANT_CONAN_HELPER_PROCESS") == "1" {
		return
	}
	t.Setenv("GO_WANT_CONAN_HELPER_PROCESS", "1")
	t.Setenv("CONAN_PASSWORD", "secret")
	client := &Client{Binary: os.Args[0], Dir: t.TempDir()}
	result, err := client.RemoteLogin(context.Background(), "nexus", "alice", "secret")
	if err != nil {
		t.Fatalf("RemoteLogin returned error: %v", err)
	}
	if result.Code != 0 {
		t.Fatalf("unexpected result code: %d", result.Code)
	}
}

func TestExportPkg(t *testing.T) {
	if os.Getenv("GO_WANT_CONAN_HELPER_PROCESS") == "1" {
		return
	}
	t.Setenv("GO_WANT_CONAN_HELPER_PROCESS", "1")
	t.Setenv("GO_CONAN_HELPER_MODE", "export-pkg")
	client := &Client{Binary: os.Args[0], Dir: t.TempDir()}
	if _, err := client.ExportPkg(context.Background(), ".conan-cli/recipes/demo", "default", "-s", "os=Linux"); err != nil {
		t.Fatalf("ExportPkg returned error: %v", err)
	}
}

func TestConanHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_CONAN_HELPER_PROCESS") != "1" {
		return
	}
	switch os.Getenv("GO_CONAN_HELPER_MODE") {
	case "export-pkg":
		hasCmd := false
		for _, argument := range os.Args {
			if argument == "export-pkg" {
				hasCmd = true
			}
			if argument == "create" {
				os.Exit(5)
			}
		}
		if hasCmd {
			os.Exit(0)
		}
		os.Exit(5)
	}
	for _, argument := range os.Args {
		if strings.Contains(argument, "secret") {
			os.Exit(2)
		}
	}
	if _, exists := os.LookupEnv("CONAN_PASSWORD"); exists {
		os.Exit(3)
	}
	input, _ := io.ReadAll(os.Stdin)
	if string(input) != "secret\n" {
		os.Exit(4)
	}
	os.Exit(0)
}
