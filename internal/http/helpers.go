package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/permissions"
	"github.com/goliatone/go-cms/internal/validation"
	"github.com/google/uuid"
)

type errorResponse struct {
	Error   string                       `json:"error"`
	Message string                       `json:"message,omitempty"`
	Issues  []validation.ValidationIssue `json:"issues,omitempty"`
}

var errBadRequest = errors.New("bad_request")

func joinPath(base, suffix string) string {
	trimmedBase := strings.TrimSpace(base)
	trimmedSuffix := strings.TrimSpace(suffix)
	if trimmedBase == "" {
		if trimmedSuffix == "" {
			return "/"
		}
		return "/" + strings.Trim(trimmedSuffix, "/")
	}
	baseClean := "/" + strings.Trim(trimmedBase, "/")
	if trimmedSuffix == "" {
		return baseClean
	}
	return baseClean + "/" + strings.Trim(trimmedSuffix, "/")
}

func decodeJSON(r *http.Request, target any) error {
	if r == nil || r.Body == nil {
		return io.EOF
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	if w == nil {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error) {
	status, payload := mapError(err)
	writeJSON(w, status, payload)
}

func mapError(err error) (int, errorResponse) {
	if err == nil {
		return http.StatusInternalServerError, errorResponse{Error: "unknown_error"}
	}

	var contentNotFound *content.NotFoundError
	if errors.As(err, &contentNotFound) {
		return http.StatusNotFound, errorResponse{
			Error:   "not_found",
			Message: contentNotFound.Error(),
		}
	}

	var blockNotFound *blocks.NotFoundError
	if errors.As(err, &blockNotFound) {
		return http.StatusNotFound, errorResponse{
			Error:   "not_found",
			Message: blockNotFound.Error(),
		}
	}

	var envNotFound *cmsenv.NotFoundError
	if errors.As(err, &envNotFound) || errors.Is(err, cmsenv.ErrEnvironmentNotFound) {
		return http.StatusNotFound, errorResponse{
			Error:   "not_found",
			Message: err.Error(),
		}
	}

	var pageNotFound *pages.PageNotFoundError
	if errors.As(err, &pageNotFound) {
		return http.StatusNotFound, errorResponse{
			Error:   "not_found",
			Message: pageNotFound.Error(),
		}
	}

	var pageVersionNotFound *pages.PageVersionNotFoundError
	if errors.As(err, &pageVersionNotFound) {
		return http.StatusNotFound, errorResponse{
			Error:   "not_found",
			Message: pageVersionNotFound.Error(),
		}
	}

	var menuNotFound *menus.NotFoundError
	if errors.As(err, &menuNotFound) || errors.Is(err, menus.ErrMenuNotFound) || errors.Is(err, menus.ErrMenuItemNotFound) || errors.Is(err, menus.ErrMenuItemPageNotFound) {
		return http.StatusNotFound, errorResponse{
			Error:   "not_found",
			Message: err.Error(),
		}
	}

	if errors.Is(err, content.ErrContentTypeRequired) ||
		errors.Is(err, content.ErrContentTranslationNotFound) ||
		errors.Is(err, pages.ErrContentRequired) ||
		errors.Is(err, pages.ErrParentNotFound) ||
		errors.Is(err, pages.ErrTemplateUnknown) ||
		errors.Is(err, pages.ErrPageTranslationNotFound) {
		return http.StatusNotFound, errorResponse{
			Error:   "not_found",
			Message: err.Error(),
		}
	}

	if errors.Is(err, permissions.ErrPermissionDenied) {
		return http.StatusForbidden, errorResponse{
			Error:   "forbidden",
			Message: err.Error(),
		}
	}

	if errors.Is(err, cmsenv.ErrEnvironmentKeyExists) ||
		errors.Is(err, content.ErrContentTypeSlugExists) ||
		errors.Is(err, content.ErrSlugExists) ||
		errors.Is(err, blocks.ErrDefinitionExists) ||
		errors.Is(err, menus.ErrMenuCodeExists) {
		return http.StatusConflict, errorResponse{
			Error:   "conflict",
			Message: err.Error(),
		}
	}

	if errors.Is(err, content.ErrContentTypeSchemaBreaking) ||
		errors.Is(err, content.ErrContentTypeStatusChange) ||
		errors.Is(err, blocks.ErrDefinitionInUse) ||
		errors.Is(err, blocks.ErrDefinitionVersionExists) ||
		errors.Is(err, pages.ErrSlugExists) ||
		errors.Is(err, pages.ErrPathExists) ||
		errors.Is(err, pages.ErrPageParentCycle) ||
		errors.Is(err, pages.ErrPageDuplicateSlug) ||
		errors.Is(err, menus.ErrMenuInUse) ||
		errors.Is(err, menus.ErrMenuItemHasChildren) {
		return http.StatusConflict, errorResponse{
			Error:   "conflict",
			Message: err.Error(),
		}
	}

	if errors.Is(err, validation.ErrSchemaInvalid) ||
		errors.Is(err, validation.ErrSchemaValidation) ||
		errors.Is(err, validation.ErrSchemaMigration) ||
		errors.Is(err, content.ErrContentTypeSchemaInvalid) ||
		errors.Is(err, blocks.ErrDefinitionSchemaInvalid) ||
		errors.Is(err, content.ErrContentSchemaInvalid) {
		return http.StatusUnprocessableEntity, errorResponse{
			Error:   "validation_failed",
			Message: err.Error(),
			Issues:  validation.Issues(err),
		}
	}

	if errors.Is(err, content.ErrContentTypeNameRequired) ||
		errors.Is(err, content.ErrContentTypeSchemaRequired) ||
		errors.Is(err, content.ErrContentTypeSlugInvalid) ||
		errors.Is(err, content.ErrContentTypeStatusInvalid) ||
		errors.Is(err, content.ErrSlugRequired) ||
		errors.Is(err, content.ErrSlugInvalid) ||
		errors.Is(err, content.ErrNoTranslations) ||
		errors.Is(err, content.ErrDefaultLocaleRequired) ||
		errors.Is(err, content.ErrDuplicateLocale) ||
		errors.Is(err, content.ErrUnknownLocale) ||
		errors.Is(err, content.ErrContentIDRequired) ||
		errors.Is(err, content.ErrContentSoftDeleteUnsupported) ||
		errors.Is(err, content.ErrContentTranslationsDisabled) ||
		errors.Is(err, pages.ErrTemplateRequired) ||
		errors.Is(err, pages.ErrSlugRequired) ||
		errors.Is(err, pages.ErrSlugInvalid) ||
		errors.Is(err, pages.ErrNoPageTranslations) ||
		errors.Is(err, pages.ErrDefaultLocaleRequired) ||
		errors.Is(err, pages.ErrDuplicateLocale) ||
		errors.Is(err, pages.ErrUnknownLocale) ||
		errors.Is(err, pages.ErrPageRequired) ||
		errors.Is(err, pages.ErrPageSoftDeleteUnsupported) ||
		errors.Is(err, menus.ErrMenuCodeRequired) ||
		errors.Is(err, menus.ErrMenuCodeInvalid) ||
		errors.Is(err, menus.ErrMenuItemParentInvalid) ||
		errors.Is(err, menus.ErrMenuItemPosition) ||
		errors.Is(err, menus.ErrMenuItemTargetMissing) ||
		errors.Is(err, menus.ErrMenuItemTranslations) ||
		errors.Is(err, menus.ErrMenuItemDuplicateLocale) ||
		errors.Is(err, menus.ErrUnknownLocale) ||
		errors.Is(err, menus.ErrTranslationExists) ||
		errors.Is(err, menus.ErrTranslationLabelRequired) ||
		errors.Is(err, menus.ErrMenuItemPageSlugRequired) ||
		errors.Is(err, menus.ErrMenuItemTypeInvalid) ||
		errors.Is(err, menus.ErrMenuItemParentUnsupported) ||
		errors.Is(err, menus.ErrMenuItemSeparatorFields) ||
		errors.Is(err, menus.ErrMenuItemGroupFields) ||
		errors.Is(err, menus.ErrMenuItemCollapsibleWithoutChildren) ||
		errors.Is(err, menus.ErrMenuItemCollapsedWithoutCollapsible) ||
		errors.Is(err, menus.ErrMenuItemTranslationTextRequired) ||
		errors.Is(err, cmsenv.ErrEnvironmentKeyRequired) ||
		errors.Is(err, cmsenv.ErrEnvironmentKeyInvalid) ||
		errors.Is(err, cmsenv.ErrEnvironmentNameRequired) ||
		errors.Is(err, blocks.ErrDefinitionNameRequired) ||
		errors.Is(err, blocks.ErrDefinitionSchemaRequired) ||
		errors.Is(err, blocks.ErrDefinitionSchemaVersionInvalid) ||
		errors.Is(err, blocks.ErrDefinitionIDRequired) {
		return http.StatusBadRequest, errorResponse{
			Error:   "bad_request",
			Message: err.Error(),
		}
	}

	if errors.Is(err, errEnvironmentServiceUnavailable) {
		return http.StatusServiceUnavailable, errorResponse{
			Error:   "service_unavailable",
			Message: err.Error(),
		}
	}

	if errors.Is(err, errBadRequest) {
		return http.StatusBadRequest, errorResponse{
			Error:   "bad_request",
			Message: err.Error(),
		}
	}

	return http.StatusInternalServerError, errorResponse{
		Error:   "internal_error",
		Message: err.Error(),
	}
}

func parseUUID(value string) (uuid.UUID, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return uuid.Nil, errors.New("uuid required")
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return uuid.Nil, err
	}
	return parsed, nil
}

func parseBoolQuery(value string, defaultValue bool) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(trimmed)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func resolveActorID(primary, secondary *uuid.UUID) uuid.UUID {
	if primary != nil && *primary != uuid.Nil {
		return *primary
	}
	if secondary != nil && *secondary != uuid.Nil {
		return *secondary
	}
	return uuid.Nil
}

func requirePermission(w http.ResponseWriter, r *http.Request, permission string) bool {
	if strings.TrimSpace(permission) == "" {
		return true
	}
	if r == nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "request missing"})
		return false
	}
	if err := permissions.Require(r.Context(), permission); err != nil {
		writeError(w, err)
		return false
	}
	return true
}
