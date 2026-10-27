package inventory

import (
	"context"
	"time"

	inventorydb "github.com/Mercer08572/stock-flow/internal/inventory/db"
)

func (r *postgresRepository) ListReservations(ctx context.Context, filter ListReservationsFilter) ([]Reservation, error) {
	rows, err := r.queries.ListInventoryReservations(ctx, inventorydb.ListInventoryReservationsParams{
		WarehouseID: filter.WarehouseID,
		SkuID:       filter.SKUID,
		Status:      filter.Status,
		Offset:      filter.Offset,
		Limit:       filter.Limit,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	reservations := make([]Reservation, 0, len(rows))
	for _, row := range rows {
		reservation, err := reservationFromListRow(row)
		if err != nil {
			return nil, err
		}
		reservations = append(reservations, reservation)
	}

	return reservations, nil
}

func (r *postgresRepository) GetReservation(ctx context.Context, reservationID int64) (*Reservation, error) {
	row, err := r.queries.GetInventoryReservation(ctx, reservationID)
	if err != nil {
		return nil, mapPostgresError(err)
	}

	record, err := reservationRecord(
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
	if err != nil {
		return nil, err
	}

	reservation := reservationFromRecord(record)
	return &reservation, nil
}

func (r *postgresRepository) ListReservationItems(ctx context.Context, reservationID int64) ([]ReservationItem, error) {
	rows, err := r.queries.ListInventoryReservationItems(ctx, reservationID)
	if err != nil {
		return nil, mapPostgresError(err)
	}

	items := make([]ReservationItem, 0, len(rows))
	for _, row := range rows {
		item, err := reservationItemFromRow(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

func reservationFromListRow(row inventorydb.ListInventoryReservationsRow) (Reservation, error) {
	record, err := reservationRecord(
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
	if err != nil {
		return Reservation{}, err
	}

	return reservationFromRecord(record), nil
}

// reservationFromRecord 只做字段搬运；数量归一化与剩余量推导在服务层完成。
func reservationFromRecord(record *ReservationRecord) Reservation {
	return Reservation{
		ID:             record.ID,
		WarehouseID:    record.WarehouseID,
		SKUID:          record.SKUID,
		TotalQty:       record.TotalQty,
		ReleasedQty:    record.ReleasedQty,
		ConsumedQty:    record.ConsumedQty,
		Status:         record.Status,
		CloseReason:    record.CloseReason,
		IdempotencyKey: record.IdempotencyKey,
		SourceType:     record.SourceType,
		SourceID:       record.SourceID,
		SourceLineID:   record.SourceLineID,
		ActorType:      record.ActorType,
		ActorID:        record.ActorID,
		ActorLabel:     record.ActorLabel,
		CreatedAt:      record.CreatedAt,
		UpdatedAt:      record.UpdatedAt,
	}
}

func reservationItemFromRow(row inventorydb.ListInventoryReservationItemsRow) (ReservationItem, error) {
	reserved, err := numericString(row.ReservedQty)
	if err != nil {
		return ReservationItem{}, err
	}
	released, err := numericString(row.ReleasedQty)
	if err != nil {
		return ReservationItem{}, err
	}
	consumed, err := numericString(row.ConsumedQty)
	if err != nil {
		return ReservationItem{}, err
	}

	var receivedAt *time.Time
	if row.ReceivedAt.Valid {
		value := row.ReceivedAt.Time
		receivedAt = &value
	}

	return ReservationItem{
		ID:            row.ID,
		ReservationID: row.ReservationID,
		StockLayerID:  row.StockLayerID,
		BatchID:       row.BatchID,
		ReceivedAt:    receivedAt,
		ReservedQty:   reserved,
		ReleasedQty:   released,
		ConsumedQty:   consumed,
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}, nil
}
