package grok

import "testing"

func TestWindowsGrokProcessRule(t *testing.T) {
	for _, tc := range []struct {
		name string
		pid  uint32
		want bool
	}{
		{"grok.exe", 101, true},
		{"GROK.EXE", 102, true},
		{`C:\Program Files\Grok\grok.exe`, 103, true},
		{"grok", 104, true},
		{"grok.exe", 42, false},
		{"grok-helper.exe", 105, false},
		{"node.exe grok", 106, false},
		{"notgrok.exe", 107, false},
		{"grok.exe.bak", 108, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pid, ok := windowsGrokProcessID(tc.pid, tc.name, 42)
			if ok != tc.want || (ok && pid != int(tc.pid)) {
				t.Fatalf("matched = %v, want %v", ok, tc.want)
			}
		})
	}
}

func TestUnixGrokRuleDoesNotAcceptWindowsSuffix(t *testing.T) {
	if isGrokExecutable("/usr/bin/grok.exe", false) {
		t.Fatal("the Swift ps rule only accepts grok")
	}
}
