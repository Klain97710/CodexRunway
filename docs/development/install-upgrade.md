# 安装、覆盖升级与回退

本版由 Klain97710 维护，基于 Licoy/CodexRunway，采用 AGPL-3.0。安装文件名仍为 CodexRunway.app，沿用原版应用和 Widget 标识、账号库、偏好、Keychain 项目、单实例锁及快照目录。它用于替代原版，不支持两版同时运行。

## 首次安装或从原版切换

1. 从本仓库 Releases 下载匹配芯片的 DMG：Apple Silicon 选 arm64，Intel 选 x86_64。
2. 通过菜单栏退出正在运行的 CodexRunway。已有用户将旧应用复制到备用位置；偏好可用 `defaults export com.github.codex-runway <备份文件.plist>` 导出。备份放在仓库外并限制访问权限。
3. Homebrew 用户先执行 `brew uninstall --cask codex-runway`，不要加 `--zap`，然后安装本版。此后从本版更新，不再安装原作者的 cask。
4. 打开 DMG，把应用放到原来的安装位置（通常为 /Applications 或 ~/Applications），确认替换。新用户可拖到 Applications。
5. 首次启动按 macOS 的“隐私与安全性”提示允许打开。本版为 ad-hoc 签名，未进行 Apple 公证。
6. 已有账号和数据会沿用。新用户先使用本机 Codex 登录，或进入设置中的多账号页面，通过浏览器登录、导入本机登录或导入凭据添加账号。

安装迁移不写入官方 `~/.codex/auth.json`。应用运行时仍保留原有 Token 刷新及用户主动切换账号行为。备份和发布包不得包含真实凭据或会话内容。

## 后续更新

正式版只访问本仓库的签名更新源。自动检查默认开启；用户确认后下载、安装并重新启动。首次从原版切换必须手动安装，因为原版的签名信任和更新地址不属于本版。

开发包不开启更新。签名验证失败、代理不可用或下载失败时保留当前应用，修复网络或从本仓库手动下载安装包。更新不会改用原作者发布源。

## 回退

退出应用，将备份应用复制回原位置，再启动。需要时在退出状态下导入之前备份的偏好。账号库和会话数据保留原地，不通过卸载清理或恢复旧凭据来回退应用。若回退到原作者版本，其更新源会随原应用恢复。

## 验证清单

- 应用版本显示 0.1.0 / 1000，“关于”显示 Klain97710 定制版。
- 账号、代理、显示设置可用；旧 Grok 选择归一化为 Codex。
- 菜单栏只有一个实例；原有 Widget 能读取 Codex 数据。
- 开机启动沿用既有偏好；首次安装按 macOS 设置授权。

## English

This is the Klain97710 custom edition of Licoy/CodexRunway, under AGPL-3.0. It replaces the upstream app and preserves its application/Widget identifiers, URL scheme, Keychain services, single-instance lock and data locations.

1. Download the matching DMG from [this repository](https://github.com/Klain97710/CodexRunway/releases/latest): arm64 for Apple Silicon, x86_64 for Intel. macOS 12+ is required; Widgets need macOS 14+.
2. Quit CodexRunway. Copy the old app to a private backup directory outside the repository. Export preferences with `defaults export com.github.codex-runway /absolute/backup/preferences.plist` and restrict access to that backup.
3. Homebrew users must first run `brew uninstall --cask codex-runway` **without `--zap`**, preserving app data. Install this edition manually afterward and stop using the upstream cask for updates.
4. Copy the new app to the same installation path, usually `/Applications/CodexRunway.app` or `~/Applications/CodexRunway.app`. First-time users can drag it to Applications.
5. The app is ad-hoc signed, without Apple notarization. Use right-click → Open or System Settings → Privacy & Security → Open Anyway after verifying the source and SHA256. If macOS specifically reports a damaged app despite a matching checksum, remove quarantine only from this verified app: `xattr -dr com.apple.quarantine /Applications/CodexRunway.app` (adjust the path if needed).
6. Open the menu bar panel. Existing accounts remain available. New users can import the current Codex login or sign in through the browser in Accounts settings, then refresh quota. API-key accounts do not provide ChatGPT subscription quota.

Installation migration itself never writes `~/.codex/auth.json`. Runtime OAuth refresh and explicit account switching retain their existing behavior. Do not include account files, sessions or credentials in releases or shared diagnostics.

After the first manual replacement, updates use only this repository's signed Sparkle feed. Checks are automatic by default; downloads and installation need user confirmation. Signature, download and proxy failures preserve the installed app. Development packages disable online updates.

To roll back, quit the app and copy the backup application to its previous path. If necessary, restore preferences while the app is stopped using `defaults import com.github.codex-runway /absolute/backup/preferences.plist`. Keep account and session data in place. Restoring the upstream app also restores its update channel.

Verify version 0.1.0 (1000), the Klain97710 label in About, account availability, proxy behavior, login-at-startup preferences, a single menu bar instance and existing Widgets. Legacy Grok / Both selections become Codex; unrelated preferences remain unchanged.
