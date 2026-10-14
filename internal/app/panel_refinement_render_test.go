package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/l10n"

	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

func TestExportPanelRefinement(t *testing.T) {
	if os.Getenv("RUNWAY_UI_QA") == "" {
		t.Skip("set RUNWAY_UI_QA to export panel fixtures")
	}
	dir := filepath.Join("..", "..", ".build", "v1-ui-qa")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		snap := uiReviewSnapshot()
		snap.Accounts.Display.Tier = codex.TierPro200
		snap.Accounts.Display.DisplayName = "m***@***.com"
		now := time.Now()
		snap.Codex.Meters = []runway.Meter{{Title: "每周", RemainingPercent: 53, ResetsAt: now.Add(126 * time.Hour), Projection: &runway.BurnProjection{ExhaustsAt: now.Add(46 * time.Hour)}}}
		a, tt := newPanel(t, func(p *store.Preferences) { p.Language = store.LanguageSimplifiedChinese }, snap, PanelActions{SelectProvider: func(store.Provider) {}})
		tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
		tt.SetScale(2)
		tt.SetDark(dark)
		row, _ := tt.Find(a.L().Text(l10n.KeyAccounts))
		tt.Move(380, row.Y+row.H/2)
		exportReviewPNG(t, tt, filepath.Join(dir, "refinement-panel-"+name+".png"))

	}
}
