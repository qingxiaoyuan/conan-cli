package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"conan-cli/internal/config"
	"conan-cli/internal/workflow"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// 根模型：信息架构对齐 VS Code 控制台——六个 Tab + 头部状态徽标 + 并行刷新。
// ---------------------------------------------------------------------------

type btTab int

const (
	tabDownload btTab = iota
	tabCatalog
	tabDeps
	tabPublish
	tabSettings
	tabDoctor
	tabCount
)

var btTabTitles = [tabCount]string{"拉取依赖", "仓库", "依赖", "发布", "设置", "诊断"}

// btStatus 是 app.Status 输出的界面投影。
type btStatus struct {
	initialized bool
	project     *config.Project
	global      config.GlobalView
	packages    []workflow.PackageInfo
	conanfile   string
}

func (s *btStatus) projectName() string {
	if s == nil || s.project == nil || s.project.Name == "" {
		return "未初始化项目"
	}
	return s.project.Name
}

func (s *btStatus) proj() *config.Project {
	if s == nil || s.project == nil {
		return &config.Project{}
	}
	return s.project
}

type btModel struct {
	ctx    context.Context
	api    tuiAPI
	active btTab
	width  int
	height int

	spinner spinner.Model

	status *btStatus
	// 依赖分析
	analyzeRows   []workflow.DependencyRow
	analyzeMsg    string
	analyzePlat   string
	analyzeLoaded bool
	// 诊断
	doctorChecks []workflow.Check

	busy map[btJob]bool

	// 连接探测（30s 缓存，对齐 VS Code 插件）
	probeOK bool
	probeAt time.Time

	toast    string
	toastBad bool

	confirm *btConfirm
	combo   btCombo

	// 保存并登录：必须等全局设置写盘后再登录，避免并行读到旧密码。
	pendingLogin         bool
	pendingLoginPassword string

	catalog  btCatalogModel
	deps     btDepsModel
	publish  btPublishModel
	settings btSettingsModel
	doctor   btDoctorModel
	logLines []string

	// 鼠标点击区域：屏幕绝对坐标（由 View 每帧重建）。
	regions []btRegion
}

func newBTModel(api tuiAPI) *btModel {
	btInitTheme()
	m := &btModel{
		ctx:     context.Background(),
		api:     api,
		status:  &btStatus{},
		width:   80,
		height:  24,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		busy:    map[btJob]bool{},
	}
	m.catalog = newBTCatalogModel()
	m.deps = newBTDepsModel()
	m.publish = newBTPublishModel()
	m.settings = newBTSettingsModel()
	m.doctor = newBTDoctorModel()
	m.combo = newBTCombo("consume")
	m.restyleCursors()
	return m
}

// allInputs 汇总模型内所有 textinput（主题切换时需要重建光标样式）。
func (m *btModel) allInputs() []*textinput.Model {
	inputs := m.settings.inputs()
	inputs = append(inputs, m.publish.inputs()...)
	return append(inputs, &m.catalog.search, &m.deps.addInput, &m.combo.text)
}

// restyleCursors 让光标样式跟随主题：块光标保持 reverse（在高亮行上自动变成
// 与选中底色反色的亮块，避免光标条和选中色混成一团）；闪烁隐藏期间字符用
// 主题前景色。
func (m *btModel) restyleCursors() {
	for _, input := range m.allInputs() {
		input.Cursor.Style = lipgloss.NewStyle()
		input.Cursor.TextStyle = lipgloss.NewStyle().Foreground(btCurrentTheme.fg)
	}
}

func (m *btModel) statusOrEmpty() *btStatus {
	if m.status == nil {
		return &btStatus{}
	}
	return m.status
}

func (m *btModel) packages() []workflow.PackageInfo {
	return m.statusOrEmpty().packages
}

// run 记录 busy 状态并启动任务；同一任务进行中不重复触发。
// 从空闲重新开工时带上 spinner.Tick，否则首轮刷新结束后动画会停住。
func (m *btModel) run(job btJob, work func(ctx context.Context) (workflow.Report, error)) tea.Cmd {
	if m.busy[job] {
		return nil
	}
	idle := len(m.busy) == 0
	m.busy[job] = true
	cmd := btRun(m.ctx, job, work)
	if idle {
		return tea.Batch(m.spinner.Tick, cmd)
	}
	return cmd
}

func (m *btModel) refreshAll() tea.Cmd {
	cmds := []tea.Cmd{
		m.run(btJobStatus, m.api.Status),
		m.run(btJobDoctor, m.api.Doctor),
		m.run(btJobAnalyze, func(ctx context.Context) (workflow.Report, error) {
			return m.api.Analyze(ctx, "", "", "")
		}),
		m.probeCmd(),
	}
	if m.active == tabCatalog {
		cmds = append(cmds, m.catalogCmd())
	}
	return tea.Batch(cmds...)
}

func (m *btModel) probeCmd() tea.Cmd {
	if time.Since(m.probeAt) < 30*time.Second {
		return nil
	}
	m.probeAt = time.Now()
	return m.run(btJobProbe, m.api.ConfigTest)
}

func (m *btModel) analyzeCmd() tea.Cmd {
	return m.run(btJobAnalyze, func(ctx context.Context) (workflow.Report, error) {
		return m.api.Analyze(ctx, "", "", "")
	})
}

func (m *btModel) installCmd() tea.Cmd {
	spec := m.status.proj().Platform.Consume
	return m.run(btJobInstall, func(ctx context.Context) (workflow.Report, error) {
		return m.api.InstallPlatform(ctx, workflow.InstallRequest{
			OS:        spec.OS,
			Arch:      spec.Arch,
			BuildType: spec.BuildType,
		})
	})
}

func (m *btModel) setToast(bad bool, format string, args ...any) {
	m.toast = fmt.Sprintf(format, args...)
	m.toastBad = bad
}

func (m *btModel) missingDependencies() []workflow.DependencyRow {
	var missing []workflow.DependencyRow
	for _, row := range m.analyzeRows {
		if row.Status != "found" && row.Status != "unknown" && row.Status != "" {
			missing = append(missing, row)
		}
	}
	return missing
}

// ---------------------------------------------------------------------------
// bubbletea.Model
// ---------------------------------------------------------------------------

func (m *btModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.refreshAll())
}

func (m *btModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 {
			m.width = msg.Width
		}
		if msg.Height > 0 {
			m.height = msg.Height
		}
		m.catalog.resize(m.width)
		m.publish.resize(m.width)
		m.settings.resize(m.width)
		m.comboResize(m.width)
		m.doctor.resize(m.width, m.contentHeight())
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if len(m.busy) == 0 {
			return m, nil
		}
		return m, cmd

	case btReportMsg:
		return m.applyReport(msg)

	case tea.MouseMsg:
		return m.mouseUpdate(msg)

	case tea.KeyMsg:
		return m.keyUpdate(msg)
	}
	return m, nil
}

func (m *btModel) keyUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirm != nil {
		return m.confirmKey(msg)
	}
	if m.combo.open {
		return m.comboKey(msg)
	}

	inputFocused := m.tabInputFocused()
	if !inputFocused {
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		}
		switch key := msg.String(); key {
		case "q":
			return m, tea.Quit
		case "T":
			btCycleTheme()
			m.restyleCursors()
			m.setToast(false, "主题：%s", btCurrentTheme.label)
			return m, nil
		case "1", "2", "3", "4", "5", "6":
			index := int(key[0] - '1')
			m.switchTab(btTab(index))
			return m, nil
		case "left":
			m.switchTab((m.active + tabCount - 1) % tabCount)
			return m, nil
		case "right":
			m.switchTab((m.active + 1) % tabCount)
			return m, nil
		case "r":
			return m, m.refreshAll()
		}
	}

	switch m.active {
	case tabDownload:
		return m.downloadKey(msg)
	case tabCatalog:
		return m.catalogKey(msg)
	case tabDeps:
		return m.depsKey(msg)
	case tabPublish:
		return m.publishKey(msg)
	case tabSettings:
		return m.settingsKey(msg)
	case tabDoctor:
		return m.doctorKey(msg)
	}
	return m, nil
}

func (m *btModel) switchTab(tab btTab) {
	m.active = tab
	m.catalog.blur()
	m.deps.blur()
	m.publish.blur()
	m.settings.blur()
}

func (m *btModel) tabInputFocused() bool {
	switch m.active {
	case tabCatalog:
		return m.catalog.inputFocused()
	case tabDeps:
		return m.deps.inputFocused()
	case tabPublish:
		return m.publish.inputFocused()
	case tabSettings:
		return m.settings.inputFocused()
	}
	return false
}

// ---------------------------------------------------------------------------
// 任务结果
// ---------------------------------------------------------------------------

func (m *btModel) applyReport(msg btReportMsg) (tea.Model, tea.Cmd) {
	delete(m.busy, msg.job)
	report, err := msg.report, msg.err
	failed := err != nil || !report.OK

	message := report.Message
	if err != nil {
		message = err.Error()
		if report.Error != "" {
			message = report.Error
		}
	}
	if report.Output != "" {
		for _, line := range strings.Split(strings.TrimRight(report.Output, "\n"), "\n") {
			m.logLines = append(m.logLines, line)
		}
		if len(m.logLines) > 400 {
			m.logLines = m.logLines[len(m.logLines)-400:]
		}
	}

	var followup tea.Cmd
	switch msg.job {
	case btJobStatus:
		m.status = btParseStatus(report)
		m.publish.syncComponents(m.status)
		m.settings.syncForm(m.status)
	case btJobDoctor:
		m.doctorChecks = report.Checks
		m.doctor.setChecks(report.Checks)
	case btJobAnalyze:
		data := reportMap(report)
		if rows, ok := data["dependencies"].([]workflow.DependencyRow); ok {
			m.analyzeRows = rows
		}
		m.analyzeMsg = report.Message
		m.analyzePlat = stringValue(data["platform"])
		m.analyzeLoaded = true
	case btJobInstall:
		if failed {
			m.setToast(true, "拉取依赖失败：%s。请到“设置”确认已登录仓库。", localMessage(message))
		} else {
			m.setToast(false, "Conan 依赖已拉取。")
			followup = m.analyzeCmd()
		}
	case btJobPublish:
		if !m.publish.publishing {
			// dry-run 预览阶段：单个失败即终止预览；全部成功才弹确认框。
			if failed {
				m.publish.queue = nil
				m.publish.realQueue = nil
				m.setToast(true, "发布预览失败：%s", localMessage(message))
				break
			}
			if len(m.publish.queue) > 0 {
				m.publish.queue = m.publish.queue[1:]
			}
			if len(m.publish.queue) > 0 {
				m.setToast(false, "预览中，剩余 %s 个组件…", itoa(len(m.publish.queue)))
				followup = m.dequeuePublish()
			} else {
				followup = m.publishPreviewArrived(report, nil)
			}
			break
		}
		m.publish.applyReport(report, err)
		if failed {
			m.publish.queue = nil
			m.setToast(true, "发布失败：%s", localMessage(message))
			followup = m.run(btJobStatus, m.api.Status)
			break
		}
		if len(m.publish.queue) > 0 {
			m.publish.queue = m.publish.queue[1:]
		}
		if len(m.publish.queue) > 0 {
			m.setToast(false, "已发布，剩余 %s 个组件…", itoa(len(m.publish.queue)))
			followup = m.dequeuePublish()
		} else {
			m.setToast(false, "%s", fallback(report.Message, "发布完成"))
			followup = m.run(btJobStatus, m.api.Status)
		}
	case btJobScan:
		followup = m.comboApplyScan(report)
	case btJobAdd:
		if failed {
			m.setToast(true, "添加依赖失败：%s", localMessage(message))
		} else {
			m.setToast(false, "依赖已添加：%s", m.deps.addInput.Value())
			m.deps.clearInput()
			followup = tea.Batch(m.run(btJobStatus, m.api.Status), m.analyzeCmd())
		}
	case btJobCatalog:
		m.catalog.applyReport(report, err)
		if failed {
			m.setToast(true, "查询仓库失败：%s", localMessage(message))
		}
	case btJobInit:
		if failed {
			m.setToast(true, "初始化失败：%s", localMessage(message))
		} else {
			m.setToast(false, "%s", fallback(report.Message, "项目已初始化"))
			followup = m.refreshAll()
		}
	case btJobRecipe:
		if failed {
			m.setToast(true, "生成配方失败：%s", localMessage(message))
		} else {
			m.setToast(false, "%s", fallback(report.Message, "配方已生成"))
			followup = m.run(btJobStatus, m.api.Status)
		}
	case btJobSave:
		if failed {
			m.setToast(true, "保存设置失败：%s", localMessage(message))
			m.clearPendingLogin()
			followup = m.run(btJobStatus, m.api.Status)
		} else if m.pendingLogin {
			password := m.pendingLoginPassword
			m.clearPendingLogin()
			followup = m.run(btJobLogin, func(ctx context.Context) (workflow.Report, error) {
				return m.api.ConfigLogin(ctx, password)
			})
		} else {
			followup = m.run(btJobStatus, m.api.Status)
		}
	case btJobLogin:
		if failed {
			m.setToast(true, "登录失败：%s", localMessage(message))
		} else {
			m.settings.password.SetValue("")
			m.setToast(false, "%s", fallback(report.Message, "已登录仓库"))
			m.probeAt = time.Time{}
			followup = tea.Batch(m.run(btJobStatus, m.api.Status), m.probeCmd())
		}
	case btJobProbe:
		m.probeOK = !failed && report.OK
		if failed && m.active == tabSettings {
			m.setToast(true, "测试连接失败：%s", localMessage(message))
		}
	}
	return m, followup
}

func (m *btModel) clearPendingLogin() {
	m.pendingLogin = false
	m.pendingLoginPassword = ""
}

func btParseStatus(report workflow.Report) *btStatus {
	data := reportMap(report)
	status := &btStatus{}
	if initialized, ok := data["initialized"].(bool); ok {
		status.initialized = initialized
	}
	if project, ok := data["project"].(*config.Project); ok {
		status.project = project
	}
	if global, ok := data["global"].(config.GlobalView); ok {
		status.global = global
	}
	if packages, ok := data["packages"].([]workflow.PackageInfo); ok {
		status.packages = packages
	}
	status.conanfile = stringValue(data["conanfile"])
	return status
}

func btParseCatalog(report workflow.Report) []workflow.CatalogPackage {
	data := reportMap(report)
	if packages, ok := data["packages"].([]workflow.CatalogPackage); ok {
		return packages
	}
	return nil
}

// ---------------------------------------------------------------------------
// 入口
// ---------------------------------------------------------------------------

func runBubble(ctx context.Context, api tuiAPI, in io.Reader, out io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	model := newBTModel(api)
	model.ctx = ctx
	program := tea.NewProgram(model,
		tea.WithAltScreen(),
		tea.WithContext(ctx),
		tea.WithMouseCellMotion(),
		tea.WithInput(in),
		tea.WithOutput(out),
	)
	_, err := program.Run()
	return err
}
