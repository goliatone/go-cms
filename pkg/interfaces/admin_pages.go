package interfaces

import (
	"time"

	"github.com/google/uuid"
)

// AdminPageRecord defines the admin read model projection for a page entry.
type AdminPageRecord struct {
	ID                 uuid.UUID
	ContentID          uuid.UUID
	TranslationGroupID *uuid.UUID
	TemplateID         uuid.UUID
	Title              string
	Slug               string
	Path               string
	RequestedLocale    string
	ResolvedLocale     string
	Translation        TranslationBundle[PageTranslation]    `json:"translation"`
	ContentTranslation TranslationBundle[ContentTranslation] `json:"content_translation"`
	Status             string
	ParentID           *uuid.UUID
	MetaTitle          string
	MetaDescription    string
	Summary            *string
	Tags               []string
	SchemaVersion      string
	Data               map[string]any
	Content            any
	Blocks             any
	PreviewURL         string
	PublishedAt        *time.Time
	CreatedAt          *time.Time
	UpdatedAt          *time.Time
}
