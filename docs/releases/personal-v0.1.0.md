# CodexRunway 0.1.0 (1000) · Klain97710 定制版

基于 [Licoy/CodexRunway](https://github.com/Licoy/CodexRunway) 的亲友使用版，保留 AGPL-3.0 许可和原作者署名。本版用于覆盖原版，沿用已有 Codex 账号、偏好、Keychain 和 Widget 标识。

- 只启用 Codex；Grok 实现与测试保留，正式版不读取或请求 Grok。旧 Grok / Both 设置转为 Codex。
- 保留多账号、配额、官方重置券、额度估算、成本、Token 图表、会话索引修复及 Widget。
- 重置动态默认每 5 分钟查询公开状态，保留提醒和来源；移除“求重置 / 感谢”、互动计数及访客标识读写。已有关闭设置保留。
- 独立 Sparkle 更新源及签名，用户确认安装；失败保留当前应用。开发包禁用在线更新。
- 保留刷新并发、失败提示、旧数据时间戳与脱敏网络诊断改进。

## 安装与回退

主应用需要 macOS 12+，Widget 需要 macOS 14+。Apple Silicon 下载 arm64 DMG，Intel 下载 x86_64 DMG。首次从原版切换请手动安装：退出原版 → 备份应用和偏好 → 覆盖相同安装位置 → 启动验证。Homebrew 用户先卸载 cask，**不要使用 `--zap`**，再安装本版。后续只使用本版更新渠道。

下载提供 SHA256SUMS。本版为免费 ad-hoc 签名、未经过 Apple 公证，首次启动可能需要在系统“隐私与安全性”允许。回退时退出并恢复备份应用，保留账号和会话数据。

详见[安装、首次登录与回退说明](https://github.com/Klain97710/CodexRunway/blob/personal/main/docs/development/install-upgrade.md)及[验收记录](https://github.com/Klain97710/CodexRunway/blob/personal/main/docs/development/personal-v0.1.0-verification.md)。Intel 提供交叉构建与签名校验结果，尚无 Intel 实机启动结论；macOS 12/13 实机尚未覆盖。

## English

Klain97710 custom edition, based on Licoy/CodexRunway under AGPL-3.0. Codex only, with existing account data, preferences and Widget identity preserved. Public reset status and alerts remain; reactions and visitor identifiers are removed.

Install manually once to switch from upstream: quit, back up the app and preferences, replace at the same path, then launch. Homebrew users should uninstall the original cask **without `--zap`** first. Future updates use this repository's signed Sparkle feed and require confirmation. Failures preserve the installed app.

macOS 12+; Widgets require 14+. Choose arm64 for Apple Silicon or x86_64 for Intel. Packages are ad-hoc signed and not notarized. Verify SHA256SUMS. Intel hardware and macOS 12/13 launches have not been tested; see the linked verification record for the precise coverage.
