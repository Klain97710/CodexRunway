package claude

import "strings"

// TierKind is the subscription badge derived from subscriptionType and
// rateLimitTier. Names that are not one of the known pairs stay unknown.
type TierKind int

const (
	TierUnknown TierKind = iota
	TierPro
	TierMax5x
	TierMax20x
	TierTeam
	TierEnterprise
	// TierMax is a Max plan whose 5x / 20x level is not known.
	TierMax
)

// tierKinds lists every named tier, used to map a label back to its kind.
var tierKinds = []TierKind{TierPro, TierMax, TierMax5x, TierMax20x, TierTeam, TierEnterprise}

// DisplayName is the English badge label, "" for an unknown tier so no
// badge is drawn. Brand names stay unlocalized.
func (k TierKind) DisplayName() string {
	switch k {
	case TierPro:
		return "Pro"
	case TierMax:
		return "Max"
	case TierMax5x:
		return "Max 5x"
	case TierMax20x:
		return "Max 20x"
	case TierTeam:
		return "Team"
	case TierEnterprise:
		return "Enterprise"
	default:
		return ""
	}
}

// TierFrom maps subscriptionType and rateLimitTier to a badge.
// rateLimitTier values look like default_claude_max_20x; its 20x / 5x marker
// sets the Max level, checked 20x first. A Max plan without a marker is the
// generic Max, and an empty subscriptionType falls back to rateLimitTier.
func TierFrom(subscriptionType, rateLimitTier string) TierKind {
	sub := strings.ToLower(strings.TrimSpace(subscriptionType))
	tier := strings.ToLower(strings.TrimSpace(rateLimitTier))
	if sub == "" && strings.Contains(tier, "max") {
		sub = "max"
	}
	switch strings.TrimPrefix(sub, "claude_") {
	case "max":
		switch {
		case strings.Contains(tier, "20x"):
			return TierMax20x
		case strings.Contains(tier, "5x"):
			return TierMax5x
		default:
			return TierMax
		}
	case "pro":
		return TierPro
	case "team":
		return TierTeam
	case "enterprise":
		return TierEnterprise
	default:
		return TierUnknown
	}
}

// TierResolve maps a badge label from DisplayName back to its kind.
func TierResolve(label string) TierKind {
	label = strings.TrimSpace(label)
	for _, k := range tierKinds {
		if strings.EqualFold(label, k.DisplayName()) {
			return k
		}
	}
	return TierUnknown
}
