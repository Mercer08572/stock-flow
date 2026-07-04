package warehouse_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Mercer08572/stock-flow/internal/warehouse"
)

func TestServiceCreateWarehouse(t *testing.T) {
	ctx := context.Background()
	location := " Main warehouse "
	contactName := " "
	repo := newFakeRepository()

	service := warehouse.NewService(repo)
	got, err := service.Create(ctx, warehouse.CreateInput{
		Code:        " WH-001 ",
		Name:        " Main ",
		Location:    &location,
		ContactName: &contactName,
	})
	if err != nil {
		t.Fatalf("create warehouse: %v", err)
	}

	if got.Code != "WH-001" {
		t.Fatalf("expected trimmed code, got %q", got.Code)
	}
	if got.Name != "Main" {
		t.Fatalf("expected trimmed name, got %q", got.Name)
	}
	if got.Type != warehouse.TypeNormal {
		t.Fatalf("expected default type normal, got %q", got.Type)
	}
	if got.Status != warehouse.StatusActive {
		t.Fatalf("expected default status active, got %q", got.Status)
	}
	if got.Location == nil || *got.Location != "Main warehouse" {
		t.Fatalf("expected trimmed location, got %#v", got.Location)
	}
	if got.ContactName != nil {
		t.Fatalf("expected blank contact name to become nil, got %#v", got.ContactName)
	}
}

func TestServiceCreateRejectsDuplicateCode(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.warehouses[1] = warehouse.Warehouse{ID: 1, Code: "WH-001"}

	service := warehouse.NewService(repo)
	_, err := service.Create(ctx, warehouse.CreateInput{
		Code: "WH-001",
		Name: "Main",
	})

	if !errors.Is(err, warehouse.ErrDuplicateCode) {
		t.Fatalf("expected duplicate code error, got %v", err)
	}
}

func TestServiceCreateValidatesRequiredFields(t *testing.T) {
	ctx := context.Background()
	service := warehouse.NewService(newFakeRepository())

	_, err := service.Create(ctx, warehouse.CreateInput{Name: "Main"})

	var validationErr *warehouse.ValidationError
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
	repo.warehouses[1] = warehouse.Warehouse{ID: 1, Code: "WH-001", Type: warehouse.TypeNormal, Status: warehouse.StatusActive}
	repo.warehouses[2] = warehouse.Warehouse{ID: 2, Code: "WH-002", Type: warehouse.TypeVirtual, Status: warehouse.StatusInactive}
	rawStatus := warehouse.Status(" active ")
	rawType := warehouse.Type(" normal ")

	service := warehouse.NewService(repo)
	result, err := service.List(ctx, warehouse.ListFilter{
		Status: &rawStatus,
		Type:   &rawType,
		Limit:  999,
		Offset: -1,
	})
	if err != nil {
		t.Fatalf("list warehouses: %v", err)
	}

	if result.Limit != warehouse.MaxListLimit {
		t.Fatalf("expected capped limit %d, got %d", warehouse.MaxListLimit, result.Limit)
	}
	if result.Offset != 0 {
		t.Fatalf("expected normalized offset 0, got %d", result.Offset)
	}
	if len(result.Items) != 1 || result.Items[0].Code != "WH-001" {
		t.Fatalf("expected active normal warehouse only, got %#v", result.Items)
	}
}

func TestServiceDisableWarehouse(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.warehouses[1] = warehouse.Warehouse{
		ID:     1,
		Code:   "WH-001",
		Name:   "Main",
		Type:   warehouse.TypeNormal,
		Status: warehouse.StatusActive,
	}

	service := warehouse.NewService(repo)
	got, err := service.Disable(ctx, 1)
	if err != nil {
		t.Fatalf("disable warehouse: %v", err)
	}

	if got.Status != warehouse.StatusInactive {
		t.Fatalf("expected inactive warehouse, got %q", got.Status)
	}
	if repo.warehouses[1].Status != warehouse.StatusInactive {
		t.Fatalf("expected repository warehouse to be inactive, got %q", repo.warehouses[1].Status)
	}
}

type fakeRepository struct {
	warehouses map[int64]warehouse.Warehouse
	nextID     int64
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		warehouses: make(map[int64]warehouse.Warehouse),
		nextID:     1,
	}
}

func (r *fakeRepository) List(_ context.Context, filter warehouse.ListFilter) ([]warehouse.Warehouse, error) {
	items := make([]warehouse.Warehouse, 0)
	for _, item := range r.warehouses {
		if filter.Status != nil && item.Status != *filter.Status {
			continue
		}
		if filter.Type != nil && item.Type != *filter.Type {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

func (r *fakeRepository) GetByID(_ context.Context, id int64) (*warehouse.Warehouse, error) {
	item, ok := r.warehouses[id]
	if !ok {
		return nil, warehouse.ErrNotFound
	}

	return &item, nil
}

func (r *fakeRepository) Create(_ context.Context, input warehouse.CreateInput) (*warehouse.Warehouse, error) {
	now := time.Now()
	id := r.nextID
	r.nextID++

	item := warehouse.Warehouse{
		ID:           id,
		Code:         input.Code,
		Name:         input.Name,
		Type:         input.Type,
		Status:       input.Status,
		Location:     input.Location,
		ContactName:  input.ContactName,
		ContactPhone: input.ContactPhone,
		Remark:       input.Remark,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	r.warehouses[id] = item

	return &item, nil
}

func (r *fakeRepository) Update(_ context.Context, input warehouse.UpdateInput) (*warehouse.Warehouse, error) {
	current, ok := r.warehouses[input.ID]
	if !ok {
		return nil, warehouse.ErrNotFound
	}

	current.Code = input.Code
	current.Name = input.Name
	current.Type = input.Type
	current.Status = input.Status
	current.Location = input.Location
	current.ContactName = input.ContactName
	current.ContactPhone = input.ContactPhone
	current.Remark = input.Remark
	current.UpdatedAt = time.Now()
	r.warehouses[input.ID] = current

	return &current, nil
}

func (r *fakeRepository) SoftDelete(_ context.Context, id int64) error {
	if _, ok := r.warehouses[id]; !ok {
		return warehouse.ErrNotFound
	}

	delete(r.warehouses, id)
	return nil
}

func (r *fakeRepository) Disable(_ context.Context, id int64) (*warehouse.Warehouse, error) {
	current, ok := r.warehouses[id]
	if !ok {
		return nil, warehouse.ErrNotFound
	}

	current.Status = warehouse.StatusInactive
	current.UpdatedAt = time.Now()
	r.warehouses[id] = current

	return &current, nil
}

func (r *fakeRepository) WarehouseCodeExists(_ context.Context, code string, excludeID int64) (bool, error) {
	for _, item := range r.warehouses {
		if item.Code == code && item.ID != excludeID {
			return true, nil
		}
	}

	return false, nil
}
