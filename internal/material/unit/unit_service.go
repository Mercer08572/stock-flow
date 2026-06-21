package unit

import (
	"context"
	"strings"
)

type UnitService interface {
	List(ctx context.Context, filter UnitListFilter) (UnitListResult, error)
	Get(ctx context.Context, id int64) (*Unit, error)
	Create(ctx context.Context, input CreateUnitInput) (*Unit, error)
	Update(ctx context.Context, input UpdateUnitInput) (*Unit, error)
	Delete(ctx context.Context, id int64) error
}

type UnitRepository interface {
	ListUnits(ctx context.Context, filter UnitListFilter) ([]Unit, error)
	GetUnitByID(ctx context.Context, id int64) (*Unit, error)
	CreateUnit(ctx context.Context, input CreateUnitInput) (*Unit, error)
	UpdateUnit(ctx context.Context, input UpdateUnitInput) (*Unit, error)
	SoftDeleteUnit(ctx context.Context, id int64) error
	UnitCodeExists(ctx context.Context, code string, excludeID int64) (bool, error)
}

type unitService struct {
	repo UnitRepository
}

func NewUnitService(repo UnitRepository) UnitService {
	return &unitService{repo: repo}
}

func (s *unitService) List(ctx context.Context, filter UnitListFilter) (UnitListResult, error) {
	normalized, err := normalizeUnitListFilter(filter)
	if err != nil {
		return UnitListResult{}, err
	}

	items, err := s.repo.ListUnits(ctx, normalized)
	if err != nil {
		return UnitListResult{}, err
	}

	return UnitListResult{
		Items:  items,
		Limit:  normalized.Limit,
		Offset: normalized.Offset,
	}, nil
}

func (s *unitService) Get(ctx context.Context, id int64) (*Unit, error) {
	if id <= 0 {
		return nil, NewValidationError("unit id must be greater than zero")
	}

	return s.repo.GetUnitByID(ctx, id)
}

func (s *unitService) Create(ctx context.Context, input CreateUnitInput) (*Unit, error) {
	normalized, err := normalizeCreateUnitInput(input)
	if err != nil {
		return nil, err
	}

	exists, err := s.repo.UnitCodeExists(ctx, normalized.Code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDuplicateCode
	}

	return s.repo.CreateUnit(ctx, normalized)
}

func (s *unitService) Update(ctx context.Context, input UpdateUnitInput) (*Unit, error) {
	normalized, err := normalizeUpdateUnitInput(input)
	if err != nil {
		return nil, err
	}

	exists, err := s.repo.UnitCodeExists(ctx, normalized.Code, normalized.ID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDuplicateCode
	}

	return s.repo.UpdateUnit(ctx, normalized)
}

func (s *unitService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return NewValidationError("unit id must be greater than zero")
	}

	return s.repo.SoftDeleteUnit(ctx, id)
}

func normalizeUnitListFilter(filter UnitListFilter) (UnitListFilter, error) {
	if filter.Status != nil {
		status := Status(strings.TrimSpace(string(*filter.Status)))
		if !status.IsValid() {
			return UnitListFilter{}, NewValidationError("status must be active or inactive")
		}
		filter.Status = &status
	}

	if filter.UnitType != nil {
		unitType := UnitType(strings.TrimSpace(string(*filter.UnitType)))
		if !unitType.IsValid() {
			return UnitListFilter{}, NewValidationError("unit_type is invalid")
		}
		filter.UnitType = &unitType
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

func normalizeCreateUnitInput(input CreateUnitInput) (CreateUnitInput, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Symbol = strings.TrimSpace(input.Symbol)
	input.UnitType = UnitType(strings.TrimSpace(string(input.UnitType)))
	input.Status = Status(strings.TrimSpace(string(input.Status)))

	if input.Status == "" {
		input.Status = StatusActive
	}

	if err := validateUnitFields(input.Code, input.Name, input.Symbol, input.UnitType, input.Precision, input.Status); err != nil {
		return CreateUnitInput{}, err
	}

	return input, nil
}

func normalizeUpdateUnitInput(input UpdateUnitInput) (UpdateUnitInput, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Symbol = strings.TrimSpace(input.Symbol)
	input.UnitType = UnitType(strings.TrimSpace(string(input.UnitType)))
	input.Status = Status(strings.TrimSpace(string(input.Status)))

	if input.ID <= 0 {
		return UpdateUnitInput{}, NewValidationError("unit id must be greater than zero")
	}
	if input.Status == "" {
		return UpdateUnitInput{}, NewValidationError("status is required")
	}
	if err := validateUnitFields(input.Code, input.Name, input.Symbol, input.UnitType, input.Precision, input.Status); err != nil {
		return UpdateUnitInput{}, err
	}

	return input, nil
}

func validateUnitFields(code string, name string, symbol string, unitType UnitType, precision int32, status Status) error {
	if code == "" {
		return NewValidationError("code is required")
	}
	if name == "" {
		return NewValidationError("name is required")
	}
	if symbol == "" {
		return NewValidationError("symbol is required")
	}
	if !unitType.IsValid() {
		return NewValidationError("unit_type is invalid")
	}
	if precision < 0 || precision > 6 {
		return NewValidationError("precision must be between 0 and 6")
	}
	if !status.IsValid() {
		return NewValidationError("status must be active or inactive")
	}

	return nil
}
