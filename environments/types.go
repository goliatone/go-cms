package environments

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Environment defines a scoped workspace for content and configuration.
type Environment struct {
	bun.BaseModel `bun:"table:environments,alias:e"`

	ID          uuid.UUID  `bun:",pk,type:uuid" json:"id"`
	Key         string     `bun:"key,notnull" json:"key"`
	Name        string     `bun:"name,notnull" json:"name"`
	Description *string    `bun:"description" json:"description,omitempty"`
	IsActive    bool       `bun:"is_active,notnull,default:true" json:"is_active"`
	IsDefault   bool       `bun:"is_default,notnull,default:false" json:"is_default"`
	CreatedAt   time.Time  `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt   time.Time  `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
	DeletedAt   *time.Time `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
}
