package pages

import "github.com/google/uuid"

type Page struct {
	ID       uuid.UUID
	Slug     string
	ParentID *uuid.UUID
}
