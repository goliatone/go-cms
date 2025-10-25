package themes

import "sort"

// RegionInfo summarises region capabilities for consumers.
type RegionInfo struct {
	Key            string
	Name           string
	AcceptsBlocks  bool
	AcceptsWidgets bool
	Fallbacks      []string
}

// InspectTemplateRegions flattens region metadata for a template.
func InspectTemplateRegions(template *Template) []RegionInfo {
	if template == nil || len(template.Regions) == 0 {
		return nil
	}
	result := make([]RegionInfo, 0, len(template.Regions))
	for key, region := range template.Regions {
		result = append(result, RegionInfo{
			Key:            key,
			Name:           region.Name,
			AcceptsBlocks:  region.AcceptsBlocks,
			AcceptsWidgets: region.AcceptsWidgets,
			Fallbacks:      append([]string{}, region.FallbackRegions...),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})
	return result
}

// InspectThemeRegions aggregates region info across templates.
func InspectThemeRegions(templates []*Template) map[string][]RegionInfo {
	if len(templates) == 0 {
		return nil
	}
	out := make(map[string][]RegionInfo, len(templates))
	for _, tpl := range templates {
		out[tpl.Slug] = InspectTemplateRegions(tpl)
	}
	return out
}
