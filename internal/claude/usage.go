package claude

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"
)

// WindowKind classifies a usage window. Unknown keys stay WindowOther and
// keep their original key.
type WindowKind string

const (
	WindowFiveHour      WindowKind = "fiveHour"
	WindowSevenDay      WindowKind = "sevenDay"
	WindowSevenDayModel WindowKind = "sevenDayModel"
	WindowOther         WindowKind = "other"
)

// UsageWindow is one limit. Null is set when the window value itself is null
// or its utilization is null. UsedPercent is the raw utilization number.
type UsageWindow struct {
	Key         string
	Kind        WindowKind
	Model       string
	UsedPercent *float64
	ResetsAt    *time.Time
	Null        bool
}

// RawNumber is a JSON number kept as the literal text from the response.
type RawNumber struct {
	Text string
}

// ExtraUsage is the extra_usage object. Nil pointers mean the field was null
// or absent. MonthlyLimit and UsedCredits are in the currency's minor unit
// (cents for USD); DecimalPlaces is how many digits that unit has, nil when
// the response did not say.
type ExtraUsage struct {
	IsEnabled     *bool
	MonthlyLimit  *RawNumber
	UsedCredits   *RawNumber
	Utilization   *RawNumber
	Currency      string
	DecimalPlaces *int
}

// Places is the minor-unit digits: the response value, else 0 for
// zero-decimal currencies and 2 otherwise.
func (e ExtraUsage) Places() int {
	if e.DecimalPlaces != nil && *e.DecimalPlaces >= 0 && *e.DecimalPlaces <= 4 {
		return *e.DecimalPlaces
	}
	switch strings.ToUpper(strings.TrimSpace(e.Currency)) {
	case "JPY", "KRW", "VND", "CLP", "ISK", "HUF", "TWD":
		return 0
	}
	return 2
}

// FormatAmount writes a minor-unit integer as a major-unit amount, e.g.
// 10587 with 2 places is "105.87". A non-integer is returned unchanged.
func (e ExtraUsage) FormatAmount(n *RawNumber) string {
	if n == nil {
		return ""
	}
	text := strings.TrimSpace(n.Text)
	places := e.Places()
	digits := strings.TrimPrefix(text, "-")
	if places == 0 || digits == "" || strings.Trim(digits, "0123456789") != "" {
		return text
	}
	sign := text[:len(text)-len(digits)]
	for len(digits) <= places {
		digits = "0" + digits
	}
	cut := len(digits) - places
	return sign + digits[:cut] + "." + digits[cut:]
}

// UsageSnapshot is a parsed usage response. Windows prefer the limits field
// when it is present (including an empty array). Flat keys are the fallback.
type UsageSnapshot struct {
	Windows   []UsageWindow
	Extra     *ExtraUsage
	FetchedAt time.Time
}

// ParseUsageSnapshot accepts the limits array (or object) and the flat
// five_hour / seven_day / seven_day_sonnet / seven_day_opus form. limits wins
// when the key is present and not null. Unknown windows keep their key.
func ParseUsageSnapshot(raw []byte) (UsageSnapshot, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var root map[string]any
	if err := dec.Decode(&root); err != nil || root == nil {
		return UsageSnapshot{}, ErrUsageParse
	}
	snap := UsageSnapshot{Extra: parseExtra(root["extra_usage"])}
	if limits, ok := root["limits"]; ok && limits != nil {
		snap.Windows = orderWindows(parseLimits(limits))
		return snap, nil
	}
	snap.Windows = orderWindows(parseFlat(root))
	return snap, nil
}

func parseLimits(v any) []UsageWindow {
	switch n := v.(type) {
	case []any:
		var out []UsageWindow
		for _, item := range n {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if w, ok := windowFromItem(obj); ok {
				out = append(out, w)
			}
		}
		return out
	case map[string]any:
		return parseFlat(n)
	default:
		return nil
	}
}

func parseFlat(root map[string]any) []UsageWindow {
	var out []UsageWindow
	for key, value := range root {
		if skipFlatKey(key) {
			continue
		}
		switch value.(type) {
		case nil:
			kind, model := classifyWindow(key)
			out = append(out, UsageWindow{Key: key, Kind: kind, Model: model, Null: true})
		case map[string]any:
			obj := value.(map[string]any)
			w := windowFromMap(key, obj)
			out = append(out, w)
		}
	}
	return out
}

func skipFlatKey(key string) bool {
	switch strings.ToLower(key) {
	case "extra_usage", "limits", "error", "type", "message":
		return true
	default:
		return false
	}
}

func windowFromItem(obj map[string]any) (UsageWindow, bool) {
	key := windowItemKey(obj)
	if key == "" {
		return UsageWindow{}, false
	}
	return windowFromMap(key, obj), true
}

func windowItemKey(obj map[string]any) string {
	for _, key := range []string{"key", "name", "id", "window", "type"} {
		s, ok := obj[key].(string)
		if ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func windowFromMap(key string, obj map[string]any) UsageWindow {
	kind, model := classifyWindow(key)
	w := UsageWindow{Key: key, Kind: kind, Model: model, ResetsAt: parseResets(obj)}
	if f, ok, isNull := utilOf(obj); isNull {
		w.Null = true
	} else if ok {
		w.UsedPercent = &f
	}
	return w
}

func utilOf(obj map[string]any) (float64, bool, bool) {
	for _, key := range []string{"utilization", "used_percent", "usedPercent"} {
		v, ok := obj[key]
		if !ok {
			continue
		}
		if v == nil {
			return 0, false, true
		}
		f, ok := numberFloat(v)
		return f, ok, false
	}
	return 0, false, false
}

func parseResets(obj map[string]any) *time.Time {
	for _, key := range []string{"resets_at", "resetsAt"} {
		v, ok := obj[key]
		if !ok || v == nil {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		if t, ok := parseTimestamp(s); ok {
			return &t
		}
	}
	return nil
}

func parseTimestamp(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func classifyWindow(key string) (WindowKind, string) {
	k := strings.ToLower(strings.TrimSpace(key))
	k = strings.ReplaceAll(k, "-", "_")
	switch k {
	case "five_hour", "5_hour", "fivehour":
		return WindowFiveHour, ""
	case "seven_day", "7_day", "sevenday":
		return WindowSevenDay, ""
	}
	if rest, ok := strings.CutPrefix(k, "seven_day_"); ok && modelFamily(rest) {
		return WindowSevenDayModel, rest
	}
	return WindowOther, ""
}

// modelFamily reports whether a seven_day_ suffix names a model family.
// Other suffixes, such as oauth_apps, are separate products, not models.
func modelFamily(name string) bool {
	switch name {
	case "opus", "sonnet", "haiku", "fable", "mythos":
		return true
	default:
		return false
	}
}

// ModelTitle is the brand title of a model family, "Opus" for "opus".
func ModelTitle(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func orderWindows(in []UsageWindow) []UsageWindow {
	if len(in) == 0 {
		return []UsageWindow{}
	}
	out := append([]UsageWindow(nil), in...)
	sortWindows(out)
	return out
}

func windowRank(w UsageWindow) int {
	switch w.Kind {
	case WindowFiveHour:
		return 0
	case WindowSevenDay:
		return 1
	case WindowSevenDayModel:
		return 2
	default:
		return 3
	}
}

func parseExtra(v any) *ExtraUsage {
	obj, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	extra := &ExtraUsage{}
	if b, ok := obj["is_enabled"].(bool); ok {
		extra.IsEnabled = &b
	}
	extra.MonthlyLimit = rawNumberPtr(obj["monthly_limit"])
	extra.UsedCredits = rawNumberPtr(obj["used_credits"])
	extra.Utilization = rawNumberPtr(obj["utilization"])
	if s, ok := obj["currency"].(string); ok {
		extra.Currency = s
	}
	if f, ok := numberFloat(obj["decimal_places"]); ok && f == float64(int(f)) {
		places := int(f)
		extra.DecimalPlaces = &places
	}
	return extra
}

func rawNumberPtr(v any) *RawNumber {
	n, ok := v.(json.Number)
	if !ok || n.String() == "" {
		return nil
	}
	return &RawNumber{Text: n.String()}
}

func numberFloat(v any) (float64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	f, err := n.Float64()
	if err != nil {
		return 0, false
	}
	return f, true
}
