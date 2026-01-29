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
	"github.com/goliatone/go-cms/internal/permissions"
	"github.com/goliatone/go-cms/internal/validation"
	"github.com/google/uuid"
)

type errorResponse struct {
	Error   string                       `json:"error"`
	Message string                       `json:"message,omitempty"`
	Issues  []validation.ValidationIssue `json:"issues,omitempty"`
}

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

	if errors.Is(err, permissions.ErrPermissionDenied) {
		return http.StatusForbidden, errorResponse{
			Error:   "forbidden",
			Message: err.Error(),
		}
	}

	if errors.Is(err, content.ErrContentTypeSlugExists) || errors.Is(err, blocks.ErrDefinitionExists) {
		return http.StatusConflict, errorResponse{
			Error:   "conflict",
			Message: err.Error(),
		}
	}

	if errors.Is(err, content.ErrContentTypeSchemaBreaking) ||
		errors.Is(err, content.ErrContentTypeStatusChange) ||
		errors.Is(err, blocks.ErrDefinitionInUse) ||
		errors.Is(err, blocks.ErrDefinitionVersionExists) {
		return http.StatusConflict, errorResponse{
			Error:   "conflict",
			Message: err.Error(),
		}
	}

	if errors.Is(err, validation.ErrSchemaInvalid) ||
		errors.Is(err, validation.ErrSchemaValidation) ||
		errors.Is(err, validation.ErrSchemaMigration) ||
		errors.Is(err, content.ErrContentTypeSchemaInvalid) ||
		errors.Is(err, blocks.ErrDefinitionSchemaInvalid) {
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
		errors.Is(err, blocks.ErrDefinitionNameRequired) ||
		errors.Is(err, blocks.ErrDefinitionSchemaRequired) ||
		errors.Is(err, blocks.ErrDefinitionSchemaVersionInvalid) ||
		errors.Is(err, blocks.ErrDefinitionIDRequired) {
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
