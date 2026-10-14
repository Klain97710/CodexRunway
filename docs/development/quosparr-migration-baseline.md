# Personal Go migration baseline

Status: in progress, 2026-10-14. This is not release acceptance.

| Item | Frozen value |
| --- | --- |
| Swift application source | `895e991d67eea3f8500065b605741f42aee54ae3` on `personal/legacy-swift` |
| Approved plan | `08d2cd5` |
| Upstream Go target | `94ce1869d2603593972846baa811672c1c2e903c` (v1.0.3) |
| Integration branch | `personal/migrate-quosparr-v1` |
| Installed Swift binary SHA256 | `e03f40140a67ea635585c96c6a65fd16771198a69632ce2d76d8677d66d84b73` |
| Published 0.1.0 Swift binary SHA256 | `d40954a0f866765745441c431feee0381769ada8c249c3a18a6ced7f4bed2175` |

Both Swift applications and exported preferences were backed up in the private release directory under `backups/20261014-093347-pre-go-migration`. Both app signatures passed `codesign --verify --deep --strict`. No real account files were read for this baseline. The installed app remains unchanged.

The upstream merge preserves history and adopts the Go architecture. Removed Swift main-app sources/tests remain available on the stable branch. Personal documentation is retained. Release automation now accepts only personal tags and creates drafts with latest=false after checks. Signing secrets are scoped to the candidate build step; branch CI stays read-only and secret-free. CI currently runs Go checks on an isolated GitHub runner without launching the desktop app.

## Validation gates

| Gate | Status | Evidence / outstanding work |
| --- | --- | --- |
| G0 Baseline and isolation branch | PASS | Pinned commits, separate worktree, two signed Swift backups |
| G1 Isolated developer runtime | PARTIAL | Go sandbox/app tests, go build and 4 packaging-policy tests PASS; packaged desktop acceptance pending |
| G2 Identity/data/lock compatibility | PARTIAL | platform/store/netx/codex/migration tests PASS; preserved identity, in-place migration/auth immutability and shared lock tested; pinned Swift → Go → Swift preferences/index/SQLite reuse PASS; real desktop/Keychain acceptance pending |
| G3 Codex only and no reactions | PARTIAL | App/runway/store/export/reset tests PASS; repeated refresh/ticks prove zero Grok calls with positive Codex/status controls; cookie/reaction transport test PASS; desktop acceptance pending |
| G4 Tray, actions and Widget | PARTIAL | Go app/runway/export regressions and Go/Swift contract checks PASS; arm64 isolated package/signatures PASS; desktop and saved Widget refresh pending |
| G5 Personal update trust | PARTIAL | URL/redirect/proxy tests, official Sparkle signed/unsigned/tampered XML and ZIP tests, draft-command tests PASS; final dual-architecture signatures and real installer acceptance pending |
| G6 Automated tests and packaging | FAIL | All automated checks and archive/signature checks PASS, but Mach-O host requires macOS 13 while config advertises 12; corrected strict gate blocks release pending compatibility decision |
| G7 Real two-generation update | NOT RUN | Disposable macOS test environment required |
| G8 Platform/rollback acceptance | NOT RUN | macOS 12/13, Intel, Widget and rollback evidence required |

Task 2 source isolation validated: default builds require a private marked sandbox; UserDefaults is replaced by sandbox JSON, credentials by memory, notifications by an unsupported backend, and background work by offline sources. External CODEX_HOME/GROK_HOME and symlinks are rejected. Replacement builds require an explicit profile and build tag. Full package validation remains part of G6.

Task 3 restores original preference domain, data and lock paths, proxy Keychain service, account export identity, app name/scheme and personal 0.2.0/2000 version. Upstream directory-copy implementation remains testable with an explicit renamed target; personal installation uses no copy. Existing Swift fixtures remain unchanged (account export assertion permits only the deliberate original format name).

Task 4 removes reaction transport, visitor store, UI state, buttons, polling and dedicated tests. Public status uses a cookie-free client, including injected clients. Old visitor files remain untouched. Grok implementations and low-level regression tests remain with explicit feature opt-in; the default runtime never constructs them. Legacy provider choices normalize to Codex while unrelated preferences are retained.

Task 5 restores remaining-quota columns, Pro-only standard 5h filtering, action-only context menu and a separate persistent details window. Restored Widget kind/scheme/group/path contracts; old provider values remain decodable and render Codex. The migration-dev arm64 app, SQLite, bridge and Widget were packaged and signature-verified without launching the app. This is build evidence, not desktop acceptance.

Task 6 adds a separate mygo key (private key outside Git, GitHub secret configured), retains the original Sparkle key, and signs the legacy XML feed as well as its ZIP. mygo v0.4.8 signs the archive SHA256; its JSON metadata is HTTPS-served and is not independently signed. The runtime validates every origin/redirect against the personal repository and its numeric CDN namespace, strips request credentials, and preserves proxy failures. Release automation creates drafts only; real two-generation install tests remain G7.

Task 7: 1,756 Go test cases/subtests and 1,742 race cases/subtests passed, plus vet/build/development-update and Python release checks. Optional UI export and real Keychain probes were not counted as passes; two retained Grok date deviations are inherited upstream skips. Separate offscreen tray exports covered all seven languages and both themes. The pinned Swift Core probe verifies Swift → Go → Swift preferences and account index equality, plus both Swift-created and Go-created SQLite reuse without rebuilding, in Asia/Shanghai with exact token/cost totals. No desktop entry point or real auth was used.

Both arm64 and x86_64 final bundles, ZIP/tar/DMG equality, mygo archive signatures, Sparkle ZIP and XML signatures were checked locally. **Those packages are diagnostic candidates, not accepted releases:** an added `vtool` check found the executable's `LC_BUILD_VERSION minos=13.0`, while Info.plist advertises 12.0. Go 1.27 officially requires macOS 13 ([release notes](https://go.dev/doc/go1.27#darwin)); merely rewriting its minimum does not restore support. The new strict packaging check rejects that mismatch. Compatibility scope is awaiting the user's decision; published 0.1.0 and the installed app remain untouched.

Desktop inventory: Parallels is installed, but its registered guests are Windows only; no disposable macOS guest was available. Intel native, macOS minimum-version, Widget refresh and real two-generation updates remain NOT RUN.
