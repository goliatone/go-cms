package markdowncmd

// FeatureGates exposes runtime feature toggles required by markdown command handlers.
// Callers should supply closures that read from cms.Config.Features.Markdown so handlers
// stay decoupled from configuration while honouring feature flags.
type FeatureGates struct {
	MarkdownEnabled func() bool
}

func (g FeatureGates) markdownEnabled() bool {
	if g.MarkdownEnabled == nil {
		return true
	}
	return g.MarkdownEnabled()
}
