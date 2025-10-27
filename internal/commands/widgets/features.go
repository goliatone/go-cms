package widgetscmd

// FeatureGates exposes the runtime toggles required by widget command handlers.
type FeatureGates struct {
	WidgetsEnabled func() bool
}

func (g FeatureGates) widgetsEnabled() bool {
	if g.WidgetsEnabled == nil {
		return true
	}
	return g.WidgetsEnabled()
}
