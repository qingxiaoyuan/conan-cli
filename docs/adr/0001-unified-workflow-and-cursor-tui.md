# 0001 - 统一 Conan Workflow 与光标式 TUI

**Status:** accepted
**Date:** 2026-09-02
**Spec:** `docs/requirements.md`, `docs/cli-contract.md`

## Context

`conan-cli` 同时提供纯 CLI、VS Code 工作台和终端控制台。三种入口需要共享
项目配置、平台映射、依赖分析、下载和发布逻辑；终端控制台还需要贴合设计稿，
支持真实终端中的光标操作。

## Decision

- VS Code、TUI 和纯 CLI 共用 `internal/workflow`，界面层不重复实现 Conan 业务逻辑。
- TUI 在真实终端使用 ANSI + raw key 实现方向键、Enter、Esc 和 Tab 导航；文本字段进入编辑模式。
- 非交互输入保留行模式，方便脚本和自动化测试。
- 所有实际 Conan 操作仍由 workflow 统一执行，消费阶段固定只下载已有二进制。
- TUI 使用标准库和 `golang.org/x/term`，并根据终端列数调整面板布局。

## Consequences

减少不同入口之间的业务分叉，并保持 CLI JSON 契约稳定；TUI 不依赖大型框架，
但需要自行维护终端尺寸、按键解码、ANSI 状态恢复和跨平台行为。

## Alternatives Considered

- 仅保留行输入：实现简单，但不符合 TUI 设计和光标操作要求。
- 引入完整 TUI 框架：可减少按键处理代码，但会增加依赖和跨平台打包复杂度。
