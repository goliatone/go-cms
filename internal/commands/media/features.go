package mediacmd

// FeatureGates exposes the runtime toggles required by media command handlers.
// Calling code can inject closures that read from cms.Config to avoid concrete
// dependencies while still honouring feature flags.
type FeatureGates struct {
	// MediaLibraryEnabled returns true when the media library feature is active.
	MediaLibraryEnabled func() bool
}

func (g FeatureGates) mediaLibraryEnabled() bool {
	if g.MediaLibraryEnabled == nil {
		return true
	}
	return g.MediaLibraryEnabled()
}
