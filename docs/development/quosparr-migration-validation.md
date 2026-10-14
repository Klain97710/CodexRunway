# Go 迁移候选验证报告

2026-10-14。结论：**源码与本地自动化通过，尚未通过日常替换验收。**

用户已选择 Go 最低 macOS 13，macOS 12 留在 Swift 稳定版；Widget 最低 macOS 14。固定上游 v1.0.3 / `94ce1869d2603593972846baa811672c1c2e903c`，稳定 Swift `895e991d67eea3f8500065b605741f42aee54ae3`。候选版本 0.2.0，冻结源码构建号 2002。

## 自动化证据

- 本地全量 Go：1,756 测试/子测试通过；race：1,742 通过；vet/build/开发包禁止更新通过。
- 固定 Swift Core → Go → Swift：偏好、账号索引字段比较通过；Swift/Go 生成的 SQLite 均能复用，精确 Token/费用与时区结果通过。
- Python 包装、发布、签名与草稿命令回归通过。Sparkle XML 和 ZIP 分别校验；合法签名但缺失/错误最低系统的 feed 被拒绝。
- 离屏状态栏覆盖七种语言和两种主题，三列布局及 Pro 单行通过。离屏输出不代表桌面系统缩放、窗口和 Widget 验收。
- 最终 2002 双架构包已通过实际 Mach-O 最低系统、签名、Widget/桥接/SQLite、归档一致性和 DMG 校验。宿主与 Widget 版本/构建一致；两代签名通过。

详细本地日志在忽略的 `build/regressions`；报告保留跳过项。可选桌面/真实 Keychain 探针和上游保留的两项 Grok 日期边界测试不计为 PASS。SQLite 跨实现探针已单独实际执行，并非依赖套件内条件跳过。

## 冻结候选与校验值

源码提交：[`6ebc6a061189fe4b561b2c405810c812b105e13c`](https://github.com/Klain97710/CodexRunway/commit/6ebc6a061189fe4b561b2c405810c812b105e13c)。两个架构二进制的 `vcs.revision` 均为该提交，`vcs.modified=false`。本地完整回归报告也绑定该提交且 `dirty=false`。后续仅记录验收结果的文档提交不改变该冻结候选。

候选文件位于忽略的 `build/personal-candidate`，未上传到 Release。`SHA256SUMS` 自身 SHA256：`83e38f0eba1b530c807c9997c7fddc2551ee5e5d33786f2293123cd1fdf5e1a9`。

```text
dd5f4b542f4420dc4687ed7e97adbdddef7cd1b8ef81ecc703d6752c4ffa12c2  CodexRunway-macos-arm64.dmg
45ea1fcccd65c239eba214d8282db8dc726eab240320211908dda10777e8f73d  CodexRunway-macos-arm64.zip
87ff11b6686568fe0c86ff7e04771471cc76a21b854790dff2db1e7c773ceff9  CodexRunway-macos-x86_64.dmg
cd7baed051dd2c7ae4700e0ca97d426b62fd9f43a7dc12e6bf59fab5c0c6d2be  CodexRunway-macos-x86_64.zip
5aefe49efe9454e75946fb7f5eca5521f9da05464d6e4c317ca7a12dcd9e4b4f  appcast-arm64.xml
0f06031948a3668193951452b8158f3aabcc5e59870129de9ff752831fdb206a  appcast-x86_64.xml
6306a2563c708f0cbdc9c897812f7ea20c63ea5ec973f83709a1a9a6a55a8570  codex-runway-0.2.0-darwin-amd64.tar.gz
71d620f7757bab669fc98afeee70d07a24a96b85b956d860541d581a6b9a8d25  codex-runway-0.2.0-darwin-arm64.tar.gz
7e457a5759811575c57345cd2fadfdced5d4ad60d36dc41ff8988941b82a6a3c  update-darwin-amd64.json
ea4a13a8319460f71f57b3433d6abd49fa64833bde78025872fb3fc3fe198fc0  update-darwin-arm64.json
```

GitHub CI：[38520238128](https://github.com/Klain97710/CodexRunway/actions/runs/38520238128)，绑定同一源码 SHA。全部通过：go-checks、arm64 开发包、x86_64 开发包。已下载 CI 回归报告，确认源码 SHA、`dirty=false`、1,756/1,742 测试计数和零失败与本地一致。CI 仅生成隔离开发包和测试日志，不使用发布私钥。

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
