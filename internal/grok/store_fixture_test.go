package grok

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/codex"
)

type swiftPreviewFixture struct {
	Failures     []string `json:"failures"`
	SourceIsPack bool     `json:"sourceIsPack"`
	Items        []struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
		Detail      string `json:"detail"`
		Conflict    string `json:"conflict"`
		Existing    string `json:"existing"`
		Alias       string `json:"alias"`
		Credential  string `json:"credential"`
	} `json:"items"`
}

func checkPreview(t *testing.T, name string, got ImportPreview, want swiftPreviewFixture) {
	t.Helper()
	if !equalStrings(got.Failures, want.Failures) {
		t.Errorf("%s failures = %v, want %v", name, got.Failures, want.Failures)
	}
	if got.SourceIsPack != want.SourceIsPack {
		t.Errorf("%s sourceIsPack = %v", name, got.SourceIsPack)
	}
	if len(got.Items) != len(want.Items) {
		t.Fatalf("%s: %d items, want %d", name, len(got.Items), len(want.Items))
	}
	for i, w := range want.Items {
		g := got.Items[i]
		conflict, existing := "willAdd", ""
		if g.Preview.Conflict.WillUpdate {
			conflict, existing = "willUpdate", g.Preview.Conflict.ExistingDisplayName
		}
		switch {
		case g.Preview.ID != w.ID, g.Preview.DisplayName != w.DisplayName, g.Preview.Detail != w.Detail,
			conflict != w.Conflict, existing != w.Existing, g.Alias != w.Alias, string(g.CredentialData) != w.Credential:
			t.Errorf("%s item %d differs: id=%s/%s name=%q/%q detail=%q/%q conflict=%s/%s existing=%q/%q alias=%q/%q cred-equal=%v",
				name, i, g.Preview.ID, w.ID, g.Preview.DisplayName, w.DisplayName, g.Preview.Detail, w.Detail,
				conflict, w.Conflict, existing, w.Existing, g.Alias, w.Alias, string(g.CredentialData) == w.Credential)
		}
	}
}

// TestSwiftStoreSequence replays the generator's store session and compares
// the bytes the Swift app wrote (index, credentials, pack) and the previews.
func TestSwiftStoreSequence(t *testing.T) {
	var fx struct {
		IDs           []string            `json:"ids"`
		Index         string              `json:"index"`
		Credentials   map[string]string   `json:"credentials"`
		Ordered       []string            `json:"ordered"`
		Pack          string              `json:"pack"`
		ExportedCount int                 `json:"exportedCount"`
		Failures      []string            `json:"failures"`
		WithStore     swiftPreviewFixture `json:"previewWithStore"`
		EmptyStore    swiftPreviewFixture `json:"previewEmptyStore"`
		Paste         swiftPreviewFixture `json:"previewPaste"`
		None          swiftPreviewFixture `json:"previewNone"`
		Sources       []string            `json:"sources"`
	}
	readFixture(t, "account_swift_store.json", &fx)

	// The quota the Swift app cached for the second account.
	fixtureIndex, err := ParseAccountIndex([]byte(fx.Index))
	if err != nil {
		t.Fatal(err)
	}
	quota, ok := fixtureIndex.Account(fx.IDs[1])
	if !ok || len(quota.CachedQuota) == 0 {
		t.Fatal("fixture lacks the cached quota")
	}

	store := newTestStore(t)
	t0 := time.Unix(1_785_499_200, 123_500_000)
	up := func(data string, current bool, offset time.Duration) ManagedAccount {
		acc, err := store.UpsertCredentialData([]byte(data), UpsertOptions{MakeCurrent: current, Now: t0.Add(offset)})
		if err != nil {
			t.Fatal(err)
		}
		return acc
	}
	a1 := up(fx.Sources[0], true, 0)
	a2 := up(fx.Sources[1], false, 60*time.Second)
	a3 := up(fx.Sources[2], false, 120*time.Second)
	up(fx.Sources[3], false, 180*time.Second)
	if got := []string{a1.ID, a2.ID, a3.ID}; !equalStrings(got, fx.IDs) {
		t.Fatalf("ids = %v, want %v", got, fx.IDs)
	}

	idx, _ := store.LoadIndex()
	m2, _ := idx.Account(a2.ID)
	m2.Alias = "Alpha / é"
	m2 = m2.ApplyQuota(quota.CachedQuota, time.Unix(1_785_499_500, 250_000_000))
	if err := store.UpdateMetadata(m2); err != nil {
		t.Fatal(err)
	}
	idx, _ = store.LoadIndex()
	m3, _ := idx.Account(a3.ID)
	if err := store.UpdateMetadata(m3.ApplyError("authentication_required", true)); err != nil {
		t.Fatal(err)
	}
	if err := store.Reorder([]string{a3.ID, a1.ID}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCurrentAccountID(a2.ID, t0.Add(300*time.Second)); err != nil {
		t.Fatal(err)
	}

	text, err := os.ReadFile(store.IndexPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != fx.Index {
		t.Errorf("grok-index.json differs from Swift:\n%s\nwant\n%s", text, fx.Index)
	}
	for id, want := range fx.Credentials {
		got, err := store.LoadCredentialData(id)
		if err != nil || string(got) != want {
			t.Errorf("credential %s differs (err %v)", id, err)
		}
	}
	idx, _ = store.LoadIndex()
	var ordered []string
	for _, a := range idx.OrderedAccounts() {
		ordered = append(ordered, a.ID)
	}
	if !equalStrings(ordered, fx.Ordered) {
		t.Errorf("order = %v, want %v", ordered, fx.Ordered)
	}

	result, err := Exporter{Store: store, AppVersion: "0.0.87"}.Export(
		[]string{a1.ID, a2.ID, a3.ID, "missing"}, time.Unix(1_785_500_000, 500_000_000))
	if err != nil {
		t.Fatal(err)
	}
	if result.ExportedCount != fx.ExportedCount || !equalStrings(result.Failures, fx.Failures) {
		t.Errorf("export count %d failures %v", result.ExportedCount, result.Failures)
	}
	pack, err := EncodePack(result.Pack)
	if err != nil {
		t.Fatal(err)
	}
	// Only the personal export format name changes; the frozen fixture stays intact.
	expectedPack := strings.Replace(fx.Pack, `"format" : "quosparr-accounts"`, `"format" : "codex-runway-accounts"`, 1)
	if string(pack) != expectedPack {
		t.Errorf("pack differs from Swift:\n%s\nwant\n%s", pack, fx.Pack)
	}

	dir := t.TempDir()
	packFile := filepath.Join(dir, "pack.json")
	if err := os.WriteFile(packFile, pack, 0o600); err != nil {
		t.Fatal(err)
	}
	checkPreview(t, "with store", TransferImporter{Store: store}.PreviewFiles([]string{packFile}), fx.WithStore)

	empty := newTestStore(t)
	checkPreview(t, "empty store", TransferImporter{Store: empty}.PreviewFiles([]string{packFile}), fx.EmptyStore)
	pasteFile := filepath.Join(dir, "paste.txt")
	pasteText := "{\"email\":\"p1@example.com\",\"key\":\"k1\",\"user_id\":\"u1\"}\n{\"email\":\"p2@example.com\",\"key\":\"k2\",\"user_id\":\"u2\"}"
	if err := os.WriteFile(pasteFile, []byte(pasteText), 0o600); err != nil {
		t.Fatal(err)
	}
	checkPreview(t, "paste", TransferImporter{Store: empty}.PreviewFiles([]string{pasteFile}), fx.Paste)
	checkPreview(t, "none", TransferImporter{Store: empty}.PreviewFiles(nil), fx.None)

	// The Swift-written pack decodes and its preview ids are stable.
	decoded, ok := decodePack([]byte(fx.Pack))
	if !ok || decoded.Provider != codex.ProviderGrok {
		t.Errorf("Swift pack not accepted: ok=%v", ok)
	}
}
