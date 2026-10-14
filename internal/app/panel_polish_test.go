package app

import (
	"bytes"

	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/l10n"

	"github.com/licoy/quosparr/internal/store"
)

func TestPanelIdentityStartsAtLeadingInset(t *testing.T) {
	snap := panelFixture()
	snap.Accounts.Display.Tier = codex.TierPro200
	_, tt := newPanel(t, nil, snap, PanelActions{})
	badge, ok := tt.Find("Pro 200")
	if !ok || badge.X < 15 || badge.X > 17 {
		t.Fatalf("tier badge must begin at the panel's 16pt inset, got %v (found %v)", badge, ok)
	}
	if os.Getenv("RUNWAY_UI_QA") != "" {
		snap := uiReviewSnapshot()
		snap.Accounts.Display.Tier = codex.TierPro200
		snap.Accounts.Display.DisplayName = "m***@***.com"
		expiry := time.Now().Add(11*24*time.Hour + 9*time.Hour)
		snap.Accounts.Display.SubscriptionExpiresAt = &expiry
		_, panel := newPanel(t, func(p *store.Preferences) { p.Language = store.LanguageSimplifiedChinese }, snap, PanelActions{})
		panel.SetScale(2)
		for _, dark := range []bool{false, true} {
			panel.SetDark(dark)
			name := "light"
			if dark {
				name = "dark"
			}
			exportReviewPNG(t, panel, filepath.Join("..", "..", ".build", "v1-ui-qa", "polish-panel-"+name+".png"))
		}
	}
}

func TestPanelExpiryUsesAvailableWidth(t *testing.T) {
	for _, lang := range l10n.Languages() {
		t.Run(string(lang), func(t *testing.T) {
			l := l10n.New(lang)
			now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.Local)
			expires := now.Add(11*24*time.Hour + 9*time.Hour + 32*time.Minute)
			remaining := expiryRemaining(l, expires, now)
			var naturalWidth float32
			tt := ui.NewTester(func(c *ui.Context) {
				applyTheme(c)
				naturalWidth, _ = c.MeasureText(1000, ui.Span{Text: remaining, Size: fsCaption, Weight: 500, Features: "tnum"})
				ui.Column(c).Fill().Padding(16).Children(func() {
					ui.Row(c).Children(func() { expiryBadge(c, l, expires, now) })
				})
			}, 400, 100)
			tt.SetScale(2)
			if os.Getenv("RUNWAY_UI_QA") != "" {
				exportReviewPNG(t, tt, filepath.Join("..", "..", ".build", "v1-ui-qa", "polish-expiry-"+string(lang)+".png"))
			}
			r, ok := tt.Find(remaining)
			if !ok || r.W+1 < naturalWidth || r.X+r.W > 384 {
				t.Fatalf("countdown clipped despite available panel space: bounds=%v naturalWidth=%v found=%v", r, naturalWidth, ok)
			}
		})
	}
}

func TestPanelHeatmapHoverShowsDayDetail(t *testing.T) {
	snap := uiReviewSnapshot()
	_, tt := newPanel(t, func(p *store.Preferences) { p.MainPanelHeight = 860 }, snap, PanelActions{})
	cell, ok := tt.Find("2026-10-11")
	if !ok {
		t.Fatal("heatmap day missing")
	}
	tt.Move(cell.X+cell.W/2, cell.Y+cell.H/2)
	tt.Frame()
	want := heatTooltip(l10n.New(l10n.English), store.ProviderCodex, "2026-10-11", snap.Codex.Heatmap.AllDevices["2026-10-11"], snap.Codex.Heatmap.AllDevices["2026-10-11"], snap.Codex.Heatmap.Local["2026-10-11"])
	if !tt.HasText(want) {
		t.Fatalf("hover did not show day details at %v", cell)
	}
	tip, _ := tt.Find(want)
	if tip.X < 0 || tip.X+tip.W > panelWidth {
		t.Fatalf("tooltip outside panel: %v", tip)
	}
	if os.Getenv("RUNWAY_UI_QA") != "" {
		exportReviewPNG(t, tt, filepath.Join("..", "..", ".build", "v1-ui-qa", "polish-heatmap.png"))
	}
	tt.Move(5, 5)
	if tt.HasText(want) {
		t.Fatal("tooltip remained after leaving heatmap")
	}
}

func TestPanelMetallicBadgeActuallyAnimates(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Padding(10).Children(func() { tierBadge(c, codexTierLook(c.Theme(), codex.TierPro200), "Pro 200") })
	}, 150, 45)
	before := append([]byte(nil), tt.Image().Pix...)
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		tt.Frame()
		if !bytes.Equal(before, tt.Image().Pix) {
			return
		}
	}
	t.Fatal("metallic badge stayed static through a complete Swift sheen cycle")
}

func TestPanelMetallicBadgeRespectsReduceMotion(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Padding(10).Children(func() { tierBadge(c, codexTierLook(c.Theme(), codex.TierPro200), "Pro 200") })
	}, 150, 45)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	before := append([]byte(nil), tt.Image().Pix...)
	time.Sleep(100 * time.Millisecond)
	tt.Frame()
	if !bytes.Equal(before, tt.Image().Pix) {
		t.Fatal("badge animated with Reduce Motion enabled")
	}
}
