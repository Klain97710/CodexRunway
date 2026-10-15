//go:build !darwin

package claude

import (
	"context"
	"errors"
)

func newSecurityRunner() CommandRunner { return unsupportedRunner{} }

type unsupportedRunner struct{}

func (unsupportedRunner) Run(context.Context, string, []string, []byte) ([]byte, int, error) {
	return nil, 1, errors.New("claude: keychain is only available on macOS")
}
