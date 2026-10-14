package usage

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"
)

var goldenZones = []string{
	"UTC", "Asia/Shanghai", "America/New_York", "Europe/London", "Asia/Kolkata",
	"Australia/Lord_Howe", "Pacific/Apia", "America/Sao_Paulo", "Asia/Singapore", "America/St_Johns",
}

var swiftEscape = regexp.MustCompile(`\\u\{([0-9A-Fa-f]+)\}`)

// unswift expands the \u{XXXX} placeholders the golden files use for
// characters that cannot sit in a tab-separated text file.
func unswift(s string) string {
	return swiftEscape.ReplaceAllStringFunc(s, func(m string) string {
		v, _ := strconv.ParseUint(swiftEscape.FindStringSubmatch(m)[1], 16, 32)
		return string(rune(v))
	})
}

func readLines(t *testing.T, name string) []string {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out
}

func bitsHex(f float64) string { return strconv.FormatUint(math.Float64bits(f), 16) }

func jsonEscapeForTS(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TestTimestampGolden replays timestamps parsed by the original
// implementation: the exact double and the day key for ten time zones.
func TestTimestampGolden(t *testing.T) {
	for _, file := range []string{"parser_golden.txt", "ts2.txt"} {
		for _, line := range readLines(t, file) {
			if !strings.HasPrefix(line, "TS\t") {
				continue
			}
			f := strings.Split(line, "\t")
			ts := unswift(f[1])
			for zi, cell := range f[2:] {
				loc := loadZone(t, goldenZones[zi])
				p := newRecordParser(loc)
				rec, err := p.parse([]byte(`{"timestamp":"` + jsonEscapeForTS(ts) + `"}`))
				var got string
				if err != nil {
					got = "ERR"
				} else {
					got = bitsHex(rec.Timestamp) + "," + rec.DayKey
				}
				if got != cell {
					t.Errorf("%s ts=%q zone=%s: got %s want %s", file, ts, goldenZones[zi], got, cell)
				}
			}
		}
	}
}

func fmtRecord(r logRecord, withCWDHex bool) string {
	str := func(p *string) string {
		if p == nil {
			return "nil"
		}
		return *p
	}
	cwd := "nil"
	if r.SessionCWD != nil {
		if withCWDHex {
			var parts []string
			for _, c := range *r.SessionCWD {
				parts = append(parts, strconv.FormatInt(int64(c), 16))
			}
			cwd = strings.Join(parts, ",")
		} else {
			cwd = *r.SessionCWD
		}
	}
	usage := "nil"
	if r.Usage != nil {
		usage = fmt.Sprintf("%d/%d/%d", r.Usage.InputTokens, r.Usage.CachedInputTokens, r.Usage.OutputTokens)
	}
	if withCWDHex {
		return fmt.Sprintf("OK cwd=%s model=%s ts=%s usage=%s turn=%t", cwd, str(r.Model), bitsHex(r.Timestamp), usage, r.CountsAsTurn)
	}
	return fmt.Sprintf("OK ts=%s day=%s model=%s ctx=%s cwd=%s usage=%s turn=%t",
		bitsHex(r.Timestamp), r.DayKey, str(r.Model), str(r.ContextModel), cwd, usage, r.CountsAsTurn)
}

// TestRecordParserGoldenInline replays the JS lines of parser_golden.txt.
func TestRecordParserGoldenInline(t *testing.T) {
	p := newRecordParser(time.UTC)
	n := 0
	for _, line := range readLines(t, "parser_golden.txt") {
		f := strings.Split(line, "\t")
		if f[0] != "JS" || len(f) < 4 {
			continue
		}
		n++
		in := unswift(f[2])
		rec, err := p.parse([]byte(in))
		got := "ERR"
		if err == nil {
			got = fmtRecord(rec, false)
		}
		if strings.TrimRight(got, " ") != strings.TrimRight(f[3], " ") {
			t.Errorf("case %s %q:\n got  %s\n want %s", f[1], in, got, f[3])
		}
	}
	if n < 50 {
		t.Fatalf("only %d inline cases", n)
	}
}

func jsonProbePrefix() string { return `{"timestamp":"2026-06-29T01:00:00Z",` }

func repeat(s string, n int) string { return strings.Repeat(s, n) }

func jsonProbeCases1() [][]byte {
	pre := jsonProbePrefix()
	var cases [][]byte
	S := func(s string) { cases = append(cases, []byte(s)) }
	S(pre + "\"payload\":{\"cwd\":\"a\tb\"}}")
	S(pre + "\"payload\":{\"cwd\":\"a\x01b\"}}")
	S(pre + `"payload":{"cwd":"a\xb"}}`)
	S(pre + `"payload":{"cwd":"a\u12"}}`)
	S(pre + `"payload":{"cwd":"a\u00zz"}}`)
	S(pre + `"payload":{"cwd":"a\ud800b"}}`)
	S(pre + `"payload":{"cwd":"a\udc00b"}}`)
	S(pre + `"payload":{"cwd":"a\/b\"c\\d\b\f\n\r\t"}}`)
	S(pre + `"x":-}`)
	S(pre + `"x":+1}`)
	S(pre + `"x":1e}`)
	S(pre + `"x":1e+}`)
	S(pre + `"x":1.e2}`)
	S(pre + `"x":--1}`)
	S(pre + `"x":0x10}`)
	S(pre + `"x":- 1}`)
	S(pre + `"x":tru}`)
	S(pre + `"x":True}`)
	S(pre + `"x":NULL}`)
	S(pre + `"x":nul}`)
	S(pre + `"x":1 "y":2}`)
	S(pre + `"x"}`)
	S(`{,}`)
	S(pre + `1:2}`)
	S(`{"time\u0073tamp":"2026-06-29T01:00:00Z"}`)
	S(pre + "// c\n\"x\":1}")
	S(pre + `"x":/*c*/1}`)
	S(pre + `"x":1e999}`)
	S(pre + `"x":-1e999}`)
	S(pre + `"payload":{"info":{"last_token_usage":{"input_tokens":-9223372036854775808}}}}`)
	S(pre + `"payload":{"model":"a","model":5}}`)
	S(pre + `"payload":{"model":null,"model":"a"}}`)
	S(pre + `"payload":{"model":"a"},"payload":[]}`)
	S("\u00a0" + pre + `"x":1}`)
	S("\u000c" + pre + `"x":1}`)
	S(pre + "\"x\":1}\u000b")
	S(pre + `"x":1} {}`)
	S(pre + "\"x\":1}\x00")
	S(`{"":1,"Timestamp":"zzz"}`)
	S(pre + `"x":[1,2,,3]}`)
	S(pre + `"x":[1,2,]}`)
	S(pre + `"x":[,]}`)
	S(pre + `"x":{"a":}}`)
	S(pre + `"x":"unterminated}`)
	S(pre + `"x":1`)
	S("\u2028" + pre + `"x":1}`)
	S(pre + "\"x\":\"\u2028\"}")
	S(pre + `"x":1.5e+3}`)
	S(pre + `"x":-}`)
	S(pre + `"x":.}`)
	S(pre + `"x":1.2.3}`)
	S(pre + `"x":0123}`)
	S(pre + `"x":-01}`)
	S(pre + `"payload":{"info":{"last_token_usage":{"input_tokens":01,"output_tokens":-0.0}}}}`)
	S(pre + `"payload":{"info":{"last_token_usage":{"input_tokens":1e19}}}}`)
	S(pre + `"payload":{"info":{"last_token_usage":{"input_tokens":9.223372036854775807e18}}}}`)
	S(pre + `"payload":{"info":{"last_token_usage":{"input_tokens":10.0000000000000000001}}}}`)
	S(pre + `"payload":{"info":{"last_token_usage":{"input_tokens":1.5e1}}}}`)
	S(pre + `"payload":{"info":{"last_token_usage":{"input_tokens":1e-0}}}}`)
	S(pre + `"payload":{"info":{"last_token_usage":[]}}}`)
	S(pre + `"payload":{"info":{"last_token_usage":null}}}`)
	S(pre + `"payload":{"info":5}}`)
	S(pre + `"x":` + repeat("[", 600) + repeat("]", 600) + "}")
	S(pre + `"x":` + repeat("[", 500) + repeat("]", 500) + "}")
	S(pre + `"x":` + repeat(`{"a":`, 100) + "1" + repeat("}", 100) + "}")
	S(`{"timestamp":"2026-06-29T01:00:00Z","payload":{"cwd":"` + "\u00e9\U0001F600" + `"}}`)
	cases = append(cases, append(append([]byte(pre+`"payload":{"cwd":"a`), 0xC3, 0x28), []byte(`b"}}`)...))
	cases = append(cases, append(append([]byte(pre+`"x":"`), 0xFF), []byte(`"}`)...))
	cases = append(cases, append(append([]byte(pre), 0xE2, 0x82), []byte(`"x":1}`)...))
	cases = append(cases, append(append([]byte(pre+`"x":"`), 0xED, 0xA0, 0x80), []byte(`"}`)...))
	cases = append(cases, append(append([]byte(pre+`"x":"`), 0xC0, 0x80), []byte(`"}`)...))
	cases = append(cases, append([]byte{0xEF, 0xBB, 0xBF, 0xEF, 0xBB, 0xBF}, []byte(pre+`"x":1}`)...))
	cases = append(cases, append([]byte{0xFF, 0xFE}, []byte(pre+`"x":1}`)...))
	S(pre + `"x":1,"payload":{"cwd":"/a"}}`)
	S(pre + `"type":"turn_context","payload":{"model":"m"},"turn_context":{"model":"t"}}`)
	S(pre + `"x":nulll}`)
	S(pre + `"x":true1}`)
	S(pre + `"x":1a}`)
	S(pre + `"x":"a"b}`)
	S("  \n\t\r " + pre + "\"x\":1}  \n\t\r ")
	return cases
}

func jsonProbeCases2() [][]byte {
	pre := jsonProbePrefix()
	var cases [][]byte
	S := func(s string) { cases = append(cases, []byte(s)) }
	U := func(v string) {
		S(pre + `"payload":{"info":{"last_token_usage":{"input_tokens":` + v + `}}}}`)
	}
	S(pre + "\"x\":\"a\tb\"}")
	S(pre + "\"x\":\"a\x01b\"}")
	S(pre + `"x":"a\xb"}`)
	S(pre + `"x":"a\u12"}`)
	S(pre + `"x":"a\u00zz"}`)
	S(pre + `"x":"a\ud800b"}`)
	S(pre + `"x":"a\"}`)
	S(pre + `"x":"a\`)
	S(pre + `"\x":1}`)
	S(pre + "\"a\tb\":1}")
	for _, v := range []string{"01", "-01", "-0.0", "1.", "1.0", "1e2", "-", "1e", "1.e2", "--1", "1.2.3", "0", "00", "-0", "1E2", "1e+2", "1e+", "5-", "5+3"} {
		U(v)
	}
	for _, n := range []int{509, 510, 511, 512, 513, 514} {
		S(pre + `"x":` + repeat("[", n-1) + repeat("]", n-1) + "}")
	}
	for _, n := range []int{509, 510, 511, 512, 513, 514} {
		S(pre + `"x":` + repeat(`{"a":`, n-1) + "1" + repeat("}", n-1) + "}")
	}
	S(pre + `"payload":{"cwd":"a\u0000b"}}`)
	S(pre + `"payload":{"cwd":"\ud83d\ude00"}}`)
	S(pre + `"payload":{"cwd":"\ude00\ud83d"}}`)
	S(pre + `"payload":{"cwd":"\ud83d\u0041"}}`)
	S(pre + `"payload":{"cwd":"\uD83D\uDE00"}}`)
	S(pre + `"payload":{"cwd":"\u00E9"}}`)
	S(pre + `"payload":{"cwd":"\U00E9"}}`)
	S(pre + "\"payload\":{\"cwd\":\"x\u007fy\"}}")
	S(pre + "\"payload\":{\"cwd\":\"x\u0080y\"}}")
	S(pre + `"payload":{"cwd":""}}`)
	S(pre + `"payload":{}}`)
	S(pre + `"payload":{"cwd":"a"} , }`)
	S(pre + `"x":1,}`)
	S(`{,"x":1}`)
	S(`{"x":1,,}`)
	S(`{"x":[1,]}`)
	S(`{"x":{"a":1,}}`)
	S(`{"x" :1}`)
	S(`{"x": 1 , "y" : 2 }`)
	S("{\"x\"\n:\n1\n}")
	S("{\"x\":\"a\"\u000b}")
	cases = append(cases, append([]byte(pre+`"x":1}`), 0xEF, 0xBB, 0xBF))
	return cases
}

func TestRecordParserGoldenProbes(t *testing.T) {
	for name, cases := range map[string][][]byte{
		"json_golden.txt":  jsonProbeCases1(),
		"json_golden2.txt": jsonProbeCases2(),
	} {
		lines := readLines(t, name)
		want := map[string]string{}
		for _, l := range lines {
			if l == "" {
				continue
			}
			i := strings.IndexByte(l, '\t')
			want[l[:i]] = l[i+1:]
		}
		if len(want) != len(cases) {
			t.Fatalf("%s: %d golden lines for %d cases", name, len(want), len(cases))
		}
		p := newRecordParser(time.UTC)
		for i, c := range cases {
			rec, err := p.parse(c)
			got := "ERR"
			if err == nil {
				got = fmtRecord(rec, true)
			}
			if w := want[strconv.Itoa(i)]; got != w {
				t.Errorf("%s case %d %q:\n got  %s\n want %s", name, i, trunc(c), got, w)
			}
		}
	}
}

func trunc(b []byte) string {
	if len(b) > 120 {
		return string(b[:120]) + "..."
	}
	return string(b)
}
