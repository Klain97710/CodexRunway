package app

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

func panelYearHeat() runway.HeatmapData {
	at := time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC)
	heat := runway.HeatmapData{AllDevices: make(map[string]int64), Local: make(map[string]int64), CalculatedAt: at}
	for day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC); !day.After(at); day = day.AddDate(0, 0, 1) {
		heat.AllDevices[day.Format("2006-01-02")] = int64(day.YearDay()) * 1300
		heat.Local[day.Format("2006-01-02")] = int64(day.YearDay()) * 300
	}
	return heat
}

func BenchmarkPanelChartFrame(b *testing.B) {
	for _, data := range []struct {
		name string
		heat runway.HeatmapData
	}{{"twoDays", panelFixture().Codex.Heatmap}, {"year365", panelYearHeat()}} {
		for _, style := range []store.ChartStyle{store.ChartStyleHeatmap, store.ChartStyleLine, store.ChartStyleBar} {
			b.Run(data.name+"/"+string(style), func(b *testing.B) {
				l, state := l10n.New(l10n.English), &panelState{}
				tt := ui.NewTester(func(c *ui.Context) {
					applyTheme(c)
					ui.Column(c).Fill().Children(func() { panelChart(c, l, style, store.ProviderCodex, data.heat, false, state) })
				}, 368, 210)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					tt.Frame()
				}
			})
		}
	}
}

func BenchmarkPanelGrid(b *testing.B) {
	heat := panelYearHeat()
	b.ReportAllocs()
	for b.Loop() {
		heat.Grid(usage.HeatmapDaily, heatmapWeekStart, heat.CalculatedAt)
	}
}

func TestPanelClickRoutesStayImmediate(t *testing.T) {
	queue := detailMainQueue(t)
	snap := panelFixture()
	snap.Accounts.Sidebar = append(snap.Accounts.Sidebar, codex.ManagedAccount{ID: "other", DisplayName: "Other fixture"})
	refresh, selected := 0, store.ProviderCodex
	switched := make(chan string, 1)
	sections := make(map[runway.Section]int)
	a, tt := newPanel(t, nil, snap, PanelActions{})
	a.SetPanelActions(PanelActions{
		Refresh: func() { refresh++ }, RefreshSection: func(s runway.Section) { sections[s]++ },
		SelectProvider: func(p store.Provider) { selected = p; a.settings.SetSelectedProvider(p) },
		Accounts:       AccountActions{Switch: func(id string, _ bool) string { switched <- id; return "" }},
	})
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	click := func(key string) {
		t.Helper()
		at := time.Now()
		if err := tt.Click(key); err != nil {
			t.Fatal(err)
		}
		if elapsed := time.Since(at); elapsed >= latencyBudget(time.Second) {
			t.Fatalf("click %q blocked for %s", key, elapsed)
		}
	}
	click(a.L().Text(l10n.KeyRefresh))
	if refresh != 1 {
		t.Fatal("refresh was not routed")
	}
	r, ok := tt.Find(a.L().Text(l10n.KeyQuota))
	if !ok {
		t.Fatal("quota missing")
	}
	tt.ClickAt(r.X+r.W+6+12, r.Y+r.H/2)
	if sections[runway.SectionQuota] != 1 {
		t.Fatalf("section refresh not routed: %v", sections)
	}
	click(a.L().Text(l10n.KeyAccounts))
	if a.panelState.detail != detailAccounts || !tt.HasText("Other fixture") {
		t.Fatal("accounts detail did not open")
	}
	click(a.L().Text(l10n.KeyAccountsMakeCurrent))
	if a.panelState.accounts.switchID != "other" || !tt.HasText(a.L().Text(l10n.KeyAccountsSwitchConfirmTitle)) {
		t.Fatal("switch confirmation did not open for selected account")
	}
	// The confirm label also appears on the underlying account action; target
	// the visible confirmation row beside Cancel, inside the real popover.
	cancel, ok := tt.Find(a.L().Text(l10n.KeyCancel))
	if !ok {
		t.Fatal("switch confirmation missing Cancel")
	}
	tt.ClickAt(cancel.X+cancel.W+8+20, cancel.Y+cancel.H/2)
	select {
	case id := <-switched:
		if id != "other" {
			t.Fatalf("switched %q", id)
		}
	case <-time.After(time.Second):
		t.Fatal("confirmation did not route switch")
	}
	finishDetail(t, queue)
	click(a.L().Text(l10n.KeyBack))
	if a.panelState.detail != "" {
		t.Fatal("back did not restore summary")
	}
	click(a.L().Text(l10n.KeyProviderGrok))
	if selected != store.ProviderGrok || a.panelState.provider != store.ProviderGrok {
		t.Fatal("provider was not applied")
	}
}

func TestPanelChartClickLatency(t *testing.T) {
	for _, style := range []store.ChartStyle{store.ChartStyleHeatmap, store.ChartStyleLine, store.ChartStyleBar} {
		t.Run(string(style), func(t *testing.T) {
			heat, state, l := panelYearHeat(), &panelState{}, l10n.New(l10n.English)
			tt := ui.NewTester(func(c *ui.Context) {
				applyTheme(c)
				ui.Column(c).Fill().Children(func() {
					first := true
					panelToken(c, l, style, runway.Snapshot{Provider: store.ProviderCodex, Codex: runway.CodexView{Heatmap: heat}}, PanelActions{}, &first, state)
				})
			}, 368, 280)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
			third := l10n.KeyHeatmapCumulative
			if style != store.ChartStyleHeatmap {
				third = l10n.KeyHeatmapMonthly
			}
			for _, target := range []struct {
				key  l10n.Key
				mode usage.HeatmapMode
			}{{l10n.KeyHeatmapWeekly, usage.HeatmapWeekly}, {third, usage.HeatmapCumulative}, {l10n.KeyHeatmapDaily, usage.HeatmapDaily}} {
				at := time.Now()
				if err := tt.Click(l.Text(target.key)); err != nil {
					t.Fatal(err)
				}
				if state.mode() != target.mode {
					t.Fatalf("mode = %s, want %s", state.mode(), target.mode)
				}
				t.Logf("365 days, %s: %s", target.mode, time.Since(at))
			}
			r, ok := tt.Find("2026-12-31")
			if !ok {
				t.Fatal("last day of the year was dropped")
			}
			t.Logf("tooltip hit box: %+v", r)
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			time.Sleep(650 * time.Millisecond)
			tt.Frame()
			if !tt.HasText("474.5K") {
				t.Fatalf("last-day token count missing from chart tooltip: %q", tt.Texts())
			}
		})
	}
}

func BenchmarkPanelCostDetailFrame(b *testing.B) {
	for _, count := range []int{10, 200, 500} {
		b.Run(strconv.Itoa(count)+"Projects", func(b *testing.B) {
			rows := make([]usage.BreakdownRow, count)
			for i := range rows {
				rows[i] = usage.BreakdownRow{Name: "Project " + strconv.Itoa(i), Totals: usage.Totals{TotalTokens: int64(i+1) * 1300}}
			}
			l := l10n.New(l10n.English)
			tt := ui.NewTester(func(c *ui.Context) {
				applyTheme(c)
				ui.Scroll(c).Fill().Children(func() {
					ui.Column(c).Padding(16).Children(func() { costResult(c, l, &panelCostState{}, usage.Summary{ProjectRows: rows}) })
				})
			}, 400, 584)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				tt.Frame()
			}
		})
	}
}

func TestPanelCostPendingQueryAllowsBack(t *testing.T) {
	queue := detailMainQueue(t)
	started := make(chan context.Context, 2)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	timer := time.AfterFunc(time.Second, unblock)
	defer timer.Stop()
	defer unblock()
	acts := PanelActions{QueryCost: func(ctx context.Context, _ store.Provider, _ CostQuery) (usage.Summary, error) {
		started <- ctx
		<-release
		return usage.Summary{}, nil
	}}
	a, tt := newPanel(t, nil, panelFixture(), acts)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	at := time.Now()
	if err := tt.Click(a.L().Text(l10n.KeyShowDetails)); err != nil {
		t.Fatal(err)
	}
	t.Logf("open cost with blocked query: %s", time.Since(at))
	var ctx context.Context
	select {
	case ctx = <-started:
	case <-time.After(time.Second):
		t.Fatal("cost disclosure did not start a query")
	}
	if !a.panelState.cost.task.Running {
		t.Fatal("pending query was not visible")
	}
	select {
	case <-release:
		t.Fatal("opening cost waited for the query")
	default:
	}
	if err := tt.Click(a.L().Text(l10n.KeyBack)); err != nil {
		t.Fatal(err)
	}
	if a.panelState.detail != "" || ctx.Err() == nil {
		t.Fatal("back failed to cancel pending query")
	}
	if a.panelState.cost.task.Running {
		t.Fatal("back launched another cost query")
	}
	unblock()
	finishDetail(t, queue)
	if a.panelState.cost.summary != nil {
		t.Fatal("cancelled query populated closed detail")
	}
}
