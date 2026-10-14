package app

import (
	"bytes"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
)

func TestPanelParityQuotaShowsProjectionAndLocalizesLiveReset(t *testing.T) {
	l := l10n.New(l10n.SimplifiedChinese)
	m := runway.Meter{Title: "fixture", RemainingPercent: 70, ResetText: "stale English duration", ResetsAt: time.Now().Add(time.Hour), Projection: &runway.BurnProjection{ProjectedUsedPercentAtReset: 75}}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { meterRows(c, l, []runway.Meter{m}, false, false) })
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
