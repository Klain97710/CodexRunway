# 个人版跨代更新：隔离执行手册与记录

2026-10-14：**U01–U08 全部 NOT RUN。** 当前仅完成构建、离线签名/损坏拒绝、URL/重定向/代理单元测试；这些不等同于真实安装更新。开发机保持原 Swift app、偏好、Keychain 与代理设置。

最低系统范围已由用户于 2026-10-14 确认为 **Go 版 macOS 13+；macOS 12 继续使用 Swift 稳定版**。旧 2000 诊断包的最低系统声明不一致，禁止使用；2001 已完成最低系统修复校验，最终源码冻结包为 0.2.0/2002，应用及 Sparkle feed 均声明 13.0。旧 macOS 12 Swift 客户端不得获得这个不兼容更新。

## 环境与可信原件

在一次性 macOS VM 的专用测试用户中执行。现有 Parallels 只注册了 Windows guests，不能代替本步骤。预留 Intel 原生机器及最低支持系统测试，Rosetta/交叉编译不算原生验收。

1. 准备未修改、未重签的 `personal-v0.1.0` 发布包，以及当前稳定 Swift 构建。保存两者应用树哈希、版本、签名和安装位置。
2. 仅导入合成账号、代理凭据及偏好；在 Swift 版添加四种 Widget。关闭所有写入者后整组备份应用、UserDefaults、账户、缓存与 SQLite/WAL。建立 VM 快照。
3. 将已完整验证的 `build/personal-candidate` 复制到 VM 的 `/private/tmp/personal-runway-update-vm/candidate`，私钥留在构建机。核验 SHA256SUMS。
4. 单独构建更高产品版本的测试包（例 `0.2.1 / 2100`），保存到 `next`，不得覆盖 0.2.0 原件或上传生产源。mygo 比较产品版本，不能假设只增加 build 就能触发更新。

## 精确 HTTPS 路由

使用 [代理样例](../testing/personal-migration-proxy.example.json)，仅在 VM 内运行 mitmdump。先核对当前 mitmproxy 版本支持这些选项，再把配置保存为 VM 专用 confdir 的 config.yaml（JSON 是合法 YAML）。测试 CA 只导入 VM；保持 TLS/签名校验开启，不使用 `curl -k`，不修改开发机代理或系统信任。

样例只拦截 github.com 和 api.github.com，所有认证域继续原始 TLS 隧道、不解密、不记录。仅完整 URL、GET 和本地文件精确匹配；没有通配版本或整个域名映射。关闭流量日志/HAR/请求保存。不要在这个 VM 的拦截主机上登录真实 GitHub 账号。

- Swift → Go：旧个人 `appcast-arm64.xml` / `appcast-x86_64.xml` 返回原字节签名 XML；其中 ZIP URL 返回对应候选原字节。
- Go → Go：mygo 先查询 `api.github.com/repos/Klain97710/CodexRunway/releases?per_page=100&page=1`，再读取 `personal-v0.2.1` 的清单。将 [公开版本列表样例](../testing/personal-migration-releases.example.json) 放为 `next/releases-page-1.json`，其版本与测试包一致。该模拟只存在于 VM，不改变真实 GitHub draft/latest 状态。
- Go JSON 不是独立签名信封；mygo v0.4.8 校验清单中的归档 SHA256 签名。测试应分别篡改 metadata、归档 URL、signature、size 和归档字节，按实际拒绝点记录，不写“验证了不存在的清单签名”。
- Sparkle 同时要求 XML 和 ZIP 签名。XML 签名后任何内容变更都须重新签名；不能把只含 ZIP 签名的 feed 算作合法候选。

先用 VM 信任库下载每个映射 URL 并核验 SHA256 与候选一致；相邻版本、非 GET、额外查询参数不得命中。另用无认证公开请求确认认证域证书仍来自真实服务且没有被代理解密。未信任测试 CA 时连接必须失败。

真实 GitHub 的个人 0.1.0 ZIP 地址已在开发机进行只读 HEAD 检查：302 到 `release-assets.githubusercontent.com/github-production-release-asset/1401041367/...`，最终 200。运行时白名单接受该仓库 CDN 命名空间；本地代理单元测试覆盖 TLS CONNECT 且不关闭证书验证。VM 中仍需正常网络与代理各完成一次真实下载。

## 执行矩阵

| ID | 操作 | 通过要求 | 当前状态 |
| --- | --- | --- | --- |
| U01 | 原始 Swift 发布包经个人 Sparkle feed 更新 | 下载、两层签名、替换、重启完成，身份/路径/数据保留 | NOT RUN |
| U02 | Go 0.2.0 经个人 API/manifest 升级 0.2.1 | 实际版本变化，旧进程释放锁，唯一新实例 | NOT RUN |
| U03 | 错 XML 签名、错归档签名、篡改 URL/metadata | 错误可见，当前 app 树哈希不变，无上游回退 | NOT RUN |
| U04 | 合法签名但损坏的归档、截断下载 | 安装/解包失败保留当前 app 和数据，可重试 | NOT RUN |
| U05 | HTTP/SOCKS5 不可用、凭据缺失、中途断网 | 不直连、不回退作者源，当前 app 可用 | NOT RUN |
| U06 | 更新时改代理、重复检查、安装/重启中断 | 周期路由一致，无双写入者/半安装，失败可恢复 | NOT RUN |
| U07 | 错架构、旧版本、同版重复检查 | 不错误安装/降级，不反复升级 | NOT RUN |
| U08 | 关闭自动检查后启动；开发包检查 | 无后台检查，开发包明确禁用 | NOT RUN |

每轮故障从同一 VM 快照开始，只改临时资产副本。记录实际提示、前后 app 树哈希、偏好差异、锁/进程、数据权限及代理配置指纹；不记录真实 token、私钥、认证内容。

## Widget、权限与回退

四种旧 Widget 保留布局，Codex 数据刷新、旧 Grok/Both 明确显示 Codex、点击旧 scheme 正常。跨至少一个计划刷新周期观察，即使快照未改变；注册成功不等于刷新通过。

测试 Keychain 允许和拒绝授权、开机启动偏好保留、通知、睡眠唤醒、面板/独立详情窗口、退出重启。用同一合成样本对比 Swift/Go CPU、RSS、刷新反馈，记录连续运行表现。

回退：退出 Go → 留存 Go 运行后数据副本 → 恢复匹配的 Swift 应用、合成数据和偏好快照 → 启动验证。真实 OAuth 若已经旋转，不用旧备份覆盖官方认证；真实账号仅在最终正常冒烟阶段使用。

在上述门槛及平台矩阵全部通过前，保持 `personal/main` 与已安装 Swift 版不变，不创建公开更新。候选即使进入 Release 也只能是 draft/latest=false；公开发布仍为单独动作。
