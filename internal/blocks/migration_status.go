package blocks

import "strings"

// ResolveDefinitionMigrationStatus determines the current migration status for a schema.
func ResolveDefinitionMigrationStatus(schema map[string]any, schemaVersion string) string {
	status := schemaMigrationStatusFromSchema(schema)
	if status != "" {
		return status
	}
	if strings.TrimSpace(schemaVersion) == "" {
		return "unversioned"
	}
	metaVersion := schemaVersionFromSchema(schema)
	if metaVersion != "" && strings.TrimSpace(schemaVersion) != "" && metaVersion != schemaVersion {
		return "mismatch"
	}
	return "current"
}

func schemaVersionFromSchema(schema map[string]any) string {
	if schema == nil {
		return ""
	}
	if meta, ok := schema["metadata"].(map[string]any); ok {
		if version, ok := meta["schema_version"].(string); ok {
			return strings.TrimSpace(version)
		}
	}
	if version, ok := schema["schema_version"].(string); ok {
		return strings.TrimSpace(version)
	}
	return ""
}

func schemaMigrationStatusFromSchema(schema map[string]any) string {
	if schema == nil {
		return ""
	}
	if meta, ok := schema["metadata"].(map[string]any); ok {
		if status, ok := meta["migration_status"].(string); ok {
			return strings.TrimSpace(status)
		}
	}
	if meta, ok := schema["x-cms"].(map[string]any); ok {
		if status, ok := meta["migration_status"].(string); ok {
			return strings.TrimSpace(status)
		}
	}
	if meta, ok := schema["x-admin"].(map[string]any); ok {
		if status, ok := meta["migration_status"].(string); ok {
			return strings.TrimSpace(status)
		}
	}
	return ""
}
