package app

import (
	"path/filepath"
	"strings"
)

// desktopEntryLaunchers start other programs, so their names do not tell
// an app's windows apart.
var desktopEntryLaunchers = map[string]bool{
	"sh": true, "bash": true, "flatpak": true, "snap": true, "gtk-launch": true,
	"python": true, "python3": true, "java": true, "node": true, "electron": true,
	"gjs": true, "perl": true, "ruby": true, "mono": true, "wine": true,
}

// desktopEntryKeys are the names the windows of a desktop entry's app go
// by: the entry's ID without .desktop (the application ID and usually the
// window class), its StartupWMClass, and the program its Exec starts.
func desktopEntryKeys(id, content string) []string {
	keys := []string{strings.TrimSuffix(id, ".desktop"), desktopEntryValue(content, "StartupWMClass")}
	for _, field := range strings.Fields(desktopEntryValue(content, "Exec")) {
		name := filepath.Base(strings.Trim(field, `"'`))
		if name == "env" || strings.Contains(field, "=") {
			continue // env and its variables
		}
		if !desktopEntryLaunchers[name] {
			keys = append(keys, name)
		}
		break
	}
	return lowerKeys(keys...)
}

// desktopEntryValue is a key's unlocalized value in the [Desktop Entry]
// group.
func desktopEntryValue(content, key string) string {
	group := ""
	for line := range strings.Lines(content) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			group = line
			continue
		}
		if group != "[Desktop Entry]" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(k) == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
