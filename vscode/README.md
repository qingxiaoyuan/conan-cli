# Conan CLI for VS Code

调用 `conan-cli --json`，在编辑器里完成初始化、扫描、按平台下载、设置和发布。

## 本地验证

发布用的 `.vsix` 已内置对应平台的 `conan-cli`，一般不用再配路径。

1. `code --install-extension dist/conan-cli-vscode-0.4.14.vsix`
2. 打开一个 C/C++ 或 Qt 项目
3. 点活动栏 Conan 图标

若仍找不到 CLI，在设置里把 `conanCli.binary` 指到绝对路径。本机 Conan 2 仍需单独安装，插件只内置 `conan-cli` 封装层。

控制台页面：

- 概览：项目、平台、Qt、编译器、最近分析
- 依赖分析：配方对齐 + 选定平台制品
- 拉取依赖：选目标平台后执行 conan install（只取仓库二进制）
- 仓库：查看远程已有组件和已发布版本
- 发布：改包名/版本后确认，先写入 `conanfile.py` 再 create + upload
- 设置：全局 Nexus 登录（本机），项目 Qt/编译器/平台
- 诊断：Conan、配置、仓库、平台

侧边栏是同一套命令的窄入口。密码只在设置页录入，通过 `CONAN_PASSWORD` 传给 CLI。
