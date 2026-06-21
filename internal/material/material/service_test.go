package material_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Mercer08572/stock-flow/internal/material/material"
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

	var validationErr *material.ValidationError
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

type fakeRepository struct {
	materials  map[int64]material.Material
	categories map[int64]bool
	units      map[int64]bool
	nextID     int64
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		materials:  make(map[int64]material.Material),
		categories: make(map[int64]bool),
		units:      make(map[int64]bool),
		nextID:     1,
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
