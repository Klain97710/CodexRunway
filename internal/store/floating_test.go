package store

import (
	"reflect"
	"strings"
	"testing"
)

func TestFloatingWindowIsGoOnlyAndOmittedByDefault(t *testing.T) {
	enc, err := EncodePreferences(DefaultPreferences())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(enc), "floatingWindow") {
		t.Errorf("the default floating window is written: %s", enc)
	}
	p := DefaultPreferences()
	p.FloatingWindow = FloatingWindow{Enabled: true, Style: FloatingStyleRings, Providers: []Provider{ProviderGrok, ProviderCodex}, Opacity: 80, Locked: true, OverFullScreen: true}
	if enc, err = EncodePreferences(p); err != nil {
		t.Fatal(err)
	}
	got := mustDecode(t, string(enc))
	want := p.FloatingWindow
	want.Providers = []Provider{ProviderCodex, ProviderGrok}
	if !reflect.DeepEqual(got.FloatingWindow, want) {
		t.Errorf("round trip = %+v, want %+v", got.FloatingWindow, want)
	}
}

func TestFloatingWindowDecodesLeniently(t *testing.T) {
	for raw, want := range map[string]FloatingWindow{
		`12`:                              {},
		`{"enabled":"yes"}`:               {},
		`{"style":"nope","enabled":true}`: {Enabled: true},
		`{"opacity":5}`:                   {Opacity: MinFloatingOpacity},
		`{"opacity":100}`:                 {},
		`{"opacity":150}`:                 {},
		`{"providers":["x","claude","claude","codex","grok"]}`: {Providers: []Provider{ProviderCodex, ProviderClaude, ProviderGrok}},
		`{"providers":[]}`: {},
	} {
		if got := mustDecode(t, `{"floatingWindow":`+raw+`}`).FloatingWindow; !reflect.DeepEqual(got, want) {
			t.Errorf("floatingWindow %s decoded to %+v, want %+v", raw, got, want)
		}
	}
}

func TestFloatingSelectionFollowsThePanel(t *testing.T) {
	p := DefaultPreferences()
	p.SelectedProvider = ProviderClaude
	if got := p.FloatingSelection(); !reflect.DeepEqual(got, []Provider{ProviderClaude}) {
		t.Errorf("selection = %v", got)
	}
	p.FloatingWindow.Providers = []Provider{ProviderGrok, ProviderCodex}
	if got := p.FloatingSelection(); !reflect.DeepEqual(got, []Provider{ProviderCodex, ProviderGrok}) {
		t.Errorf("custom selection = %v", got)
	}
	if got := p.FloatingWindow.OpacityPercent(); got != 100 {
		t.Errorf("default opacity = %d", got)
	}
}
