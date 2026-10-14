[English](./upgrade-from-codexrunway.md) · [简体中文](./upgrade-from-codexrunway_ZH.md) · [繁體中文](./upgrade-from-codexrunway_ZH_HANT.md) · [한국어](./upgrade-from-codexrunway_KO.md) · [日本語](./upgrade-from-codexrunway_JA.md) · [Русский](./upgrade-from-codexrunway_RU.md) · Français

# Passer de CodexRunway à Quosparr

La nouvelle identité est `com.quosparr.app`, `quosparr://` et `~/.quosparr` ; l’ancienne app ne peut donc pas se mettre à jour vers Quosparr. Quittez CodexRunway, installez Quosparr manuellement et lancez-le :

- Au premier lancement, les données de `~/.codex-runway` et les préférences manquantes sont copiées. L’ancien répertoire est conservé, et la migration reprend sans risque si elle est interrompue.
- Les lots de comptes exportés sous l’ancien nom peuvent toujours être importés ; les nouveaux exports utilisent `quosparr-accounts`.
- Les autorisations système ne sont pas héritées. Ajoutez de nouveau les widgets bureau et vérifiez l’élément de connexion. Sous macOS, un ancien élément de connexion devra peut-être être retiré manuellement dans Réglages Système → Général → Éléments de connexion.
- La migration ne modifie jamais les fichiers de connexion officiels de Codex / Grok.
