package schema

// RootSchemaVersion reads the _schema value from a payload.
func RootSchemaVersion(payload map[string]any) (Version, bool) {
	if payload == nil {
		return Version{}, false
	}
	value, ok := payload[RootSchemaKey].(string)
	if !ok || value == "" {
		return Version{}, false
	}
	version, err := ParseVersion(value)
	if err != nil {
		return Version{}, false
	}
	return version, true
}
