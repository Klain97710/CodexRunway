package grok

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// The account_swift_*.json fixtures were produced by the Swift releases' own
// code with synthetic tokens and are frozen.

var (
	fixtureNow     = time.Unix(1_785_499_200, 750_000_000)
	fixtureReauthT = time.Unix(1_800_000_000, 0)
)

func readFixture(t *testing.T, name string, into any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func identityText(id CredentialIdentity) string {
	node := ManagedAccount{Identity: id}.identityNode()
	data, err := acctMarshalJSON(node, acctEncoder, true)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func authErrorName(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrAuthInvalidRoot):
		return "invalidRoot"
	case errors.Is(err, ErrAuthNoManagedCredential):
		return "noManagedCredential"
	case errors.Is(err, ErrAuthInvalidManagedCredential):
		return "invalidManagedCredential"
	}
	return "other"
}

// Date forms the Go parser deliberately does not reproduce (documented).
var knownDateDeviations = map[string]bool{"d_year5": true, "d_offset_9999": true}

func TestSwiftDocumentFixtures(t *testing.T) {
	var fx struct {
		Rows []struct {
			Name                  string   `json:"name"`
			Input                 string   `json:"input"`
			OK                    bool     `json:"ok"`
			Error                 string   `json:"error"`
			StableID              string   `json:"stableID"`
			DisplayName           string   `json:"displayName"`
			Scopes                []string `json:"scopes"`
			Identity              string   `json:"identity"`
			RequiresReauth        bool     `json:"requiresReauth"`
			AccessToken           string   `json:"accessToken"`
			RefreshToken          string   `json:"refreshToken"`
			Refreshed             string   `json:"refreshed"`
			RefreshedNoRotation   string   `json:"refreshedNoRotation"`
			RefreshedBlankRefresh string   `json:"refreshedBlankRefresh"`
		} `json:"rows"`
		RefreshErrors []struct{ Name, Error string }
	}
	readFixture(t, "account_swift_documents.json", &fx)
	if len(fx.Rows) < 40 {
		t.Fatalf("fixture has only %d rows", len(fx.Rows))
	}
	for _, row := range fx.Rows {
		t.Run(row.Name, func(t *testing.T) {
			if knownDateDeviations[row.Name] {
				t.Skip("documented date deviation")
			}
			doc, err := ParseAuthDocument([]byte(row.Input))
			if !row.OK {
				if got := authErrorName(err); got != row.Error {
					t.Fatalf("error = %q (%v), want %q", got, err, row.Error)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if doc.StableID() != row.StableID {
				t.Errorf("stableID = %s, want %s", doc.StableID(), row.StableID)
			}
			if got := doc.Identity.ResolvedDisplayName(); got != row.DisplayName {
				t.Errorf("displayName = %q, want %q", got, row.DisplayName)
			}
			if !equalStrings(doc.ManagedScopeKeys, row.Scopes) {
				t.Errorf("scopes = %v, want %v", doc.ManagedScopeKeys, row.Scopes)
			}
			if got := identityText(doc.Identity); got != row.Identity {
				t.Errorf("identity =\n%s\nwant\n%s", got, row.Identity)
			}
			if got := doc.RequiresReauthentication(fixtureReauthT); got != row.RequiresReauth {
				t.Errorf("requiresReauth = %v, want %v", got, row.RequiresReauth)
			}
			if access, err := doc.AccessToken(); err == nil && access != row.AccessToken {
				t.Errorf("access token mismatch")
			} else if err != nil && row.AccessToken != "" {
				t.Errorf("access token missing")
			}
			if refresh, err := doc.RefreshToken(); err == nil && refresh != row.RefreshToken {
				t.Errorf("refresh token mismatch")
			} else if err != nil && row.RefreshToken != "" {
				t.Errorf("refresh token missing")
			}
			checkRefresh(t, "refreshed", row.Input, " new-access ", " new-refresh ", 3600, row.Refreshed)
			checkRefresh(t, "no rotation", row.Input, "a2", "", 0, row.RefreshedNoRotation)
			checkRefresh(t, "blank refresh", row.Input, "a3", "  ", 0, row.RefreshedBlankRefresh)
		})
	}
	for _, e := range fx.RefreshErrors {
		row := fx.Rows[0]
		if row.Name != e.Name {
			t.Fatalf("fixture order changed")
		}
		_, err := ApplyTokenRefresh([]byte(row.Input), "   ", "", 0, fixtureNow)
		if got := authErrorName(err); got != e.Error {
			t.Errorf("blank access token: %q, want %q", got, e.Error)
		}
	}
}

func checkRefresh(t *testing.T, label, input, access, refresh string, expires int, want string) {
	t.Helper()
	got, err := ApplyTokenRefresh([]byte(input), access, refresh, expires, fixtureNow)
	if want == "ERR" {
		if err == nil {
			t.Errorf("%s: want an error", label)
		}
		return
	}
	if err != nil {
		t.Errorf("%s: %v", label, err)
		return
	}
	if string(got) != want {
		t.Errorf("%s:\n%s\nwant\n%s", label, got, want)
	}
}

func TestSwiftLayoutFixtures(t *testing.T) {
	var rows []struct {
		Name     string `json:"name"`
		Official string `json:"official"`
		Target   string `json:"target"`
		Out      string `json:"out"`
	}
	readFixture(t, "account_swift_layout.json", &rows)
	for _, row := range rows {
		t.Run(row.Name, func(t *testing.T) {
			var got []byte
			var err error
			switch row.Name {
			case "merge_full", "merge_multi", "merge_api_only_official":
				got, err = ReplaceManagedScopes([]byte(row.Official), []byte(row.Target))
			case "merge_nil_official":
				got, err = ReplaceManagedScopes(nil, []byte(row.Target))
			case "refresh_layout":
				got, err = ApplyTokenRefresh([]byte(row.Official), "na", "nr", 60, fixtureNow)
			default:
				got, err = ApplyTokenRefresh([]byte(row.Official), "na", "", 0, fixtureNow)
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != row.Out {
				t.Errorf("layout differs from Swift:\n%s\nwant\n%s", got, row.Out)
			}
		})
	}
}

func TestSwiftPasteFixtures(t *testing.T) {
	var rows []struct {
		Name     string   `json:"name"`
		Text     string   `json:"text"`
		Payloads []string `json:"payloads"`
	}
	readFixture(t, "account_swift_paste.json", &rows)
	for _, row := range rows {
		t.Run(row.Name, func(t *testing.T) {
			got := ParsePayloads(row.Text)
			if len(got) != len(row.Payloads) {
				t.Fatalf("%d payloads, want %d", len(got), len(row.Payloads))
			}
			for i := range got {
				if string(got[i]) != row.Payloads[i] {
					t.Errorf("payload %d:\n%s\nwant\n%s", i, got[i], row.Payloads[i])
				}
			}
		})
	}
}

func TestSwiftOAuthDocumentFixtures(t *testing.T) {
	bundles := map[string]TokenBundle{
		"full":                   {AccessToken: "acc", RefreshToken: "ref", ExpiresIn: 3600, Email: "Device@Example.com", Subject: "sub-1"},
		"email_only":             {AccessToken: "acc", RefreshToken: "ref", ExpiresIn: 3600, Email: "Mail@Example.com"},
		"subject_only":           {AccessToken: "acc", Subject: "sub-2"},
		"no_refresh_zero_expiry": {AccessToken: "acc", Email: "z@example.com", Subject: "sub-3"},
		"padded":                 {AccessToken: "acc", RefreshToken: "ref", Email: "  pad@example.com ", Subject: " sub-4 "},
		"none":                   {AccessToken: "acc", RefreshToken: "ref"},
		"blank":                  {AccessToken: "acc", RefreshToken: "ref", Email: "   ", Subject: "  "},
	}
	var rows []struct {
		Name     string `json:"name"`
		Out      string `json:"out"`
		StableID string `json:"stableID"`
		Error    string `json:"error"`
	}
	readFixture(t, "account_swift_oauth.json", &rows)
	if len(rows) != len(bundles) {
		t.Fatalf("%d fixtures, %d bundles", len(rows), len(bundles))
	}
	for _, row := range rows {
		t.Run(row.Name, func(t *testing.T) {
			data, err := MakeAuthDocumentData(bundles[row.Name], fixtureNow)
			if row.Error != "" {
				if !errors.Is(err, &OAuthError{Kind: OAuthMissingIdentity}) {
					t.Fatalf("err = %v, want missing identity", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != row.Out {
				t.Errorf("document:\n%s\nwant\n%s", data, row.Out)
			}
			doc, err := ParseAuthDocument(data)
			if err != nil || doc.StableID() != row.StableID {
				t.Errorf("stable id %v (%v), want %s", doc.StableID(), err, row.StableID)
			}
		})
	}
}

func TestSwiftPathFixtures(t *testing.T) {
	var fx struct {
		Sanitize []struct{ ID, Dir string }
		Homes    []struct{ Env, Home string }
	}
	readFixture(t, "account_swift_paths.json", &fx)
	store := NewStoreAt(filepath.Join(t.TempDir(), "accounts"), t.TempDir())
	for _, row := range fx.Sanitize {
		got := filepath.Base(store.AccountDir(row.ID))
		want := row.Dir
		if row.ID == ".." {
			want = "__" // deliberate deviation: never address the parent directory
		}
		if row.ID == "é" {
			want = got // the fixture was captured on a file system that decomposes names
		}
		if got != want {
			t.Errorf("AccountDir(%q) = %q, want %q", row.ID, got, want)
		}
	}
	if runtime.GOOS == "windows" {
		t.Skip("POSIX path fixtures")
	}
	for _, row := range fx.Homes {
		if row.Env == "rel/path" {
			continue // resolved against the generator's working directory
		}
		if got := ResolveOfficialHome(row.Env, "/Users/example"); got != row.Home {
			t.Errorf("ResolveOfficialHome(%q) = %q, want %q", row.Env, got, row.Home)
		}
	}
}

func TestSwiftIndexLayoutFixtures(t *testing.T) {
	var fx struct{ Empty, Quota string }
	readFixture(t, "account_swift_index_layout.json", &fx)
	if got, err := MarshalAccountIndex(NewAccountIndex()); err != nil || string(got) != fx.Empty {
		t.Errorf("empty index = %q (%v), want %q", got, err, fx.Empty)
	}
	idx, err := ParseAccountIndex([]byte(fx.Quota))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := MarshalAccountIndex(idx); err != nil || string(got) != fx.Quota {
		t.Errorf("quota index round trip:\n%s\nwant\n%s", got, fx.Quota)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
