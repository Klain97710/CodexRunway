package app

import (
	"context"

	"github.com/licoy/quosparr/internal/sessions"
	"github.com/licoy/quosparr/internal/store"
)

// SessionActions are the settings session-management operations. A nil
// function leaves the corresponding control unavailable.
type SessionActions struct {
	Query  func(context.Context, store.Provider) ([]sessions.ManagedSession, error)
	Detail func(context.Context, sessions.ManagedSession) (sessions.ManagedDetail, error)
	Delete func(context.Context, []sessions.ManagedSession) ([]sessions.DeleteResult, error)
	Reveal func(string) error
}
