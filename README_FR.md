[English](./README.md) · [简体中文](./README_ZH.md) · [繁體中文](./README_ZH_HANT.md) · [한국어](./README_KO.md) · [日本語](./README_JA.md) · [Русский](./README_RU.md) · Français

# CodexRunway · Édition personnalisée Klain97710

Version **0.1.0 (1000)** destinée aux proches, basée sur [Licoy/CodexRunway](https://github.com/Licoy/CodexRunway). Seul Codex est activé. L’interface française reste disponible.

## Installation et mises à jour

1. Téléchargez le DMG depuis [les Releases de ce dépôt](https://github.com/Klain97710/CodexRunway/releases/latest) : arm64 pour Apple Silicon, x86_64 pour Intel.
2. Quittez l’application d’origine, sauvegardez l’application et ses préférences, puis remplacez-la au même emplacement. Comptes, trousseau, identifiants Widget et données locales sont conservés.
3. Avec Homebrew, exécutez d’abord `brew uninstall --cask codex-runway`, sans `--zap`, puis installez cette édition manuellement.
4. L’application nécessite macOS 12+, les Widgets macOS 14+. Les paquets ont une signature ad-hoc gratuite, sans notarisation Apple. Au premier lancement, utilisez clic droit → Ouvrir ou les réglages Confidentialité et sécurité.
5. Ouvrez l’icône de la barre des menus, importez la connexion locale ou connectez-vous dans le navigateur depuis les réglages de comptes, puis actualisez le quota.

Après le premier remplacement manuel, seul le flux Sparkle signé de ce dépôt est utilisé. L’installation demande votre confirmation. En cas d’échec, l’application actuelle reste intacte. Pour revenir en arrière, quittez l’application et restaurez sa sauvegarde sans supprimer les comptes ni les sessions.

Le statut public des réinitialisations est activé par défaut, toutes les cinq minutes, avec alertes et Widget. Un ancien réglage désactivé reste désactivé. Les réactions et identifiants visiteurs ont été supprimés. Le code Grok et les fichiers existants sont conservés sans être utilisés ; les anciens choix deviennent Codex.

Guide complet : [English](README.md) · [简体中文](README_ZH.md) · [Installation et restauration](docs/development/install-upgrade.md#english).
Développement : `swift test`, `swift build`, `swift run CodexRunway`. Les paquets de développement désactivent les mises à jour en ligne.

Attribution originale, [contributeurs](CONTRIBUTORS.md) et licence [AGPL-3.0](LICENSE) conservés. Les sources de chaque version sont sous les tags `personal-v*`.
