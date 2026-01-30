package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/google/uuid"
)

var errEnvironmentServiceUnavailable = errors.New("environment service unavailable")

func (api *AdminAPI) resolveEnvironmentKey(r *http.Request, payloadKey string, payloadID *uuid.UUID) (string, error) {
	if trimmed := strings.TrimSpace(payloadKey); trimmed != "" {
		return trimmed, nil
	}
	if payloadID != nil && *payloadID != uuid.Nil {
		return api.environmentKeyForID(requestContext(r), *payloadID)
	}
	if r == nil || r.URL == nil {
		return "", nil
	}
	query := r.URL.Query()
	if key := strings.TrimSpace(query.Get("env")); key != "" {
		return key, nil
	}
	if key := strings.TrimSpace(query.Get("environment")); key != "" {
		return key, nil
	}
	if key := strings.TrimSpace(query.Get("environment_key")); key != "" {
		return key, nil
	}
	if idRaw := strings.TrimSpace(query.Get("environment_id")); idRaw != "" {
		parsed, err := parseUUID(idRaw)
		if err != nil {
			return "", fmt.Errorf("%w: invalid environment_id", errBadRequest)
		}
		return api.environmentKeyForID(requestContext(r), parsed)
	}
	return "", nil
}

func (api *AdminAPI) resolveEnvironmentKeyWithDefault(r *http.Request, payloadKey string, payloadID *uuid.UUID, requireExplicit bool) (string, error) {
	key, err := api.resolveEnvironmentKey(r, payloadKey, payloadID)
	if err != nil {
		return "", err
	}
	defaultKey := ""
	if api != nil {
		defaultKey = api.defaultEnvKey
	}
	return cmsenv.ResolveKey(key, defaultKey, requireExplicit)
}

func (api *AdminAPI) environmentKeyForID(ctx context.Context, id uuid.UUID) (string, error) {
	if id == uuid.Nil {
		return "", nil
	}
	if api == nil || api.environments == nil {
		if id == cmsenv.IDForKey(cmsenv.DefaultKey) {
			return cmsenv.DefaultKey, nil
		}
		return "", errEnvironmentServiceUnavailable
	}
	env, err := api.environments.GetEnvironment(ctx, id)
	if err != nil {
		return "", err
	}
	return env.Key, nil
}

func requestContext(r *http.Request) context.Context {
	if r == nil {
		return context.Background()
	}
	return r.Context()
}
