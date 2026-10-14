package app

const (
	xOAuthPort       textKey = "oauthPortInUse"
	xOAuthTimeout    textKey = "oauthTimedOut"
	xOAuthBrowser    textKey = "oauthBrowserFailed"
	xOAuthCallback   textKey = "oauthInvalidCallback"
	xOAuthExchange   textKey = "oauthExchangeFailed"
	xOAuthStore      textKey = "oauthStoreFailed"
	xCodexRestartMac textKey = "codexRestartMacOnly"
)

func init() {
	for key, texts := range map[textKey][7]string{
		xCodexRestartMac: {"Restart is available on macOS", "重启功能仅支持 macOS", "重新啟動功能僅支援 macOS", "재시작은 macOS에서 사용할 수 있습니다", "再起動はmacOSで利用できます", "Перезапуск доступен в macOS", "Le redémarrage est disponible sur macOS"},
		xOAuthPort:       {"Sign-in port 1455 is in use; close the other login and try again", "登录端口 1455 已被占用，请关闭其他登录后重试", "登入連接埠 1455 已被佔用，請關閉其他登入後重試", "로그인 포트 1455가 사용 중입니다; 다른 로그인을 닫고 다시 시도하세요", "ログインポート1455は使用中です、他のログインを閉じて再試行してください", "Порт входа 1455 занят; закройте другой вход и повторите", "Le port de connexion 1455 est occupé ; fermez l’autre connexion et réessayez"},
		xOAuthTimeout:    {"Sign-in timed out; start a new browser login", "登录已超时，请重新开始浏览器登录", "登入已逾時，請重新開始瀏覽器登入", "로그인 시간이 초과되었습니다; 브라우저 로그인을 다시 시작하세요", "ログインがタイムアウトしました、ブラウザログインをやり直してください", "Время входа истекло; начните вход заново", "La connexion a expiré ; recommencez dans le navigateur"},
		xOAuthBrowser:    {"Could not open the browser; check your default browser", "无法打开浏览器，请检查默认浏览器", "無法開啟瀏覽器，請檢查預設瀏覽器", "브라우저를 열 수 없습니다; 기본 브라우저를 확인하세요", "ブラウザを開けません、既定のブラウザを確認してください", "Не удалось открыть браузер; проверьте браузер по умолчанию", "Impossible d’ouvrir le navigateur ; vérifiez le navigateur par défaut"},
		xOAuthCallback:   {"Invalid sign-in callback; start a new login", "登录回调无效，请重新登录", "登入回呼無效，請重新登入", "로그인 콜백이 유효하지 않습니다; 다시 로그인하세요", "ログインコールバックが無効です、ログインをやり直してください", "Недействительный ответ входа; начните вход заново", "Retour de connexion invalide ; recommencez la connexion"},
		xOAuthExchange:   {"Could not exchange the login code; check your network and try again", "登录授权交换失败，请检查网络后重试", "登入授權交換失敗，請檢查網路後重試", "로그인 코드를 교환할 수 없습니다; 네트워크를 확인하고 다시 시도하세요", "ログインコードを交換できません、ネットワークを確認して再試行してください", "Не удалось обменять код входа; проверьте сеть и повторите", "Impossible d’échanger le code de connexion ; vérifiez le réseau et réessayez"},
		xOAuthStore:      {"Login completed but the account could not be saved", "登录已完成，但账号保存失败", "登入已完成，但帳號儲存失敗", "로그인은 완료되었지만 계정을 저장할 수 없습니다", "ログインは完了しましたが、アカウントを保存できません", "Вход выполнен, но не удалось сохранить аккаунт", "Connexion réussie, mais le compte n’a pas pu être enregistré"},
	} {
		extraTexts[key] = texts
	}
}
