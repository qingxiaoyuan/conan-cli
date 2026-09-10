package tui

import (
	"context"
	"time"

	"conan-cli/internal/workflow"
	"github.com/charmbracelet/bubbletea"
)

// 超时分级与 VS Code 插件保持一致：常规命令 120s，拉取/发布 30 分钟。
const (
	btTimeoutNormal = 120 * time.Second
	btTimeoutLong   = 30 * time.Minute
)

type btJob string

const (
	btJobStatus  btJob = "状态"
	btJobDoctor  btJob = "诊断"
	btJobAnalyze btJob = "依赖分析"
	btJobInstall btJob = "拉取依赖"
	btJobPublish btJob = "发布"
	btJobRecipe  btJob = "生成配方"
	btJobScan    btJob = "扫描"
	btJobInit    btJob = "初始化"
	btJobAdd     btJob = "添加依赖"
	btJobCatalog btJob = "查询仓库"
	btJobSave    btJob = "保存设置"
	btJobLogin   btJob = "登录仓库"
	btJobProbe   btJob = "测试连接"
)

func btJobLong(job btJob) bool {
	return job == btJobInstall || job == btJobPublish
}

type btReportMsg struct {
	job    btJob
	report workflow.Report
	err    error
}

// btKey 把按键名转成 tea.KeyMsg，鼠标点击复用同一套按键处理逻辑。
func btKey(value string) tea.KeyMsg {
	switch value {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func btRun(parent context.Context, job btJob, run func(ctx context.Context) (workflow.Report, error)) tea.Cmd {
	timeout := btTimeoutNormal
	if btJobLong(job) {
		timeout = btTimeoutLong
	}
	if parent == nil {
		parent = context.Background()
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, timeout)
		defer cancel()
		report, err := run(ctx)
		return btReportMsg{job: job, report: report, err: err}
	}
}
