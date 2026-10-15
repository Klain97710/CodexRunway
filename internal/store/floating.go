package store

import (
	"encoding/json"
	"slices"
)

// FloatingStyle is how the desktop floating window draws the quotas. Only the
// Go app knows it, so the capsule is the empty value and is not written.
type FloatingStyle string

// FloatingStyle values.
const (
	FloatingStyleCapsule FloatingStyle = ""
	FloatingStyleCard    FloatingStyle = "card"
	FloatingStyleRings   FloatingStyle = "rings"
	FloatingStyleBattery FloatingStyle = "battery"
)

// FloatingStyles returns every valid FloatingStyle.
func FloatingStyles() []FloatingStyle {
	return []FloatingStyle{FloatingStyleCapsule, FloatingStyleCard, FloatingStyleRings, FloatingStyleBattery}
}

// Valid reports whether v is a known value.
func (v FloatingStyle) Valid() bool { return slices.Contains(FloatingStyles(), v) }

// Limits of the floating window opacity, in percent.
const (
	MinFloatingOpacity = 40
	MaxFloatingOpacity = 100
)

// FloatingOpacityOptions are the opacities the settings UI offers.
func FloatingOpacityOptions() []int { return []int{100, 90, 80, 70, 60, 50} }

// FloatingWindow is the desktop floating window. It is Go-only: written as
// one object, left out while every field is at its default, and an
// unreadable value is the default.
type FloatingWindow struct {
	Enabled bool          `json:"enabled,omitempty"`
	Style   FloatingStyle `json:"style,omitempty"`
	// Providers are the providers shown, in Providers() order; empty
	// follows the main panel's provider.
	Providers []Provider `json:"providers,omitempty"`
	// Opacity is in percent; 0 stands for fully opaque.
	Opacity int  `json:"opacity,omitempty"`
	Locked  bool `json:"locked,omitempty"`
	// OverFullScreen keeps the window on screen while another app is full
	// screen; otherwise it steps aside.
	OverFullScreen bool `json:"overFullScreen,omitempty"`
}

// OpacityPercent is the opacity in percent, 100 for the default.
func (f FloatingWindow) OpacityPercent() int {
	if f.Opacity == 0 {
		return MaxFloatingOpacity
	}
	return f.Opacity
}

func (f FloatingWindow) clone() FloatingWindow {
	f.Providers = slices.Clone(f.Providers)
	return f
}

// normalize drops unknown styles and providers and clamps the opacity, with
// full opacity stored as the default.
func (f *FloatingWindow) normalize() {
	if !f.Style.Valid() {
		f.Style = FloatingStyleCapsule
	}
	f.Providers = canonicalProviders(f.Providers, len(Providers()))
	if len(f.Providers) == 0 {
		f.Providers = nil
	}
	if f.Opacity != 0 {
		f.Opacity = max(MinFloatingOpacity, min(MaxFloatingOpacity, f.Opacity))
	}
	if f.Opacity == MaxFloatingOpacity {
		f.Opacity = 0
	}
}

// FloatingSelection is what the floating window shows: the chosen
// providers, or the main panel's.
func (p Preferences) FloatingSelection() []Provider {
	if sel := canonicalProviders(p.FloatingWindow.Providers, len(Providers())); len(sel) > 0 {
		return sel
	}
	if p.SelectedProvider.Valid() {
		return []Provider{p.SelectedProvider}
	}
	return []Provider{ProviderCodex}
}

// readFloatingWindow decodes the Go-only floating window leniently, as
// readThemeColor: an unreadable object is the default.
func readFloatingWindow(r *reader, key string) FloatingWindow {
	raw, ok := r.take(key)
	var f FloatingWindow
	if !ok || json.Unmarshal(raw, &f) != nil {
		return FloatingWindow{}
	}
	f.normalize()
	return f
}
