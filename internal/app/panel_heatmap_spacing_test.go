package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
	"testing"
)

func TestHeatmapHeightFollowsActualCells(t *testing.T) {
	heat := uiReviewSnapshot().Codex.Heatmap
	grid := usage.BuildHeatmap(usage.HeatmapInput{AllDevices: heat.AllDevices, Now: heat.CalculatedAt, FirstWeekday: heatmapWeekStart})
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Children(func() {
			panelHeatmap(c, l10n.New(l10n.English), grid, store.ProviderCodex)
			ui.Text(c, "following section")
		})
	}, 400, 200)
	r, ok := tt.Find("following section")
	want := heatCellSize(368, len(grid.Weeks))*7 + heatSpacing*6 + heatLabelGap + heatLabelH
	if !ok || r.Y > want+1 {
		t.Fatalf("heatmap reserves unused vertical space: next=%v actual chart height=%v", r, want)
	}
}
