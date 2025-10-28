package markdowncmd

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
)

const (
	importDirectoryMessageType = "cms.markdown.import_directory"
	syncDirectoryMessageType   = "cms.markdown.sync_directory"
)

// ImportDirectoryCommand triggers a filesystem walk for Markdown documents
// under the provided Directory. The command mirrors markdown.Service
// ImportDirectory semantics, allowing callers to supply import options that
// map directly onto interfaces.ImportOptions for content and page creation.
type ImportDirectoryCommand struct {
	// Directory selects the filesystem path (relative or absolute) to load Markdown files from.
	Directory string `json:"directory"`
	// ContentTypeID assigns the target CMS content type for imported documents.
	ContentTypeID uuid.UUID `json:"content_type_id,omitempty"`
	// AuthorID sets the author reference recorded on created content entities.
	AuthorID uuid.UUID `json:"author_id,omitempty"`
	// TemplateID optionally points at the page template applied when CreatePages is true.
	TemplateID *uuid.UUID `json:"template_id,omitempty"`
	// CreatePages instructs the importer to create CMS pages alongside content records.
	CreatePages bool `json:"create_pages,omitempty"`
	// DryRun toggles preview mode to collect import diffs without persisting changes.
	DryRun bool `json:"dry_run,omitempty"`
}

// Type implements command.Message.
func (ImportDirectoryCommand) Type() string { return importDirectoryMessageType }

// Validate ensures directory input is present before handlers execute.
func (cmd ImportDirectoryCommand) Validate() error {
	err := validation.ValidateStruct(&cmd,
		validation.Field(&cmd.Directory, validation.Required, validation.By(func(value any) error {
			if strings.TrimSpace(value.(string)) == "" {
				return validation.NewError("cms.markdown.import_directory.directory_required", "directory is required")
			}
			return nil
		})),
	)
	if err != nil {
		return err
	}
	if cmd.TemplateID != nil && *cmd.TemplateID == uuid.Nil {
		return validation.Errors{
			"template_id": validation.NewError("cms.markdown.import_directory.template_id_invalid", "template_id must be a valid UUID when provided"),
		}
	}
	return nil
}

// SyncDirectoryCommand orchestrates a Markdown sync run for the provided
// Directory, applying deletion or update flags consistent with
// interfaces.SyncOptions.
type SyncDirectoryCommand struct {
	// Directory selects the filesystem path (relative or absolute) to load Markdown files from.
	Directory string `json:"directory"`
	// ContentTypeID assigns the target CMS content type for imported documents.
	ContentTypeID uuid.UUID `json:"content_type_id,omitempty"`
	// AuthorID sets the author reference recorded on created content entities.
	AuthorID uuid.UUID `json:"author_id,omitempty"`
	// TemplateID optionally points at the page template applied when CreatePages is true.
	TemplateID *uuid.UUID `json:"template_id,omitempty"`
	// CreatePages instructs the importer to create CMS pages alongside content records.
	CreatePages bool `json:"create_pages,omitempty"`
	// DryRun toggles preview mode to collect import diffs without persisting changes.
	DryRun bool `json:"dry_run,omitempty"`
	// DeleteOrphaned removes CMS records without matching Markdown files when true.
	DeleteOrphaned bool `json:"delete_orphaned,omitempty"`
	// UpdateExisting overwrites existing CMS records when Markdown files have changed.
	UpdateExisting bool `json:"update_existing,omitempty"`
}

// Type implements command.Message.
func (SyncDirectoryCommand) Type() string { return syncDirectoryMessageType }

// Validate ensures directory input is present before handlers execute.
func (cmd SyncDirectoryCommand) Validate() error {
	err := validation.ValidateStruct(&cmd,
		validation.Field(&cmd.Directory, validation.Required, validation.By(func(value any) error {
			if strings.TrimSpace(value.(string)) == "" {
				return validation.NewError("cms.markdown.sync_directory.directory_required", "directory is required")
			}
			return nil
		})),
	)
	if err != nil {
		return err
	}
	if cmd.TemplateID != nil && *cmd.TemplateID == uuid.Nil {
		return validation.Errors{
			"template_id": validation.NewError("cms.markdown.sync_directory.template_id_invalid", "template_id must be a valid UUID when provided"),
		}
	}
	return nil
}
