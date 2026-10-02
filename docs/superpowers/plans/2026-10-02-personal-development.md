# CodexRunway 个人二开启动计划

> **执行约定：** 在主线程顺序执行，遵循仓库 `AGENTS.md`。本计划先建立个人仓库和开发基线；具体功能确定后再编写对应的实现方案。

**Goal:** 基于 CodexRunway 开展个人非商业自用增强，保留原生 macOS 体验，并让改动可验证、可回退、可持续同步上游。

**Architecture:** 沿用 SwiftPM 主工程与现有 Widget 扩展。核心逻辑位于 `Sources/CodexRunwayCore`，界面位于 `Sources/CodexRunway`，桌面组件位于 `Sources/CodexRunwayWidget`；通过已有服务和可注入的存储路径保持边界。

**Tech Stack:** Swift 6、AppKit、SwiftUI、Foundation、WidgetKit、SQLite；更新组件 Sparkle 2.9.3。

## 1. 仓库与基线

| 项目 | 约定 |
| --- | --- |
| 个人 fork | https://github.com/Klain97710/CodexRunway |
| 上游 | https://github.com/Licoy/CodexRunway |
| 起始版本 | v0.0.85 |
| 起始提交 | `b689c66aa0a9490eb69cc01b6fd7abcf1d337347` |
| 基线标签 | `personal/baseline-v0.0.85` |
| 本轮分支 | `personal/development-plan` |
| origin | 个人 fork，默认推送目标 |
| upstream | 原作者仓库，用于获取后续更新 |

本轮已选择正式 fork，保留上游历史、作者信息和 AGPL-3.0 许可。当前用途是个人自用；对外分发或提供修改版网络服务时，遵循相应的源码提供义务。

`main` 保持上游基线；个人准备与功能在独立分支上进行。发布工作流匹配 `v*` 标签，个人基线标签采用 `personal/` 前缀。

## 2. 当前范围与文件边界

本轮交付为 GitHub fork、远端配置、基线引用、计划文档和本地验证记录。首个具体功能尚未选定。

本轮新增：

- `docs/superpowers/plans/2026-10-02-personal-development.md`：开发路线与验收要求。
- `docs/development/baseline-2026-10-02.md`：本地工具链、验证命令、结果与限制。

后续按需涉及的代码入口：

| 方向 | 主要入口 |
| --- | --- |
| 菜单栏展示与快捷操作 | `StatusControllerStatusBar.swift`、`StatusBarContentView.swift`、`StatusControllerMenu.swift` |
| 主面板布局 | `RunwayPopoverView.swift`、`RunwayMainPanelSections.swift`、`MainPanelLayout.swift` |
| 偏好设置 | `RunwaySettings.swift`、Core 中的 `PreferencesStore.swift` |
| 账号管理与切换 | Core 中的 `AccountStore.swift`、`AccountSwitcher.swift`、`AccountQuotaRefresher.swift` |
| 用量统计 | Core 中的 `UsageCostRepository.swift` 及其索引、汇总模块 |
| 数据展示与状态 | `RunwayModel.swift`、`RunwayModelGrok.swift`；只拆分当前改动需要的职责 |
| 假数据截图 | `MainPanelMockRender.swift`、Core 中的 `DevPreviewFixtures.swift` |
| Widget | `Sources/CodexRunwayWidget` 与现有 `WidgetExtension` 工程 |

除标注 Core / Widget 的项外，上表文件位于 `Sources/CodexRunway`。新增文案通过 `L10n`，至少同时补英文和简体中文。

## 3. 阶段一：建立开发基线

- [x] 确认上游版本、许可、贡献约定和代码结构。
- [x] 在个人 GitHub 账号创建公开 fork。
- [x] 配置 origin / upstream，设置个人仓库为默认推送目标。
- [x] 补齐本地 Git 历史，创建准备分支与基线标签。
- [x] 在本地执行 `swift test` 和 `swift build`，记录结果与工具链版本：主程序构建通过，测试受 Swift 6.1 编译兼容性影响，详见基线报告。
- [x] 将计划与基线报告保存到本分支；交付时核对提交、远端分支与个人基线标签。

开发命令在仓库根目录执行：

```bash
swift test
swift build
```

两项检查依次执行，共用 SwiftPM 构建缓存。它们不会替代桌面启动与 Widget 打包验证。出现未修改原版即可复现的失败时，先记录基线问题，后续修复使用独立提交。

本轮识别到的优先准备项是对齐测试工具链：上游同一提交在 Swift 6.3.3 下通过 777 个测试，本机 Swift 6.1.2 在三个 Grok 测试替身的并发捕获处编译失败。先对齐并复验，再确定是否需要独立的测试兼容性修复。

**验收：** 个人远端和上游远端明确；代码历史完整；本地检查有可复现记录；准备文档能在个人仓库中查看。

## 4. 阶段二：准备可控的界面验证环境

默认先使用项目现有的假数据截图路径，适合验证布局、主题、文案和信息层级。它为 Codex 账号和缓存注入临时存储，并使用服务替身，无需导入真实账号。

在构建通过后，可将截图输出到被 Git 忽略的 `.build` 目录：

```bash
swift run CodexRunway --render-main-panel-mock=all .build/personal-preview
```

- [ ] 生成主面板和详情页的深浅色基线截图。
- [ ] 记录中英文布局、长文本、空状态和窗口尺寸的验证范围。
- [ ] 对需要真实交互的改动，先确定独立测试账户或完整路径隔离方案，再运行桌面应用。

### 当前运行隔离的实际边界

- `swift run CodexRunway` 在 macOS 14+ 会包装并注册 Dev app 与 Widget，不只是打开一个纯预览窗口。
- Dev bundle 标识独立，但默认账号库仍位于当前系统用户的 `.codex-runway/accounts`，官方凭据仍指向 `.codex/auth.json`；单实例锁和本地 Widget 快照也沿用默认目录。
- `RunwayModel.bootstrapAccounts()` 会调用 `AccountStore.syncFromOfficialAuth()`。官方凭据缺失、损坏或不可用时，有可用托管副本便可能触发恢复写入。
- 开发启动脚本会处理开发版进程和组件注册；不能只凭 Dev 名称判断它是只读运行。

首次完整桌面联调优先使用独立 macOS 标准测试账户和测试凭据。若长期在日常账户开发，再单独设计覆盖账号库、官方认证目标、缓存、Widget 快照、锁、后台任务与更新入口的开发隔离机制。保持系统级 HOME 和当前 Codex 会话的 CODEX_HOME 不变。

**验收：** 视觉改动可通过假数据复现；真实账号操作的测试路径与日常数据明确分离。

## 5. 阶段三：确定第一个自用增强

当前还没有承诺具体功能或界面方案。下一次功能设计需要明确：

1. 一个具体问题：当前操作是什么，哪里不方便。
2. 希望的行为：改完后怎样操作、展示什么信息。
3. 受影响范围：Codex / Grok、菜单栏 / 主面板 / 设置 / Widget。
4. 2–3 条可观察的验收标准。

可供选择的首期方向包括菜单栏信息简化、主面板模块布局、账号快捷操作或用量展示增强。确定一个方向后，再定位到上表的代码入口。

- [ ] 为首个功能写出简短设计和验收标准。
- [ ] 从准备分支创建独立 feature 分支。
- [ ] 有行为变化时增加相应 Core / UI 测试；纯视觉调整使用现有测试和截图验证。
- [ ] 单项功能通过后再扩大范围，保留易于同步上游的小提交。

**验收：** 每个功能都能说明改动前后的行为差别，有对应证据，并可独立回退。

## 6. 上游同步与回退

- 在工作区干净时 fetch 上游；有未提交工作则先保存，不通过强制重置丢弃它。
- 在集成分支合并上游更新，处理冲突并执行受影响范围的检查后，再纳入个人开发分支。
- `main` 只在确认可快进时更新；不会因日常开发而承载临时改动。
- 功能回退使用独立 revert 或从基线创建新分支；代码回退不等同于恢复账号、凭据和会话数据。
- 若开始使用真实测试账号，备份放在源码仓库外，限制本地访问，不提交凭据、会话原文或真实账号截图。

## 7. 个人安装包的后续要求

本轮不创建 Release。开始分发或替代日常安装之前，需要落实：

- 独立的应用 / Widget bundle 标识、数据边界和安装位置。
- 个人更新源、Sparkle 签名配置及对应验证。当前配置和发布脚本仍含上游地址，应逐项检查。
- 通过现有打包和校验脚本验证目标架构；如更改打包或更新，遵守 `CONTRIBUTORS.md` 的额外检查要求。
- 保留署名、许可和源码提供方式；准确说明 ad-hoc 签名与未公证状态。

**验收：** 用户能区分个人版与上游版；更新不会把个人版意外替换为上游产物；安装和回退均有明确路径。

## 8. 首期完成标准

- [ ] 个人仓库、计划分支与验证报告已可访问。
- [ ] 本地构建 / 测试通过，或已记录明确的兼容性阻碍及解决方向。
- [ ] 首个改动的范围与验收标准已确定。
- [ ] 使用适合该改动的假数据或隔离运行环境。
- [ ] 对应回归验证通过，修改以独立提交保存。

推荐推进顺序：**个人 fork 与基线 → 假数据预览 → 首个功能设计 → 小步实现 → 原生联调 → 个人安装包**。
