package codex

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The fixtures in testdata/swift_*.json were produced by the Swift releases'
// core and are frozen, so these tests compare Go against what the previous
// macOS app really wrote and read.

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	return must(os.ReadFile(filepath.Join("testdata", name)))
}

func TestSwiftStableHash(t *testing.T) {
	var rows []map[string]string
	noErr(t, json.Unmarshal(fixture(t, "swift_ids.json"), &rows))
	checked := 0
	for _, r := range rows {
		if r["name"] != "hash" {
			continue
		}
		checked++
		if got := StableHash(r["input"]); got != r["hash"] {
			t.Errorf("StableHash(%q) = %s, Swift %s", r["input"], got, r["hash"])
		}
	}
	if checked == 0 {
		t.Fatal("no hash fixtures")
	}
}

func TestSwiftIdentity(t *testing.T) {
	var rows []map[string]string
	noErr(t, json.Unmarshal(fixture(t, "swift_ids.json"), &rows))
	for _, r := range rows {
		if r["name"] == "hash" {
			continue
		}
		t.Run(r["name"], func(t *testing.T) {
			auth := Auth{
				AuthMode: r["authMode"],
				Tokens: Tokens{
					IDToken: r["idToken"], AccessToken: r["accessToken"],
					RefreshToken: r["refreshToken"], AccountID: r["tokenAccountId"],
				},
			}
			if r["apiKey"] != "" {
				key := r["apiKey"]
				auth = Auth{AuthMode: r["authMode"], OpenAIAPIKey: &key}
			}
			if got := MatchKey(auth); got != r["matchKey"] {
				t.Errorf("MatchKey = %s, Swift %s", got, r["matchKey"])
			}
			if got := ManagedAccountID(auth); got != r["id"] {
				t.Errorf("ManagedAccountID = %s, Swift %s", got, r["id"])
			}
			acc := NewManagedAccount("", auth, 0, "", "", "", testNow)
			checks := map[string][2]string{
				"displayName":   {acc.DisplayName, r["displayName"]},
				"accountId":     {acc.AccountID, r["accountId"]},
				"email":         {acc.Email, r["email"]},
				"planType":      {acc.PlanType, r["planType"]},
				"userId":        {acc.UserID, r["userId"]},
				"subjectId":     {acc.SubjectID, r["subjectId"]},
				"workspaceName": {acc.WorkspaceName, r["workspaceName"]},
			}
			for field, pair := range checks {
				if pair[0] != pair[1] {
					t.Errorf("%s = %q, Swift %q", field, pair[0], pair[1])
				}
			}
		})
	}
}

func TestSwiftAccountIDs(t *testing.T) {
	var ids []string
	noErr(t, json.Unmarshal(fixture(t, "swift_account_ids.json"), &ids))
	idx := must(ParseIndex(fixture(t, "swift_index.json")))
	if len(idx.Accounts) != len(ids) {
		t.Fatalf("index has %d accounts, want %d", len(idx.Accounts), len(ids))
	}
	for i, acc := range idx.Accounts {
		if acc.ID != ids[i] {
			t.Errorf("account %d id %s, want %s", i, acc.ID, ids[i])
		}
	}
}

func TestSwiftIndexRoundTripsByteForByte(t *testing.T) {
	raw := fixture(t, "swift_index.json")
	idx := must(ParseIndex(raw))
	got := must(MarshalIndex(idx))
	if !bytes.Equal(got, bytes.TrimRight(raw, "\n")) {
		t.Fatalf("Go re-encoding differs from Swift output\n--- go ---\n%s\n--- swift ---\n%s", got, raw)
	}
	if idx.ActiveAccountID != "oauth-4428a9edcf192bba" || idx.Version != 1 {
		t.Fatalf("unexpected header: active=%q version=%d", idx.ActiveAccountID, idx.Version)
	}
	a := idx.Accounts[0]
	if a.Alias != "Alpha / one" || a.Note != "néte \"quoted\"" || a.SortIndex != 0 {
		t.Fatalf("account 0 decoded wrongly: %+v", a)
	}
	b := idx.Accounts[1]
	if b.CachedQuota == nil || !b.RequiresReauth || b.LastError != "boom" || b.WorkspaceName != "WS 日本" {
		t.Fatalf("account 1 decoded wrongly: %+v", b)
	}
	if len(b.CachedQuota.AdditionalMeters) != 1 || b.CachedQuota.AdditionalMeters[0].ID != "additional-0-Spark" {
		t.Fatalf("additional meters: %+v", b.CachedQuota.AdditionalMeters)
	}
}

func TestSwiftCredentialRoundTripsByteForByte(t *testing.T) {
	for _, name := range []string{"swift_cred_oauth.json", "swift_cred_apikey.json", "swift_official_auth.json"} {
		t.Run(name, func(t *testing.T) {
			raw := fixture(t, name)
			auth := must(DecodeAuth(raw))
			got := must(auth.MarshalFile(nil))
			if !bytes.Equal(got, bytes.TrimRight(raw, "\n")) {
				t.Fatalf("Go output differs from Swift\n--- go ---\n%s\n--- swift ---\n%s", got, raw)
			}
		})
	}
}

func TestSwiftDates(t *testing.T) {
	var rows []struct {
		TS   float64 `json:"ts"`
		Text string  `json:"text"`
	}
	noErr(t, json.Unmarshal(fixture(t, "swift_dates.json"), &rows))
	for _, r := range rows {
		sec := int64(r.TS)
		if r.TS < 0 && float64(sec) != r.TS {
			sec--
		}
		frac := r.TS - float64(sec)
		tm := time.Unix(sec, int64(frac*1e9))
		if got := FormatTime(tm); got != r.Text {
			t.Errorf("FormatTime(%v) = %s, Swift %s", r.TS, got, r.Text)
		}
		back, ok := ParseTime(r.Text)
		if !ok || FormatTime(back) != r.Text {
			t.Errorf("ParseTime(%s) did not round trip: %v %v", r.Text, back, ok)
		}
	}
}

func TestSwiftMask(t *testing.T) {
	var rows []struct{ In, Mask, Emails string }
	noErr(t, json.Unmarshal(fixture(t, "swift_mask.json"), &rows))
	for _, r := range rows {
		t.Run(r.In, func(t *testing.T) {
			if got := MaskAccountText(r.In); got != r.Mask {
				t.Errorf("MaskAccountText(%q) = %q, Swift %q", r.In, got, r.Mask)
			}
			if got := MaskEmailsIn(r.In); got != r.Emails {
				t.Errorf("MaskEmailsIn(%q) = %q, Swift %q", r.In, got, r.Emails)
			}
		})
	}
}

func TestSwiftPackDecodes(t *testing.T) {
	pack := must(DecodeTransferPack(fixture(t, "swift_pack.json")))
	if pack.Provider != ProviderCodex || pack.Version != 1 || pack.AppVersion != "0.0.87" {
		t.Fatalf("header: %+v", pack)
	}
	if FormatTime(pack.ExportedAt) != "2023-11-14T22:13:20.500Z" {
		t.Fatalf("exportedAt = %s", FormatTime(pack.ExportedAt))
	}
	if len(pack.Accounts) != 3 || pack.Accounts[0].Alias != "Alpha / one" || pack.Accounts[2].DisplayName != "API ···38b4" {
		t.Fatalf("accounts: %+v", pack.Accounts)
	}
	for _, e := range pack.Accounts {
		if _, err := DecodeAuth(e.Credential); err != nil {
			t.Errorf("credential of %s does not decode: %v", e.ID, err)
		}
	}
}

func TestGoPackMatchesSwiftEncoding(t *testing.T) {
	for _, name := range []string{"swift_pack.json", "swift_pack_grok.json"} {
		t.Run(name, func(t *testing.T) {
			raw := fixture(t, name)
			pack := must(DecodeTransferPack(raw))
			got := must(EncodeTransferPack(pack))
			// This edition exports the original Swift brand. All remaining
			// bytes still match the frozen upstream fixture exactly.
			raw = bytes.Replace(raw, []byte(`"format" : "quosparr-accounts"`), []byte(`"format" : "codex-runway-accounts"`), 1)
			if !bytes.Equal(got, bytes.TrimRight(raw, "\n")) {
				t.Fatalf("Go encoding differs from Swift\n--- go ---\n%s\n--- swift ---\n%s", got, raw)
			}
		})
	}
}
