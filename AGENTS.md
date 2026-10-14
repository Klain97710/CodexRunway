# CodexRunway personal Go migration

## Scope and baselines

- This branch migrates the Klain97710 CodexRunway edition onto upstream Quosparr Go + mygo/ui. It is not an accepted daily replacement yet.
- Follow `docs/superpowers/plans/2026-10-14-quosparr-migration-validation.md`; retain Swift stable code on `personal/legacy-swift` and keep `personal/main` stable until acceptance.
- User-required behavior: Codex only; no Grok runtime or reset reactions; remaining-quota tray columns `5h | bar | %`, `7d | bar | %`; Pro hides standard 5h in the tray only; action-only context menu.
- Work sequentially in the main agent. No subagents. Independent read-only tools may run in parallel.

## Architecture and compatibility

- Go 1.27.1 and mygo v0.4.8 are pinned in go.mod. Do not combine dependency upgrades with this migration or add a frontend framework.
- Main app: Go + native mygo/ui; macOS 13+ (user-approved 2026-10-14; macOS 12 stays on Swift). Swift WidgetKit and bridge remain under Sources/QuosparrWidget*; Widget requires macOS 14+.
- Keep LSUIElement, old app identity, URL scheme, Widget kinds, Keychain service, preference domain, data directory and lock for replacement builds. Source/module names may remain upstream names.
- Centralize personal configuration in internal/personal and mygo.json (product version source). Developer builds require a separate identity and explicitly marked sandbox.
- Keep Swift fixture assets read-only; validate compatibility both ways. Preserve unknown fields and never reset damaged data to defaults and overwrite it.
- Product strings support en, zh-Hans, zh-Hant, ko, ja, ru, fr. Keep Go and Swift snapshot contracts compatible.

## Data and execution safety

- Never log or commit tokens, keys, credentials, account identifiers or full auth documents. Tests use synthetic fixtures.
- Default migration validation must not launch against real user data. On 2026-10-14 the user explicitly authorized replacing their local arm64 app with 0.2.0/2002 and normal launch verification; that installation is complete. This exception does not authorize failure tests against real accounts. A temporary HOME does not isolate Keychain/UserDefaults; full GUI/failure acceptance still requires a disposable VM or dedicated test user.
- The user-authorized local install now runs Go 0.2.0/2002; a verified Swift app/data/preferences backup is retained outside Git. Preserve that rollback point. No real auth in failure tests; no system proxy/DNS changes on this host.
- Production application data remains ~/.codex-runway; credential directories 0700, auth files and snapshots 0600, atomic writes. Acquire the legacy-compatible single lock before migrations or writes; skip upstream directory renaming when source and destination coincide.
- Official Codex auth changes are limited to legitimate OAuth refresh or user-requested account switching. Non-current account refresh only writes the managed copy; reject stale writes after external switch/logout.
- Session repair only edits session_index.jsonl after backup; never delete session directories.
- Development packages disable updates, login items, system credential access and real notifications. Retained Grok code requires explicit test opt-in, never a user setting.

## Builds and releases

- Never run upstream development/packaging scripts against production until adapted to the approved isolation profile.
- Only personal-v* tags may trigger releases. Branch CI has read-only permissions, no signing secrets, and creates artifacts only.
- All releases are drafts with latest=false until separately accepted. Never overwrite personal-v0.1.0 or published assets.
- Own repository only: Klain97710/CodexRunway. No fallback to upstream updates. Validate initial URLs, manifest assets, redirects and signature errors.
- Preserve the existing Sparkle key and sign both XML feed and ZIP for installed Swift clients; Go uses a separate mygo key. Do not embed Sparkle runtime in Go.
- Bundle SQLite, Swift bridge and Widget, sign inside-out, then generate archives from the final bundle. Cross-compilation and registration do not prove native Intel/minimum-macOS/Widget acceptance.
- Record PASS/FAIL/NOT RUN honestly. Missing VM/hardware blocks replacement acceptance, not source work and candidate builds.

## Checks

Use go test, go vet, go build and race tests plus personal packaging/release checks in the plan. Audit tests before host execution. Never run the desktop entry point as a shortcut for isolated tests. Keep generated builds, signing keys and local screenshots out of Git.
