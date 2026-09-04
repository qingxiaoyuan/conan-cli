package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"conan-cli/internal/config"
	"conan-cli/internal/platform"
	"conan-cli/internal/workflow"
)

// Run starts the terminal dashboard. It deliberately uses only ANSI escape
// sequences and the standard library so the TUI remains easy to build and
// works in both a real terminal and a piped/non-interactive environment.
// Run uses cursor navigation when attached to a real terminal. Piped input
// keeps the line-oriented fallback so the CLI remains scriptable and easy to
// test without a terminal.
func Run(ctx context.Context, app *workflow.App, in io.Reader, out io.Writer) error {
	if isInteractiveTerminal(in, out) {
		return runCursor(ctx, app, in, out)
	}
	return runLine(ctx, app, in, out)
}

func runLine(ctx context.Context, app *workflow.App, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	ui := newUI(app, out, supportsANSI(out))
	ui.enter()
	defer ui.leave()
	ui.refreshProject()

	for {
		ui.render()
		choice, ok := ui.command(reader, "选择操作 [1-8 / r / q] > ")
		if !ok {
			return nil
		}

		switch strings.ToLower(choice) {
		case "q", "quit", "exit", "0":
			return nil
		case "1":
			ui.runReport(reader, "正在初始化 / 刷新项目", func() (workflow.Report, error) { return app.Init(ctx) })
		case "2":
			ui.runReport(reader, "正在扫描本机 Qt 和编译器", func() (workflow.Report, error) { return app.Scan(ctx, false) })
		case "3":
			ui.analyzeScreen(ctx, reader)
		case "4":
			ui.installScreen(ctx, reader)
		case "5":
			ui.publishScreen(ctx, reader)
		case "6":
			ui.settingsScreen(ctx, reader)
		case "7":
			ui.doctorScreen(ctx, reader)
		case "8":
			ui.dependenciesScreen(ctx, reader)
		case "r", "refresh":
			ui.refreshProject()
			ui.lastMessage = "项目状态已刷新"
		default:
			ui.lastMessage = "无法识别该操作，请输入菜单编号、r 或 q"
		}
	}
}

type dashboard struct {
	app         *workflow.App
	out         io.Writer
	ansi        bool
	canvasWidth int
	outerWidth  int
	outerInset  int
	panelWidth  int
	panelInset  int
	project     *config.Project
	projectErr  error
	global      *config.Global
	globalErr   error
	lastReport  *workflow.Report
	lastAnalyze *workflow.Report
	lastMessage string
}

func newUI(app *workflow.App, out io.Writer, ansi bool) *dashboard {
	return &dashboard{app: app, out: out, ansi: ansi, canvasWidth: innerWidth, outerWidth: innerWidth, panelWidth: innerWidth}
}

func (ui *dashboard) setCursorLayout(columns int) {
	if columns < 1 {
		columns = innerWidth
	}
	ui.canvasWidth = columns
	ui.outerWidth = max(40, min(columns-2, 96))
	ui.outerInset = max(0, (columns-ui.outerWidth)/2)
	ui.panelWidth = max(36, ui.outerWidth-4)
	ui.panelInset = max(0, (ui.outerWidth-ui.panelWidth)/2)
}

func (ui *dashboard) refreshProject() {
	ui.project, ui.projectErr = ui.app.Project()
	ui.global, ui.globalErr = config.LoadGlobal()
	if ui.global == nil {
		ui.global = &config.Global{}
	}
}

func (ui *dashboard) render() {
	ui.clear()

	projectName := "未初始化项目"
	if ui.project != nil && ui.project.Name != "" {
		projectName = ui.project.Name
	}
	status := "未初始化"
	if ui.projectErr == nil {
		status = "就绪"
	}
	if ui.lastReport != nil && !ui.lastReport.OK {
		status = "需要处理"
	}

	ui.terminalHeader("")
	ui.boxSeparator("┌", "┐")
	ui.boxRow(ui.paint("  CONAN CLI  ·  项目工作台", "36") + "  " + ui.paint(status, statusColor(status)))
	ui.boxSeparator("├", "┤")
	ui.boxRow("  项目 " + padCell(projectName, 18) + "  构建 " + padCell(value(ui.project, func(p *config.Project) string { return p.BuildSystem }), 9) + "  channel " + padCell(value(ui.project, func(p *config.Project) string { return p.Channel }), 9))
	ui.boxRow("  平台 " + padCell(platformValue(ui.project), 18) + "  Qt " + padCell(value(ui.project, func(p *config.Project) string { return p.QtVersion }), 8) + "  编译器 " + padCell(value(ui.project, func(p *config.Project) string { return p.Compiler.Display() }), 12))
	remote := value(ui.project, func(p *config.Project) string { return p.Remote })
	if remote == "-" && ui.global != nil {
		remote = fallback(ui.global.Nexus.Name, "未配置")
	}
	login := "未登录"
	if ui.global != nil && ui.global.View().HasPassword {
		login = "已登录"
	}
	ui.boxRow("  远程 " + padCell(remote, 18) + "  登录 " + padCell(login, 9) + "  依赖 " + padCell(dependencyCount(ui.project), 8))
	ui.boxSeparator("├", "┤")

	ui.boxRow("  快捷操作")
	menu := []string{
		"1  初始化 / 刷新     2  扫描         3  依赖分析       4  下载",
		"5  发布表单          6  设置         7  诊断           8  依赖管理",
		"r  刷新项目状态      q  退出",
	}
	for _, item := range menu {
		ui.boxRow("  " + item)
	}
	ui.boxSeparator("├", "┤")

	ui.boxRow("  最近状态")
	if ui.lastReport == nil && ui.lastMessage == "" {
		ui.boxRow("  暂无操作，选择上方菜单开始")
	} else if ui.lastMessage != "" {
		ui.boxRow("  " + clip(ui.lastMessage, innerWidth-2))
	}
	if ui.lastAnalyze != nil {
		if warning := firstDependencyWarning(ui.lastAnalyze); warning != "" {
			ui.boxRow("  " + ui.paint("! "+warning, "33"))
		} else {
			ui.boxRow("  " + ui.paint("✓ 最近依赖分析通过", "32"))
		}
	}
	if ui.lastReport != nil {
		state := ui.paint("成功", "32")
		if !ui.lastReport.OK {
			state = ui.paint("失败", "31")
		}
		ui.boxRow("  " + state + "  " + clip(localAction(ui.lastReport.Action), 58))
		for _, check := range ui.lastReport.Checks {
			checkState := ui.paint("✓", "32")
			if !check.OK {
				checkState = ui.paint("×", "31")
			}
			ui.boxRow("    " + checkState + " " + padCell(localCheck(check.Name), 20) + " " + clip(check.Detail, 43))
		}
	}
	if ui.projectErr != nil && ui.lastReport == nil {
		ui.boxRow("  " + clip("提示："+localMessage(ui.projectErr.Error()), innerWidth-2))
	}
	ui.boxSeparator("└", "┘")
}

const innerWidth = 78

const (
	ansiPageBackground     = "\033[48;2;6;7;8m"
	ansiCardBackground     = "\033[48;2;11;12;14m"
	ansiDefaultForeground  = "\033[38;2;212;212;216m"
	ansiCardForeground     = "\033[38;2;161;161;170m"
	ansiBorder             = "\033[38;2;52;211;153m"
	ansiSelectedBackground = "\033[48;2;52;211;153m"
	ansiSelectedForeground = "\033[38;2;6;7;8m"
)

func (ui *dashboard) boxRow(content string) {
	// Most rows are assembled from clipped fields. Keep the final boundary
	// stable even when a remote URL or Conan error contains a long line.
	width := ui.panelWidth
	if width <= 0 {
		width = innerWidth
	}
	content = clip(content, width)
	prefix := strings.Repeat(" ", ui.outerInset+ui.panelInset)
	if ui.ansi {
		fmt.Fprintf(ui.out, "%s%s%s%s│%s%s│%s%s\n", ansiPageBackground, prefix, ansiCardBackground, ansiBorder, ansiCardForeground, padCells(content, width), ansiBorder, ansiPageBackground+ansiDefaultForeground)
		return
	}
	fmt.Fprintf(ui.out, "%s│%s│\n", prefix, padCells(content, width))
}

func (ui *dashboard) boxSeparator(left, right string) {
	width := ui.panelWidth
	if width <= 0 {
		width = innerWidth
	}
	line := strings.Repeat("─", width)
	prefix := strings.Repeat(" ", ui.outerInset+ui.panelInset)
	if ui.ansi {
		fmt.Fprintf(ui.out, "%s%s%s%s%s%s%s%s\n", ansiPageBackground, prefix, ansiCardBackground, ansiBorder, left, line, right, ansiPageBackground+ansiDefaultForeground)
		return
	}
	fmt.Fprintln(ui.out, prefix+left+line+right)
}

func (ui *dashboard) terminalHeader(title string) {
	label := "conan-cli tui"
	if title != "" {
		label += "  ·  " + title
	}
	label = center(label, ui.outerWidth)
	prefix := strings.Repeat(" ", ui.outerInset)
	if ui.ansi {
		fmt.Fprintf(ui.out, "%s%s%s%s\n", ansiPageBackground, prefix, ui.paint(label, "90"), ansiPageBackground+ansiDefaultForeground)
		fmt.Fprintf(ui.out, "%s%s%s%s\n", ansiPageBackground, prefix, ui.paint(strings.Repeat("─", ui.outerWidth), "90"), ansiPageBackground+ansiDefaultForeground)
		return
	}
	fmt.Fprintln(ui.out, prefix+label)
	fmt.Fprintln(ui.out, prefix+strings.Repeat("─", ui.outerWidth))
}

func (ui *dashboard) clear() {
	if ui.ansi {
		fmt.Fprint(ui.out, ansiPageBackground+ansiDefaultForeground+"\033[2J\033[H")
	}
}

func (ui *dashboard) enter() {
	if ui.ansi {
		fmt.Fprint(ui.out, "\033[?1049h\033[?25l"+ansiPageBackground+ansiDefaultForeground)
	}
}

func (ui *dashboard) leave() {
	if ui.ansi {
		fmt.Fprint(ui.out, "\033[0m\033[?25h\033[?1049l")
	}
}

func (ui *dashboard) cursor(visible bool) {
	if !ui.ansi {
		return
	}
	if visible {
		fmt.Fprint(ui.out, "\033[?25h")
	} else {
		fmt.Fprint(ui.out, "\033[?25l")
	}
}

func (ui *dashboard) screen(title string) {
	ui.clear()
	ui.terminalHeader(title)
	ui.boxSeparator("┌", "┐")
	ui.boxSeparator("├", "┤")
}

func (ui *dashboard) cursorScreen(title string) {
	ui.clear()
	ui.terminalHeader(title)
	ui.blankOuterLine()
	ui.boxSeparator("┌", "┐")
}

func (ui *dashboard) blankOuterLine() {
	if ui.ansi {
		fmt.Fprintf(ui.out, "%s%s\n", ansiPageBackground, strings.Repeat(" ", ui.canvasWidth))
		return
	}
	fmt.Fprintln(ui.out)
}

func (ui *dashboard) cursorHint(message string) {
	prefix := strings.Repeat(" ", ui.outerInset+ui.panelInset)
	if ui.ansi {
		fmt.Fprintf(ui.out, "%s%s%s%s\n", ansiPageBackground, prefix, ui.paint(message, "90"), ansiPageBackground+ansiDefaultForeground)
		return
	}
	fmt.Fprintln(ui.out, prefix+message)
}

func (ui *dashboard) screenEnd() {
	ui.boxRow("  Enter 返回上一级   q 返回控制台")
	ui.boxSeparator("└", "┘")
}

func (ui *dashboard) runReport(reader *bufio.Reader, message string, action func() (workflow.Report, error)) {
	ui.lastMessage = message
	ui.render()
	fmt.Fprintln(ui.out)
	report, err := action()
	ui.storeReport(report, err)
	ui.lastMessage = ""
	ui.refreshProject()
	printReport(ui.out, report, err, ui.ansi)
	ui.pause(reader)
}

func (ui *dashboard) storeReport(report workflow.Report, err error) {
	if report.Action == "" {
		report.Action = "command"
	}
	if err != nil && report.OK {
		report.OK = false
	}
	ui.lastReport = &report
}

func (ui *dashboard) pause(reader *bufio.Reader) {
	if !ui.ansi {
		return
	}
	ui.cursor(true)
	fmt.Fprint(ui.out, "\n  按回车返回控制台...")
	_, _ = reader.ReadString('\n')
	ui.cursor(false)
}

func (ui *dashboard) command(reader *bufio.Reader, label string) (string, bool) {
	ui.cursor(true)
	defer ui.cursor(false)
	fmt.Fprintf(ui.out, "\n  %s", label)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", false
	}
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" && strings.Contains(line, "\t") {
		return "tab", true
	}
	return strings.ToLower(strings.TrimSpace(line)), true
}

func (ui *dashboard) ask(reader *bufio.Reader, label string) (string, bool) {
	ui.cursor(true)
	defer ui.cursor(false)
	fmt.Fprintf(ui.out, "\n  %s：", label)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(line)
	if value == "" {
		ui.lastMessage = "输入不能为空"
		return "", false
	}
	return value, true
}

func (ui *dashboard) askDefault(reader *bufio.Reader, label, defaultValue string) (string, bool) {
	ui.cursor(true)
	defer ui.cursor(false)
	if defaultValue != "" {
		label += " [" + defaultValue + "]"
	}
	fmt.Fprintf(ui.out, "\n  %s：", label)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return defaultValue, true
	}
	return value, true
}

func (ui *dashboard) selectConsumePlatform(reader *bufio.Reader) bool {
	return ui.updateConsumePlatform(reader, true, true)
}

func (ui *dashboard) editConsumePlatform(reader *bufio.Reader, changeOS, changeArch bool) bool {
	return ui.updateConsumePlatform(reader, changeOS, changeArch)
}

func (ui *dashboard) updateConsumePlatform(reader *bufio.Reader, changeOS, changeArch bool) bool {
	project := ui.project
	if project == nil {
		project = config.NewProject(ui.app.Dir)
	}
	osName := project.Platform.Consume.OS
	arch := project.Platform.Consume.Arch
	if changeOS {
		var ok bool
		osName, ok = ui.askDefault(reader, "操作系统 windows / linux / kylin", osName)
		if !ok {
			return false
		}
		osName = config.NormalizeOS(osName)
		if !config.ValidOS(osName) {
			ui.lastMessage = "不支持的操作系统，请使用 windows、linux 或 kylin"
			return false
		}
	}
	if changeArch {
		var ok bool
		arch, ok = ui.askDefault(reader, "架构 x86 / x64 / arm / arm64", arch)
		if !ok {
			return false
		}
		arch = config.NormalizeArch(arch)
		if !config.ValidArch(arch) {
			ui.lastMessage = "不支持的架构，请使用 x86、x64、arm 或 arm64"
			return false
		}
	}
	input := workflow.ProjectSettingsInput{}
	if changeOS {
		input.OS = osName
	}
	if changeArch {
		input.Arch = arch
	}
	buildType := project.Platform.Consume.BuildType
	if buildType == "" {
		buildType = config.BuildTypeRelease
	}
	buildType, ok := ui.askDefault(reader, "构建类型 Debug / Release", buildType)
	if !ok {
		return false
	}
	buildType = config.NormalizeBuildType(buildType)
	if !config.ValidBuildType(buildType) {
		ui.lastMessage = "不支持的构建类型，请使用 Debug 或 Release"
		return false
	}
	input.BuildType = buildType
	report, err := ui.app.SaveProjectSettings(input)
	ui.storeReport(report, err)
	ui.refreshProject()
	return err == nil
}

func (ui *dashboard) installScreen(ctx context.Context, reader *bufio.Reader) {
	ui.refreshProject()
	if ui.project == nil || ui.projectErr != nil {
		ui.screen("下载")
		ui.boxRow("  项目尚未初始化，先执行初始化并生成最简 conanfile.txt")
		if choice, ok := ui.command(reader, "按 Enter 初始化，q 返回 > "); ok && choice != "q" {
			report, err := ui.app.Init(ctx)
			ui.storeReport(report, err)
			ui.refreshProject()
		} else {
			return
		}
	}
	if ui.project == nil {
		return
	}
	if missingPlatform(ui.project.Platform.Consume) {
		ui.screen("下载")
		ui.boxRow("  选择制品要运行的目标平台（与当前开发机无关）")
		if !ui.selectConsumePlatform(reader) {
			ui.screenEnd()
			return
		}
	}
	ui.refreshProject()
	project := ui.project
	settings := platform.Resolve(project.Platform.Consume, project.Compiler, project.QtVersion)
	ui.screen("下载")
	ui.boxRow("  将下载 " + project.Platform.Consume.Display() + " 的已有二进制")
	ui.boxRow("  Conan os=" + fallback(settings.ConanOS, "-") + "  arch=" + fallback(settings.ConanArch, "-"))
	ui.boxRow("  编译器 " + fallback(project.Compiler.Display(), "未设置") + "  Qt " + fallback(project.QtVersion, "未设置"))
	ui.boxRow("  策略 download-only：找不到匹配制品时不会在本机编译")
	ui.screenEnd()
	choice, ok := ui.command(reader, "按 Enter / y 开始下载，q 取消 > ")
	if !ok || choice == "q" || choice == "n" || choice == "no" {
		return
	}
	report, err := ui.app.InstallPlatform(ctx, workflow.InstallRequest{})
	ui.storeReport(report, err)
	ui.refreshProject()
	printReport(ui.out, report, err, ui.ansi)
	ui.pause(reader)
}

func (ui *dashboard) analyzeScreen(ctx context.Context, reader *bufio.Reader) {
	ui.runAnalyze(ctx)
	for {
		ui.renderAnalyze()
		choice, ok := ui.command(reader, "分析操作 [r 刷新 / o 改系统 / a 改架构 / d 下载 / q 返回] > ")
		if !ok || choice == "q" || choice == "" {
			return
		}
		switch choice {
		case "r", "refresh":
			ui.runAnalyze(ctx)
		case "o":
			if ui.editConsumePlatform(reader, true, false) {
				ui.runAnalyze(ctx)
			}
		case "a":
			if ui.editConsumePlatform(reader, false, true) {
				ui.runAnalyze(ctx)
			}
		case "platform":
			if ui.selectConsumePlatform(reader) {
				ui.runAnalyze(ctx)
			}
		case "d", "download":
			ui.installScreen(ctx, reader)
			ui.runAnalyze(ctx)
		case "add":
			ui.addDependency(reader)
			ui.runAnalyze(ctx)
		default:
			ui.lastMessage = "请输入 r、o、a、d 或 q"
		}
	}
}

func (ui *dashboard) runAnalyze(ctx context.Context) {
	report, err := ui.app.Analyze(ctx, "", "", "")
	ui.storeReport(report, err)
	ui.lastAnalyze = &report
	ui.lastMessage = ""
	ui.refreshProject()
}

func (ui *dashboard) renderAnalyze() {
	report := workflow.Report{}
	if ui.lastAnalyze != nil {
		report = *ui.lastAnalyze
	}
	data := reportMap(report)
	ui.screen("依赖分析")
	platformName := stringValue(data["platform"])
	if platformName == "" {
		platformName = "未选择目标平台"
	}
	ui.boxRow("  分析 · " + platformName + "  · 远程 " + fallback(stringValue(data["remote"]), "未配置"))
	ui.boxRow("  Conan settings：" + settingsSummary(mapValue(data["conan_settings"])))
	ui.boxSeparator("├", "┤")
	rows := dependencyRows(report)
	if len(rows) == 0 {
		ui.boxRow("  暂无直接依赖。可在依赖管理中添加包，或先初始化项目")
	}
	for _, row := range rows {
		glyph := ui.paint("✓", "32")
		if row.Status != "found" && row.Status != "unknown" {
			glyph = ui.paint("×", "33")
		}
		state := fallback(dependencyStatus(row.Status), fallback(row.Status, "未查询"))
		ui.boxRow("  " + glyph + " " + padCell(row.Reference, 27) + " " + padCell(state, 16) + " " + clip(row.Detail, 27))
	}
	if dynamic, ok := data["dynamic_recipe"].(bool); ok && dynamic {
		ui.boxRow("  " + ui.paint("提示：动态 requirements()，已跳过配方逐项对比", "33"))
	}
	if report.Message != "" {
		ui.boxRow("  " + clip(localMessage(report.Message), innerWidth-2))
	}
	ui.screenEnd()
}

func (ui *dashboard) dependenciesScreen(ctx context.Context, reader *bufio.Reader) {
	ui.runAnalyze(ctx)
	for {
		ui.renderAnalyze()
		choice, ok := ui.command(reader, "依赖操作 [a 添加 / s 搜索 / r 刷新 / d 下载 / q 返回] > ")
		if !ok || choice == "q" || choice == "" {
			return
		}
		switch choice {
		case "a", "add":
			ui.addDependency(reader)
			ui.runAnalyze(ctx)
		case "s", "search":
			ui.searchDependencies(ctx, reader)
		case "r", "refresh":
			ui.runAnalyze(ctx)
		case "d", "download":
			ui.installScreen(ctx, reader)
			ui.runAnalyze(ctx)
		default:
			ui.lastMessage = "请输入 a、s、r、d 或 q"
		}
	}
}

func (ui *dashboard) addDependency(reader *bufio.Reader) {
	dependency, ok := ui.ask(reader, "包引用（例如 fmt/10.2.1）")
	if !ok {
		return
	}
	report, err := ui.app.Add(dependency)
	ui.storeReport(report, err)
	ui.refreshProject()
}

func (ui *dashboard) searchDependencies(ctx context.Context, reader *bufio.Reader) {
	query, ok := ui.ask(reader, "搜索包名或版本（例如 fmt/*）")
	if !ok {
		return
	}
	report, err := ui.app.Search(ctx, query, "")
	ui.storeReport(report, err)
	ui.screen("搜索远程仓库")
	if err != nil {
		printReport(ui.out, report, err, ui.ansi)
	} else if data, marshalErr := json.MarshalIndent(report.Data, "", "  "); marshalErr == nil {
		printOutput(ui.out, string(data))
	}
	ui.screenEnd()
	ui.pause(reader)
}

func (ui *dashboard) settingsScreen(ctx context.Context, reader *bufio.Reader) {
	ui.refreshProject()
	scope := &settingsScope{global: ui.global, project: ui.project}
	if scope.global == nil {
		scope.global = &config.Global{}
	}
	if scope.project == nil {
		scope.project = config.NewProject(ui.app.Dir)
	}
	active := "global"

	for {
		ui.renderSettings(active, scope)
		choice, ok := ui.command(reader, "设置操作 [g 全局 / p 项目 / s 保存 / t 测试 / l 保存并登录 / q 返回] > ")
		if !ok || choice == "q" || choice == "" {
			return
		}
		switch choice {
		case "g", "global":
			active = "global"
		case "p", "project":
			active = "project"
		case "tab":
			active = toggleSettingsTab(active)
		case "s", "save":
			if active == "global" {
				report, err := ui.saveGlobal(ctx, scope.global, scope.pendingPassword)
				ui.storeReport(report, err)
				if err == nil {
					scope.pendingPassword = ""
				}
			} else {
				report, err := ui.saveProject(scope.project)
				ui.storeReport(report, err)
			}
			ui.refreshProject()
			if ui.global != nil {
				scope.global = ui.global
			}
			if ui.project != nil {
				scope.project = ui.project
			}
		case "l", "login":
			if active != "global" {
				ui.lastMessage = "请切换到全局设置后执行保存并登录"
				continue
			}
			report, err := ui.saveGlobal(ctx, scope.global, scope.pendingPassword)
			ui.storeReport(report, err)
			if err == nil {
				loginReport, loginErr := ui.app.ConfigLogin(ctx, "")
				ui.storeReport(loginReport, loginErr)
				if loginErr == nil {
					scope.pendingPassword = ""
				}
			}
			ui.refreshProject()
			if ui.global != nil {
				scope.global = ui.global
			}
		case "t", "test":
			if active == "global" {
				report, err := ui.app.ConfigTest(ctx)
				ui.storeReport(report, err)
			} else {
				ui.lastMessage = "仓库连接测试在全局设置中执行"
			}
		default:
			fields := settingsFieldsFor(active)
			if index, matched := matchSettingsField(choice, fields); matched {
				ui.editSettingsFieldLine(reader, fields[index], scope)
			} else {
				ui.lastMessage = "请先选择 g 全局或 p 项目，再输入字段编号"
			}
		}
	}
}

// editSettingsFieldLine asks for a new value and stores it through the shared
// field table (with kind-specific normalization).
func (ui *dashboard) editSettingsFieldLine(reader *bufio.Reader, field settingsField, scope *settingsScope) {
	defaultValue := ""
	if field.kind != fieldSecret {
		defaultValue = field.get(scope)
	}
	value, ok := ui.askDefault(reader, field.editHint, defaultValue)
	if !ok {
		return
	}
	if field.kind == fieldSecret && value == "" {
		return
	}
	field.apply(scope, value)
}

func (ui *dashboard) renderSettings(active string, scope *settingsScope) {
	title := "项目"
	if active == "global" {
		title = "全局"
	}
	ui.screen("设置 · " + title)
	ui.boxRow("  当前页：" + ui.paint("g 全局", ternary(active == "global", "32", "0")) + " / " + ui.paint("p 项目", ternary(active == "project", "32", "0")))
	ui.boxSeparator("├", "┤")
	if active == "global" {
		ui.boxRow("  全局设置（保存在本机，不进入项目 git）")
	} else {
		ui.boxRow("  项目设置（写入 .conan-cli/project.yaml）")
	}
	for index, field := range settingsFieldsFor(active) {
		row := fmt.Sprintf("  %2d %s%s", index+1, padCell(field.label, 13), field.displayValue(scope))
		if field.note != "" {
			row += field.note
		}
		ui.boxRow(row)
	}
	if active == "global" {
		ui.boxRow("  s 保存   t 测试连接   l 保存并登录")
	} else {
		ui.boxRow("  缺二进制策略    download-only（固定）")
		ui.boxRow("  s 保存项目设置")
	}
	ui.screenEnd()
}

func (ui *dashboard) saveGlobal(ctx context.Context, global *config.Global, pendingPassword string) (workflow.Report, error) {
	return ui.app.SaveGlobalSettings(ctx, workflow.GlobalSettingsInput{
		Name: global.Nexus.Name, URL: global.Nexus.URL, Username: global.Nexus.Username,
		Password: pendingPassword, ConanBin: global.ConanBin,
	})
}

func (ui *dashboard) saveProject(project *config.Project) (workflow.Report, error) {
	return ui.app.SaveProjectSettings(workflow.ProjectSettingsInput{
		Name: project.Name, QtVersion: project.QtVersion,
		CompilerID: project.Compiler.ID, CompilerVersion: project.Compiler.Version,
		OS: project.Platform.Consume.OS, Arch: project.Platform.Consume.Arch,
		BuildType: project.Platform.Consume.BuildType,
		PublishOS: project.Platform.Publish.OS, PublishArch: project.Platform.Publish.Arch,
		PublishBuildType: project.Platform.Publish.BuildType,
		Channel:          project.Channel, Remote: project.Remote, BuildSystem: project.BuildSystem,
		OutputFolder: project.OutputFolder,
		LibDirs:      specLibDirs(project), HasLibDirs: len(project.Packages) > 0,
		IncludeDirs: specIncludeDirs(project), HasIncludeDirs: len(project.Packages) > 0,
	})
}

func specLibDirs(project *config.Project) []string {
	if project == nil {
		return nil
	}
	return project.PrimaryPackage().LibDirs
}

func specIncludeDirs(project *config.Project) []string {
	if project == nil {
		return nil
	}
	return project.PrimaryPackage().IncludeDirs
}

func (ui *dashboard) publishScreen(ctx context.Context, reader *bufio.Reader) {
	ui.refreshProject()
	if ui.project == nil || ui.projectErr != nil {
		ui.screen("发布表单")
		ui.boxRow("  项目尚未初始化，请先返回执行 1 初始化 / 刷新")
		ui.screenEnd()
		ui.pause(reader)
		return
	}

	name, version := ui.project.Name, ""
	if metadata, _, err := ui.app.Client.Inspect(ctx); err == nil {
		if name == "" {
			name = stringValue(metadata["name"])
		}
		version = stringValue(metadata["version"])
	}
	project := ui.project
	pkgID := ""
	listed := project.ListPackages()
	if len(listed) > 1 {
		var labels []string
		for _, spec := range listed {
			labels = append(labels, spec.Name)
		}
		ui.screen("发布表单")
		ui.boxRow("  组件：" + strings.Join(labels, ", "))
		ui.screenEnd()
		var ok bool
		pkgID, ok = ui.askDefault(reader, "发布哪个组件", listed[0].Name)
		if !ok {
			return
		}
	} else if len(listed) == 1 {
		pkgID = listed[0].Name
	}
	if spec, _, ok := project.FindPackage(pkgID); ok {
		if spec.Name != "" {
			name = spec.Name
		}
		if spec.Version != "" {
			version = spec.Version
		}
	}
	var ok bool
	name, ok = ui.askDefault(reader, "Conan 包名", name)
	if !ok {
		return
	}
	version, ok = ui.askDefault(reader, "版本（写入该组件发布配方）", version)
	if !ok {
		return
	}
	qtDefault := project.QtVersion
	noQt := false
	if spec, _, found := project.FindPackage(pkgID); found {
		if spec.NoQt {
			qtDefault = ""
			noQt = true
		} else if spec.QtVersion != "" {
			qtDefault = spec.QtVersion
		}
	}
	qt, ok := ui.askDefault(reader, "Qt 版本（不依赖 Qt 填 -）", qtDefault)
	if !ok {
		return
	}
	if qt == "-" || strings.EqualFold(qt, "none") || strings.TrimSpace(qt) == "" {
		qt = ""
		noQt = true
	}
	channel, ok := ui.askDefault(reader, "channel", project.Channel)
	if !ok {
		return
	}
	publishOS := project.Platform.Publish.OS
	if publishOS == "" {
		publishOS = project.Platform.Consume.OS
	}
	publishOS, ok = ui.askDefault(reader, "发布系统 windows / linux / kylin", publishOS)
	if !ok {
		return
	}
	publishOS = config.NormalizeOS(publishOS)
	if !config.ValidOS(publishOS) {
		ui.lastMessage = "不支持的发布系统"
		return
	}
	publishArch := project.Platform.Publish.Arch
	if publishArch == "" {
		publishArch = project.Platform.Consume.Arch
	}
	publishArch, ok = ui.askDefault(reader, "发布架构 x86 / x64 / arm / arm64", publishArch)
	if !ok {
		return
	}
	publishArch = config.NormalizeArch(publishArch)
	if !config.ValidArch(publishArch) {
		ui.lastMessage = "不支持的发布架构"
		return
	}
	publishBT := project.Platform.Publish.BuildType
	if publishBT == "" {
		publishBT = project.Platform.Consume.BuildType
	}
	if publishBT == "" {
		publishBT = config.BuildTypeRelease
	}
	publishBT, ok = ui.askDefault(reader, "构建类型 Debug / Release", publishBT)
	if !ok {
		return
	}
	publishBT = config.NormalizeBuildType(publishBT)
	if !config.ValidBuildType(publishBT) {
		ui.lastMessage = "不支持的构建类型，请使用 Debug 或 Release"
		return
	}
	remote := project.Remote
	if remote == "" && ui.global != nil {
		remote = ui.global.Nexus.Name
	}
	remote, ok = ui.askDefault(reader, "远程仓库", remote)
	if !ok {
		return
	}
	note, ok := ui.askDefault(reader, "备注（可留空）", "")
	if !ok {
		return
	}
	if name == "" || version == "" {
		ui.lastMessage = "包名和版本不能为空"
		return
	}

	// Keep the selected publish target in the project just like the VS Code
	// form does, while the note remains a one-off publish field.
	if report, err := ui.app.SaveProjectSettings(workflow.ProjectSettingsInput{
		PublishOS: publishOS, PublishArch: publishArch, PublishBuildType: publishBT, Channel: channel, Remote: remote,
	}); err != nil {
		ui.storeReport(report, err)
		return
	}

	request := workflow.PublishRequest{
		Name: name, Version: version, Channel: channel, Remote: remote, Package: pkgID,
		OS: publishOS, Arch: publishArch, BuildType: publishBT, Note: note, DryRun: true,
		Compiler: project.Compiler.ID, CompilerVersion: project.Compiler.Version, QtVersion: qt, NoQt: noQt,
	}
	preview, err := ui.app.PublishPackage(ctx, request)
	ui.storeReport(preview, err)
	ui.renderPublish(preview, err)
	if err != nil {
		ui.pause(reader)
		return
	}
	choice, ok := ui.command(reader, "Enter / y 确认发布，q 取消 > ")
	if !ok || choice == "q" || choice == "n" || choice == "no" {
		return
	}
	request.DryRun = false
	result, err := ui.app.PublishPackage(ctx, request)
	ui.storeReport(result, err)
	ui.refreshProject()
	printReport(ui.out, result, err, ui.ansi)
	ui.pause(reader)
}

func (ui *dashboard) renderPublish(report workflow.Report, err error) {
	data := reportMap(report)
	ui.screen("发布表单")
	ui.boxRow("  发布确认 · " + fallback(stringValue(data["reference"]), "未生成引用"))
	ui.boxRow("  平台 " + fallback(stringValue(data["os"]), "-") + " / " + fallback(stringValue(data["arch"]), "-"))
	ui.boxRow("  channel " + fallback(stringValue(data["channel"]), "-") + "  remote " + fallback(stringValue(data["remote"]), "未配置"))
	ui.boxRow("  Conan settings：" + settingsSummary(mapValue(data["conan_settings"])))
	if hint := stringValue(data["recipe_hint"]); hint != "" {
		ui.boxRow("  " + hint)
	}
	if command := stringValue(data["command"]); command != "" {
		ui.boxRow("  $ " + clip(command, innerWidth-4))
	}
	if err != nil {
		ui.boxRow("  " + ui.paint("发布预览失败："+localMessage(err.Error()), "31"))
	} else {
		ui.boxRow("  " + ui.paint("登录信息只从全局设置读取，密码不会出现在命令中", "32"))
	}
	ui.screenEnd()
}

func (ui *dashboard) doctorScreen(ctx context.Context, reader *bufio.Reader) {
	report, err := ui.app.Doctor(ctx)
	ui.storeReport(report, err)
	ui.renderDoctor(report, err)
	for {
		choice, ok := ui.command(reader, "诊断操作 [r 刷新 / q 返回] > ")
		if !ok || choice == "q" || choice == "" {
			return
		}
		if choice == "r" || choice == "refresh" {
			report, err = ui.app.Doctor(ctx)
			ui.storeReport(report, err)
			ui.renderDoctor(report, err)
		}
	}
}

func (ui *dashboard) renderDoctor(report workflow.Report, err error) {
	ui.screen("诊断")
	if err != nil {
		ui.boxRow("  " + ui.paint(localMessage(err.Error()), "31"))
	}
	passed := 0
	for _, check := range report.Checks {
		glyph := ui.paint("✓", "32")
		if !check.OK {
			glyph = ui.paint("×", "31")
		} else {
			passed++
		}
		ui.boxRow("  " + glyph + " " + padCell(localCheck(check.Name), 20) + " " + clip(check.Detail, 43))
	}
	ui.boxRow("  通过 " + fmt.Sprintf("%d/%d", passed, len(report.Checks)))
	ui.screenEnd()
}

func printReport(out io.Writer, report workflow.Report, err error, ansi bool) {
	if err != nil {
		fmt.Fprintf(out, "%s %s\n", paintIf(ansi, "错误", "31"), localMessage(err.Error()))
	}
	if report.Message != "" && (err == nil || report.Message != err.Error()) {
		fmt.Fprintf(out, "%s %s\n", paintIf(ansi, "信息", "36"), localMessage(report.Message))
	}
	if report.Output != "" {
		fmt.Fprintln(out, "命令输出：")
		printOutput(out, report.Output)
	}
	for _, check := range report.Checks {
		status := paintIf(ansi, "✓", "32")
		if !check.OK {
			status = paintIf(ansi, "×", "31")
		}
		fmt.Fprintf(out, "  %s %-24s %s\n", status, localCheck(check.Name), check.Detail)
	}
}

func printOutput(out io.Writer, output string) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	limit := 24
	for index, line := range lines {
		if index >= limit {
			fmt.Fprintf(out, "  …… 还有 %d 行输出\n", len(lines)-limit)
			break
		}
		fmt.Fprintf(out, "  %s\n", line)
	}
}

func supportsANSI(writer io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func (ui *dashboard) paint(value, code string) string { return paintIf(ui.ansi, value, code) }

func (ui *dashboard) highlight(value string) string {
	if !ui.ansi {
		return value
	}
	return ansiSelectedBackground + ansiSelectedForeground + value + ansiCardBackground + ansiCardForeground
}

func paintIf(ansi bool, value, code string) string {
	if !ansi || code == "0" {
		return value
	}
	colors := map[string]string{
		"31": "\033[38;2;248;113;113m", // red-400
		"32": "\033[38;2;52;211;153m",  // emerald-400
		"33": "\033[38;2;251;191;36m",  // amber-400
		"36": "\033[38;2;110;231;183m", // emerald-300
		"90": "\033[38;2;113;113;122m", // zinc-500
	}
	color, ok := colors[code]
	if !ok {
		color = "\033[" + code + "m"
	}
	return color + value + ansiDefaultForeground
}

func center(value string, width int) string {
	padding := max(0, width-displayWidth(value))
	left := padding / 2
	return strings.Repeat(" ", left) + value + strings.Repeat(" ", padding-left)
}

func value(project *config.Project, getter func(*config.Project) string) string {
	if project == nil {
		return "-"
	}
	return fallback(getter(project), "-")
}

func platformValue(project *config.Project) string {
	if project == nil {
		return "-"
	}
	return fallback(project.Platform.Consume.Display(), "未选择")
}

func dependencyCount(project *config.Project) string {
	if project == nil {
		return "-"
	}
	return fmt.Sprintf("%d 个", len(project.Dependencies))
}

func fallback(value, replacement string) string {
	if strings.TrimSpace(value) == "" {
		return replacement
	}
	return value
}

func clip(value string, width int) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\n", " ")
	if width <= 0 {
		return ""
	}
	if displayWidth(value) <= width {
		return value
	}
	var result strings.Builder
	used := 0
	for index := 0; index < len(value); {
		if value[index] == '\033' && index+1 < len(value) && value[index+1] == '[' {
			start := index
			index += 2
			for index < len(value) {
				if (value[index] >= 'a' && value[index] <= 'z') || (value[index] >= 'A' && value[index] <= 'Z') {
					index++
					break
				}
				index++
			}
			result.WriteString(value[start:index])
			continue
		}
		character, size := utf8.DecodeRuneInString(value[index:])
		characterWidth := runeWidth(character)
		if used+characterWidth > width-1 {
			break
		}
		result.WriteRune(character)
		used += characterWidth
		index += size
	}
	return result.String() + "…"
}

func padCell(value string, width int) string {
	value = clip(value, width)
	return value + strings.Repeat(" ", max(0, width-displayWidth(value)))
}

func padCells(value string, width int) string {
	return value + strings.Repeat(" ", max(0, width-displayWidth(value)))
}

func displayWidth(value string) int {
	width := 0
	for index := 0; index < len(value); {
		if value[index] == '\033' && index+1 < len(value) && value[index+1] == '[' {
			index += 2
			for index < len(value) {
				if (value[index] >= 'a' && value[index] <= 'z') || (value[index] >= 'A' && value[index] <= 'Z') {
					index++
					break
				}
				index++
			}
			continue
		}
		character, size := rune(value[index]), 1
		if character >= 0x80 {
			character, size = utf8.DecodeRuneInString(value[index:])
		}
		width += runeWidth(character)
		index += size
	}
	return width
}

func runeWidth(character rune) int {
	if character == '\t' {
		return 4
	}
	if character < 0x20 {
		return 0
	}
	// Common terminal UI glyphs are single-cell symbols even though they
	// live above the CJK code-point range used by the simple width fallback.
	if character == '▸' || character == '✓' || character == '•' {
		return 1
	}
	if character >= 0x1100 {
		return 2
	}
	return 1
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func localAction(action string) string {
	translated := map[string]string{
		"init":             "初始化 / 刷新项目",
		"scan":             "扫描 Qt 和编译器",
		"analyze":          "依赖分析",
		"install":          "下载依赖",
		"add":              "添加依赖",
		"search":           "搜索包",
		"publish":          "发布包",
		"publish-preview":  "发布预览",
		"doctor":           "运行环境诊断",
		"settings":         "查看设置",
		"settings-project": "保存项目设置",
		"config":           "保存全局设置",
		"config-login":     "登录远程仓库",
		"config-test":      "测试远程仓库",
		"profile list":     "查看配置档",
		"remote list":      "查看远程仓库",
	}[action]
	return fallback(translated, fallback(action, "未知操作"))
}

func localCheck(name string) string {
	translated := map[string]string{
		"conan":                 "Conan 可执行文件",
		"project_config":        "项目配置",
		"conanfile":             "Conan 配方文件",
		"manifest_dependencies": "依赖同步",
		"profiles":              "配置档列表",
		"remotes":               "远程仓库列表",
		"global_remote":         "全局仓库登录",
		"configured_remote":     "当前远程仓库",
		"platform":              "目标平台",
	}[name]
	return fallback(translated, fallback(name, "未知检查项"))
}

func localMessage(message string) string {
	translations := map[string]string{
		"project is not initialized; run 'conan-cli init'":                            "项目尚未初始化，请先运行 conan-cli init",
		"no remote configured; use --remote or set remote in .conan-cli/project.yaml": "未配置远程仓库，请使用 --remote 或设置项目配置",
		"project initialized":          "项目已初始化",
		"dependency added":             "依赖已添加",
		"package created and uploaded": "包已创建并上传",
		"project initialized, but Conan profile detection failed": "项目已初始化，但 Conan 配置档检测失败",
	}
	if translated, ok := translations[message]; ok {
		return translated
	}
	return message
}

func reportMap(report workflow.Report) map[string]any {
	if data, ok := report.Data.(map[string]any); ok {
		return data
	}
	return map[string]any{}
}

func mapValue(value any) map[string]string {
	result := map[string]string{}
	if raw, ok := value.(map[string]string); ok {
		return raw
	}
	if raw, ok := value.(map[string]any); ok {
		for key, item := range raw {
			result[key] = fmt.Sprint(item)
		}
	}
	return result
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func settingsSummary(settings map[string]string) string {
	keys := []string{"os", "arch", "compiler", "compiler.version", "distro"}
	var values []string
	for _, key := range keys {
		if value := settings[key]; value != "" {
			values = append(values, key+"="+value)
		}
	}
	return fallback(strings.Join(values, " "), "未设置")
}

func dependencyRows(report workflow.Report) []workflow.DependencyRow {
	data := reportMap(report)
	if rows, ok := data["dependencies"].([]workflow.DependencyRow); ok {
		return rows
	}
	return nil
}

func dependencyStatus(status string) string {
	return map[string]string{
		"found":           "制品已找到",
		"missing_binary":  "无匹配二进制",
		"missing_package": "仓库无此包",
		"missing_version": "仓库无此版本",
		"mismatch":        "配方不一致",
		"unknown":         "未查询",
	}[status]
}

func firstDependencyWarning(report *workflow.Report) string {
	if report == nil {
		return ""
	}
	for _, row := range dependencyRows(*report) {
		if row.Status != "found" && row.Status != "unknown" {
			return row.Reference + " 在 " + fallback(row.Platform, "当前平台") + " 无匹配二进制"
		}
	}
	return ""
}

func missingPlatform(spec config.PlatformSpec) bool {
	return strings.TrimSpace(spec.OS) == "" || strings.TrimSpace(spec.Arch) == ""
}

func statusColor(status string) string {
	if status == "就绪" {
		return "32"
	}
	if status == "需要处理" {
		return "31"
	}
	return "33"
}

func ternary(value bool, trueValue, falseValue string) string {
	if value {
		return trueValue
	}
	return falseValue
}
