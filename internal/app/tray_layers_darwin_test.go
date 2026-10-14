//go:build darwin

package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/licoy/quosparr/internal/store"
)

// AppKit must initialize on the process main thread, before Go test workers
func init() {
	if os.Getenv("RUNWAY_TRAY_NATIVE_CHECK") == "1" {
		if err := verifyNativeTrayLayers(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}

func TestTrayNativeLayerGeometryAndHitTesting(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, exe, "-test.run=^$")
	cmd.Env = append(os.Environ(), "RUNWAY_TRAY_NATIVE_CHECK=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native layers: %v: %s", err, out)
	}
}

func verifyNativeTrayLayers() error {
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	button := objc.ID(objc.GetClass("NSButton")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithFrame:"), trayRect{Size: traySize{Width: 180, Height: 24}})
	defer button.Send(objc.RegisterName("release"))
	button.Send(objc.RegisterName("setTitle:"), nsStr(""))
	button.Send(objc.RegisterName("setImagePosition:"), uint64(1)) // NSImageOnly
	for _, style := range []store.StatusBarStyle{store.StatusBarStyleMeters, store.StatusBarStyleText, store.StatusBarStyleBattery, store.StatusBarStyleRings, store.StatusBarStyleCountdown} {
		r := TrayFromStatus(trayTestStatus(style, nil))
		foreground, err := renderTrayImage(r, trayImageOptions{light: true, foreground: true})
		if err != nil {
			return err
		}
		image, err := trayNativeImage(foreground)
		if err != nil {
			return err
		}
		image.Send(objc.RegisterName("setTemplate:"), true)
		image.Send(objc.RegisterName("setSize:"), traySize{Width: 120, Height: 22})
		button.Send(objc.RegisterName("setImage:"), image)
		image.Send(objc.RegisterName("release"))
		colors, err := renderTrayImage(r, trayImageOptions{colorsOnly: true})
		if err != nil {
			return err
		}
		if err := setTrayColorOverlay(button, colors); err != nil {
			return err
		}
		view := button.Send(objc.RegisterName("viewWithTag:"), int64(0x52575443))
		if got := view.Send(objc.RegisterName("hitTest:"), trayPoint{X: 10, Y: 10}); got != 0 {
			return fmt.Errorf("color overlay intercepts clicks")
		}
		if objc.Send[bool](view, objc.RegisterName("isAccessibilityElement")) {
			return fmt.Errorf("color overlay duplicates accessibility content")
		}
		subviews := button.Send(objc.RegisterName("subviews"))
		count := 0
		for i := range objc.Send[uint64](subviews, objc.RegisterName("count")) {
			child := subviews.Send(objc.RegisterName("objectAtIndex:"), i)
			if objc.Send[bool](child, objc.RegisterName("isKindOfClass:"), objc.GetClass("RunwayTrayColorOverlay")) {
				count++
			}
		}
		if count != 1 {
			return fmt.Errorf("style switch left %d overlays", count)
		}
		for _, width := range []float64{160, 260} {
			button.Send(objc.RegisterName("setFrameSize:"), traySize{Width: width, Height: 24})
			bounds := objc.Send[trayRect](button, objc.RegisterName("bounds"))
			want := objc.Send[trayRect](button.Send(objc.RegisterName("cell")), objc.RegisterName("imageRectForBounds:"), bounds)
			if got := objc.Send[trayRect](view, objc.RegisterName("frame")); got != want {
				return fmt.Errorf("overlay frame %v, native image frame %v", got, want)
			}
		}
	}
	return nil
}
