package grok

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"time"

	"github.com/licoy/quosparr/internal/speed"
)

// ScanSpeed reads the model speed samples of the newest speed.SessionLimit
// sessions updated within speed.Window before now, newest first. Only the end
// of each log is read; a missing sessions directory is an empty result, an
// unreadable log skips its session.
func (s SessionScanner) ScanSpeed(ctx context.Context, now time.Time) ([]speed.Session, error) {
	summaries, err := s.listSummaries(ctx, speed.SessionLimit)
	if err != nil {
		return nil, err
	}
	cutoff := now.Add(-speed.Window)
	var result []speed.Session
	var firstErr error
	for _, sum := range summaries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if sum.updatedAt.Before(cutoff) {
			continue
		}
		samples, err := scanSessionSpeed(ctx, sum)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		result = append(result, speed.Session{
			ID: sum.id, Title: sum.title, ProjectName: sum.projectName,
			UpdatedAt: sum.updatedAt, Samples: samples,
		})
	}
	if len(result) == 0 && firstErr != nil {
		return nil, firstErr
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].UpdatedAt.After(result[j].UpdatedAt) })
	return result[:min(len(result), speed.SessionLimit)], nil
}

// scanSessionSpeed reads one session: the time to first token of every model
// call from events.jsonl, and the tokens per turn from updates.jsonl.
func scanSessionSpeed(ctx context.Context, sum sessionSummaryFile) ([]speed.Sample, error) {
	model := ""
	if sum.model != nil {
		model = *sum.model
	}
	events, eventsCut, err := readSpeedLog(filepath.Join(sum.directory, "events.jsonl"))
	if err != nil {
		return nil, err
	}
	cur := &speedEvents{fallbackModel: model, effort: sum.effort}
	if err := speed.EachLine(ctx, events, cur.line); err != nil {
		return nil, err
	}
	updates, updatesCut, err := readSpeedLog(filepath.Join(sum.directory, "updates.jsonl"))
	if err != nil {
		return nil, err
	}
	var done speedCompletions
	if err := speed.EachLine(ctx, updates, done.line); err != nil {
		return nil, err
	}
	samples := cur.samples
	return append(samples, outputSamples(cur.ended, done.turns, eventsCut || updatesCut, sum.effort)...), nil
}

// readSpeedLog is the tail of a session log; a log that does not exist yet
// (a new session) is empty.
func readSpeedLog(path string) ([]byte, bool, error) {
	data, cut, err := speed.ReadTail(path, speed.TailBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	return data, cut, err
}

// ---- events.jsonl ----

var (
	speedMarkTurnStarted = []byte(`"type":"turn_started"`)
	speedMarkTurnEnded   = []byte(`"type":"turn_ended"`)
	speedMarkFirstToken  = []byte(`"type":"first_token"`)
	speedMarkWaiting     = []byte(`"phase":"waiting_for_model"`)
)

type speedEvent struct {
	TS      string `json:"ts"`
	Type    string `json:"type"`
	Phase   string `json:"phase"`
	ModelID string `json:"model_id"`
}

// speedTurn is a turn that started and ended in the log.
type speedTurn struct {
	at    time.Time
	model string
	ttfts []time.Duration
}

// speedEvents follows the turns of events.jsonl. A model call is the wait from
// the latest waiting_for_model to the next first_token.
type speedEvents struct {
	fallbackModel, effort string

	cur     *speedTurn
	waitAt  time.Time
	waiting bool

	samples []speed.Sample
	ended   []speedTurn
}

func (e *speedEvents) line(line []byte) {
	if !wantedEventLine(line) {
		return
	}
	var ev speedEvent
	if json.Unmarshal(line, &ev) != nil {
		return
	}
	at, err := time.Parse(time.RFC3339Nano, ev.TS)
	if err != nil {
		return
	}
	switch {
	case ev.Type == "turn_started":
		e.turnStarted(at, ev.ModelID)
	case ev.Type == "phase_changed" && ev.Phase == "waiting_for_model":
		e.waitAt, e.waiting = at, true
	case ev.Type == "first_token":
		e.firstToken(at)
	case ev.Type == "turn_ended" && e.cur != nil:
		e.ended = append(e.ended, *e.cur)
		e.cur = nil
	}
}

func (e *speedEvents) turnStarted(at time.Time, model string) {
	if model == "" {
		model = e.fallbackModel
	}
	e.cur, e.waiting = &speedTurn{at: at, model: model}, false
}

func wantedEventLine(line []byte) bool {
	return bytes.Contains(line, speedMarkTurnStarted) || bytes.Contains(line, speedMarkTurnEnded) ||
		bytes.Contains(line, speedMarkFirstToken) || bytes.Contains(line, speedMarkWaiting)
}

func (e *speedEvents) firstToken(at time.Time) {
	if !e.waiting {
		return
	}
	e.waiting = false
	wait := at.Sub(e.waitAt)
	if wait < 0 || wait > speed.MaxTTFT {
		return
	}
	model := e.fallbackModel
	if e.cur != nil {
		e.cur.ttfts = append(e.cur.ttfts, wait)
		if e.cur.model != "" {
			model = e.cur.model
		}
	}
	e.samples = append(e.samples, speed.Sample{
		At: e.waitAt, Model: model, Effort: e.effort, TTFT: wait, HasTTFT: true,
	})
}

// ---- updates.jsonl ----

var speedMarkTurnCompleted = []byte(`"sessionUpdate":"turn_completed"`)

// speedCompletion is the usage of one turn_completed record.
type speedCompletion struct {
	hasUsage  bool
	output    int64
	apiMillis int64
	calls     int64
}

type speedUpdateLine struct {
	Params struct {
		Update struct {
			SessionUpdate string `json:"sessionUpdate"`
			Usage         *struct {
				OutputTokens  float64 `json:"outputTokens"`
				APIDurationMs float64 `json:"apiDurationMs"`
				ModelCalls    float64 `json:"modelCalls"`
			} `json:"usage"`
		} `json:"update"`
	} `json:"params"`
}

type speedCompletions struct{ turns []speedCompletion }

func (c *speedCompletions) line(line []byte) {
	if !bytes.Contains(line, speedMarkTurnCompleted) {
		return
	}
	var l speedUpdateLine
	if json.Unmarshal(line, &l) != nil || l.Params.Update.SessionUpdate != "turn_completed" {
		return
	}
	u := l.Params.Update.Usage
	if u == nil {
		c.turns = append(c.turns, speedCompletion{})
		return
	}
	c.turns = append(c.turns, speedCompletion{
		hasUsage: true, output: int64(u.OutputTokens), apiMillis: int64(u.APIDurationMs), calls: int64(u.ModelCalls),
	})
}

// ---- pairing ----

// outputSamples pairs the ended turns of events.jsonl with the turn_completed
// records of updates.jsonl, which share no id: the Nth of one is the Nth of
// the other. Unequal counts pair only when a log was cut at its start, from
// the end; otherwise the logs disagree and no output sample is made.
func outputSamples(turns []speedTurn, done []speedCompletion, cut bool, effort string) []speed.Sample {
	if len(turns) != len(done) {
		if !cut {
			return nil
		}
		n := min(len(turns), len(done))
		turns, done = turns[len(turns)-n:], done[len(done)-n:]
	}
	var out []speed.Sample
	for i, turn := range turns {
		if s, ok := outputSample(turn, done[i], effort); ok {
			out = append(out, s)
		}
	}
	return out
}

// outputSample is the streaming part of one turn: the API time minus the waits
// for the first token. A call that logged no first_token is charged the
// turn's average wait, so that wait does not count as streaming.
func outputSample(turn speedTurn, done speedCompletion, effort string) (speed.Sample, bool) {
	if !done.hasUsage || done.output <= 0 || len(turn.ttfts) == 0 || turn.model == "" {
		return speed.Sample{}, false
	}
	var waits time.Duration
	for _, w := range turn.ttfts {
		waits += w
	}
	calls := max(done.calls, int64(len(turn.ttfts)))
	waits = waits / time.Duration(len(turn.ttfts)) * time.Duration(calls)
	decode := time.Duration(done.apiMillis)*time.Millisecond - waits
	if decode <= 0 {
		return speed.Sample{}, false
	}
	return speed.Sample{
		At: turn.at, Model: turn.model, Effort: effort, OutputTokens: done.output, Decode: decode,
	}, true
}
