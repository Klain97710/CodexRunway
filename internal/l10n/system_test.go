package l10n

import (
	"reflect"
	"testing"
)

func TestLanguagesFromEnv(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{"empty", nil, nil},
		{"lang only", map[string]string{"LANG": "zh_TW.UTF-8"}, []string{"zh-TW"}},
		{"lc_all wins", map[string]string{"LC_ALL": "fr_FR.UTF-8", "LANG": "en_US.UTF-8"}, []string{"fr-FR"}},
		{"lc_messages over lang", map[string]string{"LC_MESSAGES": "ja_JP", "LANG": "en_US"}, []string{"ja-JP"}},
		{"language list first", map[string]string{"LANGUAGE": "ko:ru", "LANG": "en_US.UTF-8"}, []string{"ko", "ru", "en-US"}},
		{"modifier stripped", map[string]string{"LANG": "sr_RS@latin"}, []string{"sr-RS"}},
		{"C locale", map[string]string{"LANG": "C"}, nil},
		{"POSIX beats LANGUAGE", map[string]string{"LC_ALL": "POSIX", "LANGUAGE": "ko"}, nil},
		{"language alone", map[string]string{"LANGUAGE": "de:fr"}, []string{"de", "fr"}},
		{"C.UTF-8", map[string]string{"LANG": "C.UTF-8"}, nil},
	}
	for _, c := range cases {
		got := languagesFromEnv(func(k string) string { return c.env[k] })
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestEnvResolution(t *testing.T) {
	env := map[string]string{"LANG": "zh_HK.UTF-8"}
	langs := languagesFromEnv(func(k string) string { return env[k] })
	if got := ResolveSystem(langs); got != TraditionalChinese {
		t.Errorf("zh_HK.UTF-8 -> %q", got)
	}
}

func TestSystemPreferredLanguages(t *testing.T) {
	langs := SystemPreferredLanguages()
	for _, l := range langs {
		if l == "" {
			t.Errorf("empty identifier in %v", langs)
		}
	}
	t.Logf("system languages: %v -> %s", langs, ResolveSystem(langs))
}
