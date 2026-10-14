package l10n

import "testing"

func TestMapLocaleIdentifier(t *testing.T) {
	cases := []struct {
		id   string
		want Language
		ok   bool
	}{
		{"en", English, true},
		{"en-US", English, true},
		{"EN_gb", English, true},
		{"zh", SimplifiedChinese, true},
		{"zh-CN", SimplifiedChinese, true},
		{"zh_CN", SimplifiedChinese, true},
		{"zh-Hans", SimplifiedChinese, true},
		{"zh-Hans-CN", SimplifiedChinese, true},
		{"zh-SG", SimplifiedChinese, true},
		{"zh-Hant", TraditionalChinese, true},
		{"zh-Hant-TW", TraditionalChinese, true},
		{"zh-TW", TraditionalChinese, true},
		{"ZH_tw", TraditionalChinese, true},
		{"zh-HK", TraditionalChinese, true},
		{"zh-Hant-HK", TraditionalChinese, true},
		{"zh-MO", TraditionalChinese, true},
		{"zh-Hans-HK", SimplifiedChinese, true}, // explicit script beats region
		{"zh-Hant-CN", TraditionalChinese, true},
		{"zh-HK-Hans", SimplifiedChinese, true}, // hans is checked before regions
		{"ko", Korean, true},
		{"ko-KR", Korean, true},
		{"ja-JP", Japanese, true},
		{"ru-RU", Russian, true},
		{"fr-CA", French, true},
		{"fr_FR", French, true},
		{"de-DE", "", false},
		{"es", "", false},
		{"", "", false},
		{"-", "", false},
		{"_", "", false},
		{"-zh-TW", TraditionalChinese, true}, // empty subsequences are dropped
		{"zhx", "", false},
		{"english", "", false},
	}
	for _, c := range cases {
		got, ok := MapLocaleIdentifier(c.id)
		if got != c.want || ok != c.ok {
			t.Errorf("MapLocaleIdentifier(%q) = %q, %v; want %q, %v", c.id, got, ok, c.want, c.ok)
		}
	}
}

func TestResolveSystem(t *testing.T) {
	cases := []struct {
		langs []string
		want  Language
	}{
		{nil, English},
		{[]string{}, English},
		{[]string{"de-DE", "es-ES"}, English},
		{[]string{"de-DE", "fr-FR", "ja-JP"}, French},
		{[]string{"zh-Hans-CN", "en-US"}, SimplifiedChinese},
		{[]string{"zh-Hant-TW", "en-US"}, TraditionalChinese},
		{[]string{"", "ko-KR"}, Korean},
		{[]string{"ru", "en"}, Russian},
	}
	for _, c := range cases {
		if got := ResolveSystem(c.langs); got != c.want {
			t.Errorf("ResolveSystem(%v) = %q, want %q", c.langs, got, c.want)
		}
	}
}

func TestResolvePreference(t *testing.T) {
	system := []string{"ja-JP"}
	for _, l := range Languages() {
		if got := Resolve(string(l), system); got != l {
			t.Errorf("explicit %q resolved to %q", l, got)
		}
	}
	if got := Resolve(PreferenceSystem, system); got != Japanese {
		t.Errorf("system -> %q, want japanese", got)
	}
	if got := Resolve(PreferenceSystem, nil); got != English {
		t.Errorf("system without languages -> %q, want english", got)
	}
	if got := Resolve("garbage", system); got != Japanese {
		t.Errorf("unknown preference -> %q, want system resolution", got)
	}
	if got := ForPreference("french", system).Language(); got != French {
		t.Errorf("ForPreference = %q", got)
	}
}

func TestPreferenceRawValues(t *testing.T) {
	want := []string{"system", "english", "simplifiedChinese", "traditionalChinese", "korean", "japanese", "russian", "french"}
	got := Preferences()
	if len(got) != len(want) {
		t.Fatalf("Preferences() = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Preferences()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLanguageProperties(t *testing.T) {
	type props struct {
		locale, posix, title, colon, open, close string
		cjkDate                                  bool
		scale                                    CompactNumberScale
	}
	cases := map[Language]props{
		English:            {"en_US", "en_US_POSIX", "English", ": ", " (", ")", false, ScaleLatin},
		SimplifiedChinese:  {"zh_Hans_CN", "zh_Hans_CN", "简体中文", "：", "（", "）", true, ScaleWanYiSimplified},
		TraditionalChinese: {"zh_Hant_TW", "zh_Hant_TW", "繁體中文", "：", "（", "）", true, ScaleWanYiTraditional},
		Korean:             {"ko_KR", "ko_KR", "한국어", ": ", " (", ")", true, ScaleManEok},
		Japanese:           {"ja_JP", "ja_JP", "日本語", "：", "（", "）", true, ScaleWanYiJapanese},
		Russian:            {"ru_RU", "ru_RU", "Русский", ": ", " (", ")", false, ScaleLatin},
		French:             {"fr_FR", "fr_FR", "Français", ": ", " (", ")", false, ScaleLatin},
	}
	for l, w := range cases {
		got := props{l.LocaleIdentifier(), l.PosixLocaleIdentifier(), l.NativeTitle(), l.ColonSeparator(), l.OpenParen(), l.CloseParen(), l.UsesCJKDatePattern(), l.CompactNumberScale()}
		if got != w {
			t.Errorf("%s: %+v, want %+v", l, got, w)
		}
	}
}

func TestMenuTitle(t *testing.T) {
	if got := MenuTitle(PreferenceSystem, SimplifiedChinese); got != New(SimplifiedChinese).Text(KeyAuto) {
		t.Errorf("system menu title = %q", got)
	}
	if got := MenuTitle("korean", English); got != "한국어" {
		t.Errorf("korean menu title = %q", got)
	}
}
