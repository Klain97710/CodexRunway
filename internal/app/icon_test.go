package app

import (
	"bytes"
	"image/png"
	"os"
	"runtime"
	"testing"
)

func TestTrayIconMatchesPlatform(t *testing.T) {
	data, template := trayIcon()
	if runtime.GOOS != "darwin" {
		if template || !bytes.Equal(data, desktopTrayIconPNG) {
			t.Fatal("desktop tray must start with the color application logo")
		}
		return
	}
	if !template {
		t.Error("tray icon should be a template image")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != trayIconWidth || b.Dy() != trayIconHeight {
		t.Errorf("size %v", b)
	}
	opaque := 0
	for y := 0; y < trayIconHeight; y++ {
		for x := 0; x < trayIconWidth; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a != 0 {
				opaque++
				if r != 0 || g != 0 || b != 0 {
					t.Fatal("template pixels must be black")
				}
			}
		}
	}
	if opaque == 0 || opaque >= trayIconWidth*trayIconHeight {
		t.Errorf("expected a glyph on transparent background, got %d pixels", opaque)
	}
}

func TestDesktopTrayIconIsSquareAndTransparent(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(desktopTrayIconPNG))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 64 {
		t.Fatalf("desktop tray icon must be 64x64, got %v", b)
	}
	if _, _, _, alpha := img.At(0, 0).RGBA(); alpha != 0 {
		t.Fatal("desktop tray icon lost transparent corners")
	}
}

func TestApplicationIconSourceIsHDAndTransparent(t *testing.T) {
	data, err := os.ReadFile("../../Resources/AppIcon.png")
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 1024 || b.Dy() != 1024 {
		t.Fatalf("application icon source is %v, want 1024x1024", b)
	}
	if _, _, _, alpha := img.At(0, 0).RGBA(); alpha != 0 {
		t.Fatal("application icon lost transparent corners")
	}
	if !bytes.Equal(data, appIconPNG) {
		t.Fatal("embedded application icon differs from Resources/AppIcon.png")
	}
}
