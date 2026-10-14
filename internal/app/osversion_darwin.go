//go:build darwin

package app

import (
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// macOSAtLeast reports whether the running macOS is at least major.
func macOSAtLeast(major int) bool {
	v, err := unix.Sysctl("kern.osproductversion")
	if err != nil {
		return false
	}
	n, err := strconv.Atoi(strings.SplitN(v, ".", 2)[0])
	return err == nil && n >= major
}
