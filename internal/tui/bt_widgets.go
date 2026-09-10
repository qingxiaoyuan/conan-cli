package tui

import (
	"strings"

	"conan-cli/internal/config"
	"conan-cli/internal/workflow"
	tea "github.com/charmbracelet/bubbletea"
)

// btConfirm 是发布等破坏性操作前的确认弹窗（对齐 VS Code 的 modal）。
type btConfirm struct {
	title string
	lines []string
	onYes func(m *btModel) tea.Cmd
}

func (m *btModel) confirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		confirm := m.confirm
		m.confirm = nil
		if confirm != nil && confirm.onYes != nil {
			return m, confirm.onYes(m)
		}
		return m, nil
	case "n", "N", "esc", "q":
		m.confirm = nil
		return m, nil
	}
	return m, nil
}

func (m *btModel) confirmView() string {
	lines := []string{btStyleFocus.Render(m.confirm.title), btKeycap("y", "确认") + "  " + btKeycap("n", "取消"), ""}
	for _, line := range m.confirm.lines {
		lines = append(lines, line)
	}
	return btStyleModal.Render(strings.Join(lines, "\n"))
}

// btDependencyTable 渲染依赖表格（对齐 VS Code 的 包 / 制品 / 说明）。
func btDependencyTable(rows []workflow.DependencyRow, width int) string {
	if len(rows) == 0 {
		return btStyleMuted.Render("  尚无依赖；到“仓库”Tab 添加，或直接编辑 conanfile。")
	}
	refWidth, statusWidth := 30, 14
	detailWidth := width - refWidth - statusWidth - 7
	if detailWidth < 10 {
		detailWidth = 10
	}
	header := "  " + btStyleLabel.Render(padCell("包", refWidth)+padCell("制品", statusWidth)+"说明")
	lines := []string{header, btStyleMuted.Render("  " + strings.Repeat("─", min(width-2, 76)))}
	for _, row := range rows {
		status := dependencyStatus(row.Status)
		statusCell := status
		switch row.Status {
		case "found":
			statusCell = btStyleOK.Render(status)
		case "unknown", "mismatch":
			statusCell = btStyleWarn.Render(status)
		default:
			statusCell = btStyleBad.Render(status)
		}
		refCell := padCell(row.Reference, refWidth)
		detail := clip(fallback(row.Detail, "-"), detailWidth)
		line := "  " + refCell + padCells(statusCell, statusWidth) + detail
		if row.Status != "found" && row.Status != "unknown" && row.Status != "" {
			line = btStyleRowWarn.Render(clip(line, width))
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// btMissCard 渲染“仓库没有这套制品”卡片，提示下一步而不是本机编译。
func btMissCard(missing []workflow.DependencyRow, spec config.PlatformSpec) string {
	if len(missing) == 0 {
		return ""
	}
	lines := []string{btStyleBad.Render("仓库没有这套制品")}
	combo := spec.Display()
	if strings.TrimSpace(combo) == "" {
		combo = "未选择"
	}
	lines = append(lines, "  当前组合："+btStyleMono.Render(combo))
	for _, row := range missing {
		lines = append(lines, "  · "+btStyleMono.Render(clip(row.Reference, 46))+"  "+btStyleMuted.Render(dependencyStatus(row.Status)))
	}
	lines = append(lines,
		"  请联系你们的制品负责人；打开组合编辑改组合，不要本机编译。",
		btStyleHint.Render("  不会执行 --build=missing"),
	)
	return btStyleMissCard.Render(strings.Join(lines, "\n"))
}

func btDoctorHidden(name string) bool {
	switch name {
	case "profiles", "remotes", "configured_remote", "manifest_dependencies":
		return true
	}
	return false
}

// btDoctorCards 渲染诊断检查卡；噪音项与 VS Code 一致地隐藏。
func btDoctorCards(checks []workflow.Check, width int) string {
	titles := map[string]struct {
		title string
		guide string
	}{
		"conan":          {"Conan 可执行文件", "请安装 Conan 2，或设置 CONAN_BIN / 全局 conan_bin"},
		"project_config": {"项目配置", "按 n 初始化项目，生成 .conan-cli/project.yaml"},
		"conanfile":      {"Conan 配方文件", "到“拉取依赖”Tab 选择“生成配方”"},
		"global_remote":  {"全局仓库登录", "到“设置”Tab 填写仓库地址、用户名并登录"},
		"platform":       {"目标平台", "到“拉取依赖”Tab 展开组合，选择操作系统和架构"},
	}
	var lines []string
	for _, check := range checks {
		if btDoctorHidden(check.Name) {
			continue
		}
		marker, title := btStyleOK.Render("✓"), localCheck(check.Name)
		guide := ""
		if meta, ok := titles[check.Name]; ok {
			title, guide = meta.title, meta.guide
		}
		detail := clip(fallback(check.Detail, "-"), max(10, width-24))
		if !check.OK {
			marker = btStyleBad.Render("!")
			if guide != "" {
				detail += "  → " + guide
			}
		}
		line := "  " + marker + " " + padCells(title, 18) + detail
		lines = append(lines, clip(line, width))
	}
	if len(lines) == 0 {
		lines = []string{btStyleOK.Render("  ✓ 所有检查就绪")}
	}
	return strings.Join(lines, "\n")
}

func btDoctorSummary(checks []workflow.Check) string {
	failed := 0
	for _, check := range checks {
		if btDoctorHidden(check.Name) {
			continue
		}
		if !check.OK {
			failed++
		}
	}
	if failed == 0 {
		return btStyleOK.Render("就绪")
	}
	return btStyleWarn.Render("有 " + itoa(failed) + " 项需要处理")
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

func btSplitList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
