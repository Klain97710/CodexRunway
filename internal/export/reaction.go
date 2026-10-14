package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/licoy/quosparr/internal/export/swiftjson"
	"github.com/licoy/quosparr/internal/platform"
)

const (
	// ReactionFileName is the visitor file inside ~/.quosparr.
	ReactionFileName = "reaction-visitor.json"
	// ReactionCookieName is the cookie the hosted reaction service sets.
	ReactionCookieName = "hr_react"
)

// DefaultReactionPath is ~/.quosparr/reaction-visitor.json under home.
func DefaultReactionPath(home string) string {
	return filepath.Join(platform.RunwayDir(home), ReactionFileName)
}

// ParseVisitorID trims, lowercases and accepts exactly 32 hex digits.
func ParseVisitorID(value string) (string, bool) {
	raw := strings.ToLower(strings.TrimSpace(value))
	if len(raw) != 32 {
		return "", false
	}
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return "", false
		}
	}
	return raw, true
}

// VisitorIDFromSetCookie reads the visitor ID from the value of a Set-Cookie
// header: "hr_react=<id>; HttpOnly; ...".
func VisitorIDFromSetCookie(header string) (string, bool) {
	pair, _, _ := strings.Cut(header, ";")
	name, value, ok := strings.Cut(pair, "=")
	if !ok || strings.Trim(name, " \t") != ReactionCookieName {
		return "", false
	}
	return ParseVisitorID(value)
}

// ReactionStore persists the visitor ID the reaction service minted, so the
// same visitor is reused on the next launch. The ID is never invented here.
// The file is {"updatedAt":"...","visitorId":"<32 hex>"} with mode 0600.
type ReactionStore struct {
	Path string
	// Now stamps updatedAt; nil means time.Now.
	Now func() time.Time
}

type reactionPayload struct {
	VisitorID string    `json:"visitorId"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// LoadVisitorID returns the stored ID, or false when the file is missing,
// undecodable (visitorId and a valid updatedAt are both required) or holds an
// invalid ID.
func (s ReactionStore) LoadVisitorID() (string, bool) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return "", false
	}
	var raw struct {
		VisitorID *string `json:"visitorId"`
		UpdatedAt *string `json:"updatedAt"`
	}
	if json.Unmarshal(data, &raw) != nil || raw.VisitorID == nil || raw.UpdatedAt == nil {
		return "", false
	}
	if _, err := swiftjson.ParseDate(*raw.UpdatedAt); err != nil {
		return "", false
	}
	return ParseVisitorID(*raw.VisitorID)
}

// SaveVisitorID stores a valid ID (an invalid one is ignored and nothing is
// written) unless it is already the stored one. The file is replaced
// atomically and is readable by the owner only.
func (s ReactionStore) SaveVisitorID(raw string) error {
	id, ok := ParseVisitorID(raw)
	if !ok {
		return nil
	}
	if current, ok := s.LoadVisitorID(); ok && current == id {
		return nil
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	data, err := swiftjson.Marshal(reactionPayload{VisitorID: id, UpdatedAt: now()}, false)
	if err != nil {
		return err
	}
	return platform.WriteFileAtomic(s.Path, data, 0o600)
}
