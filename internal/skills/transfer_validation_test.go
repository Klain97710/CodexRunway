package skills

import "testing"

func TestValidTransferNameReservesSyncedOnlyForClaude(t *testing.T) {
	if !validTransferName("synced", ProviderCodex) || !validTransferName("synced", ProviderGrok) {
		t.Fatal("Codex and Grok should allow a skill named synced")
	}
	if validTransferName("synced", ProviderClaude) {
		t.Fatal("Claude synced directory is reserved")
	}
	for _, name := range []string{"", ".", "..", ".system"} {
		if validTransferName(name, ProviderCodex) {
			t.Fatalf("hidden or empty name %q should be rejected", name)
		}
	}
}
