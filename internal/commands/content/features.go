package contentcmd

// FeatureGates exposes runtime feature toggles required by content command handlers.
// Callers can supply closures that read from cms.Config to keep the handlers decoupled
// from configuration packages while still honouring feature flags.
type FeatureGates struct {
	// VersioningEnabled should return true when the content versioning workflows are enabled.
	VersioningEnabled func() bool
	// SchedulingEnabled should return true when content scheduling features are enabled.
	SchedulingEnabled func() bool
}

func (g FeatureGates) versioningEnabled() bool {
	if g.VersioningEnabled == nil {
		return true
	}
	return g.VersioningEnabled()
}

func (g FeatureGates) schedulingEnabled() bool {
	if g.SchedulingEnabled == nil {
		return true
	}
	return g.SchedulingEnabled()
}
