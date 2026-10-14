// Package netx builds the explicit HTTP clients Quosparr uses for every
// network request. Each client belongs to one immutable proxy context; nothing
// here touches http.DefaultClient or http.DefaultTransport, and a proxy
// failure is reported instead of silently connecting directly.
package netx

import (
	"errors"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

// Errors of the network proxy layer. They mirror the Swift NetworkProxyError
// cases; the configuration ones are shared with internal/store.
var (
	ErrInvalidConfiguration   = store.ErrProxyInvalidConfiguration
	ErrInvalidHost            = store.ErrProxyInvalidHost
	ErrInvalidPort            = store.ErrProxyInvalidPort
	ErrCredentialsUnavailable = store.ErrProxyCredentialsUnavailable

	ErrInvalidCredentials    = errors.New("invalid network proxy credentials")
	ErrCredentialStoreFailed = errors.New("network proxy credential store failed")
	ErrConnectionFailed      = errors.New("network proxy connection failed")
	ErrAuthenticationFailed  = errors.New("network proxy authentication failed")
	ErrNotReady              = errors.New("network proxy is not ready")
)

// Error is a proxy-layer failure that keeps the underlying cause for
// errors.Is/As (for example context.DeadlineExceeded) while its message is the
// fixed Kind text, so a transport error can never echo a credential.
type Error struct {
	Kind  error
	Cause error
}

func (e *Error) Error() string { return e.Kind.Error() }

// Is matches the Kind sentinel, e.g. errors.Is(err, ErrConnectionFailed).
func (e *Error) Is(target error) bool { return target == e.Kind }

// Unwrap exposes the cause.
func (e *Error) Unwrap() error { return e.Cause }

// L10nKey returns the localization key of a proxy error, or "" for any other
// error.
func L10nKey(err error) l10n.Key {
	switch {
	case errors.Is(err, ErrInvalidConfiguration):
		return l10n.KeyProxyInvalidConfiguration
	case errors.Is(err, ErrInvalidHost):
		return l10n.KeyProxyInvalidHost
	case errors.Is(err, ErrInvalidPort):
		return l10n.KeyProxyInvalidPort
	case errors.Is(err, ErrInvalidCredentials):
		return l10n.KeyProxyInvalidCredentials
	case errors.Is(err, ErrCredentialsUnavailable):
		return l10n.KeyProxyCredentialsUnavailable
	case errors.Is(err, ErrCredentialStoreFailed):
		return l10n.KeyProxyCredentialStoreFailed
	case errors.Is(err, ErrConnectionFailed):
		return l10n.KeyProxyConnectionFailed
	case errors.Is(err, ErrAuthenticationFailed):
		return l10n.KeyProxyAuthenticationFailed
	case errors.Is(err, ErrNotReady):
		return l10n.KeyProxyNotReady
	}
	return ""
}
