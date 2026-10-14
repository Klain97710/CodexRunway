package alerts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/licoy/quosparr/internal/export/swiftjson"
	"github.com/licoy/quosparr/internal/platform"
)

// StateFileName is the dedupe state inside ~/.quosparr.
const StateFileName = "alerts.json"

// MaxRememberedIDs is how many alert IDs are kept.
const MaxRememberedIDs = 200

// Store remembers which alert IDs were already shown. The file is a JSON array
// of strings, as the Swift store wrote it.
type Store struct {
	Path string
}

// DefaultPath is ~/.quosparr/alerts.json under home.
func DefaultPath(home string) string {
	return filepath.Join(platform.RunwayDir(home), StateFileName)
}

var storeMu sync.Mutex // load-modify-save is one step

// Unseen returns the alerts whose ID has not been seen and remembers them. With
// nothing new the file is left alone. A missing or unreadable file means
// nothing has been seen. Only the MaxRememberedIDs lexicographically last IDs
// are kept (as in Swift: sorted, then suffix), so an old ID can come back as
// new once newer IDs push it out.
func (s Store) Unseen(alerts []Alert) ([]Alert, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	ids := s.load()
	var unseen []Alert
	for _, a := range alerts {
		if _, ok := ids[a.ID]; !ok {
			unseen = append(unseen, a)
		}
	}
	if len(unseen) == 0 {
		return nil, nil
	}
	for _, a := range unseen {
		ids[a.ID] = struct{}{}
	}
	return unseen, s.save(ids)
}

func (s Store) load() map[string]struct{} {
	ids := map[string]struct{}{}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return ids
	}
	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return ids
	}
	for _, v := range values {
		ids[v] = struct{}{}
	}
	return ids
}

func (s Store) save(ids map[string]struct{}) error {
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	sort.Strings(list)
	if len(list) > MaxRememberedIDs {
		list = list[len(list)-MaxRememberedIDs:]
	}
	data, err := swiftjson.Marshal(list, false)
	if err != nil {
		return err
	}
	return platform.WriteFileAtomic(s.Path, data, 0o644)
}
