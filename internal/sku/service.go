package sku

import (
	"context"
	"errors"
	"strings"

	material "github.com/Mercer08572/stock-flow/internal/material/material"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

type Service interface {
	List(ctx context.Context, filter ListFilter) (ListResult, error)
	Get(ctx context.Context, id int64) (*SKU, error)
	GetReference(ctx context.Context, id int64) (*Reference, error)
	Create(ctx context.Context, input CreateInput) (*SKU, error)
	Update(ctx context.Context, input UpdateInput) (*SKU, error)
	Delete(ctx context.Context, id int64) error
}

type Repository interface {
	List(ctx context.Context, filter ListFilter) ([]SKU, error)
	GetByID(ctx context.Context, id int64) (*SKU, error)
	GetReference(ctx context.Context, id int64) (*Reference, error)
	Create(ctx context.Context, input CreateInput) (*SKU, error)
	Update(ctx context.Context, input UpdateInput) (*SKU, error)
	SoftDelete(ctx context.Context, id int64) error
	SKUCodeExists(ctx context.Context, code string, excludeID int64) (bool, error)
	ActiveSKUExistsForMaterial(ctx context.Context, materialID int64, excludeID int64) (bool, error)
}

type MaterialValidator interface {
	ValidateSKUUnit(ctx context.Context, materialID int64, unitID int64) error
}

type InventoryReferenceChecker interface {
	HasSKUReferences(ctx context.Context, skuID int64) (bool, error)
}

type service struct {
	repo              Repository
	materialValidator MaterialValidator
	referenceChecker  InventoryReferenceChecker
}

func NewService(repo Repository, materialValidator MaterialValidator, referenceCheckers ...InventoryReferenceChecker) Service {
	var referenceChecker InventoryReferenceChecker
	if len(referenceCheckers) > 0 {
		referenceChecker = referenceCheckers[0]
	}

	return &service{
		repo:              repo,
		materialValidator: materialValidator,
		referenceChecker:  referenceChecker,
	}
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

func (s *service) Get(ctx context.Context, id int64) (*SKU, error) {
	if id <= 0 {
		return nil, apperr.NewValidationError("sku id must be greater than zero")
	}

	return s.repo.GetByID(ctx, id)
}

func (s *service) GetReference(ctx context.Context, id int64) (*Reference, error) {
	if id <= 0 {
		return nil, apperr.NewValidationError("sku id must be greater than zero")
	}

	return s.repo.GetReference(ctx, id)
}

func (s *service) Create(ctx context.Context, input CreateInput) (*SKU, error) {
	normalized, err := normalizeCreateInput(input)
	if err != nil {
		return nil, err
	}

	if err := s.validateReferences(ctx, normalized.MaterialID, normalized.UnitID); err != nil {
		return nil, err
	}

	exists, err := s.repo.SKUCodeExists(ctx, normalized.Code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDuplicateCode
	}

	if normalized.Status == StatusActive {
		exists, err := s.repo.ActiveSKUExistsForMaterial(ctx, normalized.MaterialID, 0)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrActiveSKUForMaterial
		}
	}

	return s.repo.Create(ctx, normalized)
}

func (s *service) Update(ctx context.Context, input UpdateInput) (*SKU, error) {
	normalized, err := normalizeUpdateInput(input)
	if err != nil {
		return nil, err
	}

	if err := s.validateReferences(ctx, normalized.MaterialID, normalized.UnitID); err != nil {
		return nil, err
	}

	exists, err := s.repo.SKUCodeExists(ctx, normalized.Code, normalized.ID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDuplicateCode
	}

	if normalized.Status == StatusActive {
		exists, err := s.repo.ActiveSKUExistsForMaterial(ctx, normalized.MaterialID, normalized.ID)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrActiveSKUForMaterial
		}
	}

	return s.repo.Update(ctx, normalized)
}

func (s *service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return apperr.NewValidationError("sku id must be greater than zero")
	}
	if s.referenceChecker == nil {
		return errors.New("sku inventory reference checker is required")
	}

	referenced, err := s.referenceChecker.HasSKUReferences(ctx, id)
	if err != nil {
		return err
	}
	if referenced {
		return ErrReferencedByInventory
	}

	return s.repo.SoftDelete(ctx, id)
}

func (s *service) validateReferences(ctx context.Context, materialID int64, unitID int64) error {
	if s.materialValidator == nil {
		return errors.New("sku material validator is required")
	}

	err := s.materialValidator.ValidateSKUUnit(ctx, materialID, unitID)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, material.ErrNotFound):
		return ErrMaterialNotFound
	case errors.Is(err, material.ErrBaseUnitNotFound):
		return ErrUnitNotFound
	case errors.Is(err, material.ErrSKUUnitNotAllowed):
		return ErrInvalidUnit
	case apperr.IsValidationError(err):
		return apperr.NewValidationError(err.Error())
	default:
		return err
	}
}

func normalizeListFilter(filter ListFilter) (ListFilter, error) {
	if filter.Status != nil {
		status := Status(strings.TrimSpace(string(*filter.Status)))
		if !status.IsValid() {
			return ListFilter{}, apperr.NewValidationError("status must be active or inactive")
		}
		filter.Status = &status
	}

	if filter.MaterialID != nil && *filter.MaterialID <= 0 {
		return ListFilter{}, apperr.NewValidationError("material_id must be greater than zero")
	}
	if filter.UnitID != nil && *filter.UnitID <= 0 {
		return ListFilter{}, apperr.NewValidationError("unit_id must be greater than zero")
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
	input.Remark = normalizeOptionalText(input.Remark)

	if input.Status == "" {
		input.Status = StatusActive
	}
	if err := validateSKUFields(input.Code, input.Name, input.MaterialID, input.UnitID, input.Status); err != nil {
		return CreateInput{}, err
	}

	return input, nil
}

func normalizeUpdateInput(input UpdateInput) (UpdateInput, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Status = Status(strings.TrimSpace(string(input.Status)))
	input.Remark = normalizeOptionalText(input.Remark)

	if input.ID <= 0 {
		return UpdateInput{}, apperr.NewValidationError("sku id must be greater than zero")
	}
	if input.Status == "" {
		return UpdateInput{}, apperr.NewValidationError("status is required")
	}
	if err := validateSKUFields(input.Code, input.Name, input.MaterialID, input.UnitID, input.Status); err != nil {
		return UpdateInput{}, err
	}

	return input, nil
}

func validateSKUFields(code string, name string, materialID int64, unitID int64, status Status) error {
	if code == "" {
		return apperr.NewValidationError("code is required")
	}
	if name == "" {
		return apperr.NewValidationError("name is required")
	}
	if materialID <= 0 {
		return apperr.NewValidationError("material_id must be greater than zero")
	}
	if unitID <= 0 {
		return apperr.NewValidationError("unit_id must be greater than zero")
	}
	if !status.IsValid() {
		return apperr.NewValidationError("status must be active or inactive")
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
