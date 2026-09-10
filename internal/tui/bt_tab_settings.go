package tui

import (
	"context"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"conan-cli/internal/workflow"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const btSettingsPasswordRow = 7

// 设置 Tab：项目身份 + 仓库登录，对齐 VS Code 的设置页。
type btSettingsModel struct {
	row        int // 0 包名 1 产物目录 2 lib 产物目录 3 头文件目录 4 组件目录 5 仓库地址 6 用户名 7 密码
	editing    bool
	name       textinput.Model
	output     textinput.Model
	libDirs    textinput.Model
	includeDir textinput.Model
	workspaces textinput.Model
	repoURL    textinput.Model
	username   textinput.Model
	password   textinput.Model
}

func newBTSettingsModel() btSettingsModel {
	newInput := func(placeholder string) textinput.Model {
		input := textinput.New()
		input.Placeholder = placeholder
		input.CharLimit = 160
		return input
	}
	model := btSettingsModel{
		name:       newInput("默认取目录名"),
		output:     newInput("默认 conan"),
		libDirs:    newInput("逗号分隔，如 lib,bin"),
		includeDir: newInput("逗号分隔，如 include,src"),
		workspaces: newInput("逗号分隔 glob，如 packages/*,src/*"),
		repoURL:    newInput("如 https://nexus.example.com/repository/conan"),
		username:   newInput("仓库用户名"),
		password:   newInput("已保存则留空不改"),
	}
	model.password.EchoMode = textinput.EchoPassword
	model.password.EchoCharacter = '*'
	return model
}

func (s *btSettingsModel) inputs() []*textinput.Model {
	return []*textinput.Model{
		&s.name, &s.output, &s.libDirs, &s.includeDir,
		&s.workspaces, &s.repoURL, &s.username, &s.password,
	}
}

// labelFor 在共享字段表里按别名查展示标签；查不到时用 fallback。
// 设置页的 8 个行标签全部取自 settings_fields.go 的同一张表，
// 行模式与 BubbleTea 轨共享字段清单，新增字段不再两处分叉。
func (s *btSettingsModel) labelFor(alias, fallback string) string {
	for _, group := range [][]settingsField{projectSettingsFields, globalSettingsFields} {
		for _, field := range group {
			if len(field.aliases) > 0 && field.aliases[0] == alias {
				return field.label
			}
		}
	}
	return fallback
}

func (s *btSettingsModel) labels() []string {
	return []string{
		s.labelFor("name", "Conan 包名"),
		s.labelFor("output", "产物目录"),
		"lib 产物目录",
		s.labelFor("include-dir", "头文件目录"),
		"组件目录",
		s.labelFor("url", "仓库地址"),
		s.labelFor("user", "用户名"),
		s.labelFor("password", "密码"),
	}
}

// fieldText 是非编辑态的展示值。密码永远走掩码，避免离开输入框后 Value() 明文上屏。
func (s *btSettingsModel) fieldText(index int) string {
	if index == btSettingsPasswordRow {
		if value := s.password.Value(); value != "" {
			return strings.Repeat("*", utf8.RuneCountInString(value))
		}
		return s.password.Placeholder
	}
	input := s.inputs()[index]
	return fallback(strings.TrimSpace(input.Value()), input.Placeholder)
}

func (s *btSettingsModel) blur() {
	s.editing = false
	for _, input := range s.inputs() {
		input.Blur()
	}
}

func (s *btSettingsModel) inputFocused() bool { return s.editing }

func (s *btSettingsModel) resize(width int) {
	for _, input := range s.inputs() {
		input.Width = max(12, width-26)
	}
}

// syncForm 在状态刷新后把当前值回填为占位提示；用户输入过的不动。
// 占位语取自共享字段表（新增字段只需改 settings_fields.go 一处）。
func (s *btSettingsModel) syncForm(status *btStatus) {
	project := status.proj()
	global := status.global
	values := []string{
		project.Name,
		project.OutputFolder,
		strings.Join(project.PrimaryPackage().LibDirs, ","),
		strings.Join(project.PrimaryPackage().IncludeDirs, ","),
		strings.Join(project.Workspaces, ","),
		global.Nexus.URL,
		global.Nexus.Username,
		"",
	}
	for index, input := range s.inputs() {
		if !input.Focused() && input.Value() == "" {
			placeholder := strings.TrimSpace(values[index])
			if index == 7 {
				placeholder = "已保存则留空不改"
			}
			input.Placeholder = placeholder
		}
	}
}

func (m *btModel) settingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := &m.settings
	if s.editing {
		switch msg.String() {
		case "esc":
			s.blur()
			return m, nil
		case "up":
			s.row = (s.row + 7) % 8
			return m, m.settingsFocusRow()
		case "down", "enter", "tab":
			s.row = (s.row + 1) % 8
			return m, m.settingsFocusRow()
		}
		var cmd tea.Cmd
		target := s.inputs()[s.row]
		*target, cmd = target.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "up":
		s.row = (s.row + 7) % 8
		return m, nil
	case "down":
		s.row = (s.row + 1) % 8
		return m, nil
	case "enter":
		s.editing = true
		return m, m.settingsFocusRow()
	case "s", "S":
		return m, m.saveProjectSettings()
	case "l", "L":
		return m, m.saveAndLogin()
	case "t", "T":
		m.probeAt = time.Time{}
		return m, m.probeCmd()
	}
	return m, nil
}

func (m *btModel) settingsFocusRow() tea.Cmd {
	s := &m.settings
	for _, input := range s.inputs() {
		input.Blur()
	}
	return s.inputs()[s.row].Focus()
}

func (m *btModel) saveProjectSettings() tea.Cmd {
	s := &m.settings
	input := workflow.ProjectSettingsInput{
		Name:         strings.TrimSpace(s.name.Value()),
		OutputFolder: strings.TrimSpace(s.output.Value()),
	}
	if value := strings.TrimSpace(s.libDirs.Value()); value != "" {
		input.LibDirs = btSplitList(value)
		input.HasLibDirs = true
	}
	if value := strings.TrimSpace(s.includeDir.Value()); value != "" {
		input.IncludeDirs = btSplitList(value)
		input.HasIncludeDirs = true
	}
	if value := strings.TrimSpace(s.workspaces.Value()); value != "" {
		input.Workspaces = btSplitList(value)
		input.HasWorkspaces = true
	}
	m.setToast(false, "正在保存项目设置…")
	return m.run(btJobSave, func(ctx context.Context) (workflow.Report, error) {
		return m.api.SaveProjectSettings(input)
	})
}

func (m *btModel) saveAndLogin() tea.Cmd {
	s := &m.settings
	global := m.statusOrEmpty().global
	input := workflow.GlobalSettingsInput{
		Name:     fallback(global.Nexus.Name, "nexus"),
		URL:      strings.TrimSpace(s.repoURL.Value()),
		Username: strings.TrimSpace(s.username.Value()),
		Password: s.password.Value(),
	}
	if strings.TrimSpace(s.repoURL.Value()) != "" {
		if name := btRemoteName(s.repoURL.Value(), global.Nexus.Name); name != "" {
			input.Name = name
		}
	}
	m.setToast(false, "正在保存并登录…")
	m.pendingLogin = true
	m.pendingLoginPassword = input.Password
	return m.run(btJobSave, func(ctx context.Context) (workflow.Report, error) {
		return m.api.SaveGlobalSettings(ctx, input)
	})
}

// btRemoteName 从 URL 推导 remote 名；已有配置则优先沿用。
func btRemoteName(rawURL, existing string) string {
	if existing != "" {
		return existing
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "nexus"
	}
	host := parsed.Hostname()
	if host == "" {
		return "nexus"
	}
	labels := strings.Split(host, ".")
	if len(labels) > 0 && labels[0] != "" && labels[0] != "www" {
		return labels[0]
	}
	return "nexus"
}

func (m *btModel) settingsView(layout *btLines) {
	s := &m.settings
	layout.card("项目身份 · .conan-cli/project.yaml，可提交 git", func(l *btLines) {
		for index, label := range s.labels()[:5] {
			selected := s.row == index
			var line string
			if s.editing && selected {
				line = "  " + padCells(label, 14) + s.inputs()[index].View()
			} else {
				line = "  " + padCells(label, 14) + btStyleMono.Render(s.fieldText(index))
			}
			l.addRow("setrow:"+itoa(index), selected, line)
			if index == 0 {
				l.add(btStyleHint.Render("    改包名会变成仓库里的新包，旧引用全部失效。"))
			}
		}
		l.add(btStyleHint.Render("    产物目录 / lib / 头文件 / 组件目录留空表示保持现有值，不会清空。"))
	})
	layout.add("")
	layout.card("仓库登录 · 保存在本机 ~/.conan-cli/，不进项目", func(l *btLines) {
		for index := 5; index < 8; index++ {
			label := s.labels()[index]
			selected := s.row == index
			var line string
			if s.editing && selected {
				line = "  " + padCells(label, 14) + s.inputs()[index].View()
			} else {
				line = "  " + padCells(label, 14) + btStyleMono.Render(s.fieldText(index))
			}
			l.addRow("setrow:"+itoa(index), selected, line)
			if index == btSettingsPasswordRow {
				l.add(btStyleHint.Render("    密码不回显；留空表示保持已保存的密码。"))
			}
		}
	})

	status := []string{}
	global := m.statusOrEmpty().global
	if global.Nexus.URL != "" {
		status = append(status, "地址已保存")
	}
	if global.Nexus.Username != "" {
		status = append(status, "用户 "+global.Nexus.Username)
	}
	if global.HasPassword {
		status = append(status, "密码已保存")
	}
	if m.probeOK {
		status = append(status, "仓库可达")
	}
	summary := strings.Join(status, " · ")
	if summary == "" {
		summary = "尚未配置仓库"
	}
	layout.add("")
	layout.card("仓库状态", func(l *btLines) {
		l.add("  " + btStyleOK.Render("●") + " " + btStyleMono.Render(summary))
	})
}
