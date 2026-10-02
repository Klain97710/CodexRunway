[English](./README.md) · [简体中文](./README_ZH.md) · 繁體中文 · [한국어](./README_KO.md) · [日本語](./README_JA.md) · [Русский](./README_RU.md) · [Français](./README_FR.md)

# CodexRunway · Klain97710 定製版

基於 [Licoy/CodexRunway](https://github.com/Licoy/CodexRunway) 的親友使用版，版本 **0.1.0（1000）**，僅啟用 Codex。介面保留繁體中文支援。

## 安裝與升級

1. 從[本倉庫 Releases](https://github.com/Klain97710/CodexRunway/releases/latest)下載 DMG：Apple Silicon 選 arm64，Intel 選 x86_64。
2. 退出原版、備份舊應用與偏好，再覆蓋原安裝位置。沿用帳號、Keychain、Widget 標識與本機資料。
3. Homebrew 使用者先執行 `brew uninstall --cask codex-runway`，不要使用 `--zap`，再手動安裝本版。
4. 主應用需要 macOS 12+，Widget 需要 macOS 14+。本版採 ad-hoc 簽名、未公證；首次可右鍵「打開」或使用系統「隱私權與安全性」允許。
5. 點選選單列圖示，在帳號設定匯入目前登入或使用瀏覽器登入，再重新整理配額。

首次手動替換後，更新只使用本倉庫的 Sparkle 簽名來源，經使用者確認才安裝。失敗保留目前應用。回退時退出應用並還原備份，保留帳號與工作階段資料。

重置動態預設開啟，每 5 分鐘查詢公開狀態，保留提醒與 Widget；既有關閉設定不變。已移除互動及訪客識別碼。Grok 程式碼與舊資料保留但不執行，舊選擇轉為 Codex。

完整說明：[English](README.md) · [简体中文](README_ZH.md) · [安裝與回退](docs/development/install-upgrade.md)。
開發：`swift test`、`swift build`、`swift run CodexRunway`。開發包停用線上更新。

保留原作者署名、[貢獻者](CONTRIBUTORS.md)及 [AGPL-3.0](LICENSE)。發佈原始碼位於對應的 `personal-v*` 標籤。
