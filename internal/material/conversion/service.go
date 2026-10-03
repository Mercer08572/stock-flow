package conversion

import (
	"context"
	"errors"
	"math/big"
	"regexp"
	"strings"

	"github.com/Mercer08572/stock-flow/internal/material/material"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

var decimalPattern = regexp.MustCompile(`^-?(?:\d+(?:\.\d*)?|\.\d+)$`)

/** 换算系数列是 NUMERIC(24,10)，取倒数时按 20 位小数格式化以留足精度 */
const factorScaleDigits = 20

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
	MaterialBaseUnitID(ctx context.Context, materialID int64) (int64, error)
	ConversionExists(ctx context.Context, materialID int64, fromUnitID int64, toUnitID int64, excludeID int64) (bool, error)
	ReverseConversionExists(ctx context.Context, materialID int64, fromUnitID int64, toUnitID int64, excludeID int64) (bool, error)
}

/**
 * 单位可公度与「一端必须是物料基础单位」的判定属于物料模块，
 * 通过这个最小契约注入（与 SKU 模块校验 SKU 单位的方式一致）。
 */
type MaterialUnitValidator interface {
	CheckUnitConversion(ctx context.Context, materialID int64, fromUnitID int64, toUnitID int64) (material.UnitConversionCheck, error)
}

type service struct {
	repo      Repository
	validator MaterialUnitValidator
}

func NewService(repo Repository, validator MaterialUnitValidator) Service {
	return &service{repo: repo, validator: validator}
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
		return nil, apperr.NewValidationError("material_id must be greater than zero")
	}
	if id <= 0 {
		return nil, apperr.NewValidationError("material unit conversion id must be greater than zero")
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
	normalized, err = s.normalizeCreateDirection(ctx, normalized)
	if err != nil {
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
	normalized, err = s.normalizeUpdateDirection(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if err := s.validatePairAvailability(ctx, normalized.MaterialID, normalized.FromUnitID, normalized.ToUnitID, normalized.ID); err != nil {
		return nil, err
	}

	return s.repo.Update(ctx, normalized)
}

func (s *service) Delete(ctx context.Context, materialID int64, id int64) error {
	if materialID <= 0 {
		return apperr.NewValidationError("material_id must be greater than zero")
	}
	if id <= 0 {
		return apperr.NewValidationError("material unit conversion id must be greater than zero")
	}

	return s.repo.SoftDelete(ctx, materialID, id)
}

/**
 * 校验一条换算规则。
 *
 * 具体规则（单位存在、类型可公度、至少一端是物料基础单位）由物料模块判定，
 * 这里只把结果映射成本模块的业务错误码。
 *
 * 方向在物料侧已被规范化为「基础单位 → 另一单位」，因此提交的反向对会被
 * 归一化成与已有规则相同的方向，随后由 `validatePairAvailability` 判为重复。
 */
func (s *service) validateReferences(ctx context.Context, materialID int64, fromUnitID int64, toUnitID int64) error {
	if err := s.validateMaterial(ctx, materialID); err != nil {
		return err
	}

	if s.validator == nil {
		return errors.New("material unit validator is required")
	}

	check, err := s.validator.CheckUnitConversion(ctx, materialID, fromUnitID, toUnitID)
	if err != nil {
		return err
	}

	switch check.Status {
	case material.UnitConversionOK:
		return nil
	case material.UnitConversionFromUnitMissing:
		return ErrFromUnitNotFound
	case material.UnitConversionToUnitMissing:
		return ErrToUnitNotFound
	case material.UnitConversionUnitTypeMismatch:
		return ErrUnitTypeMismatch
	case material.UnitConversionBaseUnitRequired:
		return ErrBaseUnitRequired
	default:
		return errors.New("unexpected material unit conversion check status")
	}
}

/**
 * 把换算方向规范化为「物料基础单位 → 另一单位」。
 *
 * 调用前必须先通过 `validateReferences`，因此此处基础单位必然存在。
 * 反向输入（另一单位 → 基础单位）会把系数取倒数，使仓储里同一对单位
 * 只有一种存储方向；顺序输入的系数是精确十进制时，倒数同样精确
 * （例如 0.001 → 1000），无法精确表达时才四舍五入到 20 位小数。
 */
func (s *service) normalizeCreateDirection(ctx context.Context, input CreateInput) (CreateInput, error) {
	inverted, err := s.reversedFactor(ctx, input.MaterialID, input.FromUnitID, input.ToUnitID, input.Factor)
	if err != nil {
		return CreateInput{}, err
	}
	if inverted == nil {
		return input, nil
	}

	input.FromUnitID, input.ToUnitID = input.ToUnitID, input.FromUnitID
	input.Factor = *inverted
	return input, nil
}

func (s *service) normalizeUpdateDirection(ctx context.Context, input UpdateInput) (UpdateInput, error) {
	inverted, err := s.reversedFactor(ctx, input.MaterialID, input.FromUnitID, input.ToUnitID, input.Factor)
	if err != nil {
		return UpdateInput{}, err
	}
	if inverted == nil {
		return input, nil
	}

	input.FromUnitID, input.ToUnitID = input.ToUnitID, input.FromUnitID
	input.Factor = *inverted
	return input, nil
}

/**
 * 方向需要翻转时返回取倒数后的系数，否则返回 nil。
 *
 * 方向规范化为「基础单位 → 另一单位」，使仓储里同一对单位只有一种存储方向。
 */
func (s *service) reversedFactor(ctx context.Context, materialID int64, fromUnitID int64, toUnitID int64, factor string) (*string, error) {
	baseUnitID, err := s.repo.MaterialBaseUnitID(ctx, materialID)
	if err != nil {
		return nil, err
	}

	if fromUnitID == baseUnitID {
		return nil, nil
	}

	inverted, err := invertDecimal(factor)
	if err != nil {
		return nil, err
	}

	return &inverted, nil
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
		return ListFilter{}, apperr.NewValidationError("material_id must be greater than zero")
	}
	if filter.FromUnitID != nil && *filter.FromUnitID <= 0 {
		return ListFilter{}, apperr.NewValidationError("from_unit_id must be greater than zero")
	}
	if filter.ToUnitID != nil && *filter.ToUnitID <= 0 {
		return ListFilter{}, apperr.NewValidationError("to_unit_id must be greater than zero")
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
		return UpdateInput{}, apperr.NewValidationError("material unit conversion id must be greater than zero")
	}
	if err := validateConversionFields(input.MaterialID, input.FromUnitID, input.ToUnitID); err != nil {
		return UpdateInput{}, err
	}

	return input, nil
}

func validateConversionFields(materialID int64, fromUnitID int64, toUnitID int64) error {
	if materialID <= 0 {
		return apperr.NewValidationError("material_id must be greater than zero")
	}
	if fromUnitID <= 0 {
		return apperr.NewValidationError("from_unit_id must be greater than zero")
	}
	if toUnitID <= 0 {
		return apperr.NewValidationError("to_unit_id must be greater than zero")
	}
	if fromUnitID == toUnitID {
		return apperr.NewValidationError("from_unit_id and to_unit_id must be different")
	}

	return nil
}

/**
 * 十进制字符串求倒数。
 *
 * 用 `big.Rat` 保证精确：`0.001` 的倒数是精确的 `1000`，
 * 只有除不尽时（如 `3`）才落到 20 位小数的近似值。
 */
func invertDecimal(value string) (string, error) {
	rat := new(big.Rat)
	if _, ok := rat.SetString(strings.TrimSpace(value)); !ok {
		return "", apperr.NewValidationError("factor must be a decimal number")
	}

	inverse := new(big.Rat).Inv(rat)
	formatted := inverse.FloatString(factorScaleDigits)

	return trimTrailingZeros(formatted), nil
}

func trimTrailingZeros(value string) string {
	if !strings.Contains(value, ".") {
		return value
	}

	trimmed := strings.TrimRight(value, "0")
	return strings.TrimSuffix(trimmed, ".")
}

func normalizeFactor(value string) (string, error) {
	factor := strings.TrimSpace(value)
	if factor == "" {
		return "", apperr.NewValidationError("factor is required")
	}
	if !decimalPattern.MatchString(factor) {
		return "", apperr.NewValidationError("factor must be a decimal number")
	}

	rat := new(big.Rat)
	if _, ok := rat.SetString(factor); !ok {
		return "", apperr.NewValidationError("factor must be a decimal number")
	}
	if rat.Sign() <= 0 {
		return "", apperr.NewValidationError("factor must be greater than zero")
	}

	return factor, nil
}
