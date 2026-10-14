package l10n

import "strings"

// languagesFromEnv builds a preferred-language list from POSIX locale
// environment variables, honouring the usual precedence: LANGUAGE (a
// colon-separated priority list, only meaningful when the locale is not "C"),
// then LC_ALL, LC_MESSAGES and LANG. The "C"/"POSIX" locales are skipped.
func languagesFromEnv(getenv func(string) string) []string {
	var out []string
	add := func(raw string) {
		if id := cleanPOSIXLocale(raw); id != "" {
			out = append(out, id)
		}
	}
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := getenv(name); v != "" {
			if cleanPOSIXLocale(v) == "" {
				// An explicit C/POSIX locale disables LANGUAGE.
				return nil
			}
			for _, part := range strings.Split(getenv("LANGUAGE"), ":") {
				add(part)
			}
			add(v)
			return out
		}
	}
	for _, part := range strings.Split(getenv("LANGUAGE"), ":") {
		add(part)
	}
	return out
}

// cleanPOSIXLocale turns "zh_TW.UTF-8@euro" into "zh-TW"; "C" and "POSIX"
// become "".
func cleanPOSIXLocale(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i]
	}
	if s == "" || s == "C" || s == "POSIX" {
		return ""
	}
	return strings.ReplaceAll(s, "_", "-")
}
