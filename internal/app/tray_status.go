package app

import (
	"strconv"
	"strings"

	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

// TrayFromStatus keeps the coordinator's style, captions and column layout
// together so a preference change reaches the actual tray renderer.
func TrayFromStatus(v runway.StatusBarView) TrayRender {
	r := TrayRender{Style: v.Style, Text: v.Text, RowsPerColumn: v.Plan.RowsPerColumn, ToolTip: v.Tooltip}
	for _, m := range v.Plan.Meters {
		r.Meters = append(r.Meters, TrayMeter{Remaining: float64(m.RemainingPercent) / 100})
	}
	for _, item := range v.Items {
		caption := item.MeterText
		switch v.Style {
		case store.StatusBarStyleCountdown:
			caption = item.Countdown
		case store.StatusBarStyleBattery:
			caption = item.Battery
			if len(v.Items) == 1 {
				caption = item.BatteryDetail
			}
		case store.StatusBarStyleRings:
			caption = item.Ring
		}
		label := item.Meter.Title
		if item.Meter.Source == runway.SourceStandard && !strings.Contains(label, " · ") {
			switch item.Meter.WindowMinutes {
			case runway.FiveHourMinutes:
				label = "5h"
			case runway.WeeklyMinutes:
				label = "7d"
			}
		}
		value := strconv.Itoa(item.Meter.RemainingPercent) + "%"
		if v.Style == store.StatusBarStyleMeters {
			caption = label + " " + value
		}
		r.Items = append(r.Items, TrayItem{Label: label, Value: value, Caption: caption, Remaining: float64(item.Meter.RemainingPercent) / 100, Health: item.Meter.Health})
		r.Lines = append(r.Lines, caption)
	}
	switch {
	case v.Style == store.StatusBarStyleText:
		r.Lines = append([]string(nil), v.TextCaptions...)
	case v.Style == "", v.Style == store.StatusBarStyleCountdown && len(v.Items) <= 1:
		r.Lines = nil
		if text := strings.TrimSpace(v.Text); text != "" {
			r.Lines = strings.Split(text, "\n")
		}
	}
	return r
}
