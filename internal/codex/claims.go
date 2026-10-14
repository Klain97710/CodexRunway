package codex

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

const (
	profileClaim = "https://api.openai.com/profile"
	authClaim    = "https://api.openai.com/auth"
)

// firstNonEmpty returns the first value that is not blank, trimmed.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

// jwtPayload decodes the claims object of a JWT without verifying anything.
func jwtPayload(jwt string) (map[string]any, bool) {
	parts := strings.FieldsFunc(jwt, func(r rune) bool { return r == '.' })
	if len(parts) < 2 {
		return nil, false
	}
	data, ok := decodeBase64URL(parts[1])
	if !ok {
		return nil, false
	}
	var claims map[string]any
	if err := json.Unmarshal(data, &claims); err != nil || claims == nil {
		return nil, false
	}
	return claims, true
}

// decodeBase64URL reads URL-safe base64 with or without padding, rejecting
// stray characters the way a strict Base64 decoder does.
func decodeBase64URL(s string) ([]byte, bool) {
	text := strings.NewReplacer("-", "+", "_", "/").Replace(s)
	for _, r := range text {
		ok := r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '+' || r == '/'
		if !ok {
			return nil, false
		}
	}
	text += strings.Repeat("=", (4-len(text)%4)%4)
	data, err := base64.StdEncoding.DecodeString(text)
	return data, err == nil
}

// IsTokenExpired reports whether the JWT expires within 60 seconds of now. A
// token that cannot be decoded, or has no numeric exp, counts as expired.
func IsTokenExpired(jwt string, now time.Time) bool {
	return tokenExpiredWithin(jwt, now, 60*time.Second)
}

func tokenExpiredWithin(jwt string, now time.Time, skew time.Duration) bool {
	claims, ok := jwtPayload(jwt)
	if !ok {
		return true
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return true
	}
	expiry := time.Unix(0, 0).Add(time.Duration(exp * float64(time.Second)))
	return expiry.Sub(now) <= skew
}

// IdentityClaims are the account facts a Codex id or access token carries.
type IdentityClaims struct {
	Email            string
	Username         string
	Subject          string
	UserID           string
	PlanType         string
	AccountID        string
	OrganizationID   string
	OrganizationName string
	// SubscriptionActiveUntil is the billing period end, when present.
	SubscriptionActiveUntil *time.Time
}

// DecodeIdentityClaims reads the claims of jwt, or returns nil when it is not
// a decodable JWT.
func DecodeIdentityClaims(jwt string) *IdentityClaims {
	claims, ok := jwtPayload(jwt)
	if !ok {
		return nil
	}
	profile, _ := claims[profileClaim].(map[string]any)
	auth, _ := claims[authClaim].(map[string]any)
	org := selectedOrganization(auth)
	return &IdentityClaims{
		Email:    firstNonEmpty(claimString(claims, "email"), claimString(profile, "email")),
		Username: firstNonEmpty(claimString(claims, "preferred_username"), claimString(claims, "name")),
		Subject:  firstNonEmpty(claimString(claims, "sub")),
		UserID: firstNonEmpty(
			claimString(auth, "chatgpt_user_id"), claimString(auth, "user_id")),
		PlanType: firstNonEmpty(claimString(auth, "chatgpt_plan_type")),
		AccountID: firstNonEmpty(
			claimString(auth, "chatgpt_account_id"), claimString(auth, "account_id")),
		OrganizationID:          firstNonEmpty(claimString(org, "id")),
		OrganizationName:        firstNonEmpty(claimString(org, "title"), claimString(org, "name")),
		SubscriptionActiveUntil: claimTime(auth["chatgpt_subscription_active_until"]),
	}
}

func claimString(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// selectedOrganization picks the single default organization, or the only
// organization when none is marked default; an ambiguous list selects none.
func selectedOrganization(auth map[string]any) map[string]any {
	list, ok := auth["organizations"].([]any)
	if !ok {
		return nil
	}
	var valid, defaults []map[string]any
	for _, e := range list {
		m, ok := e.(map[string]any)
		if !ok {
			return nil // the list is not an array of objects
		}
		if firstNonEmpty(claimString(m, "id")) == "" {
			continue
		}
		valid = append(valid, m)
		if d, _ := m["is_default"].(bool); d {
			defaults = append(defaults, m)
		}
	}
	if len(defaults) == 1 {
		return defaults[0]
	}
	if len(defaults) == 0 && len(valid) == 1 {
		return valid[0]
	}
	return nil
}

func claimTime(v any) *time.Time {
	switch t := v.(type) {
	case string:
		if d, ok := ParseTime(t); ok {
			return &d
		}
	case float64:
		if t > 0 {
			d := time.Unix(0, 0).Add(time.Duration(t * float64(time.Second)))
			return &d
		}
	}
	return nil
}

// Tier is a coarse subscription class derived from a plan type string.
type Tier string

// The subscription tiers. Official wire values stay prolite / pro / promax;
// the display names are Pro 100 / 200 / 500.
const (
	TierFree       Tier = "free"
	TierPlus       Tier = "plus"
	TierPro100     Tier = "pro100"
	TierPro200     Tier = "pro200"
	TierPro500     Tier = "pro500"
	TierBusiness   Tier = "business"
	TierTeam       Tier = "team"
	TierEnterprise Tier = "enterprise"
	TierEdu        Tier = "edu"
	TierAPI        Tier = "api"
	TierUnknown    Tier = "unknown"
)

var (
	pro500Types = []string{"promax", "pro-max", "pro-500", "pro500", "codex-pro-500"}
	pro100Types = []string{"prolite", "pro-lite", "pro-5x", "codex-pro-5x", "pro-100", "pro100"}
	pro200Types = []string{"pro", "pro-20x", "codex-pro-20x", "pro-200", "pro200"}
)

func containsString(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

// ResolveTier maps a plan type (or, when blank, the fallback) to a Tier.
// More specific Pro tiers are matched before the generic "pro" substring.
func ResolveTier(planType, fallback string) Tier {
	raw := firstNonEmpty(planType, fallback)
	if raw == "" {
		return TierUnknown
	}
	raw = strings.NewReplacer("_", "-", " ", "-").Replace(strings.ToLower(raw))
	switch {
	case containsString(pro500Types, raw):
		return TierPro500
	case containsString(pro100Types, raw):
		return TierPro100
	case containsString(pro200Types, raw):
		return TierPro200
	}
	for _, c := range []struct {
		word string
		tier Tier
	}{
		{"enterprise", TierEnterprise}, {"business", TierBusiness}, {"team", TierTeam},
		{"edu", TierEdu}, {"api", TierAPI}, {"plus", TierPlus}, {"pro", TierPro200}, {"free", TierFree},
	} {
		if strings.Contains(raw, c.word) {
			return c.tier
		}
	}
	return TierUnknown
}

// projectsBillingCycle: paid ChatGPT plans renew monthly; free, API and
// unknown plans must not get an invented future date.
func (t Tier) projectsBillingCycle() bool {
	switch t {
	case TierPlus, TierPro100, TierPro200, TierPro500, TierBusiness, TierTeam, TierEnterprise, TierEdu:
		return true
	}
	return false
}

// AccountDisplay is what a credential shows about its owner.
type AccountDisplay struct {
	Authenticated bool
	DisplayName   string
	Email         string
	Username      string
	AccountID     string
	Tier          Tier
	// SubscriptionExpiresAt is the billing period end from local JWT claims;
	// a stale date on a paid plan is advanced month by month.
	SubscriptionExpiresAt *time.Time
}

// DisplayForAuth derives the display facts of auth. quotaPlan, when known,
// beats the plan in the tokens. now anchors billing-period projection.
func DisplayForAuth(auth Auth, quotaPlan string, now time.Time) AccountDisplay {
	idc, acc := DecodeIdentityClaims(auth.Tokens.IDToken), DecodeIdentityClaims(auth.Tokens.AccessToken)
	email := firstNonEmpty(idc.email(), acc.email())
	username := firstNonEmpty(idc.username(), acc.username(), idc.userID(), acc.userID(), idc.subject(), acc.subject())
	accountID := oauthAccountID(auth)
	plan := firstNonEmpty(quotaPlan, idc.plan(), acc.plan(), auth.AuthFilePlanType, auth.PlanType)
	tier := ResolveTier(plan, "")
	var raw *time.Time
	if idc != nil && idc.SubscriptionActiveUntil != nil {
		raw = idc.SubscriptionActiveUntil
	} else if acc != nil {
		raw = acc.SubscriptionActiveUntil
	}
	return AccountDisplay{
		Authenticated:         true,
		DisplayName:           firstNonEmpty(email, username, shortenID(accountID)),
		Email:                 email,
		Username:              username,
		AccountID:             accountID,
		Tier:                  tier,
		SubscriptionExpiresAt: ResolveSubscriptionExpiry(raw, tier, now),
	}
}

// Nil-safe accessors keep DisplayForAuth and the identity code short.
func (c *IdentityClaims) email() string {
	if c == nil {
		return ""
	}
	return c.Email
}
func (c *IdentityClaims) username() string {
	if c == nil {
		return ""
	}
	return c.Username
}
func (c *IdentityClaims) subject() string {
	if c == nil {
		return ""
	}
	return c.Subject
}
func (c *IdentityClaims) userID() string {
	if c == nil {
		return ""
	}
	return c.UserID
}
func (c *IdentityClaims) plan() string {
	if c == nil {
		return ""
	}
	return c.PlanType
}
func (c *IdentityClaims) accountID() string {
	if c == nil {
		return ""
	}
	return c.AccountID
}
func (c *IdentityClaims) orgID() string {
	if c == nil {
		return ""
	}
	return c.OrganizationID
}
func (c *IdentityClaims) orgName() string {
	if c == nil {
		return ""
	}
	return c.OrganizationName
}

// ResolveSubscriptionExpiry returns raw unless it is in the past on a paid
// tier, where a JWT that was not refreshed after renewal is advanced by whole
// UTC calendar months to the next future period end.
func ResolveSubscriptionExpiry(raw *time.Time, tier Tier, now time.Time) *time.Time {
	if raw == nil || !raw.Before(now) || !tier.projectsBillingCycle() {
		return raw
	}
	projected := *raw
	for i := 0; i < 48 && projected.Before(now); i++ {
		next := addMonthClamped(projected)
		if !next.After(projected) {
			break
		}
		projected = next
	}
	return &projected
}

// addMonthClamped adds one calendar month in UTC, clamping the day to the
// length of the target month (Jan 31 becomes Feb 28).
func addMonthClamped(t time.Time) time.Time {
	t = t.UTC()
	year, month, day := t.Date()
	first := time.Date(year, month+1, 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	last := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > last {
		day = last
	}
	return time.Date(first.Year(), first.Month(), day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
}

// shortenID turns "acct_123456789" into "acct_...6789"; short ids stay.
func shortenID(id string) string {
	id = firstNonEmpty(id)
	r := []rune(id)
	if len(r) <= 9 {
		return id
	}
	return string(r[:5]) + "..." + string(r[len(r)-4:])
}
