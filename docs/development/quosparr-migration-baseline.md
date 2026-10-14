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
| G1 Isolated developer runtime | NOT RUN | Implement sandbox/profile guards before launch |
| G2 Identity/data/lock compatibility | NOT RUN | Synthetic round-trip and lock tests required |
| G3 Codex only and no reactions | NOT RUN | Runtime counters required |
| G4 Tray, actions and Widget | NOT RUN | Code and isolated desktop validation required |
| G5 Personal update trust | NOT RUN | Both signed feeds and mygo archives required |
| G6 Automated tests and packaging | NOT RUN | Exact candidate SHA and artifacts required |
| G7 Real two-generation update | NOT RUN | Disposable macOS test environment required |
| G8 Platform/rollback acceptance | NOT RUN | macOS 12/13, Intel, Widget and rollback evidence required |
