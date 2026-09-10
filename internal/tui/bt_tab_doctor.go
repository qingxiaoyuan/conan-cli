package tui

import (
	"strings"

	"conan-cli/internal/workflow"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type btDoctorModel struct {
	viewport viewport.Model
	checks   []workflow.Check
	raw      bool
}

func newBTDoctorModel() btDoctorModel {
	return btDoctorModel{viewport: viewport.New(76, 10)}
}

func (d *btDoctorModel) setChecks(checks []workflow.Check) {
	d.checks = checks
}

func (d *btDoctorModel) resize(width, height int) {
	d.viewport.Width = max(40, width-2)
	d.viewport.Height = max(4, height-8)
}

func (m *btModel) doctorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "o", "O":
		m.doctor.raw = !m.doctor.raw
		if m.doctor.raw {
			m.doctor.viewport.SetContent(strings.Join(m.logLines, "\n"))
			m.doctor.viewport.GotoBottom()
		}
		return m, nil
	case "up":
		m.doctor.viewport.LineUp(1)
		return m, nil
	case "down":
		m.doctor.viewport.LineDown(1)
		return m, nil
	}
	return m, nil
}

func (m *btModel) doctorView(layout *btLines) {
	d := &m.doctor
	layout.card("环境检查", func(l *btLines) {
		l.add("  " + btDoctorSummary(d.checks))
		l.addBlock(btDoctorCards(d.checks, l.width))
	})
	if d.raw {
		layout.add("")
		layout.card("高级 / 原始输出 · ↑↓ 滚动 · o 收起", func(l *btLines) {
			l.addBlock(d.viewport.View())
		})
	} else {
		layout.add("")
		layout.add(btStyleHint.Render("  o 查看高级 / 原始输出"))
	}
}
