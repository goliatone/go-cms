package translationconfig

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/uptrace/bun"
)

// BunRepository persists translation settings using a Bun-backed database.
type BunRepository struct {
	db          *bun.DB
	broadcaster *changeBroadcaster
}

// NewBunRepository constructs a Bun-backed repository.
func NewBunRepository(db *bun.DB) *BunRepository {
	return &BunRepository{
		db:          db,
		broadcaster: newChangeBroadcaster(),
	}
}

// Get returns the persisted translation settings.
func (r *BunRepository) Get(ctx context.Context) (Settings, error) {
	if r.db == nil {
		return Settings{}, errors.New("translationconfig: bun repository requires a database")
	}
	var model settingsModel
	if err := r.db.NewSelect().Model(&model).Where("id = ?", 1).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Settings{}, ErrSettingsNotFound
		}
		return Settings{}, err
	}
	return modelToSettings(&model), nil
}

// Upsert creates or updates the persisted translation settings.
func (r *BunRepository) Upsert(ctx context.Context, settings Settings) (Settings, error) {
	if r.db == nil {
		return Settings{}, errors.New("translationconfig: bun repository requires a database")
	}

	var existing settingsModel
	err := r.db.NewSelect().Model(&existing).Where("id = ?", 1).Scan(ctx)
	created := false
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			created = true
		} else {
			return Settings{}, err
		}
	}

	now := time.Now().UTC()
	model := modelFromSettings(settings)
	model.ID = 1
	model.UpdatedAt = now

	if created {
		if _, err := r.db.NewInsert().Model(&model).Exec(ctx); err != nil {
			return Settings{}, err
		}
	} else {
		if _, err := r.db.NewUpdate().
			Model(&model).
			Column("translations_enabled", "require_translations", "updated_at").
			WherePK().
			Exec(ctx); err != nil {
			return Settings{}, err
		}
	}

	stored, err := r.Get(ctx)
	if err != nil {
		return Settings{}, err
	}

	eventType := ChangeUpdated
	if created {
		eventType = ChangeCreated
	}
	r.broadcaster.Broadcast(newChangeEvent(eventType, stored))
	return stored, nil
}

// Delete clears persisted settings.
func (r *BunRepository) Delete(ctx context.Context) error {
	if r.db == nil {
		return errors.New("translationconfig: bun repository requires a database")
	}
	var model settingsModel
	err := r.db.NewSelect().Model(&model).Where("id = ?", 1).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrSettingsNotFound
		}
		return err
	}
	if _, err := r.db.NewDelete().Model(&model).WherePK().Exec(ctx); err != nil {
		return err
	}
	r.broadcaster.Broadcast(newChangeEvent(ChangeDeleted, Settings{}))
	return nil
}

// Subscribe delivers change events until the context is cancelled.
func (r *BunRepository) Subscribe(ctx context.Context) (<-chan ChangeEvent, error) {
	return r.broadcaster.Subscribe(ctx)
}

type settingsModel struct {
	bun.BaseModel `bun:"table:i18n_settings"`

	ID                  int       `bun:",pk"`
	TranslationsEnabled bool      `bun:"translations_enabled"`
	RequireTranslations bool      `bun:"require_translations"`
	UpdatedAt           time.Time `bun:"updated_at"`
}

func modelFromSettings(settings Settings) settingsModel {
	return settingsModel{
		TranslationsEnabled: settings.TranslationsEnabled,
		RequireTranslations: settings.RequireTranslations,
	}
}

func modelToSettings(model *settingsModel) Settings {
	if model == nil {
		return Settings{}
	}
	return Settings{
		TranslationsEnabled: model.TranslationsEnabled,
		RequireTranslations: model.RequireTranslations,
	}
}
