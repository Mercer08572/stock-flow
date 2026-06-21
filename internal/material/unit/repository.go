package unit

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

func NewPostgresRepository(db *pgxpool.Pool) UnitRepository {
	return &postgresRepository{queries: materialdb.New(db)}
}

func (r *postgresRepository) ListUnits(ctx context.Context, filter UnitListFilter) ([]Unit, error) {
	rows, err := r.queries.ListUnits(ctx, materialdb.ListUnitsParams{
		Status:   nullableStatus(filter.Status),
		UnitType: nullableUnitType(filter.UnitType),
		Offset:   filter.Offset,
		Limit:    filter.Limit,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	units := make([]Unit, 0, len(rows))
	for _, row := range rows {
		units = append(units, unitFromListRow(row))
	}

	return units, nil
}

func (r *postgresRepository) GetUnitByID(ctx context.Context, id int64) (*Unit, error) {
	row, err := r.queries.GetUnitByID(ctx, id)
	if err != nil {
		return nil, mapPostgresError(err)
	}

	unit := unitFromGetRow(row)
	return &unit, nil
}

func (r *postgresRepository) CreateUnit(ctx context.Context, input CreateUnitInput) (*Unit, error) {
	row, err := r.queries.CreateUnit(ctx, materialdb.CreateUnitParams{
		Code:      input.Code,
		Name:      input.Name,
		Symbol:    input.Symbol,
		UnitType:  string(input.UnitType),
		Precision: input.Precision,
		Status:    string(input.Status),
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	unit := unitFromCreateRow(row)
	return &unit, nil
}

func (r *postgresRepository) UpdateUnit(ctx context.Context, input UpdateUnitInput) (*Unit, error) {
	row, err := r.queries.UpdateUnit(ctx, materialdb.UpdateUnitParams{
		ID:        input.ID,
		Code:      input.Code,
		Name:      input.Name,
		Symbol:    input.Symbol,
		UnitType:  string(input.UnitType),
		Precision: input.Precision,
		Status:    string(input.Status),
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	unit := unitFromUpdateRow(row)
	return &unit, nil
}

func (r *postgresRepository) SoftDeleteUnit(ctx context.Context, id int64) error {
	rowsAffected, err := r.queries.SoftDeleteUnit(ctx, id)
	if err != nil {
		return mapPostgresError(err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *postgresRepository) UnitCodeExists(ctx context.Context, code string, excludeID int64) (bool, error) {
	exists, err := r.queries.UnitCodeExists(ctx, materialdb.UnitCodeExistsParams{
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
		if pgErr.ConstraintName == "ux_units_code" {
			return ErrDuplicateCode
		}
	case "23503":
		return fmt.Errorf("unit reference constraint failed: %w", err)
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

func nullableUnitType(value *UnitType) *string {
	if value == nil {
		return nil
	}

	unitType := string(*value)
	return &unitType
}

func unitFromListRow(row materialdb.ListUnitsRow) Unit {
	return Unit{
		ID:        row.ID,
		Code:      row.Code,
		Name:      row.Name,
		Symbol:    row.Symbol,
		UnitType:  UnitType(row.UnitType),
		Precision: row.Precision,
		Status:    Status(row.Status),
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

func unitFromGetRow(row materialdb.GetUnitByIDRow) Unit {
	return Unit{
		ID:        row.ID,
		Code:      row.Code,
		Name:      row.Name,
		Symbol:    row.Symbol,
		UnitType:  UnitType(row.UnitType),
		Precision: row.Precision,
		Status:    Status(row.Status),
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

func unitFromCreateRow(row materialdb.CreateUnitRow) Unit {
	return Unit{
		ID:        row.ID,
		Code:      row.Code,
		Name:      row.Name,
		Symbol:    row.Symbol,
		UnitType:  UnitType(row.UnitType),
		Precision: row.Precision,
		Status:    Status(row.Status),
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

func unitFromUpdateRow(row materialdb.UpdateUnitRow) Unit {
	return Unit{
		ID:        row.ID,
		Code:      row.Code,
		Name:      row.Name,
		Symbol:    row.Symbol,
		UnitType:  UnitType(row.UnitType),
		Precision: row.Precision,
		Status:    Status(row.Status),
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}
