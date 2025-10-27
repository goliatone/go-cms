package pagescmd

// FeatureGates exposes runtime feature toggles required by page command handlers.
// Callers can inject closures wired to cms.Config.Features to avoid tight coupling.
type FeatureGates struct {
	// VersioningEnabled should return true when page versioning workflows are enabled.
	VersioningEnabled func() bool
	// SchedulingEnabled should return true when page scheduling workflows are enabled.
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
