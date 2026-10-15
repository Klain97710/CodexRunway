package claude

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/licoy/quosparr/internal/speed"
	"github.com/licoy/quosparr/internal/usage"
)

// SpeedSessions reads one sample per deduped assistant message. Claude logs
// have no first-token time: At is the previous user or tool_result time,
// HasTTFT is false, and Decode is the record time minus At. Decode <= 0 is
// dropped. Effort is the record's effort field.
func (s SessionScanner) SpeedSessions(ctx context.Context, now time.Time) ([]speed.Session, error) {
	if now.IsZero() {
		now = time.Now()
	}
	window := usage.Window{Start: now.Add(-speed.Window), End: now.Add(time.Nanosecond)}
	sink, err := s.collect(ctx, speed.SessionLimit, window.Start)
	if err != nil {
		return nil, err
	}
	byID := map[string]*speed.Session{}
	var order []string
	for _, key := range sink.order {
		rec := sink.kept[key]
		sample, ok := speedSample(rec)
		if !ok {
			continue
		}
		session := byID[rec.sessionID]
		if session == nil {
			meta := sink.metas[rec.sessionID]
			session = &speed.Session{ID: rec.sessionID, UpdatedAt: meta.updated}
			if meta.title != "" {
				session.Title = meta.title
			}
			if meta.cwd != "" {
				session.ProjectName = usage.ProjectDisplayName(meta.cwd)
			}
			byID[rec.sessionID] = session
			order = append(order, rec.sessionID)
		}
		session.Samples = append(session.Samples, sample)
	}
	out := make([]speed.Session, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	if len(out) > speed.SessionLimit {
		out = out[:speed.SessionLimit]
	}
	return out, nil
}

func speedSample(rec usageRecord) (speed.Sample, bool) {
	if !rec.hasParent || !rec.hasTime || rec.model == "" {
		return speed.Sample{}, false
	}
	decode := rec.at.Sub(rec.parent)
	if decode <= 0 {
		return speed.Sample{}, false
	}
	return speed.Sample{
		At: rec.parent, Model: rec.model, Effort: rec.effort, Tier: rec.tier,
		HasTTFT: false, OutputTokens: rec.parts.Output, Decode: decode,
	}, true
}

// speedTier maps the usage speed field. Logs written before fast mode have
// no field and stay unknown rather than counting as standard.
func speedTier(v string) speed.Tier {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "fast":
		return speed.TierFast
	case "standard":
		return speed.TierStandard
	}
	return speed.TierUnknown
}

// ScanSpeed builds a speed.Report from SpeedSessions. TTFT stays empty.
func (s SessionScanner) ScanSpeed(ctx context.Context, now time.Time) (speed.Report, error) {
	sessions, err := s.SpeedSessions(ctx, now)
	if err != nil {
		return speed.Report{}, err
	}
	return speed.Build(sessions, now), nil
}
