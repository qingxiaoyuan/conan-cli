package tui

import (
	"context"
	"sort"
	"strings"

	"conan-cli/internal/config"
	"conan-cli/internal/workflow"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type btCatalogModel struct {
	search    textinput.Model
	focus     int // 0 搜索框 1 过滤器 2 结果列表
	osIdx     int
	archIdx   int
	buildIdx  int
	compIdx   int
	qtIdx     int
	filterRow int // focus==1 时选中的过滤项
	packages  []workflow.CatalogPackage
	loaded    bool
	message   string
	cursor    int // 组件行
	expanded  int // 展开的组件下标；-1 表示全部收起
	binCursor int
	// 从数据派生的过滤选项（编译器 / Qt）
	compilerOptions []string
	qtOptions       []string
}

func newBTCatalogModel() btCatalogModel {
	input := textinput.New()
	input.Placeholder = "输入包名，如 qtutils"
	input.CharLimit = 64
	return btCatalogModel{search: input, expanded: -1}
}

func (c *btCatalogModel) blur() {
	c.search.Blur()
}

func (c *btCatalogModel) inputFocused() bool { return c.focus == 0 && c.search.Focused() }

func (c *btCatalogModel) resize(width int) {
	c.search.Width = max(16, width-20)
}

var btCatalogOSLabels = []string{"全部系统", "Windows", "Linux", "麒麟"}
var btCatalogArchLabels = []string{"全部架构", "x86", "x64", "ARM", "ARM64"}
var btCatalogBuildLabels = []string{"全部构建", "Release", "Debug"}

func (m *btModel) catalogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := &m.catalog
	switch c.focus {
	case 0:
		if !c.search.Focused() {
			switch msg.String() {
			case "enter", "/", "i", "I":
				return m, c.search.Focus()
			case "a", "A":
				return m, m.catalogAdd()
			case "tab":
				c.focus = 1
				return m, nil
			case "down":
				c.focus = 2
				return m, nil
			}
			return m, nil
		}
		switch msg.String() {
		case "enter":
			return m, m.catalogCmd()
		case "tab":
			c.search.Blur()
			c.focus = 1
			return m, nil
		case "esc":
			c.search.Blur()
			return m, nil
		case "down":
			c.search.Blur()
			c.focus = 2
			return m, nil
		}
		var cmd tea.Cmd
		c.search, cmd = c.search.Update(msg)
		return m, cmd

	case 1:
		switch msg.String() {
		case "up":
			c.filterRow = (c.filterRow + 4) % 5
			return m, nil
		case "down", "tab":
			if msg.String() == "tab" {
				c.focus = 2
				return m, nil
			}
			c.filterRow = (c.filterRow + 1) % 5
			return m, nil
		case "left":
			return m, m.catalogCycle(-1)
		case "right", "enter":
			return m, m.catalogCycle(1)
		case "esc":
			c.focus = 0
			return m, c.search.Focus()
		}
		return m, nil
	}

	// 结果列表
	switch msg.String() {
	case "up":
		if c.expanded >= 0 && c.binCursor > 0 {
			c.binCursor--
		} else if c.cursor > 0 {
			c.cursor--
		}
		return m, nil
	case "down":
		if c.expanded >= 0 && c.expanded < len(c.packages) {
			if c.binCursor+1 < len(c.packages[c.expanded].Binaries) {
				c.binCursor++
				return m, nil
			}
		}
		if c.cursor+1 < len(c.packages) {
			c.cursor++
		}
		return m, nil
	case "enter":
		if c.cursor < len(c.packages) {
			if c.expanded == c.cursor {
				c.expanded = -1
			} else {
				c.expanded = c.cursor
				c.binCursor = 0
			}
		}
		return m, nil
	case "a", "A":
		return m, m.catalogAdd()
	case "tab":
		c.focus = 0
		return m, c.search.Focus()
	case "esc":
		c.expanded = -1
		return m, nil
	}
	return m, nil
}

func (m *btModel) catalogCycle(delta int) tea.Cmd {
	c := &m.catalog
	switch c.filterRow {
	case 0:
		c.osIdx = (c.osIdx + delta + len(btCatalogOSLabels)) % len(btCatalogOSLabels)
	case 1:
		c.archIdx = (c.archIdx + delta + len(btCatalogArchLabels)) % len(btCatalogArchLabels)
	case 2:
		c.buildIdx = (c.buildIdx + delta + len(btCatalogBuildLabels)) % len(btCatalogBuildLabels)
	case 3:
		if len(c.compilerOptions) > 0 {
			c.compIdx = (c.compIdx + delta + len(c.compilerOptions)) % len(c.compilerOptions)
		}
	case 4:
		if len(c.qtOptions) > 0 {
			c.qtIdx = (c.qtIdx + delta + len(c.qtOptions)) % len(c.qtOptions)
		}
	}
	return m.catalogCmd()
}

func (m *btModel) catalogAdd() tea.Cmd {
	c := &m.catalog
	if c.expanded < 0 || c.expanded >= len(c.packages) {
		if c.cursor >= 0 && c.cursor < len(c.packages) {
			c.expanded = c.cursor
			c.binCursor = 0
			m.setToast(false, "请选择一条制品后再按 a 加入依赖")
			return nil
		}
		m.setToast(true, "没有可加入的制品：先查询并展开组件")
		return nil
	}
	pkg := c.packages[c.expanded]
	if len(pkg.Binaries) == 0 || c.binCursor >= len(pkg.Binaries) {
		m.setToast(true, "这个组件没有制品记录，无法加入依赖")
		return nil
	}
	binary := pkg.Binaries[c.binCursor]
	reference := fallback(binary.Reference, pkg.Name+"/"+binary.Version)
	if reference == "/" {
		m.setToast(true, "选中制品缺少包引用，无法加入依赖")
		return nil
	}
	return m.run(btJobAdd, func(ctx context.Context) (workflow.Report, error) {
		return m.api.Add(reference)
	})
}

func (m *btModel) catalogCmd() tea.Cmd {
	c := &m.catalog
	filter := workflow.CatalogFilter{Query: strings.TrimSpace(c.search.Value())}
	if c.osIdx > 0 {
		filter.OS = btOSOptions[c.osIdx-1]
	}
	if c.archIdx > 0 {
		filter.Arch = btArchOptions[c.archIdx-1]
	}
	if c.buildIdx > 0 {
		filter.BuildType = btBuildOptions[c.buildIdx-1]
	}
	if c.compIdx > 0 && c.compIdx < len(c.compilerOptions) {
		filter.Compiler = c.compilerOptions[c.compIdx]
	}
	if c.qtIdx > 0 && c.qtIdx < len(c.qtOptions) {
		filter.Qt = c.qtOptions[c.qtIdx]
	}
	return m.run(btJobCatalog, func(ctx context.Context) (workflow.Report, error) {
		return m.api.CatalogFilter(ctx, filter)
	})
}

func (c *btCatalogModel) applyReport(report workflow.Report, err error) {
	if err != nil || !report.OK {
		c.message = fallback(report.Error, "查询失败")
		return
	}
	c.packages = btParseCatalog(report)
	c.message = fallback(report.Message, "")
	c.loaded = true
	c.compilerOptions = c.deriveOptions(func(binary workflow.CatalogPackageBinary) string { return binary.Compiler })
	c.qtOptions = c.deriveOptions(func(binary workflow.CatalogPackageBinary) string { return binary.QtVersion })
	if c.cursor >= len(c.packages) {
		c.cursor = max(0, len(c.packages)-1)
	}
	if c.expanded >= len(c.packages) {
		c.expanded = -1
	}
}

func (c *btCatalogModel) deriveOptions(pick func(workflow.CatalogPackageBinary) string) []string {
	seen := map[string]bool{}
	var options []string
	for _, pkg := range c.packages {
		for _, binary := range pkg.Binaries {
			if value := strings.TrimSpace(pick(binary)); value != "" && !seen[value] {
				seen[value] = true
				options = append(options, value)
			}
		}
	}
	sort.Strings(options)
	return append([]string{"全部"}, options...)
}

func (m *btModel) catalogView(layout *btLines) {
	c := &m.catalog
	searchLine := "  " + c.search.View()
	if c.focus == 0 {
		searchLine += "  " + btStyleHint.Render("Enter 查询 · Tab 到过滤器")
	}
	layout.regionRow("catsearch")
	layout.add(searchLine)

	// 过滤条件用 chip 呈现，与全局徽章语言一致。
	filter := func(label, value string, row int) string {
		chip := btStyleChip.Render(" " + label + " " + value + " ")
		if c.focus == 1 && c.filterRow == row {
			return btStyleFocus.Render("▸") + btStyleChipOn.Render(" "+label+" "+value+" ") + btStyleHint.Render(" ◀▶")
		}
		return " " + chip
	}
	compValue, qtValue := "全部", "全部"
	if c.compIdx > 0 && c.compIdx < len(c.compilerOptions) {
		compValue = c.compilerOptions[c.compIdx]
	}
	if c.qtIdx > 0 && c.qtIdx < len(c.qtOptions) {
		qtValue = c.qtOptions[c.qtIdx]
	}
	osValue, archValue, buildValue := "全部", "全部", "全部"
	if c.osIdx > 0 {
		osValue = config.DisplayOS(btOSOptions[c.osIdx-1])
	}
	if c.archIdx > 0 {
		archValue = btArchOptions[c.archIdx-1]
	}
	if c.buildIdx > 0 {
		buildValue = btBuildOptions[c.buildIdx-1]
	}
	filterCells := []string{
		filter("系统", osValue, 0),
		filter("架构", archValue, 1),
		filter("构建", buildValue, 2),
		filter("编译器", compValue, 3),
		filter("Qt", qtValue, 4),
	}
	addFilterLine := func(cells []string, rows []int) {
		y := len(layout.lines)
		layout.add(strings.Join(cells, " "))
		x := 0
		for i, cell := range cells {
			if i > 0 {
				x++
			}
			w := displayWidth(cell)
			layout.regions = append(layout.regions, btRegion{
				id: "catfilter:" + itoa(rows[i]),
				x:  x, y: y, w: max(1, w), h: 1,
			})
			x += w
		}
	}
	// 过滤行放不下时拆成两行，避免折行把鼠标区域顶偏。
	joined := strings.Join(filterCells, " ")
	if displayWidth(joined) <= layout.width {
		addFilterLine(filterCells, []int{0, 1, 2, 3, 4})
	} else {
		addFilterLine(filterCells[:3], []int{0, 1, 2})
		addFilterLine(filterCells[3:], []int{3, 4})
	}

	if !c.loaded {
		layout.add(btStyleMuted.Render("  输入包名回车查询；空查询依赖 Nexus 全量接口。"))
		return
	}
	if len(c.packages) == 0 {
		layout.add(btStyleMuted.Render("  仓库里没有匹配的组件：换个包名或放宽过滤条件再查。"))
		if c.message != "" {
			layout.add(btStyleHint.Render("  " + c.message))
		}
		return
	}
	if c.message != "" {
		layout.add(btStyleLabel.Render("  " + c.message))
	}

	layout.add("")
	layout.card("组件 · "+itoa(len(c.packages))+" 个", func(l *btLines) {
		for index, pkg := range c.packages {
			selected := index == c.cursor
			expanded := index == c.expanded
			caret := "▸"
			if expanded {
				caret = "▾"
			}
			count := len(pkg.Binaries)
			line := "  " + caret + " " + btStyleMono.Render(pkg.Name) + btStyleMuted.Render(
				"  "+itoa(len(pkg.Versions))+" 个版本 · "+itoa(count)+" 条制品")
			if count == 0 {
				line += btStyleHint.Render("（无二进制信息）")
			}
			l.addRow("catpkg:"+itoa(index), selected, line)
			if expanded {
				m.catalogBinariesView(l, pkg)
			}
		}
	})
}

func (m *btModel) catalogBinariesView(layout *btLines, pkg workflow.CatalogPackage) {
	if len(pkg.Binaries) == 0 {
		layout.add(btStyleMuted.Render("    没有制品；只有配方记录。"))
		return
	}
	c := &m.catalog
	layout.add("    " + btStyleLabel.Render(padCell("版本", 12)+padCell("系统", 10)+padCell("架构", 8)+padCell("编译器", 16)+padCell("Qt", 8)+"构建"))
	for index, binary := range pkg.Binaries {
		selected := index == c.binCursor
		qt := binary.QtVersion
		if binary.NoQt {
			qt = "无"
		}
		line := "    " + btStyleMono.Render(
			padCell(fallback(binary.Version, "-"), 12)+
				padCell(fallback(config.DisplayOS(binary.OS), "-"), 10)+
				padCell(fallback(binary.Arch, "-"), 8)+
				padCell(fallback(strings.TrimSpace(binary.Compiler+" "+binary.CompilerVersion), "-"), 16)+
				padCell(fallback(qt, "-"), 8)+
				fallback(binary.BuildType, "-"))
		layout.addRow("catbin:"+itoa(index), selected, line)
	}
}
