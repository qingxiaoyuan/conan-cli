# 0002 - 用 BubbleTea 重写交互式 TUI，对齐 VS Code 控制台

**Status:** accepted
**Date:** 2026-09-08
**Spec:** `docs/requirements.md`、`vscode/extension.js`（体验基准）
**Supersedes:** ADR 0001 中「TUI 不依赖大型框架」的决策（其余条目继续有效）

## Context

ADR 0001 为控制依赖规模，让 TUI 用标准库 + `golang.org/x/term` 手写 ANSI 光标模式。
随着 VS Code 插件演进为 6 个 Tab 的单页控制台（拉取依赖 / 仓库 / 依赖 / 发布 / 设置 / 诊断，
含并行状态刷新、组合编辑、发布确认弹窗等），手写实现的按键解码、布局与重绘成本
已经超过维护一个框架的成本，且两个入口的用户体验明显不一致。

用户明确要求 TUI 与 VS Code 插件体验一致，并指定 BubbleTea 方案。

## Decision

- 交互式（真实终端）TUI 改用 `charmbracelet/bubbletea` + `bubbles` + `lipgloss` 重写，
  信息架构对齐 VS Code 控制台：同一组 Tab、头部状态徽标、并行刷新 `status + doctor + analyze`。
- 业务调用仍全部经 `internal/workflow`，`tuiAPI` 接口隔离以便测试注入 fake。
- 非交互（管道/非 tty）输入继续走原有行模式，脚本契约与既有测试不变。
- 命令超时对齐 VS Code 插件：常规操作 120s，install/publish 30min。
- 语言版本升到 **Go 1.23**（bubbletea 1.3 的 `go` 指令要求；本仓库 `go.mod` 写 `go 1.23.0`）。

## Consequences

- 直接依赖增加 bubbletea / bubbles / lipgloss 三个模块（含 termenv 等传递依赖），
  打包体积与模块数上升；换取终端尺寸自适应、按键解码、重绘与组件（textinput/spinner/viewport）。
- 构建环境需要 Go 1.23+。
- ADR 0001 的「不依赖大型框架」条目作废；行模式保留意味着两套界面仍并存。
- `internal/tui/interactive.go` 的手写光标模式整体删除。

## Alternatives Considered

- 继续手写扩展：不加依赖，但要自行补齐 resize、重绘 diff、组件编辑器，且难以追平 VS Code 体验。
- 行模式也换成 BubbleTea headless：单一代码路径，但破坏 `conan-cli tui < input.txt`
  的脚本契约与既有测试，收益有限。
