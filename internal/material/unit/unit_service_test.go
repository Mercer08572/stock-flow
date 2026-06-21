package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Mercer08572/stock-flow/internal/material/unit"
)

func TestUnitServiceCreateUnit(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUnitRepository()

	service := unit.NewUnitService(repo)
	got, err := service.Create(ctx, unit.CreateUnitInput{
		Code:      " PCS ",
		Name:      " Pieces ",
		Symbol:    " pcs ",
		UnitType:  unit.UnitType(" count "),
		Precision: 0,
	})
	if err != nil {
		t.Fatalf("create unit: %v", err)
	}

	if got.Code != "PCS" || got.Name != "Pieces" || got.Symbol != "pcs" {
		t.Fatalf("expected trimmed fields, got %#v", got)
	}
	if got.UnitType != unit.UnitTypeCount {
		t.Fatalf("expected count unit type, got %q", got.UnitType)
	}
	if got.Status != unit.StatusActive {
		t.Fatalf("expected default active status, got %q", got.Status)
	}
}

func TestUnitServiceCreateRejectsDuplicateCode(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUnitRepository()
	repo.units[1] = unit.Unit{ID: 1, Code: "PCS"}

	service := unit.NewUnitService(repo)
	_, err := service.Create(ctx, unit.CreateUnitInput{
		Code:      "PCS",
		Name:      "Pieces",
		Symbol:    "pcs",
		UnitType:  unit.UnitTypeCount,
		Precision: 0,
	})

	if !errors.Is(err, unit.ErrDuplicateCode) {
		t.Fatalf("expected duplicate code error, got %v", err)
	}
}

func TestUnitServiceCreateValidatesUnitFields(t *testing.T) {
	ctx := context.Background()
	service := unit.NewUnitService(newFakeUnitRepository())

	_, err := service.Create(ctx, unit.CreateUnitInput{
		Code:      "PCS",
		Name:      "Pieces",
		Symbol:    "pcs",
		UnitType:  unit.UnitTypeCount,
		Precision: 7,
	})

	var validationErr *unit.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationErr.Message != "precision must be between 0 and 6" {
		t.Fatalf("expected precision validation message, got %q", validationErr.Message)
	}
}

type fakeUnitRepository struct {
	units  map[int64]unit.Unit
	nextID int64
}

func newFakeUnitRepository() *fakeUnitRepository {
	return &fakeUnitRepository{
		units:  make(map[int64]unit.Unit),
		nextID: 1,
	}
}

func (r *fakeUnitRepository) ListUnits(_ context.Context, filter unit.UnitListFilter) ([]unit.Unit, error) {
	items := make([]unit.Unit, 0)
	for _, item := range r.units {
		if filter.Status != nil && item.Status != *filter.Status {
			continue
		}
		if filter.UnitType != nil && item.UnitType != *filter.UnitType {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

func (r *fakeUnitRepository) GetUnitByID(_ context.Context, id int64) (*unit.Unit, error) {
	item, ok := r.units[id]
	if !ok {
		return nil, unit.ErrNotFound
	}

	return &item, nil
}

func (r *fakeUnitRepository) CreateUnit(_ context.Context, input unit.CreateUnitInput) (*unit.Unit, error) {
	now := time.Now()
	id := r.nextID
	r.nextID++

	item := unit.Unit{
		ID:        id,
		Code:      input.Code,
		Name:      input.Name,
		Symbol:    input.Symbol,
		UnitType:  input.UnitType,
		Precision: input.Precision,
		Status:    input.Status,
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.units[id] = item

	return &item, nil
}

func (r *fakeUnitRepository) UpdateUnit(_ context.Context, input unit.UpdateUnitInput) (*unit.Unit, error) {
	current, ok := r.units[input.ID]
	if !ok {
		return nil, unit.ErrNotFound
	}

	current.Code = input.Code
	current.Name = input.Name
	current.Symbol = input.Symbol
	current.UnitType = input.UnitType
	current.Precision = input.Precision
	current.Status = input.Status
	current.UpdatedAt = time.Now()
	r.units[input.ID] = current

	return &current, nil
}

func (r *fakeUnitRepository) SoftDeleteUnit(_ context.Context, id int64) error {
	if _, ok := r.units[id]; !ok {
		return unit.ErrNotFound
	}

	delete(r.units, id)
	return nil
}

func (r *fakeUnitRepository) UnitCodeExists(_ context.Context, code string, excludeID int64) (bool, error) {
	for _, item := range r.units {
		if item.Code == code && item.ID != excludeID {
			return true, nil
		}
	}

	return false, nil
}
