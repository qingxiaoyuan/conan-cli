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

// bufioReader keeps the interaction test focused on the TUI without making
// the production input API expose a concrete reader type.
func bufioReader(input string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(input))
}
