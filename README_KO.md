# CodexRunway · Klain97710 맞춤 버전

[Licoy/CodexRunway](https://github.com/Licoy/CodexRunway)를 기반으로 한 지인용 **0.1.0 (1000)** 버전입니다. Codex만 활성화하며 한국어 UI를 지원합니다.

## 설치 및 업데이트

1. [이 저장소의 Releases](https://github.com/Klain97710/CodexRunway/releases/latest)에서 DMG를 받으세요. Apple Silicon은 arm64, Intel은 x86_64를 선택합니다.
2. 기존 앱을 종료하고 앱과 설정을 백업한 뒤 같은 위치에 덮어씁니다. 계정, Keychain, Widget 식별자와 로컬 데이터는 유지됩니다.
3. Homebrew 사용자는 먼저 `brew uninstall --cask codex-runway`를 실행하세요. `--zap`은 사용하지 마세요. 이후 이 버전을 수동 설치합니다.
4. 앱은 macOS 12+, Widget은 macOS 14+가 필요합니다. 무료 ad-hoc 서명이며 Apple 공증은 없습니다. 처음 실행할 때 우클릭 → 열기 또는 시스템 개인정보 보호 및 보안 설정을 사용하세요.
5. 메뉴 막대 아이콘을 열고 계정 설정에서 현재 로그인 가져오기나 브라우저 로그인을 선택한 뒤 할당량을 새로 고칩니다.

처음 수동 설치한 후에는 이 저장소의 서명된 Sparkle 소스에서만 업데이트합니다. 설치 전에 사용자 확인을 받으며 실패하면 기존 앱을 보존합니다. 되돌리려면 앱을 종료하고 백업 앱을 복원하세요. 계정과 세션 데이터는 삭제하지 않습니다.

리셋 상태는 기본으로 켜지며 5분마다 공개 상태를 조회합니다. 알림과 Widget은 유지하고 이전에 꺼 둔 설정도 존중합니다. 반응 기능과 방문자 식별자는 제거했습니다. Grok 코드와 기존 파일은 보존하지만 실행하지 않으며 이전 선택은 Codex로 바뀝니다.

전체 안내: [English](README.md) · [简体中文](README_ZH.md) · [설치 및 복원](docs/development/install-upgrade.md#english).
개발 명령: `swift test`, `swift build`, `swift run CodexRunway`. 개발 패키지는 온라인 업데이트를 비활성화합니다.

원저자 표시, [기여자](CONTRIBUTORS.md), [AGPL-3.0](LICENSE)을 유지합니다. 각 릴리스 소스는 `personal-v*` 태그에 있습니다.
