# AGENTS.md

本文件面向 AI 编码代理，描述本仓库的架构、约定与开发流程。阅读者假定对本项目一无所知。

## 项目概述

`conan-cli` 是面向团队的 **Conan 2 便捷层**（Go 1.22 编写）。用户填一次设置、执行少量命令，即可完成：初始化项目、扫描本机 Qt/编译器、按「操作系统 + 架构」从 Nexus 仓库查找并只下载已有二进制、把包发布到 Nexus。

三个用户入口共享同一套业务逻辑：

- **纯 CLI**（`cmd/conan-cli`）：唯一执行引擎，所有命令支持 `--json`。
- **VS Code 插件**（`vscode/`，纯 JavaScript，无构建步骤）：只调用 CLI 的 `--json` 输出，不解析人类可读文本。
- **终端 TUI**（`internal/tui`）：与 VS Code 共用 workflow，提供同等能力。

核心原则（来自 `docs/requirements.md` 和 `docs/architecture.md`）：

- 消费端固定 `--build=never`（download-only），缺匹配二进制时返回结构化错误，绝不在用户机器上编译。
- 平台 = **操作系统 + 架构**：`windows` / `linux` / `kylin` × `x86` / `x64` / `arm` / `arm64`。编译器和 Qt 是项目设置，查找制品时再与平台组合，不要混进「平台」概念。
- 不维护「Qt 包」与「普通 C/C++ 包」两套模型，统一走 Conan package reference。
- 底层始终调用 Conan 2，不绕过它对接 Nexus REST。**唯一例外**：`catalog` 列出全量组件时 `internal/nexus` 直读 Nexus REST（`conan list` 无法枚举全部包名，详见 `docs/architecture.md`）。

## 技术栈与依赖

- **Go 1.22**，依赖极少：`gopkg.in/yaml.v3`（配置）、`golang.org/x/term`（TUI 按键）。其余全部标准库。**不要随意引入新依赖**（ADR 0001 明确为控制跨平台打包复杂度而避免大型框架）。
- VS Code 插件是零依赖的纯 Node.js CommonJS 脚本（`vscode/extension.js`），无 npm 构建、无 TypeScript。
- 运行时依赖本机或插件内置的 **Conan 2**。CLI 按以下顺序解析 Conan 可执行文件（见 `internal/conan/client.go` 的 `resolveBinary`）：`CONAN_BIN` 环境变量 → `CONAN_CLI_BUNDLED_PYTHON`（便携 Python，以 `-s -m conans.conan` 调用）→ PATH 中的 `conan`。

## 构建与测试命令

```bash
go mod tidy
go test ./...          # 全部测试，当前全绿
go vet ./...
go build -o bin/conan-cli ./cmd/conan-cli
```

- 生成 VS Code 插件包（交叉编译 5 个平台的 CLI + 下载便携 Python/Conan 2 运行时 + 打 `.vsix`）：

  ```bash
  scripts/package-vscode.sh            # 全平台
  scripts/package-vscode.sh linux-x64  # 只打包单个平台的运行时
  ```

  产物在 `dist/`（`.vsix` 与 `conan-cli-linux-amd64`）。`scripts/bundle-runtime.sh` 单独负责下载 python-build-standalone 并 pip 安装 `conan==2.32.0` 到 `vscode/runtime/<platform>/`，可用 `PYTHON_VERSION` / `CONAN_VERSION` / `CACHE_DIR` 等环境变量覆盖。

- `bin/`、`dist/`、`.cache/`、`vscode/bin/`、`vscode/runtime/` 都在 `.gitignore` 中，属生成物，不要提交。

## 代码组织

```text
cmd/conan-cli/main.go    命令分发、flag 解析、文本/JSON 输出切换
internal/
  workflow/              唯一业务层：init/status/scan/analyze/install/publish/settings/config/doctor/packages。
                         所有入口（CLI/TUI/VS Code）只调这里；新增业务能力先进 workflow
  conan/                 对 conan 二进制的封装（执行、list、配方解析）；Conan 命令调用只允许集中在这
  config/                .conan-cli/project.yaml 与 ~/.conan-cli/config.yaml 的读写（YAML）
  atomicfile/            原子写入（temp→sync→rename）；所有配置文件/conanfile 写入都必须走这里
  platform/              平台探测与「os+arch+compiler+Qt → Conan settings」映射
  scan/                  扫描本机 Qt 安装与编译器（只作参考，不自动写入项目）
  manifest/              包标识、命名、预制包（prebuilt）相关逻辑；workspace.go 做 npm workspaces
                         风格组件发现（默认 packages/* 与 src/*，含 conanfile.py / dist/ / lib/ 即收录）
  nexus/                 远程仓库目录查询（catalog）
  output/                统一的 JSON/文本输出（output.Printer）
  profile/               Conan profile 管理
  tui/                   终端控制台：真实终端用 ANSI + raw key（方向键/Enter/Esc/Tab）；
                         管道输入退化为行模式，便于脚本和测试；设置字段表 settings_fields.go
                         为行模式与光标模式共用，新增字段只改这一处
vscode/                  VS Code 插件（extension.js、sidebar/dashboard webview、package.json）
scripts/                 打包脚本（见上）
docs/                    requirements.md（PRD）、architecture.md、cli-contract.md（JSON 契约）、adr/
ui-design/               界面设计稿 HTML（参考用，不参与构建）
```

分层约束（`docs/architecture.md`）：入口层只做参数解析、交互和展示；Conan 进程调用集中在 `internal/conan`；业务规则集中在 `internal/workflow`。

## 配置与数据文件

- 项目配置：`<项目>/.conan-cli/project.yaml`（可提交 git）。`packages[]` 列出可发布组件（`name` / `version` / `lib_dirs` / `include_dirs`）；未填时发布仍把顶层 `name` 当作唯一组件，产物扫描仓库根 `lib/`、`bin/`。发布配方在 `.conan-cli/recipes/<包名>/`，不覆盖仓库根消费用 conanfile。`workspaces[]` 是组件发现 glob（默认 `packages/*` 与 `src/*`，可覆盖）：命中的目录自带 conanfile.py / dist/ / lib/ 即自动成为可发布组件，无需登记 `packages[]`；同名时 `packages[]` 优先。workspace 组件发布时直接在其目录 `export-pkg`、配方就地补丁，不写 `.conan-cli/recipes/`，也不回写 `packages[]`。`publish --all` 逐包发布全部组件（失败不中断，聚合在 `data.results`）。`publish --replace` 上传成功后删除远程旧版本（升降级时替换，删除失败仅告警不影响发布结果）。
- 全局配置：`~/.conan-cli/config.yaml`；密码存 `~/.conan-cli/credentials`（权限 0600）。
- 命令执行时以 `--dir <path>` 指定项目目录，默认当前目录。

## 测试约定

- 测试与源码同包并列（`*_test.go`），标准 `testing` 包，无外部测试框架。
- 需要 Conan 的地方用 fake/stub（测试不应依赖真实 Conan 或网络）；TUI 的行模式就是为自动化测试保留的。
- VS Code 插件的参数构造函数在 `vscode/args.js`（不依赖 vscode 模块），用 `node vscode/args.test.js` 跑断言；`*.test.js` 不进 `.vsix`。
- 提交前必须跑 `go test ./...` 和 `go vet ./...`，两者当前都是干净的，不要引入新的失败或警告。
- `internal/profile` 目前无测试文件；`cmd/conan-cli` 的覆盖靠 `internal/workflow` 和 `internal/tui` 的测试间接保证。

## 代码风格与约定

- **界面与终端文案一律中文**（NFR-01）；面向用户的错误信息要给出「下一步怎么做」，而不是只抛原始 Conan 输出。
- Go 代码遵循标准 `gofmt` 风格；注释中英文混用均可，现有代码里简短英文注释和中文说明都有，跟随所在文件。
- 所有供界面使用的命令必须支持 `--json`，输出单个 JSON 对象，结构为 `ok` / `action` / `data` / `error` / `exit_code`（契约见 `docs/cli-contract.md`，改动它时必须同步更新该文档）。
- 平台取值固定为小写枚举（`windows|linux|kylin`、`x86|x64|arm|arm64`），编译器为 `gcc|clang|msvc`。
- `.gitignore` 中 `/conan/` 特意锚定仓库根，**不能**写成 `conan/`，否则会误伤 `internal/conan` 源码目录——改动忽略规则时注意这一点。

## 安全注意事项

- 密码只经 `CONAN_PASSWORD` 环境变量传入，登录时通过 **stdin** 交给 Conan；**绝不**出现在命令行参数、日志、错误或 JSON 输出里。
- 密码文件 `~/.conan-cli/credentials` 必须保持 0600。
- 登录信息只存本机，不进项目仓库；`.conan-cli/` 已被忽略。
- 配置文件要原子写入，避免写一半损坏（NFR-03）。

## 重要文档

- `docs/requirements.md`：产品需求（PRD），含「已确认决策」与平台/配置模型，改行为前先对照它。
- `docs/cli-contract.md`：CLI `--json` 契约，VS Code 插件依赖它，属稳定性承诺。
- `docs/architecture.md`：分层与包处理原则。
- `docs/adr/0001-unified-workflow-and-cursor-tui.md`：为何三入口共用 workflow、TUI 不用框架。

## 开发流程提示

- 新增功能时：业务逻辑进 `internal/workflow` → CLI 在 `cmd/conan-cli/main.go` 加分发与 flag → TUI/VS Code 只接展示。不要在界面层复制业务逻辑。
- 改了 `--json` 输出结构、命令或配置字段，必须同步 `docs/cli-contract.md`、`README.md` 和本文件。
- 发布插件版本时更新 `vscode/package.json` 的 `version`，然后跑 `scripts/package-vscode.sh`。
