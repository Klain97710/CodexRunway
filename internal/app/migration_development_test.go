package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/licoy/quosparr/internal/personal"
)

func TestDevelopmentResourcesStayInsideSandbox(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, personal.SandboxMarker), []byte(personal.SandboxMarkerValue), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(personal.SandboxEnv, root)
	t.Setenv("CODEX_HOME", "")
	t.Setenv("GROK_HOME", "")
	st, dev, err := OpenStore()
	if err != nil || !dev {
		t.Fatalf("isolated store: %v", err)
	}
	s := NewSettings(st, DevSystem(), DevelopmentCredentials())
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InitializeFirstLaunch(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "preferences", "preferences.json")); err != nil {
		t.Fatal(err)
	}
	sys := DevSystem()
	if sys.LoginSupported || sys.OpenAtLogin != nil || sys.SetAutomaticUpdateChecks != nil || sys.Widgets {
		t.Fatal("development system integrations active")
	}
	n := DevelopmentNotifier()
	if n.Deliver([]Notification{{ID: "fixture"}})[0].Result != NotifyUnsupported {
		t.Fatal("real notification attempted")
	}
	t.Setenv("CODEX_HOME", filepath.Dir(root))
	if _, _, err := OpenStore(); err == nil {
		t.Fatal("external auth override accepted")
	}
}
