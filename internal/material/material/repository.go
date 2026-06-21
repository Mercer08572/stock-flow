package material

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	materialdb "github.com/Mercer08572/stock-flow/internal/material/db"
)

type Repository interface {
	List(ctx context.Context, filter ListFilter) ([]Material, error)
	GetByID(ctx context.Context, id int64) (*Material, error)
	Create(ctx context.Context, input CreateInput) (*Material, error)
	Update(ctx context.Context, input UpdateInput) (*Material, error)
	SoftDelete(ctx context.Context, id int64) error
	MaterialCodeExists(ctx context.Context, code string, excludeID int64) (bool, error)
	MaterialCategoryExists(ctx context.Context, id int64) (bool, error)
	UnitExists(ctx context.Context, id int64) (bool, error)
}

type postgresRepository struct {
	queries materialdb.Querier
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{queries: materialdb.New(db)}
}

func (r *postgresRepository) List(ctx context.Context, filter ListFilter) ([]Material, error) {
	rows, err := r.queries.ListMaterials(ctx, materialdb.ListMaterialsParams{
		Status:     nullableStatus(filter.Status),
		CategoryID: filter.CategoryID,
		Offset:     filter.Offset,
		Limit:      filter.Limit,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	materials := make([]Material, 0, len(rows))
	for _, row := range rows {
		materials = append(materials, materialFromListRow(row))
	}

	return materials, nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id int64) (*Material, error) {
	row, err := r.queries.GetMaterialByID(ctx, id)
	if err != nil {
		return nil, mapPostgresError(err)
	}

	material := materialFromGetRow(row)
	return &material, nil
}

func (r *postgresRepository) Create(ctx context.Context, input CreateInput) (*Material, error) {
	row, err := r.queries.CreateMaterial(ctx, materialdb.CreateMaterialParams{
		Code:       input.Code,
		Name:       input.Name,
		CategoryID: input.CategoryID,
		BaseUnitID: input.BaseUnitID,
		Status:     string(input.Status),
		Remark:     input.Remark,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	material := materialFromCreateRow(row)
	return &material, nil
}

func (r *postgresRepository) Update(ctx context.Context, input UpdateInput) (*Material, error) {
	row, err := r.queries.UpdateMaterial(ctx, materialdb.UpdateMaterialParams{
		ID:         input.ID,
		Code:       input.Code,
		Name:       input.Name,
		CategoryID: input.CategoryID,
		BaseUnitID: input.BaseUnitID,
		Status:     string(input.Status),
		Remark:     input.Remark,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	material := materialFromUpdateRow(row)
	return &material, nil
}

func (r *postgresRepository) SoftDelete(ctx context.Context, id int64) error {
	rowsAffected, err := r.queries.SoftDeleteMaterial(ctx, id)
	if err != nil {
		return mapPostgresError(err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *postgresRepository) MaterialCodeExists(ctx context.Context, code string, excludeID int64) (bool, error) {
	exists, err := r.queries.MaterialCodeExists(ctx, materialdb.MaterialCodeExistsParams{
		Code:      code,
		ExcludeID: excludeID,
	})
	if err != nil {
		return false, mapPostgresError(err)
	}

	return exists, nil
}

func (r *postgresRepository) MaterialCategoryExists(ctx context.Context, id int64) (bool, error) {
	exists, err := r.queries.MaterialCategoryExists(ctx, id)
	if err != nil {
		return false, mapPostgresError(err)
	}

	return exists, nil
}

func (r *postgresRepository) UnitExists(ctx context.Context, id int64) (bool, error) {
	exists, err := r.queries.UnitExists(ctx, id)
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
		if pgErr.ConstraintName == "ux_materials_code" {
			return ErrDuplicateCode
		}
	case "23503":
		return fmt.Errorf("material reference constraint failed: %w", err)
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

func materialFromListRow(row materialdb.ListMaterialsRow) Material {
	return Material{
		ID:         row.ID,
		Code:       row.Code,
		Name:       row.Name,
		CategoryID: row.CategoryID,
		BaseUnitID: row.BaseUnitID,
		Status:     Status(row.Status),
		Remark:     row.Remark,
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}
}

func materialFromGetRow(row materialdb.GetMaterialByIDRow) Material {
	return Material{
		ID:         row.ID,
		Code:       row.Code,
		Name:       row.Name,
		CategoryID: row.CategoryID,
		BaseUnitID: row.BaseUnitID,
		Status:     Status(row.Status),
		Remark:     row.Remark,
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}
}

func materialFromCreateRow(row materialdb.CreateMaterialRow) Material {
	return Material{
		ID:         row.ID,
		Code:       row.Code,
		Name:       row.Name,
		CategoryID: row.CategoryID,
		BaseUnitID: row.BaseUnitID,
		Status:     Status(row.Status),
		Remark:     row.Remark,
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}
}

func materialFromUpdateRow(row materialdb.UpdateMaterialRow) Material {
	return Material{
		ID:         row.ID,
		Code:       row.Code,
		Name:       row.Name,
		CategoryID: row.CategoryID,
		BaseUnitID: row.BaseUnitID,
		Status:     Status(row.Status),
		Remark:     row.Remark,
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}
}
