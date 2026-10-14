// Package l10n ports Quosparr's Swift localization layer: the translation
// tables, system-language resolution and Swift-style format strings.
package l10n

import "strings"

// Language is a resolved UI language. The values are the raw strings of the
// Swift ResolvedLanguage enum, so they can be stored in preferences verbatim.
type Language string

// Resolved languages.
const (
	English            Language = "english"
	SimplifiedChinese  Language = "simplifiedChinese"
	TraditionalChinese Language = "traditionalChinese"
	Korean             Language = "korean"
	Japanese           Language = "japanese"
	Russian            Language = "russian"
	French             Language = "french"
)

// PreferenceSystem is the raw value of the Swift LanguagePreference.system
// case ("follow the OS"). Every other preference raw value equals the
// corresponding Language value, so preferences are accepted as plain strings.
const PreferenceSystem = "system"

// Languages lists every resolved language in Swift enum order.
func Languages() []Language {
	return []Language{English, SimplifiedChinese, TraditionalChinese, Korean, Japanese, Russian, French}
}

// Preferences lists every preference raw value in Swift enum order, starting
// with PreferenceSystem.
func Preferences() []string {
	out := []string{PreferenceSystem}
	for _, l := range Languages() {
		out = append(out, string(l))
	}
	return out
}

// Valid reports whether l is one of the supported languages.
func (l Language) Valid() bool {
	for _, v := range Languages() {
		if v == l {
			return true
		}
	}
	return false
}

// NativeTitle returns the language's own name, shown in the language menu.
func (l Language) NativeTitle() string {
	switch l {
	case SimplifiedChinese:
		return "简体中文"
	case TraditionalChinese:
		return "繁體中文"
	case Korean:
		return "한국어"
	case Japanese:
		return "日本語"
	case Russian:
		return "Русский"
	case French:
		return "Français"
	default:
		return "English"
	}
}

// MenuTitle is the title of the menu entry for a preference. "system" follows
// the current UI language ("Auto"); explicit choices stay in their own script.
func MenuTitle(preference string, ui Language) string {
	if preference == PreferenceSystem {
		return New(ui).Text(KeyAuto)
	}
	return Language(preference).NativeTitle()
}

// LocaleIdentifier is the BCP-47-ish locale used for date/number formatting.
func (l Language) LocaleIdentifier() string {
	switch l {
	case SimplifiedChinese:
		return "zh_Hans_CN"
	case TraditionalChinese:
		return "zh_Hant_TW"
	case Korean:
		return "ko_KR"
	case Japanese:
		return "ja_JP"
	case Russian:
		return "ru_RU"
	case French:
		return "fr_FR"
	default:
		return "en_US"
	}
}

// PosixLocaleIdentifier is LocaleIdentifier, except English uses en_US_POSIX.
func (l Language) PosixLocaleIdentifier() string {
	if l == English || !l.Valid() {
		return "en_US_POSIX"
	}
	return l.LocaleIdentifier()
}

// UsesFullwidthPunctuation reports whether the language uses fullwidth
// colons and parentheses.
func (l Language) UsesFullwidthPunctuation() bool {
	return l == SimplifiedChinese || l == TraditionalChinese || l == Japanese
}

// ColonSeparator separates a label from its value.
func (l Language) ColonSeparator() string {
	if l.UsesFullwidthPunctuation() {
		return "："
	}
	return ": "
}

// OpenParen opens a parenthetical, including the leading space when needed.
func (l Language) OpenParen() string {
	if l.UsesFullwidthPunctuation() {
		return "（"
	}
	return " ("
}

// CloseParen closes a parenthetical.
func (l Language) CloseParen() string {
	if l.UsesFullwidthPunctuation() {
		return "）"
	}
	return ")"
}

// UsesCJKDatePattern reports whether dates are written year-first with CJK
// unit characters.
func (l Language) UsesCJKDatePattern() bool {
	switch l {
	case SimplifiedChinese, TraditionalChinese, Japanese, Korean:
		return true
	}
	return false
}

// CompactNumberScale selects how large numbers are abbreviated.
type CompactNumberScale int

// Compact number scales, mirroring the Swift enum.
const (
	ScaleLatin CompactNumberScale = iota
	ScaleWanYiSimplified
	ScaleWanYiTraditional
	ScaleWanYiJapanese
	ScaleManEok
)

// CompactNumberScale returns the abbreviation scale for the language.
func (l Language) CompactNumberScale() CompactNumberScale {
	switch l {
	case SimplifiedChinese:
		return ScaleWanYiSimplified
	case TraditionalChinese:
		return ScaleWanYiTraditional
	case Japanese:
		return ScaleWanYiJapanese
	case Korean:
		return ScaleManEok
	default:
		return ScaleLatin
	}
}

// Resolve turns a stored language preference ("system" or a Language value)
// into a Language. "system" and any unrecognised value pick the first match
// in preferredLanguages, falling back to English. preferredLanguages is
// normally SystemPreferredLanguages().
func Resolve(preference string, preferredLanguages []string) Language {
	if l := Language(preference); l.Valid() {
		return l
	}
	return ResolveSystem(preferredLanguages)
}

// ResolveSystem returns the first supported language among the OS preferred
// languages, or English when none matches.
func ResolveSystem(preferredLanguages []string) Language {
	for _, id := range preferredLanguages {
		if l, ok := MapLocaleIdentifier(id); ok {
			return l
		}
	}
	return English
}

// MapLocaleIdentifier maps a locale identifier such as "zh-Hant-TW",
// "zh_HK" or "ko-KR" to a supported language.
func MapLocaleIdentifier(identifier string) (Language, bool) {
	normalized := strings.ToLower(strings.ReplaceAll(identifier, "_", "-"))
	// Swift's split(separator:) omits empty subsequences.
	parts := strings.FieldsFunc(normalized, func(r rune) bool { return r == '-' })
	if len(parts) == 0 {
		return "", false
	}
	switch parts[0] {
	case "zh":
		for _, p := range parts {
			if p == "hant" {
				return TraditionalChinese, true
			}
		}
		for _, p := range parts {
			if p == "hans" {
				return SimplifiedChinese, true
			}
		}
		for _, p := range parts {
			if p == "tw" || p == "hk" || p == "mo" {
				return TraditionalChinese, true
			}
		}
		return SimplifiedChinese, true
	case "ko":
		return Korean, true
	case "ja":
		return Japanese, true
	case "ru":
		return Russian, true
	case "fr":
		return French, true
	case "en":
		return English, true
	}
	return "", false
}
