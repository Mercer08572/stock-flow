package conversion

import (
	"context"
	"math/big"
	"regexp"
	"strings"
)

var decimalPattern = regexp.MustCompile(`^-?(?:\d+(?:\.\d*)?|\.\d+)$`)

type Service interface {
	List(ctx context.Context, filter ListFilter) (ListResult, error)
	Get(ctx context.Context, materialID int64, id int64) (*MaterialUnitConversion, error)
	Create(ctx context.Context, input CreateInput) (*MaterialUnitConversion, error)
	Update(ctx context.Context, input UpdateInput) (*MaterialUnitConversion, error)
	Delete(ctx context.Context, materialID int64, id int64) error
}

type Repository interface {
	List(ctx context.Context, filter ListFilter) ([]MaterialUnitConversion, error)
	GetByID(ctx context.Context, materialID int64, id int64) (*MaterialUnitConversion, error)
	Create(ctx context.Context, input CreateInput) (*MaterialUnitConversion, error)
	Update(ctx context.Context, input UpdateInput) (*MaterialUnitConversion, error)
	SoftDelete(ctx context.Context, materialID int64, id int64) error
	MaterialExists(ctx context.Context, id int64) (bool, error)
	UnitExists(ctx context.Context, id int64) (bool, error)
	ConversionExists(ctx context.Context, materialID int64, fromUnitID int64, toUnitID int64, excludeID int64) (bool, error)
	ReverseConversionExists(ctx context.Context, materialID int64, fromUnitID int64, toUnitID int64, excludeID int64) (bool, error)
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

	if err := s.validateMaterial(ctx, normalized.MaterialID); err != nil {
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

func (s *service) Get(ctx context.Context, materialID int64, id int64) (*MaterialUnitConversion, error) {
	if materialID <= 0 {
		return nil, NewValidationError("material_id must be greater than zero")
	}
	if id <= 0 {
		return nil, NewValidationError("material unit conversion id must be greater than zero")
	}

	return s.repo.GetByID(ctx, materialID, id)
}

func (s *service) Create(ctx context.Context, input CreateInput) (*MaterialUnitConversion, error) {
	normalized, err := normalizeCreateInput(input)
	if err != nil {
		return nil, err
	}

	if err := s.validateReferences(ctx, normalized.MaterialID, normalized.FromUnitID, normalized.ToUnitID); err != nil {
		return nil, err
	}
	if err := s.validatePairAvailability(ctx, normalized.MaterialID, normalized.FromUnitID, normalized.ToUnitID, 0); err != nil {
		return nil, err
	}

	return s.repo.Create(ctx, normalized)
}

func (s *service) Update(ctx context.Context, input UpdateInput) (*MaterialUnitConversion, error) {
	normalized, err := normalizeUpdateInput(input)
	if err != nil {
		return nil, err
	}

	if err := s.validateReferences(ctx, normalized.MaterialID, normalized.FromUnitID, normalized.ToUnitID); err != nil {
		return nil, err
	}
	if err := s.validatePairAvailability(ctx, normalized.MaterialID, normalized.FromUnitID, normalized.ToUnitID, normalized.ID); err != nil {
		return nil, err
	}

	return s.repo.Update(ctx, normalized)
}

func (s *service) Delete(ctx context.Context, materialID int64, id int64) error {
	if materialID <= 0 {
		return NewValidationError("material_id must be greater than zero")
	}
	if id <= 0 {
		return NewValidationError("material unit conversion id must be greater than zero")
	}

	return s.repo.SoftDelete(ctx, materialID, id)
}

func (s *service) validateReferences(ctx context.Context, materialID int64, fromUnitID int64, toUnitID int64) error {
	if err := s.validateMaterial(ctx, materialID); err != nil {
		return err
	}

	fromExists, err := s.repo.UnitExists(ctx, fromUnitID)
	if err != nil {
		return err
	}
	if !fromExists {
		return ErrFromUnitNotFound
	}

	toExists, err := s.repo.UnitExists(ctx, toUnitID)
	if err != nil {
		return err
	}
	if !toExists {
		return ErrToUnitNotFound
	}

	return nil
}

func (s *service) validateMaterial(ctx context.Context, materialID int64) error {
	exists, err := s.repo.MaterialExists(ctx, materialID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrMaterialNotFound
	}

	return nil
}

func (s *service) validatePairAvailability(ctx context.Context, materialID int64, fromUnitID int64, toUnitID int64, excludeID int64) error {
	exists, err := s.repo.ConversionExists(ctx, materialID, fromUnitID, toUnitID, excludeID)
	if err != nil {
		return err
	}
	if exists {
		return ErrDuplicatePair
	}

	reverseExists, err := s.repo.ReverseConversionExists(ctx, materialID, fromUnitID, toUnitID, excludeID)
	if err != nil {
		return err
	}
	if reverseExists {
		return ErrReversePair
	}

	return nil
}

func normalizeListFilter(filter ListFilter) (ListFilter, error) {
	if filter.MaterialID <= 0 {
		return ListFilter{}, NewValidationError("material_id must be greater than zero")
	}
	if filter.FromUnitID != nil && *filter.FromUnitID <= 0 {
		return ListFilter{}, NewValidationError("from_unit_id must be greater than zero")
	}
	if filter.ToUnitID != nil && *filter.ToUnitID <= 0 {
		return ListFilter{}, NewValidationError("to_unit_id must be greater than zero")
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
	factor, err := normalizeFactor(input.Factor)
	if err != nil {
		return CreateInput{}, err
	}
	input.Factor = factor

	if err := validateConversionFields(input.MaterialID, input.FromUnitID, input.ToUnitID); err != nil {
		return CreateInput{}, err
	}

	return input, nil
}

func normalizeUpdateInput(input UpdateInput) (UpdateInput, error) {
	factor, err := normalizeFactor(input.Factor)
	if err != nil {
		return UpdateInput{}, err
	}
	input.Factor = factor

	if input.ID <= 0 {
		return UpdateInput{}, NewValidationError("material unit conversion id must be greater than zero")
	}
	if err := validateConversionFields(input.MaterialID, input.FromUnitID, input.ToUnitID); err != nil {
		return UpdateInput{}, err
	}

	return input, nil
}

func validateConversionFields(materialID int64, fromUnitID int64, toUnitID int64) error {
	if materialID <= 0 {
		return NewValidationError("material_id must be greater than zero")
	}
	if fromUnitID <= 0 {
		return NewValidationError("from_unit_id must be greater than zero")
	}
	if toUnitID <= 0 {
		return NewValidationError("to_unit_id must be greater than zero")
	}
	if fromUnitID == toUnitID {
		return NewValidationError("from_unit_id and to_unit_id must be different")
	}

	return nil
}

func normalizeFactor(value string) (string, error) {
	factor := strings.TrimSpace(value)
	if factor == "" {
		return "", NewValidationError("factor is required")
	}
	if !decimalPattern.MatchString(factor) {
		return "", NewValidationError("factor must be a decimal number")
	}

	rat := new(big.Rat)
	if _, ok := rat.SetString(factor); !ok {
		return "", NewValidationError("factor must be a decimal number")
	}
	if rat.Sign() <= 0 {
		return "", NewValidationError("factor must be greater than zero")
	}

	return factor, nil
}
