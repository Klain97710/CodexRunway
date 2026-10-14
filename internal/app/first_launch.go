package app

import (
	"errors"

	"github.com/licoy/quosparr/internal/store"
)

// InitializeFirstLaunch records the initial preferences before UI and background
// work starts. Existing or migrated preferences already identify returning users.
// Saving defaults makes the decision survive a restart even without a settings edit.
func (s *Settings) InitializeFirstLaunch() (bool, error) {
	if !s.loaded {
		return false, errors.New("preferences must be loaded before initial launch")
	}
	if s.loadErr != nil {
		return false, s.loadErr
	}
	if s.source != store.SourceDefaults {
		return false, nil
	}
	if err := s.st.SavePreferences(s.prefs.Clone()); err != nil {
		return false, err
	}
	s.source = store.SourceCurrent
	return true, nil
}
