package codexapi

import (
	"strings"
	"time"
)

// SubscriptionTier is the plan shown for an account.
type SubscriptionTier string

// Subscription tiers. Official wire values stay prolite / pro / promax; the
// display names are Pro 100 / 200 / 500.
const (
	TierFree     SubscriptionTier = "free"
	TierPlus     SubscriptionTier = "plus"
	TierPro100   SubscriptionTier = "pro100"
	TierPro200   SubscriptionTier = "pro200"
	TierPro500   SubscriptionTier = "pro500"
	TierBusiness SubscriptionTier = "business"
	// TierBusinessPremium is the self-serve Business plan with Pro 100
	// limits (self_serve_business_prolite).
	TierBusinessPremium SubscriptionTier = "business_premium"
	TierTeam            SubscriptionTier = "team"
	TierEnterprise      SubscriptionTier = "enterprise"
	TierEdu             SubscriptionTier = "edu"
	TierAPI             SubscriptionTier = "api"
	TierUnknown         SubscriptionTier = "unknown"
)

var (
	pro500PlanTypes = setOf("promax", "pro-max", "pro-500", "pro500", "codex-pro-500")
	pro100PlanTypes = setOf("prolite", "pro-lite", "pro-5x", "codex-pro-5x", "pro-100", "pro100")
	pro200PlanTypes = setOf("pro", "pro-20x", "codex-pro-20x", "pro-200", "pro200")
)

func setOf(values ...string) map[string]bool {
	m := make(map[string]bool, len(values))
	for _, v := range values {
		m[v] = true
	}
	return m
}

// ResolveTier maps a plan type string to a tier. The first non-blank of
// planType and fallback is used; more specific Pro tiers are matched before
// the generic "pro" fallback.
func ResolveTier(planType, fallback string) SubscriptionTier {
	raw := firstNonEmpty(planType, fallback)
	if raw == "" {
		return TierUnknown
	}
	raw = strings.ToLower(raw)
	raw = strings.NewReplacer("_", "-", " ", "-").Replace(raw)
	switch {
	case pro500PlanTypes[raw]:
		return TierPro500
	case pro100PlanTypes[raw]:
		return TierPro100
	case pro200PlanTypes[raw]:
		return TierPro200
	case strings.Contains(raw, "business") && (strings.Contains(raw, "prolite") || strings.Contains(raw, "pro-lite") || strings.Contains(raw, "premium")):
		return TierBusinessPremium
	case strings.Contains(raw, "enterprise"):
		return TierEnterprise
	case strings.Contains(raw, "business"):
		return TierBusiness
	case strings.Contains(raw, "team"):
		return TierTeam
	case strings.Contains(raw, "edu"):
		return TierEdu
	case strings.Contains(raw, "api"):
		return TierAPI
	case strings.Contains(raw, "plus"):
		return TierPlus
	case strings.Contains(raw, "pro"):
		return TierPro200
	case strings.Contains(raw, "free"):
		return TierFree
	}
	return TierUnknown
}

// ProjectsBillingCycle reports whether the tier renews monthly, so a stale
// subscription end date can be advanced. Free, API and unknown never invent a
// future date.
func (t SubscriptionTier) ProjectsBillingCycle() bool {
	switch t {
	case TierPlus, TierPro100, TierPro200, TierPro500, TierBusiness, TierBusinessPremium, TierTeam, TierEnterprise, TierEdu:
		return true
	}
	return false
}

// ResolveSubscriptionExpiry returns raw when it is still in the future. The JWT
// billing claim is often not refreshed after a renewal, so for a paid tier a
// past date is advanced by whole calendar months (UTC, month ends clamped)
// until it is in the future.
func ResolveSubscriptionExpiry(raw time.Time, tier SubscriptionTier, now time.Time) time.Time {
	if raw.IsZero() || !raw.Before(now) || !tier.ProjectsBillingCycle() {
		return raw
	}
	projected := raw.UTC()
	// The cap avoids a pathological loop if date math fails.
	for i := 0; i < 48 && projected.Before(now); i++ {
		next := addOneMonth(projected)
		if !next.After(projected) {
			break
		}
		projected = next
	}
	return projected
}

// addOneMonth adds a calendar month in UTC, clamping to the last day of a
// shorter month (31 Jan -> 28/29 Feb), as the Gregorian calendar does.
func addOneMonth(t time.Time) time.Time {
	year, month, day := t.Date()
	nextMonth := month + 1
	nextYear := year
	if nextMonth > 12 {
		nextMonth, nextYear = 1, year+1
	}
	if last := daysIn(nextYear, nextMonth); day > last {
		day = last
	}
	return time.Date(nextYear, nextMonth, day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
}

func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// AccountDisplay is the identity and plan summary of one credential
// (CodexAccountDisplay).
type AccountDisplay struct {
	DisplayName string
	Email       string
	Username    string
	AccountID   string
	Tier        SubscriptionTier
	// SubscriptionExpiresAt is zero when unknown.
	SubscriptionExpiresAt time.Time
}

// MakeAccountDisplay summarizes an OAuth credential. quotaPlan is the plan_type
// of the last quota response; authFilePlan and planType are the stored hints.
func MakeAccountDisplay(a Auth, quotaPlan, authFilePlan, planType string, now time.Time) AccountDisplay {
	id, _ := DecodeClaims(a.IDToken)
	access, _ := DecodeClaims(a.AccessToken)
	email := firstNonEmpty(id.Email, access.Email)
	username := firstNonEmpty(id.Username, access.Username, id.UserID, access.UserID, id.Subject, access.Subject)
	accountID := a.OAuthAccountID()
	plan := firstNonEmpty(quotaPlan, id.PlanType, access.PlanType, authFilePlan, planType)
	tier := ResolveTier(plan, "")
	rawExpiry := id.SubscriptionActiveUntil
	if rawExpiry.IsZero() {
		rawExpiry = access.SubscriptionActiveUntil
	}
	return AccountDisplay{
		DisplayName:           firstNonEmpty(email, username, shortenedAccountID(accountID)),
		Email:                 email,
		Username:              username,
		AccountID:             accountID,
		Tier:                  tier,
		SubscriptionExpiresAt: ResolveSubscriptionExpiry(rawExpiry, tier, now),
	}
}

func shortenedAccountID(value string) string {
	value = firstNonEmpty(value)
	runes := []rune(value)
	if len(runes) <= 9 {
		return value
	}
	return string(runes[:5]) + "..." + string(runes[len(runes)-4:])
}
