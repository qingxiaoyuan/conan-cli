package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// btTheme 是一套界面配色。主题在运行期用 T 键循环切换，也可用环境变量
// CONAN_CLI_TUI_THEME（dark|light|transparent|nord）指定初始主题。
//
// 配色哲学参考 cc-switch 的设计语言（docs/adr/0002）：全场只用三档颜色——
// 前景 fg / 注释 muted+faint / 强调 accent，层级感全部靠 surface 底色的
// chip（键帽、徽标、toast）表达；选中行是唯一用 accent 整行反色的地方。
type btTheme struct {
	name   string
	label  string
	page   lipgloss.Color
	card   lipgloss.Color
	fg     lipgloss.Color
	muted  lipgloss.Color
	faint  lipgloss.Color
	accent lipgloss.Color
	red    lipgloss.Color
	amber  lipgloss.Color
	cyan   lipgloss.Color
	mono   lipgloss.Color
	warnBg lipgloss.Color
	warnFg lipgloss.Color
	// surface 是 chip/键帽/toast 的底色；onAccent 是 accent 底上的文字色。
	// border 是普通边框色（焦点边框统一用 accent）。
	surface  lipgloss.Color
	onAccent lipgloss.Color
	border   lipgloss.Color
	selBg    lipgloss.Color
	selFg    lipgloss.Color
}

var btThemes = []btTheme{
	{
		name: "dark", label: "暗色",
		page: "#060708", card: "#0b0c0e",
		fg: "#d4d4d8", muted: "#a1a1aa", faint: "#71717a",
		accent: "#34d399", red: "#f87171", amber: "#fbbf24",
		cyan: "#6ee7b7", mono: "#c9cdd4",
		warnBg: "#2a1d1d", warnFg: "#f3c5c5",
		surface: "#23262d", onAccent: "#06130d", border: "#2f333b",
		selBg: "#34d399", selFg: "#06130d",
	},
	{
		name: "light", label: "亮色",
		page: "#eef0f4", card: "#ffffff",
		fg: "#27272a", muted: "#52525b", faint: "#6b6b74",
		accent: "#047857", red: "#dc2626", amber: "#d97706",
		cyan: "#0f766e", mono: "#3f3f46",
		warnBg: "#fdecec", warnFg: "#7f1d1d",
		surface: "#e2e4ea", onAccent: "#ffffff", border: "#9aa1ae",
		selBg: "#047857", selFg: "#ffffff",
	},
	{
		name: "transparent", label: "透明（沿用终端背景）",
		page: "", card: "",
		fg: "", muted: "#8a8a94", faint: "#8a8a94",
		accent: "#4ade80", red: "#f87171", amber: "#fbbf24",
		cyan: "#67e8f9", mono: "",
		warnBg: "", warnFg: "#f87171",
		// 透明主题没有底色：chip 退化为纯文字，选中行用强调色整行加亮。
		surface: "", onAccent: "#06120a", border: "",
		selBg: "", selFg: "#4ade80",
	},
	{
		name: "nord", label: "Nord",
		page: "#2e3440", card: "#3b4252",
		fg: "#eceff4", muted: "#b9c2d0", faint: "#7b88a1",
		accent: "#88c0d0", red: "#bf616a", amber: "#ebcb8b",
		cyan: "#8fbcbb", mono: "#d8dee9",
		warnBg: "#434c5e", warnFg: "#ebcb8b",
		surface: "#3b4252", onAccent: "#2e3440",
		// 选中行用较深的 nord9 + 近白字：frost(#88c0d0) 浅底配深字时 CJK
		// 笔画边缘会糊，深色底 + 亮字对比更干脆。
		selBg: "#5e81ac", selFg: "#eceff4",
	},
}

var btCurrentTheme = btThemes[0]

func btThemeIndex(name string) int {
	name = strings.ToLower(strings.TrimSpace(name))
	for index, theme := range btThemes {
		if theme.name == name {
			return index
		}
	}
	return 0
}

func btInitTheme() {
	theme := "dark"
	if value := strings.TrimSpace(os.Getenv("CONAN_CLI_TUI_THEME")); value != "" {
		theme = value
	}
	btApplyTheme(btThemeIndex(theme))
}

func btApplyTheme(index int) {
	btCurrentTheme = btThemes[index%len(btThemes)]
	theme := btCurrentTheme

	background := func(color lipgloss.Color) lipgloss.Style {
		style := lipgloss.NewStyle()
		if color != "" {
			style = style.Background(color)
		}
		return style
	}

	btStyleBase = background(theme.page).Foreground(theme.fg)
	btStyleCard = background(theme.card).
		Foreground(theme.muted).
		Padding(0, 1)

	btStyleBrand = lipgloss.NewStyle().Foreground(theme.cyan).Bold(true)

	// chip 体系：非激活 = surface 底 + fg；激活 = accent 底 + onAccent 加粗。
	btStyleChip = background(theme.surface).
		Foreground(theme.fg).
		Padding(0, 1)
	btStyleChipOn = background(theme.accent).
		Foreground(theme.onAccent).
		Padding(0, 1).
		Bold(true)
	if theme.surface == "" {
		btStyleChip = lipgloss.NewStyle().Foreground(theme.muted).Padding(0, 1)
	}
	if theme.page == "" && theme.accent != "" {
		// 透明主题没有底色可反白，激活态退化为强调色描边。
		btStyleChipOn = lipgloss.NewStyle().
			Foreground(theme.accent).
			Padding(0, 1).
			Bold(true).
			Underline(true)
	}
	btStyleTabActive = btStyleChipOn
	btStyleTabInactive = btStyleChip
	btStyleChipWarn = lipgloss.NewStyle().Foreground(theme.amber)
	btStyleChipBad = lipgloss.NewStyle().Foreground(theme.red)

	btStyleOK = lipgloss.NewStyle().Foreground(theme.accent)
	btStyleBad = lipgloss.NewStyle().Foreground(theme.red)
	btStyleWarn = lipgloss.NewStyle().Foreground(theme.amber)
	btStyleMuted = lipgloss.NewStyle().Foreground(theme.faint)
	btStyleLabel = lipgloss.NewStyle().Foreground(theme.muted).Bold(true)
	btStyleMono = lipgloss.NewStyle().Foreground(theme.mono)
	btStyleFocus = lipgloss.NewStyle().Foreground(theme.accent).Bold(true)
	btStyleHint = lipgloss.NewStyle().Foreground(theme.faint)

	// 边框体系：普通边框用 border 色，焦点边框统一 accent。
	btStyleBorder = lipgloss.NewStyle().Foreground(theme.border)
	btStyleBorderFocus = lipgloss.NewStyle().Foreground(theme.accent)
	// 底栏右徽章：前景/页面反色的强调块（透明主题退化为 accent 徽章）。
	btStyleBadgeRight = lipgloss.NewStyle().Foreground(theme.page).Bold(true)
	if theme.fg != "" && theme.page != "" {
		btStyleBadgeRight = btStyleBadgeRight.Background(theme.fg)
	} else if theme.accent != "" {
		btStyleBadgeRight = lipgloss.NewStyle().Background(theme.accent).Foreground(theme.onAccent).Bold(true)
	} else {
		btStyleBadgeRight = lipgloss.NewStyle().Foreground(theme.accent).Bold(true)
	}
	// fieldset 图例：普通卡图例用 muted，焦点盒/顶栏图例用强调色。
	btStyleLegend = lipgloss.NewStyle().Foreground(theme.muted)

	// 键帽 chip：键名加粗 + 描述常规，同用 surface 底，拼起来是一个胶囊。
	btStyleKey = background(theme.surface).
		Foreground(theme.fg).
		Bold(true).
		PaddingLeft(1)
	btStyleKeyDesc = background(theme.surface).
		Foreground(theme.muted)
	if theme.surface == "" {
		btStyleKey = lipgloss.NewStyle().Foreground(theme.accent).Bold(true).PaddingLeft(1)
		btStyleKeyDesc = lipgloss.NewStyle().Foreground(theme.muted)
	}

	// toast：surface 底 + 类型色文字。
	btStyleToastOK = background(theme.surface).Foreground(theme.accent).Padding(0, 1)
	btStyleToastBad = background(theme.surface).Foreground(theme.red).Padding(0, 1)
	if theme.surface == "" {
		btStyleToastOK = lipgloss.NewStyle().Foreground(theme.accent)
		btStyleToastBad = lipgloss.NewStyle().Foreground(theme.red)
	}

	missCard := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.red).
		Foreground(theme.fg).
		Padding(0, 1)
	if theme.card != "" {
		missCard = missCard.Background(theme.card)
	}
	btStyleMissCard = missCard

	modal := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.amber).
		Foreground(theme.fg).
		Padding(1, 2)
	if theme.card != "" {
		modal = modal.Background(theme.card)
	}
	btStyleModal = modal

	rowWarn := lipgloss.NewStyle().Foreground(theme.warnFg)
	if theme.warnBg != "" {
		rowWarn = rowWarn.Background(theme.warnBg)
	}
	btStyleRowWarn = rowWarn

	rowSel := lipgloss.NewStyle().Foreground(theme.selFg).Bold(true)
	if theme.selBg != "" {
		rowSel = rowSel.Background(theme.selBg)
	}
	btStyleRowSel = rowSel
}

func btCycleThemeIndex(current int) int {
	return (current + 1) % len(btThemes)
}

func btCycleTheme() {
	btApplyTheme(btCycleThemeIndex(btThemeIndex(btCurrentTheme.name)))
}
