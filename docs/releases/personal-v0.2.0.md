# CodexRunway 0.2.0 · Klain97710 定制版迁移候选

本草稿基于 Quosparr v1.6.2（同步至 `21947ff`） 的 Go + mygo/ui 架构，当前仍需隔离桌面、旧 Widget 刷新、跨代安装更新、Intel 与最低 macOS 版本验收。**尚不建议替换日常 Swift 版；验收前不公开发布。**

- 保留 CodexRunway 应用标识、账号与偏好路径、Keychain 服务和 Widget 标识。
- 新增 Codex 桌面悬浮窗、速率分析、主题颜色与显示字体、精准/相对重置时间、自定义代理测试站点。
- 新增 Codex Skill、MCP 与本机会话管理；跨服务扫描、切换和 Skill 转移入口在定制版中关闭。启停/删除操作保留备份、确认和回收站流程。
- 仅启用 Codex；Grok 与 Claude 保留源码但不初始化、读取凭据或请求网络；保留重置公开状态与提醒，移除互动请求。
- 菜单栏使用剩余额度三列；Pro 隐藏标准 5h；右键菜单保留操作项。
- 旧 Swift 版使用原个人 Sparkle 签名链；Go 版使用独立 mygo 签名与个人更新源。
- 产品版本 0.2.0，构建号 2004，Go 主程序支持 macOS 13+，Widget 要求 14+；macOS 12 请继续使用 Swift 稳定版。提供 arm64/x86_64 隔离验收候选。

首次隔离覆盖前，退出旧版并整组备份应用、偏好和数据。回退时恢复匹配的 Swift 应用和合成数据快照；不要用过期 OAuth 备份覆盖已旋转的真实令牌。本机已安装的 2004 与完整迁移验收状态见[迁移验证报告](../development/quosparr-migration-validation.md)；2004 源码同步与自动化结果见[同步记录](../development/upstream-sync-2026-10-19.md)。

This is an unaccepted migration candidate, not a daily-use replacement. Keep the Swift application until isolated upgrade, rollback, Widget and required platform checks pass. MIT license and original Licoy attribution are retained.
