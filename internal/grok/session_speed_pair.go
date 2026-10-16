package grok

import (
	"time"

	"github.com/licoy/quosparr/internal/speed"
)

// outputSamples pairs records only when a turn and completion are each other's
// sole candidate. Open turns participate in ambiguity but cannot make samples.
func outputSamples(turns []speedTurn, done []speedCompletion, effort string) []speed.Sample {
	doneAt := make(map[int64][]int)
	for i, completion := range done {
		if !completion.at.IsZero() {
			doneAt[completion.at.Unix()] = append(doneAt[completion.at.Unix()], i)
		}
	}
	turnCounts := make([]int, len(turns))
	turnUnique := make([]int, len(turns))
	bucketTurnCounts := make(map[int64]int)
	addCandidates := func(turnIndex int, second int64, bucket []int) {
		if len(bucket) == 0 || !completionCouldBelong(turns[turnIndex], done[bucket[0]]) {
			return
		}
		if turnCounts[turnIndex] == 0 && len(bucket) == 1 {
			turnUnique[turnIndex] = bucket[0]
		}
		turnCounts[turnIndex] += len(bucket)
		bucketTurnCounts[second]++
	}
	for i, turn := range turns {
		if turn.endedAt.IsZero() {
			for second, bucket := range doneAt {
				addCandidates(i, second, bucket)
			}
			continue
		}
		endSecond := turn.endedAt.Unix()
		for second := endSecond - 1; second <= endSecond+1; second++ {
			addCandidates(i, second, doneAt[second])
		}
	}
	var out []speed.Sample
	for i, turn := range turns {
		if turn.endedAt.IsZero() || turnCounts[i] != 1 {
			continue
		}
		j := turnUnique[i]
		if bucketTurnCounts[done[j].at.Unix()] != 1 {
			continue
		}
		if s, ok := outputSample(turn, done[j], effort); ok {
			out = append(out, s)
		}
	}
	return out
}

// completionCouldBelong expands the candidate set for the timestamp's
// one-second precision and the observed subsecond difference in write order.
// This bound never proves a pair by itself; both records must remain unique.
func completionCouldBelong(turn speedTurn, done speedCompletion) bool {
	if turn.at.IsZero() || done.at.IsZero() {
		return false
	}
	bucketEnd := done.at.Add(time.Second)
	if !turn.at.Before(bucketEnd) {
		return false
	}
	if turn.endedAt.IsZero() {
		return true
	}
	return turn.endedAt.After(done.at.Add(-time.Second)) &&
		turn.endedAt.Before(bucketEnd.Add(time.Second))
}

// outputSample is the streaming part of one turn: the API time minus every
// observed wait for the first token. It requires one TTFT per model call.
func outputSample(turn speedTurn, done speedCompletion, effort string) (speed.Sample, bool) {
	if !done.hasUsage || done.output <= 0 || done.calls <= 0 ||
		done.calls != int64(len(turn.ttfts)) || turn.model == "" {
		return speed.Sample{}, false
	}
	var waits time.Duration
	for _, w := range turn.ttfts {
		waits += w
	}
	decode := time.Duration(done.apiMillis)*time.Millisecond - waits
	if decode <= 0 {
		return speed.Sample{}, false
	}
	return speed.Sample{
		At: turn.at, Model: turn.model, Effort: effort, OutputTokens: done.output, Decode: decode,
	}, true
}
