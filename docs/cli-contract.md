# CLI JSON contract

VS Code and other clients should invoke commands with `--json` and parse one JSON object from stdout.

Successful response:

```json
{
  "ok": true,
  "action": "install",
  "data": {
    "os": "kylin",
    "arch": "x64",
    "remote": "nexus",
    "output_folder": "conan",
    "missing_binary_policy": "download-only"
  }
}
```

Failed response:

```json
{
  "ok": false,
  "action": "install",
  "error": "conan install ...: missing binary",
  "exit_code": 6,
  "data": {
    "os": "kylin",
    "arch": "x64",
    "hint": "仓库中没有匹配该平台的二进制。请检查 Nexus 或改操作系统/架构，不要在本机编译。"
  }
}
```

Primary commands for the GUI:

| Action | Command |
|--------|---------|
| 工作台刷新 | `status` |
| 初始化 | `init` |
| 扫描 | `scan` 返回本机参考（`qt_installs`），不写入项目。目标 os/arch/compiler/qt 由 `settings set` 手填 |
| 仓库组件 | `catalog [package]` 列出远程已有包和版本 |
| 拉取依赖 | `install --os --arch --build-type Release` 执行 `conan install . --build=never` |
| 依赖分析 | `analyze --os kylin --arch x64` |
| 下载 | `install --os kylin --arch x64` |
| 项目设置 | `settings set --qt 6.8 --compiler gcc --os kylin --arch x64` |
| 全局设置 | `config set --name nexus --url URL --username USER` |
| 登录 | `CONAN_PASSWORD=... config login` |
| 测试仓库 | `config test` |
| 发布预览 | `publish --dry-run --name qtutils --version 1.0 --os kylin --arch x64`（不改配方） |
| 发布 | `publish --name qtutils --version 1.0 --channel dev --os kylin --arch x64`（先把 name/version 写入 `conanfile.py`，再 `export-pkg` 打包本机已编译库并 upload；channel 保存在项目设置并随预览/结果返回，当前上传引用为 `name/version`） |
| 诊断 | `doctor` |

Passwords travel in `CONAN_PASSWORD` and are written to `~/.conan-cli/credentials` (mode 0600), never as CLI arguments.
