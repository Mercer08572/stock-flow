package warehouse

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	warehousedb "github.com/Mercer08572/stock-flow/internal/warehouse/db"
)

type postgresRepository struct {
	queries warehousedb.Querier
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{queries: warehousedb.New(db)}
}

func (r *postgresRepository) List(ctx context.Context, filter ListFilter) ([]Warehouse, error) {
	rows, err := r.queries.ListWarehouses(ctx, warehousedb.ListWarehousesParams{
		Status: nullableStatus(filter.Status),
		Type:   nullableType(filter.Type),
		Offset: filter.Offset,
		Limit:  filter.Limit,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	warehouses := make([]Warehouse, 0, len(rows))
	for _, row := range rows {
		warehouses = append(warehouses, warehouseFromListRow(row))
	}

	return warehouses, nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id int64) (*Warehouse, error) {
	row, err := r.queries.GetWarehouseByID(ctx, id)
	if err != nil {
		return nil, mapPostgresError(err)
	}

	warehouse := warehouseFromGetRow(row)
	return &warehouse, nil
}

func (r *postgresRepository) Create(ctx context.Context, input CreateInput) (*Warehouse, error) {
	row, err := r.queries.CreateWarehouse(ctx, warehousedb.CreateWarehouseParams{
		Code:         input.Code,
		Name:         input.Name,
		Type:         string(input.Type),
		Status:       string(input.Status),
		Location:     input.Location,
		ContactName:  input.ContactName,
		ContactPhone: input.ContactPhone,
		Remark:       input.Remark,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	warehouse := warehouseFromCreateRow(row)
	return &warehouse, nil
}

func (r *postgresRepository) Update(ctx context.Context, input UpdateInput) (*Warehouse, error) {
	row, err := r.queries.UpdateWarehouse(ctx, warehousedb.UpdateWarehouseParams{
		ID:           input.ID,
		Code:         input.Code,
		Name:         input.Name,
		Type:         string(input.Type),
		Status:       string(input.Status),
		Location:     input.Location,
		ContactName:  input.ContactName,
		ContactPhone: input.ContactPhone,
		Remark:       input.Remark,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	warehouse := warehouseFromUpdateRow(row)
	return &warehouse, nil
}

func (r *postgresRepository) SoftDelete(ctx context.Context, id int64) error {
	rowsAffected, err := r.queries.SoftDeleteWarehouse(ctx, id)
	if err != nil {
		return mapPostgresError(err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *postgresRepository) Disable(ctx context.Context, id int64) (*Warehouse, error) {
	row, err := r.queries.DisableWarehouse(ctx, id)
	if err != nil {
		return nil, mapPostgresError(err)
	}

	warehouse := warehouseFromDisableRow(row)
	return &warehouse, nil
}

func (r *postgresRepository) WarehouseCodeExists(ctx context.Context, code string, excludeID int64) (bool, error) {
	exists, err := r.queries.WarehouseCodeExists(ctx, warehousedb.WarehouseCodeExistsParams{
		Code:      code,
		ExcludeID: excludeID,
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
		if pgErr.ConstraintName == "ux_warehouses_code" {
			return ErrDuplicateCode
		}
	case "23503":
		return fmt.Errorf("warehouse reference constraint failed: %w", err)
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

func nullableType(value *Type) *string {
	if value == nil {
		return nil
	}

	warehouseType := string(*value)
	return &warehouseType
}

func warehouseFromListRow(row warehousedb.ListWarehousesRow) Warehouse {
	return Warehouse{
		ID:           row.ID,
		Code:         row.Code,
		Name:         row.Name,
		Type:         Type(row.Type),
		Status:       Status(row.Status),
		Location:     row.Location,
		ContactName:  row.ContactName,
		ContactPhone: row.ContactPhone,
		Remark:       row.Remark,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

func warehouseFromGetRow(row warehousedb.GetWarehouseByIDRow) Warehouse {
	return Warehouse{
		ID:           row.ID,
		Code:         row.Code,
		Name:         row.Name,
		Type:         Type(row.Type),
		Status:       Status(row.Status),
		Location:     row.Location,
		ContactName:  row.ContactName,
		ContactPhone: row.ContactPhone,
		Remark:       row.Remark,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

func warehouseFromCreateRow(row warehousedb.CreateWarehouseRow) Warehouse {
	return Warehouse{
		ID:           row.ID,
		Code:         row.Code,
		Name:         row.Name,
		Type:         Type(row.Type),
		Status:       Status(row.Status),
		Location:     row.Location,
		ContactName:  row.ContactName,
		ContactPhone: row.ContactPhone,
		Remark:       row.Remark,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

func warehouseFromUpdateRow(row warehousedb.UpdateWarehouseRow) Warehouse {
	return Warehouse{
		ID:           row.ID,
		Code:         row.Code,
		Name:         row.Name,
		Type:         Type(row.Type),
		Status:       Status(row.Status),
		Location:     row.Location,
		ContactName:  row.ContactName,
		ContactPhone: row.ContactPhone,
		Remark:       row.Remark,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

func warehouseFromDisableRow(row warehousedb.DisableWarehouseRow) Warehouse {
	return Warehouse{
		ID:           row.ID,
		Code:         row.Code,
		Name:         row.Name,
		Type:         Type(row.Type),
		Status:       Status(row.Status),
		Location:     row.Location,
		ContactName:  row.ContactName,
		ContactPhone: row.ContactPhone,
		Remark:       row.Remark,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}
