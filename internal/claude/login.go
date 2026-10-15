package claude

import (
	"context"
	"errors"
)

// ErrLoginUseCLI is the in-app login result. Quosparr does not start
// `claude auth login` and does not implement OAuth PKCE. Isolated login was
// not verified: this machine is an API gateway and has no subscription
// account. Run `claude auth login` in a terminal, then import the current login.
var ErrLoginUseCLI = errors.New("claude: run claude auth login in a terminal, then import the current login")

// Login refuses in-app login. See ErrLoginUseCLI.
func Login(context.Context) error { return ErrLoginUseCLI }
