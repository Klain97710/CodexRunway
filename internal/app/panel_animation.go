package app

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/resettoday"
)

// The Swift badge sweeps for 2.4 seconds, then rests for 1.2 seconds.
func drawTierSheen(p *ui.Painter, r ui.Rect, color ui.Color) {
	cycle := p.Now().Sub(time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)) % (3600 * time.Millisecond)
	if cycle >= 2400*time.Millisecond {
		p.After(3600*time.Millisecond - cycle)
		return
	}
	p.After(time.Second / 30)
	progress := float32(cycle.Seconds() / 2.4)
	band := max(10, min(16, r.W*0.22))
	margin := max(band, r.H) * 1.15
	center := -margin + progress*(r.W+2*margin)
	slope := r.H / max(1, r.W)
	p.Clip(r, r.H/2, func() {
		// Small adjacent strips form the soft diagonal highlight, without
		// rebuilding the view or laying out the badge on animation frames.
		for i := 0; i < 24; i++ {
			x := center - band/2 + float32(i)*band/24
			w := band/24 + 0.1
			var strip ui.Path
			strip.MoveTo(r.X+x+slope*r.H/2, r.Y)
			strip.LineTo(r.X+x+w+slope*r.H/2, r.Y)
			strip.LineTo(r.X+x+w-slope*r.H/2, r.Y+r.H)
			strip.LineTo(r.X+x-slope*r.H/2, r.Y+r.H)
			strip.Close()
			alpha := float32(math.Pow(1-math.Abs((float64(i)+0.5)/12-1), 2))
			p.FillPath(&strip, color.Alpha(alpha))
		}
	})
}

func reactionFloat(c *ui.Context, button ui.Element, rs *panelReactionState) {
	if rs.delta <= 0 {
		return
	}
	elapsed := c.Now().Sub(rs.deltaAt).Seconds()
	if elapsed >= 2 {
		return
	}
	c.AnimationFrame()
	progress := float32(min(1, max(0, elapsed)))
	offset := 12 * (1 - (1-progress)*(1-progress))
	if c.Preferences().ReduceMotion {
		offset = 2
	}
	ui.Overlay(c, func() {
		ui.Text(c, resettoday.FormatDelta(rs.delta)).Absolute().PassThrough().Role(ui.RoleNone).
			AttachTo(button, ui.AnchorTopRight, ui.AnchorBottomRight).Margin(0, 2, offset, 0).
			FontSize(9).FontWeight(500).TextColor(inkSecondary(c.Theme())).Opacity(float32(min(1, 2-elapsed)))
	})
}

// Dense chart cells expose details immediately and keep the overlay out of
// hit testing, so moving across neighbouring days never interrupts hover.
func chartHoverTip(c *ui.Context, cell ui.Element, detail string) {
	cell.Description(detail)
	if !cell.Hovered() {
		return
	}
	fill, ink := inverseColors(c.Theme())
	ui.Overlay(c, func() {
		ui.Box(c).Absolute().PassThrough().Role(ui.RoleTooltip).
			AttachTo(cell, ui.AnchorTop, ui.AnchorBottom).Margin(0, 0, 6, 0).
			MaxWidth(280).Padding(6, 8).Radius(5).Background(fill).TextColor(ink).
			Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.2)).Children(func() {
			ui.Text(c, detail).FontSize(fsCaption)
		})
	})
}
