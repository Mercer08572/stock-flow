package category_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Mercer08572/stock-flow/internal/material/category"
)

func TestCategoryServiceCreateCategory(t *testing.T) {
	ctx := context.Background()
	remark := " raw materials "
	repo := newFakeCategoryRepository()

	service := category.NewCategoryService(repo)
	got, err := service.Create(ctx, category.CreateCategoryInput{
		Code:   " RAW ",
		Name:   " Raw Material ",
		Remark: &remark,
	})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}

	if got.Code != "RAW" || got.Name != "Raw Material" {
		t.Fatalf("expected trimmed fields, got %#v", got)
	}
	if got.Status != category.StatusActive {
		t.Fatalf("expected default active status, got %q", got.Status)
	}
	if got.Remark == nil || *got.Remark != "raw materials" {
		t.Fatalf("expected trimmed remark, got %#v", got.Remark)
	}
}

func TestCategoryServiceCreateValidatesParent(t *testing.T) {
	ctx := context.Background()
	service := category.NewCategoryService(newFakeCategoryRepository())
	parentID := int64(99)

	_, err := service.Create(ctx, category.CreateCategoryInput{
		Code:     "STEEL",
		Name:     "Steel",
		ParentID: &parentID,
	})

	if !errors.Is(err, category.ErrParentNotFound) {
		t.Fatalf("expected parent category not found, got %v", err)
	}
}

func TestCategoryServiceUpdateRejectsSelfParent(t *testing.T) {
	ctx := context.Background()
	service := category.NewCategoryService(newFakeCategoryRepository())
	parentID := int64(1)

	_, err := service.Update(ctx, category.UpdateCategoryInput{
		ID:       1,
		Code:     "RAW",
		Name:     "Raw Material",
		ParentID: &parentID,
		Status:   category.StatusActive,
	})

	var validationErr *category.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationErr.Message != "parent_id must not equal material category id" {
		t.Fatalf("expected self-parent validation message, got %q", validationErr.Message)
	}
}

type fakeCategoryRepository struct {
	categories map[int64]category.Category
	nextID     int64
}

func newFakeCategoryRepository() *fakeCategoryRepository {
	return &fakeCategoryRepository{
		categories: make(map[int64]category.Category),
		nextID:     1,
	}
}

func (r *fakeCategoryRepository) ListCategories(_ context.Context, filter category.CategoryListFilter) ([]category.Category, error) {
	items := make([]category.Category, 0)
	for _, item := range r.categories {
		if filter.Status != nil && item.Status != *filter.Status {
			continue
		}
		if filter.ParentID != nil {
			if item.ParentID == nil || *item.ParentID != *filter.ParentID {
				continue
			}
		}
		items = append(items, item)
	}

	return items, nil
}

func (r *fakeCategoryRepository) GetCategoryByID(_ context.Context, id int64) (*category.Category, error) {
	item, ok := r.categories[id]
	if !ok {
		return nil, category.ErrNotFound
	}

	return &item, nil
}

func (r *fakeCategoryRepository) CreateCategory(_ context.Context, input category.CreateCategoryInput) (*category.Category, error) {
	now := time.Now()
	id := r.nextID
	r.nextID++

	item := category.Category{
		ID:        id,
		Code:      input.Code,
		Name:      input.Name,
		ParentID:  input.ParentID,
		Status:    input.Status,
		Remark:    input.Remark,
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.categories[id] = item

	return &item, nil
}

func (r *fakeCategoryRepository) UpdateCategory(_ context.Context, input category.UpdateCategoryInput) (*category.Category, error) {
	current, ok := r.categories[input.ID]
	if !ok {
		return nil, category.ErrNotFound
	}

	current.Code = input.Code
	current.Name = input.Name
	current.ParentID = input.ParentID
	current.Status = input.Status
	current.Remark = input.Remark
	current.UpdatedAt = time.Now()
	r.categories[input.ID] = current

	return &current, nil
}

func (r *fakeCategoryRepository) SoftDeleteCategory(_ context.Context, id int64) error {
	if _, ok := r.categories[id]; !ok {
		return category.ErrNotFound
	}

	delete(r.categories, id)
	return nil
}

func (r *fakeCategoryRepository) CategoryCodeExists(_ context.Context, code string, excludeID int64) (bool, error) {
	for _, item := range r.categories {
		if item.Code == code && item.ID != excludeID {
			return true, nil
		}
	}

	return false, nil
}

func (r *fakeCategoryRepository) CategoryExists(_ context.Context, id int64) (bool, error) {
	_, ok := r.categories[id]
	return ok, nil
}
