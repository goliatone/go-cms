package storage

// Profile captures a named storage configuration that can be referenced by
// repositories at runtime. Future phases will persist these profiles and drive
// hot-swap mechanics based on admin updates.
type Profile struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Provider    string            `json:"provider"`
	Config      Config            `json:"config"`
	Fallbacks   []string          `json:"fallbacks,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Default     bool              `json:"default,omitempty"`
}

// ConfigJSONSchema documents the runtime shape expected by storage providers.
// It is intentionally minimal; provider-specific options are captured in the
// nested "options" map.
const ConfigJSONSchema = `
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "StorageConfig",
  "type": "object",
  "required": ["name", "driver", "dsn"],
  "properties": {
    "name": {
      "type": "string",
      "description": "Human readable identifier for the storage configuration"
    },
    "driver": {
      "type": "string",
      "description": "Driver identifier understood by the storage adapter (e.g. bun, s3)"
    },
    "dsn": {
      "type": "string",
      "description": "Connection string or URI for the provider"
    },
    "readOnly": {
      "type": "boolean",
      "default": false
    },
    "options": {
      "type": "object",
      "additionalProperties": true
    }
  },
  "additionalProperties": false
}
`

// ProfileJSONSchema describes the payload used to define storage profiles in
// admin APIs. The container will validate updates against this schema before
// attempting a hot reload.
const ProfileJSONSchema = `
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "StorageProfile",
  "type": "object",
  "required": ["name", "provider", "config"],
  "properties": {
    "name": {
      "type": "string",
      "pattern": "^[a-z0-9_-]+$",
      "description": "Profile identifier referenced by repositories"
    },
    "description": {
      "type": "string"
    },
    "provider": {
      "type": "string",
      "description": "Registered provider key resolved by the DI container"
    },
    "config": ` + ConfigJSONSchema + `,
    "fallbacks": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "description": "Ordered list of fallback profiles to consult when the primary is unavailable"
    },
    "labels": {
      "type": "object",
      "additionalProperties": {
        "type": "string"
      }
    },
    "default": {
      "type": "boolean",
      "description": "Marks the profile as the default selection for repositories without explicit configuration"
    }
  },
  "additionalProperties": false
}
`
