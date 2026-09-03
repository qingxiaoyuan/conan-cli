# Conan 便捷使用工具需求文档

| 项 | 内容 |
|----|------|
| 产品 | `conan-cli`（命令行引擎 + 设置/工作台界面） |
| 文档类型 | 产品需求（PRD） |
| 版本 | 0.2 |
| 日期 | 2026-09-02 |
| 状态 | 草案（已纳入首轮产品确认） |

本文描述用户真正要的产品，不是当前第一版已经实现的全部能力。文末单独列出「已有 / 缺口」。

---

## 1. 一句话需求

做一个 **Conan 2 便捷层**：用户不必记 Conan 细节，只要填一次设置、执行少量命令（或在界面点按钮），就能完成：

1. 初始化或读取项目 Conan 配置
2. 分析包依赖
3. 扫描项目 Qt 版本和编译器
4. 按自己选定的平台，到 Conan/Nexus 仓库查找并下载对应制品
5. 在设置页管理全局仓库与项目工具链
6. 用表单快捷发布 Conan 包（含登录信息）

底层仍然调用 Conan，不替换 Conan，也不在本机现场编译缺失二进制。

---

## 2. 要解决的问题

团队里多数人不是 Conan 专家。现在要用 Conan，通常要自己处理：

- `conanfile` / Profile / remote / channel / package ID
- Qt 版本、编译器、操作系统、架构是否匹配
- Nexus 地址、账号、该拉哪套预编译包
- 发布时 `create` + `upload` 的参数组合

结果是：配置分散、容易下错 ABI、发布步骤长、出错信息难读。

本工具把这些收成 **「设置一次 + 执行命令」**。用户面对的是项目、平台、Qt、编译器、仓库，而不是一长串 Conan 参数。

---

## 3. 产品定位

| 维度 | 约定 |
|------|------|
| 是什么 | 面向团队的 Conan 工作流工具 |
| 不是什么 | 不是新的包管理器，不是 CI 构建农场，不是通用 C++ IDE |
| 执行方式 | 所有真实动作最终都是命令（CLI）。界面只负责读配置、填表、展示结果、触发命令 |
| 消费策略 | 只下载仓库中已有二进制（`--build=never`）。缺匹配制品时明确告知，不在用户机器上编译 |
| 发布策略 | 在选定 Profile/平台下 `create` 并上传到已配置的 Nexus Conan 仓库 |
| 包类型 | Qt 组件、普通 C/C++ 库、header-only、预编译 SDK 走同一套包引用，不拆两套模型 |

推荐入口：

- **VS Code 工作台**：设置页、依赖分析、扫描结果、下载、发布表单
- **终端控制台（TUI）**：同等能力，包括改全局/项目设置，不依赖 VS Code
- **纯命令**：同一套 `conan-cli`，给脚本和无交互环境用

两条 GUI 都只是触发命令，不以两套业务逻辑实现。

---

## 4. 用户与场景

### 4.1 角色

| 角色 | 诉求 |
|------|------|
| 应用开发者 | 打开项目就能看到依赖和 Qt/编译器；选平台后下载能用的包 |
| 库作者 | 填发布表、选平台/channel，一键发布到 Nexus |
| 仓库管理员 | 配好全局 Nexus 地址；开发者各自登录 |

### 4.2 主场景

**场景 A：新项目接入 Conan**

开发者在含源码的目录执行初始化（命令或按钮）。工具生成项目 Conan 配置，尽量识别已有 `conanfile`、构建系统、Qt 与编译器。开发者在设置页确认仓库和平台后，执行安装。

**场景 B：已有 Conan 项目**

工具读取 `conanfile.py` / `conanfile.txt` 和 `.conan-cli/project.yaml`（若有），给出依赖分析，而不是要求重新 init 覆盖配方。

**场景 C：按平台拉制品**

开发者指定目标平台（可与本机不同）。工具用该平台对应的 Conan 设置，在 Nexus 上查找依赖的匹配二进制并下载。找不到则列出缺哪些包、缺哪组设置，而不是丢一段原始 Conan 日志了事。

**场景 D：发布自己的包**

作者在设置页保存 Nexus 登录信息。发布时填包名/版本/channel/平台等表单，确认后由工具执行创建并上传。

---

## 5. 范围

### 5.1 要做

- 初始化生成项目 Conan 配置
- 读取项目里已有 Conan 配置（配方 + 本工具项目配置）
- 依赖分析（声明了什么、和配方是否一致、选定平台上有没有二进制）
- 扫描并展示项目 Qt 版本、编译器；允许手工覆盖
- 用户选择平台后，到 Conan/Nexus 仓库查找并下载对应制品
- 设置页：
  - **全局**：Nexus Conan 仓库地址、登录信息
  - **项目**：Qt 版本、编译器、使用/发布平台、channel 等
- 发布快捷方案：登录在设置页录入；发布用表单；一键执行 create + upload

### 5.2 明确不做（本阶段）

- 不在开发者机器上为缺失二进制执行 `--build=missing`
- 不做 CI 构建矩阵、不替用户编译多平台包
- 不对接 Nexus REST 来绕过 Conan（仓库操作仍走 `conan remote` / `conan list` / `upload`）
- 不维护 Qt 包与普通 C 库两套协议
- 不自动改写动态 `requirements()` 的 Python 配方
- 不把密码写进命令行参数或提交到 git

---

## 6. 功能需求

编号规则：`F-模块-序号`。优先级：P0 必须有，P1 应有，P2 可后置。

### 6.1 初始化与读取配置

| ID | 需求 | 优先级 |
|----|------|--------|
| F-CFG-01 | 用户可对当前项目执行初始化，生成 `.conan-cli/project.yaml`（若不存在）。已存在时不得无提示覆盖用户改过的字段，应读取并补齐缺省项 | P0 |
| F-CFG-02 | 初始化时检测构建系统（qmake / CMake / 未知），写入 `build_system` | P0 |
| F-CFG-03 | 若项目已有 `conanfile.py` 或 `conanfile.txt`，必须读取其中的静态依赖，并同步进项目配置的依赖列表（动态 `requirements()` 只提示「无法静态解析」，不报成失败） | P0 |
| F-CFG-04 | 无 conanfile 时，初始化必须同时生成最简 `conanfile.txt`（`[requires]` + 与探测到的构建系统匹配的 generators）和项目配置，用户可立刻 add/install | P0 |
| F-CFG-05 | 提供「读取/刷新项目 Conan 配置」动作：重新扫描配方、项目 yaml、探测结果，刷新工作台，不改用户未确认的设置 | P0 |

### 6.2 依赖分析

| ID | 需求 | 优先级 |
|----|------|--------|
| F-DEP-01 | 展示项目声明依赖列表（来自项目配置与 conanfile），标出两边不一致项 | P0 |
| F-DEP-02 | 对选定平台，查询 Nexus/Conan remote：每个直接依赖是否存在匹配二进制 | P0 |
| F-DEP-03 | 分析报告至少包含：包引用、当前平台、是否找到制品、未找到时的原因摘要（无包 / 无该版本 / 无匹配 settings） | P0 |
| F-DEP-04 | 支持添加依赖：同时更新项目配置和可静态编辑的配方；动态 Python 配方拒绝自动改写并给出手工指引 | P0 |
| F-DEP-05 | 支持按包名搜索 remote 中的包/版本 | P1 |
| F-DEP-06 | 间接依赖树（完整 graph）可后置；P0 先保证直接依赖 + 安装结果 | P2 |

### 6.3 扫描 Qt 与编译器

| ID | 需求 | 优先级 |
|----|------|--------|
| F-SCN-01 | 扫描项目 Qt 版本，来源包括但不限于：`.pro` 中的 Qt 相关配置、CMake 中的 Qt 查找、已有 `project.yaml` 的 `qt_version`、本机 `qmake -query` | P0 |
| F-SCN-02 | 扫描编译器信息：编译器类型（gcc / clang / msvc）、主版本。来源包括：本机默认编译器、已有 Conan profile、项目工具链文件 | P0 |
| F-SCN-03 | 扫描结果写入工作台「探测」区，默认建议填入项目配置，但必须允许用户改 | P0 |
| F-SCN-04 | 探测失败时不阻断其他功能，明确显示「未探测到，请在设置中手填」 | P0 |
| F-SCN-05 | 探测规则与置信度（文件命中 / 命令命中 / 手工）在诊断里可见 | P1 |

### 6.4 平台与制品下载

用户说的「平台」是 **操作系统 + 架构**，不是 Conan profile 名，也不是和编译器绑死的内部代号。

操作系统（P0 必选）：

- Windows
- Linux
- 麒麟

架构（P0 必选）：

- x86（32 位）
- x64（64 位）
- ARM（32 位）
- ARM64（64 位）

编译器、Qt 版本是项目设置里的另两项，查找/下载时与平台组合使用，但界面上不要把它们塞进「平台」四个字里。

| ID | 需求 | 优先级 |
|----|------|--------|
| F-PLT-01 | 用户可为「使用平台」分别选择操作系统与架构。界面展示成人话（例如「麒麟 / x64」），不要先要求用户写 profile | P0 |
| F-PLT-02 | 操作系统至少支持 Windows、Linux、麒麟；架构至少支持 x86、x64、ARM。目标平台由用户选择，不使用开发机作为默认 | P0 |
| F-PLT-03 | 工具把「操作系统 + 架构 + 项目里的编译器 + Qt」映射成 Conan settings/profile，再去仓库查找。麒麟在用户侧是独立系统；映射到 Conan 时按麒麟实际 ABI 处理（常见为 Linux 系 + 发行版/工具链差异），映射表可配置 | P0 |
| F-PLT-04 | 「查找制品」：按当前项目依赖 + 选定操作系统/架构 + 项目编译器/Qt，在已配置 remote 上列出匹配情况 | P0 |
| F-PLT-05 | 「下载制品」：对上述组合执行安装，固定只拉二进制，输出到项目配置的 `output_folder`（默认 `conan`） | P0 |
| F-PLT-06 | 下载前可预览将使用的 Conan settings（os/arch/compiler/Qt），避免静默用错 ABI | P1 |
| F-PLT-07 | 缺二进制时返回结构化失败：缺哪个引用、当前操作系统/架构/编译器/Qt、建议检查仓库还是改选项；保留 Conan 原始输出供展开 | P0 |
| F-PLT-08 | 使用平台与发布平台可分别配置；默认同步，允许拆开 | P1 |
| F-PLT-09 | 不支持的组合（若有）在选择时就禁用或警告，不要等到 Conan 报错 | P1 |

### 6.5 设置页

设置分为 **全局** 与 **项目** 两层。全局跨项目生效；项目写入当前仓库的 `.conan-cli/project.yaml`。

#### 全局设置（本机用户级）

| ID | 字段 | 说明 | 优先级 |
|----|------|------|--------|
| F-SET-G01 | Nexus Conan 仓库名称 | 如 `nexus` | P0 |
| F-SET-G02 | Nexus Conan 仓库 URL | 如 `https://nexus.example.com/repository/conan-hosted/` | P0 |
| F-SET-G03 | 登录用户名 | 保存在本机用户配置，不进项目 git | P0 |
| F-SET-G04 | 登录密码/Token | 本机安全存储；执行 `remote login` 时经 stdin 交给 Conan，不出现在 argv、日志、错误信息里 | P0 |
| F-SET-G05 | 测试连接 | 用当前全局仓库做 login 或 list，显示成功/失败 | P0 |
| F-SET-G06 | 默认 conan-cli / conan 可执行文件路径 | 可选覆盖 | P1 |

全局仓库变更后，工具负责把 Conan remote 配到本机（`remote add --force` + login），项目默认 `remote` 可引用该名称。

#### 项目设置

| ID | 字段 | 说明 | 优先级 |
|----|------|------|--------|
| F-SET-P01 | 项目名称 | 默认目录名 | P0 |
| F-SET-P02 | Qt 版本 | 扫描建议 + 手填，如 `6.8` | P0 |
| F-SET-P03 | 编译器 | 类型 + 版本，如 `gcc 11` / `msvc 193` | P0 |
| F-SET-P04 | 使用平台 | 操作系统 + 架构，消费/下载用 | P0 |
| F-SET-P05 | 发布平台 | 操作系统 + 架构，创建/上传用，可与使用平台相同 | P0 |
| F-SET-P06 | 发布 channel | 默认 `dev` | P0 |
| F-SET-P07 | 构建系统 | qmake / cmake / unknown | P1 |
| F-SET-P08 | 输出目录 | 默认 `conan` | P1 |
| F-SET-P09 | 缺二进制策略 | 本阶段固定 `download-only`，界面只读说明 | P0 |

设置页保存即生效到对应配置文件；需要登录仓库的操作在保存后可自动或手动执行一次 login。VS Code 与终端 TUI 都必须能查看和修改上述全局/项目字段。

### 6.6 发布快捷方案

| ID | 需求 | 优先级 |
|----|------|--------|
| F-PUB-01 | 发布入口是表单，不是让用户拼 `conan create` 命令 | P0 |
| F-PUB-02 | 表单字段：包名、版本、channel、发布平台、remote（默认全局 Nexus）、备注/说明（可选） | P0 |
| F-PUB-03 | 包名/版本默认从 `conan inspect` 读取；读不到则必填，禁止空引用发布 | P0 |
| F-PUB-04 | 若 `--ref` 已含 `user/channel`，以表单显式值为准；否则 channel 用项目设置 | P0 |
| F-PUB-05 | 提交前确认页：展示将执行的 create + upload 摘要（平台、channel、remote、引用） | P0 |
| F-PUB-06 | 未登录或仓库不可达时，发布被阻止，并跳转/提示去设置页补登录 | P0 |
| F-PUB-07 | 发布成功返回：引用、平台、remote、channel、命令输出摘要 | P0 |
| F-PUB-08 | 发布失败保留 Conan 退出码和输出；不出现密码 | P0 |
| F-PUB-09 | 登录信息只在设置页录入和更新，发布表单不重复要密码（除非登录已失效） | P0 |

### 6.7 命令与界面触发

用户「只需要执行命令」指：业务动作都有对应 CLI。界面按钮等于调用这些命令。

| 用户动作 | 命令（示意） |
|----------|----------------|
| 初始化 | `conan-cli init` |
| 读取/诊断 | `conan-cli doctor`，后续可增 `conan-cli inspect-project` |
| 扫描 Qt/编译器 | `conan-cli scan`（待实现） |
| 依赖分析 | `conan-cli analyze [--platform <id>]`（待实现） |
| 查找制品 | `conan-cli search` / `analyze` |
| 按平台下载 | `conan-cli install [--platform <id>]` |
| 保存全局仓库 | `conan-cli remote add` + `remote login` |
| 发布 | `conan-cli publish`（参数由表单生成） |

所有供界面使用的命令必须支持 `--json`，成功/失败结构与现有契约一致：`ok`、`action`、`data`、`error`、`exit_code`、`output`。

---

## 7. 配置模型

### 7.1 全局（本机，不进项目仓库）

建议路径：`~/.conan-cli/config.yaml`（或系统用户配置目录等价路径）。

```yaml
nexus:
  name: nexus
  url: https://nexus.example.com/repository/conan-hosted/
  username: alice
  # 密码不写明文 yaml；走本机密钥存储或独立权限文件
conan_bin: conan
cli_bin: conan-cli
```

密码存储要求：默认不出现在已跟踪的项目文件中；文档需说明存放位置。

### 7.2 项目（可提交）

路径：`.conan-cli/project.yaml`

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
dependencies:
  - fmt/10.2.1
  - qtutils/1.0
```

用户侧平台只保存 `os` + `arch`。查找制品时再叠加上项目的 `compiler` 与 `qt_version`，由内置/可配置映射表转成 Conan settings。例如：

```text
os=kylin + arch=x64 + compiler=gcc 11 + qt=6.8
  → Conan os/arch/compiler/compiler.version（及团队约定的麒麟发行版字段）
```

`os` 取值：`windows` / `linux` / `kylin`。`arch` 取值：`x86` / `x64` / `arm` / `arm64`。

Qt 版本是项目偏好，用于过滤/提示与 Qt 相关的包，不改变「一个包引用」的核心协议。

### 7.3 与 Conan 原生长文件的关系

| 文件 | 谁维护 |
|------|--------|
| `conanfile.py` / `conanfile.txt` | 项目配方，工具可读、有限可写 |
| `.conan-cli/project.yaml` | 本工具项目设置 |
| Conan profile | 由平台选择生成或复用，用户不必手写也能工作 |
| `~/.conan/` 登录态 | 由 `remote login` 写入，工具不重复发明一套 Conan 凭证协议 |

---

## 8. 界面结构（工作台）

VS Code 与终端 TUI 都要覆盖同一组页面能力（布局可以不同，字段必须齐）：

1. **概览**：项目名、构建系统、探测到的 Qt/编译器、当前使用平台（系统 + 架构）、依赖数、最近一次分析/下载结果
2. **依赖分析**：列表 + 选定平台上的制品状态
3. **设置**（两边都能改，不能只在 VS Code 里改）
   - 全局：Nexus URL、账号、测试连接
   - 项目：Qt、编译器、使用平台（系统/架构）、发布平台（系统/架构）、channel
4. **下载**：选择操作系统与架构 → 分析 → 下载
5. **发布**：表单 → 确认 → 结果
6. **诊断**：Conan 是否可用、配方是否存在、依赖是否对齐、仓库是否可达、登录是否有效
7. **终端设置**：TUI 提供与 VS Code 对应的全局/项目设置编辑，保存走同一套 CLI

纯 CLI 通过子命令和参数完成同等能力，与界面字段一一对应。

---

## 9. 非功能需求

| ID | 需求 |
|----|------|
| NFR-01 | 界面与终端文案默认中文 |
| NFR-02 | 密钥不进 argv、git、普通日志 |
| NFR-03 | 配置文件原子写入，避免写到一半损坏 |
| NFR-04 | VS Code 只解析 CLI 的 JSON，不解析人类可读文本 |
| NFR-05 | 下载/发布等长操作可取消（中断进程），并回显已产生的输出 |
| NFR-06 | 无 Conan、无网络、未初始化、动态配方等失败要有明确下一步，而不是只给退出码 |
| NFR-07 | 不要求用户理解 package ID 公式；需要时在「高级/原始输出」中展开 |

---

## 10. 与当前实现的差距

当前仓库已有：CLI 骨架、`init`、读写 `project.yaml`、静态配方 `add`、download-only `install`、`publish`（create+upload）、`doctor`、中文 TUI、VS Code 控制台、`--json`、stdin 登录。

相对本需求，主要缺口：

| 缺口 | 说明 |
|------|------|
| 全局设置与设置页 | 尚无用户级 Nexus/登录配置文件和设置 UI |
| 读取并分析已有配方 | `doctor` 只做对齐检查，没有平台维度的制品分析 |
| 扫描 Qt / 编译器 | `qt_version` 字段已预留，未实现扫描 |
| 平台选择 | 尚无「操作系统 + 架构」（Windows/Linux/麒麟 × x86/x64/ARM）的用户模型 |
| 结构化缺包说明 | 缺二进制时仍接近原始 Conan 错误 |
| 发布表单 | 插件是确认框 + 默认参数，不是填表 |
| `scan` / `analyze` 命令 | 尚未提供 |

---

## 11. 验收标准（P0 最小可用）

1. 新目录执行初始化后，生成项目配置；已有 `conanfile.txt` 的项目能被读出依赖。
2. 设置页能保存全局 Nexus 地址并登录；之后各项目可直接用该 remote。
3. 扫描能给出 Qt 或编译器的建议值，或明确「请手填」；手填后用于后续下载/发布。
4. 用户选择操作系统和架构后，能看到每个直接依赖在该平台（再叠加项目编译器/Qt）是否有制品，并能一键只下载二进制。
5. 缺匹配制品时，界面/JSON 说明缺什么，不泄露密码。
6. 设置页已登录的前提下，发布表单填完即可 create + upload，结果可追踪。
7. 上述动作在 CLI 侧均有对应命令，VS Code 只调用 `--json`。

---

## 12. 已确认决策

1. **双界面改设置**：VS Code 和终端 TUI 都能查看、修改全局/项目设置；CLI 是唯一执行引擎。不做第三套独立桌面窗口。
2. **平台 = 操作系统 + 架构**：系统为 Windows / Linux / 麒麟，架构为 x86 / x64 / ARM。编译器、Qt 是旁边的项目设置，查找制品时与平台组合，但不是「平台」本身。
3. **空项目 init**：没有 conanfile 时自动生成最简 `conanfile.txt`，并同时写出项目配置。
4. 消费端始终 download-only。
5. 一个全局默认 Nexus；多仓库管理可后置。
6. 登录只保存在本机，不进项目仓库。
7. 「nonan 仓库」按 **Conan 仓库（Nexus hosted）** 理解。

---

## 13. 仍待确认

1. **全局配置位置**：`~/.conan-cli/config.yaml` 是否可接受？密码用系统钥匙串还是权限收紧的本地文件？
2. **跨平台查询**：在 Linux 开发机上是否必须能分析/下载 Windows 或麒麟制品（本机未必能链接该 ABI）？
3. **发布 user**：包引用是否需要 `user`（如 `myteam`），还是只用 name/version + channel？
4. **麒麟映射**：仓库里麒麟包是按独立 `os=Kylin`、Linux + distro 字段，还是单独 profile 命名？需要对照现有 Nexus 包约定后再锁映射表。
5. **ARM 变体**：P0 的 `arm` 是否先按 AArch64（armv8）处理，ARMv7 另开选项后置？

---

## 14. 建议落地顺序

1. 全局设置（Nexus URL + 登录）+ 设置页读写
2. 读取已有配方 + 依赖分析命令/页面
3. 扫描 Qt、编译器，写入项目设置
4. 操作系统/架构平台模型 + 按平台查找/下载 + 缺包结构化结果
5. 发布表单（预填 inspect 信息）与发布前确认
6. 间接依赖树、多仓库、构建系统适配器（qmake `.pri` 等）
