package codex

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newImporter(t *testing.T) (*Importer, testEnv) {
	e := newEnv(t)
	return &Importer{Store: e.store, Now: func() time.Time { return testNow }}, e
}

func pasteAuthJSON(t *testing.T, refresh string) string {
	return `{"auth_mode":"chatgpt","tokens":{"access_token":"` + jwtExp(t, farFuture) + `","refresh_token":"` + refresh + `","account_id":"paste-1","id_token":"` +
		jwtOf(t, map[string]any{"email": "paste@example.com", "https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "paste-1"}}) + `"}}`
}

func TestPasteAuthJSON(t *testing.T) {
	im, _ := newImporter(t)
	batch := im.ImportPastedText(ctxBG(), pasteAuthJSON(t, "refresh-paste-token-long-enough"), false)
	if len(batch.Succeeded) != 1 || len(batch.Failures) != 0 {
		t.Fatalf("batch: %+v", batch)
	}
	if acc := batch.Succeeded[0]; acc.Email != "paste@example.com" || acc.AccountID != "paste-1" {
		t.Fatalf("account: %+v", acc)
	}
}

func TestPasteJSONSurroundedByNotes(t *testing.T) {
	im, _ := newImporter(t)
	text := "Here is my login:\n" + pasteAuthJSON(t, "refresh-paste-token-long-enough") + "\nthanks!"
	if batch := im.ImportPastedText(ctxBG(), text, false); len(batch.Succeeded) != 1 {
		t.Fatalf("batch: %+v", batch)
	}
}

func TestPasteSessionJSON(t *testing.T) {
	im, e := newImporter(t)
	access := jwtOf(t, map[string]any{"email": "session@example.com", "exp": farFuture,
		"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "sess-acct-1", "chatgpt_plan_type": "plus"}})
	session := `{"user":{"id":"user-1","email":"session@example.com","name":"Session User"},"expires":"2099-01-01T00:00:00.000Z",
		"account":{"id":"sess-acct-1","planType":"plus","structure":"personal"},"accessToken":"` + access + `","authProvider":"openai"}`
	batch := im.ImportPastedText(ctxBG(), session, false)
	if len(batch.Succeeded) != 1 || len(batch.Failures) != 0 {
		t.Fatalf("batch: %+v", batch)
	}
	acc := batch.Succeeded[0]
	cred := must(e.store.LoadCredential(acc.ID))
	if acc.Email != "session@example.com" || acc.AccountID != "sess-acct-1" || cred.Tokens.AccessToken != access || cred.Tokens.AccountID != "sess-acct-1" {
		t.Fatalf("account %+v", acc)
	}
}

func TestSessionsWithoutWorkspaceStaySeparate(t *testing.T) {
	im, e := newImporter(t)
	mk := func(jti string) string {
		return jwtOf(t, map[string]any{"email": "shared@example.com", "exp": farFuture, "jti": jti,
			"https://api.openai.com/auth": map[string]any{"chatgpt_user_id": "user-shared"}})
	}
	text := `[{"user":{"email":"shared@example.com"},"accessToken":"` + mk("session-one") + `","authProvider":"openai"},
	{"user":{"email":"shared@example.com"},"accessToken":"` + mk("session-two") + `","authProvider":"openai"}]`
	batch := im.ImportPastedText(ctxBG(), text, false)
	idx := must(e.store.LoadIndex())
	if len(batch.Succeeded) != 2 || len(idx.Accounts) != 2 {
		t.Fatalf("batch %+v, %d rows", batch, len(idx.Accounts))
	}
	for _, a := range idx.Accounts {
		if a.AccountID != "" {
			t.Errorf("synthetic workspace id %q", a.AccountID)
		}
	}
}

func TestPasteUnrecognizedJSON(t *testing.T) {
	im, _ := newImporter(t)
	for _, text := range []string{`{"hello":"world","foo":1}`, "", "   \n ", "short", `[1,2,3]`} {
		batch := im.ImportPastedText(ctxBG(), text, false)
		if len(batch.Succeeded) != 0 || len(batch.Failures) != 1 || batch.Failures[0] != FailureNoCredentials {
			t.Errorf("%q: %+v", text, batch)
		}
	}
}

func TestParseCandidatesShapes(t *testing.T) {
	longToken := "rt-" + strings.Repeat("a", 30)
	tests := []struct {
		name, in string
		want     []string // "label:kind" with kind auth|refresh
	}{
		{"bare token", longToken, []string{"token:refresh"}},
		{"too short", "rt-short", nil},
		{"token with space", "rt aaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil},
		{"one token per line", longToken + "\n" + longToken + "b\r\nshort", []string{"line 1:refresh", "line 2:refresh"}},
		// A refresh token inside an object is read as a refresh-only session
		// fragment, exactly as the macOS app does.
		{"email + refresh", `{"email":"e@x.com","refresh_token":"` + longToken + `"}`, []string{"json:auth"}},
		{"nested tokens refresh", `{"tokens":{"refreshToken":"` + longToken + `"}}`, []string{"json:auth"}},
		{"api key object", `{"OPENAI_API_KEY":"sk-test-12345678"}`, []string{"json:auth"}},
		{"api key camel", `{"apiKey":"sk-test-12345678"}`, []string{"json:auth"}},
		{"array", `[{"api_key":"sk-test-12345678"},{"refreshToken":"` + longToken + `"}]`, []string{"json[0]:auth", "json[1]:auth"}},
		// The first balanced {...} slice wins when it yields a credential.
		{"ndjson first object wins", `{"api_key":"sk-aaaaaaaa"}` + "\n" + `{"api_key":"sk-bbbbbbbb"}`, []string{"json:auth"}},
		{"ndjson after empty first", `{"x":1}` + "\n" + `{"api_key":"sk-bbbbbbbb"}`, []string{"line 2:auth"}},
		{"unicode line separator", longToken + " " + longToken + "z", []string{"line 1:refresh", "line 2:refresh"}},
		{"session refresh only", `{"session":{"refreshToken":"` + longToken + `"}}`, []string{"json:auth"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, c := range parseCandidates(tc.in) {
				kind := "refresh"
				if c.auth != nil {
					kind = "auth"
				}
				got = append(got, c.label+":"+kind)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestJSONPayloadCandidates(t *testing.T) {
	got := jsonPayloadCandidates(`note {"a":"}{","b":[1]} tail [x]`)
	want := []string{`note {"a":"}{","b":[1]} tail [x]`, `{"a":"}{","b":[1]}`, `[1]`}
	if len(got) != len(want) {
		t.Fatalf("got %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("payload %d = %q, want %q", i, got[i], want[i])
		}
	}
	if one := jsonPayloadCandidates(`{"a":1}`); len(one) != 1 {
		t.Errorf("duplicates not removed: %q", one)
	}
	if none := jsonPayloadCandidates(`{"a":`); len(none) != 1 {
		t.Errorf("unbalanced: %q", none)
	}
}

func TestRefreshOnlyImportUsesRefreshHook(t *testing.T) {
	im, e := newImporter(t)
	var seen string
	im.Refresh = func(_ context.Context, a *Auth) error {
		seen = a.Tokens.RefreshToken
		a.Tokens.IDToken = jwtOf(t, map[string]any{"email": "rt@example.com", "https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "rt-acct"}})
		a.Tokens.AccountID = "rt-acct"
		return a.MergeRefreshResponse([]byte(`{"access_token":"`+jwtExp(t, farFuture)+`","refresh_token":"rotated-import-refresh-token"}`), testNow)
	}
	token := "rt-" + strings.Repeat("z", 30)
	batch := im.ImportPastedText(ctxBG(), token, true)
	if len(batch.Succeeded) != 1 || seen != token {
		t.Fatalf("batch %+v seen=%q", batch, seen)
	}
	acc := batch.Succeeded[0]
	if acc.Email != "rt@example.com" || must(e.store.LoadIndex()).ActiveAccountID != acc.ID ||
		must(e.store.LoadCredential(acc.ID)).Tokens.RefreshToken != "rotated-import-refresh-token" {
		t.Fatalf("account %+v", acc)
	}
}

func TestRefreshOnlyImportFailures(t *testing.T) {
	token := "rt-" + strings.Repeat("z", 30)
	t.Run("no refresher", func(t *testing.T) {
		im, _ := newImporter(t)
		batch := im.ImportPastedText(ctxBG(), token, false)
		if len(batch.Succeeded) != 0 || len(batch.Failures) != 1 || !strings.HasPrefix(batch.Failures[0], "token: ") ||
			strings.Contains(batch.Failures[0], token) {
			t.Fatalf("batch: %+v", batch)
		}
	})
	t.Run("refresher refuses", func(t *testing.T) {
		im, _ := newImporter(t)
		im.Refresh = func(context.Context, *Auth) error { return ErrReauthRequired }
		batch := im.ImportPastedText(ctxBG(), token, false)
		if len(batch.Failures) != 1 || !strings.Contains(batch.Failures[0], ErrReauthRequired.Error()) {
			t.Fatalf("batch: %+v", batch)
		}
	})
}

func TestMaterializeRules(t *testing.T) {
	im, _ := newImporter(t)
	expired := Auth{AuthMode: "chatgpt", Tokens: Tokens{AccessToken: jwtExp(t, 1), RefreshToken: ""}}
	if _, err := im.materialize(ctxBG(), &expired, ""); !errors.Is(err, ErrReauthRequired) {
		t.Errorf("expired without refresh: %v", err)
	}
	empty := Auth{AuthMode: "chatgpt", Tokens: Tokens{}}
	if _, err := im.materialize(ctxBG(), &empty, ""); !errors.Is(err, ErrInvalidCredential) {
		t.Errorf("empty: %v", err)
	}
	if _, err := im.materialize(ctxBG(), nil, ""); !errors.Is(err, ErrInvalidCredential) {
		t.Errorf("nothing: %v", err)
	}
	key := APIKeyAuth("sk-test-12345678")
	if got, err := im.materialize(ctxBG(), &key, ""); err != nil || !got.IsAPIKey() {
		t.Errorf("api key: %v", err)
	}
	valid := Auth{AuthMode: "chatgpt", Tokens: Tokens{AccessToken: jwtExp(t, farFuture), RefreshToken: "valid"}}
	calls := 0
	im.Refresh = func(context.Context, *Auth) error { calls++; return nil }
	if _, err := im.materialize(ctxBG(), &valid, ""); err != nil || calls != 0 {
		t.Errorf("valid token refreshed: err=%v calls=%d", err, calls)
	}
}

func TestEmailHintFillsMissingEmail(t *testing.T) {
	im, e := newImporter(t)
	access := jwtOf(t, map[string]any{"exp": farFuture, "jti": "no-identity-claims"})
	text := `{"user":{"email":"hint@example.com"},"accessToken":"` + access + `"}`
	batch := im.ImportPastedText(ctxBG(), text, false)
	if len(batch.Succeeded) != 1 {
		t.Fatalf("batch: %+v", batch)
	}
	row, _ := must(e.store.LoadIndex()).Account(batch.Succeeded[0].ID)
	if row.Email != "hint@example.com" || row.DisplayName != "hint@example.com" {
		t.Fatalf("row: %+v", row)
	}
}

func TestImportFiles(t *testing.T) {
	im, _ := newImporter(t)
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	noErr(t, os.WriteFile(good, []byte(pasteAuthJSON(t, "refresh-file-token-long-enough")), 0o600))
	empty := filepath.Join(dir, "empty.json")
	noErr(t, os.WriteFile(empty, []byte(`{}`), 0o600))
	binary := filepath.Join(dir, "bin.dat")
	noErr(t, os.WriteFile(binary, []byte{0xff, 0xfe, 0x00}, 0o600))
	missing := filepath.Join(dir, "missing.json")

	batch := im.ImportFiles(ctxBG(), []string{good, empty, binary, missing}, true)
	if len(batch.Succeeded) != 1 || len(batch.Failures) != 3 {
		t.Fatalf("batch: %+v", batch)
	}
	if !strings.HasPrefix(batch.Failures[0], "empty.json: ") || !strings.HasPrefix(batch.Failures[1], "bin.dat: ") || !strings.HasPrefix(batch.Failures[2], "missing.json: ") {
		t.Fatalf("failures: %q", batch.Failures)
	}
	if none := im.ImportFiles(ctxBG(), nil, false); len(none.Failures) != 1 || none.Failures[0] != FailureNoCredentials {
		t.Fatalf("no files: %+v", none)
	}
}

func TestPreviewFilesRawAndDedupe(t *testing.T) {
	im, e := newImporter(t)
	existing := sampleAuth(t, sampleOpt{accountID: "paste-1", email: "paste@example.com", refresh: "refresh-existing-account"})
	_ = existing
	dir := t.TempDir()
	text := pasteAuthJSON(t, "refresh-file-token-long-enough")
	a, b := filepath.Join(dir, "a.json"), filepath.Join(dir, "b.json")
	noErr(t, os.WriteFile(a, []byte(text), 0o600))
	noErr(t, os.WriteFile(b, []byte(text), 0o600))
	token := "rt-" + strings.Repeat("q", 30)
	c := filepath.Join(dir, "c.txt")
	noErr(t, os.WriteFile(c, []byte(token), 0o600))

	preview := im.PreviewFiles([]string{a, b, c})
	if len(preview.Items) != 2 || preview.SourceIsPack || preview.RoutedProvider != ProviderCodex {
		t.Fatalf("preview: %+v", preview)
	}
	if preview.Items[1].RefreshToken != token || preview.Items[1].Preview.Conflict.WillUpdate {
		t.Errorf("refresh-only item: %+v", preview.Items[1].Preview)
	}
	if preview.Items[0].Preview.DisplayName != "paste@example.com" {
		t.Errorf("display name: %+v", preview.Items[0].Preview)
	}
	// After committing, the same input is flagged as an update.
	ids := []string{preview.Items[0].ID()}
	if batch := im.ImportPreviewSelection(ctxBG(), preview.Items, ids, false); len(batch.Succeeded) != 1 {
		t.Fatalf("batch: %+v", batch)
	}
	again := im.PreviewFiles([]string{a})
	if !again.Items[0].Preview.Conflict.WillUpdate || len(must(e.store.LoadIndex()).Accounts) != 1 {
		t.Fatalf("conflict not flagged: %+v", again.Items[0].Preview)
	}
}

func TestPreviewIDsAreStable(t *testing.T) {
	im, _ := newImporter(t)
	path := filepath.Join(t.TempDir(), "swift.json")
	noErr(t, os.WriteFile(path, fixture(t, "swift_pack.json"), 0o600))
	first, second := im.PreviewFiles([]string{path}), im.PreviewFiles([]string{path})
	for i := range first.Items {
		id := first.Items[i].ID()
		if id != second.Items[i].ID() || !strings.HasPrefix(id, "codex-") {
			t.Errorf("unstable or malformed id %q", id)
		}
	}
	// The id is the hash of "<label>|<matchKey>".
	var rows []map[string]string
	noErr(t, json.Unmarshal(fixture(t, "swift_ids.json"), &rows))
	want := "codex-" + StableHash("swift.json[0]|oauth:user:user-Aa1|account:acct-a")
	if first.Items[0].ID() != want {
		t.Errorf("first id = %s, want %s", first.Items[0].ID(), want)
	}
}

func TestImportOfficialReadsOnly(t *testing.T) {
	im, e := newImporter(t)
	e.writeOfficial(t, sampleAuth(t, sampleOpt{accountID: "off", email: "off@example.com", refresh: "refresh-official-import"}))
	before := e.officialBytes(t)
	acc := must(im.ImportOfficial(true))
	if acc.AccountID != "off" || string(e.officialBytes(t)) != string(before) {
		t.Fatal("import changed the official file or failed")
	}
}
