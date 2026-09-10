package tui

import (
	"context"
	"strings"

	"conan-cli/internal/config"
	"conan-cli/internal/workflow"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// btCombo 是目标组合（os + arch + buildType + 编译器 + Qt）编辑器，
// 消费端（拉取依赖）与发布端共用，target 区分。改动即静默保存，
// 对齐 VS Code 的 save-project-quiet。
type btCombo struct {
	target string // "consume" | "publish"
	open   bool
	row    int // 0 系统 1 架构 2 构建 3 编译器 4 编译器版本 5 Qt 版本
	text   textinput.Model
}

func newBTCombo(target string) btCombo {
	input := textinput.New()
	input.CharLimit = 32
	return btCombo{target: target, text: input}
}

var btOSOptions = []string{config.OSWindows, config.OSLinux, config.OSKylin}
var btArchOptions = []string{config.ArchX86, config.ArchX64, config.ArchARM, config.ArchARM64}
var btBuildOptions = []string{config.BuildTypeRelease, config.BuildTypeDebug}

func (m *btModel) comboSpec() config.PlatformSpec {
	project := m.status.proj()
	if m.combo.target == "publish" {
		return project.Platform.Publish
	}
	return project.Platform.Consume
}

func (m *btModel) openCombo(target string) tea.Cmd {
	m.combo.target = target
	m.combo.open = true
	m.combo.row = 0
	m.combo.text.SetValue("")
	return m.combo.text.Focus()
}

func (m *btModel) closeCombo() {
	m.combo.open = false
	m.combo.text.Blur()
	m.combo.text.SetValue("")
}

func (m *btModel) comboKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.closeCombo()
		return m, nil
	case "s", "S":
		return m, m.run(btJobScan, func(ctx context.Context) (workflow.Report, error) {
			return m.api.Scan(ctx)
		})
	}

	if m.combo.row >= 3 {
		switch msg.String() {
		case "up":
			m.combo.row = (m.combo.row + 5) % 6
			return m, nil
		case "down":
			m.combo.row = (m.combo.row + 1) % 6
			return m, nil
		case "enter":
			if value := strings.TrimSpace(m.combo.text.Value()); value != "" {
				cmd := m.comboSetValue(m.combo.row, value)
				m.combo.text.SetValue("")
				return m, cmd
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.combo.text, cmd = m.combo.text.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "up":
		m.combo.row = (m.combo.row + 5) % 6
		return m, nil
	case "down":
		m.combo.row = (m.combo.row + 1) % 6
		return m, nil
	case "left":
		return m, m.comboCycle(-1)
	case "right", "enter":
		return m, m.comboCycle(1)
	}
	return m, nil
}

func (m *btModel) comboCycle(delta int) tea.Cmd {
	spec := m.comboSpec()
	switch m.combo.row {
	case 0:
		return m.comboSetValue(0, btCycleOption(btOSOptions, config.NormalizeOS(spec.OS), delta))
	case 1:
		return m.comboSetValue(1, btCycleOption(btArchOptions, config.NormalizeArch(spec.Arch), delta))
	case 2:
		return m.comboSetValue(2, btCycleOption(btBuildOptions, config.NormalizeBuildType(spec.BuildType), delta))
	}
	return nil
}

func btCycleOption(options []string, current string, delta int) string {
	index := 0
	for i, option := range options {
		if option == current {
			index = i
			break
		}
	}
	index = (index + delta + len(options)) % len(options)
	return options[index]
}

func (m *btModel) comboSetValue(row int, value string) tea.Cmd {
	input := workflow.ProjectSettingsInput{}
	if m.combo.target == "publish" {
		switch row {
		case 0:
			input.PublishOS = value
		case 1:
			input.PublishArch = value
		case 2:
			input.PublishBuildType = value
		case 3:
			input.CompilerID = value
		case 4:
			input.CompilerVersion = value
		case 5:
			input.QtVersion = value
		}
	} else {
		switch row {
		case 0:
			input.OS = value
		case 1:
			input.Arch = value
		case 2:
			input.BuildType = value
		case 3:
			input.CompilerID = value
		case 4:
			input.CompilerVersion = value
		case 5:
			input.QtVersion = value
		}
	}
	return m.run(btJobSave, func(ctx context.Context) (workflow.Report, error) {
		return m.api.SaveProjectSettings(input)
	})
}

// comboApplyScan 用扫描结果预填 Qt / 编译器并保存（VS Code 的“尝试获取”）。
func (m *btModel) comboApplyScan(report workflow.Report) tea.Cmd {
	data := reportMap(report)
	result, ok := data["scan"].(workflow.ScanResult)
	if !ok {
		return nil
	}
	input := workflow.ProjectSettingsInput{}
	if len(result.QtInstalls) > 0 && result.QtInstalls[0].Short != "" {
		input.QtVersion = result.QtInstalls[0].Short
	}
	if result.Compiler.ID != "" {
		input.CompilerID = result.Compiler.ID
	}
	if result.Compiler.Version != "" {
		input.CompilerVersion = result.Compiler.Version
	}
	if btInputEmpty(input) {
		m.setToast(false, "本机没有扫到 Qt / 编译器，请在组合编辑里手填")
		return nil
	}
	return m.run(btJobSave, func(ctx context.Context) (workflow.Report, error) {
		return m.api.SaveProjectSettings(input)
	})
}

func btInputEmpty(input workflow.ProjectSettingsInput) bool {
	return strings.TrimSpace(input.QtVersion) == "" &&
		strings.TrimSpace(input.CompilerID) == "" &&
		strings.TrimSpace(input.CompilerVersion) == ""
}

func (m *btModel) comboResize(width int) {
	m.combo.text.Width = max(12, width-28)
}

// comboBarView 渲染收起的组合条：kicker + 人类可读标题 + mono 摘要行。
func (m *btModel) comboBarView(target string) string {
	project := m.status.proj()
	spec := project.Platform.Consume
	kicker := "当前目标组合 · 与发布默认同步"
	if target == "publish" {
		spec = project.Platform.Publish
		kicker = "发布目标组合 · 与拉取默认同步"
	}
	lines := []string{
		btStyleLabel.Render("  " + kicker),
		"  " + btStyleFocus.Render(btComboTitle(project, spec)),
		"  " + btStyleMono.Render(btComboMono(project, spec)),
	}
	if !project.Platform.Publish.Empty() && project.Platform.Publish != project.Platform.Consume {
		lines = append(lines, "  "+btStyleWarn.Render("若与拉取页拆开目标组合，制品可能对不上"))
	}
	return strings.Join(lines, "\n")
}

func btComboTitle(project *config.Project, spec config.PlatformSpec) string {
	if strings.TrimSpace(spec.OS) == "" || strings.TrimSpace(spec.Arch) == "" {
		return "未选择"
	}
	parts := []string{config.DisplayOS(spec.OS), spec.Arch}
	if project.Compiler.Display() != "" {
		parts = append(parts, project.Compiler.Display())
	}
	if project.QtVersion != "" {
		parts = append(parts, "Qt "+project.QtVersion)
	}
	parts = append(parts, fallback(config.DisplayBuildType(spec.BuildType), "Release"))
	return strings.Join(parts, " · ")
}

func btComboMono(project *config.Project, spec config.PlatformSpec) string {
	values := []string{"os=" + spec.OS, "arch=" + spec.Arch}
	if project.Compiler.ID != "" {
		values = append(values, "compiler="+project.Compiler.ID)
	}
	if project.Compiler.Version != "" {
		values = append(values, "compiler.version="+project.Compiler.Version)
	}
	if project.QtVersion != "" {
		values = append(values, "qt="+project.QtVersion)
	}
	values = append(values, "build_type="+fallback(config.NormalizeBuildType(spec.BuildType), "Release"))
	return strings.Join(values, " ")
}

// comboEditorView 渲染展开的组合编辑器（卡片式）。文本行不预填当前值，避免覆盖输入。
func (m *btModel) comboEditorView(layout *btLines) {
	project := m.status.proj()
	spec := m.comboSpec()
	legend := "组合编辑 · 拉取依赖（消费端）"
	if m.combo.target == "publish" {
		legend = "组合编辑 · 发布（发布端）"
	}
	layout.card(legend, func(l *btLines) {
		selector := func(label, value string, row int) string {
			return "   " + padCells(label, 12) + "◀ " + btStyleMono.Render(value) + " ▶"
		}
		for row, line := range []string{
			selector("系统", fallback(config.DisplayOS(spec.OS), "未选择"), 0),
			selector("架构", fallback(spec.Arch, "未选择"), 1),
			selector("构建", fallback(config.DisplayBuildType(spec.BuildType), "未选择"), 2),
		} {
			l.addRow("combo:"+itoa(row), row == m.combo.row, line)
		}

		textRow := func(label, value string, row int) string {
			if row == m.combo.row {
				m.combo.text.Placeholder = fallback(value, "输入新值")
				return "   " + padCells(label, 12) + m.combo.text.View()
			}
			return "   " + padCells(label, 12) + btStyleMono.Render(fallback(value, "（未设置）"))
		}
		for row, line := range []string{
			textRow("编译器", project.Compiler.ID, 3),
			textRow("编译器版本", project.Compiler.Version, 4),
			textRow("Qt 版本", project.QtVersion, 5),
		} {
			l.addRow("combo:"+itoa(row+3), row+3 == m.combo.row, line)
		}
	})
	layout.add(btStyleHint.Render("  ↑/↓ 选行 · ←/→ 改值 · Enter 确认 · s 尝试获取（扫描本机 Qt/编译器） · Esc 关闭"))
}
