package material

import (
	"context"
	"strings"
)

type Service interface {
	List(ctx context.Context, filter ListFilter) (ListResult, error)
	Get(ctx context.Context, id int64) (*Material, error)
	Create(ctx context.Context, input CreateInput) (*Material, error)
	Update(ctx context.Context, input UpdateInput) (*Material, error)
	Delete(ctx context.Context, id int64) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
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

func (s *service) Get(ctx context.Context, id int64) (*Material, error) {
	if id <= 0 {
		return nil, NewValidationError("material id must be greater than zero")
	}

	return s.repo.GetByID(ctx, id)
}

func (s *service) Create(ctx context.Context, input CreateInput) (*Material, error) {
	normalized, err := normalizeCreateInput(input)
	if err != nil {
		return nil, err
	}

	if err := s.validateReferences(ctx, normalized.CategoryID, normalized.BaseUnitID); err != nil {
		return nil, err
	}

	exists, err := s.repo.MaterialCodeExists(ctx, normalized.Code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDuplicateCode
	}

	return s.repo.Create(ctx, normalized)
}

func (s *service) Update(ctx context.Context, input UpdateInput) (*Material, error) {
	normalized, err := normalizeUpdateInput(input)
	if err != nil {
		return nil, err
	}

	if err := s.validateReferences(ctx, normalized.CategoryID, normalized.BaseUnitID); err != nil {
		return nil, err
	}

	exists, err := s.repo.MaterialCodeExists(ctx, normalized.Code, normalized.ID)
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
		return NewValidationError("material id must be greater than zero")
	}

	return s.repo.SoftDelete(ctx, id)
}

func (s *service) validateReferences(ctx context.Context, categoryID int64, baseUnitID int64) error {
	categoryExists, err := s.repo.MaterialCategoryExists(ctx, categoryID)
	if err != nil {
		return err
	}
	if !categoryExists {
		return ErrCategoryNotFound
	}

	unitExists, err := s.repo.UnitExists(ctx, baseUnitID)
	if err != nil {
		return err
	}
	if !unitExists {
		return ErrBaseUnitNotFound
	}

	return nil
}

func normalizeListFilter(filter ListFilter) (ListFilter, error) {
	if filter.Status != nil {
		status := Status(strings.TrimSpace(string(*filter.Status)))
		if !status.IsValid() {
			return ListFilter{}, NewValidationError("status must be active or inactive")
		}
		filter.Status = &status
	}

	if filter.CategoryID != nil && *filter.CategoryID <= 0 {
		return ListFilter{}, NewValidationError("category_id must be greater than zero")
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
	input.Status = Status(strings.TrimSpace(string(input.Status)))
	input.Remark = normalizeRemark(input.Remark)

	if input.Status == "" {
		input.Status = StatusActive
	}

	if err := validateMaterialFields(input.Code, input.Name, input.CategoryID, input.BaseUnitID, input.Status); err != nil {
		return CreateInput{}, err
	}

	return input, nil
}

func normalizeUpdateInput(input UpdateInput) (UpdateInput, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Status = Status(strings.TrimSpace(string(input.Status)))
	input.Remark = normalizeRemark(input.Remark)

	if input.ID <= 0 {
		return UpdateInput{}, NewValidationError("material id must be greater than zero")
	}
	if input.Status == "" {
		return UpdateInput{}, NewValidationError("status is required")
	}
	if err := validateMaterialFields(input.Code, input.Name, input.CategoryID, input.BaseUnitID, input.Status); err != nil {
		return UpdateInput{}, err
	}

	return input, nil
}

func validateMaterialFields(code string, name string, categoryID int64, baseUnitID int64, status Status) error {
	if code == "" {
		return NewValidationError("code is required")
	}
	if name == "" {
		return NewValidationError("name is required")
	}
	if categoryID <= 0 {
		return NewValidationError("category_id must be greater than zero")
	}
	if baseUnitID <= 0 {
		return NewValidationError("base_unit_id must be greater than zero")
	}
	if !status.IsValid() {
		return NewValidationError("status must be active or inactive")
	}

	return nil
}

func normalizeRemark(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}
