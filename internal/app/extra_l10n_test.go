package app

import (
	"regexp"
	"strings"
	"testing"

	"github.com/licoy/quosparr/internal/l10n"
)

func TestExtraTextsAreCompleteInEveryLanguage(t *testing.T) {
	core := map[string]bool{}
	for _, k := range l10n.Keys() {
		core[string(k)] = true
	}
	verb := regexp.MustCompile(`%@`)
	for k, row := range extraTexts {
		if core[string(k)] {
			t.Errorf("%s collides with a key of internal/l10n", k)
		}
		want := len(verb.FindAllString(row[0], -1))
		for i, lang := range l10n.Languages() {
			if strings.TrimSpace(row[i]) == "" {
				t.Errorf("%s: %s is empty", k, lang)
			}
			if got := len(verb.FindAllString(row[i], -1)); got != want {
				t.Errorf("%s: %s has %d arguments, English has %d", k, lang, got, want)
			}
		}
	}
}

func TestExtraTextsFallBackToEnglishForUnknownKeys(t *testing.T) {
	l := l10n.ForPreference("french", nil)
	if got := tx(l, "no-such-key"); got != "no-such-key" {
		t.Errorf("tx = %q", got)
	}
	if got := tx(l, xSettingsSaveFailed, "disque plein"); !strings.Contains(got, "disque plein") || strings.Contains(got, "%@") {
		t.Errorf("tx = %q", got)
	}
}
