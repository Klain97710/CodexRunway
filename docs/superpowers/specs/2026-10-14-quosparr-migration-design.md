# Quosparr Go 迁移验证设计

日期：2026-10-14（Asia/Shanghai）。状态：计划已形成，尚未实施；本文件不表示 Go 版本已通过验收。

## 目标与边界

在独立分支验证上游 Quosparr v1.0.3 的 Go 实现能否替代当前亲友版。沿用已确定的覆盖安装、Codex-only、自有更新源、旧数据原地使用约定；验证完成前，日常安装和 `personal/main` 的应用实现保持 Swift 版。

首轮交付仍为 macOS 12+ 的 arm64、x86_64 应用，Widget 为 macOS 14+。保留上游 Windows/Linux 源码，但不承诺安装包和平台验收。不同时升级 mygo、不重做界面、不启用 Grok、不恢复重置互动、不增加 Homebrew 或公证。

## 固定比较对象

| 对象 | 固定值 |
| --- | --- |
| 当前稳定代码 | `895e991d67eea3f8500065b605741f42aee54ae3`，上游基线 v0.0.87 |
| 目标上游 | `94ce1869d2603593972846baa811672c1c2e903c`，v1.0.3 |
| Swift 回退基线 | 当前已验证本机包及不可变的 `personal-v0.1.0` 发布包，分别记录哈希 |
| 迁移分支 | `personal/migrate-quosparr-v1`，从执行时的 `personal/main` 创建独立 worktree |
| 保留分支 | `personal/legacy-swift`，初始指向稳定代码 SHA |
| 工具链 | 上游固定 Go 1.27.1、mygo v0.4.8；Swift 仅构建 Widget 和桥接 |

本次检查发现上游新增 30 个提交，涉及 1059 个文件。Swift 主程序、SwiftPM 清单和原测试已被删除，不能直接重放旧 Swift 补丁。保持上游 Go 模块路径，避免没有必要的批量 import 改名。

## 路线比较与选择

| 路线 | 收益 | 成本与决定 |
| --- | --- | --- |
| Go 实现保留 CodexRunway 身份 | 继续覆盖原版，沿用账号、设置、Widget 与旧更新入口 | 需适配启动锁、路径和两套更新器；采用此路线，符合已有决定 |
| 完全跟随 Quosparr 更名 | 与上游打包和迁移逻辑接近 | 新身份、新目录，需手动安装、重加 Widget；不符合当前覆盖替换约定 |
| 留在 Swift，仅移植小修复 | 短期改动小 | 逐渐脱离上游主实现；作为验证未通过时的稳定维护路线 |

## 必须保持的产品契约

| 项目 | 正式候选的值或行为 |
| --- | --- |
| 名称、主应用标识 | `CodexRunway.app`、`com.github.codex-runway` |
| Widget、App Group | `com.github.codex-runway.widget`、`group.com.github.codex-runway` |
| Widget kinds | `com.github.codex-runway.widget.{overview,token-trend,metric,reset-today}` |
| URL scheme | `codex-runway://`；旧 grok/both 参数可解码并归一化为 Codex |
| 数据、偏好、代理凭据 | `~/.codex-runway`、`com.github.codex-runway`、`com.github.codex-runway.network-proxy` |
| 单实例锁 | `~/.codex-runway/codex-runway.lock`；Swift 与 Go 相互排斥 |
| 发布仓库 | `Klain97710/CodexRunway`；保留原作者署名、来源和许可 |
| 菜单栏 | `5h → 进度条 → 百分比`、`7d → 进度条 → 百分比`；延续当前“剩余”口径 |
| Pro | Pro 100/200/500 的状态栏不显示标准 5h 窗口；详情原数据和可选模型专属额度不裁剪 |
| 右键菜单 | 只含操作，无灰色详情块；保留当前入口或明确映射到等效入口 |
| 运行范围 | Grok 不构造、不扫描、不刷新、不发请求；保留其源码、旧文件及显式启用的底层测试 |
| 重置动态 | 默认开启、5 分钟刷新、状态/提醒/来源/Widget 保留；既有关闭选择保留；零互动请求、零访客 Cookie 读写 |
| 更新 | 自动检查遵循用户设置，下载/安装须确认；开发包禁用；错误不切换到上游源或绕过代理 |

新版账号隐私开关保留，七语言文案保持完整。现有刷新并发、失败反馈、旧数据时间戳和脱敏诊断都需要对应的 Go 验证证据。

## 实现边界

1. 保留 upstream 的 `internal/app`、`internal/runway`、`internal/store`、`internal/netx`、业务模块和 Widget 分层。在小型 `internal/personal` 包集中产品常量和可注入功能配置，不增加插件框架或第三方依赖。
2. `mygo.json` 是版本号的唯一来源。应用身份、发布配置与 Swift Widget 常量由契约检查保证一致，避免静默漂移；不为此引入运行时跨语言配置系统。
3. 正式候选使用旧数据路径，跳过 Quosparr 的“从旧目录复制到新目录”迁移。必须防止源目录与目标目录相同的复制，以及 legacy/current 两次获取同一文件锁。
4. 保留旧序列化枚举、账号 ID、偏好 JSON、数据库与快照兼容。以稳定版合成样本做双向读取和字段级比较，不能因新版能启动就认定数据兼容。
5. 新 Go 应用采用 mygo 更新器。旧 Swift 客户端继续通过现有 Sparkle appcast 获取同身份 Go 包；两条链使用个人仓库，分别验签。沿用已有 Sparkle 密钥，新增独立 mygo 密钥。旧包启用了 `SURequireSignedFeed` 和解包前验签，而上游 Go 生成器仅签 ZIP；过渡发布保留固定 Sparkle 2.9.3 官方工具给 XML 签名和交叉验签，工具不嵌入 Go 应用。
6. 上游正式发布会自动公开，必须改回个人标签、先生成草稿、验收后公开。旧 `appcast-arm64.xml` / `appcast-x86_64.xml` 应包含同身份候选的签名 ZIP，不能沿用上游仅提示手动更名的内容。

## 隔离和验收模型

- 单元测试：显式传入临时目录、内存偏好、内存凭据、假时钟及模拟网络。开发机不运行未完成隔离的 Go 主程序。
- 桌面冒烟：独立测试身份和独立偏好域、合成账号；与正式数据及 Keychain 无连接。上游 `Scripts/run-dev-app.sh` 默认访问真实环境，禁止原样作为验收入口。
- 同身份覆盖和更新：一次性 macOS VM/专用测试用户，真实旧包保持原字节和原签名。系统 Keychain、登录项、Widget 和测试 CA 都限制在该环境；仅修改 HOME 或偏好域不视为完整隔离。
- 日常机器：全部相关门槛通过后，才备份并替换；真实账号仅用于正常登录/配额冒烟，不用于签名、网络、迁移中断或损坏数据测试。

版本候选从个人 `0.2.0`、构建号 `2000` 起，后续候选重打递增构建号；生产上线用最终验收过的原件。候选先用 Actions artifacts，不发布到 production latest。公开发布标签为 `personal-v0.2.0`；mygo 的 tagPrefix 也必须为 `personal-v`；`personal-v0.1.0` 永不改写。Go→Go 更新演练使用独立的更高产品版本（例如测试用 `0.2.1`），不能假设仅增 CFBundleVersion 就会触发 mygo 更新。

## 通过条件与退出策略

顺序：冻结基线 → 安全构建入口 → 身份/数据 → 功能裁剪 → UI/Widget → 自动回归/打包 → 隔离更新/回退 → 候选验收。任何数据丢失、官方凭据意外改写、Grok/互动意外调用、更新源越界、验签失败替换应用，均停止推进。

macOS 12/13 启动、macOS 14+ Widget、Intel 真机/Intel macOS 环境需要单独证据，arm64 上交叉构建不能代替。缺少环境时报告“未验证”，不能宣称支持已验收；完整公开分发在这些门槛完成前不推进。

失败时日常安装保持 Swift。测试中回退应恢复应用及与其配套的数据/偏好快照，不能仅替换二进制后假设 Go 写过的数据仍兼容。Keychain/官方 token 有刷新时不能盲目覆盖为过期副本，应验证凭据可用性，必要时重新登录。

## 来源与执行文档

- 上游 [v1.0.3](https://github.com/Licoy/Quosparr/releases/tag/v1.0.3)；以下源码均以固定 SHA 为准，不追踪浮动 main。
- 上游 [启动顺序](https://github.com/Licoy/Quosparr/blob/94ce1869d2603593972846baa811672c1c2e903c/main.go)、[发布链路](https://github.com/Licoy/Quosparr/blob/94ce1869d2603593972846baa811672c1c2e903c/docs/v1-release.md)、[更名迁移说明](https://github.com/Licoy/Quosparr/blob/94ce1869d2603593972846baa811672c1c2e903c/docs/upgrade-from-codexrunway_ZH.md)。
- 详细任务、命令、门槛及证据：[迁移验证实施计划](../plans/2026-10-14-quosparr-migration-validation.md)。
