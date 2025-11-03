package storageconfig

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/goliatone/go-cms/pkg/storage"
	"github.com/uptrace/bun"
)

// BunRepository persists profiles using a Bun-backed database.
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

// List returns the stored profiles ordered by name.
func (r *BunRepository) List(ctx context.Context) ([]storage.Profile, error) {
	if r.db == nil {
		return nil, errors.New("storageconfig: bun repository requires a database")
	}
	var models []profileModel
	if err := r.db.NewSelect().Model(&models).Order("name ASC").Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]storage.Profile, len(models))
	for i := range models {
		out[i] = modelToProfile(&models[i])
	}
	return out, nil
}

// Get retrieves a profile by name.
func (r *BunRepository) Get(ctx context.Context, name string) (*storage.Profile, error) {
	if r.db == nil {
		return nil, errors.New("storageconfig: bun repository requires a database")
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, ErrProfileNameRequired
	}
	var model profileModel
	err := r.db.NewSelect().Model(&model).Where("name = ?", trimmed).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProfileNotFound
		}
		return nil, err
	}
	profile := modelToProfile(&model)
	return &profile, nil
}

// Upsert creates or updates a profile.
func (r *BunRepository) Upsert(ctx context.Context, profile storage.Profile) (*storage.Profile, error) {
	if r.db == nil {
		return nil, errors.New("storageconfig: bun repository requires a database")
	}
	name := strings.TrimSpace(profile.Name)
	if name == "" {
		return nil, ErrProfileNameRequired
	}
	profile.Name = name

	var existing profileModel
	err := r.db.NewSelect().Model(&existing).Where("name = ?", name).Scan(ctx)
	created := false
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			created = true
		} else {
			return nil, err
		}
	}

	model := modelFromProfile(profile)
	now := time.Now().UTC()
	model.UpdatedAt = now
	if created {
		model.CreatedAt = now
		if _, err := r.db.NewInsert().Model(&model).Exec(ctx); err != nil {
			return nil, err
		}
	} else {
		model.CreatedAt = existing.CreatedAt
		if _, err := r.db.NewUpdate().
			Model(&model).
			Column("description", "provider", "config", "fallbacks", "labels", "is_default", "updated_at").
			WherePK().
			Exec(ctx); err != nil {
			return nil, err
		}
	}

	var stored profileModel
	if err := r.db.NewSelect().Model(&stored).Where("name = ?", name).Scan(ctx); err != nil {
		return nil, err
	}
	out := modelToProfile(&stored)
	eventType := ChangeUpdated
	if created {
		eventType = ChangeCreated
	}
	r.broadcaster.Broadcast(newChangeEvent(eventType, out))
	return &out, nil
}

// Delete removes a profile by name.
func (r *BunRepository) Delete(ctx context.Context, name string) error {
	if r.db == nil {
		return errors.New("storageconfig: bun repository requires a database")
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ErrProfileNameRequired
	}

	var model profileModel
	err := r.db.NewSelect().Model(&model).Where("name = ?", trimmed).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProfileNotFound
		}
		return err
	}
	if _, err := r.db.NewDelete().Model(&model).WherePK().Exec(ctx); err != nil {
		return err
	}

	r.broadcaster.Broadcast(newChangeEvent(ChangeDeleted, modelToProfile(&model)))
	return nil
}

// Subscribe delivers change events until the context is cancelled.
func (r *BunRepository) Subscribe(ctx context.Context) (<-chan ChangeEvent, error) {
	return r.broadcaster.Subscribe(ctx)
}

type profileModel struct {
	bun.BaseModel `bun:"table:storage_profiles"`

	Name        string            `bun:",pk"`
	Description string            `bun:"description"`
	Provider    string            `bun:"provider"`
	Config      storage.Config    `bun:"config,type:jsonb"`
	Fallbacks   []string          `bun:"fallbacks,type:jsonb,nullzero"`
	Labels      map[string]string `bun:"labels,type:jsonb,nullzero"`
	Default     bool              `bun:"is_default"`
	CreatedAt   time.Time         `bun:"created_at"`
	UpdatedAt   time.Time         `bun:"updated_at"`
}

func modelFromProfile(profile storage.Profile) profileModel {
	cloned := cloneProfile(profile)
	if cloned.Fallbacks == nil {
		cloned.Fallbacks = []string{}
	}
	return profileModel{
		Name:        cloned.Name,
		Description: cloned.Description,
		Provider:    cloned.Provider,
		Config:      cloned.Config,
		Fallbacks:   cloned.Fallbacks,
		Labels:      cloned.Labels,
		Default:     cloned.Default,
	}
}

func modelToProfile(model *profileModel) storage.Profile {
	if model == nil {
		return storage.Profile{}
	}
	return cloneProfile(storage.Profile{
		Name:        model.Name,
		Description: model.Description,
		Provider:    model.Provider,
		Config:      model.Config,
		Fallbacks:   model.Fallbacks,
		Labels:      model.Labels,
		Default:     model.Default,
	})
}
