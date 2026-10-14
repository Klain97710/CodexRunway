package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func mustDecode(t *testing.T, doc string) Preferences {
	t.Helper()
	p, err := DecodePreferences([]byte(doc))
	if err != nil {
		t.Fatalf("DecodePreferences(%s): %v", doc, err)
	}
	return p
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.TrimSpace(b)
}

// sameJSON compares two JSON documents by value, ignoring key order and spacing.
func sameJSON(t *testing.T, got, want []byte) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("got is not JSON: %v\n%s", err, got)
	}
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatalf("want is not JSON: %v\n%s", err, want)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("JSON differs:\n got %s\nwant %s", got, want)
	}
}

// allNonDefault returns the same settings the Swift fixture swift_all.json was
// produced from: every one of the 34 fields differs from its default.
func allNonDefault() Preferences {
	return Preferences{
		SelectedProvider:                          ProviderGrok,
		Language:                                  LanguageJapanese,
		Appearance:                                AppearanceDark,
		MainPanelBackgroundStyle:                  PanelBackgroundOpaque,
		StatusBarDisplayStyle:                     StatusBarStyleRings,
		StatusBarMetersDetailStyle:                MetersDetailBoth,
		StatusBarBatteryScope:                     BatteryScopeBoth,
		StatusBarBatteryDetailStyle:               BatteryDetailRemainingPercent,
		StatusBarProviderScope:                    ProviderScopeBoth,
		RefreshIntervalSeconds:                    900,
		WidgetRefreshIntervalSeconds:              600,
		APICostSummaryRange:                       CostRangeThisMonth,
		MainPanelHeight:                           700.5,
		MainPanelModuleOrder:                      []Module{ModuleSessionRepair, ModuleRecentSessions, ModuleQuota, ModuleTokenUsage, ModuleRateLimitResetToday, ModuleQuotaEstimate, ModuleResetCredits, ModuleAPICost},
		ShowsQuotaSummary:                         false,
		ShowsResetCreditsSummary:                  false,
		ShowsQuotaEstimateSummary:                 false,
		QuotaEstimateWindowMode:                   WindowModeRollingWeek,
		ShowsCostSummary:                          false,
		ShowsRecentSessions:                       true,
		ShowsSessionRepairSummary:                 false,
		ShowsRateLimitResetToday:                  false,
		ShowsModelSpecificQuotaUsage:              true,
		ShowsTokenUsageHeatmap:                    false,
		TokenUsageChartStyle:                      ChartStyleBar,
		RateLimitResetTodayRefreshIntervalSeconds: 3600,
		AutomaticallyChecksForUpdates:             false,
		LaunchAtLoginEnabled:                      false,
		LaunchAtLoginInitialized:                  true,
		QuotaAlertsEnabled:                        true,
		ResetCreditAlertsEnabled:                  true,
		RateLimitResetTodayAlertsEnabled:          false,
		ExportsStatusJSON:                         true,
		AccountPrivacyModeEnabled:                 true,
	}
}

func TestThirtyFourFieldsAllDifferFromDefaults(t *testing.T) {
	d, a := reflect.ValueOf(DefaultPreferences()), reflect.ValueOf(allNonDefault())
	typ := d.Type()
	count := 0
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).Tag.Get("json") == "" {
			continue // the unexported extra map
		}
		count++
		if reflect.DeepEqual(d.Field(i).Interface(), a.Field(i).Interface()) {
			t.Errorf("field %s is the same in allNonDefault and the defaults", typ.Field(i).Name)
		}
	}
	if count != 34 {
		t.Errorf("Preferences has %d stored fields, want 34", count)
	}
}

func TestDefaultsMatchSwift(t *testing.T) {
	d := DefaultPreferences()
	if got := mustDecode(t, `{}`); !reflect.DeepEqual(got, d) {
		t.Errorf("decoding {} = %+v, want defaults", got)
	}
	enc, err := EncodePreferences(d)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, enc, fixture(t, "swift_default.json"))
	if !strings.Contains(string(enc), `"mainPanelHeight":584,`) {
		t.Errorf("mainPanelHeight should encode as a plain number: %s", enc)
	}
}

func TestSwiftProducedFixtures(t *testing.T) {
	if got := mustDecode(t, string(fixture(t, "swift_default.json"))); !reflect.DeepEqual(got, DefaultPreferences()) {
		t.Errorf("swift_default.json decoded to %+v", got)
	}
	got := mustDecode(t, string(fixture(t, "swift_all.json")))
	if want := allNonDefault(); !reflect.DeepEqual(got, want) {
		t.Errorf("swift_all.json decoded to\n%+v\nwant\n%+v", got, want)
	}
	enc, err := EncodePreferences(allNonDefault())
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, enc, fixture(t, "swift_all.json"))
	if !strings.Contains(string(enc), `"mainPanelHeight":700.5,`) {
		t.Errorf("fractional height lost: %s", enc)
	}
}

func TestOldPreferencesMissingKeysUseDefaults(t *testing.T) {
	// The document from the Swift test oldPreferencesDefaultProvider.
	p := mustDecode(t, `{"language":"english","appearance":"dark","refreshIntervalSeconds":300}`)
	want := DefaultPreferences()
	want.Language, want.Appearance = LanguageEnglish, AppearanceDark
	if !reflect.DeepEqual(p, want) {
		t.Errorf("got %+v, want %+v", p, want)
	}
}

func TestBoolMissingVersusExplicit(t *testing.T) {
	bools := map[string]func(*Preferences) *bool{
		"showsQuotaSummary":                func(p *Preferences) *bool { return &p.ShowsQuotaSummary },
		"showsResetCreditsSummary":         func(p *Preferences) *bool { return &p.ShowsResetCreditsSummary },
		"showsQuotaEstimateSummary":        func(p *Preferences) *bool { return &p.ShowsQuotaEstimateSummary },
		"showsCostSummary":                 func(p *Preferences) *bool { return &p.ShowsCostSummary },
		"showsRecentSessions":              func(p *Preferences) *bool { return &p.ShowsRecentSessions },
		"showsSessionRepairSummary":        func(p *Preferences) *bool { return &p.ShowsSessionRepairSummary },
		"showsRateLimitResetToday":         func(p *Preferences) *bool { return &p.ShowsRateLimitResetToday },
		"showsModelSpecificQuotaUsage":     func(p *Preferences) *bool { return &p.ShowsModelSpecificQuotaUsage },
		"showsTokenUsageHeatmap":           func(p *Preferences) *bool { return &p.ShowsTokenUsageHeatmap },
		"automaticallyChecksForUpdates":    func(p *Preferences) *bool { return &p.AutomaticallyChecksForUpdates },
		"launchAtLoginEnabled":             func(p *Preferences) *bool { return &p.LaunchAtLoginEnabled },
		"launchAtLoginInitialized":         func(p *Preferences) *bool { return &p.LaunchAtLoginInitialized },
		"quotaAlertsEnabled":               func(p *Preferences) *bool { return &p.QuotaAlertsEnabled },
		"resetCreditAlertsEnabled":         func(p *Preferences) *bool { return &p.ResetCreditAlertsEnabled },
		"rateLimitResetTodayAlertsEnabled": func(p *Preferences) *bool { return &p.RateLimitResetTodayAlertsEnabled },
		"exportsStatusJSON":                func(p *Preferences) *bool { return &p.ExportsStatusJSON },
		"accountPrivacyModeEnabled":        func(p *Preferences) *bool { return &p.AccountPrivacyModeEnabled },
	}
	if len(bools) != 17 {
		t.Fatalf("expected 17 bool fields, have %d", len(bools))
	}
	defaults := DefaultPreferences()
	for key, field := range bools {
		def := *field(&defaults)
		for _, tc := range []struct {
			name, doc string
			want      bool
		}{
			{"missing", `{}`, def},
			{"null", `{"` + key + `":null}`, def},
			{"explicit true", `{"` + key + `":true}`, true},
			{"explicit false", `{"` + key + `":false}`, false},
		} {
			p := mustDecode(t, tc.doc)
			if got := *field(&p); got != tc.want {
				t.Errorf("%s %s: got %v, want %v", key, tc.name, got, tc.want)
			}
		}
		for _, bad := range []string{`0`, `1`, `"true"`, `[]`, `{}`} {
			if _, err := DecodePreferences([]byte(`{"` + key + `":` + bad + `}`)); !errors.Is(err, ErrCorrupt) {
				t.Errorf("%s=%s: err = %v, want ErrCorrupt", key, bad, err)
			}
		}
	}
}

func checkEnum[T ~string](t *testing.T, key string, values []T, valid func(T) bool, get func(Preferences) T) {
	t.Helper()
	for _, v := range values {
		if !valid(v) {
			t.Errorf("%s: %q should be valid", key, v)
		}
		p := mustDecode(t, `{"`+key+`":"`+string(v)+`"}`)
		if got := get(p); got != v {
			t.Errorf("%s: decoded %q, want %q", key, got, v)
		}
	}
	for _, bad := range []string{`"klingon"`, `""`, `"Codex"`, `1`, `true`, `[]`} {
		if _, err := DecodePreferences([]byte(`{"` + key + `":` + bad + `}`)); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s=%s: err = %v, want ErrCorrupt", key, bad, err)
		}
	}
	if valid(T("klingon")) {
		t.Errorf("%s: klingon must not be valid", key)
	}
	if p := mustDecode(t, `{"`+key+`":null}`); get(p) != get(DefaultPreferences()) {
		t.Errorf("%s: null should give the default", key)
	}
}

func TestEveryEnumValue(t *testing.T) {
	checkEnum(t, "selectedProvider", Providers(), Provider.Valid, func(p Preferences) Provider { return p.SelectedProvider })
	checkEnum(t, "language", Languages(), Language.Valid, func(p Preferences) Language { return p.Language })
	checkEnum(t, "appearance", Appearances(), Appearance.Valid, func(p Preferences) Appearance { return p.Appearance })
	checkEnum(t, "mainPanelBackgroundStyle", PanelBackgrounds(), PanelBackground.Valid, func(p Preferences) PanelBackground { return p.MainPanelBackgroundStyle })
	checkEnum(t, "statusBarDisplayStyle", StatusBarStyles(), StatusBarStyle.Valid, func(p Preferences) StatusBarStyle { return p.StatusBarDisplayStyle })
	checkEnum(t, "statusBarMetersDetailStyle", MetersDetails(), MetersDetail.Valid, func(p Preferences) MetersDetail { return p.StatusBarMetersDetailStyle })
	checkEnum(t, "statusBarBatteryScope", BatteryScopes(), BatteryScope.Valid, func(p Preferences) BatteryScope { return p.StatusBarBatteryScope })
	checkEnum(t, "statusBarBatteryDetailStyle", BatteryDetails(), BatteryDetail.Valid, func(p Preferences) BatteryDetail { return p.StatusBarBatteryDetailStyle })
	checkEnum(t, "statusBarProviderScope", StatusBarProviderScopes(), StatusBarProviderScope.Valid, func(p Preferences) StatusBarProviderScope { return p.StatusBarProviderScope })
	checkEnum(t, "apiCostSummaryRange", CostRanges(), CostRange.Valid, func(p Preferences) CostRange { return p.APICostSummaryRange })
	checkEnum(t, "tokenUsageChartStyle", ChartStyles(), ChartStyle.Valid, func(p Preferences) ChartStyle { return p.TokenUsageChartStyle })
}

func TestEnumRawValuesMatchSwift(t *testing.T) {
	strs := func(vs ...string) []string { return vs }
	conv := func(vs any) []string {
		rv := reflect.ValueOf(vs)
		out := make([]string, rv.Len())
		for i := range out {
			out[i] = rv.Index(i).String()
		}
		return out
	}
	for name, tc := range map[string]struct {
		got  []string
		want []string
	}{
		"provider":      {conv(Providers()), strs("codex", "grok")},
		"language":      {conv(Languages()), strs("system", "english", "simplifiedChinese", "traditionalChinese", "korean", "japanese", "russian", "french")},
		"appearance":    {conv(Appearances()), strs("system", "light", "dark")},
		"background":    {conv(PanelBackgrounds()), strs("translucent", "opaque")},
		"statusBar":     {conv(StatusBarStyles()), strs("meters", "countdown", "battery", "rings", "text")},
		"metersDetail":  {conv(MetersDetails()), strs("remainingPercent", "resetTime", "both")},
		"batteryScope":  {conv(BatteryScopes()), strs("fiveHour", "weekly", "both")},
		"batteryDetail": {conv(BatteryDetails()), strs("countdown", "remainingPercent")},
		"providerScope": {conv(StatusBarProviderScopes()), strs("selected", "both")},
		"costRange":     {conv(CostRanges()), strs("today", "current", "previous", "thisMonth")},
		"chart":         {conv(ChartStyles()), strs("heatmap", "line", "bar")},
		"modules":       {conv(Modules()), strs("quota", "tokenUsage", "rateLimitResetToday", "quotaEstimate", "resetCredits", "apiCost", "sessionRepair", "recentSessions")},
		"windowMode":    {conv(WindowModes()), strs("auto", "rollingWeek")},
		"proxyMode":     {conv(ProxyModes()), strs("system", "http", "socks5")},
	} {
		if !slices.Equal(tc.got, tc.want) {
			t.Errorf("%s: %v, want %v", name, tc.got, tc.want)
		}
	}
}

func TestQuotaEstimateWindowModeIsTolerant(t *testing.T) {
	for doc, want := range map[string]WindowMode{
		`{"quotaEstimateWindowMode":"officialWeek"}`: WindowModeAuto, // legacy value
		`{"quotaEstimateWindowMode":"auto"}`:         WindowModeAuto,
		`{"quotaEstimateWindowMode":"rollingWeek"}`:  WindowModeRollingWeek,
		`{"quotaEstimateWindowMode":"future"}`:       WindowModeAuto,
		`{"quotaEstimateWindowMode":7}`:              WindowModeAuto,
		`{"quotaEstimateWindowMode":{"a":1}}`:        WindowModeAuto,
		`{"quotaEstimateWindowMode":null}`:           WindowModeAuto,
		`{}`:                                         WindowModeAuto,
	} {
		if got := mustDecode(t, doc).QuotaEstimateWindowMode; got != want {
			t.Errorf("%s: got %q, want %q", doc, got, want)
		}
	}
	// A re-encode writes the mapped value, never "officialWeek".
	enc, err := EncodePreferences(mustDecode(t, `{"quotaEstimateWindowMode":"officialWeek"}`))
	if err != nil || !strings.Contains(string(enc), `"quotaEstimateWindowMode":"auto"`) {
		t.Errorf("re-encode = %s, %v", enc, err)
	}
}

func TestIntFieldsDecodeLikeSwift(t *testing.T) {
	for doc, want := range map[string]int{
		`{"refreshIntervalSeconds":900}`:    900,
		`{"refreshIntervalSeconds":300.0}`:  300,
		`{"refreshIntervalSeconds":3e2}`:    300,
		`{"refreshIntervalSeconds":-5}`:     -5, // not clamped, as in Swift
		`{"refreshIntervalSeconds":0}`:      0,
		`{"refreshIntervalSeconds":null}`:   300,
		`{}`:                                300,
		`{"refreshIntervalSeconds":1.0e+3}`: 1000,
	} {
		if got := mustDecode(t, doc).RefreshIntervalSeconds; got != want {
			t.Errorf("%s: got %d, want %d", doc, got, want)
		}
	}
	for _, bad := range []string{`300.5`, `"300"`, `true`, `[]`, `1e999`, `9223372036854775808`, `1e19`} {
		if _, err := DecodePreferences([]byte(`{"refreshIntervalSeconds":` + bad + `}`)); !errors.Is(err, ErrCorrupt) {
			t.Errorf("refreshIntervalSeconds=%s: err = %v, want ErrCorrupt", bad, err)
		}
	}
}

func TestClamping(t *testing.T) {
	for doc, want := range map[string]float64{
		`{"mainPanelHeight":100}`:   360,
		`{"mainPanelHeight":359}`:   360,
		`{"mainPanelHeight":360}`:   360,
		`{"mainPanelHeight":612.5}`: 612.5,
		`{"mainPanelHeight":900}`:   900,
		`{"mainPanelHeight":1200}`:  900, // Swift test mainPanelHeightBounds
		`{"mainPanelHeight":-1}`:    360,
		`{}`:                        584,
	} {
		if got := mustDecode(t, doc).MainPanelHeight; got != want {
			t.Errorf("%s: got %v, want %v", doc, got, want)
		}
	}
	for _, bad := range []string{`"700"`, `true`, `[]`, `1e999`} {
		if _, err := DecodePreferences([]byte(`{"mainPanelHeight":` + bad + `}`)); !errors.Is(err, ErrCorrupt) {
			t.Errorf("mainPanelHeight=%s: err = %v, want ErrCorrupt", bad, err)
		}
	}
	for in, want := range map[int]int{-5: 60, 0: 60, 59: 60, 60: 60, 120: 120, 1800: 1800, 1801: 1800, 99999: 1800} {
		if got := ClampWidgetRefreshInterval(in); got != want {
			t.Errorf("widget clamp(%d) = %d, want %d", in, got, want)
		}
		if got := mustDecode(t, jsonInt("widgetRefreshIntervalSeconds", in)).WidgetRefreshIntervalSeconds; got != want {
			t.Errorf("decoded widget(%d) = %d, want %d", in, got, want)
		}
	}
	for in, want := range map[int]int{1: 300, 60: 300, 300: 300, 600: 600, 50_000: 21_600, 21_600: 21_600} {
		if got := ClampRateLimitResetTodayRefreshInterval(in); got != want {
			t.Errorf("reset-today clamp(%d) = %d, want %d", in, got, want)
		}
		if got := mustDecode(t, jsonInt("rateLimitResetTodayRefreshIntervalSeconds", in)).RateLimitResetTodayRefreshIntervalSeconds; got != want {
			t.Errorf("decoded reset-today(%d) = %d, want %d", in, got, want)
		}
	}
	if got := ClampMainPanelHeight(math.Inf(1)); got != DefaultMainPanelHeight {
		t.Errorf("infinite height = %v, want default", got)
	}
	if got := ClampMainPanelHeight(math.NaN()); got != DefaultMainPanelHeight {
		t.Errorf("NaN height = %v, want default", got)
	}
}

func TestRefreshOptions(t *testing.T) {
	if got, want := WidgetRefreshIntervalOptions(), []int{60, 300, 600, 900, 1800}; !slices.Equal(got, want) {
		t.Errorf("widget options = %v", got)
	}
	if got, want := RateLimitResetTodayRefreshIntervalOptions(), []int{300, 600, 900, 1800, 3600, 7200, 21600}; !slices.Equal(got, want) {
		t.Errorf("reset-today options = %v", got)
	}
	for _, o := range WidgetRefreshIntervalOptions() {
		if ClampWidgetRefreshInterval(o) != o {
			t.Errorf("widget option %d is not stable under clamping", o)
		}
	}
	for _, o := range RateLimitResetTodayRefreshIntervalOptions() {
		if ClampRateLimitResetTodayRefreshInterval(o) != o {
			t.Errorf("reset-today option %d is not stable under clamping", o)
		}
	}
}

func TestModuleOrderNormalization(t *testing.T) {
	d := Modules()
	for _, tc := range []struct {
		name string
		doc  string
		want []Module
	}{
		{"missing key", `{}`, d},
		{"empty array", `{"mainPanelModuleOrder":[]}`, d},
		{"null", `{"mainPanelModuleOrder":null}`, d},
		{"Swift test: duplicates, unknown, missing", `{"mainPanelModuleOrder":["apiCost","unknown-future-module","quota","apiCost"]}`,
			[]Module{ModuleTokenUsage, ModuleRateLimitResetToday, ModuleQuotaEstimate, ModuleResetCredits, ModuleAPICost, ModuleQuota, ModuleSessionRepair, ModuleRecentSessions}},
		{"user order kept, missing appended before neighbours", `{"mainPanelModuleOrder":["recentSessions","apiCost"]}`,
			[]Module{ModuleSessionRepair, ModuleRecentSessions, ModuleQuota, ModuleTokenUsage, ModuleRateLimitResetToday, ModuleQuotaEstimate, ModuleResetCredits, ModuleAPICost}},
		{"reversed is kept", `{"mainPanelModuleOrder":["recentSessions","sessionRepair","apiCost","resetCredits","quotaEstimate","rateLimitResetToday","tokenUsage","quota"]}`,
			[]Module{ModuleRecentSessions, ModuleSessionRepair, ModuleAPICost, ModuleResetCredits, ModuleQuotaEstimate, ModuleRateLimitResetToday, ModuleTokenUsage, ModuleQuota}},
		{"new module inserted before its default neighbour (quotaEstimate before resetCredits)",
			`{"mainPanelModuleOrder":["quota","tokenUsage","rateLimitResetToday","resetCredits","apiCost","sessionRepair","recentSessions"]}`, d},
		{"all unknown", `{"mainPanelModuleOrder":["a","b"]}`, d},
		{"case sensitive", `{"mainPanelModuleOrder":["Quota"]}`, d},
	} {
		if got := mustDecode(t, tc.doc).MainPanelModuleOrder; !slices.Equal(got, tc.want) {
			t.Errorf("%s:\n got %v\nwant %v", tc.name, got, tc.want)
		}
	}
	for _, bad := range []string{`"quota"`, `{}`, `[1]`, `["quota",null]`, `[["quota"]]`, `true`} {
		if _, err := DecodePreferences([]byte(`{"mainPanelModuleOrder":` + bad + `}`)); !errors.Is(err, ErrCorrupt) {
			t.Errorf("mainPanelModuleOrder=%s: err = %v, want ErrCorrupt", bad, err)
		}
	}
}

func TestNormalizeModuleOrderProperties(t *testing.T) {
	inputs := [][]Module{
		nil, {}, {ModuleAPICost}, {ModuleAPICost, ModuleAPICost}, {"x", ModuleQuota, "y"},
		{ModuleRecentSessions, ModuleQuota, ModuleSessionRepair},
	}
	for _, in := range inputs {
		got := NormalizeModuleOrder(in)
		if len(got) != len(Modules()) {
			t.Errorf("%v -> %v: want every module exactly once", in, got)
		}
		seen := map[Module]bool{}
		for _, m := range got {
			if seen[m] || !m.Valid() {
				t.Errorf("%v -> %v: duplicate or unknown module %q", in, got, m)
			}
			seen[m] = true
		}
		if again := NormalizeModuleOrder(got); !slices.Equal(again, got) {
			t.Errorf("not idempotent: %v -> %v", got, again)
		}
	}
	in := []Module{ModuleAPICost, ModuleQuota}
	NormalizeModuleOrder(in)
	if !slices.Equal(in, []Module{ModuleAPICost, ModuleQuota}) {
		t.Errorf("input was modified: %v", in)
	}
}

func TestMoveAndResetModuleOrder(t *testing.T) {
	p := DefaultPreferences()
	p.MoveMainPanelModule(ModuleAPICost, -1)
	want := []Module{ModuleQuota, ModuleTokenUsage, ModuleRateLimitResetToday, ModuleQuotaEstimate, ModuleAPICost, ModuleResetCredits, ModuleSessionRepair, ModuleRecentSessions}
	if !slices.Equal(p.MainPanelModuleOrder, want) {
		t.Errorf("after move: %v", p.MainPanelModuleOrder)
	}
	p.MoveMainPanelModule(ModuleQuota, -1) // already first
	if p.MainPanelModuleOrder[0] != ModuleQuota {
		t.Errorf("moving the first module up changed the order: %v", p.MainPanelModuleOrder)
	}
	p.MoveMainPanelModule(ModuleRecentSessions, 1) // already last
	p.ResetMainPanelModuleOrder()
	if !slices.Equal(p.MainPanelModuleOrder, Modules()) {
		t.Errorf("reset: %v", p.MainPanelModuleOrder)
	}
}

func TestCorruptDocumentsAreErrors(t *testing.T) {
	for _, doc := range []string{
		``, `   `, `{`, `not json`, `null`, `[]`, `"x"`, `5`, `true`, `{"a":1}}`, `{"language":"klingon"}`,
		`{"language":1}`, `{"showsQuotaSummary":"yes"}`, `{"refreshIntervalSeconds":1.5}`,
		`{"mainPanelModuleOrder":"quota"}`, "\xff\xfe",
	} {
		p, err := DecodePreferences([]byte(doc))
		if !errors.Is(err, ErrCorrupt) {
			t.Errorf("%q: err = %v, want ErrCorrupt", doc, err)
		}
		if err != nil && !reflect.DeepEqual(p, Preferences{}) {
			t.Errorf("%q: an error must come with the zero value, not defaults", doc)
		}
	}
	if _, err := DecodePreferences([]byte(`{"language":"klingon"}`)); err == nil || !strings.Contains(err.Error(), "language") {
		t.Errorf("error should name the key: %v", err)
	}
}

func TestKeysAreCaseSensitive(t *testing.T) {
	// Go's struct decoder would match "Language"; Swift ignores it as unknown.
	p := mustDecode(t, `{"Language":"english"}`)
	if p.Language != LanguageSystem {
		t.Errorf("Language matched case-insensitively: %q", p.Language)
	}
	if _, kept := p.extra["Language"]; !kept {
		t.Error("the mis-cased key should be kept as an unknown key")
	}
}

func TestUnknownKeysRoundTrip(t *testing.T) {
	doc := `{"language":"english","futureFlag":true,"futureObject":{"a":[1,2,{"b":null}],"s":"<x>&"},"futureNumber":1.50,"futureString":"é"}`
	p := mustDecode(t, doc)
	p.Appearance = AppearanceDark
	enc, err := EncodePreferences(p)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(enc, &got); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"futureFlag":   `true`,
		"futureObject": `{"a":[1,2,{"b":null}],"s":"<x>&"}`,
		"futureNumber": `1.50`,
		"futureString": `"é"`,
		"language":     `"english"`,
		"appearance":   `"dark"`,
	} {
		if string(got[key]) != want {
			t.Errorf("%s = %s, want %s", key, got[key], want)
		}
	}
	// Decoding the output again gives the same value.
	again := mustDecode(t, string(enc))
	if !reflect.DeepEqual(again, p) {
		t.Errorf("round trip changed the value:\n%+v\n%+v", again, p)
	}
	// An unknown key never overrides a known one.
	q := mustDecode(t, `{"language":"english"}`)
	q.extra = map[string]json.RawMessage{"language": json.RawMessage(`"french"`)}
	enc, _ = EncodePreferences(q)
	if !strings.Contains(string(enc), `"language":"english"`) {
		t.Errorf("extra key overrode a known key: %s", enc)
	}
}

func TestCloneIsDeep(t *testing.T) {
	p := mustDecode(t, `{"x":1}`)
	c := p.Clone()
	c.MainPanelModuleOrder[0] = ModuleRecentSessions
	c.extra["y"] = json.RawMessage(`2`)
	if p.MainPanelModuleOrder[0] != ModuleQuota || len(p.extra) != 1 {
		t.Errorf("Clone shares memory with the original: %+v", p)
	}
}

func TestEncodeNormalizesAndValidates(t *testing.T) {
	p := DefaultPreferences()
	p.MainPanelHeight = 5000
	p.WidgetRefreshIntervalSeconds = 1
	p.RateLimitResetTodayRefreshIntervalSeconds = 1
	p.MainPanelModuleOrder = []Module{ModuleAPICost}
	enc, err := EncodePreferences(p)
	if err != nil {
		t.Fatal(err)
	}
	got := mustDecode(t, string(enc))
	if got.MainPanelHeight != 900 || got.WidgetRefreshIntervalSeconds != 60 || got.RateLimitResetTodayRefreshIntervalSeconds != 300 || len(got.MainPanelModuleOrder) != 8 {
		t.Errorf("not normalized: %+v", got)
	}
	if p.MainPanelHeight != 5000 {
		t.Error("EncodePreferences modified its argument")
	}

	for name, mutate := range map[string]func(*Preferences){
		"provider":   func(p *Preferences) { p.SelectedProvider = "bogus" },
		"language":   func(p *Preferences) { p.Language = "" },
		"appearance": func(p *Preferences) { p.Appearance = "sepia" },
		"chart":      func(p *Preferences) { p.TokenUsageChartStyle = "pie" },
		"windowMode": func(p *Preferences) { p.QuotaEstimateWindowMode = "officialWeek" },
	} {
		bad := DefaultPreferences()
		mutate(&bad)
		if out, err := EncodePreferences(bad); err == nil {
			t.Errorf("%s: invalid enum encoded without error: %s", name, out)
		}
	}
	if _, err := EncodePreferences(Preferences{}); err == nil {
		t.Error("the zero Preferences has empty enums and must not encode")
	}
}

func TestEncodedFloatFormat(t *testing.T) {
	for height, want := range map[float64]string{584: `"mainPanelHeight":584,`, 700.5: `"mainPanelHeight":700.5,`, 900: `"mainPanelHeight":900,`} {
		p := DefaultPreferences()
		p.MainPanelHeight = height
		enc, err := EncodePreferences(p)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(enc), want) {
			t.Errorf("height %v: %s", height, enc)
		}
	}
}

func jsonInt(key string, v int) string {
	b, _ := json.Marshal(map[string]int{key: v})
	return string(b)
}
