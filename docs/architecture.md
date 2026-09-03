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

所有用户入口共享 `internal/workflow`。CLI、TUI 和 VS Code 扩展只负责参数、交互和结果展示；Conan 命令调用集中在 `internal/conan`。

## Nexus REST 例外

包的查找、安装、登录、发布始终通过 Conan 2 完成，不绕过它对接 Nexus REST。唯一例外是 `catalog` 的全量组件列表：`conan list` 无法枚举仓库中所有包名，因此在配置了 Nexus URL 时由 `internal/nexus` 直接调用 Nexus REST 只读接口；查询失败或按名搜索时回退到 `conan list`。该例外不涉及认证写入，密码仍只经 Conan 登录通道。

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

发布固定用 `conan export-pkg` 打包本机已编译的库（不会在本机触发编译），再 `conan upload name/version` 上传。channel 保存在项目设置（默认 `dev`），随发布预览与结果数据返回；上传引用目前是 name/version，是否引入 `user/channel` 是待定决策（见 requirements.md 的开放问题）。

远程登录的密码通过标准输入传递给 Conan，避免出现在进程参数和错误信息中。
