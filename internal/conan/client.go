package conan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Result struct {
	Stdout   string
	Stderr   string
	Code     int
	Streamed bool
}

type CommandError struct {
	Args   []string
	Result Result
	Cause  error
}

func (e *CommandError) Error() string {
	message := strings.TrimSpace(e.Result.Stderr)
	if message == "" {
		message = strings.TrimSpace(e.Result.Stdout)
	}
	if message == "" && e.Cause != nil {
		message = e.Cause.Error()
	}
	if message == "" {
		message = "command failed"
	}
	return fmt.Sprintf("conan %s: %s", strings.Join(e.Args, " "), message)
}

// ExitCode exposes the Conan process exit code without requiring callers to
// know about os/exec's concrete error types.
func (e *CommandError) ExitCode() int {
	if e.Result.Code > 0 {
		return e.Result.Code
	}
	return 1
}

type Client struct {
	Binary   string
	BaseArgs []string
	Dir      string
	Progress io.Writer
}

func New(dir string) *Client {
	binary, baseArgs := resolveBinary()
	return &Client{Binary: binary, BaseArgs: baseArgs, Dir: dir}
}

func resolveBinary() (string, []string) {
	if binary := strings.TrimSpace(os.Getenv("CONAN_BIN")); binary != "" {
		return binary, nil
	}
	if python := strings.TrimSpace(os.Getenv("CONAN_CLI_BUNDLED_PYTHON")); python != "" {
		return python, []string{"-s", "-m", "conans.conan"}
	}
	return "conan", nil
}

func (c *Client) UseExecutable(binary string) {
	c.Binary = binary
	c.BaseArgs = nil
}

func (c *Client) Run(ctx context.Context, args ...string) (Result, error) {
	return c.run(ctx, "", false, args...)
}

func (c *Client) run(ctx context.Context, input string, removePasswordEnv bool, args ...string) (Result, error) {
	invoke := append(append([]string{}, c.BaseArgs...), args...)
	command := exec.CommandContext(ctx, c.Binary, invoke...)
	command.Dir = c.Dir
	command.Env = os.Environ()
	if len(c.BaseArgs) > 0 {
		command.Env = prependPath(command.Env, filepath.Dir(c.Binary))
		command.Env = withEnvironment(command.Env, "PYTHONNOUSERSITE", "1")
	}
	if removePasswordEnv {
		command.Env = withoutEnvironment(command.Env, "CONAN_PASSWORD")
	}
	if input != "" {
		command.Stdin = strings.NewReader(input)
	}

	var stdout, stderr bytes.Buffer
	command.Stdout = io.Writer(&stdout)
	command.Stderr = io.Writer(&stderr)
	streamed := c.Progress != nil
	if streamed {
		command.Stdout = io.MultiWriter(&stdout, c.Progress)
		command.Stderr = io.MultiWriter(&stderr, c.Progress)
	}
	err := command.Run()
	result := Result{Stdout: stdout.String(), Stderr: stderr.String(), Streamed: streamed}
	if err == nil {
		return result, nil
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		result.Code = exitError.ExitCode()
	}
	return result, &CommandError{Args: args, Result: result, Cause: err}
}

func withoutEnvironment(environment []string, key string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(environment))
	for _, value := range environment {
		if !strings.HasPrefix(value, prefix) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func withEnvironment(environment []string, key, value string) []string {
	return append(withoutEnvironment(environment, key), key+"="+value)
}

func prependPath(environment []string, dir string) []string {
	key := "PATH"
	if os.PathListSeparator == ';' {
		for _, value := range environment {
			if strings.HasPrefix(value, "Path=") {
				key = "Path"
				break
			}
		}
	}
	current := os.Getenv("PATH")
	prefix := key + "="
	for _, value := range environment {
		if strings.HasPrefix(value, prefix) {
			current = strings.TrimPrefix(value, prefix)
			break
		}
	}
	if current == "" {
		return withEnvironment(environment, key, dir)
	}
	return withEnvironment(environment, key, dir+string(os.PathListSeparator)+current)
}

func (c *Client) RunJSON(ctx context.Context, target any, args ...string) (Result, error) {
	result, err := c.Run(ctx, args...)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal([]byte(result.Stdout), target); err != nil {
		return result, fmt.Errorf("parse Conan JSON output: %w", err)
	}
	return result, nil
}

func (c *Client) Version(ctx context.Context) (Result, error) {
	return c.Run(ctx, "--version")
}

func (c *Client) Profiles(ctx context.Context) (Result, error) {
	return c.Run(ctx, "profile", "list")
}

func (c *Client) Remotes(ctx context.Context) (Result, error) {
	return c.Run(ctx, "remote", "list")
}

func (c *Client) RemoteAdd(ctx context.Context, name, url string) (Result, error) {
	return c.Run(ctx, "remote", "add", name, url, "--force")
}

func (c *Client) RemoteLogin(ctx context.Context, name, username, password string) (Result, error) {
	// Supplying --password without a value makes Conan read it from stdin. This
	// keeps the secret out of argv, process listings, and CommandError.Args.
	return c.run(ctx, password+"\n", true, "remote", "login", name, username, "--password")
}

func (c *Client) Search(ctx context.Context, query, remote string) (map[string]any, Result, error) {
	args := []string{"list", query, "--format=json"}
	if remote != "" {
		args = append(args, "--remote="+remote)
	}
	var data map[string]any
	result, err := c.RunJSON(ctx, &data, args...)
	return data, result, err
}

func (c *Client) Install(ctx context.Context, outputFolder, profile, remote string, extra ...string) (Result, error) {
	args := []string{"install", ".", "--output-folder=" + outputFolder, "--build=never"}
	if profile != "" {
		args = append(args, "--profile:host="+profile)
	}
	if remote != "" {
		args = append(args, "--remote="+remote)
	}
	args = append(args, extra...)
	return c.Run(ctx, args...)
}

func (c *Client) Inspect(ctx context.Context) (map[string]any, Result, error) {
	target := "."
	if _, err := os.Stat(filepath.Join(c.Dir, "conanfile.py")); err != nil {
		if _, txtErr := os.Stat(filepath.Join(c.Dir, "conanfile.txt")); txtErr == nil {
			target = "conanfile.txt"
		}
	}
	var data map[string]any
	result, err := c.RunJSON(ctx, &data, "inspect", target, "--format=json")
	return data, result, err
}

func (c *Client) ExportPkg(ctx context.Context, profile string, extra ...string) (Result, error) {
	args := []string{"export-pkg", "."}
	if profile != "" {
		args = append(args, "--profile:host="+profile)
	}
	args = append(args, extra...)
	return c.Run(ctx, args...)
}

func (c *Client) Upload(ctx context.Context, reference, remote string) (Result, error) {
	args := []string{"upload", reference, "--confirm"}
	if remote != "" {
		args = append(args, "--remote="+remote)
	}
	return c.Run(ctx, args...)
}
