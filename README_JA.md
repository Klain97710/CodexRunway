[English](./README.md) · [简体中文](./README_ZH.md) · [繁體中文](./README_ZH_HANT.md) · [한국어](./README_KO.md) · 日本語 · [Русский](./README_RU.md) · [Français](./README_FR.md)

# CodexRunway · Klain97710 カスタム版

[Licoy/CodexRunway](https://github.com/Licoy/CodexRunway) を基にした身近な人向けの **0.1.0 (1000)** です。Codex のみ有効で、日本語 UI を引き続き利用できます。

## インストールと更新

1. [このリポジトリの Releases](https://github.com/Klain97710/CodexRunway/releases/latest) から DMG を取得します。Apple Silicon は arm64、Intel は x86_64 です。
2. 元のアプリを終了し、アプリと設定をバックアップしてから同じ場所に上書きします。アカウント、Keychain、Widget の識別子とローカルデータを引き継ぎます。
3. Homebrew 利用者は先に `brew uninstall --cask codex-runway` を実行し、`--zap` は付けずに本版を手動で入れます。
4. アプリは macOS 12+、Widget は macOS 14+ が必要です。無料の ad-hoc 署名で Apple 公証はありません。初回は右クリックの「開く」またはシステムの「プライバシーとセキュリティ」から許可してください。
5. メニューバーからアカウント設定を開き、現在のログインを取り込むかブラウザでログインして、クォータを更新します。

初回の手動置き換え後は、このリポジトリの署名付き Sparkle 更新元だけを使用します。インストールは利用者の確認後に行い、失敗時は現在のアプリを保ちます。戻す場合は終了後にバックアップしたアプリを復元し、アカウントやセッションを削除しないでください。

リセット状況は初期設定で有効、5分間隔です。通知と Widget を保ち、既に無効にした設定も尊重します。リアクションと訪問者識別子は削除しました。Grok のコードと既存ファイルは残しますが実行せず、以前の選択は Codex に変換します。

詳しい説明: [English](README.md) · [简体中文](README_ZH.md) · [インストールと復元](docs/development/install-upgrade.md#english)。
開発: `swift test`、`swift build`、`swift run CodexRunway`。開発パッケージではオンライン更新が無効です。

原作者の表記、[貢献者](CONTRIBUTORS.md)、[AGPL-3.0](LICENSE) を維持します。各版のソースは `personal-v*` タグから取得できます。
