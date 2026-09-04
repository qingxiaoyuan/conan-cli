package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"conan-cli/internal/output"
	"conan-cli/internal/tui"
	"conan-cli/internal/workflow"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUsage(out)
		return 0
	}

	dir, jsonMode, args := parseCommon(args, false)
	if len(args) == 0 {
		printUsage(out)
		return 0
	}

	command := args[0]
	commandArgs := args[1:]
	commandDir, jsonMode, commandArgs := parseCommon(commandArgs, jsonMode)
	if commandDir != "" {
		dir = commandDir
	}
	if dir == "" {
		dir = "."
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	printer := output.New(jsonMode, out, errOut)
	app := workflow.New(dir)
	if !jsonMode && command != "tui" {
		app = workflow.NewWithOutput(dir, errOut)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var report workflow.Report
	var err error
	switch command {
	case "init":
		report, err = app.Init(ctx)
	case "status":
		report, err = app.Status(ctx)
	case "scan":
		report, err = runScan(ctx, app, commandArgs)
	case "analyze":
		report, err = runAnalyze(ctx, app, commandArgs)
	case "settings":
		report, err = runSettings(app, commandArgs)
	case "config":
		report, err = runConfig(ctx, app, commandArgs)
	case "profile":
		err = runProfile(ctx, app, commandArgs, &report)
	case "remote":
		err = runRemote(ctx, app, commandArgs, &report)
	case "catalog", "search":
		query := ""
		if len(commandArgs) > 0 {
			query = commandArgs[0]
		}
		report, err = app.Catalog(ctx, query)
	case "add":
		if len(commandArgs) != 1 {
			return fail(printer, command, errors.New("usage: conan-cli add <package>/<version>"))
		}
		report, err = app.Add(commandArgs[0])
	case "install":
		report, err = runInstall(ctx, app, commandArgs)
	case "publish":
		report, err = runPublish(ctx, app, commandArgs)
	case "packages":
		report, err = runPackages(ctx, app, commandArgs)
	case "recipe":
		report, err = runRecipe(app, commandArgs)
	case "doctor":
		report, err = app.Doctor(ctx)
	case "tui":
		if err := tui.Run(ctx, app, os.Stdin, out); err != nil {
			return fail(printer, command, err)
		}
		return 0
	default:
		return fail(printer, command, fmt.Errorf("unknown command %q", command))
	}

	if err != nil {
		if report.Action != "" {
			if jsonMode {
				_ = printer.Print(report)
			} else {
				printTextReport(printer, report)
			}
			return 1
		}
		return fail(printer, command, err)
	}
	if jsonMode {
		_ = printer.Print(report)
	} else {
		printTextReport(printer, report)
	}
	if !report.OK {
		return 1
	}
	return 0
}

func runProfile(ctx context.Context, app *workflow.App, args []string, report *workflow.Report) error {
	if len(args) != 1 || args[0] != "list" {
		return errors.New("usage: conan-cli profile list")
	}
	var err error
	*report, err = app.ProfileList(ctx)
	return err
}

func runRemote(ctx context.Context, app *workflow.App, args []string, report *workflow.Report) error {
	if len(args) == 1 && args[0] == "list" {
		var err error
		*report, err = app.RemoteList(ctx)
		return err
	}
	if len(args) >= 1 && args[0] == "add" {
		if len(args) != 3 {
			return errors.New("usage: conan-cli remote add <name> <url>")
		}
		var err error
		*report, err = app.RemoteAdd(ctx, args[1], args[2])
		return err
	}
	if len(args) >= 1 && args[0] == "login" {
		if len(args) != 3 {
			return errors.New("usage: CONAN_PASSWORD=<password> conan-cli remote login <name> <username>")
		}
		password := os.Getenv("CONAN_PASSWORD")
		if password == "" {
			return errors.New("CONAN_PASSWORD is required for remote login")
		}
		var err error
		*report, err = app.RemoteLogin(ctx, args[1], args[2], password)
		return err
	}
	return errors.New("usage: conan-cli remote list|add|login")
}

func runInstall(ctx context.Context, app *workflow.App, args []string) (workflow.Report, error) {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	profile := flags.String("profile", "", "Conan host profile")
	remote := flags.String("remote", "", "Conan remote")
	outputFolder := flags.String("output-folder", "", "Conan output folder")
	osName := flags.String("os", "", "windows|linux|kylin")
	arch := flags.String("arch", "", "x86|x64|arm|arm64")
	buildType := flags.String("build-type", "", "Debug|Release")
	if err := flags.Parse(args); err != nil {
		return workflow.Report{}, err
	}
	return app.InstallPlatform(ctx, workflow.InstallRequest{
		OS: *osName, Arch: *arch, BuildType: *buildType, Profile: *profile, Remote: *remote, OutputFolder: *outputFolder,
	})
}

func runPublish(ctx context.Context, app *workflow.App, args []string) (workflow.Report, error) {
	flags := flag.NewFlagSet("publish", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	profile := flags.String("profile", "", "Conan host profile")
	remote := flags.String("remote", "", "Conan remote")
	reference := flags.String("ref", "", "package reference, e.g. mylib/1.0")
	name := flags.String("name", "", "package name")
	version := flags.String("version", "", "package version")
	channel := flags.String("channel", "", "Conan package channel")
	osName := flags.String("os", "", "windows|linux|kylin")
	arch := flags.String("arch", "", "x86|x64|arm|arm64")
	buildType := flags.String("build-type", "", "Debug|Release")
	compiler := flags.String("compiler", "", "gcc|clang|msvc")
	compilerVersion := flags.String("compiler-version", "", "compiler major version")
	qt := flags.String("qt", "", "Qt version, e.g. 6.8")
	note := flags.String("note", "", "optional publish note")
	pkg := flags.String("package", "", "component name when the project has multiple packages")
	all := flags.Bool("all", false, "publish every component (workspaces and packages[])")
	replace := flags.Bool("replace", false, "after uploading, delete the component's previous version from the remote")
	noQt := flags.Bool("no-qt", false, "this component does not depend on Qt")
	dryRun := flags.Bool("dry-run", false, "preview publish without uploading")
	var libDirs csvList
	var includeDirs csvList
	flags.Var(&libDirs, "lib-dir", "prebuilt library directory relative to the project (repeatable)")
	flags.Var(&includeDirs, "include-dir", "header directory relative to the project (repeatable)")
	if err := flags.Parse(args); err != nil {
		return workflow.Report{}, err
	}
	return app.PublishPackage(ctx, workflow.PublishRequest{
		Name: *name, Version: *version, Ref: *reference, Channel: *channel,
		Remote: *remote, OS: *osName, Arch: *arch, BuildType: *buildType, Compiler: *compiler, CompilerVersion: *compilerVersion,
		QtVersion: *qt, Profile: *profile, Note: *note, DryRun: *dryRun, All: *all, NoQt: *noQt, Replace: *replace,
		LibDirs: libDirs.values, IncludeDirs: includeDirs.values, Package: *pkg,
	})
}

func runPackages(ctx context.Context, app *workflow.App, args []string) (workflow.Report, error) {
	if len(args) != 1 || args[0] != "list" {
		return workflow.Report{}, errors.New("usage: conan-cli packages list [--json]")
	}
	return app.PackagesList(ctx)
}

func runScan(ctx context.Context, app *workflow.App, args []string) (workflow.Report, error) {
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	apply := flags.Bool("apply", false, "write empty project fields from scan results")
	if err := flags.Parse(args); err != nil {
		return workflow.Report{}, err
	}
	return app.Scan(ctx, *apply)
}

func runRecipe(app *workflow.App, args []string) (workflow.Report, error) {
	if len(args) == 0 || args[0] != "generate" {
		return workflow.Report{}, errors.New("usage: conan-cli recipe generate --kind consume|publish [--force] [--name n] [--version v] [--qt 6.8]")
	}
	flags := flag.NewFlagSet("recipe", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	kind := flags.String("kind", "consume", "consume|publish")
	name := flags.String("name", "", "package name")
	version := flags.String("version", "", "package version")
	qt := flags.String("qt", "", "Qt version")
	force := flags.Bool("force", false, "overwrite existing recipe")
	if err := flags.Parse(args[1:]); err != nil {
		return workflow.Report{}, err
	}
	return app.GenerateRecipe(*kind, *force, *name, *version, *qt)
}

func runAnalyze(ctx context.Context, app *workflow.App, args []string) (workflow.Report, error) {
	flags := flag.NewFlagSet("analyze", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	osName := flags.String("os", "", "windows|linux|kylin")
	arch := flags.String("arch", "", "x86|x64|arm|arm64")
	buildType := flags.String("build-type", "", "Debug|Release")
	if err := flags.Parse(args); err != nil {
		return workflow.Report{}, err
	}
	return app.Analyze(ctx, *osName, *arch, *buildType)
}

func runSettings(app *workflow.App, args []string) (workflow.Report, error) {
	if len(args) == 0 || args[0] == "show" {
		return app.ShowSettings()
	}
	if args[0] != "set" {
		return workflow.Report{}, errors.New("usage: conan-cli settings show|set")
	}
	flags := flag.NewFlagSet("settings", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	input := workflow.ProjectSettingsInput{}
	name := flags.String("name", "", "project name")
	qt := flags.String("qt", "", "Qt version")
	compiler := flags.String("compiler", "", "gcc|clang|msvc")
	compilerVersion := flags.String("compiler-version", "", "compiler major version")
	osName := flags.String("os", "", "consume os")
	arch := flags.String("arch", "", "consume arch")
	buildType := flags.String("build-type", "", "Debug|Release")
	publishOS := flags.String("publish-os", "", "publish os")
	publishArch := flags.String("publish-arch", "", "publish arch")
	publishBuildType := flags.String("publish-build-type", "", "Debug|Release")
	channel := flags.String("channel", "", "channel")
	remote := flags.String("remote", "", "remote name")
	buildSystem := flags.String("build-system", "", "cmake|qmake|unknown")
	outputFolder := flags.String("output-folder", "", "output folder")
	pkg := flags.String("package", "", "component name")
	version := flags.String("version", "", "component version")
	noQt := flags.Bool("no-qt", false, "this component does not depend on Qt")
	var libDirs csvList
	var includeDirs csvList
	var workspaces csvList
	flags.Var(&libDirs, "lib-dir", "prebuilt library directory relative to the project (repeatable)")
	flags.Var(&includeDirs, "include-dir", "header directory relative to the project (repeatable)")
	flags.Var(&workspaces, "workspace", "component discovery glob relative to the project root, e.g. packages/* (repeatable; empty restores the default packages/* and src/*)")
	if err := flags.Parse(args[1:]); err != nil {
		return workflow.Report{}, err
	}
	input.Name, input.QtVersion = *name, *qt
	input.CompilerID, input.CompilerVersion = *compiler, *compilerVersion
	input.OS, input.Arch, input.BuildType = *osName, *arch, *buildType
	input.PublishOS, input.PublishArch, input.PublishBuildType = *publishOS, *publishArch, *publishBuildType
	input.Channel, input.Remote, input.BuildSystem, input.OutputFolder = *channel, *remote, *buildSystem, *outputFolder
	input.LibDirs, input.HasLibDirs = libDirs.values, libDirs.set
	input.IncludeDirs, input.HasIncludeDirs = includeDirs.values, includeDirs.set
	input.Package, input.Version, input.NoQt = *pkg, *version, *noQt
	input.Workspaces, input.HasWorkspaces = workspaces.values, workspaces.set
	return app.SaveProjectSettings(input)
}

func runConfig(ctx context.Context, app *workflow.App, args []string) (workflow.Report, error) {
	if len(args) == 0 || args[0] == "show" {
		return app.ShowSettings()
	}
	switch args[0] {
	case "set":
		flags := flag.NewFlagSet("config", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		name := flags.String("name", "", "remote name")
		url := flags.String("url", "", "Nexus Conan URL")
		username := flags.String("username", "", "login username")
		conanBin := flags.String("conan-bin", "", "conan executable")
		if err := flags.Parse(args[1:]); err != nil {
			return workflow.Report{}, err
		}
		return app.SaveGlobalSettings(ctx, workflow.GlobalSettingsInput{
			Name: *name, URL: *url, Username: *username, Password: os.Getenv("CONAN_PASSWORD"), ConanBin: *conanBin,
		})
	case "login":
		return app.ConfigLogin(ctx, os.Getenv("CONAN_PASSWORD"))
	case "test":
		return app.ConfigTest(ctx)
	default:
		return workflow.Report{}, errors.New("usage: conan-cli config show|set|login|test")
	}
}

func parseCommon(args []string, jsonMode bool) (string, bool, []string) {
	dir := ""
	remaining := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--json":
			jsonMode = true
		case arg == "--dir" && index+1 < len(args):
			dir = args[index+1]
			index++
		case strings.HasPrefix(arg, "--dir="):
			dir = strings.TrimPrefix(arg, "--dir=")
		default:
			remaining = append(remaining, arg)
		}
	}
	return dir, jsonMode, remaining
}

func printTextReport(printer *output.Printer, report workflow.Report) {
	if report.Message != "" {
		_ = printer.Text("%s", report.Message)
	}
	if report.Output != "" {
		_ = printer.Text("%s", report.Output)
	}
	for _, check := range report.Checks {
		status := "FAIL"
		if check.OK {
			status = "OK"
		}
		_ = printer.Text("[%s] %s: %s", status, check.Name, check.Detail)
	}
	if report.Data != nil && report.Message == "" && report.Output == "" && len(report.Checks) == 0 {
		_ = printer.Print(report.Data)
	}
}

type csvList struct {
	values []string
	set    bool
}

func (c *csvList) String() string { return strings.Join(c.values, ",") }

func (c *csvList) Set(value string) error {
	c.set = true
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			c.values = append(c.values, part)
		}
	}
	return nil
}

func fail(printer *output.Printer, action string, err error) int {
	printer.Error(action, err)
	return 1
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, `conan-cli - Conan 2 团队工作流

Usage:
  conan-cli init [--dir <path>] [--json]
  conan-cli status [--dir <path>] [--json]
  conan-cli scan [--apply] [--json]
  conan-cli analyze [--os windows|linux|kylin] [--arch x86|x64|arm|arm64] [--json]
  conan-cli settings show|set [--package NAME] [--version V] [--qt 6.8] [--compiler gcc] [--os kylin] [--arch x64] [--lib-dir DIR] [--include-dir DIR] [--workspace GLOB] [--json]
  conan-cli config show|set|login|test [--name nexus] [--url URL] [--username USER] [--json]
  conan-cli profile list [--json]
  conan-cli remote list|add|login [--json]
  conan-cli catalog [package] [--json]
  conan-cli search [package] [--json]
  conan-cli add <package>/<version> [--json]
  conan-cli install [--os kylin] [--arch x64] [--build-type Release] [--remote nexus] [--json]
  conan-cli publish [--package NAME|--all] [--version v] [--os kylin] [--arch x64] [--no-qt] [--lib-dir DIR] [--include-dir DIR] [--replace] [--dry-run] [--json]
  conan-cli packages list [--json]
  conan-cli recipe generate --kind consume|publish [--force] [--name n] [--version v] [--qt 6.8] [--json]
  conan-cli doctor [--json]
  conan-cli tui [--dir <path>]

安装默认只下载仓库中已有二进制。登录密码用 CONAN_PASSWORD，不会出现在命令行参数里。`)
}
