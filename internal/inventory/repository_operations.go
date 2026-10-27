package inventory

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	inventorydb "github.com/Mercer08572/stock-flow/internal/inventory/db"
)

func (r *postgresRepository) GetBatch(ctx context.Context, batchID int64, skuID int64) (*BatchInfo, error) {
	row, err := r.queries.GetInventoryBatchForSKU(ctx, inventorydb.GetInventoryBatchForSKUParams{
		ID:    batchID,
		SkuID: skuID,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	return &BatchInfo{
		ID:               row.ID,
		SKUID:            row.SkuID,
		BatchNo:          row.BatchNo,
		DispatchPriority: row.DispatchPriority,
		Status:           row.Status,
	}, nil
}

func (r *postgresRepository) EnsureStockRow(ctx context.Context, warehouseID int64, skuID int64) error {
	return r.queries.EnsureInventoryStockRow(ctx, inventorydb.EnsureInventoryStockRowParams{
		WarehouseID: warehouseID,
		SkuID:       skuID,
	})
}

func (r *postgresRepository) LockStockRow(ctx context.Context, warehouseID int64, skuID int64) (*StockBalance, error) {
	row, err := r.queries.LockInventoryStockRow(ctx, inventorydb.LockInventoryStockRowParams{
		WarehouseID: warehouseID,
		SkuID:       skuID,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	onHand, err := numericString(row.OnHandQty)
	if err != nil {
		return nil, err
	}
	reserved, err := numericString(row.ReservedQty)
	if err != nil {
		return nil, err
	}
	updatedAt := row.UpdatedAt.Time

	return &StockBalance{
		WarehouseID: row.WarehouseID,
		SKUID:       row.SkuID,
		OnHandQty:   onHand,
		ReservedQty: reserved,
		UpdatedAt:   &updatedAt,
	}, nil
}

func (r *postgresRepository) AddStockOnHand(ctx context.Context, warehouseID int64, skuID int64, qty string) error {
	quantity, err := numericFromString(qty)
	if err != nil {
		return err
	}

	_, err = r.queries.AddInventoryStockOnHand(ctx, inventorydb.AddInventoryStockOnHandParams{
		Qty:         quantity,
		WarehouseID: warehouseID,
		SkuID:       skuID,
	})
	return err
}

func (r *postgresRepository) ReserveStock(ctx context.Context, warehouseID int64, skuID int64, qty string) (bool, error) {
	quantity, err := numericFromString(qty)
	if err != nil {
		return false, err
	}

	rows, err := r.queries.ReserveInventoryStock(ctx, inventorydb.ReserveInventoryStockParams{
		Qty:         quantity,
		WarehouseID: warehouseID,
		SkuID:       skuID,
	})
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (r *postgresRepository) ReleaseStockReserved(ctx context.Context, warehouseID int64, skuID int64, qty string) (bool, error) {
	quantity, err := numericFromString(qty)
	if err != nil {
		return false, err
	}

	rows, err := r.queries.ReleaseInventoryStockReserved(ctx, inventorydb.ReleaseInventoryStockReservedParams{
		Qty:         quantity,
		WarehouseID: warehouseID,
		SkuID:       skuID,
	})
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (r *postgresRepository) DecreaseStockAvailable(ctx context.Context, warehouseID int64, skuID int64, qty string) (bool, error) {
	quantity, err := numericFromString(qty)
	if err != nil {
		return false, err
	}

	rows, err := r.queries.DecreaseInventoryStockAvailable(ctx, inventorydb.DecreaseInventoryStockAvailableParams{
		Qty:         quantity,
		WarehouseID: warehouseID,
		SkuID:       skuID,
	})
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (r *postgresRepository) ConsumeStockReserved(ctx context.Context, warehouseID int64, skuID int64, qty string) (bool, error) {
	quantity, err := numericFromString(qty)
	if err != nil {
		return false, err
	}

	rows, err := r.queries.ConsumeInventoryStockReserved(ctx, inventorydb.ConsumeInventoryStockReservedParams{
		Qty:         quantity,
		WarehouseID: warehouseID,
		SkuID:       skuID,
	})
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (r *postgresRepository) InsertStockLayer(ctx context.Context, warehouseID int64, skuID int64, batchID *int64, qty string) (*StockLayer, error) {
	quantity, err := numericFromString(qty)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.InsertInventoryStockLayer(ctx, inventorydb.InsertInventoryStockLayerParams{
		WarehouseID: warehouseID,
		SkuID:       skuID,
		BatchID:     batchID,
		Qty:         quantity,
	})
	if err != nil {
		return nil, err
	}

	onHand, err := numericString(row.OnHandQty)
	if err != nil {
		return nil, err
	}
	reserved, err := numericString(row.ReservedQty)
	if err != nil {
		return nil, err
	}

	return &StockLayer{
		ID:          row.ID,
		WarehouseID: row.WarehouseID,
		SKUID:       row.SkuID,
		BatchID:     row.BatchID,
		ReceivedAt:  row.ReceivedAt.Time,
		OnHandQty:   onHand,
		ReservedQty: reserved,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (r *postgresRepository) ListAvailableStockLayersForUpdate(ctx context.Context, warehouseID int64, skuID int64, batchID *int64) ([]StockLayer, error) {
	rows, err := r.queries.ListInventoryStockLayersForUpdate(ctx, inventorydb.ListInventoryStockLayersForUpdateParams{
		WarehouseID: warehouseID,
		SkuID:       skuID,
		BatchID:     batchID,
	})
	if err != nil {
		return nil, err
	}

	layers := make([]StockLayer, 0, len(rows))
	for _, row := range rows {
		onHand, err := numericString(row.OnHandQty)
		if err != nil {
			return nil, err
		}
		reserved, err := numericString(row.ReservedQty)
		if err != nil {
			return nil, err
		}
		layers = append(layers, StockLayer{
			ID:          row.ID,
			WarehouseID: row.WarehouseID,
			SKUID:       row.SkuID,
			BatchID:     row.BatchID,
			ReceivedAt:  row.ReceivedAt.Time,
			OnHandQty:   onHand,
			ReservedQty: reserved,
		})
	}

	return layers, nil
}

func (r *postgresRepository) AddStockLayerReserved(ctx context.Context, stockLayerID int64, qty string) (bool, error) {
	quantity, err := numericFromString(qty)
	if err != nil {
		return false, err
	}

	rows, err := r.queries.AddInventoryStockLayerReserved(ctx, inventorydb.AddInventoryStockLayerReservedParams{
		Qty: quantity,
		ID:  stockLayerID,
	})
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (r *postgresRepository) DecreaseStockLayerAvailable(ctx context.Context, stockLayerID int64, qty string) (bool, error) {
	quantity, err := numericFromString(qty)
	if err != nil {
		return false, err
	}

	rows, err := r.queries.DecreaseInventoryStockLayerAvailable(ctx, inventorydb.DecreaseInventoryStockLayerAvailableParams{
		Qty: quantity,
		ID:  stockLayerID,
	})
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (r *postgresRepository) ReleaseStockLayerReserved(ctx context.Context, stockLayerID int64, qty string) (bool, error) {
	quantity, err := numericFromString(qty)
	if err != nil {
		return false, err
	}

	rows, err := r.queries.ReleaseInventoryStockLayerReserved(ctx, inventorydb.ReleaseInventoryStockLayerReservedParams{
		Qty: quantity,
		ID:  stockLayerID,
	})
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (r *postgresRepository) ConsumeStockLayerReserved(ctx context.Context, stockLayerID int64, qty string) (bool, error) {
	quantity, err := numericFromString(qty)
	if err != nil {
		return false, err
	}

	rows, err := r.queries.ConsumeInventoryStockLayerReserved(ctx, inventorydb.ConsumeInventoryStockLayerReservedParams{
		Qty: quantity,
		ID:  stockLayerID,
	})
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (r *postgresRepository) InsertReservation(ctx context.Context, input ReservationInput) (*ReservationRecord, error) {
	totalQty, err := numericFromString(input.TotalQty)
	if err != nil {
		return nil, err
	}
	actorType, actorID, actorLabel := actorValues(input.Actor)

	row, err := r.queries.InsertInventoryReservation(ctx, inventorydb.InsertInventoryReservationParams{
		WarehouseID:    input.WarehouseID,
		SkuID:          input.SKUID,
		TotalQty:       totalQty,
		IdempotencyKey: input.IdempotencyKey,
		SourceType:     input.SourceType,
		SourceID:       input.SourceID,
		SourceLineID:   input.SourceLineID,
		ActorType:      actorType,
		ActorID:        actorID,
		ActorLabel:     actorLabel,
	})
	if err != nil {
		return nil, err
	}

	return reservationRecord(
		row.ID,
		row.WarehouseID,
		row.SkuID,
		row.TotalQty,
		row.ReleasedQty,
		row.ConsumedQty,
		row.Status,
		row.CloseReason,
		row.IdempotencyKey,
		row.SourceType,
		row.SourceID,
		row.SourceLineID,
		row.ActorType,
		row.ActorID,
		row.ActorLabel,
		row.CreatedAt,
		row.UpdatedAt,
	)
}

func (r *postgresRepository) LockReservation(ctx context.Context, reservationID int64) (*ReservationRecord, error) {
	row, err := r.queries.LockInventoryReservation(ctx, reservationID)
	if err != nil {
		return nil, mapPostgresError(err)
	}

	return reservationRecord(
		row.ID,
		row.WarehouseID,
		row.SkuID,
		row.TotalQty,
		row.ReleasedQty,
		row.ConsumedQty,
		row.Status,
		row.CloseReason,
		row.IdempotencyKey,
		row.SourceType,
		row.SourceID,
		row.SourceLineID,
		row.ActorType,
		row.ActorID,
		row.ActorLabel,
		row.CreatedAt,
		row.UpdatedAt,
	)
}

func (r *postgresRepository) AddReservationReleased(ctx context.Context, reservationID int64, qty string) (*ReservationRecord, error) {
	quantity, err := numericFromString(qty)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.AddInventoryReservationReleased(ctx, inventorydb.AddInventoryReservationReleasedParams{
		Qty: quantity,
		ID:  reservationID,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	return reservationRecord(
		row.ID,
		row.WarehouseID,
		row.SkuID,
		row.TotalQty,
		row.ReleasedQty,
		row.ConsumedQty,
		row.Status,
		row.CloseReason,
		row.IdempotencyKey,
		row.SourceType,
		row.SourceID,
		row.SourceLineID,
		row.ActorType,
		row.ActorID,
		row.ActorLabel,
		row.CreatedAt,
		row.UpdatedAt,
	)
}

func (r *postgresRepository) AddReservationConsumed(ctx context.Context, reservationID int64, qty string) (*ReservationRecord, error) {
	quantity, err := numericFromString(qty)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.AddInventoryReservationConsumed(ctx, inventorydb.AddInventoryReservationConsumedParams{
		Qty: quantity,
		ID:  reservationID,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	return reservationRecord(
		row.ID,
		row.WarehouseID,
		row.SkuID,
		row.TotalQty,
		row.ReleasedQty,
		row.ConsumedQty,
		row.Status,
		row.CloseReason,
		row.IdempotencyKey,
		row.SourceType,
		row.SourceID,
		row.SourceLineID,
		row.ActorType,
		row.ActorID,
		row.ActorLabel,
		row.CreatedAt,
		row.UpdatedAt,
	)
}

func (r *postgresRepository) InsertReservationItem(ctx context.Context, reservationID int64, stockLayerID int64, qty string) (*ReservationItemRecord, error) {
	reservedQty, err := numericFromString(qty)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.InsertInventoryReservationItem(ctx, inventorydb.InsertInventoryReservationItemParams{
		ReservationID: reservationID,
		StockLayerID:  stockLayerID,
		ReservedQty:   reservedQty,
	})
	if err != nil {
		return nil, err
	}

	reserved, err := numericString(row.ReservedQty)
	if err != nil {
		return nil, err
	}
	released, err := numericString(row.ReleasedQty)
	if err != nil {
		return nil, err
	}
	consumed, err := numericString(row.ConsumedQty)
	if err != nil {
		return nil, err
	}

	return &ReservationItemRecord{
		ID:            row.ID,
		ReservationID: row.ReservationID,
		StockLayerID:  row.StockLayerID,
		ReservedQty:   reserved,
		ReleasedQty:   released,
		ConsumedQty:   consumed,
	}, nil
}

func (r *postgresRepository) ListReservationItemsForUpdate(ctx context.Context, reservationID int64) ([]ReservationItemRecord, error) {
	rows, err := r.queries.ListInventoryReservationItemsForUpdate(ctx, reservationID)
	if err != nil {
		return nil, err
	}

	items := make([]ReservationItemRecord, 0, len(rows))
	for _, row := range rows {
		reserved, err := numericString(row.ReservedQty)
		if err != nil {
			return nil, err
		}
		released, err := numericString(row.ReleasedQty)
		if err != nil {
			return nil, err
		}
		consumed, err := numericString(row.ConsumedQty)
		if err != nil {
			return nil, err
		}
		items = append(items, ReservationItemRecord{
			ID:            row.ID,
			ReservationID: row.ReservationID,
			StockLayerID:  row.StockLayerID,
			ReservedQty:   reserved,
			ReleasedQty:   released,
			ConsumedQty:   consumed,
			BatchID:       row.BatchID,
		})
	}

	return items, nil
}

func (r *postgresRepository) AddReservationItemReleased(ctx context.Context, reservationItemID int64, qty string) (bool, error) {
	quantity, err := numericFromString(qty)
	if err != nil {
		return false, err
	}

	rows, err := r.queries.AddInventoryReservationItemReleased(ctx, inventorydb.AddInventoryReservationItemReleasedParams{
		Qty: quantity,
		ID:  reservationItemID,
	})
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (r *postgresRepository) AddReservationItemConsumed(ctx context.Context, reservationItemID int64, qty string) (bool, error) {
	quantity, err := numericFromString(qty)
	if err != nil {
		return false, err
	}

	rows, err := r.queries.AddInventoryReservationItemConsumed(ctx, inventorydb.AddInventoryReservationItemConsumedParams{
		Qty: quantity,
		ID:  reservationItemID,
	})
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (r *postgresRepository) InsertMovement(ctx context.Context, input MovementInput) (*Movement, error) {
	quantity, err := numericFromString(input.Qty)
	if err != nil {
		return nil, err
	}
	actorType, actorID, actorLabel := actorValues(input.Actor)

	row, err := r.queries.InsertInventoryMovement(ctx, inventorydb.InsertInventoryMovementParams{
		OperationType:     input.OperationType,
		WarehouseID:       input.WarehouseID,
		SkuID:             input.SKUID,
		BatchID:           input.BatchID,
		StockLayerID:      input.StockLayerID,
		ReservationID:     input.ReservationID,
		ReservationItemID: input.ReservationItemID,
		Qty:               quantity,
		IdempotencyKey:    input.IdempotencyKey,
		SourceType:        input.SourceType,
		SourceID:          input.SourceID,
		SourceLineID:      input.SourceLineID,
		ActorType:         actorType,
		ActorID:           actorID,
		ActorLabel:        actorLabel,
	})
	if err != nil {
		return nil, err
	}

	qty, err := numericString(row.Qty)
	if err != nil {
		return nil, err
	}

	return &Movement{
		ID:                row.ID,
		OperationType:     row.OperationType,
		WarehouseID:       row.WarehouseID,
		SKUID:             row.SkuID,
		BatchID:           row.BatchID,
		StockLayerID:      row.StockLayerID,
		ReservationID:     row.ReservationID,
		ReservationItemID: row.ReservationItemID,
		Qty:               qty,
		IdempotencyKey:    row.IdempotencyKey,
		SourceType:        row.SourceType,
		SourceID:          row.SourceID,
		SourceLineID:      row.SourceLineID,
		ActorType:         row.ActorType,
		ActorID:           row.ActorID,
		ActorLabel:        row.ActorLabel,
		CreatedAt:         row.CreatedAt.Time,
	}, nil
}

func (r *postgresRepository) ListMovements(ctx context.Context, filter ListMovementsFilter) ([]Movement, error) {
	rows, err := r.queries.ListInventoryMovements(ctx, inventorydb.ListInventoryMovementsParams{
		WarehouseID:   filter.WarehouseID,
		SkuID:         filter.SKUID,
		OperationType: filter.OperationType,
		CreatedFrom:   timestamptzValue(filter.CreatedFrom),
		CreatedTo:     timestamptzValue(filter.CreatedTo),
		Offset:        filter.Offset,
		Limit:         filter.Limit,
	})
	if err != nil {
		return nil, err
	}

	movements := make([]Movement, 0, len(rows))
	for _, row := range rows {
		qty, err := numericString(row.Qty)
		if err != nil {
			return nil, err
		}
		movements = append(movements, Movement{
			ID:                row.ID,
			OperationType:     row.OperationType,
			WarehouseID:       row.WarehouseID,
			SKUID:             row.SkuID,
			BatchID:           row.BatchID,
			StockLayerID:      row.StockLayerID,
			ReservationID:     row.ReservationID,
			ReservationItemID: row.ReservationItemID,
			Qty:               qty,
			IdempotencyKey:    row.IdempotencyKey,
			SourceType:        row.SourceType,
			SourceID:          row.SourceID,
			SourceLineID:      row.SourceLineID,
			ActorType:         row.ActorType,
			ActorID:           row.ActorID,
			ActorLabel:        row.ActorLabel,
			CreatedAt:         row.CreatedAt.Time,
		})
	}

	return movements, nil
}

func (r *postgresRepository) CountMovements(ctx context.Context, filter ListMovementsFilter) (int64, error) {
	count, err := r.queries.CountInventoryMovements(ctx, inventorydb.CountInventoryMovementsParams{
		WarehouseID:   filter.WarehouseID,
		SkuID:         filter.SKUID,
		OperationType: filter.OperationType,
		CreatedFrom:   timestamptzValue(filter.CreatedFrom),
		CreatedTo:     timestamptzValue(filter.CreatedTo),
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *postgresRepository) ClaimIdempotencyKey(ctx context.Context, input IdempotencyClaim) (*IdempotencyRecord, bool, error) {
	row, err := r.queries.ClaimInventoryIdempotencyKey(ctx, inventorydb.ClaimInventoryIdempotencyKeyParams{
		OperationType:  input.OperationType,
		IdempotencyKey: input.Key,
		RequestHash:    input.RequestHash,
		RequestPayload: input.RequestPayload,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return &IdempotencyRecord{
		ID:              row.ID,
		OperationType:   row.OperationType,
		Key:             row.IdempotencyKey,
		RequestHash:     row.RequestHash,
		RequestPayload:  row.RequestPayload,
		Status:          row.Status,
		ResponsePayload: row.ResponsePayload,
	}, true, nil
}

func (r *postgresRepository) GetIdempotencyKey(ctx context.Context, operationType string, key string) (*IdempotencyRecord, error) {
	row, err := r.queries.GetInventoryIdempotencyKey(ctx, inventorydb.GetInventoryIdempotencyKeyParams{
		OperationType:  operationType,
		IdempotencyKey: key,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	return &IdempotencyRecord{
		ID:              row.ID,
		OperationType:   row.OperationType,
		Key:             row.IdempotencyKey,
		RequestHash:     row.RequestHash,
		RequestPayload:  row.RequestPayload,
		Status:          row.Status,
		ResponsePayload: row.ResponsePayload,
	}, nil
}

func (r *postgresRepository) CompleteIdempotencyKey(ctx context.Context, input IdempotencyCompletion) error {
	rows, err := r.queries.CompleteInventoryIdempotencyKey(ctx, inventorydb.CompleteInventoryIdempotencyKeyParams{
		ID:              input.ID,
		ResponseRefType: input.ResponseRefType,
		ResponseRefID:   input.ResponseRefID,
		ResponsePayload: input.ResponsePayload,
	})
	if err != nil {
		return err
	}
	if rows != 1 {
		return ErrNotFound
	}

	return nil
}

func numericFromString(value string) (pgtype.Numeric, error) {
	var quantity pgtype.Numeric
	if err := quantity.Scan(value); err != nil {
		return pgtype.Numeric{}, err
	}

	return quantity, nil
}

func timestamptzValue(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func actorValues(actor *Actor) (*string, *int64, *string) {
	if actor == nil {
		return nil, nil, nil
	}
	actorType := actor.Type
	actorID := actor.ID
	actorLabel := actor.Label
	return &actorType, &actorID, &actorLabel
}

func reservationRecord(
	id int64,
	warehouseID int64,
	skuID int64,
	totalQty pgtype.Numeric,
	releasedQty pgtype.Numeric,
	consumedQty pgtype.Numeric,
	status string,
	closeReason *string,
	idempotencyKey string,
	sourceType *string,
	sourceID *string,
	sourceLineID *string,
	actorType *string,
	actorID *int64,
	actorLabel *string,
	createdAt pgtype.Timestamptz,
	updatedAt pgtype.Timestamptz,
) (*ReservationRecord, error) {
	total, err := numericString(totalQty)
	if err != nil {
		return nil, err
	}
	released, err := numericString(releasedQty)
	if err != nil {
		return nil, err
	}
	consumed, err := numericString(consumedQty)
	if err != nil {
		return nil, err
	}

	return &ReservationRecord{
		ID:             id,
		WarehouseID:    warehouseID,
		SKUID:          skuID,
		TotalQty:       total,
		ReleasedQty:    released,
		ConsumedQty:    consumed,
		Status:         status,
		CloseReason:    closeReason,
		IdempotencyKey: idempotencyKey,
		SourceType:     sourceType,
		SourceID:       sourceID,
		SourceLineID:   sourceLineID,
		ActorType:      actorType,
		ActorID:        actorID,
		ActorLabel:     actorLabel,
		CreatedAt:      createdAt.Time,
		UpdatedAt:      updatedAt.Time,
	}, nil
}
