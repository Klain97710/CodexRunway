package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/personal"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

func BenchmarkLivePreferencesCallback(b *testing.B) {
	for _, accounts := range []int{2, 50} {
		b.Run(fmt.Sprintf("accounts-%d", accounts), func(b *testing.B) {
			p := store.DefaultPreferences()
			p.ShowsQuotaEstimateSummary = false
			st := grok.NewStoreAt(b.TempDir(), b.TempDir())
			idx := grok.NewAccountIndex()
			// Inactive cached billing responses are intentionally retained by the index,
			// as normal account refreshes do; they do not need parsing for a UI preference.
			for i := 0; i < accounts; i++ {
				idx.Accounts = append(idx.Accounts, grok.ManagedAccount{ID: fmt.Sprintf("synthetic-%d", i), SortIndex: i, CachedQuota: json.RawMessage(`{"synthetic":"` + strings.Repeat("x", 16384) + `"}`)})
			}
			if err := st.SaveIndex(idx); err != nil {
				b.Fatal(err)
			}
			g := newGrokBridge(grok.NewModule(st), p, nil)
			g.reload(false)
			svc := runway.New(runway.Options{Features: personal.Features{Grok: true, Claude: true}, Preferences: p, Grok: g})
			defer svc.Close()
			rt := &liveApp{svc: svc, grok: g, reset: newResetBridge(nil, p, nil)}
			settings := NewSettings(nil, System{Features: personal.Features{Grok: true, Claude: true}}, nil)
			settings.prefs = p
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				settings.prefs.AccountPrivacyModeEnabled = !settings.prefs.AccountPrivacyModeEnabled
				rt.pushPreferences(settings)
			}
			b.StopTimer()
			rt.closePreferences()
		})
	}
}
