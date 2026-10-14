package app

import (
	"slices"

	"github.com/licoy/quosparr/internal/l10n"
)

// textKey names a string that the Swift tables do not have. These are the
// strings the Go interface needs on top of internal/l10n (which is generated
// from the Swift tables and must not be edited by hand). Each has all seven
// languages; TestExtraTextsAreComplete enforces it. When they move into the
// Swift tables they can be deleted here.
type textKey string

const (
	xAccountCount              textKey = "accountCount"
	xAccountCountOne           textKey = "accountCountOne"
	xPrivacyDisplaySection     textKey = "privacyDisplaySection"
	xPrivacyDisplayDescription textKey = "privacyDisplayDescription"
	xProvider                  textKey = "provider"
	xProviderDescription       textKey = "providerDescription"
	xPanelHeight               textKey = "panelHeight"
	xPanelHeightHint           textKey = "panelHeightDescription"
	xMacOnly                   textKey = "settingMacOnly"
	xWidgetMac14               textKey = "settingWidgetMac14"
	xSettingsUnreadable        textKey = "settingsUnreadableTitle"
	xSettingsReadOnly          textKey = "settingsReadOnlyNote"
	xProxyBlocked              textKey = "proxyBlockedNote"
	xSettingsSaveFailed        textKey = "settingsSaveFailed"
	xAccountsComingSoon        textKey = "accountsComingSoon"
	xTrayUnavailable           textKey = "trayUnavailable"
	xLinuxDesktopDependencies  textKey = "linuxDesktopDependencies"
	xLinuxReleaseUnreadable    textKey = "linuxReleaseUnreadable"
	xPanelPlaceholder          textKey = "panelPlaceholder"
	xNotAvailableYet           textKey = "notAvailableYet"
	xProxyLocalSaveWarning     textKey = "proxyPasswordNotStored"
)

// extraTexts is indexed by textKey, then by position in l10n.Languages():
// English, Simplified Chinese, Traditional Chinese, Korean, Japanese,
// Russian, French.
var extraTexts = map[textKey][7]string{
	xAccountCount: {
		"%@ accounts", "%@ 个账号", "%@ 個帳號", "계정 %@개", "%@ アカウント", "Аккаунтов: %@", "%@ comptes",
	},
	xAccountCountOne: {
		"%@ account", "%@ 个账号", "%@ 個帳號", "계정 %@개", "%@ アカウント", "%@ аккаунт", "%@ compte",
	},
	xPrivacyDisplaySection: {
		"Privacy display", "隐私显示", "隱私顯示", "개인정보 표시", "プライバシー表示", "Приватное отображение", "Affichage confidentiel",
	},
	xPrivacyDisplayDescription: {
		"Mask account names and emails in account lists and the main panel.",
		"隐藏账号列表和主面板中的账号名称与邮箱",
		"隱藏帳號清單與主面板中的帳號名稱和電子郵件",
		"계정 목록과 메인 패널에서 계정 이름과 이메일을 가립니다.",
		"アカウント一覧とメインパネルのアカウント名とメールアドレスを隠します。",
		"Скрывает имена и адреса почты в списках аккаунтов и на главной панели.",
		"Masque les noms et adresses e-mail dans les listes de comptes et le panneau principal.",
	},
	xProvider: {
		"Provider", "平台", "平台", "플랫폼", "プラットフォーム", "Платформа", "Plateforme",
	},
	xProviderDescription: {
		"The platform the main panel and the status bar show first.",
		"主面板和状态栏优先显示的平台。",
		"主面板與狀態列優先顯示的平台。",
		"메인 패널과 상태 표시줄에 먼저 표시할 플랫폼입니다.",
		"メインパネルとステータスバーで最初に表示するプラットフォームです。",
		"Платформа, которая показывается первой на главной панели и в строке состояния.",
		"La plateforme affichée en premier dans le panneau principal et la barre d'état.",
	},
	xPanelHeight: {
		"Main panel height", "主面板高度", "主面板高度", "메인 패널 높이", "メインパネルの高さ",
		"Высота главной панели", "Hauteur du panneau principal",
	},
	xPanelHeightHint: {
		"From 360 to 900 points.", "范围 360 至 900 点。", "範圍 360 至 900 點。",
		"360~900 포인트입니다.", "360〜900 ポイントの範囲です。", "От 360 до 900 пунктов.", "De 360 à 900 points.",
	},
	xMacOnly: {
		"Applies to the macOS menu bar and panel only. Your saved choice is kept.",
		"仅适用于 macOS 菜单栏和面板，已保存的选择会保留。",
		"僅適用於 macOS 選單列與面板，已儲存的選擇會保留。",
		"macOS 메뉴 막대와 패널에만 적용됩니다. 저장된 선택은 유지됩니다.",
		"macOS のメニューバーとパネルにのみ適用されます。保存済みの選択は保持されます。",
		"Действует только в строке меню и панели macOS. Сохранённый выбор сохраняется.",
		"S'applique uniquement à la barre des menus et au panneau de macOS. Votre choix enregistré est conservé.",
	},
	xWidgetMac14: {
		"Widgets need macOS 14 or later. Your saved choice is kept.",
		"小组件需要 macOS 14 或更新版本，已保存的选择会保留。",
		"小工具需要 macOS 14 或更新版本，已儲存的選擇會保留。",
		"위젯은 macOS 14 이상에서 사용할 수 있습니다. 저장된 선택은 유지됩니다.",
		"ウィジェットには macOS 14 以降が必要です。保存済みの選択は保持されます。",
		"Виджеты требуют macOS 14 или новее. Сохранённый выбор сохраняется.",
		"Les widgets nécessitent macOS 14 ou une version ultérieure. Votre choix enregistré est conservé.",
	},
	xSettingsUnreadable: {
		"Settings can't be read", "无法读取设置", "無法讀取設定", "설정을 읽을 수 없습니다", "設定を読み込めません",
		"Не удалось прочитать настройки", "Impossible de lire les réglages",
	},
	xSettingsReadOnly: {
		"Settings are read-only and background tasks are paused. The stored value was not changed; fix or remove it, then restart Quosparr.",
		"设置处于只读状态，后台任务已暂停。已存储的值未被改动；请修复或移除后重新启动 Quosparr。",
		"設定為唯讀，背景工作已暫停。已儲存的值未被更動；請修復或移除後重新啟動 Quosparr。",
		"설정은 읽기 전용이며 백그라운드 작업이 일시 중지되었습니다. 저장된 값은 변경되지 않았습니다. 값을 수정하거나 삭제한 뒤 Quosparr를 다시 시작하세요.",
		"設定は読み取り専用で、バックグラウンド処理は停止しています。保存された値は変更されていません。修正または削除してから Quosparr を再起動してください。",
		"Настройки доступны только для чтения, фоновые задачи приостановлены. Сохранённое значение не изменено: исправьте или удалите его и перезапустите Quosparr.",
		"Les réglages sont en lecture seule et les tâches d'arrière-plan sont suspendues. La valeur enregistrée n'a pas été modifiée : corrigez-la ou supprimez-la, puis relancez Quosparr.",
	},
	xProxyBlocked: {
		"Network tasks are paused until the proxy settings are valid. Saving a valid proxy below replaces the damaged one.",
		"代理设置有效前网络任务已暂停。在下方保存有效的代理将替换已损坏的设置。",
		"代理設定有效前網路工作已暫停。在下方儲存有效的代理將取代已損壞的設定。",
		"프록시 설정이 올바를 때까지 네트워크 작업이 일시 중지됩니다. 아래에서 올바른 프록시를 저장하면 손상된 설정이 대체됩니다.",
		"プロキシ設定が有効になるまでネットワーク処理は停止しています。下で有効なプロキシを保存すると、破損した設定が置き換えられます。",
		"Сетевые задачи приостановлены, пока настройки прокси недействительны. Сохранение корректного прокси ниже заменит повреждённое значение.",
		"Les tâches réseau sont suspendues tant que les réglages du proxy ne sont pas valides. Enregistrer un proxy valide ci-dessous remplace la valeur endommagée.",
	},
	xSettingsSaveFailed: {
		"Could not save settings: %@", "无法保存设置：%@", "無法儲存設定：%@", "설정을 저장할 수 없습니다: %@",
		"設定を保存できませんでした: %@", "Не удалось сохранить настройки: %@", "Impossible d'enregistrer les réglages : %@",
	},
	xAccountsComingSoon: {
		"Account management is being rebuilt for this version. Your saved accounts are not changed.",
		"此版本的账号管理正在重建，已保存的账号不会被改动。",
		"此版本的帳號管理正在重建，已儲存的帳號不會被更動。",
		"이 버전의 계정 관리는 다시 구축하는 중입니다. 저장된 계정은 변경되지 않습니다.",
		"このバージョンのアカウント管理は再構築中です。保存済みのアカウントは変更されません。",
		"Управление аккаунтами для этой версии переписывается. Сохранённые аккаунты не изменяются.",
		"La gestion des comptes est en cours de reconstruction pour cette version. Vos comptes enregistrés ne sont pas modifiés.",
	},
	xTrayUnavailable: {
		"Cannot create the tray icon.", "无法创建托盘图标", "無法建立系統匣圖示", "트레이 아이콘을 만들 수 없습니다.",
		"トレイアイコンを作成できません。", "Не удалось создать значок в трее.", "Impossible de créer l'icône de la zone de notification.",
	},
	xLinuxDesktopDependencies: {
		"Quosparr needs GTK 3 and AppIndicator support. Install the desktop dependencies, then restart the app:\n%@",
		"Quosparr 需要 GTK 3 和 AppIndicator 支持，请安装桌面依赖后重新启动应用：\n%@",
		"Quosparr 需要 GTK 3 與 AppIndicator 支援，請安裝桌面相依套件後重新啟動應用程式：\n%@",
		"Quosparr에는 GTK 3와 AppIndicator 지원이 필요합니다. 데스크톱 의존성을 설치한 뒤 앱을 다시 시작하세요:\n%@",
		"Quosparr には GTK 3 と AppIndicator が必要です。デスクトップの依存パッケージをインストールしてから、アプリを再起動してください:\n%@",
		"Для Quosparr нужны GTK 3 и AppIndicator. Установите зависимости рабочего стола и перезапустите приложение:\n%@",
		"Quosparr nécessite GTK 3 et AppIndicator. Installez les dépendances du bureau, puis relancez l'application :\n%@",
	},
	xLinuxReleaseUnreadable: {
		"Could not identify the Linux distribution from /etc/os-release: %@",
		"无法从 /etc/os-release 识别 Linux 发行版：%@", "無法從 /etc/os-release 識別 Linux 發行版：%@",
		"/etc/os-release에서 Linux 배포판을 확인할 수 없습니다: %@",
		"/etc/os-release から Linux ディストリビューションを確認できませんでした: %@",
		"Не удалось определить дистрибутив Linux из /etc/os-release: %@",
		"Impossible d'identifier la distribution Linux depuis /etc/os-release : %@",
	},
	xPanelPlaceholder: {
		"The full panel is being built. Quota, reset credits and cost will appear here.",
		"完整面板正在构建中，额度、重置次数和费用将显示在这里。",
		"完整面板正在建置中，額度、重置次數與費用將顯示在這裡。",
		"전체 패널을 구축하는 중입니다. 할당량, 리셋 크레딧, 비용이 여기에 표시됩니다.",
		"完全なパネルは開発中です。クォータ、リセットクレジット、コストがここに表示されます。",
		"Полная панель в разработке. Здесь появятся квота, кредиты сброса и стоимость.",
		"Le panneau complet est en cours de développement. Le quota, les crédits de réinitialisation et le coût s'afficheront ici.",
	},
	xNotAvailableYet: {
		"Not available in this build yet.", "此版本暂未提供。", "此版本暫未提供。", "이 빌드에서는 아직 사용할 수 없습니다.",
		"このビルドではまだ利用できません。", "Пока недоступно в этой сборке.", "Pas encore disponible dans cette version.",
	},
	xProxyLocalSaveWarning: {
		"The password cannot be stored securely on this system yet, so a proxy with authentication cannot be saved.",
		"此系统上暂时无法安全保存密码，因此无法保存需要认证的代理。",
		"此系統上暫時無法安全儲存密碼，因此無法儲存需要驗證的代理。",
		"이 시스템에서는 아직 비밀번호를 안전하게 저장할 수 없어 인증이 필요한 프록시를 저장할 수 없습니다.",
		"このシステムではまだパスワードを安全に保存できないため、認証付きのプロキシは保存できません。",
		"Пока невозможно безопасно сохранить пароль в этой системе, поэтому прокси с аутентификацией сохранить нельзя.",
		"Le mot de passe ne peut pas encore être stocké de façon sécurisée sur ce système ; un proxy avec authentification ne peut donc pas être enregistré.",
	},
}

// tx returns the supplemental text for loc, formatted with args. A missing
// language falls back to English, as the Swift tables do.
func tx(loc l10n.Localizer, k textKey, args ...any) string {
	row, ok := extraTexts[k]
	if !ok {
		return string(k)
	}
	i := slices.Index(l10n.Languages(), loc.Language())
	if i < 0 || row[i] == "" {
		i = 0
	}
	return l10n.Format(row[i], args...)
}
