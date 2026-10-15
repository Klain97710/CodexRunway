package app

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/codex"
)

func TestClaudeTierLooksAreDistinct(t *testing.T) {
	for _, dark := range []bool{false, true} {
		theme := &ui.Theme{Dark: dark}
		plain := plainLook(theme)
		seen := map[claude.TierKind]tierLook{}
		for _, k := range previewClaudeTiers {
			look := claudeTierLook(theme, k)
			if reflect.DeepEqual(look, plain) {
				t.Errorf("dark=%t %s uses the plain look", dark, k.DisplayName())
			}
			seen[k] = look
		}
		for _, pair := range [][2]claude.TierKind{{claude.TierPro, claude.TierMax5x}, {claude.TierMax5x, claude.TierMax20x}, {claude.TierTeam, claude.TierEnterprise}} {
			if reflect.DeepEqual(seen[pair[0]], seen[pair[1]]) {
				t.Errorf("dark=%t %s and %s share a look", dark, pair[0].DisplayName(), pair[1].DisplayName())
			}
		}
		if !reflect.DeepEqual(claudeTierLook(theme, claude.TierUnknown), plain) {
			t.Errorf("dark=%t unknown tier must stay plain", dark)
		}
		if reflect.DeepEqual(codexTierLook(theme, codex.TierBusinessPremium), plain) {
			t.Errorf("dark=%t Business Premium uses the plain look", dark)
		}
	}
}

func TestBusinessPremiumLabel(t *testing.T) {
	if accountTierKey(codex.TierBusinessPremium) == accountTierKey(codex.TierBusiness) {
		t.Fatal("Business Premium needs its own label")
	}
}
