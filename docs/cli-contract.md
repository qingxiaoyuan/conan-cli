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
| 项目设置 | `settings set --qt 6.8 --compiler gcc --os kylin --arch x64 [--lib-dir DIR] [--include-dir DIR] [--workspace GLOB]`（`--workspace` 可重复、支持逗号分隔，传空值恢复默认 `packages/*` 与 `src/*`） |
| 全局设置 | `config set --name nexus --url URL --username USER` |
| 登录 | `CONAN_PASSWORD=... config login` |
| 测试仓库 | `config test` |
| 发布预览 | `publish --dry-run --name qtutils --version 1.0 --os kylin --arch x64`（不改配方） |
| 发布 | `publish [--package NAME] --name qtutils --version 1.0 --channel dev --os kylin --arch x64 [--lib-dir DIR] [--include-dir DIR] [--replace]`（写入 `.conan-cli/recipes/<包名>/conanfile.py`，不覆盖仓库根消费配方；再 `export-pkg` 该目录。多组件必须 `--package` 或 `--all`。产物目录相对项目根；预览含 `data.package` / `data.recipe_dir` / `data.lib_dirs`；版本与组件原版本不同时预览含 `data.previous_version`） |
| 全量发布 | `publish --all [--dry-run] [--replace]` 逐一发布全部组件（workspaces ∪ packages[]）。单包失败不中断；`data.results[]` 每项 `{package, ok, reference, error, recipe_action, replaced, replace_warning}`；有失败时 `ok:false`、exit_code 非 0。`--all` 与 `--package` 互斥 |
| 替换旧版本 | `publish --replace`：上传成功后删除该组件之前的版本（`conan remove name/<旧版本> -r <remote> --force`）。旧版本取自组件记录/配方（`data.previous_version`）；删除失败不影响发布结果，以 `data.replace_warning` 提示。无旧版本或版本未变时不执行 |
| 组件列表 | `packages list` 合并视图：`data.packages[]` 每项 `{name, version, source: "workspace"\|"declared", dir, lib_dirs, include_dirs, no_qt, has_artifacts, has_recipe}`。`packages/*` 与 `src/*`（可用 project.yaml 的 `workspaces` glob 覆盖）下含 conanfile.py / dist/ / lib/ 的目录自动成为组件；同名时 `packages[]` 优先。`status` 的 `data.packages` 同构。workspace 组件发布时 `recipe_dir` 指向其目录，配方就地补丁（`recipe_action` 为 `patch`/`update`），不写 `.conan-cli/recipes/` |
| 诊断 | `doctor` |

Passwords travel in `CONAN_PASSWORD` and are written to `~/.conan-cli/credentials` (mode 0600), never as CLI arguments.
