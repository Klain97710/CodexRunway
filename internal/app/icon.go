package app

import _ "embed"

const trayIconWidth, trayIconHeight = 140, 44

//go:embed assets/tray-placeholder.png
var trayIconPNG []byte

//go:embed assets/tray-app-icon.png
var desktopTrayIconPNG []byte

// trayIcon uses the application logo on desktop trays. The macOS menu bar
// starts with Swift's empty meters placeholder until its first snapshot.
func trayIcon() (png []byte, template bool) {
	if !trayIsTemplate() {
		return desktopTrayIconPNG, false
	}
	return trayIconPNG, true
}
