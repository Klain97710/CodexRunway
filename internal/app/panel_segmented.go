package app

import "github.com/egoist/mygo/ui"

// blueSegmented is the small segmented control of the Swift panel: a gray
// track, the chosen segment filled with the accent color. It reports whether
// the choice changed; the control keeps its natural width.
func blueSegmented(c *ui.Context, selected *int, disabled bool, labels ...string) bool {
	t := c.Theme()
	seg := ui.SegmentedBase(c, selected, len(labels))
	seg.Track.Padding(1).Radius(radiusRow).Background(mono(t, 0.07, 0.12)).Gap(0).AlignItems(ui.Center).Children(func() {
		for i, label := range labels {
			s := seg.Segment(i).Padding(3, 13).Radius(radiusRow - 1).Disabled(disabled).Cursor(ui.CursorPointer)
			ink := t.Text
			if i == *selected {
				fill := t.Accent
				if s.Pressed() {
					fill = t.AccentPressed
				} else if s.Hovered() {
					fill = t.AccentHover
				}
				s.Background(fill)
				ink = t.AccentText
			}
			if disabled {
				ink = ink.Alpha(0.5)
			}
			s.Children(func() { ui.Text(c, label).FontSize(fsCallout).FontWeight(500).TextColor(ink).SingleLine() })
		}
	})
	// Querying Changed applies the click now, while selected still points at
	// the caller's variable.
	return seg.Track.Changed()
}
