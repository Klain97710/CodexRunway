> 本机安装记录（2026-10-14）：用户随后明确要求“替换本机应用”，已备份后覆盖 Go 0.2.0/2002，并检查进程、账号/偏好保留和新配额快照。此授权仅提前执行本机正常安装，不将尚未执行的隔离更新、Widget、Intel 或回退门槛记为通过。

> 2026-10-14 执行修订（用户选择 1）：Go 版最低 macOS 13；macOS 12 保留 Swift 稳定线。Go 1.27 原生最低系统为 13，不能仅改 plist 继续宣称 12+。首个诊断包 build 2000 已作废，build 2001 已通过双架构校验；最终冻结源码包使用 build 2002。以下原计划中的 macOS 12 要求改为验证旧 Swift 留存和拒绝不兼容更新；Go 最低系统实测改为 macOS 13。

# Quosparr 迁移分支验证 Implementation Plan

> **For agentic workers:** 在主线程顺序执行，遵循用户 AGENTS.md 的工具映射，不派发子代理。下列复选框记录实际结果；完成计划不等于已执行迁移。

**Goal:** 在独立分支验证 Go 版能够完整接替当前 CodexRunway 亲友版，并形成可回退、可审查的 macOS 候选包。

**Architecture:** 从稳定定制分支创建迁移 worktree，合并固定的上游 v1.0.3，保留 Go 架构并逐项重做定制适配。正式候选保留旧应用身份、数据目录及 Widget 契约；Swift 稳定版保留在独立基线中。先隔离验证，再考虑替换日常安装和合入集成分支。

**Tech Stack:** Go 1.27.1、mygo v0.4.8、purego 与上游锁定依赖；Swift WidgetKit/桥接；GitHub Actions；旧客户端 Sparkle → Go 包，后续 mygo 更新。

**设计依据：** [迁移验证设计](../specs/2026-10-14-quosparr-migration-design.md)。计划已获用户授权执行。源码适配、自动化和本地候选构建已完成，远程 CI 全部通过，隔离桌面验收按下表继续；已按用户随后明确要求完成本机候选覆盖，完整隔离验收仍待执行。

## 执行状态（2026-10-14）

| Task | 当前状态 | 证据 / 剩余 |
| --- | --- | --- |
| 1 | 完成 | 隔离 worktree、固定合并、双份 Swift 备份；稳定安装未改 |
| 2 | 源码完成，桌面未验收 | 隔离与禁用更新测试、双架构开发包 CI 通过 |
| 3 | 源码和合成数据测试完成 | 固定 Swift Core 双向读回、SQLite 复用通过；真实 Keychain/桌面锁待测 |
| 4 | 源码和运行层测试完成 | Codex 正向控制、Grok/互动零调用通过；桌面待测 |
| 5 | 源码、离屏布局与契约测试完成 | 三列/Pro/动作菜单已恢复；旧 Widget 实际刷新待测 |
| 6 | 源码、离线信任与草稿策略通过 | 两代签名及个人下载限制通过；真实安装链待测 |
| 7 | 自动化和打包完成 | 2002 冻结源码 `6ebc6a0`，双架构包、全部本地及 CI 检查通过 |
| 8 | NOT RUN | 已提供隔离代理/执行手册；缺一次性 macOS 测试环境 |
| 9 | NOT RUN | 缺桌面、最低系统、Intel、Widget 和真实回退证据 |

复选框仅勾选完整执行的项目；包含真实桌面/系统验收的组合项保持未完成。详见[基线与门槛](../../development/quosparr-migration-baseline.md)和[隔离更新手册](../../development/personal-migration-update-validation.md)。

---

## Chunk 1：冻结基线与隔离构建

### Task 1：创建可比较、可回退的分支

**涉及文件：** 新增 `docs/development/quosparr-migration-baseline.md`、`docs/development/quosparr-migration-validation.md`；修改迁移分支 `AGENTS.md`；不改稳定分支应用源码。

- [x] 检查工作区和远程分支，记录稳定代码 `895e991d67eea3f8500065b605741f42aee54ae3`、上游 `94ce1869d2603593972846baa811672c1c2e903c`。当前工具链已读到 Go 1.27.1；执行时重新核对，不自动升级框架。
- [x] 保存当前实际安装包和 `personal-v0.1.0` 原始发布包的独立备份、SHA256、签名、架构和版本清单。当前本机包包含后续定制，不能用最早的发布包冒充其等价回退包。
- [x] 创建 `personal/legacy-swift` 指向稳定代码；从执行时 `personal/main` 创建 `personal/migrate-quosparr-v1` 和独立 worktree。已有同名分支先核对，不强制覆盖。文档提交可以在稳定代码之后，应用代码基线仍固定上述 SHA。
- [x] 在迁移 worktree 合并固定上游，采用正常合并记录祖先关系；逐个解决修改/删除冲突，采用 Go 结构，不将旧 Swift UI 文件机械恢复到新架构。旧定制提交用作行为规格，不整体 cherry-pick。
- [x] 更新迁移分支 `AGENTS.md`：Go 分层、固定依赖、Codex-only、旧产品身份、隔离运行、草稿发布；保留隐私和数据保护规则。稳定分支继续遵循原 Swift 约定。
- [x] 首次推送前移除上游自动公开步骤，限制发布标签为 `personal-v*`，分支 push 只跑验证。此阶段禁用发布 job，不接入私钥。
- [x] 提交 `chore: establish isolated Quosparr migration baseline`。

**门槛 G0：** 稳定应用未替换；工作目录隔离；固定上游可追溯；迁移分支 push 不会发布 Release。`main` 保持上游基线用途，其快进与 `personal/main` 的候选接纳分开处理。

### Task 2：先修好试运行入口

**修改：** `mygo.json`、`main.go`、`Scripts/stage-v1-config.py`、`Scripts/package-v1-app.sh`、`Scripts/run-dev-app.sh`、`internal/app/storeopen_darwin.go`、`internal/app/paths.go`、`.github/workflows/ci.yml`。

**新增：** `Scripts/run-personal-migration.py`、`Scripts/Tests/test-personal-package.py`、`internal/app/personal_isolation_test.go`。

- [x] 用测试先定义两个构建档位：`migration-dev` 与 `replacement-candidate`。前者用 `CodexRunway Migration.app`、`com.github.codex-runway.migration`、独立 scheme/Widget/Keychain/偏好；后者严格使用设计文档中的旧正式身份。
- [x] `migration-dev` 强制禁用在线更新、生产偏好导入、登录项、真实通知和真实凭据访问；只接合成 fixture/内存凭据。身份不同不等于隔离：测试必须证明文件路径、系统凭据服务和后台任务也隔离。
- [x] `run-personal-migration.py` 只接受有测试标记的沙盒目录，记录非敏感配置，拒绝真实账号路径、符号链接逃逸和生产偏好域。退出时只关闭该脚本启动的进程，不清理其他应用数据。
- [ ] 完整主程序先仅在一次性 VM/专用用户运行。开发机只进行显式目录依赖注入的测试和构建；不覆盖 shell 的 HOME、CODEX_HOME 等现有环境变量来模拟安全隔离。
- [x] 不原样执行上游 `Scripts/run-dev-app.sh`；它以正式身份和真实用户环境运行。开发入口应默认使用上述隔离档位，对正式候选运行明确提示实际目标路径。
- [x] 保留 `QUOSPARR_DEV_*` 测试钩子；检查 `QUOSPARR_PREFS_DOMAIN` 仅隔离偏好，不把它当作文件、Keychain 或 Widget 的隔离证明。
- [x] 无 secrets 生成两个架构的开发包，验证开发构建标签和 updater-disabled 行为；发布构建缺少密钥必须失败。
- [x] 提交 `build: isolate personal migration development packages`。

**门槛 G1：** 试运行不会触碰日常账号/偏好/登录项/生产更新源。若 VM 尚不可用，继续纯测试与打包，桌面验收标记未执行。

## Chunk 2：身份、数据与功能适配

### Task 3：恢复旧身份，验证数据原地兼容

**新增：** `internal/personal/edition.go`、`internal/personal/edition_test.go`、`internal/platform/personal_identity_test.go`、`internal/store/personal_compat_test.go`、`internal/app/testdata/personal-swift/`。

**修改：** `main.go`、`mygo.json`、`internal/platform/{paths,brand_migration}.go`、`internal/store/{domain,brand_migration,preferences_codec}.go`、`internal/netx/{credentials,secrets_darwin,brand_migration}.go`、`internal/app/{paths,login_migration}.go`、`Scripts/{stage-v1-config.py,package-v1-app.sh,verify-v1-app.sh}`。

- [ ] 从稳定 Swift 基线生成合成样本：账号索引/导入包、代理引用、完整偏好、缓存、带 WAL 的成本库、额度历史和 Widget 快照。保留所有上游 `testdata/swift_*`，新增亲友版 fixture，禁止用真实账号文件填充 fixture。
- [x] 产品常量集中 `internal/personal`；`mygo.json` 保持唯一版本源。测试比较包内 manifest、运行时常量、Swift Widget 输出，防止名称/标识/仓库分叉。不重命名上游 Go module。
- [ ] 正式配置恢复 `CodexRunway.app`、旧 bundle ID、数据目录、偏好域、Keychain service、URL scheme 和 App Group。保留标识型 ad-hoc designated requirement，实际测试钥匙串授权行为，不能承诺无需系统授权提示。
- [ ] 正式旧目录与目标目录相同，跳过上游更名复制事务；旧 legacy lock 与 current lock 只获取一次，且在任何迁移或写入前持有锁。验证 Swift 先启/Go 先启、退出重启及更新重启均只有一个写入者。
- [x] Go 读取稳定版 fixture，再由 Swift 基线读取 Go 写回的副本；字段级比较，允许的变化只有提供方归一化、已声明新字段默认值和正常派生缓存。保留未知字段和旧枚举，坏数据不得重置为默认并覆盖原件。
- [ ] 验证账号 ID、当前账号、排序、别名、订阅、token 到期语义、数值精度、时区、SQLite/WAL、缓存时间戳。账号数据的存储格式不得通过“重新导入账号”掩盖兼容问题。
- [x] 测试纯安装迁移不写官方 auth；正常 token 刷新允许按原约定写回当前账号；非当前账号只写托管副本，切号/退出登录后的迟到响应不得覆盖当前认证。
- [x] 代理凭据读取旧 service 原始 JSON，拒绝双重 base64 包装或 Keychain 失败后直连；目录 0700、账号/快照文件 0600、原子写入保持。
- [x] 提交 `feat: preserve personal identity and Swift data compatibility`。

**门槛 G2：** 无需重新导入账号；既有偏好可读取且保留；Swift/Go 单实例排斥；合成数据往返兼容；损坏数据有可见错误且原件不变。

### Task 4：在运行入口限制 Grok 和重置互动

**新增：** `internal/personal/features.go`、`internal/app/personal_runtime_test.go`、`internal/runway/personal_features_test.go`。

**修改：** `internal/app/{live,live_grok_accounts,live_grok_usage,live_preferences,settings_view,panel_header,accounts_page,app}.go`、`internal/runway/{service,providers,schedule,tick,widget}.go`、`internal/store/{preferences,preferences_codec}.go`、`internal/resettoday/{client,refresh}.go`、`internal/export/{contract,widget}.go`。

**删除/断开：** `internal/app/panel_reaction.go`、`internal/resettoday/reaction*.go` 的业务实现及其专用测试、`internal/export/reaction.go` 的访客存储调用；先核对引用，保留为旧文件解码所必需的无行为类型。

- [x] 新配置默认只启用 Codex；测试可显式开启 Grok。沿用已有 `Options.Grok` 可选接口，无需新建多提供方框架。
- [x] 在 `StartBackground` 构造 `grok.NewStore`/`NewModule` 之前拦截；初始加载、定时 tick、手动刷新、账号操作、自检、会话/成本扫描和 Widget 需求都不得触发 Grok。仅隐藏控件不算完成。
- [x] 读取旧 Grok/Both 偏好后归一化为 Codex/单服务，其余字段不改。保留枚举解码能力、磁盘旧文件和底层测试；无开启入口。
- [x] 重置动态保留公开 `/api/status.json`、默认 300 秒、提醒、来源链接与 Widget。缺失字段默认开启；已有 false 保持；删除互动按钮、计数、任务、轮询、cookie jar 和访客文件读写。
- [ ] 用可计数的文件/进程/网络替身验证启动、面板打开/关闭、设置变化、超过两次刷新周期、旧深链与 Widget 需求。生产路径 Grok 调用计数、`/api/reaction` 请求数、访客 Cookie 读写均为零；测试同时断言 Codex/公开状态确实有调用，避免以“后台未运行”获得假通过。
- [x] Grok 保留实现用显式全提供方配置继续测试；旧 reaction fixture 无运行引用的不必额外清理用户文件。
- [x] 提交 `feat: enforce Codex-only personal runtime`。

**门槛 G3：** 黑盒行为证实 Grok 与互动零调用，公开状态和 Codex 正常；不能只靠源码搜索、隐藏 UI 或把测试跳过。

### Task 5：恢复菜单栏、右键菜单和 Widget 契约

**修改：** `internal/app/{app,tray_status,tray_render,tray_title,tray_layers_darwin,panel_header,widgets,widgets_native_darwin}.go`、`internal/runway/{meters,statusbar,widget}.go`、`internal/export/{contract,widget_store}.go`、`internal/l10n/tables.go`、`Sources/QuosparrWidget/RunwayWidget{Contract,Intent,Snapshot,Timeline,Types,L10n}.swift`、`Sources/QuosparrWidget/RunwayWidgets.swift`、`Sources/QuosparrWidgetBridge/WidgetBridge.swift`、`WidgetExtension/project.yml` 及生成工程。

**测试：** 扩展 `internal/runway/statusbar_test.go`、`internal/app/{tray_title,tray_layers_darwin,widgets,privacy_display}_test.go`；新增 `internal/app/personal_widget_contract_test.go`。

- [x] 以当前已验收 UI 作为基准：`5h | 进度条 | 百分比`、`7d | 进度条 | 百分比`，百分比和条长均为剩余额度；列对齐、100% 不挤压、单行居中；不把上游整段 caption 当作三列。
- [x] Pro 100/200/500 隐藏标准 5h，非 Pro 按实际窗口显示；未知订阅不误判为 Pro。保留详情原值和用户主动启用的模型专属窗口。覆盖登录切换时的 tier 变化、缺 weekly 数据及空态。
- [x] 上游新右键菜单目前仅有面板/设置/退出。补齐现有定制版的刷新、更新、索引修复、Codex 文件夹及详情窗口等动作；映射到既有 UI 能力，禁止恢复灰色摘要块。独立详情窗口若新框架无等效能力，记录为迁移差异并实现后再判通过，不能静默删除。
- [ ] 复用既有布局/渲染测试，覆盖 1x/2x、深浅色、7 种语言、0/100%、长错误文本和 Pro 单行。菜单行删除不新增只数行数的实现镜像测试；动作分派及副作用保留必要行为测试。
- [x] Widget 除 bundle ID 外，还需恢复四个 kind 字符串、Intent 名称/参数、URL scheme、快照路径和读取 entitlements；源码目录可沿用上游名称，用户保存的系统契约不可随之改名。
- [x] Go 仅发布 Codex 快照；Swift 端旧 grok/both 配置可解码、显示 Codex；使用静态契约测试对比 Go 和 Swift，不依赖已经移除的 Core 包。
- [ ] 独立测试用户先在旧版添加四种 Widget，再覆盖候选，检查保留布局、更新、点击跳转；快照未变化也要跨一个计划刷新周期验证。
- [x] 提交 `feat: restore personal tray and Widget behavior`。

**门槛 G4：** 定制视觉与操作均保留；旧 Widget 无需重新添加且有真实刷新证据；新增账号隐私模式可用。Widget 只注册成功不算完整验收。

## Chunk 3：更新链与自动化门槛

### Task 6：接回个人发布链，保留两代更新信任

**修改：** `mygo.json`、`internal/personal/edition.go`、`internal/app/{update,update_http,update_restart,live,proxy_settings}.go`、`Scripts/{package-v1-app.sh,stage-v1-config.py,legacy-appcast.sh,legacy-install-notice.py,verify-legacy-appcast.py,verify-v1-release.sh,release-checksums.py}`、`Scripts/release-assets/{main,sparkle}.go`、`.github/workflows/release.yml`、`cliff.toml`。

**测试：** `internal/app/{update_http,update_restart,update_development_package}_test.go`、`Scripts/release-assets/sparkle_test.go`、`Scripts/Tests/test-workflow-drafts.py`、新增 `Scripts/Tests/test-personal-release.py`。

**新增发布工具入口：** `Scripts/prepare-personal-sparkle-tools.sh`、`Scripts/sign-personal-appcast.sh`。仅供构建和验收使用，不恢复 Swift 主应用依赖。固定 Sparkle 2.9.3 的 `Sparkle-for-Swift-Package-Manager.zip`，SHA256 为 `3a5d7fd698acc39c122e75764ed3614b472b284cc483f32ae7006d86c513370c`；该值已从稳定版锁定的 Sparkle 依赖自身 `Package.swift` 核对。

- [x] 所有更新/下载/反馈/主页/代理测试入口改为 `Klain97710/CodexRunway`。代理测试用公开仓库地址，不依赖首个新版清单存在。
- [x] 更新 URL 校验覆盖初始 URL、feed/manifest 内资产 URL 和重定向；拒绝上游仓库、相似域、任意仓库、降级 HTTP、URL 内凭据和异常路径。允许 GitHub 正常资产 CDN 重定向，并剥离不应跨主机转发的凭据；不能用只接受 github.com 导致所有资产下载失败。
- [x] 保留旧 Sparkle 公钥/私钥；为 mygo 单独生成并限制权限保存密钥，新增 `MYGO_UPDATE_PRIVATE_KEY` secret。读取框架固定版本签名接口确认编码，不以两者同为 Ed25519 就复用密钥或猜测格式；日志不输出密钥。
- [x] 旧 Swift 客户端继续请求个人 `appcast-arm64.xml` / `appcast-x86_64.xml`，其中 enclosure 指向同身份 CodexRunway ZIP，由旧 Sparkle 密钥签名。删除上游的“旧名称只提供更名下载页”分支对本定制包的影响。
- [x] 为旧包的 `SURequireSignedFeed=true` 补齐 XML feed 签名：上游 `release-assets/sparkle.go` 只有 ZIP 的 `sparkle:edSignature`，不可直接用作最终 feed。采用固定 Sparkle 2.9.3 `sign_update` 对生成后的 XML 签名，再以同版本官方工具验签 XML 与 ZIP；密钥从受限文件/标准输入提供。新增“ZIP 签名正确但 feed 未签/损坏仍被拒绝”用例。XML 签名后不得再改下载 URL、版本或排版。
- [x] 新 Go 包包含原 `SUPublicEDKey` 以维持旧客户端安装校验，同时 mygo 配置使用自己的新公钥；后续用自己的 `update-darwin-arm64.json` 和 `update-darwin-amd64.json` 及签名归档。Go 包不重新嵌入 Sparkle 运行库。
- [ ] 验证实际旧客户端要求的全部签名层和解包前校验，不能仅以 ZIP 离线签名通过代替安装验证。更新失败应保留原应用；关闭自动检查的偏好跨代保持，开启时可沿用上游启动检查/每小时检查。
- [x] 发布版本从个人 `0.2.0`、build `2000` 起。候选重打递增 build，宿主/Widget/清单一致；与上游 `1.0.3/103` 分离；正式标签 `personal-v0.2.0` 与候选 manifest 版本匹配。将 config staging 强制写入的 `updates.tagPrefix="v"` 改成 `personal-v`，验证 mygo 对个人标签和产品版本的解析。
- [x] 分支 push 仅产生 Actions artifacts；正式标签仅生成 draft、`latest=false`，没有自动公开步骤。Release notes/资产 URL 使用当前个人仓库；不触发上游 `v*` / `beta-v*`；不修改既有发布/标签。
- [x] 针对最终嵌入 Widget、桥接、SQLite 并完成签名后的同一 bundle 生成 ZIP、tar.gz、DMG、两代清单和 SHA256SUMS，防止签名发生在后续资源修改之前。
- [x] 提交 `build: bridge personal Sparkle and mygo update channels`。

**门槛 G5：** 两套签名分别验证，个人 URL 限制不阻断正常下载；开发包不更新；工作流只能创建个人草稿。

### Task 7：自动回归与双架构产物

**修改：** `.github/workflows/ci.yml`、`Scripts/Tests/test-personal-package.py`；新增 `Scripts/Tests/verify-personal-regressions.py`。后者只编排确定的检查并汇总结果，不吞掉失败或改写 fixture。

- [ ] 在独立 runner/测试用户中先跑上游基础测试，再跑定制测试。记录上游原生问题、环境问题和迁移引入问题，不以整体 skip 或改期望值掩盖失败。
- [x] 在 Go worktree 执行以下命令；Swift 基线的 `swift test` 留在保留 worktree，不能在已移除 Package.swift 的 Go 分支执行：

```bash
go test -count=1 -timeout=10m ./...
go vet ./...
go build ./...
go test -race -count=1 -timeout=10m ./internal/...
go test -tags=quosparr_dev_package -count=1 ./internal/app -run '^TestPackagedDevelopmentUpdate'
python3 Scripts/Tests/test-personal-package.py
python3 Scripts/Tests/test-personal-release.py
python3 Scripts/Tests/test-workflow-drafts.py
```

上述 `test-personal-*` 为本计划新增脚本，须先实现后执行；预期全部退出 0，无跳过的核心验收项。Race 运行在原生 arm64 CI；需要桌面会话的测试安排到隔离桌面环境，不把无桌面跳过计为通过。

- [x] Codex 回归覆盖 OAuth/import/API Key、多账号刷新、配额、重置券、估算、成本、Token 图、会话索引修复、隐私开关；故障用 fake 网络和假凭据，OAuth 真实授权只在正常人工冒烟阶段验证。
- [x] 并发回归覆盖刷新重入、部分失败、超时取消、切号后迟到响应、退出时 goroutine 回收；缓存保留真实采集时间，网络错误日志脱敏；代理失败不能直连。
- [x] 依次构建 arm64、x86_64，分开输出目录；在 Task 2 脚本适配后运行：

```bash
V1_DEVELOPMENT_PACKAGE=1 ARCH=arm64 bash Scripts/package-v1-app.sh
V1_DEVELOPMENT_PACKAGE=1 ARCH=x86_64 bash Scripts/package-v1-app.sh
```

- [x] 生产候选从受控签名环境构建，用适配后的 `verify-v1-app.sh` 验证最终 manifest、Mach-O 架构/最低系统、Widget/桥接/SQLite、动态库 rpath、签名、产品身份、版本和更新公钥；DMG 运行 `hdiutil verify`。构建主机的 SDK 版本不是最低系统运行证明。
- [x] 保存产物 SHA、CI run URL、测试统计和失败摘要；验证报告只写结论，不放认证数据。本轮 CI 必需目标为 macOS 两架构，Windows/Linux 构建如保留仅作额外兼容信号，不是已支持声明。
- [x] 提交 `test: gate personal migration regressions and packaging`。

**门槛 G6：** 自动回归和签名构建全部通过，CI 绑定候选确切 SHA。出现后续代码/资源改动须重打受影响产物，不能沿用旧报告。

## Chunk 4：隔离安装、更新和回退验收

### Task 8：用真实旧包验证两段更新

**新增：** `docs/development/personal-migration-update-validation.md`、`docs/testing/personal-migration-proxy.example.json`。复用并按个人仓库适配上游 `docs/v1-isolated-update-proxy.md` 的拓扑。

- [ ] 在一次性 macOS VM 的专用用户安装原始、未修改、未重签的 `personal-v0.1.0`。另做当前稳定本机包到候选的覆盖路径；所有账号数据使用合成样本。
- [ ] 对现有 Sparkle 客户端，用 VM 内 HTTPS 测试代理将**精确的个人 feed 与候选下载 URL**映射到候选原字节。测试 CA 只进 VM，TLS 及签名校验保持开启；真实认证主机不解密、不记录。不修改开发机代理、生产 feed 或 GitHub latest。
- [ ] 先确认从 VM 下载的 XML/ZIP/manifest/归档 SHA 与候选一致，再执行下表。后续 Go→Go 验证用更高产品版本及 build 的临时测试候选（例如 `0.2.1 / 2100`）；先核实 mygo 比较规则，不假设只加 build 可触发升级。验证资产不覆盖拟发布原件、不公开到真实更新源。

| 编号 | 场景 | 必须得到的结果 |
| --- | --- | --- |
| U01 | 原始 Swift 包经旧个人 Sparkle feed 升级 Go | 完整检查/下载/验签/安装/重启；身份/路径/账号/偏好保持 |
| U02 | Go 候选经个人 mygo 清单升级更高测试产品版本 | 新旧进程交接正常，唯一实例，版本实际变化 |
| U03 | 错误 feed/清单签名、错误 ZIP/归档签名 | 错误可见；原 app 文件树哈希不变 |
| U04 | 使用测试 key 签名但内容损坏的归档、截断下载 | 解包/校验失败；不替换、不丢数据；可重试 |
| U05 | HTTP/SOCKS5 不可用、凭据缺失、中途断网 | 不直连，不回退作者源，当前应用可继续运行 |
| U06 | 同步期间修改代理、重复点更新、安装/重启中断 | 当前更新路由一致，无双实例/半成品覆盖，错误后可恢复 |
| U07 | 错架构、较旧 build、同版本重复检查 | 不安装错误或降级资产，不陷入重复升级 |
| U08 | 关闭自动检查后启动并推进时钟，开发包检查 | 零后台检查；开发包更新入口给出禁用状态 |

- [ ] U03/U04/U06 各从同一 VM 快照恢复再执行；故障注入只改临时副本。报告记录原/新 bundle 指纹、数据比较、结果和截图，不记录真实 token。
- [ ] 独立验证更新下载 URL 白名单与 GitHub 实际重定向，正常网络和代理各至少一次；代理映射成功不能代替真实 GitHub 资产可获取性检查。

**门槛 G7：** 两段正常更新及所有失败保护有实际执行证据。测试密钥模拟成功不能替代 U01 原始旧包的真实信任链。

### Task 9：桌面矩阵、回退演练与候选结论

| 环境 | 必须验证 | 环境不足时的处理 |
| --- | --- | --- |
| macOS 12 | 继续运行 Swift，旧 Sparkle 客户端拒绝 Go 的最低系统 13 更新 | 标记未验证，阻止完整公开发布 |
| macOS 13 | Go 启动、状态栏/菜单/设置、配额、更新、无 Widget 时启动 | 标记未验证，阻止完整公开发布 |
| macOS 14+ arm64 | 上述项目、旧 Widget 保留及刷新、登录项、权限、覆盖升级 | VM/专用用户完成后才做日常机正常冒烟 |
| Intel macOS | x86_64 原生启动、更新/重启、配额和可用系统上的 Widget | 交叉构建/Rosetta 不算原生验收；缺机器则保留候选 |

- [ ] 同场景记录面板打开/刷新反馈、闲置 CPU/RSS、睡眠唤醒及连续运行表现；以稳定版同机同 fixture 对照。出现无响应、持续空转、重复刷新或内存持续增长须修复，不用单次截图替代运行验证。
- [ ] 测试用户退出旧版后备份 app、偏好、账号库、缓存和数据库整组；关闭写入者或用 SQLite backup API，不只复制仍活跃的主 db 文件。
- [ ] 覆盖安装候选，检查旧账号自动出现、代理仍有效、用户关闭的启动/提醒/自动检查偏好保留、四类旧 Widget 更新及点击正常。不要因 bundle ID 相同就提前宣称系统权限继承。
- [ ] 演练“退出 Go → 保留 Go 运行后数据副本 → 恢复匹配的 Swift app 与数据/偏好快照 → 验证 Swift 可启动”。模拟凭据可整组恢复；真实 OAuth token 若已旋转，不用旧备份覆盖官方 auth，必要时正常重新授权。默认要求 Go 不做破坏性存储 schema 迁移。
- [ ] 所有门槛通过后，形成候选报告：每项 PASS/FAIL/NOT RUN、候选 SHA/build/hash、CI、平台、缺陷及回退位置。NOT RUN 不转换为 PASS。
- [ ] 最后才安排日常机正常安装冒烟，先备份，验证登录/配额/代理/Widget；失败测试仍留在 VM。形成包含源代码标签、DMG、ZIP、mygo 归档、两代清单、SHA256SUMS、版本说明的 Release **草稿**。
- [ ] 形成可审查差异后合入 `personal/main`；若稳定分支期间有更新，先整合到候选并重跑受影响门槛。`main` 只快进上游固定基线，保留 `personal/legacy-swift`，不强推重写。
- [ ] 公开发布作为候选验收后的单独动作；不因 Actions 成功或文档完成自动发布，不覆盖 `personal-v0.1.0`。最终正式资产必须与验收候选哈希一致。
- [ ] 提交 `docs: record personal Go migration acceptance and rollback`。

**门槛 G8：** 必需平台/数据/Widget/更新/回退均通过，才能推荐日常替换与公开分发。若失败，保留 Swift 稳定线，只修复迁移分支或暂缓迁移，不削减既定功能来获得通过。

## 执行节奏与审查清单

依次完成 Task 1–9，每项形成独立提交。Task 1–2 结束只需证明能安全验证；Task 3–5 结束展示定制行为和合成数据证据；Task 6–7 结束提交可安装候选；Task 8–9 结束才给出是否能替换稳定版的结论。

审查在主线程进行，按代码实际行为逐项确认：产品身份、锁顺序、数据写入边界、Grok 构造点、互动任务、URL 重定向与签名、版本比较、取消/重启、Widget kind/Intent、测试是否覆盖失败分支。测试和构建按门槛执行，已通过且未受后续变更影响的检查不反复重跑。

计划不把缺少 VM、Intel 环境或最低系统运行证据当成代码错误；这些是可明确记录的验收依赖。它们不妨碍完成前面的源码适配和候选构建，但不能省略后宣布迁移完成。
