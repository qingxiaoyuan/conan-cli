# Architecture

## Layers

```text
CLI / TUI / future VS Code extension
                  ↓
             workflow.App
                  ↓
        Conan client / config / adapters
                  ↓
                Conan 2
                  ↓
             Nexus remote
```

所有用户入口共享 `internal/workflow`。CLI 和未来的 VS Code 扩展只负责参数、交互和结果展示；Conan 命令调用集中在 `internal/conan`。

## Package handling

工具不维护 Qt 包和普通 C/C++ 包两套模型。安装和发布统一使用 Conan package reference、Profile 和 Conan 依赖元数据。

Qt 特有的内容应放在后续的构建适配器和诊断中，例如：

- Qt 版本偏好
- qmake `.pri` 文件生成
- Qt runtime/plugin 部署
- Qt ABI 提示

这些能力不应改变基础的包搜索、拉取和发布协议。

## Missing binaries

第一版消费端固定使用 `--build=never`。缺少匹配的 package ID 时直接返回 Conan 错误，并在后续版本中增加结构化诊断和 CI 构建请求。

发布时使用项目配置中的 `channel`（或 `publish --channel` 的显式值）传给 `conan create`；如果 `--ref` 已包含 user/channel，则以显式引用为准。

远程登录的密码通过标准输入传递给 Conan，避免出现在进程参数和错误信息中。
