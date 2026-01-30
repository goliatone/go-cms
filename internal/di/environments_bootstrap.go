package di

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/internal/identity"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/runtimeconfig"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

const defaultEnvironmentKey = "default"
const defaultEnvironmentID = "00000000-0000-0000-0000-000000000001"

var defaultEnvironmentUUID = uuid.MustParse(defaultEnvironmentID)

type environmentRecord struct {
	bun.BaseModel `bun:"table:environments"`
	ID            uuid.UUID  `bun:",pk,type:uuid"`
	Key           string     `bun:"key"`
	Name          string     `bun:"name"`
	Description   string     `bun:"description"`
	IsActive      bool       `bun:"is_active"`
	IsDefault     bool       `bun:"is_default"`
	CreatedAt     time.Time  `bun:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at"`
	DeletedAt     *time.Time `bun:"deleted_at"`
}

type environmentDefinition struct {
	Key         string
	Name        string
	Description string
	IsActive    bool
	IsDefault   bool
}

func (c *Container) initializeEnvironments(ctx context.Context) error {
	logger := logging.ModuleLogger(c.loggerProvider, "cms.environments")
	if !c.Config.Features.Environments {
		logger.Debug("environments.bootstrap.skip", "reason", "feature_disabled")
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	definitions, defaultKey := resolveEnvironmentDefinitions(c.Config.Environments)
	if len(definitions) == 0 {
		logger.Debug("environments.bootstrap.skip", "reason", "no_definitions")
		return nil
	}

	backend := "memory"
	if c.bunDB != nil {
		backend = "bun"
	}
	logger.Info("environments.bootstrap.start", "count", len(definitions), "default_key", defaultKey, "backend", backend)

	if c.bunDB == nil {
		if c.environmentRepo == nil {
			logger.Debug("environments.bootstrap.skip", "reason", "repository_unavailable")
			return nil
		}
		svc := environments.NewService(c.environmentRepo)
		for _, def := range definitions {
			record, err := svc.GetEnvironmentByKey(ctx, def.Key)
			if err != nil {
				if !errors.Is(err, environments.ErrEnvironmentNotFound) {
					return err
				}
				active := def.IsActive
				desc := strings.TrimSpace(def.Description)
				created, err := svc.CreateEnvironment(ctx, environments.CreateEnvironmentInput{
					Key:         def.Key,
					Name:        def.Name,
					Description: &desc,
					IsActive:    &active,
					IsDefault:   def.IsDefault,
				})
				if err != nil {
					return err
				}
				if created != nil {
					logger.Info("environments.bootstrap.create", "key", created.Key, "id", created.ID, "default", created.IsDefault, "active", created.IsActive)
				}
				continue
			}
			name := def.Name
			desc := strings.TrimSpace(def.Description)
			active := def.IsActive
			update := environments.UpdateEnvironmentInput{
				ID:          record.ID,
				Name:        &name,
				Description: &desc,
				IsActive:    &active,
			}
			if def.IsDefault {
				update.IsDefault = boolPtr(true)
			}
			if _, err := svc.UpdateEnvironment(ctx, update); err != nil {
				return err
			}
			logger.Info("environments.bootstrap.update", "key", def.Key, "default", def.IsDefault, "active", def.IsActive)
		}
		logger.Info("environments.bootstrap.complete", "count", len(definitions), "default_key", defaultKey, "backend", backend)
		return nil
	}

	if defaultKey != "" {
		logger.Debug("environments.bootstrap.clear_default", "default_key", defaultKey)
		if _, err := c.bunDB.NewUpdate().Table("environments").
			Set("is_default = ?", false).
			Where("key <> ?", defaultKey).
			Exec(ctx); err != nil {
			return fmt.Errorf("di: clear environment defaults: %w", err)
		}
	}

	keys := make([]string, 0, len(definitions))
	for _, def := range definitions {
		keys = append(keys, def.Key)
	}

	var existing []environmentRecord
	if err := c.bunDB.NewSelect().
		Model(&existing).
		Column("id", "key").
		Where("key IN (?)", bun.In(keys)).
		Scan(ctx); err != nil {
		return fmt.Errorf("di: list environments: %w", err)
	}

	byKey := make(map[string]environmentRecord, len(existing))
	for _, record := range existing {
		key := normalizeEnvironmentKey(record.Key)
		byKey[key] = record
	}

	now := time.Now().UTC()
	for _, def := range definitions {
		if record, ok := byKey[def.Key]; ok {
			update := environmentRecord{
				ID:          record.ID,
				Key:         def.Key,
				Name:        def.Name,
				Description: def.Description,
				IsActive:    def.IsActive,
				IsDefault:   def.IsDefault,
				UpdatedAt:   now,
				DeletedAt:   nil,
			}
			if _, err := c.bunDB.NewUpdate().
				Model(&update).
				Column("key", "name", "description", "is_active", "is_default", "updated_at", "deleted_at").
				Where("id = ?", record.ID).
				Exec(ctx); err != nil {
				return fmt.Errorf("di: update environment %s: %w", def.Key, err)
			}
			logger.Info("environments.bootstrap.update", "key", def.Key, "default", def.IsDefault, "active", def.IsActive)
			continue
		}

		record := environmentRecord{
			ID:          environmentIDForKey(def.Key),
			Key:         def.Key,
			Name:        def.Name,
			Description: def.Description,
			IsActive:    def.IsActive,
			IsDefault:   def.IsDefault,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if _, err := c.bunDB.NewInsert().Model(&record).Exec(ctx); err != nil {
			return fmt.Errorf("di: insert environment %s: %w", def.Key, err)
		}
		logger.Info("environments.bootstrap.create", "key", def.Key, "id", record.ID, "default", def.IsDefault, "active", def.IsActive)
	}

	if defaultKey != "" {
		logger.Info("environments.bootstrap.set_default", "default_key", defaultKey)
		if _, err := c.bunDB.NewUpdate().Table("environments").
			Set("is_default = ?", true).
			Where("key = ?", defaultKey).
			Exec(ctx); err != nil {
			return fmt.Errorf("di: set default environment: %w", err)
		}
	}

	logger.Info("environments.bootstrap.complete", "count", len(definitions), "default_key", defaultKey, "backend", backend)
	return nil
}

func boolPtr(value bool) *bool {
	return &value
}

func resolveEnvironmentDefinitions(cfg runtimeconfig.EnvironmentsConfig) ([]environmentDefinition, string) {
	if len(cfg.Definitions) == 0 {
		key := normalizeEnvironmentKey(cfg.DefaultKey)
		if key == "" {
			key = defaultEnvironmentKey
		}
		name := deriveEnvironmentName(key)
		return []environmentDefinition{
			{
				Key:         key,
				Name:        name,
				Description: "",
				IsActive:    true,
				IsDefault:   true,
			},
		}, key
	}

	defaultKey := normalizeEnvironmentKey(cfg.DefaultKey)
	if defaultKey == "" {
		for _, def := range cfg.Definitions {
			if def.Default {
				defaultKey = normalizeEnvironmentKey(def.Key)
				break
			}
		}
	}
	if defaultKey == "" && len(cfg.Definitions) == 1 {
		defaultKey = normalizeEnvironmentKey(cfg.Definitions[0].Key)
	}

	definitions := make([]environmentDefinition, 0, len(cfg.Definitions))
	for _, def := range cfg.Definitions {
		key := normalizeEnvironmentKey(def.Key)
		if key == "" {
			continue
		}
		name := strings.TrimSpace(def.Name)
		if name == "" {
			name = deriveEnvironmentName(key)
		}
		definitions = append(definitions, environmentDefinition{
			Key:         key,
			Name:        name,
			Description: strings.TrimSpace(def.Description),
			IsActive:    !def.Disabled,
			IsDefault:   key == defaultKey,
		})
	}
	return definitions, defaultKey
}

func normalizeEnvironmentKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func deriveEnvironmentName(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) == 1 {
		return strings.ToUpper(key)
	}
	return strings.ToUpper(key[:1]) + key[1:]
}

func environmentIDForKey(key string) uuid.UUID {
	if normalizeEnvironmentKey(key) == defaultEnvironmentKey {
		return defaultEnvironmentUUID
	}
	return identity.UUID("go-cms:environment:" + normalizeEnvironmentKey(key))
}
