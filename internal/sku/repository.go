package sku

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	skudb "github.com/Mercer08572/stock-flow/internal/sku/db"
)

type postgresRepository struct {
	queries skudb.Querier
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{queries: skudb.New(db)}
}

func (r *postgresRepository) List(ctx context.Context, filter ListFilter) ([]SKU, error) {
	rows, err := r.queries.ListSKUs(ctx, skudb.ListSKUsParams{
		Status:     nullableStatus(filter.Status),
		MaterialID: filter.MaterialID,
		UnitID:     filter.UnitID,
		Offset:     filter.Offset,
		Limit:      filter.Limit,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	skus := make([]SKU, 0, len(rows))
	for _, row := range rows {
		skus = append(skus, skuFromListRow(row))
	}

	return skus, nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id int64) (*SKU, error) {
	row, err := r.queries.GetSKUByID(ctx, id)
	if err != nil {
		return nil, mapPostgresError(err)
	}

	item := skuFromGetRow(row)
	return &item, nil
}

func (r *postgresRepository) GetReference(ctx context.Context, id int64) (*Reference, error) {
	row, err := r.queries.GetSKUReference(ctx, id)
	if err != nil {
		return nil, mapPostgresError(err)
	}

	return &Reference{ID: row.ID, Code: row.Code, Name: row.Name, Deleted: row.DeletedAt.Valid}, nil
}

func (r *postgresRepository) Create(ctx context.Context, input CreateInput) (*SKU, error) {
	row, err := r.queries.CreateSKU(ctx, skudb.CreateSKUParams{
		MaterialID: input.MaterialID,
		Code:       input.Code,
		Name:       input.Name,
		UnitID:     input.UnitID,
		Status:     string(input.Status),
		Remark:     input.Remark,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	item := skuFromCreateRow(row)
	return &item, nil
}

func (r *postgresRepository) Update(ctx context.Context, input UpdateInput) (*SKU, error) {
	row, err := r.queries.UpdateSKU(ctx, skudb.UpdateSKUParams{
		ID:         input.ID,
		MaterialID: input.MaterialID,
		Code:       input.Code,
		Name:       input.Name,
		UnitID:     input.UnitID,
		Status:     string(input.Status),
		Remark:     input.Remark,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	item := skuFromUpdateRow(row)
	return &item, nil
}

func (r *postgresRepository) SoftDelete(ctx context.Context, id int64) error {
	rowsAffected, err := r.queries.SoftDeleteSKU(ctx, id)
	if err != nil {
		return mapPostgresError(err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *postgresRepository) SKUCodeExists(ctx context.Context, code string, excludeID int64) (bool, error) {
	exists, err := r.queries.SKUCodeExists(ctx, skudb.SKUCodeExistsParams{
		Code:      code,
		ExcludeID: excludeID,
	})
	if err != nil {
		return false, mapPostgresError(err)
	}

	return exists, nil
}

func (r *postgresRepository) ActiveSKUExistsForMaterial(ctx context.Context, materialID int64, excludeID int64) (bool, error) {
	exists, err := r.queries.ActiveSKUExistsForMaterial(ctx, skudb.ActiveSKUExistsForMaterialParams{
		MaterialID: materialID,
		ExcludeID:  excludeID,
	})
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
		switch pgErr.ConstraintName {
		case "ux_skus_code":
			return ErrDuplicateCode
		case "ux_skus_active_material_current_stage":
			return ErrActiveSKUForMaterial
		}
	case "23503":
		switch pgErr.ConstraintName {
		case "skus_material_id_fkey":
			return ErrMaterialNotFound
		case "skus_unit_id_fkey":
			return ErrUnitNotFound
		}
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

func skuFromListRow(row skudb.ListSKUsRow) SKU {
	return SKU{
		ID:         row.ID,
		MaterialID: row.MaterialID,
		Code:       row.Code,
		Name:       row.Name,
		UnitID:     row.UnitID,
		Status:     Status(row.Status),
		Remark:     row.Remark,
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}
}

func skuFromGetRow(row skudb.GetSKUByIDRow) SKU {
	return SKU{
		ID:         row.ID,
		MaterialID: row.MaterialID,
		Code:       row.Code,
		Name:       row.Name,
		UnitID:     row.UnitID,
		Status:     Status(row.Status),
		Remark:     row.Remark,
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}
}

func skuFromCreateRow(row skudb.CreateSKURow) SKU {
	return SKU{
		ID:         row.ID,
		MaterialID: row.MaterialID,
		Code:       row.Code,
		Name:       row.Name,
		UnitID:     row.UnitID,
		Status:     Status(row.Status),
		Remark:     row.Remark,
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}
}

func skuFromUpdateRow(row skudb.UpdateSKURow) SKU {
	return SKU{
		ID:         row.ID,
		MaterialID: row.MaterialID,
		Code:       row.Code,
		Name:       row.Name,
		UnitID:     row.UnitID,
		Status:     Status(row.Status),
		Remark:     row.Remark,
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}
}
