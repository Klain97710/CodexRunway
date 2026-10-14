# Go 迁移候选验证报告

2026-10-14。结论：**源码与本地自动化通过，尚未通过日常替换验收。**

用户已选择 Go 最低 macOS 13，macOS 12 留在 Swift 稳定版；Widget 最低 macOS 14。固定上游 v1.0.3 / `94ce1869d2603593972846baa811672c1c2e903c`，稳定 Swift `895e991d67eea3f8500065b605741f42aee54ae3`。候选版本 0.2.0，冻结源码构建号 2002。

## 自动化证据

- 本地全量 Go：1,756 测试/子测试通过；race：1,742 通过；vet/build/开发包禁止更新通过。
- 固定 Swift Core → Go → Swift：偏好、账号索引字段比较通过；Swift/Go 生成的 SQLite 均能复用，精确 Token/费用与时区结果通过。
- Python 包装、发布、签名与草稿命令回归通过。Sparkle XML 和 ZIP 分别校验；合法签名但缺失/错误最低系统的 feed 被拒绝。
- 离屏状态栏覆盖七种语言和两种主题，三列布局及 Pro 单行通过。离屏输出不代表桌面系统缩放、窗口和 Widget 验收。
- 2001 双架构包已通过实际 Mach-O 最低系统、签名、Widget/桥接/SQLite、归档一致性和 DMG 校验。最终 2002 冻结源码包、提交 SHA、校验值与 CI 链接将在执行后追加。

详细本地日志在忽略的 `build/regressions`；报告保留跳过项。可选桌面/真实 Keychain 探针和上游保留的两项 Grok 日期边界测试不计为 PASS。SQLite 跨实现探针已单独实际执行，并非依赖套件内条件跳过。

## 尚未执行的验收

| 项目 | 状态 | 原因 |
| --- | --- | --- |
| 原始 Swift → Go Sparkle 安装/重启 | NOT RUN | 需要一次性 macOS VM/专用测试用户 |
| Go → 更高版本 mygo 安装及故障保护 | NOT RUN | 同上；离线签名测试不能代替真实更新器 |
| 旧四类 Widget 保留、定时刷新、点击 | NOT RUN | 需要 macOS 14+ 隔离桌面 |
| macOS 12 留在 Swift、macOS 13 原生启动 | NOT RUN | 缺最低系统环境 |
| Intel 原生启动/更新 | NOT RUN | 缺 Intel 硬件，交叉构建不替代实机 |
| 系统权限、登录项、睡眠唤醒、连续运行/性能 | NOT RUN | 需要隔离桌面及同机 Swift 对照 |
| 真实安装后的整组数据回退 | NOT RUN | 必须与上述迁移实验一起执行 |

目前 Parallels 只有 Windows guests。未启动 Go 桌面主程序接触真实账号，未改本机代理/信任，未覆盖已安装 Swift app；未改变个人稳定分支、默认分支、已发布 0.1.0、latest 或公开更新源。

## 下一步与回退

按[隔离更新手册](personal-migration-update-validation.md)执行 U01–U08 和桌面矩阵。只在全部门槛通过后安排日常安装冒烟、候选草稿与合入 `personal/main`。缺环境不妨碍源码和候选构建，但不能宣布完整迁移通过。

原始发布 Swift 与当前本机 Swift 均已备份到仓库外受限目录 `backups/20261014-093347-pre-go-migration`。退出 Go 后先保存其运行后数据，再恢复匹配的 Swift 应用、合成数据与偏好；真实 OAuth 已旋转时不得用旧 token 备份覆盖官方认证。
