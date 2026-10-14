package grok

import (
	"strings"
	"unicode"
)

// ParsePayloads turns pasted Grok credential text into normalized auth.json
// payloads that ParseAuthDocument accepts.
//
// Accepted shapes (Grok-specific, not ChatGPT session JSON):
//   - a full ~/.grok/auth.json with managed OAuth or legacy scopes;
//   - a single credential object (key / refresh_token plus identity fields);
//   - a JSON array of either form;
//   - newline-delimited JSON objects.
//
// Like the Swift app, the whole text is tried first, then the first balanced
// {...} slice, then the first balanced [...] slice; the first of those that
// yields any credential wins. Newline-delimited input therefore returns only
// the first object when that object is complete on its own line.
func ParsePayloads(text string) [][]byte {
	trimmed := strings.TrimFunc(text, unicode.IsSpace)
	if trimmed == "" {
		return nil
	}
	for _, candidate := range jsonPayloadCandidates(trimmed) {
		if value, ok := acctJSONContainer([]byte(candidate)); ok {
			if parsed := payloadsFrom(value); len(parsed) > 0 {
				return uniquePayloads(parsed)
			}
		}
	}
	lines := splitLines(trimmed)
	if len(lines) > 1 {
		var collected [][]byte
		for _, line := range lines {
			if value, ok := acctJSONContainer([]byte(line)); ok {
				collected = append(collected, payloadsFrom(value)...)
			}
		}
		if len(collected) > 0 {
			return uniquePayloads(collected)
		}
	}
	return nil
}

func splitLines(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case '\n', '\r', '\v', '\f', '\u0085', ' ', ' ':
			return true
		}
		return false
	})
	var lines []string
	for _, p := range parts {
		if t := strings.TrimFunc(p, unicode.IsSpace); t != "" {
			lines = append(lines, t)
		}
	}
	return lines
}

func payloadsFrom(value any) [][]byte {
	switch v := value.(type) {
	case []any:
		var out [][]byte
		for _, item := range v {
			out = append(out, payloadsFrom(item)...)
		}
		return out
	case map[string]any:
		if importIsAuthRoot(v) {
			data := authCredentialJSON(v)
			if _, err := ParseAuthDocument(data); err == nil {
				return [][]byte{data}
			}
			return nil
		}
		if importIsCredentialBody(v) {
			if data := wrapCredentialBody(v); data != nil {
				return [][]byte{data}
			}
		}
	}
	return nil
}

func importIsAuthRoot(o map[string]any) bool {
	for k := range o {
		if isManagedScope(k) {
			return true
		}
	}
	return false
}

func importIsCredentialBody(o map[string]any) bool {
	hasToken := false
	for _, key := range []string{"key", "refresh_token", "access_token", "accessToken"} {
		if authString(o[key]) != "" {
			hasToken = true
		}
	}
	if !hasToken {
		return false
	}
	return acctFirstNonEmpty(authString(o["email"]), authString(o["user_id"]), authString(o["principal_id"])) != ""
}

// wrapCredentialBody normalizes a bare credential object into the official
// auth.json shape: alternate token names become key / refresh_token, and the
// object is filed under its scope with auth_mode and OIDC fields filled in.
func wrapCredentialBody(object map[string]any) []byte {
	body := make(map[string]any, len(object)+4)
	for k, v := range object {
		body[k] = v
	}
	if authString(body["key"]) == "" {
		if access := acctFirstNonEmpty(authString(body["access_token"]), authString(body["accessToken"])); access != "" {
			body["key"] = access
		}
	}
	if authString(body["refresh_token"]) == "" {
		if refresh := authString(body["refreshToken"]); refresh != "" {
			body["refresh_token"] = refresh
		}
	}
	scope := scopeForCredentialBody(body)
	if _, ok := body["auth_mode"]; !ok {
		if scope == LegacyScope {
			body["auth_mode"] = "grok"
		} else {
			body["auth_mode"] = "oidc"
		}
	}
	if scope != LegacyScope {
		if _, ok := body["oidc_issuer"]; !ok {
			body["oidc_issuer"] = defaultIssuer
		}
		if _, ok := body["oidc_client_id"]; !ok {
			if client := strings.TrimPrefix(scope, OAuthScopePrefix); client != "" {
				body["oidc_client_id"] = client
			}
		}
	}
	data := authCredentialJSON(map[string]any{scope: body})
	if _, err := ParseAuthDocument(data); err != nil {
		return nil
	}
	return data
}

func scopeForCredentialBody(o map[string]any) string {
	authMode := strings.ToLower(authString(o["auth_mode"]))
	if authMode == "grok" || authMode == "web_login" {
		return LegacyScope
	}
	if client := authString(o["oidc_client_id"]); client != "" {
		return OAuthScopePrefix + client
	}
	if authMode == "oidc" || authString(o["oidc_issuer"]) != "" || authString(o["refresh_token"]) != "" ||
		authString(o["refreshToken"]) != "" || authString(o["principal_id"]) != "" {
		return OAuthScopePrefix + "desktop-client"
	}
	return LegacyScope
}

// uniquePayloads keeps the first payload of each identity.
func uniquePayloads(payloads [][]byte) [][]byte {
	seen := map[string]bool{}
	var out [][]byte
	for _, data := range payloads {
		doc, err := ParseAuthDocument(data)
		if err != nil {
			continue
		}
		if id := doc.StableID(); !seen[id] {
			seen[id] = true
			out = append(out, data)
		}
	}
	return out
}

// jsonPayloadCandidates is the whole text plus the first balanced {...} and
// [...] slices, for pastes with prose around the JSON.
func jsonPayloadCandidates(text string) []string {
	candidates := []string{text}
	for _, pair := range [][2]byte{{'{', '}'}, {'[', ']'}} {
		if slice, ok := firstBalancedSlice(text, pair[0], pair[1]); ok {
			dup := false
			for _, c := range candidates {
				dup = dup || c == slice
			}
			if !dup {
				candidates = append(candidates, slice)
			}
		}
	}
	return candidates
}

func firstBalancedSlice(text string, open, closing byte) (string, bool) {
	start := strings.IndexByte(text, open)
	if start < 0 {
		return "", false
	}
	depth, inString, escaped := 0, false, false
	for i := start; i < len(text); i++ {
		ch := text[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case open:
			depth++
		case closing:
			depth--
			if depth == 0 {
				return text[start : i+1], true
			}
		}
	}
	return "", false
}
