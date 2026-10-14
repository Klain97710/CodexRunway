[English](./upgrade-from-codexrunway.md) · [简体中文](./upgrade-from-codexrunway_ZH.md) · [繁體中文](./upgrade-from-codexrunway_ZH_HANT.md) · [한국어](./upgrade-from-codexrunway_KO.md) · 日本語 · [Русский](./upgrade-from-codexrunway_RU.md) · [Français](./upgrade-from-codexrunway_FR.md)

# CodexRunway からのアップグレード

新しい識別子は `com.quosparr.app`、`quosparr://`、`~/.quosparr` であるため、旧アプリが自分自身を Quosparr に更新することはできません。CodexRunway を終了し、Quosparr を手動でインストールして起動してください。

- 初回起動時に、`~/.codex-runway` のデータがコピーされ、不足している設定が取り込まれます。旧ディレクトリはそのまま残り、移行が中断されても安全に再開されます。
- 旧名称でエクスポートしたアカウントバンドルは引き続きインポートできます。新しいエクスポートは `quosparr-accounts` を使います。
- システム権限は引き継がれません。デスクトップウィジェットを再度追加し、ログイン項目を確認してください。macOS では、古いログイン項目をシステム設定 → 一般 → ログイン項目から手動で削除する必要がある場合があります。
- 移行によって、公式の Codex / Grok のログインファイルが変更されることはありません。
