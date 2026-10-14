[English](./upgrade-from-codexrunway.md) · [简体中文](./upgrade-from-codexrunway_ZH.md) · 繁體中文 · [한국어](./upgrade-from-codexrunway_KO.md) · [日本語](./upgrade-from-codexrunway_JA.md) · [Русский](./upgrade-from-codexrunway_RU.md) · [Français](./upgrade-from-codexrunway_FR.md)

# 從 CodexRunway 升級

新的識別為 `com.quosparr.app`、`quosparr://` 與 `~/.quosparr`，因此舊應用程式無法自行更新為 Quosparr。請結束 CodexRunway，手動安裝 Quosparr 後啟動：

- 首次啟動時會複製 `~/.codex-runway` 中的資料並匯入缺少的偏好設定，原目錄會保留，遷移中斷後可安全接續。
- 以舊名稱匯出的帳號套件仍可匯入，新的匯出使用 `quosparr-accounts`。
- 系統權限不會被繼承，請重新新增桌面小工具並檢查登入項目。在 macOS 上，舊的登入項目可能需要到「系統設定 → 一般 → 登入項目」手動移除。
- 遷移不會修改官方 Codex / Grok 的登入檔案。
