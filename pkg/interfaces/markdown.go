package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// MarkdownParser defines how raw Markdown bytes are converted into HTML.
// Implementations should follow the file-centric workflows captured in
// docs/FEAT_MARKDOWN.md, supporting reusable parser instances and extension
// toggles so hosts can tailor rendering without rewriting the core service.
type MarkdownParser interface {
	// Parse converts Markdown into HTML using the parser's default settings.
	Parse(markdown []byte) ([]byte, error)
	// ParseWithOptions converts Markdown into HTML using the supplied overrides.
	ParseWithOptions(markdown []byte, opts ParseOptions) ([]byte, error)
}

// ParseOptions customises Markdown parsing behaviour, keeping option names
// readable for configuration unmarshalling and CLI flags.
type ParseOptions struct {
	Extensions []string
	Sanitize   bool
	HardWraps  bool
	SafeMode   bool
}

// MarkdownService exposes the high-level file workflows described in
// docs/FEAT_MARKDOWN.md Option 1, enabling hosts to load Markdown documents,
// convert them into HTML, and synchronise them with CMS content.
type MarkdownService interface {
	Load(ctx context.Context, path string, opts LoadOptions) (*Document, error)
	LoadDirectory(ctx context.Context, dir string, opts LoadOptions) ([]*Document, error)
	Render(ctx context.Context, markdown []byte, opts ParseOptions) ([]byte, error)
	RenderDocument(ctx context.Context, doc *Document, opts ParseOptions) ([]byte, error)
	Import(ctx context.Context, doc *Document, opts ImportOptions) (*ImportResult, error)
	ImportDirectory(ctx context.Context, dir string, opts ImportOptions) (*ImportResult, error)
	Sync(ctx context.Context, dir string, opts SyncOptions) (*SyncResult, error)
}

// Document represents a Markdown file with parsed metadata and content. The
// struct is shared between the interfaces package and internal implementations
// so consumers can depend on a stable contract.
type Document struct {
	FilePath     string
	Locale       string
	FrontMatter  FrontMatter
	Body         []byte
	BodyHTML     []byte
	LastModified time.Time
	// Checksum stores a digest of the original file content (typically SHA-256)
	// so sync workflows can detect changes without re-importing unchanged files.
	Checksum []byte
}

// FrontMatter models metadata extracted from Markdown files. Fields align with
// the canonical examples in docs/FEAT_MARKDOWN.md and remain flexible thanks to
// the Custom map for template- or domain-specific values.
type FrontMatter struct {
	Title    string         `yaml:"title" json:"title"`
	Slug     string         `yaml:"slug" json:"slug"`
	Summary  string         `yaml:"summary" json:"summary"`
	Status   string         `yaml:"status" json:"status"`
	Template string         `yaml:"template" json:"template"`
	Tags     []string       `yaml:"tags" json:"tags"`
	Author   string         `yaml:"author" json:"author"`
	Date     time.Time      `yaml:"date" json:"date"`
	Draft    bool           `yaml:"draft" json:"draft"`
	Custom   map[string]any `yaml:",inline" json:"custom"`
	Raw      map[string]any `yaml:"-" json:"raw"`
}

// LoadOptions fine-tunes how documents are discovered and parsed from disk.
type LoadOptions struct {
	Recursive      *bool
	Pattern        string
	LocalePatterns map[string]string
	Parser         ParseOptions
}

// ImportOptions controls how Markdown documents are converted into CMS content.
// UUID fields reference existing CMS entities (content types, authors, etc.).
type ImportOptions struct {
	ContentTypeID uuid.UUID
	AuthorID      uuid.UUID
	CreatePages   bool
	TemplateID    *uuid.UUID
	DryRun        bool
}

// SyncOptions extends ImportOptions to handle update/delete semantics for
// repeated synchronisation runs.
type SyncOptions struct {
	ImportOptions
	DeleteOrphaned bool
	UpdateExisting bool
}

// ImportResult reports the outcome of a single import operation, exposing
// counts and IDs so callers can audit behaviour or trigger follow-up actions.
type ImportResult struct {
	CreatedContentIDs []uuid.UUID
	UpdatedContentIDs []uuid.UUID
	SkippedContentIDs []uuid.UUID
	Errors            []error
}

// SyncResult summarises a bulk sync run across many files.
type SyncResult struct {
	Created int
	Updated int
	Deleted int
	Skipped int
	Errors  []error
}
