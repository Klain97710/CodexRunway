package grok

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/licoy/quosparr/internal/speed"
)

// ScanSpeed reads the model speed samples of the newest speed.SessionLimit
// sessions updated within speed.Window before now, newest first. Only the end
// of each log is read; a missing sessions directory is an empty result, an
// unreadable log fails the scan.
func (s SessionScanner) ScanSpeed(ctx context.Context, now time.Time) ([]speed.Session, error) {
	summaries, err := s.listSummaries(ctx, speed.SessionLimit)
	if err != nil {
		return nil, err
	}
	cutoff := now.Add(-speed.Window)
	var result []speed.Session
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
			return nil, err
		}
		result = append(result, speed.Session{
			ID: sum.id, Title: sum.title, ProjectName: sum.projectName,
			UpdatedAt: sum.updatedAt, Samples: samples,
		})
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
	eventsPath := filepath.Join(sum.directory, "events.jsonl")
	updatesPath := filepath.Join(sum.directory, "updates.jsonl")
	beforeEvents, err := speedLogStateAt(eventsPath)
	if err != nil {
		return nil, err
	}
	beforeUpdates, err := speedLogStateAt(updatesPath)
	if err != nil {
		return nil, err
	}
	events, _, err := readSpeedLog(eventsPath)
	if err != nil {
		return nil, err
	}
	cur := &speedEvents{fallbackModel: model, effort: sum.effort}
	if err := speed.EachLine(ctx, events, cur.line); err != nil {
		return nil, err
	}
	updates, _, err := readSpeedLog(updatesPath)
	if err != nil {
		return nil, err
	}
	var done speedCompletions
	if err := speed.EachLine(ctx, updates, done.line); err != nil {
		return nil, err
	}
	samples := cur.samples
	afterEvents, err := speedLogStateAt(eventsPath)
	if err != nil {
		return nil, err
	}
	afterUpdates, err := speedLogStateAt(updatesPath)
	if err != nil {
		return nil, err
	}
	if beforeEvents != afterEvents || beforeUpdates != afterUpdates {
		return samples, nil
	}
	turns := cur.ended
	if cur.cur != nil {
		turns = append(turns, *cur.cur)
	}
	return append(samples, outputSamples(turns, done.turns, sum.effort)...), nil
}

type speedLogState struct {
	exists   bool
	size     int64
	modified int64
}

func speedLogStateAt(path string) (speedLogState, error) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return speedLogState{}, nil
	}
	if err != nil {
		return speedLogState{}, err
	}
	return speedLogState{exists: true, size: info.Size(), modified: info.ModTime().UnixNano()}, nil
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
	at      time.Time
	endedAt time.Time
	model   string
	ttfts   []time.Duration
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
		e.cur.endedAt = at
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
	at        time.Time
	hasUsage  bool
	output    int64
	apiMillis int64
	calls     int64
}

type speedUpdateLine struct {
	Timestamp int64 `json:"timestamp"`
	Params    struct {
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
	at := time.Time{}
	if l.Timestamp > 0 {
		at = time.Unix(l.Timestamp, 0)
	}
	u := l.Params.Update.Usage
	if u == nil {
		c.turns = append(c.turns, speedCompletion{at: at})
		return
	}
	calls := int64(u.ModelCalls)
	if u.ModelCalls != float64(calls) {
		calls = 0
	}
	c.turns = append(c.turns, speedCompletion{
		at: at, hasUsage: true, output: int64(u.OutputTokens),
		apiMillis: int64(u.APIDurationMs), calls: calls,
	})
}
