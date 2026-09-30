package conversion_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Mercer08572/stock-flow/internal/material/conversion"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

func TestServiceCreateConversion(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.materials[10] = true
	repo.units[20] = true
	repo.units[30] = true

	service := conversion.NewService(repo)
	got, err := service.Create(ctx, conversion.CreateInput{
		MaterialID: 10,
		FromUnitID: 30,
		ToUnitID:   20,
		Factor:     " 12.5000 ",
	})
	if err != nil {
		t.Fatalf("create conversion: %v", err)
	}

	if got.MaterialID != 10 || got.FromUnitID != 30 || got.ToUnitID != 20 {
		t.Fatalf("unexpected conversion units: %#v", got)
	}
	if got.Factor != "12.5000" {
		t.Fatalf("expected normalized factor 12.5000, got %q", got.Factor)
	}
}

func TestServiceCreateRejectsDuplicatePair(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.materials[10] = true
	repo.units[20] = true
	repo.units[30] = true
	repo.conversions[1] = conversion.MaterialUnitConversion{
		ID:         1,
		MaterialID: 10,
		FromUnitID: 30,
		ToUnitID:   20,
		Factor:     "12",
	}

	service := conversion.NewService(repo)
	_, err := service.Create(ctx, conversion.CreateInput{
		MaterialID: 10,
		FromUnitID: 30,
		ToUnitID:   20,
		Factor:     "12",
	})

	if !errors.Is(err, conversion.ErrDuplicatePair) {
		t.Fatalf("expected duplicate pair, got %v", err)
	}
}

func TestServiceCreateRejectsReversePair(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.materials[10] = true
	repo.units[20] = true
	repo.units[30] = true
	repo.conversions[1] = conversion.MaterialUnitConversion{
		ID:         1,
		MaterialID: 10,
		FromUnitID: 20,
		ToUnitID:   30,
		Factor:     "0.0833333333",
	}

	service := conversion.NewService(repo)
	_, err := service.Create(ctx, conversion.CreateInput{
		MaterialID: 10,
		FromUnitID: 30,
		ToUnitID:   20,
		Factor:     "12",
	})

	if !errors.Is(err, conversion.ErrReversePair) {
		t.Fatalf("expected reverse pair, got %v", err)
	}
}

func TestServiceCreateValidatesReferences(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.units[20] = true
	repo.units[30] = true

	service := conversion.NewService(repo)
	_, err := service.Create(ctx, conversion.CreateInput{
		MaterialID: 10,
		FromUnitID: 30,
		ToUnitID:   20,
		Factor:     "12",
	})

	if !errors.Is(err, conversion.ErrMaterialNotFound) {
		t.Fatalf("expected material not found, got %v", err)
	}
}

func TestServiceCreateValidatesFactor(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.materials[10] = true
	repo.units[20] = true
	repo.units[30] = true

	service := conversion.NewService(repo)
	_, err := service.Create(ctx, conversion.CreateInput{
		MaterialID: 10,
		FromUnitID: 30,
		ToUnitID:   20,
		Factor:     "0",
	})

	var validationErr *apperr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationErr.Message != "factor must be greater than zero" {
		t.Fatalf("expected factor validation message, got %q", validationErr.Message)
	}
}

func TestServiceListNormalizesFilter(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.materials[10] = true
	repo.conversions[1] = conversion.MaterialUnitConversion{
		ID:         1,
		MaterialID: 10,
		FromUnitID: 30,
		ToUnitID:   20,
		Factor:     "12",
	}
	repo.conversions[2] = conversion.MaterialUnitConversion{
		ID:         2,
		MaterialID: 10,
		FromUnitID: 40,
		ToUnitID:   20,
		Factor:     "24",
	}
	fromUnitID := int64(30)
	toUnitID := int64(20)

	service := conversion.NewService(repo)
	result, err := service.List(ctx, conversion.ListFilter{
		MaterialID: 10,
		FromUnitID: &fromUnitID,
		ToUnitID:   &toUnitID,
		Limit:      999,
		Offset:     -1,
	})
	if err != nil {
		t.Fatalf("list conversions: %v", err)
	}

	if result.Limit != conversion.MaxListLimit {
		t.Fatalf("expected capped limit %d, got %d", conversion.MaxListLimit, result.Limit)
	}
	if result.Offset != 0 {
		t.Fatalf("expected normalized offset 0, got %d", result.Offset)
	}
	if len(result.Items) != 1 || result.Items[0].ID != 1 {
		t.Fatalf("expected filtered conversion only, got %#v", result.Items)
	}
}

func TestServiceDeleteValidatesID(t *testing.T) {
	ctx := context.Background()
	service := conversion.NewService(newFakeRepository())

	err := service.Delete(ctx, 10, 0)

	var validationErr *apperr.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationErr.Message != "material unit conversion id must be greater than zero" {
		t.Fatalf("expected conversion id validation message, got %q", validationErr.Message)
	}
}

type fakeRepository struct {
	materials   map[int64]bool
	units       map[int64]bool
	conversions map[int64]conversion.MaterialUnitConversion
	nextID      int64
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		materials:   make(map[int64]bool),
		units:       make(map[int64]bool),
		conversions: make(map[int64]conversion.MaterialUnitConversion),
		nextID:      1,
	}
}

func (r *fakeRepository) List(_ context.Context, filter conversion.ListFilter) ([]conversion.MaterialUnitConversion, error) {
	items := make([]conversion.MaterialUnitConversion, 0)
	for _, item := range r.conversions {
		if item.MaterialID != filter.MaterialID {
			continue
		}
		if filter.FromUnitID != nil && item.FromUnitID != *filter.FromUnitID {
			continue
		}
		if filter.ToUnitID != nil && item.ToUnitID != *filter.ToUnitID {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

func (r *fakeRepository) GetByID(_ context.Context, materialID int64, id int64) (*conversion.MaterialUnitConversion, error) {
	item, ok := r.conversions[id]
	if !ok || item.MaterialID != materialID {
		return nil, conversion.ErrNotFound
	}

	return &item, nil
}

func (r *fakeRepository) Create(_ context.Context, input conversion.CreateInput) (*conversion.MaterialUnitConversion, error) {
	now := time.Now()
	id := r.nextID
	r.nextID++

	item := conversion.MaterialUnitConversion{
		ID:         id,
		MaterialID: input.MaterialID,
		FromUnitID: input.FromUnitID,
		ToUnitID:   input.ToUnitID,
		Factor:     input.Factor,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	r.conversions[id] = item

	return &item, nil
}

func (r *fakeRepository) Update(_ context.Context, input conversion.UpdateInput) (*conversion.MaterialUnitConversion, error) {
	current, ok := r.conversions[input.ID]
	if !ok || current.MaterialID != input.MaterialID {
		return nil, conversion.ErrNotFound
	}

	current.FromUnitID = input.FromUnitID
	current.ToUnitID = input.ToUnitID
	current.Factor = input.Factor
	current.UpdatedAt = time.Now()
	r.conversions[input.ID] = current

	return &current, nil
}

func (r *fakeRepository) SoftDelete(_ context.Context, materialID int64, id int64) error {
	item, ok := r.conversions[id]
	if !ok || item.MaterialID != materialID {
		return conversion.ErrNotFound
	}

	delete(r.conversions, id)
	return nil
}

func (r *fakeRepository) MaterialExists(_ context.Context, id int64) (bool, error) {
	return r.materials[id], nil
}

func (r *fakeRepository) UnitExists(_ context.Context, id int64) (bool, error) {
	return r.units[id], nil
}

func (r *fakeRepository) ConversionExists(_ context.Context, materialID int64, fromUnitID int64, toUnitID int64, excludeID int64) (bool, error) {
	for _, item := range r.conversions {
		if item.MaterialID == materialID && item.FromUnitID == fromUnitID && item.ToUnitID == toUnitID && item.ID != excludeID {
			return true, nil
		}
	}

	return false, nil
}

func (r *fakeRepository) ReverseConversionExists(_ context.Context, materialID int64, fromUnitID int64, toUnitID int64, excludeID int64) (bool, error) {
	for _, item := range r.conversions {
		if item.MaterialID == materialID && item.FromUnitID == toUnitID && item.ToUnitID == fromUnitID && item.ID != excludeID {
			return true, nil
		}
	}

	return false, nil
}
