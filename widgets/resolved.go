package widgets

// ResolvedWidget pairs a widget instance with its placement metadata.
type ResolvedWidget struct {
	Instance  *Instance      `json:"instance"`
	Placement *AreaPlacement `json:"placement"`
}
