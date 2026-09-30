package sku_test

import (
	"context"
	"errors"
	"testing"
	"time"

	material "github.com/Mercer08572/stock-flow/internal/material/material"
	"github.com/Mercer08572/stock-flow/internal/sku"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

func TestServiceCreateSKU(t *testing.T) {
	ctx := context.Background()
	remark := " "
	repo := newFakeRepository()
	validator := &fakeMaterialValidator{}

	service := sku.NewService(repo, validator)
	got, err := service.Create(ctx, sku.CreateInput{
		MaterialID: 10,
		Code:       " SKU-001 ",
		Name:       " Steel plate pcs ",
		UnitID:     20,
		Remark:     &remark,
	})
	if err != nil {
		t.Fatalf("create sku: %v", err)
	}

	if got.Code != "SKU-001" {
		t.Fatalf("expected trimmed code, got %q", got.Code)
	}
	if got.Name != "Steel plate pcs" {
		t.Fatalf("expected trimmed name, got %q", got.Name)
	}
	if got.Status != sku.StatusActive {
		t.Fatalf("expected default active status, got %q", got.Status)
	}
	if got.Remark != nil {
		t.Fatalf("expected blank remark to become nil, got %#v", got.Remark)
	}
	if len(validator.calls) != 1 || validator.calls[0] != [2]int64{10, 20} {
		t.Fatalf("expected material validator call for material/unit, got %#v", validator.calls)
	}
}

func TestServiceCreateRejectsDuplicateCode(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.skus[1] = sku.SKU{ID: 1, Code: "SKU-001"}

	service := sku.NewService(repo, &fakeMaterialValidator{})
	_, err := service.Create(ctx, sku.CreateInput{
		MaterialID: 10,
		Code:       "SKU-001",
		Name:       "Steel plate pcs",
		UnitID:     20,
	})

	if !errors.Is(err, sku.ErrDuplicateCode) {
		t.Fatalf("expected duplicate code error, got %v", err)
	}
}

func TestServiceCreateRejectsActiveSKUForMaterial(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.skus[1] = sku.SKU{ID: 1, MaterialID: 10, Code: "SKU-001", Status: sku.StatusActive}

	service := sku.NewService(repo, &fakeMaterialValidator{})
	_, err := service.Create(ctx, sku.CreateInput{
		MaterialID: 10,
		Code:       "SKU-002",
		Name:       "Steel plate box",
		UnitID:     20,
	})

	if !errors.Is(err, sku.ErrActiveSKUForMaterial) {
		t.Fatalf("expected active sku conflict, got %v", err)
	}
}

func TestServiceCreateValidatesRequiredFields(t *testing.T) {
	ctx := context.Background()
	service := sku.NewService(newFakeRepository(), &fakeMaterialValidator{})

	_, err := service.Create(ctx, sku.CreateInput{
		Name:       "Steel plate pcs",
		MaterialID: 10,
		UnitID:     20,
	})

	var validationErr *apperr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationErr.Message != "code is required" {
		t.Fatalf("expected code validation message, got %q", validationErr.Message)
	}
}

func TestServiceCreateValidatesMaterialReference(t *testing.T) {
	ctx := context.Background()
	validator := &fakeMaterialValidator{err: material.ErrNotFound}

	service := sku.NewService(newFakeRepository(), validator)
	_, err := service.Create(ctx, sku.CreateInput{
		MaterialID: 10,
		Code:       "SKU-001",
		Name:       "Steel plate pcs",
		UnitID:     20,
	})

	if !errors.Is(err, sku.ErrMaterialNotFound) {
		t.Fatalf("expected material not found, got %v", err)
	}
}

func TestServiceCreateValidatesSKUUnitRule(t *testing.T) {
	ctx := context.Background()
	validator := &fakeMaterialValidator{err: material.ErrSKUUnitNotAllowed}

	service := sku.NewService(newFakeRepository(), validator)
	_, err := service.Create(ctx, sku.CreateInput{
		MaterialID: 10,
		Code:       "SKU-001",
		Name:       "Steel plate pcs",
		UnitID:     20,
	})

	if !errors.Is(err, sku.ErrInvalidUnit) {
		t.Fatalf("expected invalid unit, got %v", err)
	}
}

func TestServiceListNormalizesFilter(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.skus[1] = sku.SKU{ID: 1, MaterialID: 10, Code: "SKU-001", UnitID: 20, Status: sku.StatusActive}
	repo.skus[2] = sku.SKU{ID: 2, MaterialID: 10, Code: "SKU-002", UnitID: 20, Status: sku.StatusInactive}
	rawStatus := sku.Status(" active ")
	materialID := int64(10)
	unitID := int64(20)

	service := sku.NewService(repo, &fakeMaterialValidator{})
	result, err := service.List(ctx, sku.ListFilter{
		Status:     &rawStatus,
		MaterialID: &materialID,
		UnitID:     &unitID,
		Limit:      999,
		Offset:     -1,
	})
	if err != nil {
		t.Fatalf("list skus: %v", err)
	}

	if result.Limit != sku.MaxListLimit {
		t.Fatalf("expected capped limit %d, got %d", sku.MaxListLimit, result.Limit)
	}
	if result.Offset != 0 {
		t.Fatalf("expected normalized offset 0, got %d", result.Offset)
	}
	if len(result.Items) != 1 || result.Items[0].Code != "SKU-001" {
		t.Fatalf("expected active sku only, got %#v", result.Items)
	}
}

func TestServiceDeleteValidatesID(t *testing.T) {
	ctx := context.Background()
	service := sku.NewService(newFakeRepository(), &fakeMaterialValidator{})

	err := service.Delete(ctx, 0)

	var validationErr *apperr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationErr.Message != "sku id must be greater than zero" {
		t.Fatalf("expected sku id validation message, got %q", validationErr.Message)
	}
}

func TestServiceDeleteRejectsInventoryReference(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.skus[1] = sku.SKU{ID: 1, Code: "SKU-001"}
	checker := &fakeInventoryReferenceChecker{skuReferenced: true}

	service := sku.NewService(repo, &fakeMaterialValidator{}, checker)
	err := service.Delete(ctx, 1)

	if !errors.Is(err, sku.ErrReferencedByInventory) {
		t.Fatalf("expected inventory reference conflict, got %v", err)
	}
	if _, exists := repo.skus[1]; !exists {
		t.Fatal("expected referenced sku to remain")
	}
}

func TestServiceDeleteAllowsUnreferencedSKU(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.skus[1] = sku.SKU{ID: 1, Code: "SKU-001"}

	service := sku.NewService(repo, &fakeMaterialValidator{}, &fakeInventoryReferenceChecker{})
	if err := service.Delete(ctx, 1); err != nil {
		t.Fatalf("delete sku: %v", err)
	}
	if _, exists := repo.skus[1]; exists {
		t.Fatal("expected unreferenced sku to be deleted")
	}
}

type fakeRepository struct {
	skus   map[int64]sku.SKU
	nextID int64
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		skus:   make(map[int64]sku.SKU),
		nextID: 1,
	}
}

func (r *fakeRepository) List(_ context.Context, filter sku.ListFilter) ([]sku.SKU, error) {
	items := make([]sku.SKU, 0)
	for _, item := range r.skus {
		if filter.Status != nil && item.Status != *filter.Status {
			continue
		}
		if filter.MaterialID != nil && item.MaterialID != *filter.MaterialID {
			continue
		}
		if filter.UnitID != nil && item.UnitID != *filter.UnitID {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

func (r *fakeRepository) GetByID(_ context.Context, id int64) (*sku.SKU, error) {
	item, ok := r.skus[id]
	if !ok {
		return nil, sku.ErrNotFound
	}

	return &item, nil
}

func (r *fakeRepository) GetReference(_ context.Context, id int64) (*sku.Reference, error) {
	item, ok := r.skus[id]
	if !ok {
		return nil, sku.ErrNotFound
	}
	return &sku.Reference{ID: item.ID, Code: item.Code, Name: item.Name}, nil
}

func (r *fakeRepository) Create(_ context.Context, input sku.CreateInput) (*sku.SKU, error) {
	now := time.Now()
	id := r.nextID
	r.nextID++

	item := sku.SKU{
		ID:         id,
		MaterialID: input.MaterialID,
		Code:       input.Code,
		Name:       input.Name,
		UnitID:     input.UnitID,
		Status:     input.Status,
		Remark:     input.Remark,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	r.skus[id] = item

	return &item, nil
}

func (r *fakeRepository) Update(_ context.Context, input sku.UpdateInput) (*sku.SKU, error) {
	current, ok := r.skus[input.ID]
	if !ok {
		return nil, sku.ErrNotFound
	}

	current.MaterialID = input.MaterialID
	current.Code = input.Code
	current.Name = input.Name
	current.UnitID = input.UnitID
	current.Status = input.Status
	current.Remark = input.Remark
	current.UpdatedAt = time.Now()
	r.skus[input.ID] = current

	return &current, nil
}

func (r *fakeRepository) SoftDelete(_ context.Context, id int64) error {
	if _, ok := r.skus[id]; !ok {
		return sku.ErrNotFound
	}

	delete(r.skus, id)
	return nil
}

func (r *fakeRepository) SKUCodeExists(_ context.Context, code string, excludeID int64) (bool, error) {
	for _, item := range r.skus {
		if item.Code == code && item.ID != excludeID {
			return true, nil
		}
	}

	return false, nil
}

func (r *fakeRepository) ActiveSKUExistsForMaterial(_ context.Context, materialID int64, excludeID int64) (bool, error) {
	for _, item := range r.skus {
		if item.MaterialID == materialID && item.Status == sku.StatusActive && item.ID != excludeID {
			return true, nil
		}
	}

	return false, nil
}

type fakeMaterialValidator struct {
	err   error
	calls [][2]int64
}

type fakeInventoryReferenceChecker struct {
	skuReferenced bool
	err           error
}

func (c *fakeInventoryReferenceChecker) HasSKUReferences(context.Context, int64) (bool, error) {
	return c.skuReferenced, c.err
}

func (v *fakeMaterialValidator) ValidateSKUUnit(_ context.Context, materialID int64, unitID int64) error {
	v.calls = append(v.calls, [2]int64{materialID, unitID})
	return v.err
}
