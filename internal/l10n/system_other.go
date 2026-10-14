//go:build !darwin && !windows

package l10n

import "os"

// SystemPreferredLanguages returns the user's preferred languages from the
// LANGUAGE, LC_ALL, LC_MESSAGES and LANG environment variables, for example
// ["zh-TW", "en"].
func SystemPreferredLanguages() []string {
	return languagesFromEnv(os.Getenv)
}
