package app

// Texts of the system integration (updates, notifications) that the Swift
// tables do not have. They join extraTexts, so the completeness test of
// extra_l10n_test.go covers them: all seven languages, the same arguments.
const (
	xUpdateUnavailableTitle    textKey = "updateUnavailableTitle"
	xUpdateUnavailableDev      textKey = "updateUnavailableDev"
	xUpdateUnavailablePackage  textKey = "updateUnavailablePackage"
	xUpdateUnavailableLocation textKey = "updateUnavailableLocation"
	xUpdateNetworkBlocked      textKey = "updateNetworkBlocked"
	xUpdateOpenDownloadPage    textKey = "updateOpenDownloadPage"
	xUpdateClose               textKey = "updateClose"
	xNotifyRequested           textKey = "notifyRequested"
	xNotifyUnsupported         textKey = "notifyUnsupported"
	xNotifyDenied              textKey = "notifyDenied"
	xNotifyFailed              textKey = "notifyFailed"
)

func init() {
	for k, row := range systemTexts {
		extraTexts[k] = row
	}
}

// systemTexts is indexed like extraTexts: English, Simplified Chinese,
// Traditional Chinese, Korean, Japanese, Russian, French.
var systemTexts = map[textKey][7]string{
	xUpdateUnavailableTitle: {
		"Updates are unavailable", "无法更新", "無法更新", "업데이트할 수 없습니다",
		"アップデートできません", "Обновление недоступно", "Mise à jour impossible",
	},
	xUpdateUnavailableDev: {
		"Online updates are disabled in this development build, including CI packages. Install a newer build manually, or use a release with signed updates enabled.",
		"此开发版本未启用在线更新（包括 CI 开发包），请手动安装较新的构建，或使用启用了签名更新的正式发布版。",
		"此開發版本未啟用線上更新（包括 CI 開發套件），請手動安裝較新的組建，或使用啟用了簽章更新的正式發佈版。",
		"CI 패키지를 포함한 이 개발 빌드에서는 온라인 업데이트가 비활성화되어 있습니다. 새 빌드를 수동으로 설치하거나 서명된 업데이트가 활성화된 정식 릴리스를 사용하세요.",
		"CI パッケージを含むこの開発ビルドでは、オンライン更新が無効です。新しいビルドを手動でインストールするか、署名付き更新が有効な正式版を使用してください。",
		"В этой сборке для разработки, включая пакеты CI, онлайн-обновления отключены. Установите новую сборку вручную или используйте выпуск с подписанными обновлениями.",
		"Les mises à jour en ligne sont désactivées dans cette version de développement, y compris les paquets CI. Installez manuellement une version plus récente ou utilisez une version publiée avec mises à jour signées.",
	},
	xUpdateUnavailablePackage: {
		"This copy of Quosparr is managed by a package manager (for example a .deb package). Update it with the package manager, or download the latest version from the release page.",
		"此 Quosparr 由包管理器管理（例如 .deb 包）。请使用包管理器更新，或从发布页面下载最新版本。",
		"此 Quosparr 由套件管理器管理（例如 .deb 套件）。請使用套件管理器更新，或從發佈頁面下載最新版本。",
		"이 Quosparr는 패키지 관리자(예: .deb 패키지)로 관리됩니다. 패키지 관리자로 업데이트하거나 릴리스 페이지에서 최신 버전을 내려받으세요.",
		"この Quosparr はパッケージマネージャ（例: .deb パッケージ）で管理されています。パッケージマネージャで更新するか、リリースページから最新版をダウンロードしてください。",
		"Эта копия Quosparr управляется менеджером пакетов (например, пакетом .deb). Обновите её через менеджер пакетов или скачайте последнюю версию со страницы релизов.",
		"Cette copie de Quosparr est gérée par un gestionnaire de paquets (par exemple un paquet .deb). Mettez-la à jour avec celui-ci, ou téléchargez la dernière version depuis la page des versions.",
	},
	xUpdateUnavailableLocation: {
		"Quosparr cannot write to the folder it is installed in (for example a read-only volume or a system folder), so it cannot update itself. Download the latest version from the release page and install it yourself.",
		"Quosparr 无法写入其安装所在的文件夹（例如只读卷或系统目录），因此无法自行更新。请从发布页面下载最新版本并自行安装。",
		"Quosparr 無法寫入其安裝所在的資料夾（例如唯讀磁碟區或系統目錄），因此無法自行更新。請從發佈頁面下載最新版本並自行安裝。",
		"Quosparr가 설치된 폴더에 쓸 수 없어(예: 읽기 전용 볼륨 또는 시스템 폴더) 스스로 업데이트할 수 없습니다. 릴리스 페이지에서 최신 버전을 내려받아 직접 설치하세요.",
		"Quosparr はインストール先のフォルダに書き込めない（読み取り専用ボリュームやシステムフォルダなど）ため、自動更新できません。リリースページから最新版をダウンロードして手動でインストールしてください。",
		"Quosparr не может записывать в папку установки (например, том только для чтения или системная папка) и не может обновить себя. Скачайте последнюю версию со страницы релизов и установите её вручную.",
		"Quosparr ne peut pas écrire dans son dossier d'installation (par exemple un volume en lecture seule ou un dossier système) et ne peut donc pas se mettre à jour. Téléchargez la dernière version depuis la page des versions et installez-la vous-même.",
	},
	xUpdateNetworkBlocked: {
		"Your network proxy settings cannot be used right now, so the update check was not sent. It will not connect directly instead. Fix the proxy in Settings, or download the latest version from the release page.",
		"当前无法使用你的网络代理设置，因此未发送更新检查，也不会改为直接连接。请在设置中修复代理，或从发布页面下载最新版本。",
		"目前無法使用你的網路代理設定，因此未發送更新檢查，也不會改為直接連線。請在設定中修復代理，或從發佈頁面下載最新版本。",
		"지금은 네트워크 프록시 설정을 사용할 수 없어 업데이트 확인을 보내지 않았으며 직접 연결로 바꾸지도 않습니다. 설정에서 프록시를 고치거나 릴리스 페이지에서 최신 버전을 내려받으세요.",
		"現在ネットワークプロキシ設定を使えないため、更新の確認は送信されず、直接接続に切り替えることもしません。設定でプロキシを修正するか、リリースページから最新版をダウンロードしてください。",
		"Сейчас настройки прокси недоступны, поэтому проверка обновлений не отправлена и прямого подключения не будет. Исправьте прокси в настройках или скачайте последнюю версию со страницы релизов.",
		"Vos réglages de proxy sont inutilisables pour le moment : la vérification n'a pas été envoyée et ne se fera pas en connexion directe. Corrigez le proxy dans les réglages ou téléchargez la dernière version depuis la page des versions.",
	},
	xUpdateOpenDownloadPage: {
		"Open Download Page", "打开下载页面", "開啟下載頁面", "다운로드 페이지 열기",
		"ダウンロードページを開く", "Открыть страницу загрузки", "Ouvrir la page de téléchargement",
	},
	xUpdateClose: {
		"Close", "关闭", "關閉", "닫기", "閉じる", "Закрыть", "Fermer",
	},
	xNotifyRequested: {
		"Test notification requested. If nothing appears, allow notifications for Quosparr in the system settings.",
		"已请求发送测试提醒。若没有出现，请在系统设置中允许 Quosparr 发送通知。",
		"已請求發送測試提醒。若沒有出現，請在系統設定中允許 Quosparr 發送通知。",
		"테스트 알림을 요청했습니다. 아무것도 표시되지 않으면 시스템 설정에서 Quosparr의 알림을 허용하세요.",
		"テスト通知をリクエストしました。何も表示されない場合は、システム設定で Quosparr の通知を許可してください。",
		"Тестовое уведомление запрошено. Если ничего не появилось, разрешите уведомления для Quosparr в системных настройках.",
		"Notification de test demandée. Si rien n'apparaît, autorisez les notifications de Quosparr dans les réglages système.",
	},
	xNotifyUnsupported: {
		"System notifications are not available here. On macOS they need the packaged Quosparr.app; on Linux they need a running notification service.",
		"此环境无法显示系统通知。macOS 需要运行打包后的 Quosparr.app，Linux 需要正在运行的通知服务。",
		"此環境無法顯示系統通知。macOS 需要執行打包後的 Quosparr.app，Linux 需要正在執行的通知服務。",
		"이 환경에서는 시스템 알림을 표시할 수 없습니다. macOS에서는 패키지된 Quosparr.app이, Linux에서는 실행 중인 알림 서비스가 필요합니다.",
		"この環境ではシステム通知を表示できません。macOS ではパッケージ化された Quosparr.app、Linux では動作中の通知サービスが必要です。",
		"Системные уведомления здесь недоступны. В macOS нужен собранный Quosparr.app, в Linux — запущенная служба уведомлений.",
		"Les notifications système ne sont pas disponibles ici. Sur macOS, il faut le Quosparr.app packagé ; sur Linux, un service de notification actif.",
	},
	xNotifyDenied: {
		"Notifications are turned off for Quosparr. Allow them in the system settings to receive alerts.",
		"Quosparr 的通知已被关闭。请在系统设置中允许通知以接收提醒。",
		"Quosparr 的通知已被關閉。請在系統設定中允許通知以接收提醒。",
		"Quosparr의 알림이 꺼져 있습니다. 알림을 받으려면 시스템 설정에서 허용하세요.",
		"Quosparr の通知はオフになっています。アラートを受け取るには、システム設定で許可してください。",
		"Уведомления для Quosparr отключены. Разрешите их в системных настройках, чтобы получать оповещения.",
		"Les notifications de Quosparr sont désactivées. Autorisez-les dans les réglages système pour recevoir les alertes.",
	},
	xNotifyFailed: {
		"The notification could not be shown: %@", "无法显示通知：%@", "無法顯示通知：%@",
		"알림을 표시할 수 없습니다: %@", "通知を表示できませんでした: %@",
		"Не удалось показать уведомление: %@", "La notification n'a pas pu être affichée : %@",
	},
}
