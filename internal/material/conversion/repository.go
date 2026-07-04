package conversion

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	materialdb "github.com/Mercer08572/stock-flow/internal/material/db"
)

type postgresRepository struct {
	queries materialdb.Querier
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{queries: materialdb.New(db)}
}

func (r *postgresRepository) List(ctx context.Context, filter ListFilter) ([]MaterialUnitConversion, error) {
	rows, err := r.queries.ListMaterialUnitConversions(ctx, materialdb.ListMaterialUnitConversionsParams{
		MaterialID: filter.MaterialID,
		FromUnitID: filter.FromUnitID,
		ToUnitID:   filter.ToUnitID,
		Offset:     filter.Offset,
		Limit:      filter.Limit,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	conversions := make([]MaterialUnitConversion, 0, len(rows))
	for _, row := range rows {
		conversion, err := conversionFromListRow(row)
		if err != nil {
			return nil, err
		}
		conversions = append(conversions, conversion)
	}

	return conversions, nil
}

func (r *postgresRepository) GetByID(ctx context.Context, materialID int64, id int64) (*MaterialUnitConversion, error) {
	row, err := r.queries.GetMaterialUnitConversionByID(ctx, materialdb.GetMaterialUnitConversionByIDParams{
		ID:         id,
		MaterialID: materialID,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	conversion, err := conversionFromGetRow(row)
	if err != nil {
		return nil, err
	}

	return &conversion, nil
}

func (r *postgresRepository) Create(ctx context.Context, input CreateInput) (*MaterialUnitConversion, error) {
	factor, err := numericFromString(input.Factor)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.CreateMaterialUnitConversion(ctx, materialdb.CreateMaterialUnitConversionParams{
		MaterialID: input.MaterialID,
		FromUnitID: input.FromUnitID,
		ToUnitID:   input.ToUnitID,
		Factor:     factor,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	conversion, err := conversionFromCreateRow(row)
	if err != nil {
		return nil, err
	}

	return &conversion, nil
}

func (r *postgresRepository) Update(ctx context.Context, input UpdateInput) (*MaterialUnitConversion, error) {
	factor, err := numericFromString(input.Factor)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.UpdateMaterialUnitConversion(ctx, materialdb.UpdateMaterialUnitConversionParams{
		ID:         input.ID,
		MaterialID: input.MaterialID,
		FromUnitID: input.FromUnitID,
		ToUnitID:   input.ToUnitID,
		Factor:     factor,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	conversion, err := conversionFromUpdateRow(row)
	if err != nil {
		return nil, err
	}

	return &conversion, nil
}

func (r *postgresRepository) SoftDelete(ctx context.Context, materialID int64, id int64) error {
	rowsAffected, err := r.queries.SoftDeleteMaterialUnitConversion(ctx, materialdb.SoftDeleteMaterialUnitConversionParams{
		ID:         id,
		MaterialID: materialID,
	})
	if err != nil {
		return mapPostgresError(err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *postgresRepository) MaterialExists(ctx context.Context, id int64) (bool, error) {
	exists, err := r.queries.MaterialExists(ctx, id)
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

func (r *postgresRepository) ConversionExists(ctx context.Context, materialID int64, fromUnitID int64, toUnitID int64, excludeID int64) (bool, error) {
	exists, err := r.queries.MaterialUnitConversionExists(ctx, materialdb.MaterialUnitConversionExistsParams{
		MaterialID: materialID,
		FromUnitID: fromUnitID,
		ToUnitID:   toUnitID,
		ExcludeID:  excludeID,
	})
	if err != nil {
		return false, mapPostgresError(err)
	}

	return exists, nil
}

func (r *postgresRepository) ReverseConversionExists(ctx context.Context, materialID int64, fromUnitID int64, toUnitID int64, excludeID int64) (bool, error) {
	exists, err := r.queries.ReverseMaterialUnitConversionExists(ctx, materialdb.ReverseMaterialUnitConversionExistsParams{
		MaterialID: materialID,
		FromUnitID: fromUnitID,
		ToUnitID:   toUnitID,
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
		if pgErr.ConstraintName == "ux_material_unit_conversions_material_units" {
			return ErrDuplicatePair
		}
	case "23503":
		switch pgErr.ConstraintName {
		case "material_unit_conversions_material_id_fkey":
			return ErrMaterialNotFound
		case "material_unit_conversions_from_unit_id_fkey":
			return ErrFromUnitNotFound
		case "material_unit_conversions_to_unit_id_fkey":
			return ErrToUnitNotFound
		}
	}

	return err
}

func numericFromString(value string) (pgtype.Numeric, error) {
	var numeric pgtype.Numeric
	if err := numeric.Scan(value); err != nil {
		return pgtype.Numeric{}, err
	}

	return numeric, nil
}

func factorString(value pgtype.Numeric) (string, error) {
	driverValue, err := value.Value()
	if err != nil {
		return "", err
	}

	factor, ok := driverValue.(string)
	if !ok {
		return "", errors.New("material unit conversion factor is not numeric text")
	}

	return factor, nil
}

func conversionFromListRow(row materialdb.ListMaterialUnitConversionsRow) (MaterialUnitConversion, error) {
	factor, err := factorString(row.Factor)
	if err != nil {
		return MaterialUnitConversion{}, err
	}

	return MaterialUnitConversion{
		ID:         row.ID,
		MaterialID: row.MaterialID,
		FromUnitID: row.FromUnitID,
		ToUnitID:   row.ToUnitID,
		Factor:     factor,
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}, nil
}

func conversionFromGetRow(row materialdb.GetMaterialUnitConversionByIDRow) (MaterialUnitConversion, error) {
	factor, err := factorString(row.Factor)
	if err != nil {
		return MaterialUnitConversion{}, err
	}

	return MaterialUnitConversion{
		ID:         row.ID,
		MaterialID: row.MaterialID,
		FromUnitID: row.FromUnitID,
		ToUnitID:   row.ToUnitID,
		Factor:     factor,
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}, nil
}

func conversionFromCreateRow(row materialdb.CreateMaterialUnitConversionRow) (MaterialUnitConversion, error) {
	factor, err := factorString(row.Factor)
	if err != nil {
		return MaterialUnitConversion{}, err
	}

	return MaterialUnitConversion{
		ID:         row.ID,
		MaterialID: row.MaterialID,
		FromUnitID: row.FromUnitID,
		ToUnitID:   row.ToUnitID,
		Factor:     factor,
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}, nil
}

func conversionFromUpdateRow(row materialdb.UpdateMaterialUnitConversionRow) (MaterialUnitConversion, error) {
	factor, err := factorString(row.Factor)
	if err != nil {
		return MaterialUnitConversion{}, err
	}

	return MaterialUnitConversion{
		ID:         row.ID,
		MaterialID: row.MaterialID,
		FromUnitID: row.FromUnitID,
		ToUnitID:   row.ToUnitID,
		Factor:     factor,
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}, nil
}
