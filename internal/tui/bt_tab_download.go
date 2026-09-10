package tui

import (
	"context"

	"conan-cli/internal/config"
	"conan-cli/internal/workflow"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *btModel) downloadKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "e", "E", "enter":
		return m, m.openCombo("consume")
	case "a", "A":
		return m, m.analyzeCmd()
	case "g", "G":
		return m, m.run(btJobRecipe, func(ctx context.Context) (workflow.Report, error) {
			return m.api.GenerateRecipe("consume", false, "", "", "")
		})
	case "i", "I":
		spec := m.status.proj().Platform.Consume
		if missingPlatform(spec) {
			m.setToast(true, "请先选择目标平台（操作系统 + 架构）。")
			return m, m.openCombo("consume")
		}
		if m.status != nil && !m.status.global.HasPassword {
			m.setToast(true, "还没有保存仓库密码：请先到“设置”登录仓库。")
			m.switchTab(tabSettings)
			return m, nil
		}
		return m, m.installCmd()
	case "n", "N":
		if m.status == nil || !m.status.initialized {
			return m, m.run(btJobInit, m.api.Init)
		}
	}
	return m, nil
}

func (m *btModel) downloadView(layout *btLines) {
	if m.status == nil || !m.status.initialized {
		layout.add(btStyleWarn.Render("  项目尚未初始化。按 n 初始化（生成 .conan-cli/project.yaml 与 conanfile）。"))
		layout.add("")
	}

	if !m.combo.open {
		layout.card("目标组合 · 与发布默认同步", func(l *btLines) {
			l.addBlock(m.comboBarView("consume"))
			l.add(btStyleHint.Render("  e 展开修改"))
		})
	}

	layout.add("")
	if m.analyzeLoaded {
		missing := m.missingDependencies()
		legend := "依赖分析"
		if missing != nil {
			legend += " · " + itoa(len(missing)) + " 个缺制品"
		}
		layout.card(legend, func(l *btLines) {
			l.addBlock(btDependencyTable(m.analyzeRows, l.width))
		})
		if len(missing) > 0 {
			layout.add("")
			missStart := len(layout.lines)
			layout.addBlock(btMissCard(missing, m.status.proj().Platform.Consume))
			layout.regionWide(len(layout.lines)-missStart, "miss")
			layout.add(btStyleHint.Render("  按 e 或点上方卡片改组合"))
		}
	} else {
		layout.add(btStyleMuted.Render("  按 a 检查依赖，查看每个包是否有匹配二进制。"))
	}

	if intent := m.installIntentView(); intent != "" {
		layout.add("")
		layout.add(intent)
	}
}

func (m *btModel) installIntentView() string {
	project := m.status.proj()
	spec := project.Platform.Consume
	if missingPlatform(spec) {
		return ""
	}
	output := fallback(project.OutputFolder, config.DefaultOutputFolder)
	login := "请先到设置里登录仓库"
	if m.status != nil && m.status.global.HasPassword {
		login = "已登录，按 i 拉取"
	}
	return btStyleHint.Render("  将按 " + btComboTitle(project, spec) + " 拉取到 " + output +
		"/（只取仓库二进制，不在本机编译）· " + login)
}
