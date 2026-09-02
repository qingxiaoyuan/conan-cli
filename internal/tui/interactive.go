package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"conan-cli/internal/config"
	"conan-cli/internal/platform"
	"conan-cli/internal/workflow"
	"golang.org/x/term"
)

// Cursor mode is used only when both stdin and stdout are terminals. This
// keeps `conan-cli tui < input.txt` useful for smoke tests and automation,
// while an actual terminal behaves like the design: arrows move focus and
// Enter activates the focused item.
func runCursor(ctx context.Context, app *workflow.App, in io.Reader, out io.Writer) error {
	input, ok := in.(*os.File)
	if !ok {
		return runLine(ctx, app, in, out)
	}
	state, err := term.MakeRaw(int(input.Fd()))
	if err != nil {
		return runLine(ctx, app, in, out)
	}
	defer term.Restore(int(input.Fd()), state)

	ui := newUI(app, out, true)
	columns, _, sizeErr := term.GetSize(int(input.Fd()))
	if sizeErr == nil {
		ui.setCursorLayout(columns)
	} else {
		ui.setCursorLayout(innerWidth)
	}
	ui.enter()
	defer ui.leave()
	controller := &cursorController{ui: ui, keys: newKeyReader(in)}
	return controller.run(ctx)
}

func isInteractiveTerminal(in io.Reader, out io.Writer) bool {
	input, inputOK := in.(*os.File)
	output, outputOK := out.(*os.File)
	if !inputOK || !outputOK {
		return false
	}
	return terminalFD(int(input.Fd())) && terminalFD(int(output.Fd()))
}

func terminalFD(fd int) bool {
	return term.IsTerminal(fd)
}

type cursorKeyKind uint8

const (
	keyUnknown cursorKeyKind = iota
	keyUp
	keyDown
	keyLeft
	keyRight
	keyEnter
	keyEscape
	keyTab
	keyBackspace
	keyDelete
	keyHome
	keyEnd
	keyCharacter
	keyInterrupt
)

type cursorKey struct {
	kind cursorKeyKind
	char rune
}

type byteEvent struct {
	value byte
	err   error
}

// keyReader decodes terminal escape sequences without requiring a TUI
// framework. The byte pump lets a lone Escape return promptly while still
// allowing arrow sequences to arrive a few milliseconds later.
type keyReader struct {
	events  chan byteEvent
	pending []byte
}

func newKeyReader(input io.Reader) *keyReader {
	reader := &keyReader{events: make(chan byteEvent, 16)}
	go func() {
		buffer := []byte{0}
		for {
			count, err := input.Read(buffer)
			if count > 0 {
				reader.events <- byteEvent{value: buffer[0]}
			}
			if err != nil {
				reader.events <- byteEvent{err: err}
				close(reader.events)
				return
			}
		}
	}()
	return reader
}

func (reader *keyReader) nextByte(timeout time.Duration) (byte, bool, error) {
	if len(reader.pending) > 0 {
		value := reader.pending[0]
		reader.pending = reader.pending[1:]
		return value, true, nil
	}
	var event byteEvent
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case received, ok := <-reader.events:
			if !ok {
				return 0, false, io.EOF
			}
			event = received
		case <-timer.C:
			return 0, false, nil
		}
	} else {
		var ok bool
		event, ok = <-reader.events
		if !ok {
			return 0, false, io.EOF
		}
	}
	if event.err != nil {
		return 0, false, event.err
	}
	return event.value, true, nil
}

func (reader *keyReader) unread(value byte) {
	reader.pending = append([]byte{value}, reader.pending...)
}

func (reader *keyReader) next() (cursorKey, error) {
	value, ok, err := reader.nextByte(0)
	if err != nil {
		return cursorKey{}, err
	}
	if !ok {
		return cursorKey{}, io.EOF
	}
	switch value {
	case 3:
		return cursorKey{kind: keyInterrupt}, nil
	case '\r', '\n':
		return cursorKey{kind: keyEnter}, nil
	case '\t':
		return cursorKey{kind: keyTab}, nil
	case 8, 127:
		return cursorKey{kind: keyBackspace}, nil
	case 27:
		return reader.escapeSequence()
	}
	if value < utf8.RuneSelf {
		return cursorKey{kind: keyCharacter, char: rune(value)}, nil
	}
	return reader.utf8Key(value)
}

func (reader *keyReader) escapeSequence() (cursorKey, error) {
	value, ok, err := reader.nextByte(45 * time.Millisecond)
	if err != nil || !ok {
		return cursorKey{kind: keyEscape}, nil
	}
	if value != '[' && value != 'O' {
		reader.unread(value)
		return cursorKey{kind: keyEscape}, nil
	}
	value, ok, err = reader.nextByte(45 * time.Millisecond)
	if err != nil || !ok {
		return cursorKey{kind: keyEscape}, nil
	}
	switch value {
	case 'A':
		return cursorKey{kind: keyUp}, nil
	case 'B':
		return cursorKey{kind: keyDown}, nil
	case 'C':
		return cursorKey{kind: keyRight}, nil
	case 'D':
		return cursorKey{kind: keyLeft}, nil
	case 'H':
		return cursorKey{kind: keyHome}, nil
	case 'F':
		return cursorKey{kind: keyEnd}, nil
	case '1', '3', '4':
		sequence := value
		for {
			value, ok, err = reader.nextByte(45 * time.Millisecond)
			if err != nil || !ok {
				return cursorKey{kind: keyUnknown}, nil
			}
			if value != '~' {
				continue
			}
			break
		}
		if value == '~' {
			switch sequence {
			case '1':
				return cursorKey{kind: keyHome}, nil
			case '3':
				return cursorKey{kind: keyDelete}, nil
			case '4':
				return cursorKey{kind: keyEnd}, nil
			}
		}
	}
	return cursorKey{kind: keyUnknown}, nil
}

func (reader *keyReader) utf8Key(first byte) (cursorKey, error) {
	size := utf8SequenceLength(first)
	if size == 1 {
		return cursorKey{kind: keyCharacter, char: rune(first)}, nil
	}
	buffer := []byte{first}
	for len(buffer) < size {
		value, ok, err := reader.nextByte(0)
		if err != nil {
			return cursorKey{}, err
		}
		if !ok {
			return cursorKey{}, io.EOF
		}
		buffer = append(buffer, value)
	}
	character, _ := utf8.DecodeRune(buffer)
	return cursorKey{kind: keyCharacter, char: character}, nil
}

func utf8SequenceLength(first byte) int {
	switch {
	case first < 0x80:
		return 1
	case first&0xe0 == 0xc0:
		return 2
	case first&0xf0 == 0xe0:
		return 3
	case first&0xf8 == 0xf0:
		return 4
	default:
		return 1
	}
}

type cursorController struct {
	ui   *dashboard
	keys *keyReader
}

func (controller *cursorController) run(ctx context.Context) error {
	selected := 0
	for {
		controller.ui.refreshProject()
		controller.ui.renderCursorMain(selected)
		key, err := controller.keys.next()
		if err != nil {
			return nil
		}
		if isCursorQuit(key) {
			return nil
		}
		switch key.kind {
		case keyUp:
			selected = moveGrid2D(selected, 0, -1, 4, 8)
		case keyDown:
			selected = moveGrid2D(selected, 0, 1, 4, 8)
		case keyLeft:
			selected = moveGrid2D(selected, -1, 0, 4, 8)
		case keyRight:
			selected = moveGrid2D(selected, 1, 0, 4, 8)
		case keyHome:
			selected = 0
		case keyEnd:
			selected = 7
		case keyEnter:
			if err := controller.activateMain(ctx, selected); err != nil {
				return err
			}
		}
	}
}

func (controller *cursorController) activateMain(ctx context.Context, selected int) error {
	switch selected {
	case 0:
		controller.actionResult(ctx, "初始化 / 刷新项目", func() (workflow.Report, error) { return controller.ui.app.Init(ctx) })
	case 1:
		controller.actionResult(ctx, "扫描 Qt 和编译器", func() (workflow.Report, error) { return controller.ui.app.Scan(ctx, false) })
	case 2:
		controller.analyzePage(ctx)
	case 3:
		controller.installPage(ctx)
	case 4:
		controller.publishPage(ctx)
	case 5:
		controller.settingsPage(ctx)
	case 6:
		controller.doctorPage(ctx)
	case 7:
		controller.dependenciesPage(ctx)
	}
	return nil
}

func (controller *cursorController) actionResult(ctx context.Context, title string, action func() (workflow.Report, error)) {
	controller.ui.cursorScreen(title)
	controller.ui.boxRow("  " + actionProgress(title))
	report, err := action()
	controller.ui.storeReport(report, err)
	controller.ui.refreshProject()
	controller.showResult(title, report, err)
}

func (controller *cursorController) showResult(title string, report workflow.Report, err error) {
	controller.ui.cursorScreen(title)
	printReport(controller.ui.out, report, err, controller.ui.ansi)
	controller.ui.boxSeparator("├", "┤")
	controller.ui.boxRow("  按任意键返回")
	controller.ui.boxSeparator("└", "┘")
	_, _ = controller.keys.next()
}

func actionProgress(title string) string {
	return "正在" + strings.TrimSpace(strings.TrimSuffix(title, " / 刷新项目")) + "…"
}

func (ui *dashboard) renderCursorMain(selected int) {
	ui.cursorScreen("")
	status := "未初始化"
	if ui.projectErr == nil {
		status = "就绪"
	}
	if ui.lastReport != nil && !ui.lastReport.OK {
		status = "需要处理"
	}
	ui.boxRow(ui.paint("  CONAN CLI  ·  项目工作台", "36") + "  " + ui.paint(status, statusColor(status)))
	ui.boxSeparator("├", "┤")
	ui.boxRow("  项目 " + padCell(value(ui.project, func(p *config.Project) string { return p.Name }), 15) + "  构建 " + padCell(value(ui.project, func(p *config.Project) string { return p.BuildSystem }), 8) + "  频道 " + padCell(value(ui.project, func(p *config.Project) string { return p.Channel }), 8))
	remote := value(ui.project, func(p *config.Project) string { return p.Remote })
	if remote == "-" && ui.global != nil {
		remote = fallback(ui.global.Nexus.Name, "未配置")
	}
	ui.boxRow("  平台 " + padCell(platformValue(ui.project), 15) + "  Qt " + padCell(value(ui.project, func(p *config.Project) string { return p.QtVersion }), 7) + "  编译器 " + padCell(value(ui.project, func(p *config.Project) string { return p.Compiler.Display() }), 12) + "  仓库 " + padCell(remote, 12))
	ui.boxSeparator("├", "┤")
	ui.boxRow("  快捷操作（方向键移动，Enter 选择）")
	menu := []string{"初始化 / 刷新", "扫描", "依赖分析", "下载", "发布表单", "设置", "诊断", "依赖管理"}
	for row := 0; row < 2; row++ {
		cells := make([]string, 0, 4)
		for column := 0; column < 4; column++ {
			index := row*4 + column
			cells = append(cells, cursorItem(ui, index, menu[index], selected == index))
		}
		ui.boxRow("  " + strings.Join(cells, ""))
	}
	ui.boxSeparator("├", "┤")
	if warning := firstDependencyWarning(ui.lastAnalyze); warning != "" {
		ui.boxRow("  " + ui.paint("! "+warning, "33"))
	} else if ui.lastMessage != "" {
		ui.boxRow("  " + clip(ui.lastMessage, innerWidth-2))
	} else {
		ui.boxRow("  最近状态：" + fallback(lastAction(ui.lastReport), "暂无操作"))
	}
	ui.boxSeparator("└", "┘")
	ui.blankOuterLine()
	ui.cursorHint("↑ ↓ ← → 移动 · Enter 选择 · Esc 退出")
}

func cursorItem(ui *dashboard, index int, label string, selected bool) string {
	text := fmt.Sprintf("%d  %s", index+1, label)
	if selected {
		return ui.highlight(padCell("▸ "+text, 18))
	}
	return padCell("  "+text, 18)
}

func lastAction(report *workflow.Report) string {
	if report == nil {
		return ""
	}
	return localAction(report.Action)
}

func moveGrid(selected, delta, count int) int {
	selected += delta
	if selected < 0 {
		return 0
	}
	if selected >= count {
		return count - 1
	}
	return selected
}

func moveGrid2D(selected, deltaX, deltaY, columns, count int) int {
	if columns <= 0 || count <= 0 {
		return 0
	}
	row, column := selected/columns, selected%columns
	row += deltaY
	column += deltaX
	if row < 0 {
		row = 0
	}
	if column < 0 {
		column = 0
	}
	maxRow := (count - 1) / columns
	if row > maxRow {
		row = maxRow
	}
	if column >= columns {
		column = columns - 1
	}
	result := row*columns + column
	if result >= count {
		result = count - 1
	}
	return result
}

func isCursorQuit(key cursorKey) bool {
	return key.kind == keyEscape || key.kind == keyInterrupt || (key.kind == keyCharacter && (key.char == 'q' || key.char == 'Q'))
}

func (controller *cursorController) analyzePage(ctx context.Context) {
	report, err := controller.analyze(ctx)
	_ = err
	selected := 0
	for {
		controller.ui.renderCursorAnalyze(report, selected)
		key, keyErr := controller.keys.next()
		if keyErr != nil || isCursorQuit(key) {
			return
		}
		switch key.kind {
		case keyUp, keyLeft:
			selected = moveGrid(selected, -1, 5)
		case keyDown, keyRight:
			selected = moveGrid(selected, 1, 5)
		case keyHome:
			selected = 0
		case keyEnd:
			selected = 4
		case keyEnter:
			switch selected {
			case 0:
				report, err = controller.analyze(ctx)
			case 1:
				if controller.choosePlatform(ctx, true) {
					report, err = controller.analyze(ctx)
				}
			case 2:
				if controller.choosePlatform(ctx, false) {
					report, err = controller.analyze(ctx)
				}
			case 3:
				controller.installPage(ctx)
				report, err = controller.analyze(ctx)
			case 4:
				return
			}
		}
		if keyErr != nil {
			return
		}
	}
}

func (controller *cursorController) analyze(ctx context.Context) (workflow.Report, error) {
	report, err := controller.ui.app.Analyze(ctx, "", "", "")
	controller.ui.storeReport(report, err)
	controller.ui.lastAnalyze = &report
	controller.ui.refreshProject()
	return report, err
}

func (ui *dashboard) renderCursorAnalyze(report workflow.Report, selected int) {
	data := reportMap(report)
	ui.cursorScreen("依赖分析")
	name := fallback(stringValue(data["platform"]), "未选择目标平台")
	ui.boxRow("  分析 · " + name + "  · 远程 " + fallback(stringValue(data["remote"]), "未配置"))
	ui.boxRow("  Conan settings：" + settingsSummary(mapValue(data["conan_settings"])))
	ui.boxSeparator("├", "┤")
	rows := dependencyRows(report)
	if len(rows) == 0 {
		ui.boxRow("  暂无直接依赖。可在依赖管理页添加包")
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
	ui.boxSeparator("├", "┤")
	ui.boxRow("  " + cursorAction(ui, "刷新分析", selected == 0) + "   " + cursorAction(ui, "改系统", selected == 1) + "   " + cursorAction(ui, "改架构", selected == 2))
	ui.boxRow("  " + cursorAction(ui, "下载", selected == 3) + "   " + cursorAction(ui, "返回", selected == 4))
	ui.boxSeparator("└", "┘")
}

func cursorAction(ui *dashboard, label string, selected bool) string {
	if selected {
		return ui.paint("▸ "+label, "32")
	}
	return "  " + label
}

func (controller *cursorController) choosePlatform(ctx context.Context, chooseOS bool) bool {
	controller.ui.refreshProject()
	project := controller.ui.project
	if project == nil {
		project = config.NewProject(controller.ui.app.Dir)
	}
	if chooseOS {
		options := []choiceOption{{config.OSWindows, "Windows"}, {config.OSLinux, "Linux"}, {config.OSKylin, "麒麟"}}
		value, ok := controller.choicePage("选择操作系统", options, project.Platform.Consume.OS)
		if !ok {
			return false
		}
		report, err := controller.ui.app.SaveProjectSettings(workflow.ProjectSettingsInput{OS: value})
		controller.ui.storeReport(report, err)
		controller.ui.refreshProject()
		if err != nil {
			controller.showResult("保存平台", report, err)
		}
		return err == nil
	}
	options := []choiceOption{{config.ArchX86, "x86 32 位"}, {config.ArchX64, "x64 64 位"}, {config.ArchARM, "ARM 32 位"}, {config.ArchARM64, "ARM 64 位"}}
	value, ok := controller.choicePage("选择架构", options, project.Platform.Consume.Arch)
	if !ok {
		return false
	}
	report, err := controller.ui.app.SaveProjectSettings(workflow.ProjectSettingsInput{Arch: value})
	controller.ui.storeReport(report, err)
	controller.ui.refreshProject()
	if err != nil {
		controller.showResult("保存平台", report, err)
	}
	return err == nil
}

type choiceOption struct {
	value string
	label string
}

func (controller *cursorController) choicePage(title string, options []choiceOption, current string) (string, bool) {
	selected := 0
	for index, option := range options {
		if option.value == current {
			selected = index
			break
		}
	}
	for {
		controller.ui.cursorScreen(title)
		controller.ui.boxRow("  方向键移动，Enter 选择，Esc 取消")
		for index, option := range options {
			marker := "  "
			if index == selected {
				marker = controller.ui.paint("▸ ", "32")
			}
			controller.ui.boxRow(marker + padCell(option.label, 20) + " " + option.value)
		}
		controller.ui.boxSeparator("└", "┘")
		key, err := controller.keys.next()
		if err != nil || key.kind == keyEscape {
			return "", false
		}
		switch key.kind {
		case keyUp, keyLeft:
			selected = moveGrid(selected, -1, len(options))
		case keyDown, keyRight:
			selected = moveGrid(selected, 1, len(options))
		case keyHome:
			selected = 0
		case keyEnd:
			selected = len(options) - 1
		case keyEnter:
			return options[selected].value, true
		}
	}
}

func (controller *cursorController) installPage(ctx context.Context) {
	controller.ui.refreshProject()
	if controller.ui.project == nil || controller.ui.projectErr != nil {
		controller.actionResult(ctx, "初始化项目", func() (workflow.Report, error) { return controller.ui.app.Init(ctx) })
		controller.ui.refreshProject()
		if controller.ui.project == nil || controller.ui.projectErr != nil {
			return
		}
	}
	if missingPlatform(controller.ui.project.Platform.Consume) {
		if !controller.choosePlatform(ctx, true) || !controller.choosePlatform(ctx, false) {
			return
		}
	}
	controller.ui.refreshProject()
	project := controller.ui.project
	settings := platform.Resolve(project.Platform.Consume, project.Compiler, project.QtVersion)
	selected := 0
	for {
		controller.ui.cursorScreen("下载")
		controller.ui.boxRow("  将下载 " + project.Platform.Consume.Display() + " 的已有二进制")
		controller.ui.boxRow("  Conan os=" + fallback(settings.ConanOS, "-") + "  arch=" + fallback(settings.ConanArch, "-"))
		controller.ui.boxRow("  编译器 " + fallback(project.Compiler.Display(), "未设置") + "  Qt " + fallback(project.QtVersion, "未设置"))
		controller.ui.boxRow("  策略 download-only：缺制品时不会在本机编译")
		controller.ui.boxSeparator("├", "┤")
		controller.ui.boxRow("  " + cursorAction(controller.ui, "开始下载", selected == 0) + "   " + cursorAction(controller.ui, "返回", selected == 1))
		controller.ui.boxSeparator("└", "┘")
		key, err := controller.keys.next()
		if err != nil || key.kind == keyEscape || (key.kind == keyCharacter && (key.char == 'q' || key.char == 'Q')) {
			return
		}
		switch key.kind {
		case keyUp, keyLeft, keyDown, keyRight:
			selected = 1 - selected
		case keyEnter:
			if selected == 1 {
				return
			}
			controller.actionResult(ctx, "下载依赖", func() (workflow.Report, error) {
				return controller.ui.app.InstallPlatform(ctx, workflow.InstallRequest{})
			})
			return
		}
	}
}

func (controller *cursorController) dependenciesPage(ctx context.Context) {
	report, err := controller.analyze(ctx)
	_ = err
	selected := 0
	for {
		controller.ui.renderCursorDependencies(report, selected)
		key, keyErr := controller.keys.next()
		if keyErr != nil || isCursorQuit(key) {
			return
		}
		switch key.kind {
		case keyUp, keyLeft:
			selected = moveGrid(selected, -1, 5)
		case keyDown, keyRight:
			selected = moveGrid(selected, 1, 5)
		case keyEnter:
			switch selected {
			case 0:
				controller.addDependencyCursor()
				report, err = controller.analyze(ctx)
			case 1:
				controller.searchCursor(ctx)
			case 2:
				report, err = controller.analyze(ctx)
			case 3:
				controller.installPage(ctx)
				report, err = controller.analyze(ctx)
			case 4:
				return
			}
		}
	}
}

func (ui *dashboard) renderCursorDependencies(report workflow.Report, selected int) {
	data := reportMap(report)
	ui.cursorScreen("依赖管理")
	ui.boxRow("  当前项目直接依赖 · " + fallback(stringValue(data["platform"]), "未选择目标平台"))
	ui.boxRow("  Conan settings：" + settingsSummary(mapValue(data["conan_settings"])))
	ui.boxSeparator("├", "┤")
	rows := dependencyRows(report)
	if len(rows) == 0 {
		ui.boxRow("  暂无直接依赖")
	}
	for _, row := range rows {
		glyph := ui.paint("✓", "32")
		if row.Status != "found" && row.Status != "unknown" {
			glyph = ui.paint("×", "33")
		}
		ui.boxRow("  " + glyph + " " + padCell(row.Reference, 27) + " " + padCell(fallback(dependencyStatus(row.Status), "未查询"), 16) + " " + clip(row.Detail, 27))
	}
	ui.boxSeparator("├", "┤")
	ui.boxRow("  " + cursorAction(ui, "添加依赖", selected == 0) + "   " + cursorAction(ui, "搜索包", selected == 1) + "   " + cursorAction(ui, "刷新", selected == 2))
	ui.boxRow("  " + cursorAction(ui, "下载", selected == 3) + "   " + cursorAction(ui, "返回", selected == 4))
	ui.boxSeparator("└", "┘")
}

func (controller *cursorController) addDependencyCursor() {
	value, ok := controller.editText("添加依赖", "包引用", "")
	if !ok || strings.TrimSpace(value) == "" {
		return
	}
	report, err := controller.ui.app.Add(value)
	controller.ui.storeReport(report, err)
	controller.ui.refreshProject()
	controller.showResult("添加依赖", report, err)
}

func (controller *cursorController) searchCursor(ctx context.Context) {
	value, ok := controller.editText("搜索远程仓库", "包名或版本", "")
	if !ok || strings.TrimSpace(value) == "" {
		return
	}
	report, err := controller.ui.app.Search(ctx, value, "")
	controller.ui.storeReport(report, err)
	controller.ui.cursorScreen("搜索远程仓库")
	if err != nil {
		printReport(controller.ui.out, report, err, controller.ui.ansi)
	} else if data, marshalErr := json.MarshalIndent(report.Data, "", "  "); marshalErr == nil {
		printOutput(controller.ui.out, string(data))
	}
	controller.ui.boxSeparator("├", "┤")
	controller.ui.boxRow("  按任意键返回")
	controller.ui.boxSeparator("└", "┘")
	_, _ = controller.keys.next()
}

func (controller *cursorController) doctorPage(ctx context.Context) {
	report, err := controller.ui.app.Doctor(ctx)
	controller.ui.storeReport(report, err)
	selected := 0
	for {
		controller.ui.renderCursorDoctor(report, selected)
		key, keyErr := controller.keys.next()
		if keyErr != nil || isCursorQuit(key) {
			return
		}
		switch key.kind {
		case keyUp, keyLeft, keyDown, keyRight:
			selected = 1 - selected
		case keyEnter:
			if selected == 1 {
				return
			}
			report, err = controller.ui.app.Doctor(ctx)
			controller.ui.storeReport(report, err)
		}
	}
}

func (ui *dashboard) renderCursorDoctor(report workflow.Report, selected int) {
	ui.cursorScreen("诊断")
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
	ui.boxSeparator("├", "┤")
	ui.boxRow("  " + cursorAction(ui, "重新诊断", selected == 0) + "   " + cursorAction(ui, "返回", selected == 1))
	ui.boxSeparator("└", "┘")
}

type settingsCursorState struct {
	active          string
	selected        int
	global          *config.Global
	project         *config.Project
	pendingPassword string
}

func (controller *cursorController) settingsPage(ctx context.Context) {
	controller.ui.refreshProject()
	state := &settingsCursorState{active: "global", global: controller.ui.global, project: controller.ui.project}
	if state.global == nil {
		state.global = &config.Global{}
	}
	if state.project == nil {
		state.project = config.NewProject(controller.ui.app.Dir)
	}
	for {
		controller.ui.renderCursorSettings(state)
		key, err := controller.keys.next()
		if err != nil || key.kind == keyEscape || key.kind == keyInterrupt || (key.kind == keyCharacter && (key.char == 'q' || key.char == 'Q')) {
			return
		}
		fieldCount := settingsFieldCount(state.active)
		actionCount := settingsActionCount(state.active)
		maxIndex := fieldCount + actionCount - 1
		switch key.kind {
		case keyTab, keyLeft, keyRight:
			state.active = toggleSettingsTab(state.active)
			state.selected = 0
		case keyUp:
			state.selected = moveGrid(state.selected, -1, maxIndex+1)
		case keyDown:
			state.selected = moveGrid(state.selected, 1, maxIndex+1)
		case keyHome:
			state.selected = 0
		case keyEnd:
			state.selected = maxIndex
		case keyEnter:
			controller.settingsEnter(ctx, state)
		}
	}
}

func (controller *cursorController) settingsEnter(ctx context.Context, state *settingsCursorState) {
	fieldCount := settingsFieldCount(state.active)
	if state.selected < fieldCount {
		controller.editSettingField(state)
		return
	}
	action := state.selected - fieldCount
	if state.active == "global" {
		switch action {
		case 0:
			report, err := controller.ui.saveGlobal(ctx, state.global, state.pendingPassword)
			controller.ui.storeReport(report, err)
			if err == nil {
				state.pendingPassword = ""
			}
			controller.ui.refreshProject()
			controller.showResult("保存全局设置", report, err)
			state.global = controller.ui.global
		case 1:
			report, err := controller.ui.app.ConfigTest(ctx)
			controller.ui.storeReport(report, err)
			controller.showResult("测试连接", report, err)
		case 2:
			report, err := controller.ui.saveGlobal(ctx, state.global, state.pendingPassword)
			controller.ui.storeReport(report, err)
			if err == nil {
				loginReport, loginErr := controller.ui.app.ConfigLogin(ctx, "")
				controller.ui.storeReport(loginReport, loginErr)
				controller.showResult("保存并登录", loginReport, loginErr)
				if loginErr == nil {
					state.pendingPassword = ""
				}
			} else {
				controller.showResult("保存并登录", report, err)
			}
			controller.ui.refreshProject()
			state.global = controller.ui.global
		case 3:
			return
		}
		return
	}
	if action == 0 {
		report, err := controller.ui.saveProject(state.project)
		controller.ui.storeReport(report, err)
		controller.ui.refreshProject()
		controller.showResult("保存项目设置", report, err)
		state.project = controller.ui.project
		if state.project == nil {
			state.project = config.NewProject(controller.ui.app.Dir)
		}
	} else {
		// The final project action is Back.
		return
	}
}

func settingsFieldCount(active string) int {
	if active == "global" {
		return 5
	}
	return 14
}

func settingsActionCount(active string) int {
	if active == "global" {
		return 4
	}
	return 2
}

func toggleSettingsTab(active string) string {
	if active == "global" {
		return "project"
	}
	return "global"
}

func (ui *dashboard) renderCursorSettings(state *settingsCursorState) {
	title := "项目"
	if state.active == "global" {
		title = "全局"
	}
	ui.cursorScreen("设置 · " + title)
	ui.boxRow("  Tab / ← → 切换：" + cursorAction(ui, "全局", state.active == "global") + "   " + cursorAction(ui, "项目", state.active == "project"))
	ui.boxSeparator("├", "┤")
	if state.active == "global" {
		view := state.global.View()
		rows := []string{
			"仓库名       " + fallback(state.global.Nexus.Name, "nexus"),
			"URL          " + fallback(clip(state.global.Nexus.URL, 52), "未配置"),
			"用户         " + fallback(state.global.Nexus.Username, "未配置"),
			"密码 / Token  " + passwordLabel(view.HasPassword || state.pendingPassword != ""),
			"Conan 路径    " + fallback(state.global.ConanBin, "PATH 中的 conan"),
		}
		for index, row := range rows {
			ui.boxRow("  " + settingRow(ui, index, row, state.selected == index))
		}
		ui.boxSeparator("├", "┤")
		ui.boxRow("  " + cursorAction(ui, "保存", state.selected == 5) + "   " + cursorAction(ui, "测试连接", state.selected == 6) + "   " + cursorAction(ui, "保存并登录", state.selected == 7) + "   " + cursorAction(ui, "返回", state.selected == 8))
	} else {
		rows := []string{
			"项目名称      " + fallback(state.project.Name, "-"),
			"Qt 版本       " + fallback(state.project.QtVersion, "未设置"),
			"编译器        " + fallback(state.project.Compiler.ID, "未设置"),
			"编译器版本    " + fallback(state.project.Compiler.Version, "未设置"),
			"使用平台      " + fallback(state.project.Platform.Consume.Display(), "未选择"),
			"使用架构      " + fallback(config.DisplayArch(state.project.Platform.Consume.Arch), "未选择"),
			"使用构建      " + fallback(config.DisplayBuildType(state.project.Platform.Consume.BuildType), "Release"),
			"发布平台      " + fallback(state.project.Platform.Publish.Display(), "未选择"),
			"发布架构      " + fallback(config.DisplayArch(state.project.Platform.Publish.Arch), "未选择"),
			"发布构建      " + fallback(config.DisplayBuildType(state.project.Platform.Publish.BuildType), "Release"),
			"channel       " + fallback(state.project.Channel, "dev"),
			"远程仓库      " + fallback(state.project.Remote, "跟随全局"),
			"构建系统      " + fallback(state.project.BuildSystem, "unknown"),
			"输出目录      " + fallback(state.project.OutputFolder, "conan"),
		}
		for index, row := range rows {
			ui.boxRow("  " + settingRow(ui, index, row, state.selected == index))
		}
		ui.boxRow("  缺二进制策略  download-only（固定）")
		ui.boxSeparator("├", "┤")
		ui.boxRow("  " + cursorAction(ui, "保存项目设置", state.selected == 14) + "   " + cursorAction(ui, "返回", state.selected == 15))
	}
	ui.boxSeparator("└", "┘")
}

func settingRow(ui *dashboard, index int, value string, selected bool) string {
	if selected {
		return ui.paint("▸ "+fmt.Sprintf("%d ", index+1)+value, "32")
	}
	return fmt.Sprintf("  %d ", index+1) + value
}

func passwordLabel(saved bool) string {
	if saved {
		return "********（本机保存）"
	}
	return "未保存"
}

func (controller *cursorController) editSettingField(state *settingsCursorState) {
	index := state.selected
	if state.active == "global" {
		values := []string{state.global.Nexus.Name, state.global.Nexus.URL, state.global.Nexus.Username, "", state.global.ConanBin}
		labels := []string{"仓库名", "Nexus Conan URL", "登录用户名", "密码 / Token（留空保留）", "Conan 可执行文件路径"}
		value, ok := controller.editText("编辑全局设置", labels[index], values[index])
		if !ok {
			return
		}
		switch index {
		case 0:
			state.global.Nexus.Name = value
		case 1:
			state.global.Nexus.URL = value
		case 2:
			state.global.Nexus.Username = value
		case 3:
			if value != "" {
				state.pendingPassword = value
			}
		case 4:
			state.global.ConanBin = value
		}
		return
	}
	if index == 4 || index == 5 || index == 6 || index == 7 {
		controller.editProjectPlatform(state, index)
		return
	}
	values := []string{
		state.project.Name, state.project.QtVersion, state.project.Compiler.ID, state.project.Compiler.Version,
		"", "", "", "", "", "", state.project.Channel, state.project.Remote, state.project.BuildSystem, state.project.OutputFolder,
	}
	labels := []string{"项目名称", "Qt 版本", "编译器 gcc / clang / msvc", "编译器版本", "", "", "", "", "", "", "channel", "项目远程仓库名", "构建系统 cmake / qmake / unknown", "输出目录"}
	value, ok := controller.editText("编辑项目设置", labels[index], values[index])
	if !ok {
		return
	}
	switch index {
	case 0:
		state.project.Name = value
	case 1:
		state.project.QtVersion = value
	case 2:
		state.project.Compiler.ID = value
	case 3:
		state.project.Compiler.Version = value
	case 10:
		state.project.Channel = value
	case 11:
		state.project.Remote = value
	case 12:
		state.project.BuildSystem = value
	case 13:
		state.project.OutputFolder = value
	}
}

func (controller *cursorController) editProjectPlatform(state *settingsCursorState, index int) {
	if index == 4 || index == 7 {
		current := state.project.Platform.Consume.OS
		if index == 7 {
			current = state.project.Platform.Publish.OS
		}
		value, ok := controller.choicePage("选择操作系统", []choiceOption{{config.OSWindows, "Windows"}, {config.OSLinux, "Linux"}, {config.OSKylin, "麒麟"}}, current)
		if ok {
			if index == 4 {
				state.project.Platform.Consume.OS = value
			} else {
				state.project.Platform.Publish.OS = value
			}
		}
		return
	}
	if index == 5 || index == 8 {
		current := state.project.Platform.Consume.Arch
		if index == 8 {
			current = state.project.Platform.Publish.Arch
		}
		value, ok := controller.choicePage("选择架构", []choiceOption{{config.ArchX86, "x86 32 位"}, {config.ArchX64, "x64 64 位"}, {config.ArchARM, "ARM 32 位"}, {config.ArchARM64, "ARM 64 位"}}, current)
		if ok {
			if index == 5 {
				state.project.Platform.Consume.Arch = value
			} else {
				state.project.Platform.Publish.Arch = value
			}
		}
		return
	}
	if index == 6 || index == 9 {
		current := state.project.Platform.Consume.BuildType
		if index == 9 {
			current = state.project.Platform.Publish.BuildType
		}
		value, ok := controller.choicePage("选择构建类型", []choiceOption{{config.BuildTypeRelease, "Release"}, {config.BuildTypeDebug, "Debug"}}, current)
		if ok {
			if index == 6 {
				state.project.Platform.Consume.BuildType = value
			} else {
				state.project.Platform.Publish.BuildType = value
			}
		}
	}
}

func (controller *cursorController) editText(title, label, initial string) (string, bool) {
	value := []rune(initial)
	if strings.Contains(label, "密码") {
		value = nil
	}
	position := len(value)
	for {
		controller.ui.cursorScreen(title)
		controller.ui.boxRow("  " + label)
		controller.ui.boxRow("  " + editorValue(value, position, strings.Contains(label, "密码")))
		controller.ui.boxSeparator("├", "┤")
		controller.ui.boxRow("  输入文字 · ← → 移动 · Backspace 删除 · Enter 保存 · Esc 取消")
		controller.ui.boxSeparator("└", "┘")
		key, err := controller.keys.next()
		if err != nil || key.kind == keyEscape {
			return "", false
		}
		switch key.kind {
		case keyLeft:
			if position > 0 {
				position--
			}
		case keyRight:
			if position < len(value) {
				position++
			}
		case keyHome:
			position = 0
		case keyEnd:
			position = len(value)
		case keyBackspace:
			if position > 0 {
				value = append(value[:position-1], value[position:]...)
				position--
			}
		case keyDelete:
			if position < len(value) {
				value = append(value[:position], value[position+1:]...)
			}
		case keyCharacter:
			if key.char >= 0x20 {
				value = append(value, 0)
				copy(value[position+1:], value[position:])
				value[position] = key.char
				position++
			}
		case keyEnter:
			return string(value), true
		}
	}
}

func editorValue(value []rune, position int, secret bool) string {
	display := make([]rune, len(value))
	for index, character := range value {
		if secret && character != ' ' {
			display[index] = '•'
		} else {
			display[index] = character
		}
	}
	if position < 0 {
		position = 0
	}
	if position > len(display) {
		position = len(display)
	}
	display = append(display, 0)
	copy(display[position+1:], display[position:])
	display[position] = '▌'
	return string(display)
}

type publishCursorForm struct {
	name      string
	version   string
	channel   string
	os        string
	arch      string
	buildType string
	remote    string
	note      string
}

func (controller *cursorController) publishPage(ctx context.Context) {
	controller.ui.refreshProject()
	if controller.ui.project == nil || controller.ui.projectErr != nil {
		controller.showResult("发布表单", workflow.Report{}, fmt.Errorf("项目尚未初始化，请先执行初始化"))
		return
	}
	form := publishCursorForm{channel: controller.ui.project.Channel, os: controller.ui.project.Platform.Publish.OS, arch: controller.ui.project.Platform.Publish.Arch, remote: controller.ui.project.Remote}
	if form.os == "" {
		form.os = controller.ui.project.Platform.Consume.OS
	}
	if form.arch == "" {
		form.arch = controller.ui.project.Platform.Consume.Arch
	}
	form.buildType = controller.ui.project.Platform.Publish.BuildType
	if form.buildType == "" {
		form.buildType = controller.ui.project.Platform.Consume.BuildType
	}
	if form.buildType == "" {
		form.buildType = config.BuildTypeRelease
	}
	if form.remote == "" && controller.ui.global != nil {
		form.remote = controller.ui.global.Nexus.Name
	}
	if metadata, _, err := controller.ui.app.Client.Inspect(ctx); err == nil {
		form.name = stringValue(metadata["name"])
		form.version = stringValue(metadata["version"])
	}
	selected := 0
	for {
		controller.ui.renderCursorPublish(form, selected)
		key, err := controller.keys.next()
		if err != nil || isCursorQuit(key) {
			return
		}
		switch key.kind {
		case keyUp, keyLeft:
			selected = moveGrid(selected, -1, 10)
		case keyDown, keyRight:
			selected = moveGrid(selected, 1, 10)
		case keyHome:
			selected = 0
		case keyEnd:
			selected = 9
		case keyEnter:
			if !controller.publishEnter(ctx, &form, selected) {
				return
			}
		}
	}
}

func (ui *dashboard) renderCursorPublish(form publishCursorForm, selected int) {
	ui.cursorScreen("发布表单")
	ui.boxRow("  登录在全局设置完成 · 方向键移动，Enter 编辑 / 选择")
	ui.boxSeparator("├", "┤")
	rows := []string{
		"包名        " + fallback(form.name, "未填写（inspect 失败时必填）"),
		"版本        " + fallback(form.version, "未填写（inspect 失败时必填）"),
		"channel     " + fallback(form.channel, "dev"),
		"发布系统    " + fallback(config.DisplayOS(form.os), "未选择"),
		"发布架构    " + fallback(config.DisplayArch(form.arch), "未选择"),
		"构建类型    " + fallback(config.DisplayBuildType(form.buildType), "Release"),
		"远程仓库    " + fallback(form.remote, "跟随全局"),
		"备注        " + fallback(form.note, "可选"),
	}
	for index, row := range rows {
		ui.boxRow("  " + settingRow(ui, index, row, selected == index))
	}
	ui.boxSeparator("├", "┤")
	ui.boxRow("  " + cursorAction(ui, "生成预览", selected == 8) + "   " + cursorAction(ui, "返回", selected == 9))
	ui.boxSeparator("└", "┘")
}

func (controller *cursorController) publishEnter(ctx context.Context, form *publishCursorForm, selected int) bool {
	if selected < 8 {
		switch selected {
		case 0:
			if value, ok := controller.editText("发布表单", "包名", form.name); ok {
				form.name = value
			}
		case 1:
			if value, ok := controller.editText("发布表单", "版本", form.version); ok {
				form.version = value
			}
		case 2:
			if value, ok := controller.editText("发布表单", "channel", form.channel); ok {
				form.channel = value
			}
		case 3:
			value, ok := controller.choicePage("选择发布系统", []choiceOption{{config.OSWindows, "Windows"}, {config.OSLinux, "Linux"}, {config.OSKylin, "麒麟"}}, form.os)
			if ok {
				form.os = value
			}
		case 4:
			value, ok := controller.choicePage("选择发布架构", []choiceOption{{config.ArchX86, "x86 32 位"}, {config.ArchX64, "x64 64 位"}, {config.ArchARM, "ARM 32 位"}, {config.ArchARM64, "ARM 64 位"}}, form.arch)
			if ok {
				form.arch = value
			}
		case 5:
			value, ok := controller.choicePage("选择构建类型", []choiceOption{{config.BuildTypeRelease, "Release"}, {config.BuildTypeDebug, "Debug"}}, form.buildType)
			if ok {
				form.buildType = value
			}
		case 6:
			if value, ok := controller.editText("发布表单", "远程仓库", form.remote); ok {
				form.remote = value
			}
		case 7:
			if value, ok := controller.editText("发布表单", "备注", form.note); ok {
				form.note = value
			}
		}
		return true
	}
	if selected == 9 {
		return false
	}
	if strings.TrimSpace(form.name) == "" || strings.TrimSpace(form.version) == "" {
		controller.showResult("发布预览", workflow.Report{}, fmt.Errorf("包名和版本不能为空"))
		return true
	}
	if report, err := controller.ui.app.SaveProjectSettings(workflow.ProjectSettingsInput{PublishOS: form.os, PublishArch: form.arch, PublishBuildType: form.buildType, Channel: form.channel, Remote: form.remote}); err != nil {
		controller.ui.storeReport(report, err)
		controller.showResult("发布预览", report, err)
		return true
	}
	request := workflow.PublishRequest{Name: form.name, Version: form.version, Channel: form.channel, Remote: form.remote, OS: form.os, Arch: form.arch, BuildType: form.buildType, Note: form.note, DryRun: true}
	preview, err := controller.ui.app.PublishPackage(ctx, request)
	controller.ui.storeReport(preview, err)
	if err != nil {
		controller.showResult("发布预览", preview, err)
		return true
	}
	if controller.confirmPublish(preview) {
		request.DryRun = false
		controller.actionResult(ctx, "发布包", func() (workflow.Report, error) { return controller.ui.app.PublishPackage(ctx, request) })
	}
	return true
}

func (controller *cursorController) confirmPublish(report workflow.Report) bool {
	selected := 1
	for {
		data := reportMap(report)
		controller.ui.cursorScreen("确认发布")
		controller.ui.boxRow("  " + fallback(stringValue(data["reference"]), "未生成引用") + " → " + fallback(stringValue(data["remote"]), "未配置"))
		controller.ui.boxRow("  平台 " + fallback(stringValue(data["os"]), "-") + " / " + fallback(stringValue(data["arch"]), "-"))
		controller.ui.boxRow("  channel " + fallback(stringValue(data["channel"]), "-"))
		controller.ui.boxRow("  $ " + clip(stringValue(data["command"]), innerWidth-6))
		controller.ui.boxSeparator("├", "┤")
		controller.ui.boxRow("  " + cursorAction(controller.ui, "取消", selected == 0) + "   " + cursorAction(controller.ui, "发布", selected == 1))
		controller.ui.boxSeparator("└", "┘")
		key, err := controller.keys.next()
		if err != nil || key.kind == keyEscape || (key.kind == keyCharacter && (key.char == 'q' || key.char == 'Q')) {
			return false
		}
		switch key.kind {
		case keyLeft, keyRight, keyUp, keyDown:
			selected = 1 - selected
		case keyEnter:
			return selected == 1
		}
	}
}
