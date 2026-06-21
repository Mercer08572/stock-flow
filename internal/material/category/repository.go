package category

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	materialdb "github.com/Mercer08572/stock-flow/internal/material/db"
)

type postgresRepository struct {
	queries materialdb.Querier
}

func NewPostgresRepository(db *pgxpool.Pool) CategoryRepository {
	return &postgresRepository{queries: materialdb.New(db)}
}

func (r *postgresRepository) ListCategories(ctx context.Context, filter CategoryListFilter) ([]Category, error) {
	rows, err := r.queries.ListMaterialCategories(ctx, materialdb.ListMaterialCategoriesParams{
		Status:   nullableStatus(filter.Status),
		ParentID: filter.ParentID,
		Offset:   filter.Offset,
		Limit:    filter.Limit,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	categories := make([]Category, 0, len(rows))
	for _, row := range rows {
		categories = append(categories, categoryFromListRow(row))
	}

	return categories, nil
}

func (r *postgresRepository) GetCategoryByID(ctx context.Context, id int64) (*Category, error) {
	row, err := r.queries.GetMaterialCategoryByID(ctx, id)
	if err != nil {
		return nil, mapPostgresError(err)
	}

	category := categoryFromGetRow(row)
	return &category, nil
}

func (r *postgresRepository) CreateCategory(ctx context.Context, input CreateCategoryInput) (*Category, error) {
	row, err := r.queries.CreateMaterialCategory(ctx, materialdb.CreateMaterialCategoryParams{
		Code:     input.Code,
		Name:     input.Name,
		ParentID: input.ParentID,
		Status:   string(input.Status),
		Remark:   input.Remark,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	category := categoryFromCreateRow(row)
	return &category, nil
}

func (r *postgresRepository) UpdateCategory(ctx context.Context, input UpdateCategoryInput) (*Category, error) {
	row, err := r.queries.UpdateMaterialCategory(ctx, materialdb.UpdateMaterialCategoryParams{
		ID:       input.ID,
		Code:     input.Code,
		Name:     input.Name,
		ParentID: input.ParentID,
		Status:   string(input.Status),
		Remark:   input.Remark,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	category := categoryFromUpdateRow(row)
	return &category, nil
}

func (r *postgresRepository) SoftDeleteCategory(ctx context.Context, id int64) error {
	rowsAffected, err := r.queries.SoftDeleteMaterialCategory(ctx, id)
	if err != nil {
		return mapPostgresError(err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *postgresRepository) CategoryCodeExists(ctx context.Context, code string, excludeID int64) (bool, error) {
	exists, err := r.queries.MaterialCategoryCodeExists(ctx, materialdb.MaterialCategoryCodeExistsParams{
		Code:      code,
		ExcludeID: excludeID,
	})
	if err != nil {
		return false, mapPostgresError(err)
	}

	return exists, nil
}

func (r *postgresRepository) CategoryExists(ctx context.Context, id int64) (bool, error) {
	exists, err := r.queries.MaterialCategoryExists(ctx, id)
	if err != nil {
		return false, mapPostgresError(err)
	}

	return exists, nil
}

func mapPostgresError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}

	switch pgErr.Code {
	case "23505":
		if pgErr.ConstraintName == "ux_material_categories_code" {
			return ErrDuplicateCode
		}
	case "23503":
		if pgErr.ConstraintName == "material_categories_parent_id_fkey" {
			return ErrParentNotFound
		}
		return fmt.Errorf("material category reference constraint failed: %w", err)
	}

	return err
}

func nullableStatus(value *Status) *string {
	if value == nil {
		return nil
	}

	status := string(*value)
	return &status
}

func categoryFromListRow(row materialdb.ListMaterialCategoriesRow) Category {
	return Category{
		ID:        row.ID,
		Code:      row.Code,
		Name:      row.Name,
		ParentID:  row.ParentID,
		Status:    Status(row.Status),
		Remark:    row.Remark,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

func categoryFromGetRow(row materialdb.GetMaterialCategoryByIDRow) Category {
	return Category{
		ID:        row.ID,
		Code:      row.Code,
		Name:      row.Name,
		ParentID:  row.ParentID,
		Status:    Status(row.Status),
		Remark:    row.Remark,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

func categoryFromCreateRow(row materialdb.CreateMaterialCategoryRow) Category {
	return Category{
		ID:        row.ID,
		Code:      row.Code,
		Name:      row.Name,
		ParentID:  row.ParentID,
		Status:    Status(row.Status),
		Remark:    row.Remark,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

func categoryFromUpdateRow(row materialdb.UpdateMaterialCategoryRow) Category {
	return Category{
		ID:        row.ID,
		Code:      row.Code,
		Name:      row.Name,
		ParentID:  row.ParentID,
		Status:    Status(row.Status),
		Remark:    row.Remark,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}
