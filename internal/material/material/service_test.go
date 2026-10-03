package material_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Mercer08572/stock-flow/internal/material/material"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

func TestServiceCreateMaterial(t *testing.T) {
	ctx := context.Background()
	remark := " note "
	repo := newFakeRepository()
	repo.categories[10] = true
	repo.units[20] = true

	service := material.NewService(repo)
	got, err := service.Create(ctx, material.CreateInput{
		Code:       " M-001 ",
		Name:       " Steel plate ",
		CategoryID: 10,
		BaseUnitID: 20,
		Remark:     &remark,
	})
	if err != nil {
		t.Fatalf("create material: %v", err)
	}

	if got.Code != "M-001" {
		t.Fatalf("expected trimmed code, got %q", got.Code)
	}
	if got.Name != "Steel plate" {
		t.Fatalf("expected trimmed name, got %q", got.Name)
	}
	if got.Status != material.StatusActive {
		t.Fatalf("expected default active status, got %q", got.Status)
	}
	if got.Remark == nil || *got.Remark != "note" {
		t.Fatalf("expected trimmed remark, got %#v", got.Remark)
	}
}

func TestServiceCreateRejectsDuplicateCode(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.categories[10] = true
	repo.units[20] = true
	repo.materials[1] = material.Material{ID: 1, Code: "M-001"}

	service := material.NewService(repo)
	_, err := service.Create(ctx, material.CreateInput{
		Code:       "M-001",
		Name:       "Steel plate",
		CategoryID: 10,
		BaseUnitID: 20,
	})

	if !errors.Is(err, material.ErrDuplicateCode) {
		t.Fatalf("expected duplicate code error, got %v", err)
	}
}

func TestServiceCreateValidatesReferences(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.units[20] = true

	service := material.NewService(repo)
	_, err := service.Create(ctx, material.CreateInput{
		Code:       "M-001",
		Name:       "Steel plate",
		CategoryID: 10,
		BaseUnitID: 20,
	})

	if !errors.Is(err, material.ErrCategoryNotFound) {
		t.Fatalf("expected category not found, got %v", err)
	}
}

func TestServiceCreateValidatesRequiredFields(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()

	service := material.NewService(repo)
	_, err := service.Create(ctx, material.CreateInput{
		Name:       "Steel plate",
		CategoryID: 10,
		BaseUnitID: 20,
	})

	var validationErr *apperr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationErr.Message != "code is required" {
		t.Fatalf("expected code validation message, got %q", validationErr.Message)
	}
}

func TestServiceListNormalizesFilter(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.materials[1] = material.Material{ID: 1, Code: "M-001", Status: material.StatusActive, CategoryID: 10}
	repo.materials[2] = material.Material{ID: 2, Code: "M-002", Status: material.StatusInactive, CategoryID: 10}
	rawStatus := material.Status(" active ")

	service := material.NewService(repo)
	result, err := service.List(ctx, material.ListFilter{
		Status: &rawStatus,
		Limit:  999,
		Offset: -1,
	})
	if err != nil {
		t.Fatalf("list materials: %v", err)
	}

	if result.Limit != material.MaxListLimit {
		t.Fatalf("expected capped limit %d, got %d", material.MaxListLimit, result.Limit)
	}
	if result.Offset != 0 {
		t.Fatalf("expected normalized offset 0, got %d", result.Offset)
	}
	if len(result.Items) != 1 || result.Items[0].Code != "M-001" {
		t.Fatalf("expected active material only, got %#v", result.Items)
	}
}

func TestServiceValidateSKUUnitAllowsBaseUnit(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.units[20] = true
	repo.materials[1] = material.Material{ID: 1, BaseUnitID: 20}

	service := material.NewService(repo)
	if err := service.ValidateSKUUnit(ctx, 1, 20); err != nil {
		t.Fatalf("validate sku unit: %v", err)
	}
}

func TestServiceValidateSKUUnitAllowsConvertibleUnit(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.units[20] = true
	repo.units[30] = true
	repo.materials[1] = material.Material{ID: 1, BaseUnitID: 20}
	repo.allowedSKUUnits[[2]int64{1, 30}] = true

	service := material.NewService(repo)
	if err := service.ValidateSKUUnit(ctx, 1, 30); err != nil {
		t.Fatalf("validate sku unit: %v", err)
	}
}

func TestServiceValidateSKUUnitRejectsUnrelatedUnit(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.units[20] = true
	repo.units[30] = true
	repo.materials[1] = material.Material{ID: 1, BaseUnitID: 20}

	service := material.NewService(repo)
	err := service.ValidateSKUUnit(ctx, 1, 30)

	if !errors.Is(err, material.ErrSKUUnitNotAllowed) {
		t.Fatalf("expected sku unit not allowed, got %v", err)
	}
}

type fakeRepository struct {
	materials       map[int64]material.Material
	categories      map[int64]bool
	units           map[int64]bool
	unitTypes       map[int64]string
	allowedSKUUnits map[[2]int64]bool
	nextID          int64
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		materials:       make(map[int64]material.Material),
		categories:      make(map[int64]bool),
		units:           make(map[int64]bool),
		unitTypes:       make(map[int64]string),
		allowedSKUUnits: make(map[[2]int64]bool),
		nextID:          1,
	}
}

func (r *fakeRepository) List(_ context.Context, filter material.ListFilter) ([]material.Material, error) {
	items := make([]material.Material, 0)
	for _, item := range r.materials {
		if filter.Status != nil && item.Status != *filter.Status {
			continue
		}
		if filter.CategoryID != nil && item.CategoryID != *filter.CategoryID {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

func (r *fakeRepository) GetByID(_ context.Context, id int64) (*material.Material, error) {
	item, ok := r.materials[id]
	if !ok {
		return nil, material.ErrNotFound
	}

	return &item, nil
}

func (r *fakeRepository) Create(_ context.Context, input material.CreateInput) (*material.Material, error) {
	now := time.Now()
	id := r.nextID
	r.nextID++

	item := material.Material{
		ID:         id,
		Code:       input.Code,
		Name:       input.Name,
		CategoryID: input.CategoryID,
		BaseUnitID: input.BaseUnitID,
		Status:     input.Status,
		Remark:     input.Remark,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	r.materials[id] = item

	return &item, nil
}

func (r *fakeRepository) Update(_ context.Context, input material.UpdateInput) (*material.Material, error) {
	current, ok := r.materials[input.ID]
	if !ok {
		return nil, material.ErrNotFound
	}

	current.Code = input.Code
	current.Name = input.Name
	current.CategoryID = input.CategoryID
	current.BaseUnitID = input.BaseUnitID
	current.Status = input.Status
	current.Remark = input.Remark
	current.UpdatedAt = time.Now()
	r.materials[input.ID] = current

	return &current, nil
}

func (r *fakeRepository) SoftDelete(_ context.Context, id int64) error {
	if _, ok := r.materials[id]; !ok {
		return material.ErrNotFound
	}

	delete(r.materials, id)
	return nil
}

func (r *fakeRepository) MaterialCodeExists(_ context.Context, code string, excludeID int64) (bool, error) {
	for _, item := range r.materials {
		if item.Code == code && item.ID != excludeID {
			return true, nil
		}
	}

	return false, nil
}

func (r *fakeRepository) MaterialCategoryExists(_ context.Context, id int64) (bool, error) {
	return r.categories[id], nil
}

func (r *fakeRepository) UnitExists(_ context.Context, id int64) (bool, error) {
	return r.units[id], nil
}

func (r *fakeRepository) UnitTypes(_ context.Context, unitIDs []int64) (map[int64]string, error) {
	types := make(map[int64]string, len(unitIDs))
	for _, id := range unitIDs {
		if !r.units[id] {
			continue
		}
		types[id] = r.unitType(id)
	}

	return types, nil
}

func (r *fakeRepository) MaterialBaseUnitID(_ context.Context, materialID int64) (int64, error) {
	item, ok := r.materials[materialID]
	if !ok {
		return 0, material.ErrNotFound
	}

	return item.BaseUnitID, nil
}

/** 未显式登记类型的单位按重量处理：既有用例只关心「存在 + 同一类型」 */
func (r *fakeRepository) unitType(id int64) string {
	if unitType, ok := r.unitTypes[id]; ok {
		return unitType
	}

	return material.UnitTypeNameWeight
}

func (r *fakeRepository) MaterialSKUUnitAllowed(_ context.Context, materialID int64, unitID int64) (bool, error) {
	return r.allowedSKUUnits[[2]int64{materialID, unitID}], nil
}

func TestServiceCheckUnitConversionRejectsMismatchedTypes(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.materials[1] = material.Material{ID: 1, BaseUnitID: 20}
	repo.units[20] = true
	repo.units[30] = true
	repo.unitTypes[20] = material.UnitTypeNameWeight
	repo.unitTypes[30] = material.UnitTypeNameCount

	service := material.NewService(repo)
	check, err := service.CheckUnitConversion(ctx, 1, 20, 30)
	if err != nil {
		t.Fatalf("check unit conversion: %v", err)
	}

	if check.Status != material.UnitConversionUnitTypeMismatch {
		t.Fatalf("expected unit type mismatch, got %q", check.Status)
	}
}

func TestServiceCheckUnitConversionRequiresBaseUnit(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.materials[1] = material.Material{ID: 1, BaseUnitID: 20}
	repo.units[20] = true
	repo.units[30] = true
	repo.units[40] = true

	service := material.NewService(repo)
	check, err := service.CheckUnitConversion(ctx, 1, 30, 40)
	if err != nil {
		t.Fatalf("check unit conversion: %v", err)
	}

	if check.Status != material.UnitConversionBaseUnitRequired {
		t.Fatalf("expected base unit required, got %q", check.Status)
	}
}

func TestServiceCheckUnitConversionAllowsPackageCountAgainstBaseUnit(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.materials[1] = material.Material{ID: 1, BaseUnitID: 20}
	repo.units[20] = true
	repo.units[30] = true
	repo.unitTypes[20] = material.UnitTypeNameCount
	repo.unitTypes[30] = material.UnitTypeNamePackage

	service := material.NewService(repo)
	check, err := service.CheckUnitConversion(ctx, 1, 20, 30)
	if err != nil {
		t.Fatalf("check unit conversion: %v", err)
	}

	if !check.IsOK() {
		t.Fatalf("expected ok for 1 box = 12 pcs, got %q", check.Status)
	}
}

func TestServiceCheckUnitConversionReportsMissingUnits(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.materials[1] = material.Material{ID: 1, BaseUnitID: 20}
	repo.units[20] = true

	service := material.NewService(repo)

	missingTo, err := service.CheckUnitConversion(ctx, 1, 20, 30)
	if err != nil {
		t.Fatalf("check unit conversion: %v", err)
	}
	if missingTo.Status != material.UnitConversionToUnitMissing {
		t.Fatalf("expected to unit missing, got %q", missingTo.Status)
	}

	missingFrom, err := service.CheckUnitConversion(ctx, 1, 30, 20)
	if err != nil {
		t.Fatalf("check unit conversion: %v", err)
	}
	if missingFrom.Status != material.UnitConversionFromUnitMissing {
		t.Fatalf("expected from unit missing, got %q", missingFrom.Status)
	}
}
