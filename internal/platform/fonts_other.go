//go:build !darwin && !windows && !linux

package platform

func systemFontFamilies() []string { return nil }
