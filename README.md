English · [简体中文](./README_ZH.md) · [繁體中文](./README_ZH_HANT.md) · [한국어](./README_KO.md) · [日本語](./README_JA.md) · [Русский](./README_RU.md) · [Français](./README_FR.md)

<p align="center"><img src="Resources/AppIcon.png" alt="CodexRunway" width="128" height="128"></p>

# CodexRunway · Klain97710 custom edition

A native macOS menu bar companion for Codex, customized for friends and family. Based on [Licoy/CodexRunway](https://github.com/Licoy/CodexRunway). First release: **0.1.0 (1000)**.

This edition **replaces the original app**. It keeps the `CodexRunway.app` name, app and Widget identifiers, URL scheme, Keychain services and `~/.codex-runway` data. Install it manually once; subsequent updates use this repository's signed Sparkle channel.

## Install and sign in

1. Download a DMG from [this repository's Releases](https://github.com/Klain97710/CodexRunway/releases/latest): `CodexRunway-macos-arm64.dmg` for Apple Silicon or `CodexRunway-macos-x86_64.dmg` for Intel. The app requires macOS 12+; Widgets require macOS 14+.
2. Quit any running CodexRunway. Existing users should back up the old app and preferences, then replace the app at its **existing location**. New users can drag it to Applications.
3. If installed with Homebrew, first run `brew uninstall --cask codex-runway` **without `--zap`**, then install this edition manually. Do not reinstall the upstream cask over this edition.
4. Packages are ad-hoc signed and not Apple notarized. For the first launch, use right-click → Open or System Settings → Privacy & Security → Open Anyway. Verify the download source and SHA256 before handling a security prompt.
5. Click the menu bar item. Existing accounts remain available. New users can open Accounts in settings and import the current local login, sign in through the browser, import a file or paste credentials. Refresh to see quota. API-key accounts do not expose ChatGPT subscription quota.

There is no Dock icon. If the menu bar item is hidden, open the panel with:

```bash
open -a /Applications/CodexRunway.app 'codex-runway://widget?provider=codex&section=overview'
```

Use the matching path for an installation in `~/Applications`. Login-at-startup follows macOS authorization; existing preferences are preserved. See [replacement, backup and rollback instructions](docs/development/install-upgrade.md#english).

## Features

- Codex browser login, multiple accounts, imports, aliases, explicit account switching and quota refresh.
- Five-hour, weekly and additional quota windows; official reset credits, quota estimates and alerts.
- API-equivalent cost, Token heatmap / line / bar charts, recent sessions and local session-index repair.
- Public Codex reset status, alerts, source links and Widget. Enabled by default with a five-minute interval; previously disabled preferences stay disabled.
- macOS 14+ Widgets for quota, Token trends, key metrics and reset status.
- Seven UI languages, light / dark / system appearance, system / HTTP / SOCKS5 proxy settings.
- Concurrent local statistics and network refresh, visible failures with original data timestamps, and opt-in redacted network diagnostics.

Grok implementation, serialized types and tests remain in the source. Production has no enable switch and performs no Grok CLI probing, credential reads, session scans or network requests. Legacy Grok / Both preferences and Widget configurations resolve to Codex; unrelated preferences and existing files remain intact.

Reset-status reactions, counts, polling and visitor-identifier storage are removed.

## Updates and rollback

Automatic checking is enabled by default. Downloads and installation require user confirmation. Home, feedback, allowed download URLs and appcasts point to `Klain97710/CodexRunway`.

The first replacement of the upstream edition must be manual. Invalid signatures, damaged downloads and proxy failures preserve the current application and never switch to the upstream update source. Development packages disable online updates; release packaging requires a valid public key, and publishing verifies that the private key matches it.

To roll back, quit the app and restore the backed-up application at the same path. Restore preferences only if needed; keep account and session files in place. Restoring the upstream application also restores its update channel.

Releases contain DMG, ZIP, app.tar.gz, signed appcasts, SHA256SUMS, release notes and a corresponding source tag. The first tag is `personal-v0.1.0`.

## Network and privacy

Configure the proxy in Control Panel → General → Network, then save. Connection testing reads this repository's public page, so it works before the first release exists and sends no account credentials. Failed custom proxies never silently fall back to a direct connection. Proxy passwords use Keychain.

- Codex credentials are read from `~/.codex/auth.json`. Managed copies use `~/.codex-runway/accounts/<id>/auth.json` with 0700 directories and 0600 files; account indexes contain no tokens.
- Installation migration does not write the official authentication file. Existing OAuth refresh and explicit account-switch behavior remains; non-current managed accounts refresh only their own copies.
- Tokens, API keys and authentication JSON are excluded from logs and release packages. Session contents are not uploaded. Derived caches live under `~/.codex-runway`.
- Session repair backs up and rebuilds only `~/.codex/session_index.jsonl`; it does not delete sessions.
- Reset status requests only the [Did Codex Reset public feed](https://didcodexreset.com/api/status.json), without account details, tokens, sessions or visitor identifiers. No `/api/reaction` requests are sent. Old visitor files are left unused. This third-party AI-derived status can be late or unavailable.
- Quota, reset credits, estimates and official Token statistics use ChatGPT / Codex endpoints. Official statistics describe the current account; local logs can span accounts. The two measures cannot be directly subtracted.
- Quota estimates extrapolate Credits and weekly utilization (1000 Credits ≈ $40; rule version `credits-usd-2026-08-26`) and are not an official guarantee. API-equivalent costs use bundled and online prices; unknown models do not get precise estimates.
- Widgets read the 0600 `~/.codex-runway/widget-snapshot.json` file. Only Codex derived data is published, without email addresses, account IDs or credentials.

## Development and packaging

Use Swift 6 and Xcode. Local validation uses Xcode 16.4 / Swift 6.1.2. Quit the installed application before launching development builds.

```bash
swift test
swift build
swift run CodexRunway
swift run CodexRunway --self-check
ARCH=arm64 bash Scripts/package-app.sh
bash Scripts/verify-packaged-app.sh dist/CodexRunway.app arm64 local
```

On macOS 14+, development launch assembles a separate Dev app with its Widget. It still shares the single-instance lock and account directory; a development identifier does not isolate user data. Self-check prints only redacted local Codex diagnostics.

Default packages disable updates. See [release maintenance](docs/development/personal-release.md), [installation and rollback](docs/development/install-upgrade.md#english), and [first-release verification](docs/development/personal-v0.1.0-verification.md).

`personal/main` is the integration and release branch; `main` retains the upstream baseline. Only `personal-v*` tags trigger release drafts. Review artifacts before publishing, and recheck distribution configuration, feature guards and preference migration when merging upstream changes.

## Attribution and license

Original project: [Licoy/CodexRunway](https://github.com/Licoy/CodexRunway). Custom edition: [Klain97710/CodexRunway](https://github.com/Klain97710/CodexRunway). Original attribution, [contributors](CONTRIBUTORS.md) and the [AGPL-3.0 license](LICENSE) are preserved. Matching source is available under each release tag.
