package estimate

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/licoy/quosparr/internal/export/swiftjson"
	"github.com/licoy/quosparr/internal/platform"
)

// HistoryFileName is the history file inside ~/.quosparr.
const HistoryFileName = "quota-estimate-history.json"

// HistorySample is the estimate of one finished or running cycle. The JSON
// names and the whole-second ISO 8601 recordedAt are the Swift ones.
type HistorySample struct {
	CycleStartDate   string    `json:"cycleStartDate"`
	EstimatedCredits float64   `json:"estimatedCredits"`
	UsedPercent      float64   `json:"usedPercent"`
	UsedCredits      float64   `json:"usedCredits"`
	RecordedAt       time.Time `json:"recordedAt"`
}

// Normalized keeps one sample per cycle (the most recently recorded; on equal
// times the later one in the list), sorted by cycle start, and only the last
// MaxHistorySamples of them.
func Normalized(samples []HistorySample) []HistorySample {
	latest := map[string]HistorySample{}
	for _, s := range samples {
		if existing, ok := latest[s.CycleStartDate]; ok && s.RecordedAt.Before(existing.RecordedAt) {
			continue
		}
		latest[s.CycleStartDate] = s
	}
	out := make([]HistorySample, 0, len(latest))
	for _, s := range latest {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CycleStartDate < out[j].CycleStartDate })
	if len(out) > MaxHistorySamples {
		out = out[len(out)-MaxHistorySamples:]
	}
	return out
}

// Merging adds sample to samples and normalizes.
func Merging(samples []HistorySample, sample HistorySample) []HistorySample {
	merged := make([]HistorySample, 0, len(samples)+1)
	merged = append(merged, samples...)
	return Normalized(append(merged, sample))
}

// HistoryStore keeps the samples of every account in one file:
//
//	{"accounts":{"<account key>":[{"cycleStartDate":...},...]}}
//
// The account key is the identity match key of the signed-in account (for
// example "oauth:user:<user>|account:<account>", "unknown" when there is no
// auth), so a switched account keeps its own history. The file is a derived
// cache: a damaged or unreadable file reads as empty history and is replaced
// by the next Upsert.
type HistoryStore struct {
	Path string
}

// DefaultHistoryPath is ~/.quosparr/quota-estimate-history.json under home.
func DefaultHistoryPath(home string) string {
	return filepath.Join(platform.RunwayDir(home), HistoryFileName)
}

var historyMu sync.Mutex // Load-modify-save of the file is one step

type historyFile struct {
	Accounts map[string][]HistorySample `json:"accounts"`
}

// Load returns the normalized history of an account.
func (s HistoryStore) Load(accountKey string) []HistorySample {
	historyMu.Lock()
	defer historyMu.Unlock()
	return Normalized(s.loadFile().Accounts[accountKey])
}

// Upsert merges sample into the account's history, saves the file and returns
// the merged samples. When saving fails the merged samples are still returned
// together with the error; callers treat that as non-fatal, since the history
// is only a cache and a failed save must not fail a quota refresh.
func (s HistoryStore) Upsert(accountKey string, sample HistorySample) ([]HistorySample, error) {
	historyMu.Lock()
	defer historyMu.Unlock()
	payload := s.loadFile()
	samples := Merging(payload.Accounts[accountKey], sample)
	payload.Accounts[accountKey] = samples
	return samples, s.save(payload)
}

// loadFile reads the file; anything but a valid file with every sample intact
// is an empty payload, like the all-or-nothing Swift decoder.
func (s HistoryStore) loadFile() historyFile {
	empty := historyFile{Accounts: map[string][]HistorySample{}}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return empty
	}
	var raw struct {
		Accounts map[string][]struct {
			CycleStartDate   *string  `json:"cycleStartDate"`
			EstimatedCredits *float64 `json:"estimatedCredits"`
			UsedPercent      *float64 `json:"usedPercent"`
			UsedCredits      *float64 `json:"usedCredits"`
			RecordedAt       *string  `json:"recordedAt"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || raw.Accounts == nil {
		return empty
	}
	payload := historyFile{Accounts: make(map[string][]HistorySample, len(raw.Accounts))}
	for key, samples := range raw.Accounts {
		list := make([]HistorySample, 0, len(samples))
		for _, r := range samples {
			if r.CycleStartDate == nil || r.EstimatedCredits == nil || r.UsedPercent == nil || r.UsedCredits == nil || r.RecordedAt == nil {
				return empty
			}
			at, err := swiftjson.ParseDate(*r.RecordedAt)
			if err != nil {
				return empty
			}
			list = append(list, HistorySample{*r.CycleStartDate, *r.EstimatedCredits, *r.UsedPercent, *r.UsedCredits, at})
		}
		payload.Accounts[key] = list
	}
	return payload
}

func (s HistoryStore) save(payload historyFile) error {
	data, err := swiftjson.Marshal(payload, false)
	if err != nil {
		return err
	}
	if s.Path == "" {
		return errors.New("estimate: history path is empty")
	}
	return platform.WriteFileAtomic(s.Path, data, fs.FileMode(0o644))
}
