//go:build linux

package usage

import "syscall"

type statT = syscall.Stat_t
