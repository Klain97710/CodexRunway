package app

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
)

type expiryPhase int

const (
	expiryActive expiryPhase = iota
	expirySoon
	expiryExpired
)

// expiryWindow is the reference window of the countdown ring: a billing month.
const expiryWindow = 30 * 24 * time.Hour

// expiryLook is the palette of one phase of the subscription chip.
type expiryLook struct {
	ring, foreground, fill, stroke ui.Color
}

func expiryPhaseOf(expiresAt, now time.Time) expiryPhase {
	switch {
	case runway.SubscriptionIsExpired(expiresAt, now):
		return expiryExpired
	case expiresAt.Sub(now) <= 7*24*time.Hour:
		return expirySoon
	}
	return expiryActive
}

func expiryLookOf(t *ui.Theme, phase expiryPhase) expiryLook {
	light := !t.Dark
	switch phase {
	case expirySoon:
		tint := sys(t, sysOrange)
		return expiryLook{
			ring:       pick(light, ui.RGBA(184, 97, 13, 1), tint),
			foreground: pick(light, ui.RGBA(158, 82, 10, 1), tint),
			fill:       tint.Alpha(pick[float32](light, 0.18, 0.26)),
			stroke:     tint.Alpha(pick[float32](light, 0.40, 0.46)),
		}
	case expiryExpired:
		tint := sys(t, sysRed)
		return expiryLook{
			ring:       tint,
			foreground: pick(light, ui.RGBA(255, 255, 255, 1), ui.RGBA(255, 204, 204, 1)),
			fill:       pick(light, ui.RGBA(199, 33, 28, 1), tint.Alpha(0.42)),
			stroke:     tint.Alpha(0.55),
		}
	}
	tint := sys(t, sysGreen)
	return expiryLook{
		ring:       pick(light, ui.RGBA(26, 133, 71, 1), tint),
		foreground: pick(light, ui.RGBA(20, 107, 56, 1), tint),
		fill:       tint.Alpha(pick[float32](light, 0.16, 0.26)),
		stroke:     tint.Alpha(pick[float32](light, 0.35, 0.42)),
	}
}

func expiryStatusKey(phase expiryPhase) l10n.Key {
	switch phase {
	case expirySoon:
		return l10n.KeySubscriptionExpiringSoon
	case expiryExpired:
		return l10n.KeySubscriptionExpired
	}
	return l10n.KeySubscriptionExpires
}

// expiryRemaining is the localized time left, "" once expired.
func expiryRemaining(l l10n.Localizer, expiresAt, now time.Time) string {
	if runway.SubscriptionIsExpired(expiresAt, now) {
		return ""
	}
	return runway.DurationLocalized(expiresAt.Sub(now).Seconds(), l.Language(), false)
}

// subscriptionExpiryHelp is the tooltip: the exact local expiry instant.
func subscriptionExpiryHelp(l l10n.Localizer, expiresAt time.Time) string {
	return l10n.Format(l.Text(l10n.KeySubscriptionExpiryLocalTime), runway.SubscriptionExpiresAt(expiresAt, l.Language(), time.Local))
}

// expiryBadge is SubscriptionExpiryBadge: a tinted status segment with a
// countdown ring and a neutral data segment, joined by a hairline.
// Intrinsic, non-wrapping text avoids SingleLine truncating CJK fallback
// glyphs at a fractional measured width even when the panel has room.
func expiryBadge(c *ui.Context, l l10n.Localizer, expiresAt, now time.Time) {
	t := c.Theme()
	phase := expiryPhaseOf(expiresAt, now)
	look := expiryLookOf(t, phase)
	remaining := expiryRemaining(l, expiresAt, now)
	date := runway.SubscriptionExpiresOn(expiresAt, l.Language(), time.Local)
	dataInk := pick(!t.Dark, inkSecondary(t), ui.RGBA(255, 255, 255, 0.78))
	ui.Row(c).Shrink(0).Radius(radiusPlate).Clip().Border(1, look.stroke).Tooltip(subscriptionExpiryHelp(l, expiresAt)).Children(func() {
		ui.Row(c).Shrink(0).Padding(3, 7).Gap(5).AlignItems(ui.Center).Background(look.fill).Children(func() {
			if phase == expiryExpired {
				ui.Icon(c, icCalendarAlert).Size(10, 10).TextColor(look.foreground)
			} else {
				countdownRing(c, look.ring, float32(expiresAt.Sub(now))/float32(expiryWindow))
			}
			ui.Text(c, l.Text(expiryStatusKey(phase))).FontSize(fsCaption).FontWeight(600).TextColor(look.foreground).NoWrap().Shrink(0)
		})
		ui.Box(c).Width(1).Background(look.stroke.Alpha(0.5))
		ui.Row(c).Shrink(0).Padding(3, 7).Gap(4).AlignItems(ui.Center).Background(surfaceSunken(t)).Children(func() {
			ui.Text(c, date).FontSize(fsCaption).FontWeight(500).FontFeatures("tnum").TextColor(dataInk).NoWrap().Shrink(0)
			if remaining == "" {
				return
			}
			ui.Text(c, "·").FontSize(fsCaption).FontWeight(600).TextColor(dataInk.Alpha(0.45))
			ui.Text(c, remaining).FontSize(fsCaption).FontWeight(500).FontFeatures("tnum").TextColor(dataInk).NoWrap().Shrink(0)
		})
	})
}

// countdownRing draws the remaining fraction of the billing month as a tiny
// progress ring, clockwise from twelve o'clock.
func countdownRing(c *ui.Context, tint ui.Color, fraction float32) {
	fraction = min(1, max(0, fraction))
	ui.Box(c).Size(11, 11).Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy, radius := r.X+r.W/2, r.Y+r.H/2, r.W/2-1
		var ring ui.Path
		ring.Circle(cx, cy, radius)
		p.StrokePath(&ring, 2, tint.Alpha(0.25))
		if fraction > 0 {
			p.StrokePath(arcPath(cx, cy, radius, -math.Pi/2, float64(fraction)*2*math.Pi), 2, tint)
		}
	})
}

// arcPath is an open arc of a circle, built from cubic segments of at most a
// quarter turn. Angles are radians, clockwise on screen.
func arcPath(cx, cy, r float32, start, sweep float64) *ui.Path {
	path := &ui.Path{}
	segments := int(math.Ceil(math.Abs(sweep) / (math.Pi / 2)))
	if segments < 1 {
		segments = 1
	}
	step := sweep / float64(segments)
	k := 4.0 / 3.0 * math.Tan(step/4)
	point := func(a float64) (float64, float64) { return math.Cos(a), math.Sin(a) }
	x0, y0 := point(start)
	path.MoveTo(cx+r*float32(x0), cy+r*float32(y0))
	for i := 0; i < segments; i++ {
		a0, a1 := start+float64(i)*step, start+float64(i+1)*step
		ax, ay := point(a0)
		bx, by := point(a1)
		path.CubeTo(
			cx+r*float32(ax-k*ay), cy+r*float32(ay+k*ax),
			cx+r*float32(bx+k*by), cy+r*float32(by-k*bx),
			cx+r*float32(bx), cy+r*float32(by))
	}
	return path
}
