package interfaces

import (
	"time"

	"github.com/google/uuid"
)

// AdminBlockDefinitionRecord defines the admin read projection for a block definition.
type AdminBlockDefinitionRecord struct {
	ID              uuid.UUID
	Name            string
	Slug            string
	Type            string
	Description     *string
	Icon            *string
	Category        string
	Status          string
	Channel         string
	Schema          map[string]any
	UISchema        map[string]any
	SchemaVersion   string
	MigrationStatus string
	Locale          string
	CreatedAt       *time.Time
	UpdatedAt       *time.Time
}

// AdminBlockDefinitionVersionRecord defines the admin read projection for a definition version.
type AdminBlockDefinitionVersionRecord struct {
	ID              uuid.UUID
	DefinitionID    uuid.UUID
	SchemaVersion   string
	Schema          map[string]any
	Defaults        map[string]any
	MigrationStatus string
	CreatedAt       *time.Time
	UpdatedAt       *time.Time
}

// AdminBlockRecord defines the admin read projection for a block instance.
type AdminBlockRecord struct {
	ID             uuid.UUID
	DefinitionID   uuid.UUID
	ContentID      uuid.UUID
	Region         string
	Locale         string
	Status         string
	Data           map[string]any
	Position       int
	BlockType      string
	BlockSchemaKey string
}

// AdminBlockDefinitionListOptions defines admin block definition list reads.
type AdminBlockDefinitionListOptions struct {
	EnvironmentKey string
	Page           int
	PerPage        int
	SortBy         string
	SortDesc       bool
	Search         string
	Filters        map[string]any
	Fields         []string
}

// AdminBlockDefinitionGetOptions defines admin block definition detail reads.
type AdminBlockDefinitionGetOptions struct {
	EnvironmentKey string
}

// AdminBlockListOptions defines block-instance list reads for one content entry.
type AdminBlockListOptions struct {
	Locale         string
	FallbackLocale string
	EnvironmentKey string
}

// AdminBlockDefinitionCreateRequest captures block definition creation inputs.
type AdminBlockDefinitionCreateRequest struct {
	Name           string
	Slug           string
	Type           string
	Description    *string
	Icon           *string
	Category       *string
	Status         string
	Channel        string
	Schema         map[string]any
	UISchema       map[string]any
	EnvironmentKey string
}

// AdminBlockDefinitionUpdateRequest captures block definition update inputs.
type AdminBlockDefinitionUpdateRequest struct {
	ID             uuid.UUID
	Name           *string
	Slug           *string
	Type           *string
	Description    *string
	Icon           *string
	Category       *string
	Status         *string
	Channel        *string
	Schema         map[string]any
	UISchema       map[string]any
	EnvironmentKey string
}

// AdminBlockDefinitionDeleteRequest captures block definition deletion inputs.
type AdminBlockDefinitionDeleteRequest struct {
	ID         uuid.UUID
	HardDelete bool
}

// AdminBlockSaveRequest captures create-or-update block instance inputs.
type AdminBlockSaveRequest struct {
	ID             uuid.UUID
	DefinitionID   uuid.UUID
	ContentID      uuid.UUID
	Region         string
	Locale         string
	Status         string
	Data           map[string]any
	Position       int
	BlockType      string
	BlockSchemaKey string
	CreatedBy      uuid.UUID
	UpdatedBy      uuid.UUID
}

// AdminBlockDeleteRequest captures block deletion inputs.
type AdminBlockDeleteRequest struct {
	ID         uuid.UUID
	DeletedBy  uuid.UUID
	HardDelete bool
}
