package cms

import (
	"context"
	"errors"

	"github.com/goliatone/go-cms/internal/menus"
	"github.com/google/uuid"
)

// MenuInfo is a stable public view of a menu record.
type MenuInfo struct {
	ID          uuid.UUID
	Code        string
	Description *string
}

var errNilModule = errors.New("cms: module is nil")

// GetOrCreateMenu returns a stable menu record, creating it when missing.
func (m *Module) GetOrCreateMenu(ctx context.Context, code string, description *string, actor uuid.UUID) (*MenuInfo, error) {
	if m == nil || m.container == nil {
		return nil, errNilModule
	}

	record, err := m.container.MenuService().GetOrCreateMenu(ctx, menus.CreateMenuInput{
		Code:        code,
		Description: description,
		CreatedBy:   actor,
		UpdatedBy:   actor,
	})
	if err != nil {
		return nil, err
	}

	return &MenuInfo{
		ID:          record.ID,
		Code:        record.Code,
		Description: record.Description,
	}, nil
}

