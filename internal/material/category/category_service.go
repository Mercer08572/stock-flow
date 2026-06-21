package category

import (
	"context"
	"strings"
)

type CategoryService interface {
	List(ctx context.Context, filter CategoryListFilter) (CategoryListResult, error)
	Get(ctx context.Context, id int64) (*Category, error)
	Create(ctx context.Context, input CreateCategoryInput) (*Category, error)
	Update(ctx context.Context, input UpdateCategoryInput) (*Category, error)
	Delete(ctx context.Context, id int64) error
}

type CategoryRepository interface {
	ListCategories(ctx context.Context, filter CategoryListFilter) ([]Category, error)
	GetCategoryByID(ctx context.Context, id int64) (*Category, error)
	CreateCategory(ctx context.Context, input CreateCategoryInput) (*Category, error)
	UpdateCategory(ctx context.Context, input UpdateCategoryInput) (*Category, error)
	SoftDeleteCategory(ctx context.Context, id int64) error
	CategoryCodeExists(ctx context.Context, code string, excludeID int64) (bool, error)
	CategoryExists(ctx context.Context, id int64) (bool, error)
}

type categoryService struct {
	repo CategoryRepository
}

func NewCategoryService(repo CategoryRepository) CategoryService {
	return &categoryService{repo: repo}
}

func (s *categoryService) List(ctx context.Context, filter CategoryListFilter) (CategoryListResult, error) {
	normalized, err := normalizeCategoryListFilter(filter)
	if err != nil {
		return CategoryListResult{}, err
	}

	items, err := s.repo.ListCategories(ctx, normalized)
	if err != nil {
		return CategoryListResult{}, err
	}

	return CategoryListResult{
		Items:  items,
		Limit:  normalized.Limit,
		Offset: normalized.Offset,
	}, nil
}

func (s *categoryService) Get(ctx context.Context, id int64) (*Category, error) {
	if id <= 0 {
		return nil, NewValidationError("material category id must be greater than zero")
	}

	return s.repo.GetCategoryByID(ctx, id)
}

func (s *categoryService) Create(ctx context.Context, input CreateCategoryInput) (*Category, error) {
	normalized, err := normalizeCreateCategoryInput(input)
	if err != nil {
		return nil, err
	}

	if err := s.validateParent(ctx, 0, normalized.ParentID); err != nil {
		return nil, err
	}

	exists, err := s.repo.CategoryCodeExists(ctx, normalized.Code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDuplicateCode
	}

	return s.repo.CreateCategory(ctx, normalized)
}

func (s *categoryService) Update(ctx context.Context, input UpdateCategoryInput) (*Category, error) {
	normalized, err := normalizeUpdateCategoryInput(input)
	if err != nil {
		return nil, err
	}

	if err := s.validateParent(ctx, normalized.ID, normalized.ParentID); err != nil {
		return nil, err
	}

	exists, err := s.repo.CategoryCodeExists(ctx, normalized.Code, normalized.ID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDuplicateCode
	}

	return s.repo.UpdateCategory(ctx, normalized)
}

func (s *categoryService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return NewValidationError("material category id must be greater than zero")
	}

	return s.repo.SoftDeleteCategory(ctx, id)
}

func (s *categoryService) validateParent(ctx context.Context, categoryID int64, parentID *int64) error {
	if parentID == nil {
		return nil
	}
	if *parentID <= 0 {
		return NewValidationError("parent_id must be greater than zero")
	}
	if categoryID > 0 && *parentID == categoryID {
		return NewValidationError("parent_id must not equal material category id")
	}

	exists, err := s.repo.CategoryExists(ctx, *parentID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrParentNotFound
	}

	return nil
}

func normalizeCategoryListFilter(filter CategoryListFilter) (CategoryListFilter, error) {
	if filter.Status != nil {
		status := Status(strings.TrimSpace(string(*filter.Status)))
		if !status.IsValid() {
			return CategoryListFilter{}, NewValidationError("status must be active or inactive")
		}
		filter.Status = &status
	}

	if filter.ParentID != nil && *filter.ParentID <= 0 {
		return CategoryListFilter{}, NewValidationError("parent_id must be greater than zero")
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

func normalizeCreateCategoryInput(input CreateCategoryInput) (CreateCategoryInput, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Status = Status(strings.TrimSpace(string(input.Status)))
	input.Remark = normalizeRemark(input.Remark)

	if input.Status == "" {
		input.Status = StatusActive
	}
	if err := validateCategoryFields(input.Code, input.Name, input.Status); err != nil {
		return CreateCategoryInput{}, err
	}

	return input, nil
}

func normalizeUpdateCategoryInput(input UpdateCategoryInput) (UpdateCategoryInput, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Status = Status(strings.TrimSpace(string(input.Status)))
	input.Remark = normalizeRemark(input.Remark)

	if input.ID <= 0 {
		return UpdateCategoryInput{}, NewValidationError("material category id must be greater than zero")
	}
	if input.Status == "" {
		return UpdateCategoryInput{}, NewValidationError("status is required")
	}
	if err := validateCategoryFields(input.Code, input.Name, input.Status); err != nil {
		return UpdateCategoryInput{}, err
	}

	return input, nil
}

func validateCategoryFields(code string, name string, status Status) error {
	if code == "" {
		return NewValidationError("code is required")
	}
	if name == "" {
		return NewValidationError("name is required")
	}
	if !status.IsValid() {
		return NewValidationError("status must be active or inactive")
	}

	return nil
}

func normalizeRemark(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}
