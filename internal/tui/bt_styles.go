package tui

import "github.com/charmbracelet/lipgloss"

// 样式全部由 btApplyTheme 按当前主题重建（见 bt_theme.go）。
// 视觉语言参考 cc-switch（ADR 0002）：三档颜色（fg/muted/accent）+
// surface 底色的 chip（键帽/徽标/toast）+ 选中行 accent 整行反色。
var (
	btStyleBase  lipgloss.Style
	btStyleCard  lipgloss.Style
	btStyleBrand lipgloss.Style

	btStyleTabActive   lipgloss.Style
	btStyleTabInactive lipgloss.Style
	btStyleChip        lipgloss.Style
	btStyleChipOn      lipgloss.Style
	btStyleChipWarn    lipgloss.Style
	btStyleChipBad     lipgloss.Style

	btStyleKey     lipgloss.Style
	btStyleKeyDesc lipgloss.Style

	btStyleToastOK  lipgloss.Style
	btStyleToastBad lipgloss.Style

	btStyleOK    lipgloss.Style
	btStyleBad   lipgloss.Style
	btStyleWarn  lipgloss.Style
	btStyleMuted lipgloss.Style
	btStyleLabel lipgloss.Style
	btStyleMono  lipgloss.Style
	btStyleFocus lipgloss.Style
	btStyleHint  lipgloss.Style

	btStyleMissCard lipgloss.Style
	btStyleModal    lipgloss.Style
	btStyleRowWarn  lipgloss.Style
	btStyleRowSel   lipgloss.Style

	btStyleBorder      lipgloss.Style
	btStyleBorderFocus lipgloss.Style
	btStyleBadgeRight  lipgloss.Style
	btStyleLegend      lipgloss.Style
)
