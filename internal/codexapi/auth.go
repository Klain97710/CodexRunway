package codexapi

// Auth is the part of a Codex credential the API calls need. The storage layer
// owns the full auth document; callers copy these fields out of it.
type Auth struct {
	IDToken     string
	AccessToken string
	// AccountID is tokens.account_id, the workspace selected for the credential.
	AccountID string
}

// OAuthAccountID is the workspace id sent as ChatGPT-Account-Id
// (AccountIdentity.oauthAccountId): the selected id, else the account id both
// tokens agree on, else the organization id both agree on. Conflicting claims
// yield "".
func (a Auth) OAuthAccountID() string {
	if selected := firstNonEmpty(a.AccountID); selected != "" {
		return selected
	}
	id, _ := DecodeClaims(a.IDToken)
	access, _ := DecodeClaims(a.AccessToken)
	if explicit := []string{id.AccountID, access.AccountID}; anyNonEmpty(explicit) {
		return uniqueIdentifier(explicit)
	}
	return uniqueIdentifier([]string{id.OrganizationID, access.OrganizationID})
}

func anyNonEmpty(values []string) bool {
	for _, v := range values {
		if firstNonEmpty(v) != "" {
			return true
		}
	}
	return false
}

// uniqueIdentifier returns the single distinct non-blank identifier, or "" if
// there is none or they disagree.
func uniqueIdentifier(values []string) string {
	found := ""
	for _, v := range values {
		v = firstNonEmpty(v)
		switch {
		case v == "":
		case found == "":
			found = v
		case found != v:
			return ""
		}
	}
	return found
}
