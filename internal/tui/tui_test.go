package tui

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"conan-cli/internal/config"
	"conan-cli/internal/workflow"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
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

func TestSettingsFieldTableDrivesBothPages(t *testing.T) {
	if count := len(settingsFieldsFor("global")); count != 5 {
		t.Fatalf("global field count = %d, want 5", count)
	}
	if count := len(settingsFieldsFor("project")); count != 16 {
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

// bufioReader keeps the interaction test focused on the TUI without making
// the production input API expose a concrete reader type.
func bufioReader(input string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(input))
}

// ---------------------------------------------------------------------------
// BubbleTea 交互模式（ADR 0002）：fake API 驱动，不依赖真实 Conan 与网络。
// ---------------------------------------------------------------------------

type btFakeAPI struct {
	tuiAPI
	project     *config.Project
	status      workflow.Report
	saved       []workflow.ProjectSettingsInput
	globalSaved []workflow.GlobalSettingsInput
	logins      []string
	published   []workflow.PublishRequest
	added       []string
}

func btFakeStatus(project *config.Project, packages []workflow.PackageInfo) workflow.Report {
	return workflow.Report{OK: true, Action: "status", Data: map[string]any{
		"initialized": true,
		"project":     project,
		"global":      config.GlobalView{},
		"packages":    packages,
		"conanfile":   "conanfile.txt",
	}}
}

func (fake *btFakeAPI) Status(ctx context.Context) (workflow.Report, error) {
	return fake.status, nil
}

func (fake *btFakeAPI) Project() (*config.Project, error) {
	return fake.project, nil
}

func (fake *btFakeAPI) Analyze(ctx context.Context, osName, arch, buildType string) (workflow.Report, error) {
	return workflow.Report{OK: true, Action: "analyze", Message: "依赖分析完成", Data: map[string]any{
		"dependencies": []workflow.DependencyRow{},
	}}, nil
}

func (fake *btFakeAPI) Doctor(ctx context.Context) (workflow.Report, error) {
	return workflow.Report{OK: true, Action: "doctor", Checks: []workflow.Check{{Name: "conan", OK: true, Detail: "2.x"}}}, nil
}

func (fake *btFakeAPI) ConfigTest(ctx context.Context) (workflow.Report, error) {
	return workflow.Report{OK: true, Action: "config-test"}, nil
}

func (fake *btFakeAPI) SaveProjectSettings(input workflow.ProjectSettingsInput) (workflow.Report, error) {
	fake.saved = append(fake.saved, input)
	return workflow.Report{OK: true, Action: "settings-project"}, nil
}

func (fake *btFakeAPI) PublishPackage(ctx context.Context, request workflow.PublishRequest) (workflow.Report, error) {
	fake.published = append(fake.published, request)
	action := "publish"
	if request.DryRun {
		action = "publish-preview"
	}
	return workflow.Report{OK: true, Action: action, Message: "已发布 1 个组件", Data: map[string]any{
		"package":   request.Package,
		"reference": request.Package + "/" + request.Version,
	}}, nil
}

// realPublishes 过滤掉 dry-run 预览请求，只留确认后真正上传的。
func (fake *btFakeAPI) realPublishes() []workflow.PublishRequest {
	var real []workflow.PublishRequest
	for _, request := range fake.published {
		if !request.DryRun {
			real = append(real, request)
		}
	}
	return real
}

func (fake *btFakeAPI) Add(dependency string) (workflow.Report, error) {
	fake.added = append(fake.added, dependency)
	return workflow.Report{OK: true, Action: "add", Message: "dependency added"}, nil
}

func (fake *btFakeAPI) SaveGlobalSettings(ctx context.Context, input workflow.GlobalSettingsInput) (workflow.Report, error) {
	fake.globalSaved = append(fake.globalSaved, input)
	return workflow.Report{OK: true, Action: "config", Message: "全局设置已保存"}, nil
}

func (fake *btFakeAPI) ConfigLogin(ctx context.Context, password string) (workflow.Report, error) {
	fake.logins = append(fake.logins, password)
	return workflow.Report{OK: true, Action: "config-login", Message: "已登录"}, nil
}

func (fake *btFakeAPI) CatalogFilter(ctx context.Context, filter workflow.CatalogFilter) (workflow.Report, error) {
	return workflow.Report{OK: true, Action: "catalog", Data: map[string]any{
		"packages": []workflow.CatalogPackage{},
	}}, nil
}

func btNewTestModel(t *testing.T) (*btModel, *btFakeAPI) {
	t.Helper()
	project := config.NewProject(t.TempDir())
	project.Name = "demo"
	project.Platform.Consume = config.PlatformSpec{OS: config.OSKylin, Arch: config.ArchX64, BuildType: config.BuildTypeRelease}
	fake := &btFakeAPI{
		project: project,
		status: btFakeStatus(project, []workflow.PackageInfo{
			{Name: "demo", Version: "1.0.0", Source: "registered", HasArtifacts: true},
		}),
	}
	m := newBTModel(fake)
	m.status = btParseStatus(fake.status)
	return m, fake
}

// btStep 注入按键并执行返回的命令（含结果回灌），模拟一个完整交互步。
func btStep(m *btModel, msg tea.Msg) *btModel {
	model, cmd := m.Update(msg)
	next, _ := model.(*btModel)
	return btExecCmd(next, cmd)
}

func btExecCmd(m *btModel, cmd tea.Cmd) *btModel {
	for hops := 0; cmd != nil && hops < 32; hops++ {
		msg := cmd()
		if msg == nil {
			return m
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, item := range batch {
				m = btExecCmd(m, item)
			}
			return m
		}
		if _, isTick := msg.(spinner.TickMsg); isTick {
			model, _ := m.Update(msg)
			next, _ := model.(*btModel)
			return next
		}
		model, next := m.Update(msg)
		m, _ = model.(*btModel)
		cmd = next
	}
	return m
}

func btPress(m *btModel, keys ...string) *btModel {
	for _, key := range keys {
		m = btStep(m, btKey(key))
	}
	return m
}

func TestBTViewRendersTabsAndHeader(t *testing.T) {
	m, _ := btNewTestModel(t)
	view := m.View()
	for _, want := range []string{"Conan 控制台", "拉取依赖", "仓库", "依赖", "发布", "设置", "诊断", "麒麟"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view does not contain %q:\n%s", want, view)
		}
	}
}

func TestBTNumberKeysSwitchTabs(t *testing.T) {
	m, _ := btNewTestModel(t)
	m = btPress(m, "2")
	if m.active != tabCatalog {
		t.Fatalf("active tab = %d, want catalog", m.active)
	}
	if !strings.Contains(m.View(), "输入包名回车查询") {
		t.Fatal("catalog view not rendered after switching tabs")
	}
	m = btPress(m, "right")
	if m.active != tabDeps {
		t.Fatalf("active tab = %d, want deps", m.active)
	}
	m = btPress(m, "6")
	if m.active != tabDoctor {
		t.Fatalf("active tab = %d, want doctor", m.active)
	}
}

func TestBTComboCycleSavesQuietly(t *testing.T) {
	m, fake := btNewTestModel(t)
	// 项目当前消费端是 kylin；向右循环应落到 windows 并静默保存。
	m = btPress(m, "e", "right")
	if len(fake.saved) != 1 {
		t.Fatalf("quiet saves = %d, want 1", len(fake.saved))
	}
	if got := fake.saved[0].OS; got != config.OSWindows {
		t.Fatalf("saved os = %q, want windows", got)
	}
}

func TestBTMissCardAppearsForMissingBinaries(t *testing.T) {
	m, _ := btNewTestModel(t)
	m.height = 34 // 卡片边框占行，矮屏下会触发截断
	m.analyzeLoaded = true
	m.analyzeRows = []workflow.DependencyRow{
		{Reference: "qtutils/1.0", Status: "missing_binary", Detail: "没有 Kylin/x86 的预编译二进制"},
	}
	view := m.View()
	for _, want := range []string{"仓库没有这套制品", "不要本机编译", "--build=missing"} {
		if !strings.Contains(view, want) {
			t.Fatalf("download view does not contain %q:\n%s", want, view)
		}
	}
}

func TestBTPublishShowsConfirmModalThenPublishes(t *testing.T) {
	m, fake := btNewTestModel(t)
	// p 触发 dry-run 预览；确认框应在预览回来后才出现。
	m = btPress(m, "4", "p")
	if m.confirm == nil {
		t.Fatal("confirm modal did not open after the dry-run preview")
	}
	if len(fake.published) != 1 || !fake.published[0].DryRun {
		t.Fatalf("preview requests = %#v, want one dry run before confirmation", fake.published)
	}
	view := m.View()
	for _, want := range []string{"确认发布", "平台", "channel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("confirm modal does not contain %q:\n%s", want, view)
		}
	}
	m = btPress(m, "y")
	real := fake.realPublishes()
	if len(real) != 1 {
		t.Fatalf("real published requests = %d, want 1 (all = %#v)", len(real), fake.published)
	}
	request := real[0]
	if request.Package != "demo" || request.OS != config.OSKylin || request.Arch != config.ArchX64 {
		t.Fatalf("publish request = %#v", request)
	}
	if request.DryRun {
		t.Fatal("publish request must not be a dry run after modal confirmation")
	}
}

func TestBTSettingsPasswordIsMasked(t *testing.T) {
	m, _ := btNewTestModel(t)
	m = btPress(m, "5")
	m.settings.row = 7
	m = btPress(m, "enter")
	m = btPress(m, "s", "3", "c", "r", "e", "t")
	view := m.View()
	if strings.Contains(view, "s3cret") {
		t.Fatalf("password leaked into the view:\n%s", view)
	}
	if !strings.Contains(view, strings.Repeat("*", 6)) {
		t.Fatalf("password not masked in the view:\n%s", view)
	}
	m = btPress(m, "esc")
	view = m.View()
	if strings.Contains(view, "s3cret") {
		t.Fatalf("password leaked after leaving edit:\n%s", view)
	}
	if !strings.Contains(view, strings.Repeat("*", 6)) {
		t.Fatalf("password not masked after leaving edit:\n%s", view)
	}
}

func TestBTDepsAddUsesReference(t *testing.T) {
	m, fake := btNewTestModel(t)
	m = btPress(m, "3", "a")
	m.deps.addInput.SetValue("qtutils/1.0")
	m = btPress(m, "enter")
	if len(fake.added) != 1 || fake.added[0] != "qtutils/1.0" {
		t.Fatalf("added = %v, want [qtutils/1.0]", fake.added)
	}
}

func TestBTMouseClickSwitchesTab(t *testing.T) {
	m, _ := btNewTestModel(t)
	view := m.View()
	var region btRegion
	for _, candidate := range m.regions {
		if candidate.id == "tab:1" {
			region = candidate
		}
	}
	if region.w == 0 {
		t.Fatalf("tab region not registered:\n%s", view)
	}
	m = btStep(m, tea.MouseMsg{X: region.x + 1, Y: region.y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if m.active != tabCatalog {
		t.Fatalf("active tab after click = %d, want catalog", m.active)
	}
}

func TestBTMouseClickActionButton(t *testing.T) {
	m, _ := btNewTestModel(t)
	m.analyzeLoaded = false
	m.View()
	var region btRegion
	for _, candidate := range m.regions {
		if candidate.id == "btn:a" {
			region = candidate
		}
	}
	if region.w == 0 {
		t.Fatal("action button region not registered")
	}
	m = btStep(m, tea.MouseMsg{X: region.x + 1, Y: region.y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if !m.analyzeLoaded && !m.busy[btJobAnalyze] {
		t.Fatal("clicking 检查依赖 button did not trigger analyze")
	}
}

func TestBTMouseWheelScrolls(t *testing.T) {
	m, _ := btNewTestModel(t)
	m.status.packages = []workflow.PackageInfo{
		{Name: "a"}, {Name: "b"}, {Name: "c"},
	}
	m = btPress(m, "4")
	m.publish.cursor = 0
	m = btStep(m, tea.MouseMsg{X: 1, Y: 8, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	if m.publish.cursor != 1 {
		t.Fatalf("cursor after wheel down = %d, want 1", m.publish.cursor)
	}
}

func TestBTThemeCycleChangesPalette(t *testing.T) {
	m, _ := btNewTestModel(t)
	if btCurrentTheme.name != "dark" {
		t.Fatalf("initial theme = %q, want dark (default)", btCurrentTheme.name)
	}
	m = btPress(m, "T")
	if btCurrentTheme.name != "light" {
		t.Fatalf("theme after one cycle = %q, want light", btCurrentTheme.name)
	}
	if !strings.Contains(m.View(), "亮色") {
		t.Fatal("theme label not shown in footer")
	}
	m = btPress(m, "T")
	if btCurrentTheme.name != "transparent" {
		t.Fatalf("theme after two cycles = %q, want transparent", btCurrentTheme.name)
	}
	m = btPress(m, "T")
	if btCurrentTheme.name != "nord" {
		t.Fatalf("theme after three cycles = %q, want nord", btCurrentTheme.name)
	}
	m = btPress(m, "T")
	if btCurrentTheme.name != "dark" {
		t.Fatalf("theme should wrap back to dark, got %q", btCurrentTheme.name)
	}
}

func TestBTViewFillsScreen(t *testing.T) {
	m, _ := btNewTestModel(t)
	m.width, m.height = 100, 30
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 30 {
		t.Fatalf("view height = %d lines, want 30", len(lines))
	}
	for index, line := range lines {
		if got := lipgloss.Width(line); got != 100 {
			t.Fatalf("line %d width = %d, want 100 (theme %s)", index, got, btCurrentTheme.name)
		}
	}
	if !strings.Contains(view, "拉取依赖") {
		t.Fatal("view lost its content after full-screen padding")
	}
}

// TestBTViewExactFrameAcrossTabsAndSizes 保证任何 Tab/尺寸下帧都精确等于终端
// 高、每行都精确等于终端宽：行宽溢出会被 lipgloss 折行，把 toast/footer 推出
// 屏幕并让鼠标区域整体错位（皮肤看起来没铺满、点击点不中）。
func TestBTViewExactFrameAcrossTabsAndSizes(t *testing.T) {
	themes := []int{0, 3}
	sizes := [][2]int{{100, 30}, {80, 24}, {60, 20}, {50, 10}}
	for _, theme := range themes {
		btApplyTheme(theme)
		for tab := btTab(0); tab < tabCount; tab++ {
			for _, size := range sizes {
				m, _ := btNewTestModel(t)
				m.width, m.height = size[0], size[1]
				m.switchTab(tab)
				for _, overlay := range []string{"none", "combo", "confirm"} {
					mm := m
					switch overlay {
					case "combo":
						mm.combo.open = true
					case "confirm":
						mm.confirm = &btConfirm{}
					}
					lines := strings.Split(mm.View(), "\n")
					if len(lines) != size[1] {
						t.Fatalf("theme=%d tab=%d %dx%d overlay=%s: %d lines, want %d",
							theme, tab, size[0], size[1], overlay, len(lines), size[1])
					}
					for index, line := range lines {
						if got := lipgloss.Width(line); got != size[0] {
							t.Fatalf("theme=%d tab=%d %dx%d overlay=%s: line %d width = %d, want %d",
								theme, tab, size[0], size[1], overlay, index, got, size[0])
						}
					}
				}
			}
		}
	}
	btApplyTheme(0)
}

// TestBTMouseClickFocusesSettingsInput 点设置行的任意位置应直接进入编辑并聚焦输入框。
func TestBTMouseClickFocusesSettingsInput(t *testing.T) {
	m, _ := btNewTestModel(t)
	m.width, m.height = 80, 24
	m = btPress(m, "5")
	m.View()
	var region btRegion
	for _, candidate := range m.regions {
		if candidate.id == "setrow:2" {
			region = candidate
		}
	}
	if region.w == 0 {
		t.Fatal("settings row region not registered")
	}
	m = btStep(m, tea.MouseMsg{X: region.x + 1, Y: region.y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if !m.settings.editing || m.settings.row != 2 {
		t.Fatalf("after click editing=%v row=%d, want editing row 2", m.settings.editing, m.settings.row)
	}
	if !m.settings.libDirs.Focused() {
		t.Fatal("clicking settings row did not focus its input")
	}
	m = btStep(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("lib,bin")})
	if m.settings.libDirs.Value() != "lib,bin" {
		t.Fatalf("typed text did not land in input, got %q", m.settings.libDirs.Value())
	}
}

// TestBTMouseClickFocusesCatalogSearch 点仓库 Tab 的搜索框应直接聚焦，可立即输入。
func TestBTMouseClickFocusesCatalogSearch(t *testing.T) {
	m, _ := btNewTestModel(t)
	m.width, m.height = 80, 24
	m = btPress(m, "2")
	m.View()
	var region btRegion
	for _, candidate := range m.regions {
		if candidate.id == "catsearch" {
			region = candidate
		}
	}
	if region.w == 0 {
		t.Fatal("catalog search region not registered")
	}
	m = btStep(m, tea.MouseMsg{X: region.x + 1, Y: region.y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if !m.catalog.search.Focused() {
		t.Fatal("clicking search box did not focus it")
	}
	m = btStep(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("qt")})
	if m.catalog.search.Value() != "qt" {
		t.Fatalf("typed text did not land in search, got %q", m.catalog.search.Value())
	}
}

// btCoverageChecker 逐行解析 SGR 状态，检查每个可打印单元格是否有底色。
type btCoverageChecker struct {
	bgSet bool
}

func (c *btCoverageChecker) apply(seq string) {
	params := strings.Split(strings.Trim(seq, ";"), ";")
	if params[0] == "" || params[0] == "0" {
		c.bgSet = false
	}
	for index := 0; index < len(params); index++ {
		switch params[index] {
		case "48", "58", "4":
			if params[index] == "48" {
				c.bgSet = true
			}
			// 跳过 48;2;r;g;b / 48;5;n 的参数
			if index+1 < len(params) {
				if params[index+1] == "5" {
					index += 2
				} else if params[index+1] == "2" {
					index += 4
				}
			}
		case "49":
			c.bgSet = false
		}
	}
}

// TestBTViewEveryCellHasBackground 断言渲染帧里每个可打印单元格都带底色：
// 行内 \x1b[0m 复位会杀掉页面底色，曾导致大片区域裸露终端默认背景。
func TestBTViewEveryCellHasBackground(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	for _, theme := range []int{0, 1, 3} {
		btApplyTheme(theme)
		for tab := btTab(0); tab < tabCount; tab++ {
			m, _ := btNewTestModel(t)
			m.width, m.height = 80, 24
			m.switchTab(tab)
			m.combo.open = true
			m.toast = "已保存"
			view := m.View()
			for lineIndex, line := range strings.Split(view, "\n") {
				checker := &btCoverageChecker{bgSet: false}
				printable := 0
				for index := 0; index < len(line); {
					if line[index] == '\x1b' && index+1 < len(line) && line[index+1] == '[' {
						index += 2
						start := index
						for index < len(line) && line[index] != 'm' {
							index++
						}
						checker.apply(line[start:index])
						index++
						continue
					}
					if line[index] == ' ' {
						if !checker.bgSet {
							t.Fatalf("theme=%d tab=%d line %d col %d: 空格无底色", theme, tab, lineIndex, printable)
						}
						printable++
						index++
						continue
					}
					if !checker.bgSet {
						t.Fatalf("theme=%d tab=%d line %d col %d: 字符 %q 无底色", theme, tab, lineIndex, printable, line[index])
					}
					_, size := utf8.DecodeRuneInString(line[index:])
					printable += max(1, size)
					index += size
				}
			}
		}
	}
	btApplyTheme(0)
}

// TestBTSelectedRowHighlighted 断言选中行整行带高亮底色并铺满终端宽。
func TestBTSelectedRowHighlighted(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m, _ := btNewTestModel(t)
	m.width, m.height = 80, 24
	m = btPress(m, "5")
	view := m.View()
	selSeq := btSGRSeq(btStyleRowSel)
	if selSeq == "" {
		t.Fatal("dark theme should define a selection style")
	}
	var selectedLine string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Conan 包名") {
			selectedLine = line
			break
		}
	}
	if selectedLine == "" {
		t.Fatal("settings row not found in view")
	}
	if !strings.Contains(selectedLine, selSeq) {
		t.Fatal("selected settings row lacks the selection background")
	}
	if got := lipgloss.Width(selectedLine); got != 80 {
		t.Fatalf("selected row width = %d, want 80", got)
	}

	// 未选中的行不应带选中底色。
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "仓库地址") && strings.Contains(line, selSeq) {
			t.Fatal("unselected row should not carry the selection background")
		}
	}

	// 组合弹窗与发布页的选中行同样整行高亮。
	m = btPress(m, "4")
	m.publish.cursor = 0
	view = m.View()
	hits := 0
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, selSeq) {
			hits++
		}
	}
	if hits == 0 {
		t.Fatal("publish tab selected row not highlighted")
	}
}

func TestBTSaveAndLoginWaitsForSave(t *testing.T) {
	m, fake := btNewTestModel(t)
	m = btPress(m, "5")
	m.settings.repoURL.SetValue("https://nexus.example.com/repository/conan")
	m.settings.username.SetValue("alice")
	m.settings.password.SetValue("s3cret")
	m = btPress(m, "l")
	if len(fake.globalSaved) != 1 {
		t.Fatalf("saves = %d, want 1", len(fake.globalSaved))
	}
	if fake.globalSaved[0].Password != "s3cret" {
		t.Fatalf("saved password = %q", fake.globalSaved[0].Password)
	}
	if len(fake.logins) != 1 || fake.logins[0] != "s3cret" {
		t.Fatalf("logins = %v, want [s3cret]", fake.logins)
	}
	if m.settings.password.Value() != "" {
		t.Fatal("password field should be cleared after login")
	}
}

func TestBTPublishAPublishesAllWhenUnchecked(t *testing.T) {
	m, fake := btNewTestModel(t)
	m.status.packages = []workflow.PackageInfo{
		{Name: "demo", Version: "1.0.0", HasArtifacts: true},
		{Name: "extra", Version: "2.0.0", HasArtifacts: true},
	}
	m = btPress(m, "4", "A", "y")
	real := fake.realPublishes()
	if len(real) != 1 || !real[0].All {
		t.Fatalf("real published = %#v, want one All=true request (all = %#v)", real, fake.published)
	}
}

func TestBTPublishAPublishesCheckedOnly(t *testing.T) {
	m, fake := btNewTestModel(t)
	m.status.packages = []workflow.PackageInfo{
		{Name: "demo", Version: "1.0.0", HasArtifacts: true},
		{Name: "extra", Version: "2.0.0", HasArtifacts: true},
		{Name: "lib", Version: "3.0.0", HasArtifacts: true},
	}
	m = btPress(m, "4")
	m.publish.checked["extra"] = true
	m = btPress(m, "A")
	if m.confirm == nil {
		t.Fatal("expected confirm modal")
	}
	view := m.View()
	// 预览队列按勾选构造：确认框只描述 extra，不含未勾选的 lib。
	if !strings.Contains(view, "extra") {
		t.Fatalf("confirm does not describe checked packages:\n%s", view)
	}
	if strings.Contains(view, "lib") {
		t.Fatal("unchecked package listed in confirm")
	}
	m = btPress(m, "y")
	real := fake.realPublishes()
	if len(real) != 1 || real[0].Package != "extra" || real[0].All {
		t.Fatalf("real published = %#v, want extra only (all = %#v)", real, fake.published)
	}
}

func TestBTPublishCheckedCountIgnoresUnchecked(t *testing.T) {
	m, _ := btNewTestModel(t)
	m.status.packages = []workflow.PackageInfo{{Name: "demo"}, {Name: "extra"}}
	m = btPress(m, "4")
	m.publish.cursor = 0
	m = btPress(m, " ", " ")
	view := m.View()
	if strings.Contains(view, "已勾选 1") {
		t.Fatalf("unchecked leftover counted:\n%s", view)
	}
}

func TestBTComboEditorVisibleOnPublishTab(t *testing.T) {
	m, _ := btNewTestModel(t)
	m = btPress(m, "4", "e")
	if !m.combo.open {
		t.Fatal("combo not open")
	}
	if !strings.Contains(m.View(), "组合编辑") {
		t.Fatalf("combo editor not visible on publish tab:\n%s", m.View())
	}
}

func TestBTViewNilStatusDoesNotPanic(t *testing.T) {
	m, _ := btNewTestModel(t)
	m.status = nil
	for tab := btTab(0); tab < tabCount; tab++ {
		m.active = tab
		_ = m.View()
	}
	m.active = tabPublish
	m = btPress(m, "down", "p", "A")
}

func TestBTMouseRegionsStayInContentBox(t *testing.T) {
	m, _ := btNewTestModel(t)
	m.width, m.height = 80, 12
	m.switchTab(tabSettings)
	m.View()
	contentBottom := btLayoutTop + max(0, m.height-btLayoutTop-btLayoutBottom)
	for _, region := range m.regions {
		if strings.HasPrefix(region.id, "setrow:") && region.y >= contentBottom {
			t.Fatalf("region %s y=%d overlaps chrome (contentBottom=%d)", region.id, region.y, contentBottom)
		}
	}
}

func TestBTCatalogAddWithoutExpandToasts(t *testing.T) {
	m, _ := btNewTestModel(t)
	m = btPress(m, "2")
	m.catalog.packages = []workflow.CatalogPackage{{Name: "fmt", Binaries: []workflow.CatalogPackageBinary{{Version: "10.0", Reference: "fmt/10.0"}}}}
	m.catalog.loaded = true
	m.catalog.cursor = 0
	m.catalog.expanded = -1
	m = btPress(m, "a")
	if m.toast == "" {
		t.Fatal("expected toast when adding without selecting a binary")
	}
	if m.catalog.expanded != 0 {
		t.Fatalf("expanded = %d, want 0", m.catalog.expanded)
	}
}

func TestBTMouseClickCatalogFilterCycles(t *testing.T) {
	m, _ := btNewTestModel(t)
	m = btPress(m, "2")
	m.View()
	var region btRegion
	for _, candidate := range m.regions {
		if candidate.id == "catfilter:0" {
			region = candidate
		}
	}
	if region.w == 0 {
		t.Fatal("catalog filter region not registered")
	}
	m = btStep(m, tea.MouseMsg{X: region.x, Y: region.y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if m.catalog.osIdx != 1 {
		t.Fatalf("osIdx = %d, want 1 after clicking system filter", m.catalog.osIdx)
	}
}

func TestBTDoctorSummaryIgnoresHiddenChecks(t *testing.T) {
	got := btDoctorSummary([]workflow.Check{
		{Name: "conan", OK: true},
		{Name: "profiles", OK: false},
	})
	if !strings.Contains(got, "就绪") {
		t.Fatalf("summary = %q, hidden failure should not count", got)
	}
}
