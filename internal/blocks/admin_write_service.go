package blocks

import (
	"context"
	"errors"
	"strings"

	"github.com/goliatone/go-cms/internal/adminreadutil"
	internalcontent "github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

// AdminBlockWriteOption configures the admin block write service.
type AdminBlockWriteOption func(*adminBlockWriteService)

// WithAdminBlockWriteLogger overrides the logger used by the admin block write service.
func WithAdminBlockWriteLogger(logger interfaces.Logger) AdminBlockWriteOption {
	return func(s *adminBlockWriteService) {
		s.logger = logger
	}
}

// NewAdminBlockWriteService constructs the admin block write service.
func NewAdminBlockWriteService(blockSvc Service, contentTypes internalcontent.ContentTypeService, locales internalcontent.LocaleRepository, pageResolver AdminBlockPageResolver, opts ...AdminBlockWriteOption) interfaces.AdminBlockWriteService {
	service := &adminBlockWriteService{
		blocks:       blockSvc,
		contentTypes: contentTypes,
		locales:      locales,
		pageResolver: pageResolver,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

type adminBlockWriteService struct {
	blocks       Service
	contentTypes internalcontent.ContentTypeService
	locales      internalcontent.LocaleRepository
	pageResolver AdminBlockPageResolver
	logger       interfaces.Logger
}

func (s *adminBlockWriteService) CreateDefinition(ctx context.Context, req interfaces.AdminBlockDefinitionCreateRequest) (*interfaces.AdminBlockDefinitionRecord, error) {
	if s == nil || s.blocks == nil {
		return nil, errors.New("blocks: admin write service requires block service")
	}
	input := RegisterDefinitionInput{
		Name:           firstNonEmptyBlockValue(req.Name, req.Type, req.Slug),
		Slug:           firstNonEmptyBlockValue(req.Slug, req.Type),
		Description:    req.Description,
		Icon:           req.Icon,
		Category:       req.Category,
		Status:         strings.TrimSpace(req.Status),
		Schema:         cloneAdminBlockMap(req.Schema),
		UISchema:       cloneAdminBlockMap(req.UISchema),
		EnvironmentKey: strings.TrimSpace(req.EnvironmentKey),
	}
	record, err := s.blocks.RegisterDefinition(ctx, input)
	if err != nil {
		return nil, err
	}
	item := mapAdminBlockDefinition(record, strings.TrimSpace(req.EnvironmentKey))
	return &item, nil
}

func (s *adminBlockWriteService) UpdateDefinition(ctx context.Context, req interfaces.AdminBlockDefinitionUpdateRequest) (*interfaces.AdminBlockDefinitionRecord, error) {
	if s == nil || s.blocks == nil {
		return nil, errors.New("blocks: admin write service requires block service")
	}
	input := UpdateDefinitionInput{
		ID:             req.ID,
		Name:           req.Name,
		Slug:           resolveBlockDefinitionSlugUpdate(req.Slug, req.Type),
		Description:    req.Description,
		Icon:           req.Icon,
		Category:       req.Category,
		Status:         req.Status,
		Schema:         cloneAdminBlockMap(req.Schema),
		UISchema:       cloneAdminBlockMap(req.UISchema),
		EnvironmentKey: normalizeOptionalString(req.EnvironmentKey),
	}
	record, err := s.blocks.UpdateDefinition(ctx, input)
	if err != nil {
		return nil, err
	}
	item := mapAdminBlockDefinition(record, strings.TrimSpace(req.EnvironmentKey))
	return &item, nil
}

func (s *adminBlockWriteService) DeleteDefinition(ctx context.Context, req interfaces.AdminBlockDefinitionDeleteRequest) error {
	if s == nil || s.blocks == nil {
		return errors.New("blocks: admin write service requires block service")
	}
	return s.blocks.DeleteDefinition(ctx, DeleteDefinitionRequest{
		ID:         req.ID,
		HardDelete: req.HardDelete,
	})
}

func (s *adminBlockWriteService) SaveBlock(ctx context.Context, req interfaces.AdminBlockSaveRequest) (*interfaces.AdminBlockRecord, error) {
	if s == nil || s.blocks == nil {
		return nil, errors.New("blocks: admin write service requires block service")
	}
	if req.ContentID == uuid.Nil {
		return nil, internalcontent.ErrContentIDRequired
	}
	if req.DefinitionID == uuid.Nil {
		return nil, ErrInstanceDefinitionRequired
	}
	pageIDs, err := s.resolvePageIDs(ctx, req.ContentID, "")
	if err != nil {
		return nil, err
	}
	if len(pageIDs) == 0 {
		return nil, &internalcontent.NotFoundError{Resource: "page", Key: req.ContentID.String()}
	}
	var pageID *uuid.UUID
	if len(pageIDs) > 0 && pageIDs[0] != uuid.Nil {
		pageID = &pageIDs[0]
	}
	definition, err := s.blocks.GetDefinition(ctx, req.DefinitionID)
	if err != nil {
		return nil, err
	}
	if req.ID == uuid.Nil {
		instance, err := s.blocks.CreateInstance(ctx, CreateInstanceInput{
			DefinitionID:  req.DefinitionID,
			PageID:        pageID,
			Region:        strings.TrimSpace(req.Region),
			Position:      req.Position,
			Configuration: cloneAdminBlockMap(req.Data),
			CreatedBy:     req.CreatedBy,
			UpdatedBy:     pickAdminBlockActor(req.UpdatedBy, req.CreatedBy),
		})
		if err != nil {
			return nil, err
		}
		if err := s.upsertBlockTranslation(ctx, instance.ID, req, false); err != nil {
			return nil, err
		}
		record := s.mapInstanceRecord(ctx, instance, definition, req)
		return &record, nil
	}

	updateReq := UpdateInstanceInput{
		InstanceID:    req.ID,
		Configuration: cloneAdminBlockMap(req.Data),
		UpdatedBy:     req.UpdatedBy,
	}
	if pageID != nil {
		updateReq.PageID = pageID
	}
	if region := strings.TrimSpace(req.Region); region != "" {
		updateReq.Region = &region
	}
	if req.Position >= 0 {
		position := req.Position
		updateReq.Position = &position
	}
	instance, err := s.blocks.UpdateInstance(ctx, updateReq)
	if err != nil {
		return nil, err
	}
	if err := s.upsertBlockTranslation(ctx, instance.ID, req, true); err != nil {
		return nil, err
	}
	record := s.mapInstanceRecord(ctx, instance, definition, req)
	return &record, nil
}

func (s *adminBlockWriteService) DeleteBlock(ctx context.Context, req interfaces.AdminBlockDeleteRequest) error {
	if s == nil || s.blocks == nil {
		return errors.New("blocks: admin write service requires block service")
	}
	return s.blocks.DeleteInstance(ctx, DeleteInstanceRequest{
		ID:         req.ID,
		DeletedBy:  req.DeletedBy,
		HardDelete: req.HardDelete,
	})
}

func (s *adminBlockWriteService) upsertBlockTranslation(ctx context.Context, instanceID uuid.UUID, req interfaces.AdminBlockSaveRequest, allowCreate bool) error {
	localeID, err := s.resolveLocaleID(ctx, req.Locale)
	if err != nil || localeID == uuid.Nil {
		return err
	}
	updateReq := UpdateTranslationInput{
		BlockInstanceID: instanceID,
		LocaleID:        localeID,
		Content:         cloneAdminBlockMap(req.Data),
		UpdatedBy:       pickAdminBlockActor(req.UpdatedBy, req.CreatedBy),
	}
	if _, err := s.blocks.UpdateTranslation(ctx, updateReq); err == nil {
		return nil
	} else if !allowCreate || !errors.Is(err, ErrTranslationNotFound) {
		return err
	}
	_, err = s.blocks.AddTranslation(ctx, AddTranslationInput{
		BlockInstanceID: instanceID,
		LocaleID:        localeID,
		Content:         cloneAdminBlockMap(req.Data),
	})
	return err
}

func (s *adminBlockWriteService) mapInstanceRecord(ctx context.Context, instance *Instance, definition *Definition, req interfaces.AdminBlockSaveRequest) interfaces.AdminBlockRecord {
	status := firstNonEmptyBlockValue(req.Status, instanceStatus(instance))
	return interfaces.AdminBlockRecord{
		ID:             instance.ID,
		DefinitionID:   instance.DefinitionID,
		ContentID:      req.ContentID,
		Region:         firstNonEmptyBlockValue(req.Region, instance.Region),
		Locale:         strings.TrimSpace(req.Locale),
		Status:         status,
		Data:           cloneAdminBlockMap(req.Data),
		Position:       req.Position,
		BlockType:      blockDefinitionType(definition),
		BlockSchemaKey: firstNonEmptyBlockValue(definition.SchemaVersion, definition.Slug),
	}
}

func (s *adminBlockWriteService) resolvePageIDs(ctx context.Context, contentID uuid.UUID, envKey string) ([]uuid.UUID, error) {
	if s == nil || s.pageResolver == nil || contentID == uuid.Nil {
		return nil, nil
	}
	return s.pageResolver(ctx, contentID, envKey)
}

func (s *adminBlockWriteService) resolveLocaleID(ctx context.Context, locale string) (uuid.UUID, error) {
	normalized := adminreadutil.NormalizeLocale(locale)
	if normalized == "" {
		return uuid.Nil, nil
	}
	if s == nil || s.locales == nil {
		return uuid.Nil, nil
	}
	record, err := s.locales.GetByCode(ctx, normalized)
	if err != nil {
		return uuid.Nil, err
	}
	if record == nil {
		return uuid.Nil, internalcontent.ErrUnknownLocale
	}
	return record.ID, nil
}

func resolveBlockDefinitionSlugUpdate(slug *string, typ *string) *string {
	if slug != nil {
		return slug
	}
	if typ != nil {
		value := strings.TrimSpace(*typ)
		return &value
	}
	return nil
}

func normalizeOptionalString(value string) *string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return &trimmed
	}
	return nil
}

func pickAdminBlockActor(values ...uuid.UUID) uuid.UUID {
	for _, value := range values {
		if value != uuid.Nil {
			return value
		}
	}
	return uuid.Nil
}
