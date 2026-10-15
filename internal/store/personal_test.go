package store

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/licoy/quosparr/internal/personal"
)

func TestPersonalProviderMigrationPreservesEveryOtherPreference(t *testing.T) {
	raw := fixture(t, "swift_all.json")
	p, err := DecodePreferences(raw)
	if err != nil {
		t.Fatal(err)
	}
	before, err := EncodePreferences(p)
	if err != nil {
		t.Fatal(err)
	}
	p.ApplyFeatures(personal.Features{})
	after, err := EncodePreferences(p)
	if err != nil {
		t.Fatal(err)
	}
	var a, b map[string]any
	if json.Unmarshal(before, &a) != nil || json.Unmarshal(after, &b) != nil {
		t.Fatal("invalid output")
	}
	a["selectedProvider"] = "codex"
	a["statusBarProviderScope"] = "selected"
	a["panelProviders"] = []any{"codex"}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("provider migration changed unrelated preferences")
	}
	if p.ShowsRateLimitResetToday {
		t.Fatal("explicit disabled reset status was lost")
	}
	if !DefaultPreferences().ShowsRateLimitResetToday {
		t.Fatal("new install reset status disabled")
	}
}
