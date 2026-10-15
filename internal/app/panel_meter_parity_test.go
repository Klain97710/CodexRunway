package app

import (
	"bytes"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

func TestPanelParityQuotaShowsProjectionAndLocalizesLiveReset(t *testing.T) {
	l := l10n.New(l10n.SimplifiedChinese)
	m := runway.Meter{Title: "fixture", RemainingPercent: 70, ResetText: "stale English duration", ResetsAt: time.Now().Add(time.Hour), Projection: &runway.BurnProjection{ProjectedUsedPercentAtReset: 75}}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { meterRows(c, l, []runway.Meter{m}, false, false, resetSwitch{}) })
	}, 368, 200)
	if !tt.HasText(l.Text(l10n.KeyProjectedAtReset)+" 75%") || tt.HasText("stale English duration") || !tt.HasText(l.Text(l10n.KeyNextResetIn)) {
		t.Fatalf("quota projection or live localized reset omitted; texts %q", tt.Texts())
	}
}

func TestPanelParityQuotaDrawsThresholdMarkers(t *testing.T) {
	m := runway.Meter{RemainingPercent: 70}
	render := func() []byte {
		tt := ui.NewTester(func(c *ui.Context) { quotaBar(c, m) }, 368, 20)
		return append([]byte(nil), tt.Image().Pix...)
	}
	plain := render()
	m.MarkerPercents = []int{25, 50, 75}
	if bytes.Equal(plain, render()) {
		t.Fatal("threshold markers did not change the actual quota bar image")
	}
}

func TestQuotaResetLineSwitchesFormatOnClick(t *testing.T) {
	l := l10n.New(l10n.SimplifiedChinese)
	now := time.Now()
	m := runway.Meter{Title: "fixture", RemainingPercent: 57, ResetsAt: now.Add(6*24*time.Hour + 3*time.Hour)}
	format := store.ResetTimeRelative
	st := &panelState{setResetTimeFormat: func(f store.ResetTimeFormat) { format = f }}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { meterRows(c, l, []runway.Meter{m}, false, true, st.resetSwitch(format)) })
	}, 368, 200)
	exact := l.Text(l10n.KeyNextResetIn) + runway.ResetExactTime(m.ResetsAt, now, l10n.SimplifiedChinese, nil)
	if tt.HasText(exact) || !tt.HasText(l.Text(l10n.KeyNextResetIn)) {
		t.Fatalf("relative reset expected; texts %q", tt.Texts())
	}
	label := resetText(l, m.ResetsAt, time.Now(), store.ResetTimeRelative)
	if err := tt.Click(label); err != nil {
		t.Fatalf("click %q: %v; texts %q", label, err, tt.Texts())
	}
	tt.Frame()
	if format != store.ResetTimeExact || !tt.HasText(exact) {
		t.Fatalf("format %q after click; texts %q", format, tt.Texts())
	}
	if err := tt.Click(exact); err != nil {
		t.Fatal(err)
	}
	if format != store.ResetTimeRelative {
		t.Fatalf("second click left format %q", format)
	}
}
