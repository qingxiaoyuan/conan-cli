package tui

import (
	"context"
	"strings"

	"conan-cli/internal/workflow"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type btPublishModel struct {
	cursor  int // 组件行
	checked map[string]bool
	formRow int // 0 包名 1 版本 2 channel 3 备注 4 替换旧版本
	editing bool
	name    textinput.Model
	version textinput.Model
	channel textinput.Model
	note    textinput.Model
	replace bool
	results map[string]string // 包名 → 最近一次发布结果
	// queue 服务两个阶段：dry-run 预览阶段 publishing=false，逐个探测；
	// 确认后 queue 重填为真实请求并置 publishing=true 开始上传。
	queue       []workflow.PublishRequest
	realQueue   []workflow.PublishRequest // 预览阶段的真实请求暂存
	publishing  bool                      // queue 当前跑的是正式发布还是预览
	previewDone int                       // 预览阶段已成功的个数
}

func newBTPublishModel() btPublishModel {
	newInput := func(placeholder string) textinput.Model {
		input := textinput.New()
		input.Placeholder = placeholder
		input.CharLimit = 64
		return input
	}
	return btPublishModel{
		checked: map[string]bool{},
		results: map[string]string{},
		name:    newInput("默认取组件名"),
		version: newInput("如 1.0.0"),
		channel: newInput("默认用项目 channel"),
		note:    newInput("发布备注，可留空"),
	}
}

func (p *btPublishModel) blur() {
	p.editing = false
	p.name.Blur()
	p.version.Blur()
	p.channel.Blur()
	p.note.Blur()
}

func (p *btPublishModel) inputFocused() bool { return p.editing }

func (p *btPublishModel) resize(width int) {
	inputs := []*textinput.Model{&p.name, &p.version, &p.channel, &p.note}
	for _, input := range inputs {
		input.Width = max(12, width/2-20)
	}
}

func (p *btPublishModel) syncComponents(status *btStatus) {
	if p.results == nil {
		p.results = map[string]string{}
	}
	if status == nil {
		return
	}
	valid := map[string]bool{}
	for _, pkg := range status.packages {
		valid[pkg.Name] = true
	}
	for name := range p.checked {
		if !valid[name] {
			delete(p.checked, name)
		}
	}
}

func (p *btPublishModel) checkedNames(packages []workflow.PackageInfo) []string {
	var names []string
	for _, pkg := range packages {
		if p.checked[pkg.Name] {
			names = append(names, pkg.Name)
		}
	}
	return names
}

func (p *btPublishModel) packageByName(packages []workflow.PackageInfo, name string) *workflow.PackageInfo {
	for index := range packages {
		if packages[index].Name == name {
			return &packages[index]
		}
	}
	return nil
}

func (p *btPublishModel) inputs() []*textinput.Model {
	return []*textinput.Model{&p.name, &p.version, &p.channel, &p.note}
}

func (m *btModel) publishKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := &m.publish
	if p.editing {
		switch msg.String() {
		case "esc":
			p.blur()
			return m, nil
		case "up":
			p.formRow = (p.formRow + 4) % 5
			return m, m.publishFocusRow()
		case "down", "tab":
			p.formRow = (p.formRow + 1) % 5
			return m, m.publishFocusRow()
		case "enter", " ":
			if p.formRow == 4 {
				p.replace = !p.replace
				return m, nil
			}
			p.formRow = (p.formRow + 1) % 5
			return m, m.publishFocusRow()
		}
		if p.formRow == 4 {
			return m, nil
		}
		var cmd tea.Cmd
		target := p.inputTarget(p.formRow)
		*target, cmd = target.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "up":
		if p.cursor > 0 {
			p.cursor--
		}
		return m, nil
	case "down":
		if p.cursor+1 < len(m.packages()) {
			p.cursor++
		}
		return m, nil
	case " ":
		if pkg := m.selectedPackage(); pkg != nil {
			if p.checked[pkg.Name] {
				delete(p.checked, pkg.Name)
			} else {
				p.checked[pkg.Name] = true
			}
		}
		return m, nil
	case "enter":
		p.formRow = 0
		p.editing = true
		return m, m.publishFocusRow()
	case "e", "E":
		return m, m.openCombo("publish")
	case "p", "P":
		return m.publishConfirm(false)
	case "A":
		return m.publishConfirm(true)
	}
	return m, nil
}

func (m *btModel) publishFocusRow() tea.Cmd {
	p := &m.publish
	for _, input := range p.inputs() {
		input.Blur()
	}
	if p.formRow < 4 {
		return p.inputTarget(p.formRow).Focus()
	}
	return nil
}

// inputTarget 返回第 row 个表单字段的指针（textinput.Model 值拷贝陷阱）。
func (p *btPublishModel) inputTarget(row int) *textinput.Model {
	switch row {
	case 0:
		return &p.name
	case 1:
		return &p.version
	case 2:
		return &p.channel
	default:
		return &p.note
	}
}

func (m *btModel) selectedPackage() *workflow.PackageInfo {
	packages := m.packages()
	if m.publish.cursor < 0 || m.publish.cursor >= len(packages) {
		return nil
	}
	return &packages[m.publish.cursor]
}

func (m *btModel) publishRequest(pkg *workflow.PackageInfo, all bool) workflow.PublishRequest {
	project := m.status.proj()
	spec := project.Platform.Publish
	if spec.Empty() {
		spec = project.Platform.Consume
	}
	request := workflow.PublishRequest{
		OS:              spec.OS,
		Arch:            spec.Arch,
		BuildType:       spec.BuildType,
		Compiler:        project.Compiler.ID,
		CompilerVersion: project.Compiler.Version,
		QtVersion:       project.QtVersion,
		Channel:         fallback(strings.TrimSpace(m.publish.channel.Value()), project.Channel),
		Note:            strings.TrimSpace(m.publish.note.Value()),
		Replace:         m.publish.replace,
		All:             all,
	}
	if !all && pkg != nil {
		request.Package = pkg.Name
		if value := strings.TrimSpace(m.publish.name.Value()); value != "" {
			request.Name = value
		}
		if value := strings.TrimSpace(m.publish.version.Value()); value != "" {
			request.Version = value
		}
		if pkg.NoQt {
			request.NoQt = true
			request.QtVersion = ""
		}
	}
	return request
}

// publishConfirm 进入发布前置流程：先跑 dry-run 生成预览（对齐行模式与
// CLI 的 publish --dry-run 语义），预览回来后再弹确认框，y 才真正上传。
func (m *btModel) publishConfirm(all bool) (tea.Model, tea.Cmd) {
	packages := m.packages()
	pkg := m.selectedPackage()
	checked := m.publish.checkedNames(packages)

	var reqs []workflow.PublishRequest
	if all {
		if len(packages) == 0 {
			m.setToast(true, "没有可发布的组件：先初始化项目或在组件目录放好 conanfile/dist/lib。")
			return m, nil
		}
		if len(checked) > 0 {
			for _, name := range checked {
				reqs = append(reqs, m.publishRequest(m.publish.packageByName(packages, name), false))
			}
		} else {
			reqs = []workflow.PublishRequest{m.publishRequest(nil, true)}
		}
	} else {
		if pkg == nil {
			m.setToast(true, "没有可发布的组件：先初始化项目或在组件目录放好 conanfile/dist/lib。")
			return m, nil
		}
		reqs = []workflow.PublishRequest{m.publishRequest(pkg, false)}
	}

	project := m.status.proj()
	spec := project.Platform.Publish
	if spec.Empty() {
		spec = project.Platform.Consume
	}
	if missingPlatform(spec) {
		m.setToast(true, "发布前先选择目标平台（按 e 打开发布组合）。")
		return m, m.openCombo("publish")
	}

	// dry-run 请求逐个排队探测：配方身份、组件清单、制品目录是否齐备。
	previews := make([]workflow.PublishRequest, len(reqs))
	for index, req := range reqs {
		req.DryRun = true
		previews[index] = req
	}
	m.publish.queue = previews
	m.publish.realQueue = append([]workflow.PublishRequest(nil), reqs...)
	m.publish.publishing = false
	m.setToast(false, "正在生成发布预览…")
	return m, m.dequeuePublish()
}

// publishPreviewArrived 在 dry-run 队列跑完后把预览结果弹进确认框。
func (m *btModel) publishPreviewArrived(report workflow.Report, err error) tea.Cmd {
	p := &m.publish
	realQueue := p.realQueue
	p.realQueue = nil
	if err != nil || !report.OK {
		p.queue = nil
		m.setToast(true, "发布预览失败：%s", localMessage(fallback(report.Error, errString(err))))
		return nil
	}

	sample := m.publishRequest(m.selectedPackage(), false)
	project := m.status.proj()
	spec := project.Platform.Publish
	if spec.Empty() {
		spec = project.Platform.Consume
	}
	lines := []string{
		"包：" + btStyleMono.Render(btPublishPreviewTarget(m, realQueue)),
		"平台：" + btStyleMono.Render(spec.OS+"/"+spec.Arch+" · "+fallback(spec.BuildType, "Release")),
		"工具链：" + btStyleMono.Render(fallback(project.Compiler.Display(), "未设置")+" · Qt "+fallback(btPublishQt(sample), "无")),
		"channel：" + btStyleMono.Render(fallback(sample.Channel, "stable")) + "  备注：" + fallback(sample.Note, "（无）"),
	}
	if sample.Replace {
		lines = append(lines, btStyleWarn.Render("上传成功后删除远程上的旧版本。"))
	}
	m.confirm = &btConfirm{
		title: "确认发布",
		lines: lines,
	}
	m.confirm.onYes = func(model *btModel) tea.Cmd {
		model.publish.queue = append([]workflow.PublishRequest(nil), realQueue...)
		model.publish.publishing = true
		return model.dequeuePublish()
	}
	return nil
}

func (m *btModel) dequeuePublish() tea.Cmd {
	if len(m.publish.queue) == 0 {
		return nil
	}
	req := m.publish.queue[0]
	return m.run(btJobPublish, func(ctx context.Context) (workflow.Report, error) {
		return m.api.PublishPackage(ctx, req)
	})
}

func btPublishTarget(request workflow.PublishRequest, pkg *workflow.PackageInfo) string {
	name := request.Package
	if request.Name != "" {
		name = request.Name
	}
	version := request.Version
	if version == "" && pkg != nil {
		version = pkg.Version
	}
	if version == "" {
		version = "（自动探测）"
	}
	return name + "/" + version
}

// btPublishPreviewTarget 汇总预览队列的目标：单包保持 name/version，
// 多包列出全部名字，供确认框展示。
func btPublishPreviewTarget(m *btModel, queue []workflow.PublishRequest) string {
	if len(queue) == 0 {
		return "（空）"
	}
	if queue[0].All {
		packages := m.packages()
		names := make([]string, 0, len(packages))
		for _, item := range packages {
			names = append(names, item.Name)
		}
		return "全部 " + itoa(len(names)) + " 个组件：" + strings.Join(names, ", ")
	}
	if len(queue) == 1 {
		return btPublishTarget(queue[0], nil)
	}
	names := make([]string, 0, len(queue))
	for _, req := range queue {
		names = append(names, btPublishTarget(req, nil))
	}
	return strings.Join(names, ", ")
}

func btPublishQt(request workflow.PublishRequest) string {
	if request.NoQt {
		return "无"
	}
	return request.QtVersion
}

func (p *btPublishModel) applyReport(report workflow.Report, err error) {
	if p.results == nil {
		p.results = map[string]string{}
	}
	if data, ok := report.Data.(map[string]any); ok {
		if results, ok := data["results"].([]map[string]any); ok {
			for _, result := range results {
				p.applyResult(result)
			}
			return
		}
		if name := stringValue(data["package"]); name != "" {
			if err != nil || !report.OK {
				p.results[name] = "失败：" + fallback(report.Error, "未知错误")
			} else {
				p.results[name] = "成功 " + fallback(stringValue(data["reference"]), name)
				if replaced := stringValue(data["replaced_reference"]); replaced != "" {
					p.results[name] += "（已删旧版 " + replaced + "）"
				}
			}
			return
		}
	}
	// 数据形态对不上时无包名可记；失败信息由 applyReport 的调用方
	//（btModel.applyReport 的 toast）呈现，这里不写键。
}

func errString(err error) string {
	if err == nil {
		return "未知错误"
	}
	return err.Error()
}

func (p *btPublishModel) applyResult(result map[string]any) {
	name := stringValue(result["package"])
	if ok, _ := result["ok"].(bool); ok {
		line := "成功 " + fallback(stringValue(result["reference"]), name)
		if replaced := stringValue(result["replaced"]); replaced != "" {
			line += "（已删旧版 " + replaced + "）"
		}
		p.results[name] = line
		return
	}
	p.results[name] = "失败：" + fallback(stringValue(result["error"]), "未知错误")
}

func (m *btModel) publishView(layout *btLines) {
	p := &m.publish
	packages := m.packages()
	checked := p.checkedNames(packages)
	layout.card("组件清单 · 已勾选 "+itoa(len(checked)), func(l *btLines) {
		if len(packages) == 0 {
			l.add(btStyleMuted.Render("  没有发现可发布的组件：在 packages/* 或 src/* 放好 conanfile.py / dist / lib，或在设置里登记。"))
			return
		}
		l.add("  " + btStyleLabel.Render(padCell("", 2)+padCell("组件", 22)+padCell("来源", 10)+padCell("产物", 10)+"状态"))
		for index, pkg := range packages {
			selected := index == p.cursor
			checkbox := "[ ]"
			if p.checked[pkg.Name] {
				checkbox = "[x]"
			}
			source := "已登记"
			if pkg.Source != "" && pkg.Source != "registered" {
				source = pkg.Source
			}
			artifact := "缺产物"
			if pkg.HasArtifacts {
				artifact = "就绪"
			}
			status := fallback(p.results[pkg.Name], fallback(ternary(pkg.HasRecipe, "配方就绪", "缺配方"), ""))
			component := pkg.Name
			if pkg.Version != "" {
				component += "@" + pkg.Version
			}
			line := "  " + checkbox + " " + btStyleMono.Render(padCell(component, 22)) +
				padCell(source, 10) + padCell(artifact, 10) + btStyleMuted.Render(clip(status, 24))
			l.addRow("pubrow:"+itoa(index), selected, line)
		}
	})

	layout.add("")
	layout.card("发布信息", func(l *btLines) {
		rows := []struct {
			label string
			value string
		}{
			{"包名", p.name.Value()},
			{"版本", p.version.Value()},
			{"channel", p.channel.Value()},
			{"备注", p.note.Value()},
		}
		for index, row := range rows {
			selected := p.formRow == index
			var line string
			if p.editing && selected {
				line = "  " + padCells(row.label, 10) + p.inputTarget(index).View()
			} else {
				line = "  " + padCells(row.label, 10) + btStyleMono.Render(fallback(row.value, "（默认）"))
			}
			l.addRow("pubform:"+itoa(index), selected, line)
		}
		replace := "否"
		if p.replace {
			replace = "是"
		}
		l.addRow("pubform:4", p.formRow == 4,
			"  "+padCells("替换旧版本", 10)+btStyleMono.Render(replace)+btStyleHint.Render("  （点击该行或进入表单后按 Enter 切换）"))
	})

	if !m.combo.open {
		layout.add("")
		layout.addBlock(m.comboBarView("publish"))
	}
	if len(checked) > 0 {
		layout.add(btStyleHint.Render("  已勾选 " + itoa(len(checked)) + " 个组件，按 A 只发布勾选项"))
	} else if len(packages) > 0 {
		layout.add(btStyleHint.Render("  未勾选时按 A 发布全部 " + itoa(len(packages)) + " 个组件"))
	}
}
