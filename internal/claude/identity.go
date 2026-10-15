package claude

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/licoy/quosparr/internal/codex"
)

// CredentialIdentity is who a Claude subscription login belongs to.
// It is read from .claude.json oauthAccount or from the profile endpoint,
// never from mcpOAuth. Empty fields stay empty.
type CredentialIdentity struct {
	AccountUUID      string     `json:"accountUuid,omitempty"`
	OrganizationUUID string     `json:"organizationUuid,omitempty"`
	Email            string     `json:"email,omitempty"`
	OrganizationName string     `json:"organizationName,omitempty"`
	DisplayName      string     `json:"displayName,omitempty"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
	HasAccessToken   bool       `json:"hasAccess"`
	HasRefreshToken  bool       `json:"hasRefresh"`
}

// StableID is "claude-" plus the codex stable hash of the account UUID and
// the organization UUID. Both are trimmed and lower-cased. Email is not part
// of the id.
func (c CredentialIdentity) StableID() string {
	material := strings.ToLower(strings.TrimSpace(c.AccountUUID)) + "|" +
		strings.ToLower(strings.TrimSpace(c.OrganizationUUID))
	return "claude-" + codex.StableHash(material)
}

// ResolvedDisplayName is the best human label.
func (c CredentialIdentity) ResolvedDisplayName() string {
	return firstNonEmpty(c.DisplayName, c.Email, c.OrganizationName, c.AccountUUID, c.StableID())
}

// OAuthAccountValue is the oauthAccount object written on a switch.
// Token fields are not part of the identity and are not written.
func (c CredentialIdentity) OAuthAccountValue() map[string]any {
	out := map[string]any{}
	put := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			out[key] = value
		}
	}
	put("accountUuid", c.AccountUUID)
	put("emailAddress", c.Email)
	put("organizationUuid", c.OrganizationUUID)
	put("organizationName", c.OrganizationName)
	put("displayName", c.DisplayName)
	return out
}

func (c CredentialIdentity) withCredential(doc AuthDocument) CredentialIdentity {
	c.HasAccessToken = doc.OAuth().AccessToken != ""
	c.HasRefreshToken = doc.OAuth().RefreshToken != ""
	if doc.OAuth().HasExpiry {
		t := doc.ExpiresAt()
		c.ExpiresAt = &t
	}
	return c
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

// IdentityFromOAuthAccount reads only the oauthAccount object.
// A missing oauthAccount is an empty identity, not an error.
func IdentityFromOAuthAccount(file []byte) (CredentialIdentity, error) {
	if len(bytes.TrimSpace(file)) == 0 {
		return CredentialIdentity{}, nil
	}
	root, err := rawObject(file)
	if err != nil {
		return CredentialIdentity{}, err
	}
	raw, ok := root["oauthAccount"]
	if !ok || len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return CredentialIdentity{}, nil
	}
	if !isJSONObject(raw) {
		return CredentialIdentity{}, ErrAuthInvalidRoot
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return CredentialIdentity{}, ErrAuthInvalidRoot
	}
	return CredentialIdentity{
		AccountUUID:      firstRaw(obj, "accountUuid", "accountUUID"),
		OrganizationUUID: firstRaw(obj, "organizationUuid", "organizationUUID"),
		Email:            firstRaw(obj, "emailAddress", "email"),
		OrganizationName: firstRaw(obj, "organizationName"),
		DisplayName:      firstRaw(obj, "displayName"),
	}, nil
}

func firstRaw(obj map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		if s := strings.TrimSpace(stringField(obj[key])); s != "" {
			return s
		}
	}
	return ""
}

// ReadOAuthIdentity reads oauthAccount from the config candidates.
// The first file that actually carries an account UUID wins.
func ReadOAuthIdentity(configDir, home string) (CredentialIdentity, error) {
	var lastErr error
	for _, path := range GlobalConfigCandidates(configDir, home) {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			lastErr = err
			continue
		}
		id, err := IdentityFromOAuthAccount(data)
		if err != nil {
			lastErr = err
			continue
		}
		if id.AccountUUID != "" {
			return id, nil
		}
	}
	if lastErr != nil {
		return CredentialIdentity{}, lastErr
	}
	return CredentialIdentity{}, nil
}

// IdentityFromProfile reads GET /api/oauth/profile.
// account.uuid, account.email and organization.uuid/name are the fields used.
func IdentityFromProfile(data []byte) (CredentialIdentity, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil || root == nil {
		return CredentialIdentity{}, ErrUsageParse
	}
	account := nestedStrings(root["account"])
	org := nestedStrings(root["organization"])
	return CredentialIdentity{
		AccountUUID:      firstNonEmpty(account["uuid"], account["accountUuid"]),
		OrganizationUUID: firstNonEmpty(org["uuid"], org["organizationUuid"]),
		Email:            firstNonEmpty(account["email"], account["emailAddress"], account["email_address"]),
		OrganizationName: firstNonEmpty(org["name"], org["organizationName"]),
		DisplayName:      firstNonEmpty(account["displayName"], account["display_name"], account["name"]),
	}, nil
}

func nestedStrings(raw json.RawMessage) map[string]string {
	out := map[string]string{}
	if !isJSONObject(raw) {
		return out
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return out
	}
	for key, value := range obj {
		if s := strings.TrimSpace(stringField(value)); s != "" {
			out[key] = s
		}
	}
	return out
}
