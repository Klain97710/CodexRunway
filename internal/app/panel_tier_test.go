package app

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTierFillHasNoCenterColorJump(t *testing.T) {
	background := ui.RGB(45, 45, 45)
	for _, light := range []bool{false, true} {
		for _, look := range []tierLook{silverLook(light), goldLook(light), blackGoldLook(light), platinumLook(light)} {
			for _, height := range []float32{12, 18} {
				img := ui.Render(func(c *ui.Context) {
					ui.Box(c).Fill().Background(background).Draw(func(p *ui.Painter, r ui.Rect) {
						fillStops(p, ui.Rect{X: 8, Y: 8, W: 54, H: height}, look.fill, height/2)
					})
				}, 70, 40, 2)
				for _, y := range []int{16 + int(height/2), 16 + int(height*1.5)} {
					left := color.NRGBAModel.Convert(img.At(69, y)).(color.NRGBA)
					right := color.NRGBAModel.Convert(img.At(70, y)).(color.NRGBA)
					if absChannel(left.R, right.R) > 8 || absChannel(left.G, right.G) > 8 || absChannel(left.B, right.B) > 8 {
						t.Fatalf("gradient jumps at center, light=%v height=%v: %v -> %v", light, height, left, right)
					}
				}
				corner := color.NRGBAModel.Convert(img.At(16, 16)).(color.NRGBA)
				if corner.R != background.R || corner.G != background.G || corner.B != background.B {
					t.Fatalf("fill leaked outside capsule corner: %v", corner)
				}
			}
		}
	}
}
