package tui

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"

	"conan-cli/internal/config"
	"conan-cli/internal/workflow"
)

func TestRunRendersDashboardActions(t *testing.T) {
	app := workflow.New(t.TempDir())
	var output bytes.Buffer

	if err := Run(context.Background(), app, strings.NewReader("q\n"), &output); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, want := range []string{"CONAN CLI", "依赖分析", "发布表单", "设置", "诊断", "下载"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("dashboard output does not contain %q:\n%s", want, output.String())
		}
	}
}

func TestRenderUsesDesignTerminalPalette(t *testing.T) {
	var output bytes.Buffer
	ui := newUI(workflow.New(t.TempDir()), &output, true)
	ui.global = &config.Global{}
	ui.render()

	for _, want := range []string{ansiPageBackground, ansiCardBackground, "conan-cli tui", "\033[38;2;52;211;153m"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("ANSI dashboard output does not contain %q", want)
		}
	}
}

func TestSelectConsumePlatformSavesTarget(t *testing.T) {
	app := workflow.New(t.TempDir())
	var output bytes.Buffer
	ui := newUI(app, &output, false)
	ui.refreshProject()

	if !ui.selectConsumePlatform(bufioReader("kylin\nx64\nRelease\n")) {
		t.Fatal("selectConsumePlatform() = false")
	}
	project, err := app.Project()
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if got := project.Platform.Consume; got != (config.PlatformSpec{OS: config.OSKylin, Arch: config.ArchX64, BuildType: config.BuildTypeRelease}) {
		t.Fatalf("consume platform = %#v", got)
	}
}

func TestCursorNavigationDecodesArrowsAndUTF8(t *testing.T) {
	reader := newKeyReader(strings.NewReader("\x1b[B中\r"))

	key, err := reader.next()
	if err != nil || key.kind != keyDown {
		t.Fatalf("first key = %#v, err = %v; want Down", key, err)
	}
	key, err = reader.next()
	if err != nil || key.kind != keyCharacter || key.char != '中' {
		t.Fatalf("UTF-8 key = %#v, err = %v; want 中", key, err)
	}
	key, err = reader.next()
	if err != nil || key.kind != keyEnter {
		t.Fatalf("last key = %#v, err = %v; want Enter", key, err)
	}
}

func TestMoveGrid2DClampsToFourColumnMenu(t *testing.T) {
	tests := []struct {
		name                     string
		selected, deltaX, deltaY int
		want                     int
	}{
		{name: "right within row", selected: 0, deltaX: 1, want: 1},
		{name: "down to second row", selected: 1, deltaY: 1, want: 5},
		{name: "left clamps", selected: 0, deltaX: -1, want: 0},
		{name: "down clamps at last row", selected: 7, deltaY: 1, want: 7},
		{name: "right clamps at row edge", selected: 3, deltaX: 1, want: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := moveGrid2D(test.selected, test.deltaX, test.deltaY, 4, 8); got != test.want {
				t.Fatalf("moveGrid2D() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestCursorMenuFitsCursorPanel(t *testing.T) {
	ui := newUI(workflow.New(t.TempDir()), &bytes.Buffer{}, true)
	ui.setCursorLayout(80)
	menu := []string{"初始化 / 刷新", "扫描", "依赖分析", "下载", "发布表单", "设置", "诊断", "依赖管理"}
	cells := make([]string, 0, 4)
	for index := range menu[:4] {
		cells = append(cells, cursorItem(ui, index, menu[index], index == 0))
	}
	line := "  " + strings.Join(cells, "")
	if got := displayWidth(line); got > ui.panelWidth {
		t.Fatalf("cursor menu width = %d, panel width = %d", got, ui.panelWidth)
	}
}

func TestSettingsFieldTableDrivesBothPages(t *testing.T) {
	if count := settingsFieldCount("global"); count != 5 {
		t.Fatalf("global field count = %d, want 5", count)
	}
	if count := settingsFieldCount("project"); count != 16 {
		t.Fatalf("project field count = %d, want 16", count)
	}
	global := settingsFieldsFor("global")
	if index, ok := matchSettingsField("3", global); !ok || global[index].label != "用户" {
		t.Fatalf("global choice 3 resolved to %d, %v", index, ok)
	}
	if _, ok := matchSettingsField("qt", global); ok {
		t.Fatal("qt alias must not match on the global page")
	}
	project := settingsFieldsFor("project")
	if index, ok := matchSettingsField("qt", project); !ok || project[index].label != "Qt 版本" {
		t.Fatalf("project alias qt resolved to %d, %v", index, ok)
	}
}

func TestSettingsFieldApplyNormalizesValues(t *testing.T) {
	scope := &settingsScope{project: config.NewProject(t.TempDir())}
	fields := settingsFieldsFor("project")
	_, indexOS := matchSettingsFieldIndex(t, "os", fields)
	fields[indexOS].apply(scope, "麒麟")
	if scope.project.Platform.Consume.OS != config.OSKylin {
		t.Fatalf("os = %q, want kylin", scope.project.Platform.Consume.OS)
	}
	_, indexArch := matchSettingsFieldIndex(t, "arch", fields)
	fields[indexArch].apply(scope, "amd64")
	if scope.project.Platform.Consume.Arch != config.ArchX64 {
		t.Fatalf("arch = %q, want x64", scope.project.Platform.Consume.Arch)
	}
	_, indexBT := matchSettingsFieldIndex(t, "build-type", fields)
	fields[indexBT].apply(scope, "release-mode")
	if scope.project.Platform.Consume.BuildType != config.BuildTypeRelease {
		t.Fatalf("build_type = %q, want Release", scope.project.Platform.Consume.BuildType)
	}
}

func matchSettingsFieldIndex(t *testing.T, alias string, fields []settingsField) (settingsField, int) {
	t.Helper()
	index, ok := matchSettingsField(alias, fields)
	if !ok {
		t.Fatalf("alias %q not found", alias)
	}
	return fields[index], index
}

func TestSettingsScreenLineModeEditsAndSaves(t *testing.T) {
	t.Setenv("CONAN_CLI_HOME", t.TempDir())
	dir := t.TempDir()
	app := workflow.New(dir)
	var output bytes.Buffer
	ui := newUI(app, &output, false)
	ui.refreshProject()

	input := "p\n2\n6.8\n3\ngcc\n5\nkylin\ns\nq\n"
	ui.settingsScreen(context.Background(), bufioReader(input))

	project, err := app.Project()
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if project.QtVersion != "6.8" || project.Compiler.ID != "gcc" {
		t.Fatalf("project = qt %q compiler %q", project.QtVersion, project.Compiler.ID)
	}
	if got := project.Platform.Consume.OS; got != config.OSKylin {
		t.Fatalf("consume os = %q, want kylin", got)
	}
	if !strings.Contains(output.String(), "设置 · 项目") {
		t.Fatalf("settings screen did not render project page:\n%s", output.String())
	}
}

func TestCursorSettingsEditOSViaChoicePage(t *testing.T) {
	t.Setenv("CONAN_CLI_HOME", t.TempDir())
	ui := newUI(workflow.New(t.TempDir()), &bytes.Buffer{}, false)
	ui.refreshProject()
	// 按下方向键（选中 Linux），再按 Enter 确认。
	controller := &cursorController{ui: ui, keys: newKeyReader(strings.NewReader("\x1b[B\r"))}
	state := &settingsCursorState{
		active:   "project",
		selected: 4, // 使用系统
		scope:    settingsScope{project: config.NewProject(t.TempDir())},
	}
	controller.editSettingField(state)
	if got := state.scope.project.Platform.Consume.OS; got != config.OSLinux {
		t.Fatalf("consume os = %q, want linux", got)
	}
}

// bufioReader keeps the interaction test focused on the TUI without making
// the production input API expose a concrete reader type.
func bufioReader(input string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(input))
}
