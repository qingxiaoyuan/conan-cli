# Conan CLI for VS Code

调用 `conan-cli --json`，在编辑器里完成初始化、扫描、按平台下载、设置和发布。

## 本地验证

发布用的 `.vsix` 已内置对应平台的 `conan-cli`、便携 Python 3.12 和 Conan 2，一般不用再配路径，也不用本机再装 Conan。

1. `code --install-extension dist/conan-cli-vscode-0.4.21.vsix`
2. 打开一个 C/C++ 或 Qt 项目
3. 点活动栏 Conan 图标

若仍找不到 CLI，在设置里把 `conanCli.binary` 指到绝对路径。若要改用系统 Conan，填写 `conanCli.conanBinary`。

打包：

```bash
./scripts/package-vscode.sh
```

脚本会下载各平台便携 Python、把 Conan 2.32 装进 `vscode/runtime/<platform>/`，再打出 `.vsix`（约 220MB，含 5 个平台）。只准备某一个平台的 runtime 可加参数，例如 `./scripts/bundle-runtime.sh linux-x64`。

控制台页面：

- 概览：项目、平台、Qt、编译器、最近分析
- 依赖分析：配方对齐 + 选定平台制品
- 拉取依赖：选目标平台后执行 conan install（只取仓库二进制）
- 仓库：查看远程已有组件和已发布版本
- 发布：顶部组件清单列出全部可发布组件（自动发现 / 已登记、产物是否就绪）；点行在下方表单发单个，底部「发布全部」一次发全部（勾选后逐个发布，逐行显示进度）
- 设置：Conan 包名（改名会变成新包）、全局 Nexus 登录
- 诊断：Conan、配置、仓库、平台

侧边栏是同一套命令的窄入口。密码只在设置页录入，通过 `CONAN_PASSWORD` 传给 CLI。
