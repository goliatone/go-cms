package translationconfig

import "sync/atomic"

// State provides a concurrency-safe view of translation toggles.
type State struct {
	enabled atomic.Bool
	require atomic.Bool
}

// NewState constructs a new state seeded with settings.
func NewState(settings Settings) *State {
	st := &State{}
	st.enabled.Store(settings.TranslationsEnabled)
	st.require.Store(settings.RequireTranslations)
	return st
}

// Enabled reports whether translations are enabled globally.
func (s *State) Enabled() bool {
	if s == nil {
		return false
	}
	return s.enabled.Load()
}

// RequireTranslations reports whether translations are required (regardless of Enabled).
func (s *State) RequireTranslations() bool {
	if s == nil {
		return false
	}
	return s.require.Load()
}

// Required reports whether translations are required when enabled.
func (s *State) Required() bool {
	if s == nil {
		return false
	}
	return s.enabled.Load() && s.require.Load()
}

// SetEnabled updates the global translations enabled toggle.
func (s *State) SetEnabled(enabled bool) {
	if s == nil {
		return
	}
	s.enabled.Store(enabled)
}

// SetRequireTranslations updates the translations required toggle.
func (s *State) SetRequireTranslations(required bool) {
	if s == nil {
		return
	}
	s.require.Store(required)
}
