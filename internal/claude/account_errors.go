package claude

import (
	"errors"
	"fmt"
	"time"
)

const (
	// ErrorCodeReauth is stored when a refresh token is rejected.
	ErrorCodeReauth = "authentication_required"
	// ErrorCodeTokenExpired is stored when the current access token is stale.
	// Quosparr does not refresh that account; the user runs claude once.
	ErrorCodeTokenExpired = "token_expired"
	// ErrorCodeRateLimited is stored when usage returned 429.
	ErrorCodeRateLimited = "rate_limited"
	// ErrorCodeUsageParse is stored when the usage body could not be parsed.
	ErrorCodeUsageParse = "usage_parse_failed"
	// ErrorCodeUsage is stored for other usage failures.
	ErrorCodeUsage = "usage_failed"
	// ErrorCodeRefresh is stored when a non-current refresh failed.
	ErrorCodeRefresh = "refresh_failed"
	// ErrorCodeAPIUnauthorized is stored when a relay rejected an API key.
	ErrorCodeAPIUnauthorized = "api_unauthorized"
	// ErrorCodeAPI is stored for any other failed API balance read.
	ErrorCodeAPI = "api_failed"
)

var (
	// ErrIdentityRequired means the account UUID needed for a stable id is missing.
	ErrIdentityRequired = errors.New("claude: account UUID is required")
	// ErrCurrentAccountNotRefreshed means a token refresh was asked for the
	// account currently installed in Claude Code. That refresh is refused.
	ErrCurrentAccountNotRefreshed = errors.New("claude: current account is not refreshed")
	// ErrInvalidGrant means the token endpoint rejected the refresh token.
	ErrInvalidGrant = errors.New("claude: refresh token rejected")
	// ErrRefreshFailed is a token-endpoint failure that is not invalid_grant.
	ErrRefreshFailed = errors.New("claude: token refresh failed")
	// ErrCredentialChanged means the managed copy changed while a refresh was in flight.
	ErrCredentialChanged = errors.New("claude: credential changed during refresh")
	// ErrUsageParse means the usage or profile body was not the expected JSON.
	ErrUsageParse = errors.New("claude: usage response could not be parsed")
	// ErrUnauthorized is HTTP 401 or 403 from the usage or profile endpoint.
	ErrUnauthorized = errors.New("claude: usage unauthorized")
	// ErrUsageStatus is some other non-success HTTP status.
	ErrUsageStatus = errors.New("claude: usage request failed")
	// ErrOAuthAccountConflict means .claude.json changed on all three attempts.
	ErrOAuthAccountConflict = errors.New("claude: oauthAccount update conflicted")
	// ErrTokenExpired means the current account's access token is stale.
	// Quosparr does not refresh it. The user runs claude once.
	ErrTokenExpired = errors.New("claude: access token expired; run claude to refresh")
)

// AccountNotFoundError means the id is not in the index.
type AccountNotFoundError struct{ ID string }

func (e *AccountNotFoundError) Error() string {
	return fmt.Sprintf("claude: account %q not found", e.ID)
}

// CurrentAccountCannotBeRemovedError means Remove was asked to delete the current account.
type CurrentAccountCannotBeRemovedError struct{ ID string }

func (e *CurrentAccountCannotBeRemovedError) Error() string {
	return fmt.Sprintf("claude: current account %q cannot be removed", e.ID)
}

// ProcessesRunningError means Claude Code is running and the switch was not confirmed.
type ProcessesRunningError struct{ PIDs []int }

func (e *ProcessesRunningError) Error() string {
	return "claude: Claude Code is running; its process may keep the previous account"
}

// RateLimitedError is HTTP 429. HasRetryAfter is false when the response had
// no Retry-After header, so the caller can see that it still has to wait.
type RateLimitedError struct {
	RetryAfter    time.Duration
	HasRetryAfter bool
}

func (e *RateLimitedError) Error() string { return "claude: usage rate limited" }
