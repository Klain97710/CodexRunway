package runway

import (
	"context"
	"errors"
	"time"

	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/codexapi"
)

// ErrNoAccount means a refreshed token has no account to be stored under.
var ErrNoAccount = errors.New("runway: no managed account for the refreshed credential")

// TokenPersister implements codexapi.TokenPersister over the account store.
// For the current account it updates the managed copy and the official
// auth.json; for any other account only the managed copy (see
// codex.Store.WriteRefreshedCredential). It never logs token values.
type TokenPersister struct {
	Store *codex.Store
}

var _ codexapi.TokenPersister = TokenPersister{}

// SaveRefreshed merges tokens into the stored credential of accountID and
// writes it back.
func (p TokenPersister) SaveRefreshed(ctx context.Context, accountID string, tokens codexapi.TokenSet, refreshedAt time.Time) error {
	// ctx is deliberately not checked: the server may have rotated the
	// refresh token already, so persisting must not be skipped on cancel.
	_ = ctx
	if p.Store == nil || accountID == "" {
		return ErrNoAccount
	}
	auth, err := p.Store.LoadCredential(accountID)
	if err != nil {
		return err
	}
	merged := tokens.MergedInto(codexapi.TokenSet{
		IDToken:      auth.Tokens.IDToken,
		AccessToken:  auth.Tokens.AccessToken,
		RefreshToken: auth.Tokens.RefreshToken,
	})
	auth.Tokens.IDToken = merged.IDToken
	auth.Tokens.AccessToken = merged.AccessToken
	auth.Tokens.RefreshToken = merged.RefreshToken
	auth.LastRefresh = codexapi.FormatTimestamp(refreshedAt)
	_, err = p.Store.WriteRefreshedCredential(accountID, auth, refreshedAt)
	return err
}
