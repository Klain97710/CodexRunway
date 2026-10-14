package grok

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/codex"
)

func TestExportPackRoundTripsThroughPreviewAndImport(t *testing.T) {
	src := newTestStore(t)
	a := mustUpsert(t, src, credSpec{email: "a@example.com", user: "a", team: "T1"}.json(), true)
	idx, _ := src.LoadIndex()
	acc, _ := idx.Account(a.ID)
	acc.Alias = "Alpha"
	if err := src.UpdateMetadata(acc); err != nil {
		t.Fatal(err)
	}
	result, err := Exporter{Store: src, AppVersion: "1.2.3"}.Export([]string{a.ID}, testClock)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "export.json")
	if err := WritePack(path, result.Pack); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("pack mode = %v (%v)", info.Mode().Perm(), err)
		}
	}

	dst := newTestStore(t)
	importer := TransferImporter{Store: dst, Now: func() time.Time { return testClock }}
	preview := importer.PreviewFiles([]string{path})
	if len(preview.Items) != 1 || !preview.SourceIsPack || preview.RoutedProvider != codex.ProviderGrok {
		t.Fatalf("preview = %+v", preview)
	}
	item := preview.Items[0]
	if item.Preview.DisplayName != "Alpha" || item.Preview.Detail != "a@example.com" || item.Preview.Conflict.WillUpdate {
		t.Errorf("item = %+v", item.Preview)
	}
	batch, err := importer.ImportPreviewSelection(preview.Items, []string{item.ID()}, true)
	if err != nil || batch.SuccessCount() != 1 {
		t.Fatalf("batch %+v err %v", batch, err)
	}
	got, _ := dst.LoadIndex()
	imported, _ := got.Account(a.ID)
	if imported.Alias != "Alpha" || got.CurrentAccountID != a.ID {
		t.Errorf("imported = %+v, current %q", imported, got.CurrentAccountID)
	}
	// A second preview now reports the update.
	again := importer.PreviewFiles([]string{path})
	if !again.Items[0].Preview.Conflict.WillUpdate || again.Items[0].Preview.Conflict.ExistingDisplayName != "Alpha" {
		t.Errorf("conflict = %+v", again.Items[0].Preview.Conflict)
	}
}

func TestPackOfAnotherProviderIsRouted(t *testing.T) {
	store := newTestStore(t)
	pack := codex.NewTransferPack(codex.ProviderCodex, testClock, "1.0", []codex.TransferEntry{
		{ID: "x", Credential: []byte(`{"tokens":{}}`)},
	})
	if _, err := (TransferImporter{Store: store}).PreviewPack(pack); err == nil {
		t.Error("a Codex pack must not preview as Grok")
	} else {
		var mismatch *codex.ProviderMismatchError
		if !errors.As(err, &mismatch) {
			t.Errorf("err = %v", err)
		}
	}
	data, err := EncodePack(pack)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "codex-pack.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	preview := TransferImporter{Store: store}.PreviewFiles([]string{path})
	if len(preview.Items) != 0 || preview.RoutedProvider != codex.ProviderCodex || len(preview.Failures) != 1 {
		t.Errorf("preview = %+v", preview)
	}
}

func TestExportReportsMissingAccountsAndEmptySelection(t *testing.T) {
	store := newTestStore(t)
	a := mustUpsert(t, store, cred("a@example.com", "a"), false)
	if _, err := (Exporter{Store: store}).Export(nil, testClock); !errors.Is(err, codex.ErrEmptySelection) {
		t.Errorf("empty: %v", err)
	}
	if _, err := (Exporter{Store: store}).Export([]string{"ghost"}, testClock); !errors.Is(err, codex.ErrNoAccountsExported) {
		t.Errorf("nothing exportable: %v", err)
	}
	if err := os.Remove(store.CredentialPath(a.ID)); err != nil {
		t.Fatal(err)
	}
	result, err := Exporter{Store: store}.Export([]string{a.ID, "ghost"}, testClock)
	if err == nil {
		t.Fatalf("result = %+v", result)
	}
}

func TestPreviewFilesFailureMessagesHideContents(t *testing.T) {
	store := newTestStore(t)
	dir := t.TempDir()
	bad := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(bad, []byte("my secret token: abc123"), 0o600); err != nil {
		t.Fatal(err)
	}
	preview := TransferImporter{Store: store}.PreviewFiles([]string{bad, filepath.Join(dir, "missing.json")})
	if len(preview.Items) != 0 || len(preview.Failures) != 2 {
		t.Fatalf("preview = %+v", preview)
	}
	for _, f := range preview.Failures {
		if strings.Contains(f, "abc123") {
			t.Errorf("failure %q leaks file contents", f)
		}
	}
}

func TestPreviewDeduplicatesIdentitiesAcrossFiles(t *testing.T) {
	store := newTestStore(t)
	dir := t.TempDir()
	one := filepath.Join(dir, "one.json")
	two := filepath.Join(dir, "two.json")
	_ = os.WriteFile(one, cred("same@example.com", "same"), 0o600)
	_ = os.WriteFile(two, credSpec{email: "same@example.com", user: "same", access: "newer"}.json(), 0o600)
	preview := TransferImporter{Store: store}.PreviewFiles([]string{one, two})
	if len(preview.Items) != 1 {
		t.Errorf("%d items, want the identity once", len(preview.Items))
	}
}
