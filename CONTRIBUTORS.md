# 贡献者

[README](README.md) | [简体中文](README_ZH.md)

感谢所有为 Quosparr 提交代码、测试、反馈、文档和设计建议的人。

## 当前维护

- 项目维护者：仓库所有者。
- 贡献者名单：以 Git commit、issue、pull request 和 release 记录为准。
- 新贡献合并后，可以在本文件中补充贡献类型，但不要手工添加无法追溯的姓名或身份。

## 可以贡献什么

- Go / mygo 原生界面功能实现，以及 macOS WidgetKit 扩展（Swift）。
- 配额、reset credits、API 等价成本、多账号导入/切号和会话修复逻辑的测试。
- macOS 菜单栏、Windows / Linux 系统托盘、弹层和设置体验优化。
- 七种语言文案改进。
- 打包、CI、Release 和在线更新流程改进。
- Bug 报告和复现步骤。

## 开发环境

- macOS 12+、Windows 10 / 11 或带系统托盘的 Linux 桌面
- Go 与 mygo 版本以 `go.mod` 为准
- 构建 macOS 桌面组件需要 Xcode
- 推荐本机已使用过 Codex；可通过 `~/.codex/auth.json` 或应用内导入/登录添加测试账号

常用命令：

```bash
go test ./...
go vet ./...
go build ./...
QUOSPARR_TEST_HOME="$(mktemp -d)"
HOME="$QUOSPARR_TEST_HOME" QUOSPARR_PREFS_DOMAIN=com.quosparr.app.dev-test go run . --self-check
bash Scripts/run-dev-app.sh
```

`run-dev-app.sh` 使用正式身份和真实数据运行开发包，需先退出已安装的 Quosparr；只想隔离试用时使用上面的临时 HOME 与独立偏好域。

打包命令：

```bash
V1_DEVELOPMENT_PACKAGE=1 ARCH=arm64 bash Scripts/package-v1-app.sh
V1_DEVELOPMENT_PACKAGE=1 ARCH=x86_64 bash Scripts/package-v1-app.sh
```

## 代码边界

- 原生界面与系统协调放在 `internal/app`，业务模块放在其余 `internal/*` 包。
- macOS 桌面组件放在 `Sources/QuosparrWidget`，桥接库放在 `Sources/QuosparrWidgetBridge`。
- UI 不直接解析 JSON、不直接扫描本地会话文件。
- 所有用户可见文案必须走 `internal/l10n`，新增 key 需要同时补齐英文、简体中文、繁体中文、韩文、日文、俄文、法文。
- 默认不新增依赖；标准库和系统框架能解决的问题不要加包。

## 隐私和安全

- 不要在代码、日志、README、issue、测试输出中写入 access token、refresh token、id token 或 API key。
- 读取 `~/.codex/auth.json` 时只使用必要字段。
- 多账号凭据仅允许存放在 `~/.quosparr/accounts/<id>/auth.json`（目录 `0700`、文件 `0600`，原子写入）；账号索引 `index.json` 不得包含 token。
- 修改官方 `~/.codex/auth.json` 仅限：OAuth refresh 回写、以及用户主动切号时的原子写入。
- 刷新非当前托管账号 token 时只写账号库副本，不得写官方 `auth.json`；刷新当前账号时同步官方 auth 与账号库副本。
- 无效或 mock 凭据不得写回官方 `auth.json`。
- 会话修复只允许处理 `session_index.jsonl`，写入前必须备份。
- 不删除 `~/.codex/sessions/**`、`~/.codex/archived_sessions/**` 或全局状态文件。
- 更新检测只应请求 Release 元数据和 appcast，不上传 Codex 会话内容。

## 测试要求

提交前至少运行：

```bash
go vet ./...
go test ./...
```

涉及打包或桌面组件时额外运行：

```bash
V1_DEVELOPMENT_PACKAGE=1 bash Scripts/package-v1-app.sh
```

如果某项验证无法运行，需要在提交说明或 PR 说明里写明原因。

## 提交规范

提交信息使用 Conventional Commits：

```text
<type>(<scope>): <subject>
```

常用 type：

- `feat`
- `fix`
- `docs`
- `test`
- `refactor`
- `build`
- `ci`
- `chore`

示例：

```text
docs(readme): 完善中英文使用说明
```

## 文档规则

- `README.md` 默认为英文。
- `README_ZH.md` 是简体中文说明，不替代英文首页。
- 文档不要包含外部参考应用名称、链接或代码片段。
- 文档应清楚说明隐私边界、打包方式、在线更新和系统要求。
