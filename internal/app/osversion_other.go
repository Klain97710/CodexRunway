//go:build !darwin

package app

func macOSAtLeast(int) bool { return false }
