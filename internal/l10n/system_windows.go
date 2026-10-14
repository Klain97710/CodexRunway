//go:build windows

package l10n

import (
	"os"

	"golang.org/x/sys/windows"
)

// SystemPreferredLanguages returns the user's preferred UI languages in
// priority order (GetUserPreferredUILanguages), for example
// ["zh-CN", "en-US"]. It falls back to the environment if the call fails.
func SystemPreferredLanguages() []string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(langs) == 0 {
		return languagesFromEnv(os.Getenv)
	}
	return langs
}
