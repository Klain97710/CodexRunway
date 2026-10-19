package grok

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/speed"
)

var speedBase = time.Date(2026, 10, 12, 14, 0, 0, 0, time.UTC)

// speedLogs builds the events.jsonl and updates.jsonl of one session.
type speedLogs struct {
	events, updates strings.Builder
}

func speedStamp(sec float64) string {
	return speedBase.Add(time.Duration(sec * float64(time.Second))).Format("2006-01-02T15:04:05.000Z")
}

func (l *speedLogs) event(sec float64, typ, extra string) {
	fmt.Fprintf(&l.events, `{"ts":%q,"type":%q%s}`+"\n", speedStamp(sec), typ, extra)
}

func (l *speedLogs) turnStart(sec float64, model string) {
	l.event(sec, "turn_started", fmt.Sprintf(`,"model_id":%q`, model))
}

func (l *speedLogs) wait(sec float64) {
	l.event(sec, "phase_changed", `,"phase":"waiting_for_model"`)
}

func (l *speedLogs) firstToken(sec float64) { l.event(sec, "first_token", "") }
func (l *speedLogs) turnEnd(sec float64)    { l.event(sec, "turn_ended", `,"outcome":"completed"`) }

func (l *speedLogs) completed(sec float64, output, apiMs, calls int) {
	fmt.Fprintf(&l.updates, `{"timestamp":%d,"method":"x","params":{"sessionId":"s","update":{"sessionUpdate":"turn_completed","stop_reason":"end_turn","usage":{"inputTokens":10,"outputTokens":%d,"apiDurationMs":%d,"modelCalls":%d}}}}`+"\n",
		speedBase.Add(time.Duration(sec*float64(time.Second))).Unix(), output, apiMs, calls)
}

func (l *speedLogs) completedWithoutUsage(sec float64) {
	fmt.Fprintf(&l.updates, `{"timestamp":%d,"params":{"update":{"sessionUpdate":"turn_completed","stop_reason":"cancelled"}}}`+"\n",
		speedBase.Add(time.Duration(sec*float64(time.Second))).Unix())
}

// write stores a session under home and dates its summary.
func (l *speedLogs) write(t *testing.T, home, id string, active time.Time) {
	t.Helper()
	dir := filepath.Join(home, "sessions", "%2Ftmp%2Fdemo", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	summary := fmt.Sprintf(`{"info":{"id":%q,"cwd":"/tmp/demo"},"generated_title":"title %s","last_active_at":%q,"current_model_id":"summary-model","reasoning_effort":"xhigh"}`,
		id, id, active.UTC().Format(time.RFC3339))
	for name, content := range map[string]string{
		"summary.json": summary, "events.jsonl": l.events.String(), "updates.jsonl": l.updates.String(),
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, active, active); err != nil {
			t.Fatal(err)
		}
	}
}

func scanSpeedLogs(t *testing.T, l *speedLogs) []speed.Sample {
	t.Helper()
	home := t.TempDir()
	l.write(t, home, "s1", speedBase)
	sessions, err := SessionScanner{GrokHome: home}.ScanSpeed(context.Background(), speedBase.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions = %+v", sessions)
	}
	return sessions[0].Samples
}

func splitSamples(samples []speed.Sample) (ttfts, outputs []speed.Sample) {
	for _, s := range samples {
		if s.HasTTFT {
			ttfts = append(ttfts, s)
		}
		if s.OutputTokens > 0 {
			outputs = append(outputs, s)
		}
	}
	return ttfts, outputs
}

func TestSessionSpeedTTFTAndOutput(t *testing.T) {
	l := &speedLogs{}
	l.turnStart(10, "grok-fast")
	l.wait(10.1)
	l.firstToken(12.1) // 2s
	l.wait(20)
	l.firstToken(24) // 4s
	l.turnEnd(30.5)
	l.completed(30.2, 900, 30000, 2) // completion precedes turn_ended in the same recorded second
	l.turnStart(40, "grok-slow")
	l.wait(40.5)
	l.firstToken(41.5) // 1s
	l.turnEnd(45)
	l.completed(45.004, 100, 5000, 2) // two calls but only one observed first_token

	ttfts, outputs := splitSamples(scanSpeedLogs(t, l))
	if len(ttfts) != 3 || len(outputs) != 1 {
		t.Fatalf("ttfts = %+v, outputs = %+v", ttfts, outputs)
	}
	want := []struct {
		model string
		at    float64
		ttft  time.Duration
	}{{"grok-fast", 10.1, 2 * time.Second}, {"grok-fast", 20, 4 * time.Second}, {"grok-slow", 40.5, time.Second}}
	for i, w := range want {
		s := ttfts[i]
		if s.Model != w.model || s.Effort != "xhigh" || s.TTFT != w.ttft || !s.At.Equal(speedBase.Add(time.Duration(w.at*1000)*time.Millisecond)) {
			t.Fatalf("ttft %d = %+v, want %+v", i, s, w)
		}
	}
	if o := outputs[0]; o.Model != "grok-fast" || o.Effort != "xhigh" || o.OutputTokens != 900 || o.Decode != 24*time.Second ||
		!o.At.Equal(speedBase.Add(10*time.Second)) {
		t.Fatalf("first output = %+v", o)
	}
}

func TestSessionSpeedRetryUsesLatestWait(t *testing.T) {
	l := &speedLogs{}
	l.turnStart(0, "m")
	l.wait(1) // the request that never answered
	l.wait(5) // the retry
	l.firstToken(6)
	l.firstToken(7) // no wait pending: not a call
	l.turnEnd(8)
	l.completed(8, 10, 2000, 1)

	ttfts, _ := splitSamples(scanSpeedLogs(t, l))
	if len(ttfts) != 1 || ttfts[0].TTFT != time.Second {
		t.Fatalf("ttfts = %+v", ttfts)
	}
}

func TestSessionSpeedPairingSurvivesMissingUsageAndOpenTurn(t *testing.T) {
	l := &speedLogs{}
	l.turnStart(0, "a")
	l.wait(0.1)
	l.firstToken(1.1)
	l.turnEnd(2)
	l.completedWithoutUsage(2) // keeps the pairing, has no output
	l.turnStart(10, "b")
	l.wait(10.1)
	l.firstToken(12.1)
	l.turnEnd(20.5)
	l.completed(20.2, 60, 12000, 1) // 10s of streaming
	l.turnStart(30, "c")            // still running: no turn_ended, no turn_completed yet
	l.wait(30.1)
	l.firstToken(30.6)

	ttfts, outputs := splitSamples(scanSpeedLogs(t, l))
	if len(ttfts) != 3 {
		t.Fatalf("ttfts = %+v", ttfts)
	}
	if len(outputs) != 1 || outputs[0].Model != "b" || outputs[0].Decode != 10*time.Second {
		t.Fatalf("outputs = %+v", outputs)
	}
}

func TestSessionSpeedCountMismatchKeepsOnlyTTFT(t *testing.T) {
	l := &speedLogs{}
	for i := 0; i < 2; i++ {
		l.turnStart(float64(i*10), "m")
		l.wait(float64(i*10) + 0.1)
		l.firstToken(float64(i*10) + 1.1)
		l.turnEnd(float64(i*10) + 5)
	}
	l.completed(7, 50, 4000, 1) // its timestamp matches neither turn

	ttfts, outputs := splitSamples(scanSpeedLogs(t, l))
	if len(ttfts) != 2 || len(outputs) != 0 {
		t.Fatalf("ttfts = %+v, outputs = %+v", ttfts, outputs)
	}
}

func TestSessionSpeedFallsBackToSummaryModel(t *testing.T) {
	l := &speedLogs{}
	l.wait(0)         // a log that starts mid-turn: no turn_started seen
	l.firstToken(1.5) // the summary names the model
	ttfts, _ := splitSamples(scanSpeedLogs(t, l))
	if len(ttfts) != 1 || ttfts[0].Model != "summary-model" || ttfts[0].TTFT != 1500*time.Millisecond {
		t.Fatalf("ttfts = %+v", ttfts)
	}
}

func TestSessionSpeedNoLogsIsEmpty(t *testing.T) {
	home := t.TempDir()
	(&speedLogs{}).write(t, home, "s1", speedBase)
	dir := filepath.Join(home, "sessions", "%2Ftmp%2Fdemo", "s1")
	for _, name := range []string{"events.jsonl", "updates.jsonl"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := SessionScanner{GrokHome: home}.ScanSpeed(context.Background(), speedBase.Add(time.Hour))
	if err != nil || len(got) != 1 || len(got[0].Samples) != 0 {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestOutputSamplesPairByUniqueCompletionSecond(t *testing.T) {
	turn := func(sec float64, model string) speedTurn {
		return speedTurn{at: speedBase.Add(time.Duration((sec - 5) * float64(time.Second))), endedAt: speedBase.Add(time.Duration(sec * float64(time.Second))), model: model, ttfts: []time.Duration{time.Second}}
	}
	use := func(sec int64, tokens int64) speedCompletion {
		return speedCompletion{at: time.Unix(sec, 0), hasUsage: true, output: tokens, apiMillis: 11000, calls: 1}
	}
	base := speedBase.Unix()
	turns := []speedTurn{turn(12.9, "b"), turn(20.001, "c")}
	done := []speedCompletion{use(base+5, 1), use(base+12, 2), use(base+20, 3)}

	got := outputSamples(turns, done, "high")
	if len(got) != 2 || got[0].Model != "b" || got[0].OutputTokens != 2 || got[1].Model != "c" || got[1].OutputTokens != 3 {
		t.Fatalf("timestamp pairing = %+v", got)
	}
	if got[0].Effort != "high" || got[0].Decode != 10*time.Second {
		t.Fatalf("sample = %+v", got[0])
	}
}

func TestOutputSampleNeedsWaitsAndPositiveDecode(t *testing.T) {
	turn := speedTurn{model: "m", ttfts: []time.Duration{3 * time.Second}}
	if _, ok := outputSample(turn, speedCompletion{hasUsage: true, output: 10, apiMillis: 3000, calls: 1}, ""); ok {
		t.Fatal("a turn that was all waiting has a decode time")
	}
	if _, ok := outputSample(speedTurn{model: "m"}, speedCompletion{hasUsage: true, output: 10, apiMillis: 9000, calls: 1}, ""); ok {
		t.Fatal("a turn without waits can not tell its decode time")
	}
	if _, ok := outputSample(turn, speedCompletion{hasUsage: true, output: 0, apiMillis: 9000, calls: 1}, ""); ok {
		t.Fatal("a turn without tokens has output")
	}
	if _, ok := outputSample(turn, speedCompletion{hasUsage: true, output: 10, apiMillis: 9000, calls: 2}, ""); ok {
		t.Fatal("a turn with an unobserved first token has output")
	}
}

func TestOutputSamplesRejectsAmbiguousSameSecond(t *testing.T) {
	turns := []speedTurn{
		{at: speedBase, endedAt: speedBase.Add(2 * time.Second), model: "a", ttfts: []time.Duration{time.Second}},
		{at: speedBase.Add(2100 * time.Millisecond), endedAt: speedBase.Add(2500 * time.Millisecond), model: "b", ttfts: []time.Duration{time.Second}},
	}
	done := []speedCompletion{
		{at: speedBase.Add(2 * time.Second), hasUsage: true, output: 10, apiMillis: 2000, calls: 1},
		{at: speedBase.Add(2 * time.Second), hasUsage: true, output: 20, apiMillis: 2000, calls: 1},
	}
	if got := outputSamples(turns, done, ""); len(got) != 0 {
		t.Fatalf("ambiguous same-second records paired: %+v", got)
	}
}

func TestOutputSamplesPairsUniqueCrossSecondCompletion(t *testing.T) {
	for _, tc := range []struct {
		name       string
		endedAt    time.Time
		completion time.Time
	}{
		{"completion after end", speedBase.Add(10999 * time.Millisecond), speedBase.Add(11 * time.Second)},
		{"completion before end", speedBase.Add(11001 * time.Millisecond), speedBase.Add(10 * time.Second)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			turns := []speedTurn{{at: speedBase, endedAt: tc.endedAt, model: "a", ttfts: []time.Duration{time.Second}}}
			done := []speedCompletion{{at: tc.completion, hasUsage: true, output: 10, apiMillis: 2000, calls: 1}}
			if got := outputSamples(turns, done, ""); len(got) != 1 || got[0].OutputTokens != 10 {
				t.Fatalf("cross-second pairing = %+v", got)
			}
		})
	}
}

func TestOutputSamplesRejectsAdjacentCrossSecondTurns(t *testing.T) {
	t.Run("two ended turns", func(t *testing.T) {
		turns := []speedTurn{
			{at: speedBase, endedAt: speedBase.Add(10999 * time.Millisecond), model: "a", ttfts: []time.Duration{time.Second}},
			{at: speedBase.Add(11010 * time.Millisecond), endedAt: speedBase.Add(11500 * time.Millisecond), model: "b", ttfts: []time.Duration{time.Second}},
		}
		done := []speedCompletion{
			{at: speedBase.Add(11 * time.Second), hasUsage: true, output: 10, apiMillis: 2000, calls: 1},
			{at: speedBase.Add(11 * time.Second), hasUsage: true, output: 20, apiMillis: 2000, calls: 1},
		}
		if got := outputSamples(turns, done, ""); len(got) != 0 {
			t.Fatalf("adjacent turns paired ambiguously: %+v", got)
		}
	})

	t.Run("newer turn still open", func(t *testing.T) {
		turns := []speedTurn{
			{at: speedBase, endedAt: speedBase.Add(11001 * time.Millisecond), model: "a", ttfts: []time.Duration{time.Second}},
			{at: speedBase.Add(11010 * time.Millisecond), model: "b", ttfts: []time.Duration{time.Second}},
		}
		done := []speedCompletion{
			{at: speedBase.Add(10 * time.Second), hasUsage: true, output: 10, apiMillis: 2000, calls: 1},
			{at: speedBase.Add(11 * time.Second), hasUsage: true, output: 999, apiMillis: 2000, calls: 1},
		}
		if got := outputSamples(turns, done, ""); len(got) != 0 {
			t.Fatalf("open turn completion assigned to older turn: %+v", got)
		}
	})
}

func TestSessionSpeedTailCutDoesNotShiftCompletions(t *testing.T) {
	home := t.TempDir()
	l := &speedLogs{}
	l.turnStart(0, "old-model")
	l.wait(0)
	l.firstToken(1)
	l.turnEnd(2)
	l.completed(2, 10, 2000, 1)
	filler := `{"type":"ignored","pad":"` + strings.Repeat("x", 1<<20) + `"}` + "\n"
	for i := 0; i < speed.TailBytes/(1<<20)+2; i++ {
		l.events.WriteString(filler)
	}
	l.turnStart(10, "model-b")
	l.wait(10)
	l.firstToken(11)
	l.turnEnd(12)
	l.completed(12, 20, 2000, 1)
	l.turnStart(20, "model-c")
	l.wait(20)
	l.firstToken(21)
	l.completed(21, 999, 2000, 1) // events snapshot has no ended turn for this completion
	l.write(t, home, "s", speedBase.Add(time.Minute))

	sessions, err := (SessionScanner{GrokHome: home}).ScanSpeed(context.Background(), speedBase.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	_, outputs := splitSamples(sessions[0].Samples)
	if len(outputs) != 1 || outputs[0].Model != "model-b" || outputs[0].OutputTokens != 20 {
		t.Fatalf("outputs = %+v", outputs)
	}
}

func TestScanSpeedPicksNewestSessionsInsideTheWindow(t *testing.T) {
	home := t.TempDir()
	now := speedBase.Add(48 * time.Hour)
	for i := 1; i <= 12; i++ {
		l := &speedLogs{}
		l.turnStart(0, "m")
		l.wait(0.1)
		l.firstToken(1.1)
		l.turnEnd(2)
		l.completed(2, 10, 1500, 1)
		l.write(t, home, fmt.Sprintf("s%02d", i), now.Add(-time.Duration(13-i)*time.Hour))
	}
	old := &speedLogs{}
	old.write(t, home, "old", now.Add(-speed.Window-time.Hour))

	got, err := SessionScanner{GrokHome: home}.ScanSpeed(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != speed.SessionLimit {
		t.Fatalf("got %d sessions, want %d", len(got), speed.SessionLimit)
	}
	if got[0].ID != "s12" || got[0].Title != "title s12" || got[0].ProjectName != "demo" || got[len(got)-1].ID != "s03" {
		t.Fatalf("order: first %+v, last %s", got[0], got[len(got)-1].ID)
	}
	if len(got[0].Samples) != 2 {
		t.Fatalf("samples = %+v", got[0].Samples)
	}
}

func TestScanSpeedMissingHomeIsEmpty(t *testing.T) {
	got, err := SessionScanner{GrokHome: filepath.Join(t.TempDir(), "none")}.ScanSpeed(context.Background(), time.Now())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, err %v", got, err)
	}
}

func TestScanSpeedHonoursCancel(t *testing.T) {
	home := t.TempDir()
	(&speedLogs{}).write(t, home, "s1", speedBase)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (SessionScanner{GrokHome: home}).ScanSpeed(ctx, speedBase); err == nil {
		t.Fatal("cancelled scan returned no error")
	}
}

func TestScanSpeedPropagatesIOErrorWithOtherResults(t *testing.T) {
	home := t.TempDir()
	(&speedLogs{}).write(t, home, "good", speedBase)
	(&speedLogs{}).write(t, home, "broken", speedBase)
	path := filepath.Join(home, "sessions", "%2Ftmp%2Fdemo", "broken", "events.jsonl")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := (SessionScanner{GrokHome: home}).ScanSpeed(context.Background(), speedBase.Add(time.Hour)); err == nil {
		t.Fatal("I/O failure was hidden by another successful session")
	}
}
