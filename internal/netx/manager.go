package netx

import (
	"context"
	"net/http"
	"sync"

	"github.com/licoy/quosparr/internal/store"
)

// The process-wide current context, the equivalent of Swift's RunwayNetwork.
// New requests resolve it when they start; requests already running keep the
// context they started with.
var current struct {
	mu  sync.Mutex
	ctx *Context
	err error
}

// Current returns the context new requests should use: the one set by
// Configure, the stored failure set by Block, or the system-proxy default.
func Current() (*Context, error) {
	current.mu.Lock()
	defer current.mu.Unlock()
	if current.err != nil {
		return nil, current.err
	}
	if current.ctx == nil {
		current.ctx = SystemContext()
	}
	return current.ctx, nil
}

// Configure installs c as the current context.
func Configure(c *Context) {
	swap(c, nil)
}

// ConfigureFrom builds a context from cfg and the credential store and installs
// it. On error the previous context stays in place; call Block to stop all
// network traffic instead.
func ConfigureFrom(cfg store.NetworkProxyConfiguration, creds CredentialStore, allowInteraction bool, opts ...Option) error {
	c, err := LoadContext(cfg, creds, allowInteraction, opts...)
	if err != nil {
		return err
	}
	Configure(c)
	return nil
}

// Block makes every new request fail with err (a broken proxy setting must not
// turn into a direct connection). A nil err is reported as ErrNotReady.
func Block(err error) {
	if err == nil {
		err = ErrNotReady
	}
	swap(nil, err)
}

func swap(c *Context, err error) {
	current.mu.Lock()
	previous := current.ctx
	current.ctx, current.err = c, err
	current.mu.Unlock()
	// Idle connections of the replaced context must not linger; requests still
	// running on it finish normally.
	if previous != nil && previous != c {
		previous.CloseIdleConnections()
	}
}

type contextKey struct{}

// WithContext scopes c to ctx: Resolve(ctx) returns it instead of Current.
func WithContext(ctx context.Context, c *Context) context.Context {
	return context.WithValue(ctx, contextKey{}, c)
}

// Resolve returns the network context scoped to ctx, or the current one.
func Resolve(ctx context.Context) (*Context, error) {
	if c, ok := ctx.Value(contextKey{}).(*Context); ok && c != nil {
		return c, nil
	}
	return Current()
}

// ClientFor returns the client of the context that applies to ctx.
func ClientFor(ctx context.Context, policy Policy) (*http.Client, error) {
	c, err := Resolve(ctx)
	if err != nil {
		return nil, err
	}
	return c.Client(policy), nil
}
