# 个人发布维护

集成分支为 `personal/main`，`main` 仅保留上游基线。首版版本 0.1.0、构建号 1000；后续两者递增，应用和 Widget 版本保持一致。标签使用 `personal-v<版本>`，只由该前缀触发发布。

## 签名身份

首次初始化使用 `swift Scripts/generate-update-keys.swift /absolute/private/key-directory`，生成的目录为 0700、文件为 0600。私钥必须放在仓库外，保留安全的离线备份；已有密钥不可覆盖。公钥进入应用，私钥只用于签名更新包和清单。

首次配置后，后续发布继续使用同一密钥。GitHub Secrets 名称为 `SPARKLE_PUBLIC_KEY` 和 `SPARKLE_PRIVATE_KEY`，用重定向上传，不在命令参数或日志中展开私钥：

```bash
gh secret set SPARKLE_PUBLIC_KEY --repo Klain97710/CodexRunway < /absolute/private/key-directory/sparkle-public.key
gh secret set SPARKLE_PRIVATE_KEY --repo Klain97710/CodexRunway < /absolute/private/key-directory/sparkle-private.key
```

密钥配对验证：

```bash
SPARKLE_PUBLIC_KEY="$(cat /absolute/private/key-directory/sparkle-public.key)" \
  swift Scripts/verify-update-key.swift < /absolute/private/key-directory/sparkle-private.key
```

验证脚本只输出成功或失败，不打印密钥。缺失、格式错误、不匹配均终止发布。丢失私钥后不能给现有安装继续签名更新，应通过新的手动安装恢复信任。

## 本地验收

```bash
swift test
swift build
python3 Scripts/Tests/verify-update-proxy.py
```

更新验收使用临时 app、独立签名密钥和临时安装位置。正常升级验证下载、解包、安装及重新启动；坏清单、坏安装包签名、已签名但损坏的 ZIP、不可用的真实本机代理均要求保留原应用字节内容。不会对日常应用执行失败测试。

两种架构依次构建，`dist/CodexRunway.app` 每次都会被覆盖；对应架构的归档文件各自保留：

```bash
SPARKLE_PUBLIC_KEY="$(cat /absolute/private/key-directory/sparkle-public.key)" \
  RUNWAY_RELEASE=1 ARCH=arm64 bash Scripts/package-app.sh
bash Scripts/verify-packaged-app.sh dist/CodexRunway.app arm64 local release
hdiutil verify dist/CodexRunway-macos-arm64.dmg

SPARKLE_PUBLIC_KEY="$(cat /absolute/private/key-directory/sparkle-public.key)" \
  RUNWAY_RELEASE=1 ARCH=x86_64 bash Scripts/package-app.sh
bash Scripts/verify-packaged-app.sh dist/CodexRunway.app x86_64 local release
hdiutil verify dist/CodexRunway-macos-x86_64.dmg
```

省略 `RUNWAY_RELEASE=1` 时，打包产物禁用更新，适用于本地开发。正式包保留原版标识并强制 Sparkle 验证清单及解包前签名。本项目使用免费 ad-hoc 签名，不使用 Developer ID 或 Apple 公证。

## 草稿与公开发布

1. 更新 `Resources/Info.plist` 版本/构建号和 Widget 项目版本，添加 `docs/releases/personal-v<版本>.md`。后续没有手写说明时，git-cliff 只识别 `personal-v*` 标签。
2. 测试、双架构包与实际安装验收后提交并推送 `personal/main`。创建对应源码标签并推送；不要将上游 `v*` 标签作为个人发布触发器。
3. Release 工作流再次测试、核验密钥、构建双架构、生成带签名的 appcast、计算 SHA256SUMS，然后创建 **draft**。
4. 从草稿下载产物，核对版本、架构、Widget、ad-hoc 签名、清单签名、安装包签名和 SHA256。确认下载前缀指向当前个人仓库及正确标签。
5. 在本机验证 arm64 覆盖安装、启动、旧偏好、Codex 配额与 Widget 注册。Intel 实机未验证时在发布说明中明确记录，不能把交叉构建等同于实机验证。
6. 验收完成才将草稿公开，并检查 `releases/latest/download/appcast-arm64.xml` 和 x86_64 对应地址。仓库默认分支设为 `personal/main`。

首版必须手动安装。之后默认自动检查，由用户确认下载和安装；任何失败均不回退到原作者更新源。

## 同步上游

先更新 `main` 基线，再在 `personal/main` 合并并逐项复核：`RunwayDistribution`、`RunwayFeatures.production`、偏好迁移、Widget 选择、更新下载白名单、发布触发器与签名要求。保持 Grok 只在测试注入时启用，不重新引入互动请求或访客文件使用。保留原作者署名和 AGPL-3.0。
