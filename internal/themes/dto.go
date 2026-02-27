package themes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	cmsthemes "github.com/goliatone/go-cms/themes"
	"github.com/google/uuid"
)

type (
	RegisterThemeInput    = cmsthemes.RegisterThemeInput
	RegisterTemplateInput = cmsthemes.RegisterTemplateInput
	UpdateTemplateInput   = cmsthemes.UpdateTemplateInput
)

var (
	// ErrTemplateThemeRequired indicates the theme ID is missing.
	ErrTemplateThemeRequired = cmsthemes.ErrTemplateThemeRequired
	// ErrTemplateNameRequired indicates the template name is missing.
	ErrTemplateNameRequired = cmsthemes.ErrTemplateNameRequired
	// ErrTemplateSlugRequired indicates the slug is missing.
	ErrTemplateSlugRequired = cmsthemes.ErrTemplateSlugRequired
	// ErrTemplatePathRequired indicates the file path is missing.
	ErrTemplatePathRequired = cmsthemes.ErrTemplatePathRequired
	// ErrTemplateSlugConflict indicates a duplicate slug within a theme.
	ErrTemplateSlugConflict = cmsthemes.ErrTemplateSlugConflict
	// ErrTemplateRegionsInvalid indicates malformed region metadata.
	ErrTemplateRegionsInvalid = cmsthemes.ErrTemplateRegionsInvalid
)

// ValidateRegisterTemplate ensures new template inputs are well formed.
func ValidateRegisterTemplate(ctx context.Context, repo TemplateRepository, input RegisterTemplateInput) error {
	if input.ThemeID == uuid.Nil {
		return ErrTemplateThemeRequired
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ErrTemplateNameRequired
	}
	slug := canonicalSlug(input.Slug)
	if slug == "" {
		return ErrTemplateSlugRequired
	}
	path := strings.TrimSpace(input.TemplatePath)
	if path == "" {
		return ErrTemplatePathRequired
	}

	if err := validateRegions(input.Regions); err != nil {
		return err
	}

	if repo != nil {
		if _, err := repo.GetBySlug(ctx, input.ThemeID, slug); err == nil {
			return ErrTemplateSlugConflict
		} else {
			var nf *NotFoundError
			if !errors.As(err, &nf) && err != nil {
				return err
			}
		}
	}
	return nil
}

// PrepareTemplateRecord normalises register template input for persistence.
func PrepareTemplateRecord(input RegisterTemplateInput, idGenerator func() uuid.UUID) *Template {
	record := &Template{
		ID:           uuid.Nil,
		ThemeID:      input.ThemeID,
		Name:         strings.TrimSpace(input.Name),
		Slug:         canonicalSlug(input.Slug),
		Description:  cloneString(input.Description),
		TemplatePath: strings.TrimSpace(input.TemplatePath),
		Regions:      cloneTemplateRegions(input.Regions),
		Metadata:     deepCloneMap(input.Metadata),
	}
	if idGenerator == nil {
		record.ID = uuid.New()
	} else {
		record.ID = idGenerator()
	}
	return record
}

// ValidateUpdateTemplate ensures updates preserve invariants.
func ValidateUpdateTemplate(input UpdateTemplateInput) error {
	if input.TemplateID == uuid.Nil {
		return &NotFoundError{Resource: "template", Key: ""}
	}
	if input.Name != nil && strings.TrimSpace(*input.Name) == "" {
		return ErrTemplateNameRequired
	}
	if input.TemplatePath != nil && strings.TrimSpace(*input.TemplatePath) == "" {
		return ErrTemplatePathRequired
	}
	if input.Regions != nil {
		if err := validateRegions(input.Regions); err != nil {
			return err
		}
	}
	return nil
}

func validateRegions(regions map[string]TemplateRegion) error {
	if len(regions) == 0 {
		return fmt.Errorf("%w: at least one region required", ErrTemplateRegionsInvalid)
	}
	for key, region := range regions {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("%w: region key cannot be empty", ErrTemplateRegionsInvalid)
		}
		if strings.TrimSpace(region.Name) == "" {
			return fmt.Errorf("%w: region %s missing name", ErrTemplateRegionsInvalid, key)
		}
		if !region.AcceptsBlocks && !region.AcceptsWidgets {
			return fmt.Errorf("%w: region %s must accept blocks or widgets", ErrTemplateRegionsInvalid, key)
		}
	}
	return nil
}

func canonicalSlug(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
