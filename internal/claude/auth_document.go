package claude

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

const oauthKey = "claudeAiOauth"

// Auth document errors. None of them includes credential text.
var (
	// ErrAuthInvalidRoot means the bytes are not a JSON object.
	ErrAuthInvalidRoot = errors.New("claude: auth document is not a JSON object")
	// ErrAuthNoOAuth means claudeAiOauth is missing or not an object.
	ErrAuthNoOAuth = errors.New("claude: auth document has no claudeAiOauth")
	// ErrIncompleteRefresh means a refresh response did not include both tokens.
	ErrIncompleteRefresh = errors.New("claude: refresh response missing access_token or refresh_token")
)

// OAuthFields is the claudeAiOauth object without any other top-level key.
type OAuthFields struct {
	AccessToken      string
	RefreshToken     string
	ExpiresAt        time.Time
	HasExpiry        bool
	Scopes           []string
	SubscriptionType string
	RateLimitTier    string
}

// AuthDocument is a parsed credentials document. Raw is the file as read.
type AuthDocument struct {
	Raw   []byte
	oauth OAuthFields
}

// OAuth returns the parsed claudeAiOauth fields.
func (d AuthDocument) OAuth() OAuthFields { return d.oauth }

// ExpiresAt is the access-token expiry. The zero time means the document
// did not carry expiresAt. The unit in the file is Unix milliseconds.
func (d AuthDocument) ExpiresAt() time.Time { return d.oauth.ExpiresAt }

// AccessExpired reports whether expiresAt is present and not after now.
// A missing expiry is not treated as expired.
func (d AuthDocument) AccessExpired(now time.Time) bool {
	return d.oauth.HasExpiry && !d.oauth.ExpiresAt.After(now)
}

// RequiresReauthentication reports whether the document cannot be used as a
// managed credential: no access token, or an expired one with no refresh token.
func (d AuthDocument) RequiresReauthentication(now time.Time) bool {
	if d.oauth.AccessToken == "" {
		return true
	}
	return d.AccessExpired(now) && d.oauth.RefreshToken == ""
}

// ParseAuthDocument parses a credentials document. Only claudeAiOauth is read.
// mcpOAuth and every other key are left uninterpreted in Raw.
func ParseAuthDocument(data []byte) (AuthDocument, error) {
	raw, err := oauthObject(data)
	if err != nil {
		return AuthDocument{}, err
	}
	fields, err := parseOAuthFields(raw)
	if err != nil {
		return AuthDocument{}, err
	}
	return AuthDocument{Raw: append([]byte(nil), data...), oauth: fields}, nil
}

func parseOAuthFields(raw json.RawMessage) (OAuthFields, error) {
	fields, err := decodeOAuthMap(raw)
	if err != nil {
		return OAuthFields{}, err
	}
	out := OAuthFields{
		AccessToken:      stringField(fields["accessToken"]),
		RefreshToken:     stringField(fields["refreshToken"]),
		SubscriptionType: stringField(fields["subscriptionType"]),
		RateLimitTier:    stringField(fields["rateLimitTier"]),
		Scopes:           stringList(fields["scopes"]),
	}
	if ms, ok := rawInt64(fields["expiresAt"]); ok {
		out.ExpiresAt = time.UnixMilli(ms)
		out.HasExpiry = true
	}
	return out, nil
}

func stringList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var items []string
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	return items
}

// ManagedDocument keeps only claudeAiOauth. The account library stores this
// shape and nothing else, so mcpOAuth never lands in ~/.quosparr.
func ManagedDocument(data []byte) ([]byte, error) {
	oauth, err := oauthObject(data)
	if err != nil {
		return nil, err
	}
	return marshalRawObject(map[string]json.RawMessage{oauthKey: oauth})
}

// ReplaceOAuth replaces only claudeAiOauth in official. Every other key,
// including mcpOAuth, is preserved byte-for-byte.
func ReplaceOAuth(official, target []byte) ([]byte, error) {
	oauth, err := oauthObject(target)
	if err != nil {
		return nil, err
	}
	root, err := rawObject(official)
	if err != nil {
		return nil, err
	}
	root[oauthKey] = oauth
	return marshalRawObject(root)
}

// ApplyTokenRefresh sets both rotated tokens on claudeAiOauth and leaves every
// other field and every sibling key unchanged. Both tokens are required so a
// one-time rotation cannot drop the previous refresh token. setExpiry writes
// expiresAt in Unix milliseconds when true.
func ApplyTokenRefresh(doc []byte, access, refresh string, expiresAt time.Time, setExpiry bool) ([]byte, error) {
	if access == "" || refresh == "" {
		return nil, ErrIncompleteRefresh
	}
	root, err := rawObject(doc)
	if err != nil {
		return nil, err
	}
	fields, err := decodeOAuthMap(root[oauthKey])
	if err != nil {
		return nil, err
	}
	fields["accessToken"] = rawString(access)
	fields["refreshToken"] = rawString(refresh)
	if setExpiry {
		fields["expiresAt"] = json.RawMessage(strconv.FormatInt(expiresAt.UnixMilli(), 10))
	}
	encoded, err := marshalRawObject(fields)
	if err != nil {
		return nil, err
	}
	root[oauthKey] = encoded
	return marshalRawObject(root)
}

// AccessTokenOf returns claudeAiOauth.accessToken.
func AccessTokenOf(data []byte) (string, error) {
	doc, err := ParseAuthDocument(data)
	if err != nil {
		return "", err
	}
	return doc.OAuth().AccessToken, nil
}
