package grok

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSessionScanPropagatesIOErrors(t *testing.T) {
	for _, broken := range []string{"sessions", "updates.jsonl"} {
		t.Run(broken, func(t *testing.T) {
			home := t.TempDir()
			if broken == "sessions" {
				if err := os.WriteFile(filepath.Join(home, "sessions"), []byte("not a directory"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				sessionWrite(t, home, "one", "project", "grok", "2026-01-01T00:00:00Z", "")
				path := filepath.Join(home, "sessions", "%2Ftmp", "one", "updates.jsonl")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := (SessionScanner{GrokHome: home}).Scan(context.Background(), DefaultSessionScanOptions(testClock)); err == nil {
				t.Fatal("IO failure reported as successful empty usage")
			}
		})
	}
}
