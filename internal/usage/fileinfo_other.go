//go:build !darwin && !linux && !windows

package usage

import (
	"errors"
	"os"
)

var errUnsupportedPlatform = errors.New("usage: source scanning is not supported on this platform")

func lstatMeta(path string) (fileMeta, error) { return fileMeta{}, errUnsupportedPlatform }
func fstatMeta(f *os.File) (fileMeta, error)  { return fileMeta{}, errUnsupportedPlatform }
