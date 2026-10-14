//go:build !windows

package sqliteruntime

import "os"

func publishFile(source, destination string) error {
	return os.Link(source, destination)
}
