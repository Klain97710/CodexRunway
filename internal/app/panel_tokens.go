package app

import "github.com/egoist/mygo/ui"

// Design tokens of the main panel, taken from the Swift RunwaySurface and
// from the point sizes of the SwiftUI text styles on macOS.
const (
	fsCaption  float32 = 10 // .caption, .caption2
	fsCallout  float32 = 12 // .callout
	fsBody     float32 = 13 // .body, .headline
	fsHeadline float32 = 13
	fsTitle3   float32 = 15 // .title3

	radiusControl float32 = 5
	radiusRow     float32 = 6
	radiusCard    float32 = 10
	radiusPlate   float32 = 5
)

// sysTint names an AppKit system color.
type sysTint int

const (
	sysGreen sysTint = iota
	sysOrange
	sysRed
	sysBlue
	sysPurple
	sysYellow
	sysTeal
	sysGray
)

var sysHex = [...][2]string{
	sysGreen:  {"#34C759", "#30D158"},
	sysOrange: {"#FF9500", "#FF9F0A"},
	sysRed:    {"#FF3B30", "#FF453A"},
	sysBlue:   {"#007AFF", "#0A84FF"},
	sysPurple: {"#AF52DE", "#BF5AF2"},
	sysYellow: {"#FFCC00", "#FFD60A"},
	sysTeal:   {"#30B0C7", "#40CBE0"},
	sysGray:   {"#8E8E93", "#98989D"},
}

var sysColors [len(sysHex)][2]ui.Color

func init() {
	for i, h := range sysHex {
		sysColors[i] = [2]ui.Color{ui.Hex(h[0]), ui.Hex(h[1])}
	}
}

// sys is the system color in the theme's appearance.
func sys(t *ui.Theme, k sysTint) ui.Color {
	if t.Dark {
		return sysColors[k][1]
	}
	return sysColors[k][0]
}

// ink is a label color: primary, secondary or tertiary.
func inkSecondary(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(255, 255, 255, 0.55)
	}
	return ui.RGBA(0, 0, 0, 0.5)
}

func inkTertiary(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(255, 255, 255, 0.25)
	}
	return ui.RGBA(0, 0, 0, 0.26)
}

func mono(t *ui.Theme, light, dark float32) ui.Color {
	if t.Dark {
		return ui.RGBA(255, 255, 255, dark)
	}
	return ui.RGBA(0, 0, 0, light)
}

// Surfaces of RunwaySurface: raised for cards and rows, sunken for tables
// and progress tracks, tableHead for column headers, hover for pointers.
func surfaceRaised(t *ui.Theme) ui.Color    { return mono(t, 0.045, 0.065) }
func surfaceSunken(t *ui.Theme) ui.Color    { return mono(t, 0.03, 0.045) }
func surfaceTableHead(t *ui.Theme) ui.Color { return mono(t, 0.04, 0.05) }
func hoverNeutral(t *ui.Theme) ui.Color     { return mono(t, 0.06, 0.09) }

// hairlineColor is separatorColor; the faint one is for table rows.
func hairlineColor(t *ui.Theme) ui.Color      { return mono(t, 0.10, 0.10) }
func hairlineFaintColor(t *ui.Theme) ui.Color { return mono(t, 0.055, 0.055) }

type surfaceKind int

const (
	surfaceRaisedKind surfaceKind = iota
	surfaceSunkenKind
)

// cardColumn is a runwayCard: a translucent wash, plus a hairline stroke in dark.
func cardColumn(c *ui.Context, kind surfaceKind, radius float32) ui.Element {
	t := c.Theme()
	fill := surfaceRaised(t)
	if kind == surfaceSunkenKind {
		fill = surfaceSunken(t)
	}
	col := ui.Column(c).Radius(radius).Background(fill)
	if t.Dark {
		col.Border(1, hairlineColor(t))
	}
	return col
}

// hairline is the 1pt rule between sections.
func hairline(c *ui.Context) ui.Element {
	return ui.Box(c).Height(1).Background(hairlineColor(c.Theme()))
}

func hairlineFaint(c *ui.Context) ui.Element {
	return ui.Box(c).Height(1).Background(hairlineFaintColor(c.Theme()))
}
