package tui

import (
	"strconv"

	"conan-cli/internal/config"
)

// settingsFieldKind decides how a field is edited (text input vs. choice
// page vs. secret) and how its raw value is normalized and displayed.
type settingsFieldKind uint8

const (
	fieldText settingsFieldKind = iota
	fieldSecret
	fieldOS
	fieldArch
	fieldBuildType
)

// settingsScope is the editable state of the settings page, shared by the
// line-oriented screen and the cursor screen.
type settingsScope struct {
	global          *config.Global
	project         *config.Project
	pendingPassword string
}

// settingsField describes one editable settings entry. Both input modes
// render and edit this single table, so adding a field can no longer diverge
// between the line mode and the cursor mode.
type settingsField struct {
	aliases     []string // 行模式下可输入的别名（编号始终有效）
	label       string   // 列表中的展示标签
	editHint    string   // 编辑输入框的提示语
	placeholder string   // 值为空时显示的占位文本
	note        string   // 行尾附加说明（可空）
	kind        settingsFieldKind
	get         func(*settingsScope) string
	set         func(*settingsScope, string)
	display     func(*settingsScope) string // 可选：覆盖默认展示（如密码）
}

// globalSettingsFields mirrors the five global config fields.
var globalSettingsFields = []settingsField{
	{
		aliases: []string{"name"}, label: "仓库名", editHint: "仓库名", placeholder: "nexus",
		get: func(s *settingsScope) string { return s.global.Nexus.Name },
		set: func(s *settingsScope, value string) { s.global.Nexus.Name = value },
	},
	{
		aliases: []string{"url"}, label: "URL", editHint: "Nexus Conan URL", placeholder: "未配置",
		get: func(s *settingsScope) string { return s.global.Nexus.URL },
		set: func(s *settingsScope, value string) { s.global.Nexus.URL = value },
	},
	{
		aliases: []string{"user"}, label: "用户", editHint: "登录用户名", placeholder: "未配置",
		get: func(s *settingsScope) string { return s.global.Nexus.Username },
		set: func(s *settingsScope, value string) { s.global.Nexus.Username = value },
	},
	{
		aliases: []string{"password"}, label: "密码 / Token", kind: fieldSecret,
		editHint: "密码 / Token（留空保留已保存密码）",
		get:      func(s *settingsScope) string { return "" },
		set: func(s *settingsScope, value string) {
			if value != "" {
				s.pendingPassword = value
			}
		},
		display: func(s *settingsScope) string {
			if s.global.View().HasPassword || s.pendingPassword != "" {
				return "********（本机保存）"
			}
			return "未保存"
		},
	},
	{
		aliases: []string{"conan-bin"}, label: "Conan 路径", editHint: "Conan 可执行文件路径",
		placeholder: "插件内置或 PATH 中的 conan",
		get:         func(s *settingsScope) string { return s.global.ConanBin },
		set:         func(s *settingsScope, value string) { s.global.ConanBin = value },
	},
}

// projectSettingsFields mirrors the project.yaml fields editable in the UI.
var projectSettingsFields = []settingsField{
	{
		aliases: []string{"name"}, label: "Conan 包名", placeholder: "-",
		editHint: "Conan 包名（改名会变成仓库里的新包）",
		note:     "（改名会变成新包）",
		get:      func(s *settingsScope) string { return s.project.Name },
		set:      func(s *settingsScope, value string) { s.project.Name = value },
	},
	{
		aliases: []string{"qt"}, label: "Qt 版本", editHint: "Qt 版本", placeholder: "未设置",
		get: func(s *settingsScope) string { return s.project.QtVersion },
		set: func(s *settingsScope, value string) { s.project.QtVersion = value },
	},
	{
		aliases: []string{"compiler"}, label: "编译器", editHint: "编译器 gcc / clang / msvc", placeholder: "未设置",
		get: func(s *settingsScope) string { return s.project.Compiler.ID },
		set: func(s *settingsScope, value string) { s.project.Compiler.ID = value },
	},
	{
		aliases: []string{"compiler-version"}, label: "编译器版本", editHint: "编译器版本", placeholder: "未设置",
		get: func(s *settingsScope) string { return s.project.Compiler.Version },
		set: func(s *settingsScope, value string) { s.project.Compiler.Version = value },
	},
	{
		aliases: []string{"os"}, label: "使用系统", editHint: "使用平台操作系统", kind: fieldOS, placeholder: "未选择",
		get: func(s *settingsScope) string { return s.project.Platform.Consume.OS },
		set: func(s *settingsScope, value string) { s.project.Platform.Consume.OS = value },
	},
	{
		aliases: []string{"arch"}, label: "使用架构", editHint: "使用平台架构", kind: fieldArch, placeholder: "未选择",
		get: func(s *settingsScope) string { return s.project.Platform.Consume.Arch },
		set: func(s *settingsScope, value string) { s.project.Platform.Consume.Arch = value },
	},
	{
		aliases: []string{"build-type"}, label: "使用构建", editHint: "使用构建类型 Debug / Release", kind: fieldBuildType, placeholder: "Release",
		get: func(s *settingsScope) string { return s.project.Platform.Consume.BuildType },
		set: func(s *settingsScope, value string) { s.project.Platform.Consume.BuildType = value },
	},
	{
		aliases: []string{"publish-os"}, label: "发布系统", editHint: "发布平台操作系统", kind: fieldOS, placeholder: "未选择",
		get: func(s *settingsScope) string { return s.project.Platform.Publish.OS },
		set: func(s *settingsScope, value string) { s.project.Platform.Publish.OS = value },
	},
	{
		aliases: []string{"publish-arch"}, label: "发布架构", editHint: "发布平台架构", kind: fieldArch, placeholder: "未选择",
		get: func(s *settingsScope) string { return s.project.Platform.Publish.Arch },
		set: func(s *settingsScope, value string) { s.project.Platform.Publish.Arch = value },
	},
	{
		aliases: []string{"publish-build-type"}, label: "发布构建", editHint: "发布构建类型 Debug / Release", kind: fieldBuildType, placeholder: "Release",
		get: func(s *settingsScope) string { return s.project.Platform.Publish.BuildType },
		set: func(s *settingsScope, value string) { s.project.Platform.Publish.BuildType = value },
	},
	{
		aliases: []string{"channel"}, label: "channel", editHint: "channel", placeholder: "dev",
		get: func(s *settingsScope) string { return s.project.Channel },
		set: func(s *settingsScope, value string) { s.project.Channel = value },
	},
	{
		aliases: []string{"remote"}, label: "远程仓库", editHint: "项目远程仓库名", placeholder: "跟随全局",
		get: func(s *settingsScope) string { return s.project.Remote },
		set: func(s *settingsScope, value string) { s.project.Remote = value },
	},
	{
		aliases: []string{"build"}, label: "构建系统", editHint: "构建系统 cmake / qmake / unknown", placeholder: "unknown",
		get: func(s *settingsScope) string { return s.project.BuildSystem },
		set: func(s *settingsScope, value string) { s.project.BuildSystem = value },
	},
	{
		aliases: []string{"output"}, label: "输出目录", editHint: "输出目录", placeholder: "conan",
		get: func(s *settingsScope) string { return s.project.OutputFolder },
		set: func(s *settingsScope, value string) { s.project.OutputFolder = value },
	},
	{
		aliases: []string{"lib-dir"}, label: "产物目录", editHint: "预编译库目录（逗号分隔，相对项目根）", placeholder: "lib/, bin/",
		get: func(s *settingsScope) string { return config.JoinPathList(s.project.PrimaryPackage().LibDirs) },
		set: func(s *settingsScope, value string) {
			_ = s.project.SetPrimaryArtifactDirs(config.SplitPathList(value), s.project.PrimaryPackage().IncludeDirs)
		},
	},
	{
		aliases: []string{"include-dir"}, label: "头文件目录", editHint: "头文件目录（逗号分隔，相对项目根）", placeholder: "include/",
		get: func(s *settingsScope) string { return config.JoinPathList(s.project.PrimaryPackage().IncludeDirs) },
		set: func(s *settingsScope, value string) {
			_ = s.project.SetPrimaryArtifactDirs(s.project.PrimaryPackage().LibDirs, config.SplitPathList(value))
		},
	},
}

func settingsFieldsFor(active string) []settingsField {
	if active == "global" {
		return globalSettingsFields
	}
	return projectSettingsFields
}

// matchSettingsField resolves a line-mode menu choice ("1", "name", "qt", …)
// to a field index in the table.
func matchSettingsField(choice string, fields []settingsField) (int, bool) {
	for index, field := range fields {
		if choice == strconv.Itoa(index+1) {
			return index, true
		}
		for _, alias := range field.aliases {
			if choice == alias {
				return index, true
			}
		}
	}
	return -1, false
}

// apply normalizes the input according to the field kind and stores it.
func (f settingsField) apply(scope *settingsScope, value string) {
	switch f.kind {
	case fieldOS:
		value = config.NormalizeOS(value)
	case fieldArch:
		value = config.NormalizeArch(value)
	case fieldBuildType:
		value = config.NormalizeBuildType(value)
	}
	f.set(scope, value)
}

// displayValue renders the stored value for listing; empty values fall back
// to the field placeholder.
func (f settingsField) displayValue(scope *settingsScope) string {
	if f.display != nil {
		return f.display(scope)
	}
	value := f.get(scope)
	switch f.kind {
	case fieldOS:
		value = config.DisplayOS(value)
	case fieldArch:
		value = config.DisplayArch(value)
	case fieldBuildType:
		value = config.DisplayBuildType(value)
	}
	if value == "" {
		return f.placeholder
	}
	return value
}

func osChoiceOptions() []choiceOption {
	return []choiceOption{{config.OSWindows, "Windows"}, {config.OSLinux, "Linux"}, {config.OSKylin, "麒麟"}}
}

func archChoiceOptions() []choiceOption {
	return []choiceOption{{config.ArchX86, "x86 32 位"}, {config.ArchX64, "x64 64 位"}, {config.ArchARM, "ARM 32 位"}, {config.ArchARM64, "ARM 64 位"}}
}

func buildTypeChoiceOptions() []choiceOption {
	return []choiceOption{{config.BuildTypeRelease, "Release"}, {config.BuildTypeDebug, "Debug"}}
}
