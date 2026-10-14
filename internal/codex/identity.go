package codex

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
)

// StableHash is the short hex hash behind account ids and de-duplication:
// h = 127*h + byte over the UTF-8 bytes, starting at 5381, in uint64. It is
// not cryptographic and must stay bit-identical to the Swift app, because
// account ids and directory names derive from it.
func StableHash(s string) string {
	h := uint64(5381)
	for i := 0; i < len(s); i++ {
		h = 127*h + uint64(s[i])
	}
	return strconv.FormatUint(h, 16)
}

// APIKeyFingerprint hashes a trimmed API key.
func APIKeyFingerprint(key string) string { return StableHash(strings.TrimSpace(key)) }

// oauthFingerprint identifies a credential by its refresh token, then its
// access token. Without either it is random, so it never matches anything.
func oauthFingerprint(a Auth) string {
	switch {
	case a.Tokens.RefreshToken != "":
		return "rt-" + StableHash(a.Tokens.RefreshToken)
	case a.Tokens.AccessToken != "":
		return "at-" + StableHash(a.Tokens.AccessToken)
	}
	var b [16]byte
	_, _ = rand.Read(b[:])
	return "unknown-" + StableHash(hex.EncodeToString(b[:]))
}

// tokenClaims decodes the id and access token claims once.
func tokenClaims(a Auth) (id, access *IdentityClaims) {
	return DecodeIdentityClaims(a.Tokens.IDToken), DecodeIdentityClaims(a.Tokens.AccessToken)
}

// uniqueIdentifier returns the common non-blank value, or "" if the values
// disagree or are all blank.
func uniqueIdentifier(values ...string) string {
	found := ""
	for _, v := range values {
		v = firstNonEmpty(v)
		if v == "" {
			continue
		}
		if found != "" && found != v {
			return ""
		}
		found = v
	}
	return found
}

func oauthUserID(a Auth) string {
	id, acc := tokenClaims(a)
	return uniqueIdentifier(id.userID(), acc.userID())
}

func oauthSubjectID(a Auth) string {
	id, acc := tokenClaims(a)
	return uniqueIdentifier(id.subject(), acc.subject())
}

// oauthAccountID is the workspace of a credential: the selected tokens.account_id
// wins, then explicit JWT account claims, then the organization fallback.
func oauthAccountID(a Auth) string {
	if v := firstNonEmpty(a.Tokens.AccountID); v != "" {
		return v
	}
	id, acc := tokenClaims(a)
	if firstNonEmpty(id.accountID(), acc.accountID()) != "" {
		return uniqueIdentifier(id.accountID(), acc.accountID())
	}
	return uniqueIdentifier(id.orgID(), acc.orgID())
}

// oauthEmail is the lowercase email when the tokens agree on one.
func oauthEmail(a Auth) string {
	id, acc := tokenClaims(a)
	e1, e2 := strings.ToLower(firstNonEmpty(id.email())), strings.ToLower(firstNonEmpty(acc.email()))
	if e1 != "" && e2 != "" && e1 != e2 {
		return ""
	}
	return firstNonEmpty(e1, e2)
}

// oauthWorkspaceName is the organization title, only when no explicit
// workspace id exists (an explicit id makes the title just a guess).
func oauthWorkspaceName(a Auth) string {
	id, acc := tokenClaims(a)
	if firstNonEmpty(a.Tokens.AccountID, id.accountID(), acc.accountID()) != "" {
		return ""
	}
	return firstNonEmpty(id.orgName(), acc.orgName())
}

// oauthUserConflict reports tokens that name different users.
func oauthUserConflict(a Auth) bool {
	id, acc := tokenClaims(a)
	if n := distinct(id.userID(), acc.userID()); n > 0 {
		return n > 1
	}
	return distinct(id.subject(), acc.subject()) > 1
}

// distinct counts the different non-blank values.
func distinct(values ...string) int {
	seen := map[string]struct{}{}
	for _, v := range values {
		if v = firstNonEmpty(v); v != "" {
			seen[v] = struct{}{}
		}
	}
	return len(seen)
}

// ManagedAccountID derives the stable id of a credential: "api-<fingerprint>"
// for API keys, else "oauth-" plus the hash of its match key.
func ManagedAccountID(a Auth) string {
	if a.IsAPIKey() && a.OpenAIAPIKey != nil {
		return "api-" + APIKeyFingerprint(*a.OpenAIAPIKey)
	}
	return "oauth-" + StableHash(MatchKey(a))
}

// MatchKey is the identity used to decide whether two credentials are the
// same account. It never falls back to the workspace id alone.
func MatchKey(a Auth) string {
	if a.IsAPIKey() && a.OpenAIAPIKey != nil {
		return "id:api-" + APIKeyFingerprint(*a.OpenAIAPIKey)
	}
	if oauthUserConflict(a) {
		return "oauth:credential:" + oauthFingerprint(a)
	}
	user, subject := firstNonEmpty(oauthUserID(a)), firstNonEmpty(oauthSubjectID(a))
	workspace, email := firstNonEmpty(oauthAccountID(a)), oauthEmail(a)
	switch {
	case user != "" && workspace != "":
		return "oauth:user:" + user + "|account:" + workspace
	case user == "" && subject != "" && workspace != "":
		return "oauth:subject:" + subject + "|account:" + workspace
	case user == "" && subject == "" && email != "" && workspace != "":
		return "oauth:email:" + email + "|account:" + workspace
	}
	return "oauth:credential:" + oauthFingerprint(a)
}

// AccountMatchKey is MatchKey computed from a stored row.
func AccountMatchKey(acc ManagedAccount) string {
	if acc.AuthMode == AuthModeAPIKey && firstNonEmpty(acc.AccountID) != "" {
		return "id:" + strings.ToLower(firstNonEmpty(acc.AccountID))
	}
	user, workspace := firstNonEmpty(acc.UserID), firstNonEmpty(acc.AccountID)
	subject, email := firstNonEmpty(acc.SubjectID), strings.ToLower(firstNonEmpty(acc.Email))
	switch {
	case user != "" && workspace != "":
		return "oauth:user:" + user + "|account:" + workspace
	case acc.UserID == "" && subject != "" && workspace != "":
		return "oauth:subject:" + subject + "|account:" + workspace
	case acc.UserID == "" && acc.SubjectID == "" && email != "" && workspace != "":
		return "oauth:email:" + email + "|account:" + workspace
	}
	return "row:" + acc.ID
}

// AccountMatches reports whether stored account acc and credential a are the
// same identity. It lets a legacy row without a user id be upgraded by the
// same email and workspace, and never merges on the workspace id alone when
// either side has a user-level identity.
func AccountMatches(acc ManagedAccount, a Auth) bool {
	if acc.AuthMode == AuthModeAPIKey || a.IsAPIKey() {
		return AccountMatchKey(acc) == MatchKey(a)
	}
	if oauthUserConflict(a) {
		return acc.ID == ManagedAccountID(a)
	}
	accUser, authUser := firstNonEmpty(acc.UserID), firstNonEmpty(oauthUserID(a))
	accSubject, authSubject := firstNonEmpty(acc.SubjectID), firstNonEmpty(oauthSubjectID(a))
	accWS, authWS := firstNonEmpty(acc.AccountID), firstNonEmpty(oauthAccountID(a))
	accEmail, authEmail := strings.ToLower(firstNonEmpty(acc.Email)), oauthEmail(a)

	switch {
	case accUser != "" && authUser != "" && accWS != "" && authWS != "":
		return accUser == authUser && accWS == authWS
	case accSubject != "" && authSubject != "" && accWS != "" && authWS != "":
		return accSubject == authSubject && accWS == authWS
	case accUser == "" && accSubject == "" && accWS != "" && authWS != "" && accEmail != "" && authEmail != "":
		return accWS == authWS && accEmail == authEmail
	}
	return acc.ID == ManagedAccountID(a)
}
