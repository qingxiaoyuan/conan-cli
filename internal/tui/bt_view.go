package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 布局骨架（参考 cc-switch 的整体式盒式布局，设计稿见 ui-design/tui-v2.html）：
//
//	y=0..2  顶栏盒（图例「Conan 控制台」+ 项目/Tab 徽标/登录徽标）
//	y=3     动作键帽行
//	y=4     内容盒顶边框（图例 = 当前 Tab 名 + busy 提示）
//	y=5..   内容盒内部（每行以 │ 包边）
//	h-3     内容盒底边框
//	h-2     toast
//	h-1     底栏徽章
//
// 鼠标点击区域按这个骨架换算：内容区相对坐标 x+2 / y+5。
const (
	btLayoutTop    = 5 // header box(3) + actionbar(1) + content top border(1)
	btLayoutBottom = 3 // content bottom border + toast + footer
)

type btRegion struct {
	id   string
	x, y int
	w, h int
}

// btLines 是内容区逐行构建器：收集行、登记鼠标点击区域（相对当前书写宽度）。
type btLines struct {
	m       *btModel
	lines   []string
	regions []btRegion
	width   int // 当前书写文本的显示宽度预算（盒内/卡片内自动收窄）
}

func (l *btLines) add(line string) {
	l.lines = append(l.lines, line)
}

func (l *btLines) addBlock(text string) {
	l.lines = append(l.lines, strings.Split(text, "\n")...)
}

func (l *btLines) addf(format string, args ...any) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

// addRow 登记整行可点区域并渲染；selected 时整行高亮（补满当前书写宽度，
// 行内复位后重注入高亮底色，透明主题退化为整行强调色文字）。
func (l *btLines) addRow(id string, selected bool, line string) {
	l.regionRow(id)
	if !selected {
		l.add(line)
		return
	}
	l.add(l.highlight(line))
}

func (l *btLines) highlight(line string) string {
	// 剥掉内嵌前景色，整行统一 selFg，保证任何嵌套样式下选中行都可读。
	line = btStripForeground(truncateANSI(line, l.width))
	rendered := btStyleRowSel.Width(l.width).Render(line)
	if seq := btSGRSeq(btStyleRowSel); seq != "" {
		rendered = strings.ReplaceAll(rendered, "\x1b[0m", "\x1b[0m"+seq)
	}
	return rendered
}

// region 登记从当前行开始、(x, w, h) 矩形的可点区域（坐标相对本构建器）。
func (l *btLines) region(x, w, h int, id string) {
	l.regions = append(l.regions, btRegion{id: id, x: x, y: len(l.lines), w: w, h: h})
}

func (l *btLines) regionRow(id string) {
	l.region(0, max(1, l.width), 1, id)
}

// regionWide 登记从当前行开始、跨 h 行的整宽可点区域。
func (l *btLines) regionWide(h int, id string) {
	l.region(0, max(1, l.width), h, id)
}

// card 把 fn 写出的行包进带图例标题的直角边框盒；盒内登记的点击区域
// 自动换算成父级坐标（图例骑上边框，盒内内容内缩 2 列）。
func (l *btLines) card(legend string, fn func(l *btLines)) {
	start := len(l.lines)
	inner := &btLines{m: l.m, width: max(10, l.width-4)}
	fn(inner)
	l.add(btFrameLine("┌", "┐", legend, btStyleLegend, btStyleBorder, l.width))
	for _, line := range inner.lines {
		l.add(btStyleBorder.Render("│ ") + padRightANSI(truncateANSI(line, inner.width), inner.width) + btStyleBorder.Render(" │"))
	}
	l.add(btStyleBorder.Render("└" + strings.Repeat("─", max(0, l.width-2)) + "┘"))
	for _, region := range inner.regions {
		region.x += 2
		region.y += start + 1
		l.regions = append(l.regions, region)
	}
}

// btFrameLine 渲染带图例标题的边框线：┌─ 图例 ─────┐。
func btFrameLine(left, right, legend string, legendStyle, borderStyle lipgloss.Style, width int) string {
	if width < 8 {
		legend = ""
	}
	if legend == "" {
		return borderStyle.Render(left + strings.Repeat("─", max(0, width-2)) + right)
	}
	text := " " + legend + " "
	fill := max(0, width-3-displayWidth(text))
	return borderStyle.Render(left+"─") + legendStyle.Render(text) + borderStyle.Render(strings.Repeat("─", fill)+right)
}

// padRightANSI 按显示宽度在行尾补空格。
func padRightANSI(line string, width int) string {
	if d := width - displayWidth(line); d > 0 {
		return line + strings.Repeat(" ", d)
	}
	return line
}

func (m *btModel) contentHeight() int {
	height := m.height - btLayoutTop - btLayoutBottom
	if height < 6 {
		height = 6
	}
	return height
}

// headerBox 渲染顶栏盒的三行；Tab 徽标的点击区域在这里登记（y=1）。
func (m *btModel) headerBox() []string {
	inner := max(4, m.width-2)

	login := "未登录"
	style := btStyleChipBad
	if m.status != nil && m.status.global.HasPassword {
		login = "已登录"
		style = btStyleChipOn
	}
	if m.probeOK && login == "未登录" {
		login = "已连接"
		style = btStyleChipWarn
	}
	chip := "未选择平台"
	platform := m.status.proj().Platform.Consume
	if !platform.Empty() {
		chip = platform.Display()
	}
	loginBadge := style.Render(login)
	comboChip := btStyleChip.Render("目标组合 · " + chip)

	// 内容行：项目名 + Tab 徽标 + 右侧徽标；窄屏依次降级省略。
	project := btStyleMuted.Render(m.status.projectName())
	left, right := project, joinNonEmpty(loginBadge, comboChip)
	if lipgloss.Width(left)+btTabsNaturalWidth()+lipgloss.Width(right)+4 > inner {
		left = ""
	}
	if lipgloss.Width(left)+btTabsNaturalWidth()+lipgloss.Width(right)+4 > inner {
		right = loginBadge
	}
	tabs, tabWidths := m.tabBadges(2 + lipgloss.Width(left) + 2)
	content := left + "  " + tabs + strings.Repeat(" ", max(1, inner-lipgloss.Width(left)-2-tabWidths-lipgloss.Width(right))) + right

	line := btStyleBorder.Render("│") + padRightANSI(content, inner) + btStyleBorder.Render("│")

	top := btFrameLine("┌", "┐", "Conan 控制台", btStyleBrand, btStyleBorder, m.width)
	bottom := btStyleBorder.Render("└" + strings.Repeat("─", max(0, m.width-2)) + "┘")
	return []string{top, line, bottom}
}

// tabBadges 渲染 Tab 徽标并登记点击区域（y=1，startX 为屏幕绝对起始列）。
func (m *btModel) tabBadges(startX int) (string, int) {
	padding := 1
	if m.width < btTabsNaturalWidth() {
		padding = 0
	}
	activeStyle := btStyleTabActive.Padding(0, padding)
	inactiveStyle := btStyleTabInactive.Padding(0, padding)

	line := ""
	x := startX
	total := 0
	for tab := btTab(0); tab < tabCount; tab++ {
		var cell string
		if tab == m.active {
			cell = activeStyle.Render(btTabTitles[tab])
		} else {
			cell = inactiveStyle.Render(btTabTitles[tab])
		}
		width := lipgloss.Width(cell)
		m.regions = append(m.regions, btRegion{id: fmt.Sprintf("tab:%d", int(tab)), x: x, y: 1, w: width, h: 1})
		line += cell + " "
		x += width + 1
		total += width + 1
	}
	return strings.TrimRight(line, " "), total - 1
}

func joinNonEmpty(items ...string) string {
	var visible []string
	for _, item := range items {
		if item != "" {
			visible = append(visible, item)
		}
	}
	return strings.Join(visible, " ")
}

func (m *btModel) busyView() string {
	for _, job := range []btJob{btJobInstall, btJobPublish, btJobAnalyze, btJobCatalog, btJobStatus, btJobDoctor, btJobProbe, btJobScan, btJobSave, btJobLogin, btJobAdd, btJobRecipe, btJobInit} {
		if m.busy[job] {
			return btStyleFocus.Render(m.spinner.View() + "正在" + string(job) + "…")
		}
	}
	return ""
}

// btTabsNaturalWidth 是 Tab 栏 chip 样式（内边距 1 + 单空格分隔）下的显示宽度，
// 终端比它窄时切换为紧凑样式（零内边距）。
func btTabsNaturalWidth() int {
	width := 0
	for _, title := range btTabTitles {
		width += displayWidth(title) + 2
	}
	return width + (int(tabCount)-1)*1 + 4 // +4: 「│ 」与项目名间隔
}

type btButtonSpec struct {
	key  string
	desc string
	id   string
}

// btKeycap 渲染键帽 chip：键名加粗、描述常规，整体 surface 底色胶囊。
func btKeycap(key, desc string) string {
	if desc == "" {
		return btStyleKey.Render(key) + btStyleKeyDesc.Render(" ")
	}
	return btStyleKey.Render(key) + btStyleKeyDesc.Render(" "+desc+" ")
}

// actionBar 渲染当前 Tab 的动作键帽（y=3，鼠标可点，键名即快捷键）。
func (m *btModel) actionBar() string {
	var buttons []btButtonSpec
	if m.combo.open {
		buttons = []btButtonSpec{
			{"s", "尝试获取", "btn:S"},
			{"Esc", "关闭", "btn:esc"},
		}
	} else if m.confirm != nil {
		buttons = []btButtonSpec{
			{"y", "确认", "btn:y"},
			{"n", "取消", "btn:n"},
		}
	} else {
		switch m.active {
		case tabDownload:
			buttons = []btButtonSpec{{"a", "检查依赖", "btn:a"}, {"g", "生成配方", "btn:g"}, {"i", "拉取依赖", "btn:i"}, {"e", "组合", "btn:e"}}
			if m.status == nil || !m.status.initialized {
				buttons = append(buttons, btButtonSpec{"n", "初始化", "btn:n"})
			}
		case tabCatalog:
			buttons = []btButtonSpec{{"Enter", "查询", "btn:enter"}, {"a", "加入依赖", "btn:a"}}
		case tabDeps:
			buttons = []btButtonSpec{{"a", "添加依赖", "btn:a"}, {"r", "重新查找", "btn:r"}}
		case tabPublish:
			buttons = []btButtonSpec{{"p", "发布选中", "btn:p"}, {"A", "发布全部", "btn:A"}, {"e", "发布组合", "btn:e"}}
		case tabSettings:
			buttons = []btButtonSpec{{"s", "保存项目", "btn:s"}, {"l", "保存并登录", "btn:l"}, {"t", "测试连接", "btn:t"}}
		case tabDoctor:
			buttons = []btButtonSpec{{"r", "重新检查", "btn:r"}, {"o", "原始输出", "btn:o"}}
		}
	}
	buttons = append(buttons, btButtonSpec{"T", "主题", "btn:T"})

	line := ""
	x := 0
	for _, button := range buttons {
		rendered := btKeycap(button.key, button.desc)
		if line != "" {
			line += "  "
			x += 2
		}
		line += rendered
		m.regions = append(m.regions, btRegion{id: button.id, x: x, y: 3, w: lipgloss.Width(rendered), h: 1})
		x += lipgloss.Width(rendered)
	}
	return line
}

// View 渲染整帧。按 BubbleTea 惯例 View 应为纯函数，但鼠标点击需要每个
// 可交互元素的最终屏幕坐标，而坐标只有渲染后才知道——这里选择在渲染时
// 顺手登记 m.regions（View 是唯一知道全部布局的地方），换取 mouseMsg 的
// 命中测试零额外计算。代价是 View 不可并发调用；BubbleTea 保证单线程
// Update/View 交替驱动，此约束成立。
func (m *btModel) View() string {
	if m.width == 0 {
		return btStyleBase.Render("正在加载…")
	}
	m.regions = nil

	sections := m.headerBox()
	sections = append(sections, m.actionBar())

	// 内容盒顶边框：图例 = 当前 Tab 名，busy 时内联 spinner。
	legend := btTabTitles[m.active]
	if busy := m.busyView(); busy != "" {
		legend += " · " + busy
	}
	sections = append(sections, btFrameLine("┌", "┐", legend, btStyleFocus, btStyleBorderFocus, m.width))

	content, relRegions := m.contentView()
	if m.confirm != nil {
		content = m.confirmView()
		relRegions = nil
	}
	contentLines := strings.Split(content, "\n")

	// 内容超出可视高度时截断，保证头部与底部固定在屏幕内。
	available := m.height - btLayoutTop - btLayoutBottom
	if available < 0 {
		available = 0
	}
	if available > 0 && len(contentLines) > available {
		contentLines = append(contentLines[:available-1], btStyleHint.Render("…（内容超出终端高度，已截断；放大窗口查看）"))
	}
	for len(contentLines) < available {
		contentLines = append(contentLines, "")
	}
	// 把内容区登记的相对坐标换成屏幕绝对坐标（内容盒内缩 2 列）。
	// 截断后的行不再可点，避免点到底栏/toast 却命中被裁掉的内容行。
	visible := available
	if visible < 0 {
		visible = 0
	}
	contentBottom := btLayoutTop + visible
	for _, region := range relRegions {
		absY := region.y + btLayoutTop
		if absY >= contentBottom {
			continue
		}
		if absY+region.h > contentBottom {
			region.h = contentBottom - absY
		}
		if region.h <= 0 {
			continue
		}
		region.x += 2
		region.y = absY
		m.regions = append(m.regions, region)
	}
	// 内容行逐行包 │ 边框。
	innerWidth := max(1, m.width-4)
	for _, line := range contentLines {
		sections = append(sections, btStyleBorderFocus.Render("│ ")+padRightANSI(truncateANSI(line, innerWidth), innerWidth)+btStyleBorderFocus.Render(" │"))
	}
	sections = append(sections, btStyleBorderFocus.Render("└"+strings.Repeat("─", max(0, m.width-2))+"┘"))

	toast := ""
	if m.toast != "" {
		if m.toastBad {
			toast = btStyleToastBad.Render("✗ " + m.toast)
		} else {
			toast = btStyleToastOK.Render("✓ " + m.toast)
		}
	}
	sections = append(sections, toast, m.footerView())

	// 硬截断安全网：任何一行超出终端宽度都会被 lipgloss 折行，导致帧高
	// 超过终端高度——底行被推出屏幕、鼠标区域整体错位。这里保证每行
	// 显示宽度不超过终端宽（保留缩进，区域坐标才对得上）。
	for index := range sections {
		sections[index] = truncateANSI(sections[index], m.width)
	}
	// 极矮终端（<9 行）下布局装不下，优先保头部。
	if len(sections) > m.height {
		sections = sections[:m.height]
	}

	// 全屏覆盖：行内 styled 段的 \x1b[0m 复位会顺带杀掉页面前景与底色，
	// 其后纯文本回落成终端默认配色（暗色终端的白字压亮色页面会隐形）。
	// 在每个复位后重新注入「页面前景 + 底色」，再交给 btStyleBase 的
	// Width/Height 补位（那部分补位自带配色）。
	if seq := btPageSGRSeq(); seq != "" {
		for index := range sections {
			sections[index] = seq + strings.ReplaceAll(sections[index], "\x1b[0m", "\x1b[0m"+seq)
		}
	}
	return btStyleBase.
		Width(m.width).
		Height(m.height).
		Render(strings.Join(sections, "\n"))
}

// btSGRSeq 返回样式的 SGR 前缀（用于复位后重注入；Ascii 档返回空串）。
func btSGRSeq(style lipgloss.Style) string {
	return strings.TrimSuffix(style.Render(""), "\x1b[0m")
}

// btPageSGRSeq 返回当前主题「页面前景 + 底色」的 SGR 前缀
// （透明主题返回空串，表示无需注入）。
func btPageSGRSeq() string {
	if btCurrentTheme.page == "" {
		return ""
	}
	return btSGRSeq(btStyleBase)
}

// btStripForeground 剥掉行内的前景色 SGR 参数（38;…/39），保留背景/粗体/
// 反色等其余参数——选中行要用统一的 selFg，内嵌的 mono/标签前景色会破坏可读性。
func btStripForeground(line string) string {
	var out strings.Builder
	for index := 0; index < len(line); {
		if line[index] == '\x1b' && index+1 < len(line) && line[index+1] == '[' {
			end := index + 2
			for end < len(line) && (line[end] < 'a' || line[end] > 'z') && (line[end] < 'A' || line[end] > 'Z') {
				end++
			}
			if end >= len(line) || line[end] != 'm' {
				// 非 SGR 序列原样保留
				if end < len(line) {
					end++
				}
				out.WriteString(line[index:end])
				index = end
				continue
			}
			params := strings.Split(line[index+2:end], ";")
			kept := params[:0]
			for p := 0; p < len(params); p++ {
				switch params[p] {
				case "38", "39":
					// 38;2;r;g;b / 38;5;n / 39：丢弃
					if params[p] == "38" && p+1 < len(params) {
						if params[p+1] == "5" {
							p += 2
						} else if params[p+1] == "2" {
							p += 4
						}
					}
				default:
					kept = append(kept, params[p])
				}
			}
			if len(kept) == 0 || (len(kept) == 1 && kept[0] == "0") {
				if len(kept) == 1 {
					out.WriteString("\x1b[0m")
				}
			} else {
				out.WriteString("\x1b[" + strings.Join(kept, ";") + "m")
			}
			index = end + 1
			continue
		}
		out.WriteByte(line[index])
		index++
	}
	return out.String()
}

func (m *btModel) contentView() (string, []btRegion) {
	layout := &btLines{m: m, width: max(10, m.width-4)}
	if m.combo.open {
		m.comboEditorView(layout)
		layout.add("")
	}
	switch m.active {
	case tabDownload:
		m.downloadView(layout)
	case tabCatalog:
		m.catalogView(layout)
	case tabDeps:
		m.depsView(layout)
	case tabPublish:
		m.publishView(layout)
	case tabSettings:
		m.settingsView(layout)
	case tabDoctor:
		m.doctorView(layout)
	}
	return strings.Join(layout.lines, "\n"), layout.regions
}

// footerView 底栏：左侧导航徽章（surface 底）+ 右侧功能徽章（反色强调块）。
func (m *btModel) footerView() string {
	nav := map[btTab]string{
		tabDownload: "←→ 切Tab ↑↓ 滚动",
		tabCatalog:  "Tab 焦点 ↑↓ 选择 Enter 展开",
		tabDeps:     "↑↓ 滚动",
		tabPublish:  "↑↓ 选组件 space 勾选 Enter 表单",
		tabSettings: "↑↓ 选字段 Enter 编辑",
		tabDoctor:   "↑↓ 滚动",
	}
	left := btStyleKey.Render(" 导航") + btStyleKeyDesc.Render(" "+nav[m.active]+" ")
	right := btStyleBadgeRight.Render(" 功能 r 刷新 T 主题 q 退出 ")
	tail := btStyleMuted.Render(" 主题 " + btCurrentTheme.label + " · 鼠标可点")

	line := left + " " + right + tail
	if displayWidth(line) > m.width {
		line = left + " " + right
	}
	if displayWidth(line) > m.width {
		line = left + " " + btStyleBadgeRight.Render(" 功能 T 主题 q 退出 ")
	}
	return line
}

// ---------------------------------------------------------------------------
// 鼠标
// ---------------------------------------------------------------------------

func (m *btModel) mouseUpdate(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelUp:
		return m.keyUpdate(btKey("up"))
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelDown:
		return m.keyUpdate(btKey("down"))
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		return m.clickAt(msg.X, msg.Y)
	}
	return m, nil
}

func (m *btModel) clickAt(x, y int) (tea.Model, tea.Cmd) {
	for index := len(m.regions) - 1; index >= 0; index-- {
		region := m.regions[index]
		if y >= region.y && y < region.y+region.h && x >= region.x && x < region.x+region.w {
			return m.activateRegion(region.id)
		}
	}
	return m, nil
}

func (m *btModel) activateRegion(id string) (tea.Model, tea.Cmd) {
	parts := strings.SplitN(id, ":", 2)
	switch parts[0] {
	case "tab":
		if len(parts) > 1 {
			if tab, err := atoi(parts[1]); err == nil && tab >= 0 && tab < int(tabCount) {
				m.switchTab(btTab(tab))
			}
		}
		return m, nil
	case "btn":
		if len(parts) > 1 {
			return m.keyUpdate(btKey(parts[1]))
		}
		return m, nil
	case "miss":
		return m, m.openCombo("consume")
	case "combo":
		if len(parts) > 1 {
			if row, err := atoi(parts[1]); err == nil && row >= 0 && row < 6 {
				m.combo.row = row
				if row < 3 {
					return m, m.comboCycle(1)
				}
			}
		}
		return m, nil
	case "catpkg":
		if index, err := atoi(parts[1]); err == nil && index >= 0 && index < len(m.catalog.packages) {
			m.catalog.cursor = index
			m.catalog.focus = 2
			if m.catalog.expanded == index {
				m.catalog.expanded = -1
			} else {
				m.catalog.expanded = index
				m.catalog.binCursor = 0
			}
		}
		return m, nil
	case "catbin":
		if index, err := atoi(parts[1]); err == nil {
			m.catalog.binCursor = index
		}
		return m, nil
	case "catsearch":
		m.catalog.focus = 0
		return m, m.catalog.search.Focus()
	case "pubrow":
		if index, err := atoi(parts[1]); err == nil && index >= 0 && index < len(m.packages()) {
			m.publish.cursor = index
		}
		return m, nil
	case "pubform":
		if index, err := atoi(parts[1]); err == nil && index >= 0 && index < 5 {
			m.publish.formRow = index
			m.publish.editing = true
			return m, m.publishFocusRow()
		}
		return m, nil
	case "setrow":
		if index, err := atoi(parts[1]); err == nil && index >= 0 && index < 8 {
			m.settings.row = index
			m.settings.editing = true
			return m, m.settingsFocusRow()
		}
		return m, nil
	case "depsadd":
		m.deps.adding = true
		return m, m.deps.addInput.Focus()
	case "catfilter":
		if index, err := atoi(parts[1]); err == nil && index >= 0 && index < 5 {
			m.catalog.search.Blur()
			m.catalog.focus = 1
			m.catalog.filterRow = index
			return m, m.catalogCycle(1)
		}
		return m, nil
	}
	return m, nil
}

func atoi(value string) (int, error) {
	if value == "" {
		return 0, errEmptyArg
	}
	result := 0
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, errEmptyArg
		}
		result = result*10 + int(char-'0')
	}
	return result, nil
}

var errEmptyArg = fmt.Errorf("not a number")
