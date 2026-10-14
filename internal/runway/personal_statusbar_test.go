package runway

import (
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
	"testing"
)

func TestPersonalProTrayHidesOnlyStandardFiveHour(t *testing.T) {
	meters := []Meter{{WindowMinutes: FiveHourMinutes, Source: SourceStandard, RemainingPercent: 5}, {WindowMinutes: WeeklyMinutes, Source: SourceStandard, RemainingPercent: 32}, {WindowMinutes: FiveHourMinutes, Source: SourceModelSpecific, RemainingPercent: 90}}
	for _, tier := range []codex.Tier{codex.TierPro100, codex.TierPro200, codex.TierPro500, codex.TierPlus, ""} {
		in := StatusInputs{Prefs: store.DefaultPreferences(), Provider: store.ProviderCodex, CodexMeters: meters, CodexTier: tier}
		view := BuildStatusBar(l10n.New(l10n.English), in)
		want := 3
		if tier != codex.TierPlus && tier != "" {
			want = 2
		}
		if len(view.Items) != want || len(in.CodexMeters) != 3 {
			t.Fatalf("tier %s changed detail data or wrong tray count", tier)
		}
		if want == 2 && (view.Items[0].Meter.WindowMinutes != WeeklyMinutes || view.Items[1].Meter.Source != SourceModelSpecific) {
			t.Fatal("removed weekly or model-specific quota")
		}
	}
	in := StatusInputs{Prefs: store.DefaultPreferences(), Provider: store.ProviderCodex, CodexMeters: meters[:2], CodexTier: codex.TierPro200}
	if view := BuildStatusBar(l10n.New(l10n.English), in); len(view.Items) != 1 || view.Plan.RowsPerColumn != 1 {
		t.Fatal("Pro weekly not a centered single row")
	}
	in.CodexTier = codex.TierPlus
	if len(BuildStatusBar(l10n.New(l10n.English), in).Items) != 2 {
		t.Fatal("tier switch did not restore 5h")
	}
}
