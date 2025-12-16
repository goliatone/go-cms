package translationconfig

import (
	"context"
	"errors"
)

// ErrSettingsNotFound indicates that translation settings have not been configured yet.
var ErrSettingsNotFound = errors.New("translationconfig: settings not found")

// Settings capture runtime translation enforcement toggles.
type Settings struct {
	TranslationsEnabled bool
	RequireTranslations bool
}

// Repository persists translation settings and emits change notifications.
type Repository interface {
	Get(ctx context.Context) (Settings, error)
	Upsert(ctx context.Context, settings Settings) (Settings, error)
	Delete(ctx context.Context) error
	Subscribe(ctx context.Context) (<-chan ChangeEvent, error)
}

// ChangeType enumerates settings change events.
type ChangeType string

const (
	// ChangeCreated indicates settings were first persisted.
	ChangeCreated ChangeType = "created"
	// ChangeUpdated indicates settings were updated.
	ChangeUpdated ChangeType = "updated"
	// ChangeDeleted indicates settings were cleared.
	ChangeDeleted ChangeType = "deleted"
)

// ChangeEvent reports settings mutations to interested subscribers.
type ChangeEvent struct {
	Type     ChangeType
	Settings Settings
}

func newChangeEvent(changeType ChangeType, settings Settings) ChangeEvent {
	return ChangeEvent{
		Type:     changeType,
		Settings: settings,
	}
}
