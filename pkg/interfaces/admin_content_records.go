package interfaces

import (
	"time"

	"github.com/google/uuid"
)

// AdminContentRecord defines the admin read model projection for a content entry.
type AdminContentRecord struct {
	ID                     uuid.UUID
	FamilyID               *uuid.UUID
	Title                  string
	Slug                   string
	Locale                 string
	RequestedLocale        string
	ResolvedLocale         string
	AvailableLocales       []string
	MissingRequestedLocale bool
	Navigation             map[string]string
	EffectiveMenuLocations []string
	ContentType            string
	ContentTypeSlug        string
	Status                 string
	Blocks                 []string
	EmbeddedBlocks         []map[string]any
	SchemaVersion          string
	Data                   map[string]any
	Metadata               map[string]any
	CreatedAt              *time.Time
	UpdatedAt              *time.Time
	PublishedAt            *time.Time
}

// AdminContentCreateRequest captures admin-shaped fields for content creation.
type AdminContentCreateRequest struct {
	ContentTypeID            uuid.UUID
	ContentType              string
	ContentTypeSlug          string
	Title                    string
	Slug                     string
	Locale                   string
	FamilyID                 *uuid.UUID
	Status                   string
	Navigation               map[string]string
	EffectiveMenuLocations   []string
	Blocks                   []string
	EmbeddedBlocks           []map[string]any
	SchemaVersion            string
	Data                     map[string]any
	Metadata                 map[string]any
	EnvironmentKey           string
	CreatedBy                uuid.UUID
	UpdatedBy                uuid.UUID
	AllowMissingTranslations bool
}

// AdminContentUpdateRequest captures admin-shaped fields for content updates.
type AdminContentUpdateRequest struct {
	ID                       uuid.UUID
	ContentTypeID            uuid.UUID
	ContentType              string
	ContentTypeSlug          string
	Title                    string
	Slug                     string
	Locale                   string
	FamilyID                 *uuid.UUID
	Status                   string
	Navigation               map[string]string
	EffectiveMenuLocations   []string
	Blocks                   []string
	EmbeddedBlocks           []map[string]any
	SchemaVersion            string
	Data                     map[string]any
	Metadata                 map[string]any
	EnvironmentKey           string
	UpdatedBy                uuid.UUID
	AllowMissingTranslations bool
}

// AdminContentDeleteRequest captures content deletion inputs.
type AdminContentDeleteRequest struct {
	ID         uuid.UUID
	DeletedBy  uuid.UUID
	HardDelete bool
}

// AdminContentCreateTranslationRequest captures admin translation clone inputs.
type AdminContentCreateTranslationRequest struct {
	SourceID       uuid.UUID
	SourceLocale   string
	TargetLocale   string
	EnvironmentKey string
	ActorID        uuid.UUID
	Status         string
	Path           string
	RouteKey       string
	Metadata       map[string]any
}
