English · [简体中文](./upgrade-from-codexrunway_ZH.md) · [繁體中文](./upgrade-from-codexrunway_ZH_HANT.md) · [한국어](./upgrade-from-codexrunway_KO.md) · [日本語](./upgrade-from-codexrunway_JA.md) · [Русский](./upgrade-from-codexrunway_RU.md) · [Français](./upgrade-from-codexrunway_FR.md)

# Upgrading from CodexRunway

The new identity is `com.quosparr.app`, `quosparr://`, and `~/.quosparr`, so the old app cannot update itself into Quosparr. Quit CodexRunway, install Quosparr manually, and launch it:

- On first launch, data from `~/.codex-runway` and missing preferences are copied in. The old directory is kept, and the migration resumes safely if interrupted.
- Account bundles exported by the old name can still be imported; new exports use `quosparr-accounts`.
- System permissions are not inherited. Add desktop widgets again and check the login item. On macOS, an old login item may need to be removed manually in System Settings → General → Login Items.
- Migration never modifies the official Codex / Grok login files.
