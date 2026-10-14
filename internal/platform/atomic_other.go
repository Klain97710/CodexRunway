//go:build !windows

package platform

import "os"

func createAtomicTemp(dir, pattern string, _ bool) (*os.File, error) {
	return os.CreateTemp(dir, pattern)
}

func replaceFile(oldPath, path string) error { return os.Rename(oldPath, path) }
