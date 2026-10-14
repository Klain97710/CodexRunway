[English](./upgrade-from-codexrunway.md) · [简体中文](./upgrade-from-codexrunway_ZH.md) · [繁體中文](./upgrade-from-codexrunway_ZH_HANT.md) · 한국어 · [日本語](./upgrade-from-codexrunway_JA.md) · [Русский](./upgrade-from-codexrunway_RU.md) · [Français](./upgrade-from-codexrunway_FR.md)

# CodexRunway에서 업그레이드

새 식별자는 `com.quosparr.app`, `quosparr://`, `~/.quosparr`이므로 이전 앱은 스스로 Quosparr로 업데이트할 수 없습니다. CodexRunway를 종료하고 Quosparr를 직접 설치한 뒤 실행하세요.

- 처음 실행하면 `~/.codex-runway`의 데이터와 없는 환경설정이 복사됩니다. 이전 디렉터리는 그대로 두며, 중단되어도 마이그레이션을 안전하게 이어 갑니다.
- 이전 이름으로 내보낸 계정 번들도 가져올 수 있으며, 새로 내보내는 파일은 `quosparr-accounts`를 사용합니다.
- 시스템 권한은 상속되지 않습니다. 데스크톱 위젯을 다시 추가하고 로그인 항목을 확인하세요. macOS에서는 시스템 설정 → 일반 → 로그인 항목에서 이전 로그인 항목을 직접 제거해야 할 수 있습니다.
- 마이그레이션은 공식 Codex / Grok 로그인 파일을 수정하지 않습니다.
