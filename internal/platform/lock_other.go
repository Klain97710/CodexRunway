//go:build !unix && !windows

package platform

import (
	"errors"
	"os"
)

func lockFile(*os.File) error {
	return errors.New("single-instance lock is not supported on this platform")
}
func unlockFile(*os.File) error { return nil }
