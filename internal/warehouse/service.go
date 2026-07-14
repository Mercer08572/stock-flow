package warehouse

import (
	"context"
	"errors"
	"strings"
)

type Service interface {
	List(ctx context.Context, filter ListFilter) (ListResult, error)
	Get(ctx context.Context, id int64) (*Warehouse, error)
	GetReference(ctx context.Context, id int64) (*Reference, error)
	Create(ctx context.Context, input CreateInput) (*Warehouse, error)
	Update(ctx context.Context, input UpdateInput) (*Warehouse, error)
	Delete(ctx context.Context, id int64) error
	Disable(ctx context.Context, id int64) (*Warehouse, error)
}

type Repository interface {
	List(ctx context.Context, filter ListFilter) ([]Warehouse, error)
	GetByID(ctx context.Context, id int64) (*Warehouse, error)
	GetReference(ctx context.Context, id int64) (*Reference, error)
	Create(ctx context.Context, input CreateInput) (*Warehouse, error)
	Update(ctx context.Context, input UpdateInput) (*Warehouse, error)
	SoftDelete(ctx context.Context, id int64) error
	Disable(ctx context.Context, id int64) (*Warehouse, error)
	WarehouseCodeExists(ctx context.Context, code string, excludeID int64) (bool, error)
}

type InventoryReferenceChecker interface {
	HasWarehouseReferences(ctx context.Context, warehouseID int64) (bool, error)
}

type service struct {
	repo             Repository
	referenceChecker InventoryReferenceChecker
}

func NewService(repo Repository, referenceCheckers ...InventoryReferenceChecker) Service {
	var referenceChecker InventoryReferenceChecker
	if len(referenceCheckers) > 0 {
		referenceChecker = referenceCheckers[0]
	}

	return &service{repo: repo, referenceChecker: referenceChecker}
}

func (s *service) List(ctx context.Context, filter ListFilter) (ListResult, error) {
	normalized, err := normalizeListFilter(filter)
	if err != nil {
		return ListResult{}, err
	}

	items, err := s.repo.List(ctx, normalized)
	if err != nil {
		return ListResult{}, err
	}

	return ListResult{
		Items:  items,
		Limit:  normalized.Limit,
		Offset: normalized.Offset,
	}, nil
}

func (s *service) Get(ctx context.Context, id int64) (*Warehouse, error) {
	if id <= 0 {
		return nil, NewValidationError("warehouse id must be greater than zero")
	}

	return s.repo.GetByID(ctx, id)
}

func (s *service) GetReference(ctx context.Context, id int64) (*Reference, error) {
	if id <= 0 {
		return nil, NewValidationError("warehouse id must be greater than zero")
	}

	return s.repo.GetReference(ctx, id)
}

func (s *service) Create(ctx context.Context, input CreateInput) (*Warehouse, error) {
	normalized, err := normalizeCreateInput(input)
	if err != nil {
		return nil, err
	}

	exists, err := s.repo.WarehouseCodeExists(ctx, normalized.Code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDuplicateCode
	}

	return s.repo.Create(ctx, normalized)
}

func (s *service) Update(ctx context.Context, input UpdateInput) (*Warehouse, error) {
	normalized, err := normalizeUpdateInput(input)
	if err != nil {
		return nil, err
	}

	exists, err := s.repo.WarehouseCodeExists(ctx, normalized.Code, normalized.ID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDuplicateCode
	}

	return s.repo.Update(ctx, normalized)
}

func (s *service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return NewValidationError("warehouse id must be greater than zero")
	}
	if s.referenceChecker == nil {
		return errors.New("warehouse inventory reference checker is required")
	}

	referenced, err := s.referenceChecker.HasWarehouseReferences(ctx, id)
	if err != nil {
		return err
	}
	if referenced {
		return ErrReferencedByInventory
	}

	return s.repo.SoftDelete(ctx, id)
}

func (s *service) Disable(ctx context.Context, id int64) (*Warehouse, error) {
	if id <= 0 {
		return nil, NewValidationError("warehouse id must be greater than zero")
	}

	return s.repo.Disable(ctx, id)
}

func normalizeListFilter(filter ListFilter) (ListFilter, error) {
	if filter.Status != nil {
		status := Status(strings.TrimSpace(string(*filter.Status)))
		if !status.IsValid() {
			return ListFilter{}, NewValidationError("status must be active or inactive")
		}
		filter.Status = &status
	}

	if filter.Type != nil {
		warehouseType := Type(strings.TrimSpace(string(*filter.Type)))
		if !warehouseType.IsValid() {
			return ListFilter{}, NewValidationError("type must be normal or virtual")
		}
		filter.Type = &warehouseType
	}

	if filter.Limit <= 0 {
		filter.Limit = DefaultListLimit
	}
	if filter.Limit > MaxListLimit {
		filter.Limit = MaxListLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	return filter, nil
}

func normalizeCreateInput(input CreateInput) (CreateInput, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Type = Type(strings.TrimSpace(string(input.Type)))
	input.Status = Status(strings.TrimSpace(string(input.Status)))
	input.Location = normalizeOptionalText(input.Location)
	input.ContactName = normalizeOptionalText(input.ContactName)
	input.ContactPhone = normalizeOptionalText(input.ContactPhone)
	input.Remark = normalizeOptionalText(input.Remark)

	if input.Type == "" {
		input.Type = TypeNormal
	}
	if input.Status == "" {
		input.Status = StatusActive
	}
	if err := validateWarehouseFields(input.Code, input.Name, input.Type, input.Status); err != nil {
		return CreateInput{}, err
	}

	return input, nil
}

func normalizeUpdateInput(input UpdateInput) (UpdateInput, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Type = Type(strings.TrimSpace(string(input.Type)))
	input.Status = Status(strings.TrimSpace(string(input.Status)))
	input.Location = normalizeOptionalText(input.Location)
	input.ContactName = normalizeOptionalText(input.ContactName)
	input.ContactPhone = normalizeOptionalText(input.ContactPhone)
	input.Remark = normalizeOptionalText(input.Remark)

	if input.ID <= 0 {
		return UpdateInput{}, NewValidationError("warehouse id must be greater than zero")
	}
	if input.Type == "" {
		return UpdateInput{}, NewValidationError("type is required")
	}
	if input.Status == "" {
		return UpdateInput{}, NewValidationError("status is required")
	}
	if err := validateWarehouseFields(input.Code, input.Name, input.Type, input.Status); err != nil {
		return UpdateInput{}, err
	}

	return input, nil
}

func validateWarehouseFields(code string, name string, warehouseType Type, status Status) error {
	if code == "" {
		return NewValidationError("code is required")
	}
	if name == "" {
		return NewValidationError("name is required")
	}
	if !warehouseType.IsValid() {
		return NewValidationError("type must be normal or virtual")
	}
	if !status.IsValid() {
		return NewValidationError("status must be active or inactive")
	}

	return nil
}

func normalizeOptionalText(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}
