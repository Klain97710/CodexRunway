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

The upstream merge preserves history and adopts the Go architecture. Removed Swift main-app sources/tests remain available on the stable branch. Personal documentation is retained. Release automation is fail-closed while signing and validation are ported; it has no publish step or signing secrets. CI currently runs Go checks on an isolated GitHub runner without launching the desktop app.

## Validation gates

| Gate | Status | Evidence / outstanding work |
| --- | --- | --- |
| G0 Baseline and isolation branch | PASS | Pinned commits, separate worktree, two signed Swift backups |
| G1 Isolated developer runtime | PARTIAL | Go sandbox/app tests, go build and 4 packaging-policy tests PASS; packaged desktop acceptance pending |
| G2 Identity/data/lock compatibility | PARTIAL | platform/store/netx/codex/migration tests PASS; preserved identity, in-place migration/auth immutability and shared lock tested; Swift write-back acceptance pending |
| G3 Codex only and no reactions | PARTIAL | App/runway/store/export/reset tests PASS; repeated refresh/ticks prove zero Grok calls with positive Codex/status controls; cookie/reaction transport test PASS; desktop acceptance pending |
| G4 Tray, actions and Widget | PARTIAL | Go app/runway/export regressions and Go/Swift contract checks PASS; arm64 isolated package/signatures PASS; desktop and saved Widget refresh pending |
| G5 Personal update trust | NOT RUN | Both signed feeds and mygo archives required |
| G6 Automated tests and packaging | NOT RUN | Exact candidate SHA and artifacts required |
| G7 Real two-generation update | NOT RUN | Disposable macOS test environment required |
| G8 Platform/rollback acceptance | NOT RUN | macOS 12/13, Intel, Widget and rollback evidence required |

Task 2 source isolation validated: default builds require a private marked sandbox; UserDefaults is replaced by sandbox JSON, credentials by memory, notifications by an unsupported backend, and background work by offline sources. External CODEX_HOME/GROK_HOME and symlinks are rejected. Replacement builds require an explicit profile and build tag. Full package validation remains part of G6.

Task 3 restores original preference domain, data and lock paths, proxy Keychain service, account export identity, app name/scheme and personal 0.2.0/2000 version. Upstream directory-copy implementation remains testable with an explicit renamed target; personal installation uses no copy. Existing Swift fixtures remain unchanged (account export assertion permits only the deliberate original format name).

Task 4 removes reaction transport, visitor store, UI state, buttons, polling and dedicated tests. Public status uses a cookie-free client, including injected clients. Old visitor files remain untouched. Grok implementations and low-level regression tests remain with explicit feature opt-in; the default runtime never constructs them. Legacy provider choices normalize to Codex while unrelated preferences are retained.

Task 5 restores remaining-quota columns, Pro-only standard 5h filtering, action-only context menu and a separate persistent details window. Restored Widget kind/scheme/group/path contracts; old provider values remain decodable and render Codex. The migration-dev arm64 app, SQLite, bridge and Widget were packaged and signature-verified without launching the app. This is build evidence, not desktop acceptance.
