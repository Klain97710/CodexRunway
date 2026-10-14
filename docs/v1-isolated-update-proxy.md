> 上游历史文档：本定制版保持原身份，不走更名提示或作者更新源。执行时改用 [个人迁移更新验收](development/personal-migration-update-validation.md)。

# v1 隔离 HTTPS 更新代理

状态：配置与步骤用于后续版本公开发布前的隔离验收，代理配置不等于真实更新已通过；旧名称客户端只能接收手动下载提示，以下在线 ZIP 升级流程仅用于 bundle ID 已为 com.quosparr.app 的 Swift 构建；本文件服务于[隔离验收清单](v1-acceptance-checklist.md)，不能把示例语法检查记为 Sparkle / mygo 真实更新通过

## 边界与测试拓扑

在一次性 VM 的专用测试用户中安装未修改、未重签的真实 Quosparr Swift 基线包；代理仅把明确列出的更新 URL 映射到候选文件，保留 feed、签名 ZIP、mygo 清单和归档的原字节；不修改旧 app 的 Info.plist、不替换公钥、不重签旧包、不改生产 feed / DNS / GitHub latest

```text
VM 内真实 Quosparr Swift app
  → VM 内 127.0.0.1:8080 HTTPS 代理
    → 精确 Quosparr appcast URL 对应的候选 XML
    → XML 中精确 ZIP URL 对应的候选原始 signed ZIP
  → 同身份 Swift app 自带 Sparkle 用原 SUPublicEDKey 验签并安装 Go

Go 候选 app
  → 同一隔离代理
    → 固定 v1-beta 的精确 update-<target>.json
    → 清单中精确归档 URL 对应的原始 mygo 签名归档
  → mygo 验签安装、释放锁后重启

所有未列入拦截主机白名单的认证域
  → 原始 TLS 直通隧道，不解密、不记录认证内容
```

建议代理和 app 均在同一 VM；示例监听回环地址，不开放局域网端口；测试 CA 仅装到该一次性 VM 的系统信任存储，不装到开发机；客户端必须继续校验服务器证书，代理的 `ssl_insecure` 必须保持 `false`，不得使用 `curl -k`、关闭 Sparkle / mygo 验签或 TLS 验证来获得「通过」

旧名称历史客户端仍请求 `appcast-<arch>.xml`，该候选文件只含下载页 `<link>`，无 `<enclosure>`；单独检查提示后退出旧应用，再手动安装 Quosparr 并验证本地迁移，不能把新 ZIP 映射到旧安装入口

## 候选资产准备

1. 在受控构建环境生成并验证候选资产，按[发布文档](v1-release.md)保存最终文件的 SHA256、长度、版本、架构、签名与构建号，私钥只在签名环境，不传入代理 VM
2. Swift 过渡所需 `quosparr-appcast-arm64.xml`、`quosparr-appcast-x86_64.xml` 由 `release-assets sparkle-appcast` 生成，指向同一最终签名 app 生成的 `Quosparr-macos-{arm64|x86_64}.zip`，ZIP 使用现有 Sparkle 密钥签名，旧包 `SUPublicEDKey` 验证必须成功
3. 六目标 mygo 清单分别为 `update-darwin-arm64.json`、`update-darwin-amd64.json`、`update-windows-amd64.json`、`update-windows-arm64.json`、`update-linux-amd64.json`、`update-linux-arm64.json`，每份对应 `quosparr-<version>-<target>.tar.gz`
4. beta 的 Go 配置通过 `updates.url` 指向 `https://github.com/Licoy/Quosparr/releases/download/v1-beta`，清单及归档同 base；不可变候选 Release 为 `beta-v<version>`，两者均为 prerelease 且 `latest=false`；不得改用原生 tag feed 期待它包含 prerelease
5. 样例采用具体演示版本 `1.0.0-beta.1`，不是已发布事实；使用前逐一替换为实际候选版本、legacy appcast 的实际 TAG、XML enclosure 的实际 URL、清单归档的实际 URL 与 VM 内文件路径，不扩大为版本通配符
6. 将经过验证的原始文件复制到 VM 的 `/private/tmp/runway-update-vm/candidate`，完整候选应有两份 XML、两份 ZIP、六份 JSON、六份 tar.gz；故障注入使用另外保存的副本，不能覆盖可发布原件

Quosparr Swift 客户端请求代码中固定的 `https://github.com/Licoy/Quosparr/releases/latest/download/quosparr-appcast-<arch>.xml`，代理将它返回为候选 feed；候选 feed 的 ZIP URL 示例是 `.../releases/download/beta-v1.0.0-beta.1/Quosparr-macos-arm64.zip`，必须逐字符核对，不能映射整个 GitHub `/latest/`、整个域名或全部 `.zip`

## 配置示例与启动

[配置样例](testing/v1-update-proxy.example.yaml)采用 JSON 形式的 YAML，便于用 Python 标准库检查语法；不需要项目新增 YAML 库；实际运行使用 VM 中安装的 mitmproxy / mitmdump，先记录 `mitmdump --version` 和 `mitmdump --options`，核对该版本支持样例选项后才继续

该配置只允许 `github.com` 的 TLS 拦截，16 条 `map_local` 均为完整 URL、首尾锚定和 GET 条件，单个本地文件返回；其他主机走不解密的 TLS 隧道，尤其是实际 Codex / ChatGPT / Grok 登录与业务认证主机；没有将 `auth.openai.com`、`chatgpt.com`、`x.ai` 或它们的子域加入拦截列表，域名新增时仍保持默认直通，不做认证域通配映射；不在被拦截的 GitHub 主机登录账号

`allow_hosts` 控制哪些 TLS 主机被拦截，`map_local` 才控制哪些完整请求 URL 返回本地文件；未命中本地映射的 GitHub 请求仍可向线上发起正常请求，不是本地候选响应，也不是离线模式；`connection_strategy=lazy` 仅延迟上游连接，使已经本地映射的请求不必预先连接服务器；非白名单认证域走不解密隧道，真正的 TLS 路径仍须在 VM 用 X-04 验证

`flow_detail=0` 关闭请求输出，不使用 `-w`、不设置 `save_stream_file` / HAR，不启用 sticky auth / sticky cookies、内容修改、认证回放或凭据日志；只留候选文件的公开校验记录；选项语义以 [mitmproxy 官方选项说明](https://docs.mitmproxy.org/stable/concepts/options/)为准，精确本地映射格式见 [Map Local 官方说明](https://docs.mitmproxy.org/stable/overview/features/#map-local)

以下命令只在一次性 VM 执行，路径是 VM 内路径：

```bash
VM_TEST_ROOT=/private/tmp/runway-update-vm
mkdir -p "$VM_TEST_ROOT/proxy" "$VM_TEST_ROOT/candidate"
cp docs/testing/v1-update-proxy.example.yaml "$VM_TEST_ROOT/proxy/config.yaml"
# 先编辑 config.yaml 为本次候选的精确 URL 与实际文件路径
python3 -m json.tool "$VM_TEST_ROOT/proxy/config.yaml" >/dev/null
mitmdump --set confdir="$VM_TEST_ROOT/proxy"
```

首次启动会在专用 confdir 生成测试 CA；只将其中 `mitmproxy-ca-cert.pem` 导入该 VM 的系统证书信任存储，并记录指纹；不能导出 / 上传带私钥的 CA 文件；设置 VM 的 HTTP / HTTPS 系统代理为 `127.0.0.1:8080`，或在支持代理设置的历史 app 内保存 HTTP 代理，旧版无应用代理能力时使用 VM 系统代理；记录选择与恢复方式，禁止修改开发机系统代理

Go 开发包会移除 updates 配置，不能用于在线验收；当前开发包仍沿用生产 bundle ID，因此仅运行在隔离环境；真实迁移用启用候选更新配置并已验签的包，在 VM 专用用户里保留应用的真实历史偏好域；macOS UserDefaults 不随 HOME 自动隔离，开发机临时检查仍必须同时设置 `QUOSPARR_PREFS_DOMAIN=<测试域>`

## 在安装前验证代理

| ID | 步骤 | 预期与记录 |
| --- | --- | --- |
| X-01 | 在 VM 核对配置的 16 个完整 URL、GET 过滤条件、本地文件存在性，验证 JSON/YAML 解析 | 全部候选路径存在，版本与实际 feed / manifest 相同；记录代理版本与配置 SHA256 |
| X-02 | 用 VM 系统信任连接已映射的旧 arm64 / x86_64 feed URL，再连接 XML 内 ZIP URL，保存响应到 VM 临时目录并计算 SHA256 | 与原始 XML / ZIP 完全一致，无内容改写 / 解压重包；保存公开文件指纹 |
| X-03 | 对本次 CPU target 连接固定 `v1-beta` manifest 和其中归档 URL | 与原始 mygo JSON / tar.gz 完全一致，版本、长度、签名不变化 |
| X-04 | 请求非白名单认证主机，只核对其公开证书链或公开无认证路径，不传真实凭据给调试工具 | 仍看到真实服务证书链而非测试 CA，走直通且无流量日志；有任何认证内容被代理解密 / 记录立即停止 |
| X-05 | 检查版本相邻 URL、错架构 URL、带查询参数 / 多余路径的 URL、非 GET 请求与认证域 | 不命中任何本地候选路由；真实环境可能正常访问远端，但绝不被候选替换；不要用宽泛 regex 修复 |
| X-06 | 在未信任 CA 的 VM 快照执行 feed 请求，之后恢复已信任的测试快照 | 未信任时 TLS 明确失败，信任后正常；不能关闭 TLS 来处理 |
| X-07 | 对损坏 ZIP / 错签 ZIP 副本单独配置精确候选映射并运行旧 app | 真实 Sparkle 拒绝，不可将「代理返回了字节」当成真实验签通过 |

只有代理检查通过后才启动历史 app 的真实更新流程；若代理规则不匹配或更新失败，保留错误、请求的公开 URL 与配置指纹，先诊断，不追加宽泛路由或把真实归档验签替换为模拟成功

## 切换阶段与故障用例

- 每一轮绑定明确的候选版本和公开文件指纹，修改映射前停掉代理并保存 VM 快照，重新启动代理后验证响应字节；不要在一轮安装中替换候选文件
- Swift → G1 使用旧 feed 路径加 immutable beta ZIP，G1 → G2 使用固定 beta mygo JSON / tar.gz；每个阶段重新精确填入本轮实际 URL；Swift 版本不再作为回滚目标
- Windows / Linux 使用各自 target 的实际 JSON / 归档，认证域同样直通；Linux deb 用包管理器更新，不用此代理绕过包管理器规则
- 更新中断可在 VM 停代理 / 断网 / 退出应用，错误签名 / 损坏归档用候选副本，不修改原生产资产、不重签历史 app
- 读旧代理密码可能首次触发 Keychain 授权，分别记录允许 / 拒绝的实际体验；更新窗口语言首次显示后固定，记录同进程与重启后的差异
- 手动恢复应用保留最新 token，不能通过恢复轮换前 VM 凭据快照修复认证；启动前隔离准备的 VM 快照与数据无损恢复验收应明确分开

## 收尾与证据

保存每个阶段的源码 / 包版本、CPU、系统版本、资产校验与验签结果、公开代理配置指纹、脱敏错误、PID / 锁状态、实际安装时间与清单记录；不保存认证请求、流量 dump、完整 auth、token、私钥或账号内容

结束后退出自己启动的代理与应用，恢复 VM 内代理设置，移除测试 CA 并销毁一次性 VM；开发机真实 HOME、UserDefaults、Keychain、登录项与生产源应保持未经访问；最终报告分开列出「配置检查通过」「代理实测通过」「真实签名更新通过」和仍未执行的项目，不能混为同一个结论
