# conan-cli

面向团队的 Conan 2 便捷层。用户填一次设置、执行少量命令，即可初始化项目、扫描 Qt/编译器、按操作系统和架构下载已有二进制，并发布包到 Nexus。

## 当前能力

- CLI 是唯一执行引擎；VS Code 插件只调用 `--json`
- `init` 生成 `.conan-cli/project.yaml`；没有配方时生成最简 `conanfile.txt`
- `scan` 可选，只列出本机 Qt/编译器作参考，不写入项目；目标平台、Qt、编译器在设置里手填
- `analyze --os/--arch` 对比配方与项目依赖，并查询仓库制品
- `install --os/--arch --build-type` 只下载二进制（`--build=never`）；Debug 与 Release 是两套制品
- `config` 管理本机 Nexus 地址和登录（密码不进 git）
- `settings` 管理项目 Qt、编译器、使用/发布平台、channel
- `publish` 支持表单字段和 `--dry-run` 预览。正式发布把该组件配方写到 `.conan-cli/recipes/<包名>/`，不改仓库根的消费配方，再 `export-pkg` 打包本机已编译库。多组件用 `--package`，或用 `--all` 逐包发布全部组件（单包失败不中断，结果聚合在 `data.results`）；产物路径用 `--lib-dir` / `packages[].lib_dirs`；`--replace` 在上传成功后删除远程上的旧版本（升降级场景）
- `packages list` 列出全部可发布组件：`packages/*` 与 `src/*` 下自带 conanfile.py 或 dist/ 产物的目录自动成为组件（npm workspaces 风格，无需登记 packages[]），与 `packages[]` 合并展示；workspace 组件发布时直接在其目录就地补丁配方并 `export-pkg`
- `catalog` 列出远程包及每条制品的平台、编译器、Qt、Debug/Release，可用 `--os/--arch/--compiler/--qt/--build-type/--no-qt` 筛选
- VS Code 控制台：概览、仓库表（含制品字段与筛选）、依赖分析、下载、发布、设置、诊断

平台是 **操作系统 + 架构**：Windows / Linux / 麒麟 × x86 32 位 / x64 64 位 / ARM 32 位 / ARM 64 位。编译器和 Qt 是项目设置，查找时再组合。

## 构建

需要 Go 1.23+。命令行直接用时本机还要有 Conan 2；VS Code 插件已内置便携 Python 和 Conan 2。

```bash
go mod tidy
go test ./...
go build -o bin/conan-cli ./cmd/conan-cli
```

## 命令示例

```bash
conan-cli init
conan-cli config set --name nexus --url https://nexus.example.com/repository/conan-hosted/ --username alice
CONAN_PASSWORD=PASSWORD conan-cli config login
conan-cli settings set --qt 6.8 --compiler gcc --compiler-version 11 --os kylin --arch x64
conan-cli scan
conan-cli analyze --os kylin --arch x64
conan-cli catalog --os linux --arch x64 --build-type Release
conan-cli add fmt/10.2.1
conan-cli install --os kylin --arch x64
conan-cli publish --dry-run --os kylin --arch x64
  conan-cli publish --os kylin --arch x64 --channel dev --lib-dir build/Release
conan-cli packages list
conan-cli publish --all --dry-run --os kylin --arch x64
conan-cli doctor
```

## 配置

项目：`.conan-cli/project.yaml`

全局：`~/.conan-cli/config.yaml`，密码：`~/.conan-cli/credentials`（0600）

```yaml
name: conan-demo
build_system: qmake
qt_version: "6.8"
compiler:
  id: gcc
  version: "11"
platform:
  consume:
    os: kylin
    arch: x64
  publish:
    os: kylin
    arch: x64
remote: nexus
channel: dev
missing_binary_policy: download-only
output_folder: conan
workspaces:        # 可选，组件发现 glob，默认 ["packages/*", "src/*"]
  - packages/*
  - src/*
dependencies:
  - fmt/10.2.1
```

## VS Code 插件

见 [vscode/README.md](vscode/README.md)。发布用的 `.vsix` 已带各平台 `conan-cli`、Python 和 Conan 2。开发时若要用仓库里的 CLI：

```json
{ "conanCli.binary": "bin/conan-cli" }
```

## TUI 终端控制台

```bash
conan-cli tui
```

终端控制台用 [BubbleTea](https://github.com/charmbracelet/bubbletea) 实现（ADR 0002），体验与 VS Code 控制台一致：六个 Tab（拉取依赖 / 仓库 / 依赖 / 发布 / 设置 / 诊断），顶部常驻登录徽标、目标组合 chip 和任务进行中提示，进入时并行刷新 `status + doctor + analyze`。界面铺满整个终端（全屏皮肤），支持鼠标操作和四套主题。

**鼠标**：可点击 Tab 栏、Tab 下方的动作按钮条、组合编辑器的选项行、仓库过滤器 chip、仓库/发布的列表行、设置字段行和“仓库没有这套制品”卡片（点卡片直接改组合）；滚轮等同 `↑/↓`。

**主题**：默认暗色。`T` 键循环切换 暗色 → 亮色 → 透明（沿用终端自身背景）→ Nord，或用环境变量 `CONAN_CLI_TUI_THEME=dark|light|transparent|nord` 指定初始主题。

常用按键：

- `1`–`6` 或 `←/→` 切换 Tab，`q` / `Ctrl-C` 退出，`r` 刷新全部状态，`T` 切换主题。
- **拉取依赖**：`e` 展开目标组合编辑器（`←/→` 改值、`s` 扫描本机 Qt/编译器预填、改动即静默保存），`a` 检查依赖，`g` 生成配方，`i` 拉取依赖（只取仓库二进制，缺件时展示“仓库没有这套制品”卡片）。
- **仓库**：输入包名回车查询，`Tab` 在搜索 / 过滤器 / 结果间切换焦点，`Enter` 展开组件，`a` 把选中制品加入依赖。
- **发布**：`↑/↓` 选组件、`space` 勾选、`Enter` 填发布信息，`p` 发布当前行、`A` 在有勾选时只发布勾选项、未勾选时发布全部；发布前弹出确认摘要（含替换旧版本提示）。
- **设置**：`↑/↓` 选字段、`Enter` 编辑，`s` 保存项目设置、`l` 保存并登录仓库、`t` 测试连接；密码不回显，留空表示保持已保存。

通过管道运行时保留行输入兼容模式（与 VS Code 无关的旧菜单），便于脚本和测试。
