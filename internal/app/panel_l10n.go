package app

import (
	"strings"

	"github.com/licoy/quosparr/internal/l10n"
)

const xWeekdayAbbreviations textKey = "calendarWeekdayAbbreviations"

func init() {
	extraTexts[xWeekdayAbbreviations] = [7]string{
		"S|M|T|W|T|F|S",
		"日|一|二|三|四|五|六",
		"日|一|二|三|四|五|六",
		"일|월|화|수|목|금|토",
		"日|月|火|水|木|金|土",
		"Вс|Пн|Вт|Ср|Чт|Пт|Сб",
		"D|L|M|M|J|V|S",
	}
}

func heatmapWeekdays(l l10n.Localizer) []string {
	return strings.Split(tx(l, xWeekdayAbbreviations), "|")
}

const xDetailUnavailable textKey = "detailServiceUnavailable"

func init() {
	extraTexts[xDetailUnavailable] = [7]string{"Detail service unavailable", "详情服务暂不可用", "詳情服務暫不可用", "상세 서비스를 사용할 수 없습니다", "詳細サービスを利用できません", "Сервис подробностей недоступен", "Service de détails indisponible"}
}

const xGrokCycleUnavailable textKey = "grokBillingCycleUnavailable"

func init() {
	extraTexts[xGrokCycleUnavailable] = [7]string{"Grok billing cycle unavailable; choose Today, This month, or Custom", "Grok 账单周期暂不可用，请选择今日、本月或自定义", "Grok 帳單週期暫不可用，請選擇今日、本月或自訂", "Grok 청구 주기를 사용할 수 없습니다; 오늘, 이번 달 또는 사용자 지정을 선택하세요", "Grok の請求期間を取得できません、今日、今月、カスタムを選択してください", "Расчётный период Grok недоступен; выберите сегодня, месяц или свой период", "Période de facturation Grok indisponible ; choisissez aujourd’hui, ce mois ou personnalisé"}
}
