package codexapi

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// IdentityClaims are the account facts a Codex ID or access token carries
// (CodexIdentityClaims in the Swift app). The token is only decoded, never
// verified: the claims are display and matching hints.
type IdentityClaims struct {
	Email            string
	Username         string
	Subject          string
	UserID           string
	PlanType         string
	AccountID        string
	OrganizationID   string
	OrganizationName string
	// SubscriptionActiveUntil is the billing period end claim, zero if absent.
	SubscriptionActiveUntil time.Time
}

// DecodeClaims decodes the payload of a JWT. ok is false when jwt is not a
// decodable token.
func DecodeClaims(jwt string) (IdentityClaims, bool) {
	object, ok := jwtPayload(jwt)
	if !ok {
		return IdentityClaims{}, false
	}
	profile, _ := object["https://api.openai.com/profile"].(map[string]any)
	auth, _ := object["https://api.openai.com/auth"].(map[string]any)
	org := selectedOrganization(auth)
	return IdentityClaims{
		Email:    firstNonEmpty(str(object["email"]), str(profile["email"])),
		Username: firstNonEmpty(str(object["preferred_username"]), str(object["name"])),
		Subject:  firstNonEmpty(str(object["sub"])),
		UserID: firstNonEmpty(
			str(auth["chatgpt_user_id"]),
			str(auth["user_id"])),
		PlanType: firstNonEmpty(str(auth["chatgpt_plan_type"])),
		AccountID: firstNonEmpty(
			str(auth["chatgpt_account_id"]),
			str(auth["account_id"])),
		OrganizationID:          firstNonEmpty(str(org["id"])),
		OrganizationName:        firstNonEmpty(str(org["title"]), str(org["name"])),
		SubscriptionActiveUntil: parseClaimDate(auth["chatgpt_subscription_active_until"]),
	}, true
}

// TokenExpired reports whether the JWT access token is expired or about to be
// (within skew). A token without a readable exp claim counts as expired.
func TokenExpired(jwt string, now time.Time, skew time.Duration) bool {
	object, ok := jwtPayload(jwt)
	if !ok {
		return true
	}
	exp, ok := object["exp"].(float64)
	if !ok {
		return true
	}
	return unixSeconds(exp).Sub(now) <= skew
}

func jwtPayload(jwt string) (map[string]any, bool) {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return nil, false
	}
	data, ok := decodeBase64URL(parts[1])
	if !ok {
		return nil, false
	}
	var object map[string]any
	if json.Unmarshal(data, &object) != nil || object == nil {
		return nil, false
	}
	return object, true
}

func decodeBase64URL(value string) ([]byte, bool) {
	text := strings.NewReplacer("-", "+", "_", "/").Replace(value)
	if pad := (4 - len(text)%4) % 4; pad > 0 {
		text += strings.Repeat("=", pad)
	}
	data, err := base64.StdEncoding.DecodeString(text)
	return data, err == nil
}

// selectedOrganization is the single default organization, or the only valid
// one when none is marked default.
func selectedOrganization(auth map[string]any) map[string]any {
	organizations, _ := auth["organizations"].([]any)
	var valid, defaults []map[string]any
	for _, item := range organizations {
		org, ok := item.(map[string]any)
		if !ok || firstNonEmpty(str(org["id"])) == "" {
			continue
		}
		valid = append(valid, org)
		if isDefault, _ := org["is_default"].(bool); isDefault {
			defaults = append(defaults, org)
		}
	}
	switch {
	case len(defaults) == 1:
		return defaults[0]
	case len(defaults) == 0 && len(valid) == 1:
		return valid[0]
	}
	return nil
}

func parseClaimDate(value any) time.Time {
	switch v := value.(type) {
	case string:
		if t, ok := parseTimestamp(v); ok {
			return t
		}
	case float64:
		if v > 0 {
			return unixSeconds(v)
		}
	}
	return time.Time{}
}

func unixSeconds(seconds float64) time.Time {
	whole := int64(seconds)
	return time.Unix(whole, int64((seconds-float64(whole))*1e9)).UTC()
}

// parseTimestamp reads an ISO 8601 internet date-time, with or without
// fractional seconds (RunwayDates.parse).
func parseTimestamp(text string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

// FormatTimestamp renders t as the whole-second UTC form used for last_refresh
// (RunwayDates.string).
func FormatTimestamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05Z") }

func str(value any) string {
	s, _ := value.(string)
	return s
}

// firstNonEmpty returns the first value that is not blank, trimmed.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}
