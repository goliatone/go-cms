package interfaces

// RequiredFieldsValidationStrategy controls how translation checks handle
// required-field keys that are not supported by the entity or schema.
type RequiredFieldsValidationStrategy string

const (
	RequiredFieldsValidationError  RequiredFieldsValidationStrategy = "error"
	RequiredFieldsValidationWarn   RequiredFieldsValidationStrategy = "warn"
	RequiredFieldsValidationIgnore RequiredFieldsValidationStrategy = "ignore"
)

// TranslationCheckOptions configures translation completeness checks.
//
// RequiredFields keys are locale codes (case-insensitive). The per-locale field
// list follows these rules:
//   - Pages: field keys refer to page translation fields: title, path, summary,
//     seo_title, seo_description.
//   - Content: field keys refer to schema field paths in the content translation
//     payload (e.g., body, seo.title).
//
// Unknown required-field keys are handled according to RequiredFieldsStrategy
// (defaults to "error"). For pages, Version is ignored unless translation
// snapshots are introduced in the page versioning workflow.
type TranslationCheckOptions struct {
	State                  string
	Environment            string
	Version                string
	RequiredFields         map[string][]string
	RequiredFieldsStrategy RequiredFieldsValidationStrategy
}
