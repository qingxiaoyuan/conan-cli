package tui

import (
	"context"
	"strings"

	"conan-cli/internal/workflow"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type btDepsModel struct {
	addInput textinput.Model
	adding   bool
}

func newBTDepsModel() btDepsModel {
	input := textinput.New()
	input.Placeholder = "如 qtutils/1.0"
	input.CharLimit = 120
	return btDepsModel{addInput: input}
}

func (d *btDepsModel) blur() {
	d.adding = false
	d.addInput.Blur()
}

func (d *btDepsModel) inputFocused() bool { return d.adding }

func (d *btDepsModel) clearInput() {
	d.addInput.SetValue("")
}

func (m *btModel) depsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.deps.adding {
		switch msg.String() {
		case "esc":
			m.deps.blur()
			return m, nil
		case "enter":
			value := strings.TrimSpace(m.deps.addInput.Value())
			if value == "" {
				m.deps.blur()
				return m, nil
			}
			return m, m.run(btJobAdd, func(ctx context.Context) (workflow.Report, error) {
				return m.api.Add(value)
			})
		}
		var cmd tea.Cmd
		m.deps.addInput, cmd = m.deps.addInput.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "a", "A":
		m.deps.adding = true
		return m, m.deps.addInput.Focus()
	}
	return m, nil
}

func (m *btModel) depsView(layout *btLines) {
	layout.card("直接依赖 · conanfile / project.yaml 同步维护", func(l *btLines) {
		if m.analyzeLoaded {
			l.addBlock(btDependencyTable(m.analyzeRows, l.width))
			if missing := m.missingDependencies(); len(missing) > 0 {
				missStart := len(l.lines)
				l.addBlock(btMissCard(missing, m.status.proj().Platform.Consume))
				l.regionWide(len(l.lines)-missStart, "miss")
			}
		} else {
			l.add(btStyleMuted.Render("  按 r 刷新后查看每个包的制品状态。"))
		}
	})

	layout.add("")
	layout.card("添加依赖", func(l *btLines) {
		if m.deps.adding {
			l.add("  " + m.deps.addInput.View() + "  " + btStyleHint.Render("Enter 确认 · Esc 取消"))
		} else {
			l.regionRow("depsadd")
			l.add(btStyleHint.Render("  按 a 或点此行输入包引用（如 qtutils/1.0）加入依赖"))
		}
	})
}
