package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/licoy/quosparr/internal/sessions"
	"github.com/licoy/quosparr/internal/store"
)

const (
	sessionPageSize = 50
	dateLayout      = "2006-01-02"
)

type sessionFilter struct {
	search       string
	from, to     string
	project      string
	projectLabel string
	start, end   time.Time // end is the exclusive start of the day after "to"
}

func (f sessionFilter) apply(location *time.Location) (sessionFilter, error) {
	if location == nil {
		location = time.Local
	}
	next := sessionFilter{search: strings.TrimSpace(f.search), from: strings.TrimSpace(f.from), to: strings.TrimSpace(f.to), project: f.project, projectLabel: f.projectLabel}
	var err error
	if next.from != "" {
		next.start, err = time.ParseInLocation(dateLayout, next.from, location)
		if err != nil {
			return sessionFilter{}, errors.New("invalid start date")
		}
	}
	if next.to != "" {
		to, parseErr := time.ParseInLocation(dateLayout, next.to, location)
		if parseErr != nil {
			return sessionFilter{}, errors.New("invalid end date")
		}
		next.end = to.AddDate(0, 0, 1)
	}
	if !next.start.IsZero() && !next.end.IsZero() && !next.start.Before(next.end) {
		return sessionFilter{}, errors.New("date range is reversed")
	}
	return next, nil
}

func (f sessionFilter) matches(item sessions.ManagedSession) bool {
	if f.project != "" && f.project != sessionProjectKey(item) {
		return false
	}
	q := strings.ToLower(f.search)
	if q != "" && !containsAnyFold(q, item.Title, item.ProjectName, item.ID) {
		return false
	}
	local := item.UpdatedAt.Local()
	return (f.start.IsZero() || !local.Before(f.start)) && (f.end.IsZero() || local.Before(f.end))
}

func containsAnyFold(query string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

type sessionDetailState struct {
	key        string
	task       Task
	generation uint64
	detail     sessions.ManagedDetail
	loaded     bool
}

type settingsSessions struct {
	provider  store.Provider
	draft     sessionFilter
	applied   sessionFilter
	filterErr error

	task       Task
	cancel     context.CancelFunc
	generation uint64
	items      []sessions.ManagedSession
	loaded     bool
	shown      int
	selected   map[string]bool
	dateOpen   [2]bool
	dateCursor [2]time.Time

	open    string
	detail  sessionDetailState
	confirm bool
	// confirmKey names the one session a row or detail page asked to
	// delete; empty, the confirmation deletes the selection.
	confirmKey string
	work       Task
	notice     string
	noticeErr  bool
}

func (u *settingsUI) sessionsState() *settingsSessions {
	if u.sessions == nil {
		u.sessions = &settingsSessions{provider: store.ProviderCodex, shown: sessionPageSize, selected: map[string]bool{}}
	}
	return u.sessions
}

// cancelSessionTasks stops read-only scans and invalidates detail callbacks.
// A confirmed delete keeps running so its real per-file result is preserved.
func (u *settingsUI) cancelSessionTasks() {
	if u.sessions == nil {
		return
	}
	s := u.sessions
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.generation++
	s.detail.generation++
}

func (u *settingsUI) sessionActions() SessionActions {
	if u.opts.Sessions == nil {
		return SessionActions{}
	}
	return u.opts.Sessions()
}

func sessionKey(item sessions.ManagedSession) string {
	if item.Path != "" {
		return item.Path
	}
	return item.Provider + "\x00" + item.ID
}

func (s *settingsSessions) filtered() []sessions.ManagedSession {
	result := make([]sessions.ManagedSession, 0, len(s.items))
	for _, item := range s.items {
		if s.applied.matches(item) {
			result = append(result, item)
		}
	}
	slices.SortStableFunc(result, func(a, b sessions.ManagedSession) int {
		if n := b.UpdatedAt.Compare(a.UpdatedAt); n != 0 {
			return n
		}
		return strings.Compare(a.ID, b.ID)
	})
	return result
}

func (s *settingsSessions) clearSelection() {
	s.selected = map[string]bool{}
	s.confirm, s.confirmKey = false, ""
}

// confirmDelete asks to delete one session, or the selection for "".
func (s *settingsSessions) confirmDelete(key string) {
	s.confirm, s.confirmKey = true, key
}

// deleteTargets are the sessions the open confirmation deletes.
func (s *settingsSessions) deleteTargets() []sessions.ManagedSession {
	if s.confirmKey == "" {
		return s.selectedItems()
	}
	for _, item := range s.items {
		if sessionKey(item) == s.confirmKey {
			return []sessions.ManagedSession{item}
		}
	}
	return nil
}

func (s *settingsSessions) applyFilter(location *time.Location) bool {
	next, err := s.draft.apply(location)
	s.filterErr = err
	if err != nil {
		return false
	}
	s.applied, s.shown = next, sessionPageSize
	s.clearSelection()
	return true
}

// filterPending reports whether the draft differs from the applied filter,
// so Apply has changes waiting.
func (s *settingsSessions) filterPending() bool {
	d, a := s.draft, s.applied
	return strings.TrimSpace(d.search) != a.search || strings.TrimSpace(d.from) != a.from ||
		strings.TrimSpace(d.to) != a.to || d.project != a.project
}

// quickRange reports the quick range f covers on now's day: 0 for all
// dates, or the days ending today that setQuickRange would choose.
func (f sessionFilter) quickRange(now time.Time) (int, bool) {
	if f.from == "" && f.to == "" {
		return 0, true
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if f.to != today.Format(dateLayout) {
		return 0, false
	}
	for _, days := range []int{1, 7, 30} {
		if f.from == today.AddDate(0, 0, 1-days).Format(dateLayout) {
			return days, true
		}
	}
	return 0, false
}

func (s *settingsSessions) setQuickRange(now time.Time, days int) {
	local := now.In(now.Location())
	if days <= 0 {
		s.draft.from, s.draft.to = "", ""
	} else {
		today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
		s.draft.from = today.AddDate(0, 0, 1-days).Format(dateLayout)
		s.draft.to = today.Format(dateLayout)
	}
	s.applyFilter(local.Location())
}

func (s *settingsSessions) switchProvider(provider store.Provider) {
	if s.provider == provider {
		return
	}
	s.provider, s.open = provider, ""
	s.draft.project, s.applied.project = "", ""
	s.draft.projectLabel, s.applied.projectLabel = "", ""
	s.detail = sessionDetailState{generation: s.detail.generation + 1}
	s.items, s.loaded, s.shown = nil, false, sessionPageSize
	s.clearSelection()
}

func (u *settingsUI) startSessionsScan(actions SessionActions) {
	s := u.sessionsState()
	if s.cancel != nil {
		s.cancel()
	}
	s.generation++
	gen, provider := s.generation, s.provider
	if actions.Query == nil {
		s.task = Task{Err: errDetailUnavailable}
		return
	}
	s.task = Task{Running: true}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go func() {
		items, err := actions.Query(ctx, provider)
		u.post(func() {
			if ctx.Err() != nil || gen != s.generation || provider != s.provider {
				return
			}
			s.cancel, s.task = nil, Task{Err: err}
			if err == nil {
				s.items, s.loaded, s.shown = items, true, sessionPageSize
				s.clearSelection()
				s.detail = sessionDetailState{generation: s.detail.generation + 1}
			}
		})
	}()
}

func (u *settingsUI) loadSessionDetail(actions SessionActions, item sessions.ManagedSession) {
	s := u.sessionsState()
	key := sessionKey(item)
	if s.detail.key == key && (s.detail.task.Running || s.detail.task.Err != nil || s.detail.loaded) {
		return
	}
	s.detail.generation++
	gen := s.detail.generation
	s.detail.key, s.detail.task = key, Task{Running: true}
	if actions.Detail == nil {
		s.detail.task = Task{Err: errDetailUnavailable}
		return
	}
	go func() {
		detail, err := actions.Detail(context.Background(), item)
		u.post(func() {
			if s.detail.key != key || s.detail.generation != gen {
				return
			}
			s.detail.task = Task{Err: err}
			if err == nil {
				s.detail.detail, s.detail.loaded = detail, true
			}
		})
	}()
}

func (s *settingsSessions) selectedItems() []sessions.ManagedSession {
	var result []sessions.ManagedSession
	for _, item := range s.items {
		if s.selected[sessionKey(item)] {
			result = append(result, item)
		}
	}
	return result
}
