# 上游同步记录：2026-10-19

将上游 `07ca0fe4f7d39c8c12e0e3ddc98d9ecadf41b76c` 之后的 **50 个提交**合入 `personal/migrate-quosparr-v1`，截至 `21947ff00f4b3b3beb01af7bc8373a09c7bca79b`（Quosparr v1.6.2 及后续官网更新）。个人产品版本保持 **0.2.0**，构建号递增至 **2004**。用户明确授权本次同步并替换本机应用。

## 同步与定制适配

- 合入 Skill、MCP、本机会话管理、API 账号余额展示、悬浮窗跟随、加载状态及速率修复。
- 新增管理页仅提供 Codex。Skill 扫描不再无条件遍历三个服务；使用运行时传入的 home，不回退读取真实用户目录。读取、启停、删除、转移、会话详情及回收站操作均受提供方限制；跨服务 Skill 转移入口隐藏。
- Skill 页面保留原回收站状态编号，Codex-only 视图的第二个标签正确对应回收站。过期的 Claude/Grok 页面状态归到 Codex。保留其他服务实现与显式启用的合成测试。
- 保留剩余额度三列、Pro 仅在菜单栏隐藏标准 5h、仅操作项的右键菜单、独立详情窗口和 Codex-only Widget。保留重置公开状态和提醒，不恢复互动或访客 Cookie。
- 上游新增原生 UI API，mygo 从 0.4.8 升到 **0.5.5**，同步 SQLite 资源与打包器版本。Go 1.27.1 不变。检查更新器仍通过受限制的全局 HTTP transport，代理、签名和个人仓库白名单继续生效。
- 仍沿用应用和 Widget 身份、数据目录、锁与个人更新源。稳定分支、默认分支和已发布 `personal-v0.1.0` 不变；不创建公开 Release。新增文案中的备份路径和应用名改为定制版实际值。
- 新管理功能中对 Skill/MCP 的修改和会话回收仅由用户显式操作触发；安装和验证不执行这些动作。原会话索引修复仍不删除会话目录。

## 本地自动化验证

| 检查 | 结果 |
| --- | --- |
| 全量 `go test ./...` | PASS，2,840 项测试/子测试 |
| `go test -race ./internal/...` | PASS，2,820 项测试/子测试 |
| `go vet ./...`、`go build ./...` | PASS |
| 开发包禁止更新、包装/签名/草稿工作流/历史版本检查 | PASS |
| 固定 Swift Core → Go → Swift | PASS，偏好、账号索引、缓存、日期和 SQLite 复用 |
| 新增管理页提供方限制、禁用操作拒绝、缺少 home 拒绝及回收站标签映射 | PASS |
| mygo 0.5.5 `TestUpdater` | PASS，临时应用正常升级，错误签名、错误密钥和大小不符均保留旧应用 |
| 本地 arm64、x86_64 隔离开发包 | PASS，宿主/Widget 构建号、架构、最低系统、SQLite/桥接及最终签名 |

本地检查针对合并工作区。回归日志位于忽略的 `build/regressions`；开发包位于 `build/sync-20261019-dev-{arm64,x86_64}`。Go 套件各跳过 23 项环境/预览探针；SQLite 跨实现探针另行通过，其余跳过项不计为通过。macOS 最低版本保持宿主 13、Widget 14。

## 交付状态

GitHub CI [39222232702](https://github.com/Klain97710/CodexRunway/actions/runs/39222232702) 全部通过，包括 Go 回归、固定 Swift 数据兼容和 arm64/x86_64 开发包。下载的 CI 报告确认源码为 `81bf09426b6d3be9872f9e33413c93f3444bba94`、`dirty=false`、零失败，测试计数与本地一致。

2026-10-19，按用户授权，从上述干净提交构建 arm64 replacement-candidate，安装到 `/Users/apple/Applications/CodexRunway.app` 并正常启动：

- 正式身份、宿主/Widget 构建号 2004、Mach-O 最低系统、SQLite/桥接、最终签名、mygo 清单签名与 DMG 校验通过，二进制记录 `vcs.modified=false`。
- 退出旧版 2003 后持有共享锁，备份旧应用、完整偏好和 19 个应用数据文件；逐文件指纹一致后原子替换。私有备份为 `~/.codex-runway-release/backups/20261019-162921-before-local-go-2004`，早期所有回退点保留。
- 启动后只有一个宿主进程，共享锁正常持有，安装二进制与验证包一致。
- 5 个账号、账号顺序、当前账号、重登录标记、完整偏好（包括开机启动设置）和代理保持不变。
- Codex 配额与 Codex-only Widget 快照均在安装后刷新，快照权限 0600；Widget 插件注册在新安装路径。插件注册不视为桌面 Widget 的完整视觉验收。
- 未切换账号、修复或回收会话，也未对真实账号执行失败注入。安装迁移不改写官方认证文件；启动后的合法刷新按原有路径运行。

安装二进制 SHA256：`e2819f240acc7e011c64ab7f6222f61139934b6ce510397db32859fe3f36e6e2`。ZIP SHA256：`28c4cf35b69d519e3d3b6d2f51af7647ce59f6e6db3975cf04e64c685db2f1c1`。本地包和校验记录位于忽略的 `build/local-install-2004-arm64`；私有备份内保存安装、启动验证记录和回退说明。

Intel 实机、最低系统原生启动、桌面 Widget、完整跨代更新与故障回退仍以[迁移验证报告](quosparr-migration-validation.md)中的未验收项为准。本次正常本机安装不等同于公开发布验收。
