package claude

import (
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/licoy/quosparr/internal/codexapi"
)

// resetGrantQuery asks the usage endpoint for the cedar_ember reset grant
// block, as the Claude Code CLI does.
const resetGrantQuery = "cedar_ember=1"

// maxGrantResets bounds how many credits one grant expands into.
const maxGrantResets = 100

var resetGrantIDPattern = regexp.MustCompile(`^[a-z0-9_-]{1,40}$`)

// ResetGrant is one banked usage reset grant. ResetsLeft of ResetsTotal can
// still be spent; zero times mean no bound.
type ResetGrant struct {
	ID          string
	ResetsTotal int
	ResetsLeft  int
	StartsAt    time.Time
	EndsAt      time.Time
	Paused      bool
}

// ResetGrantStatus is the cedar_ember block of the usage response.
type ResetGrantStatus struct {
	Eligible bool
	Grants   []ResetGrant
}

// Shown reports whether the account takes part in the reset grant program.
func (s ResetGrantStatus) Shown() bool {
	return s.Eligible || len(s.Grants) > 0
}

// parseResetGrants reads the cedar_ember block. A missing or malformed block
// is nil: one bad grant, a duplicate id or a bad timestamp drops the whole
// block rather than showing a partial count.
func parseResetGrants(v any) *ResetGrantStatus {
	block, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	eligible, ok := block["eligible"].(bool)
	if !ok {
		return nil
	}
	status := &ResetGrantStatus{Eligible: eligible}
	raw, ok := block["grants"].([]any)
	if !ok && block["grants"] != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, item := range raw {
		grant, ok := parseResetGrant(item)
		if !ok || seen[grant.ID] {
			return nil
		}
		seen[grant.ID] = true
		status.Grants = append(status.Grants, grant)
	}
	sort.Slice(status.Grants, func(i, j int) bool { return status.Grants[i].ID < status.Grants[j].ID })
	return status
}

func parseResetGrant(v any) (ResetGrant, bool) {
	obj, ok := v.(map[string]any)
	if !ok {
		return ResetGrant{}, false
	}
	id, _ := obj["id"].(string)
	total, okTotal := grantCount(obj["resets_total"])
	left, okLeft := grantCount(obj["resets_left"])
	if !resetGrantIDPattern.MatchString(id) || !okTotal || !okLeft || left > total {
		return ResetGrant{}, false
	}
	grant := ResetGrant{ID: id, ResetsTotal: total, ResetsLeft: left}
	if grant.StartsAt, ok = grantTime(obj["starts_at"]); !ok {
		return ResetGrant{}, false
	}
	if grant.EndsAt, ok = grantTime(obj["ends_at"]); !ok {
		return ResetGrant{}, false
	}
	switch p := obj["paused"].(type) {
	case nil:
	case bool:
		grant.Paused = p
	default:
		return ResetGrant{}, false
	}
	return grant, true
}

func grantCount(v any) (int, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	i, err := n.Int64()
	if err != nil || i < 0 || i > 1<<31-1 {
		return 0, false
	}
	return int(i), true
}

// grantTime is an optional timestamp: absent or null is the zero time, any
// other non-timestamp value fails.
func grantTime(v any) (time.Time, bool) {
	if v == nil {
		return time.Time{}, true
	}
	s, ok := v.(string)
	if !ok {
		return time.Time{}, false
	}
	return parseTimestamp(s)
}

// CodexSnapshot expands the grants into one reset credit per reset for the
// shared reset credit views. Resets left are "available" while the grant can
// be spent at now, else "unavailable"; spent resets are "used". updatedAt
// stamps the snapshot.
func (s ResetGrantStatus) CodexSnapshot(now, updatedAt time.Time) codexapi.ResetCreditsSnapshot {
	var credits []codexapi.ResetCredit
	available := 0
	for _, g := range s.Grants {
		spendable := s.Eligible && !g.Paused && !g.StartsAt.After(now) && (g.EndsAt.IsZero() || g.EndsAt.After(now))
		for i := 0; i < min(g.ResetsTotal, maxGrantResets); i++ {
			credit := codexapi.ResetCredit{ID: g.ID + "-" + strconv.Itoa(i+1), CreatedAt: g.StartsAt, ExpiresAt: g.EndsAt}
			switch {
			case i >= g.ResetsLeft:
				credit.Status = "used"
			case spendable:
				credit.Status = "available"
				available++
			default:
				credit.Status = "unavailable"
			}
			if left := g.EndsAt.Sub(now).Seconds(); !g.EndsAt.IsZero() && left > 0 {
				credit.RemainingSeconds = left
			}
			credits = append(credits, credit)
		}
	}
	if credits == nil {
		credits = []codexapi.ResetCredit{}
	}
	return codexapi.ResetCreditsSnapshot{AvailableCount: available, Credits: credits, UpdatedAt: updatedAt}
}
