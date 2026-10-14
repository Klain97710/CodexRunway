//go:build darwin

package app

import (
	"fmt"
	"strings"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

// trayIsTemplate: the menu bar tints a black-and-transparent icon itself.
func trayIsTemplate() bool { return true }

func trayLight() bool {
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	app := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	appearance := app.Send(objc.RegisterName("effectiveAppearance"))
	if button := trayButton(); button != 0 {
		appearance = button.Send(objc.RegisterName("effectiveAppearance"))
	}
	names := objc.ID(objc.GetClass("NSArray")).Send(objc.RegisterName("arrayWithObjects:"), nsStr("NSAppearanceNameAqua"), nsStr("NSAppearanceNameDarkAqua"), objc.ID(0))
	match := appearance.Send(objc.RegisterName("bestMatchFromAppearancesWithNames:"), names)
	return !objc.Send[bool](match, objc.RegisterName("isEqualToString:"), nsStr("NSAppearanceNameDarkAqua"))
}

// MyGo's tray images are normally 18 pt high; these complete contents use
// the Swift status item's 22 pt coordinate system, including two text rows
func setTrayContent(tray *mygo.Tray, r TrayRender, _ bool) error {
	foreground, err := renderTrayImage(r, trayImageOptions{light: true, foreground: true})
	if err != nil {
		return err
	}
	colors, err := renderTrayImage(r, trayImageOptions{colorsOnly: true})
	if err != nil {
		return err
	}
	tray.SetTitle("")
	button := trayButton()
	if button == 0 {
		return fmt.Errorf("status item button unavailable")
	}
	image, err := trayNativeImage(foreground)
	if err != nil {
		return err
	}
	defer image.Send(objc.RegisterName("release"))
	s := objc.Send[traySize](image, objc.RegisterName("size"))
	if s.Height > 0 {
		image.Send(objc.RegisterName("setSize:"), traySize{Width: s.Width * 22 / s.Height, Height: 22})
		image.Send(objc.RegisterName("setTemplate:"), true)
		// NSStatusBarButton sizes the highlight/hit area when its image is
		// assigned; changing the 18pt framework image afterwards is too late.
		button.Send(objc.RegisterName("setImage:"), image)
		return setTrayColorOverlay(button, colors)
	}
	return fmt.Errorf("status item image has no height")
}

// setTrayLines shows lines one over the other, in a small font, as the title
// of the menu bar item. mygo sets only plain titles, so the item's button is
// found among the app's windows, by the action mygo gives it, and given an
// attributed title. Without a color of its own the text takes the menu bar's.
// No lines clears the title.
func setTrayLines(tray *mygo.Tray, lines []string) {
	if len(lines) == 0 {
		tray.SetTitle("")
		return
	}
	mygo.RunOnMain(func() {
		button := trayButton()
		if button == 0 {
			tray.SetTitle(strings.Join(lines, " · "))
			return
		}
		pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
		defer pool.Send(objc.RegisterName("drain"))

		const size, lineHeight = 9.0, 10.0
		para := objc.ID(objc.GetClass("NSMutableParagraphStyle")).Send(objc.RegisterName("new"))
		defer para.Send(objc.RegisterName("release"))
		para.Send(objc.RegisterName("setMinimumLineHeight:"), float64(lineHeight))
		para.Send(objc.RegisterName("setMaximumLineHeight:"), float64(lineHeight))
		const weightMedium = 0.23 // NSFontWeightMedium
		font := objc.ID(objc.GetClass("NSFont")).Send(objc.RegisterName("monospacedDigitSystemFontOfSize:weight:"), float64(size), float64(weightMedium))
		// The lines are lowered to sit around the middle of the bar, beside
		// the bars of the icon, which are lineHeight apart.
		offset := objc.ID(objc.GetClass("NSNumber")).Send(objc.RegisterName("numberWithDouble:"), -lineHeight*float64(len(lines)-1)/2+0.75)

		attrs := objc.ID(objc.GetClass("NSMutableDictionary")).Send(objc.RegisterName("dictionary"))
		set := objc.RegisterName("setObject:forKey:")
		attrs.Send(set, font, nsStr("NSFont"))
		attrs.Send(set, para, nsStr("NSParagraphStyle"))
		attrs.Send(set, offset, nsStr("NSBaselineOffset"))

		title := objc.ID(objc.GetClass("NSAttributedString")).Send(objc.RegisterName("alloc")).
			Send(objc.RegisterName("initWithString:attributes:"), nsStr(strings.Join(lines, "\n")), attrs)
		defer title.Send(objc.RegisterName("release"))
		button.Send(objc.RegisterName("setAttributedTitle:"), title)
	})
}

// trayButton is the button of the app's menu bar item: the content of a
// status bar window, which mygo has send mygoTrayClicked:. 0 when none.
func trayButton() objc.ID {
	app := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	windows := app.Send(objc.RegisterName("windows"))
	action := objc.RegisterName("mygoTrayClicked:")
	n := objc.Send[uint](windows, objc.RegisterName("count"))
	for i := range n {
		w := windows.Send(objc.RegisterName("objectAtIndex:"), i)
		if b := findTrayButton(w.Send(objc.RegisterName("contentView")), action); b != 0 {
			return b
		}
	}
	return 0
}

// findTrayButton looks for the button with action in view and its subviews.
func findTrayButton(view objc.ID, action objc.SEL) objc.ID {
	if view == 0 {
		return 0
	}
	if objc.Send[bool](view, objc.RegisterName("respondsToSelector:"), objc.RegisterName("action")) &&
		objc.Send[objc.SEL](view, objc.RegisterName("action")) == action {
		return view
	}
	subviews := view.Send(objc.RegisterName("subviews"))
	n := objc.Send[uint](subviews, objc.RegisterName("count"))
	for i := range n {
		if b := findTrayButton(subviews.Send(objc.RegisterName("objectAtIndex:"), i), action); b != 0 {
			return b
		}
	}
	return 0
}

func nsStr(s string) objc.ID {
	str := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("alloc")).
		Send(objc.RegisterName("initWithUTF8String:"), s)
	return str.Send(objc.RegisterName("autorelease"))
}
