[English](./upgrade-from-codexrunway.md) · 简体中文 · [繁體中文](./upgrade-from-codexrunway_ZH_HANT.md) · [한국어](./upgrade-from-codexrunway_KO.md) · [日本語](./upgrade-from-codexrunway_JA.md) · [Русский](./upgrade-from-codexrunway_RU.md) · [Français](./upgrade-from-codexrunway_FR.md)

# 从 CodexRunway 升级

新身份为 `com.quosparr.app`、`quosparr://` 和 `~/.quosparr`，旧应用无法自动更新为 Quosparr。请退出 CodexRunway，手动安装 Quosparr 后启动：

- 首次启动会复制 `~/.codex-runway` 中的数据并导入缺失偏好，原目录保留不动，迁移中断后可安全恢复
- 旧名称导出的账号包仍可导入，新导出采用 `quosparr-accounts`
- 系统权限不会自动继承，需要重新添加桌面组件并检查登录项；macOS 上旧登录项可能需要在“系统设置 → 通用 → 登录项”中手动移除
- 迁移不会修改官方 Codex / Grok 登录文件
