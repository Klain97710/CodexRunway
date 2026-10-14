//go:build darwin

package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSessionLoginMigrationFindsPreviousBundleIdentity(t *testing.T) {
	root := t.TempDir()
	paths := map[uintptr]string{}
	for i, id := range []string{"com.github.codex-runway", "com.github.quosparr", "com.quosparr.app", "org.example.other"} {
		path := filepath.Join(root, id+".app")
		if err := os.MkdirAll(filepath.Join(path, "Contents"), 0o700); err != nil {
			t.Fatal(err)
		}
		plist := `<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>` + id + `</string></dict></plist>`
		if err := os.WriteFile(filepath.Join(path, "Contents", "Info.plist"), []byte(plist), 0o600); err != nil {
			t.Fatal(err)
		}
		paths[uintptr(i+1)] = path
	}
	var removed []uintptr
	api := sharedLoginAPI{
		count:   func(uintptr) int { return len(paths) },
		value:   func(_ uintptr, i int) uintptr { return uintptr(i + 1) },
		resolve: func(item uintptr, _ uint32, _ uintptr) uintptr { return item },
		path:    func(url uintptr) string { return paths[url] },
		release: func(uintptr) {},
		remove:  func(_ uintptr, item uintptr) int32 { removed = append(removed, item); return 0 },
	}
	// No current-path match: only validated previous product identities qualify.
	found, err := matchingSessionLoginItems(&api, 10, 20, "", true)
	if err != nil || !found || !slices.Equal(removed, []uintptr{1, 2}) {
		t.Fatalf("previous login items were not selected safely: found=%v removed=%v err=%v", found, removed, err)
	}
}

func TestSessionLoginItemsMatchAndRemoveOnlyThisBundle(t *testing.T) {
	paths := map[uintptr]string{
		1: "/Applications/Quosparr.app",
		2: "/Applications/Other.app",
		3: "/Applications/Quosparr.app", // duplicate resolved bundle path
	}
	var removed, released []uintptr
	api := sharedLoginAPI{
		count: func(uintptr) int { return 4 },
		value: func(_ uintptr, index int) uintptr { return uintptr(index + 1) },
		resolve: func(item uintptr, flags uint32, outErr uintptr) uintptr {
			if flags != 3 || outErr != 0 {
				t.Fatal("resolution may interact with the user or mount volumes")
			}
			if item == 4 {
				return 0 // stale item, skipped like Swift
			}
			return item
		},
		path: func(url uintptr) string { return paths[url] },
		remove: func(_ uintptr, item uintptr) int32 {
			removed = append(removed, item)
			return 0
		},
		release: func(url uintptr) { released = append(released, url) },
	}
	found, err := matchingSessionLoginItems(&api, 10, 20, "/Applications/Quosparr.app", true)
	if err != nil || !found || !slices.Equal(removed, []uintptr{1, 3}) || !slices.Equal(released, []uintptr{1, 2, 3}) {
		t.Fatalf("found %v err %v removed %v released %v", found, err, removed, released)
	}
	removed = nil
	if found, err := matchingSessionLoginItems(&api, 10, 20, "/Applications/Quosparr.app", false); err != nil || !found || len(removed) != 0 {
		t.Fatalf("read changed the list: found %v err %v removed %v", found, err, removed)
	}
	api.remove = func(uintptr, uintptr) int32 { return -50 }
	if _, err := matchingSessionLoginItems(&api, 10, 20, "/Applications/Quosparr.app", true); err == nil || !strings.Contains(err.Error(), "-50") {
		t.Fatalf("removal failure hidden: %v", err)
	}
}

func TestPreviousLaunchAgentsRequireMatchingBundleAndCommand(t *testing.T) {
	home := t.TempDir()
	app := filepath.Join(home, "Previous.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents/Info.plist"), []byte(`<plist><dict><key>CFBundleIdentifier</key><string>com.github.codex-runway</string></dict></plist>`), 0600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, "Library/LaunchAgents")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "com.github.codex-runway.plist")
	content := `<plist><dict><key>Label</key><string>com.github.codex-runway</string><key>ProgramArguments</key><array><string>/usr/bin/open</string><string>-a</string><string>` + app + `</string><string>--args</string><string>--mygo-login</string></array><key>RunAtLoad</key><true/><key>Disabled</key><true/></dict></plist>`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	agents, err := (macLegacyLogin{home: home}).agents()
	if err != nil || len(agents) != 1 || agents[0].enabled {
		t.Fatalf("disabled state not retained: %v %v", agents, err)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(content, "/usr/bin/open", "/bin/sh")), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := (macLegacyLogin{home: home}).agents(); err == nil {
		t.Fatal("changed command accepted")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("unrecognised item removed")
	}
}
