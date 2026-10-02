# 本机启动与打包

验证日期：2026-10-02。当前优先目标是本机开发启动、生成安装包，并从安装包启动应用。

## 已验证的环境与结果

- macOS 15.7.3，Apple Silicon arm64。
- Xcode 16.4，Swift 6.1.2，macOS SDK 15.5。
- 应用版本 v0.0.85，源码基线 `personal/baseline-v0.0.85`。
- 开发分支：`personal/local-launch`，位于个人 fork `Klain97710/CodexRunway`。

本轮沿用上游应用、测试和打包脚本，没有修改业务代码、依赖或工具链。

| 验证项 | 结果 |
| --- | --- |
| Debug 主程序构建 | 通过 |
| `swift run CodexRunway` | 通过，生成并启动 Dev app，包含 Widget |
| arm64 Release 打包 | 通过，生成 `.app`、`.dmg`、`.zip`、`.app.tar.gz` |
| 主程序 / Widget 信息与签名校验 | 通过 |
| `hdiutil verify` | DMG 校验通过 |
| `dist/CodexRunway.app` 启动 | 通过，打开真实主面板 |
| 从 DMG 复制并启动 | 通过，安装到 `~/Applications/CodexRunway.app` |
| 开发版 / 打包版界面 | 实际打开主面板；开发版另做了截图目视检查 |
| 本地 `swift test` | 测试目标编译失败；详见基线报告中的三个 Swift 6.1 并发错误 |

安装版启动后保持运行；开发版和 `dist` 目录中的验证进程已停止。安装版首次运行配置为手动启动，未启用开机自启。

## 开发启动

先退出正在运行的 CodexRunway，再在仓库根目录执行：

```bash
swift run CodexRunway
```

macOS 14+ 上，该命令会编译主程序、构建 Widget、组装并启动：

```text
.build/codex-runway-widget-dev/CodexRunway-dev.app
```

启动脚本完成后终端命令退出是正常现象，Dev app 会独立运行。后续修改代码后，重新执行上述命令。

应用位于菜单栏，没有 Dock 图标，也不会默认打开主面板。点击菜单栏项目查看面板；需要直接展示窗口时可执行：

```bash
open -a "$PWD/.build/codex-runway-widget-dev/CodexRunway-dev.app" \
  'codex-runway://widget?provider=codex&section=overview'
```

开发版与安装版共享单实例锁。如果提示 `CodexRunway is already running`，先通过应用中的“退出”结束安装版，再运行开发命令。

## 打包与校验

在仓库根目录执行：

```bash
ARCH=arm64 bash Scripts/package-app.sh
bash Scripts/verify-packaged-app.sh dist/CodexRunway.app arm64 local
hdiutil verify dist/CodexRunway-macos-arm64.dmg
```

产物位于被 Git 忽略的 `dist/`：

```text
dist/CodexRunway.app
dist/CodexRunway-macos-arm64.dmg
dist/CodexRunway-macos-arm64.zip
dist/CodexRunway-macos-arm64.app.tar.gz
```

主程序、Sparkle 和 Widget 都包含在 `.app` 中。可直接启动打包产物：

```bash
open "$PWD/dist/CodexRunway.app"
```

本机已从经过校验的 DMG 复制应用到用户应用目录。日常启动使用：

```bash
open "$HOME/Applications/CodexRunway.app"
```

若菜单栏项目暂时不可见，可以直接打开窗口：

```bash
open -a "$HOME/Applications/CodexRunway.app" \
  'codex-runway://widget?provider=codex&section=overview'
```

重新打包只更新 `dist/`，不会更新已安装的副本。更新本机安装时，先退出应用，再从新的 DMG 复制 `.app` 到相同安装位置。

## 当前边界

- 这是用于本机运行的 ad-hoc 签名产物，未做 Developer ID 签名或 Apple 公证。当前只验证了本机 arm64，没有验证 Intel 或其他 macOS 版本。
- Widget 已构建、随包签名并被系统注册；尚未验证手动添加到桌面后的展示与时间线刷新。
- 本地包仍保留上游应用标识，Sparkle 公钥为未配置占位值。本轮没有发布 GitHub Release，也没有启用可安装的自动更新链路；正式维护个人更新源属于后续工作。
- 正常启动会读取当前用户的 Codex 数据，并可将账号凭据导入 `~/.codex-runway/accounts`。Dev app 的独立 bundle ID 不会隔离账号库、会话读取或 Widget 快照。
- 本次启动验证前后，`~/.codex/auth.json` 与 `~/.codex/session_index.jsonl` 的文件指纹一致；没有执行切号、会话修复或新账号登录。真实账号截图和运行数据不进入 Git。
- 联网请求中观察到过 `NSURLErrorDomain -1001` 超时；配额和本机成本已显示，部分订阅推算 / 重置次数请求曾未加载。本次验收确认启动和打包可用，不代表所有远端接口已完成稳定性测试。
- 全量测试仍受本机 Swift 6.1 的测试替身编译兼容性影响，不影响当前主程序与安装包构建。具体位置见 [开发基线记录](baseline-2026-10-02.md)。
