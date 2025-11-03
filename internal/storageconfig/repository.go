package storageconfig

import (
	"context"
	"errors"
	"maps"

	"github.com/goliatone/go-cms/pkg/storage"
)

// ErrProfileNotFound indicates that a requested storage profile does not exist.
var ErrProfileNotFound = errors.New("storageconfig: profile not found")

// ErrProfileNameRequired indicates that profile operations require a non-empty name.
var ErrProfileNameRequired = errors.New("storageconfig: profile name is required")

// Repository exposes persistence operations for runtime storage profiles.
type Repository interface {
	List(ctx context.Context) ([]storage.Profile, error)
	Get(ctx context.Context, name string) (*storage.Profile, error)
	Upsert(ctx context.Context, profile storage.Profile) (*storage.Profile, error)
	Delete(ctx context.Context, name string) error
	Subscribe(ctx context.Context) (<-chan ChangeEvent, error)
}

// ChangeType enumerates storage profile change events.
type ChangeType string

const (
	// ChangeCreated indicates a new profile was persisted.
	ChangeCreated ChangeType = "created"
	// ChangeUpdated indicates an existing profile was modified.
	ChangeUpdated ChangeType = "updated"
	// ChangeDeleted indicates a profile was removed.
	ChangeDeleted ChangeType = "deleted"
)

// ChangeEvent reports profile mutations to interested subscribers.
type ChangeEvent struct {
	Type    ChangeType
	Profile storage.Profile
}

func cloneConfig(cfg storage.Config) storage.Config {
	cloned := cfg
	if cfg.Options != nil {
		cloned.Options = maps.Clone(cfg.Options)
	}
	return cloned
}

func cloneProfile(profile storage.Profile) storage.Profile {
	cloned := profile
	cloned.Config = cloneConfig(profile.Config)
	if profile.Fallbacks != nil {
		cloned.Fallbacks = append([]string(nil), profile.Fallbacks...)
	}
	if profile.Labels != nil {
		cloned.Labels = maps.Clone(profile.Labels)
	}
	return cloned
}

func newChangeEvent(changeType ChangeType, profile storage.Profile) ChangeEvent {
	return ChangeEvent{
		Type:    changeType,
		Profile: cloneProfile(profile),
	}
}
