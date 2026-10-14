package codex

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaskAccountText hides the identifying part of a display string:
// "example@qq.com" becomes "e***@***.com"; anything else keeps its first
// character and continues with "***". Whitespace-only input is returned as
// is. It is for display only, never for storage.
func MaskAccountText(value string) string {
	trimmed := strings.TrimFunc(value, unicode.IsSpace)
	if trimmed == "" {
		return value
	}
	if masked, ok := maskWholeEmail(trimmed); ok {
		return masked
	}
	if strings.Contains(trimmed, "@") {
		return MaskEmailsIn(trimmed)
	}
	return maskHead(trimmed)
}

// MaskEmailsIn masks every email-looking token inside a longer string and
// leaves the rest as is.
func MaskEmailsIn(text string) string {
	if !strings.Contains(text, "@") {
		return text
	}
	runes := []rune(text)
	var out strings.Builder
	pos := 0
	for pos < len(runes) {
		at := indexRune(runes, '@', pos)
		if at < 0 {
			out.WriteString(string(runes[pos:]))
			break
		}
		tok := findEmailToken(runes, at, pos)
		out.WriteString(string(runes[pos:tok.start]))
		if tok.local == "" || tok.domain == "" {
			out.WriteString(string(runes[tok.start:tok.end]))
		} else {
			out.WriteString(maskHead(tok.local) + "@" + maskDomain(tok.domain))
		}
		pos = tok.end
	}
	return out.String()
}

type emailToken struct {
	start, end    int
	local, domain string
}

func findEmailToken(runes []rune, at, searchFrom int) emailToken {
	start := at
	for start > searchFrom && isEmailTokenRune(runes[start-1]) {
		start--
	}
	end := at + 1
	for end < len(runes) && isEmailTokenRune(runes[end]) {
		end++
	}
	for end > at+1 && runes[end-1] == '.' {
		end--
	}
	return emailToken{
		start:  start,
		end:    end,
		local:  string(runes[start:at]),
		domain: string(runes[at+1 : end]),
	}
}

func maskWholeEmail(value string) (string, bool) {
	at := strings.IndexByte(value, '@')
	if at < 0 || at+1 >= len(value) {
		return "", false
	}
	local, domain := value[:at], value[at+1:]
	if local == "" || domain == "" || strings.HasSuffix(domain, ".") {
		return "", false
	}
	if strings.ContainsFunc(value, unicode.IsSpace) || strings.Contains(domain, "@") {
		return "", false
	}
	return maskHead(local) + "@" + maskDomain(domain), true
}

// maskHead keeps the first grapheme cluster and hides the rest.
func maskHead(value string) string {
	first, rest := splitFirstCluster(value)
	if first == "" {
		return "***"
	}
	if rest == "" {
		return first
	}
	return first + "***"
}

// maskDomain turns "qq.com" and "mail.qq.com" into "***.com".
func maskDomain(domain string) string {
	dot := strings.LastIndexByte(domain, '.')
	if dot <= 0 || dot+1 >= len(domain) {
		return "***"
	}
	return "***" + domain[dot:]
}

func isEmailTokenRune(r rune) bool {
	if unicode.IsSpace(r) {
		return false
	}
	return !strings.ContainsRune("@,;:<>()[]\"'{}", r)
}

func indexRune(runes []rune, target rune, from int) int {
	for i := from; i < len(runes); i++ {
		if runes[i] == target {
			return i
		}
	}
	return -1
}

// splitFirstCluster cuts s after its first user-perceived character. It
// covers combining marks, variation selectors, emoji modifiers, zero-width
// joiner sequences, regional-indicator pairs and CRLF, which is what account
// names contain in practice; it is not a full UAX #29 implementation.
func splitFirstCluster(s string) (first, rest string) {
	if s == "" {
		return "", ""
	}
	r, n := utf8.DecodeRuneInString(s)
	end := n
	if r == '\r' && strings.HasPrefix(s[n:], "\n") {
		return s[:n+1], s[n+1:]
	}
	if isRegionalIndicator(r) {
		if next, m := utf8.DecodeRuneInString(s[end:]); m > 0 && isRegionalIndicator(next) {
			end += m
		}
	}
	for end < len(s) {
		next, m := utf8.DecodeRuneInString(s[end:])
		switch {
		case next == '‍':
			end += m
			if joined, k := utf8.DecodeRuneInString(s[end:]); k > 0 && !unicode.IsSpace(joined) {
				end += k
			}
		case extendsCluster(next):
			end += m
		default:
			return s[:end], s[end:]
		}
	}
	return s, ""
}

func extendsCluster(r rune) bool {
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc) ||
		(r >= 0x1F3FB && r <= 0x1F3FF) || // emoji skin tones
		(r >= 0xE0020 && r <= 0xE007F) // tag characters
}

func isRegionalIndicator(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }
