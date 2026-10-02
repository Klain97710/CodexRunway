# 本机启动与打包

验证日期：2026-10-02。当前优先目标是本机开发启动、生成安装包，并从安装包启动应用。

## 已验证的环境与结果

- macOS 15.7.3，Apple Silicon arm64。
- Xcode 16.4，Swift 6.1.2，macOS SDK 15.5。
- 应用版本 v0.0.85，源码基线 `personal/baseline-v0.0.85`。
- 开发分支：`personal/local-launch`，位于个人 fork `Klain97710/CodexRunway`。

首轮沿用上游完成启动与打包；随后修复了本机测试兼容性、慢请求阻塞本地统计、配额与重置次数的失败提示，并增加默认关闭的脱敏网络诊断。依赖、工具链和打包脚本保持现有配置。

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
| 本地 `swift test` | 787 个测试通过，退出码 0 |
| 故障与恢复 | 回归覆盖旧数据保留、原始时间戳、恢复后清错、切号隔离及取消；Dev app 实际验证连接失败提示 |

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

## 本次修复与网络复验

- 三个 Grok URLProtocol 测试替身保留具体的 Sendable 类型，解决 Swift 6.1 编译错误。
- 错误码测试不再意外访问真实 OAuth 服务；布局测试按实际控件尺寸验证容纳和滚动，避免依赖特定系统的像素数值。
- 全量刷新中，订阅额度推算与本地成本 / Token 扫描同时推进；晚到结果继续受账号代次与取消检查约束。
- 配额和重置次数请求失败时，卡片显示具体原因；已有数据保留原成功时间并标明“正在显示上次获取的数据”。首次失败不显示为 0，认证失效仍清除账号相关数据。
- 2026-10-02 使用跟随系统路由记录 70 次真实请求，均返回 HTTP 200，耗时 310–1379 毫秒，覆盖配额、重置次数、每日用量、profile 统计、工作区名称及重置状态。本次未复现先前的超时，因此未增加自动重试或延长超时；该样本不代表长期服务可用性。
- Dev app 临时使用关闭的本机代理端口验证了“刷新失败 / 连接失败，请检查代理地址及网络连接”。验证后已逐值恢复原代理配置，再从新 DMG 更新安装版。

需要排查后续偶发问题时，先退出正在运行的 CodexRunway，再执行：

```bash
swift build
CODEX_RUNWAY_DISABLE_DEV_APP=1 CODEX_RUNWAY_NETWORK_DIAGNOSTICS=1 \
  .build/debug/CodexRunway > /tmp/codexrunway-network.log 2>&1
```

这是临时诊断进程，退出后再正常启动 `.app`。在另一个终端读取白名单诊断行：

```bash
rg '^RunwayNetwork ' /tmp/codexrunway-network.log
```

这些行只包含固定的操作类别、路由模式、尝试次数、耗时、HTTP 状态及已知错误域 / 错误码，不包含完整 URL、请求头、正文、账号标识或凭据。未知操作和错误域统一归为 `other`。默认不输出诊断。

## 当前边界

- 这是用于本机运行的 ad-hoc 签名产物，未做 Developer ID 签名或 Apple 公证。当前只验证了本机 arm64，没有验证 Intel 或其他 macOS 版本。
- Widget 已构建、随包签名并被系统注册；尚未验证手动添加到桌面后的展示与时间线刷新。
- 本地包仍保留上游应用标识，Sparkle 公钥为未配置占位值。本轮没有发布 GitHub Release，也没有启用可安装的自动更新链路；正式维护个人更新源属于后续工作。
- 正常启动会读取当前用户的 Codex 数据，并可将账号凭据导入 `~/.codex-runway/accounts`。Dev app 的独立 bundle ID 不会隔离账号库、会话读取或 Widget 快照。
- 首轮启动验收中两个官方文件的指纹一致。本次修复复验中，`~/.codex/session_index.jsonl` 仍一致，但 `~/.codex/auth.json` 在运行期间有更新，不能声称凭据文件未变化，也不能仅凭指纹认定写入来源。未执行切号、会话修复、新账号登录或凭据恢复；保留当前登录状态。真实账号截图和运行数据不进入 Git。
- 首次启动时若配额也不可达，本地全量统计仍需要先取得周期信息；本次解除了订阅推算对后续扫描的阻塞，没有引入无配额元数据的首屏离线推导。

## 方案与执行记录

原始失败记录保留在 [开发基线](baseline-2026-10-02.md)。本次执行范围与后续条件见：

1. [Swift 6.1 测试兼容性](../superpowers/plans/2026-10-02-swift61-test-compatibility.md)：已落地并完成全量回归。
2. [联网异常处理](../superpowers/plans/2026-10-02-network-reliability.md)：诊断、独立刷新和失败状态已落地；有限重试保留为有复现证据后的后续方案。
