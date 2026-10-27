package inventory

import (
	"context"
	"errors"
	"time"

	"github.com/Mercer08572/stock-flow/internal/sku"
	"github.com/Mercer08572/stock-flow/internal/warehouse"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

// Reservation 是库存占用单的只读视图。
type Reservation struct {
	ID             int64                `json:"id"`
	WarehouseID    int64                `json:"warehouse_id"`
	Warehouse      *warehouse.Reference `json:"warehouse,omitempty"`
	SKUID          int64                `json:"sku_id"`
	SKU            *sku.Reference       `json:"sku,omitempty"`
	TotalQty       string               `json:"total_qty"`
	ReleasedQty    string               `json:"released_qty"`
	ConsumedQty    string               `json:"consumed_qty"`
	RemainingQty   string               `json:"remaining_qty"`
	Status         string               `json:"status"`
	CloseReason    *string              `json:"close_reason"`
	IdempotencyKey string               `json:"idempotency_key"`
	SourceType     *string              `json:"source_type"`
	SourceID       *string              `json:"source_id"`
	SourceLineID   *string              `json:"source_line_id"`
	ActorType      *string              `json:"actor_type"`
	ActorID        *int64               `json:"actor_id"`
	ActorLabel     *string              `json:"actor_label"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
	// Items 只在详情接口返回，列表接口不携带分配明细。
	Items []ReservationItem `json:"items,omitempty"`
}

// ReservationItem 是占用单在库存层上的 FIFO 分配明细。
type ReservationItem struct {
	ID            int64      `json:"id"`
	ReservationID int64      `json:"reservation_id"`
	StockLayerID  int64      `json:"stock_layer_id"`
	BatchID       *int64     `json:"batch_id"`
	ReceivedAt    *time.Time `json:"received_at"`
	ReservedQty   string     `json:"reserved_qty"`
	ReleasedQty   string     `json:"released_qty"`
	ConsumedQty   string     `json:"consumed_qty"`
	RemainingQty  string     `json:"remaining_qty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type ListReservationsFilter struct {
	WarehouseID *int64
	SKUID       *int64
	Status      *string
	Limit       int32
	Offset      int32
}

type ReservationListResult struct {
	Items  []Reservation `json:"items"`
	Limit  int32         `json:"limit"`
	Offset int32         `json:"offset"`
}

func (s *service) ListReservations(ctx context.Context, filter ListReservationsFilter) (ReservationListResult, error) {
	normalized, err := normalizeListReservationsFilter(filter)
	if err != nil {
		return ReservationListResult{}, err
	}

	if normalized.WarehouseID != nil {
		if _, err := s.getWarehouseReference(ctx, *normalized.WarehouseID); err != nil {
			return ReservationListResult{}, err
		}
	}
	if normalized.SKUID != nil {
		if _, err := s.getSKUReference(ctx, *normalized.SKUID); err != nil {
			return ReservationListResult{}, err
		}
	}

	items, err := s.repo.ListReservations(ctx, normalized)
	if err != nil {
		return ReservationListResult{}, err
	}
	for i := range items {
		if err := applyReservationQuantities(&items[i]); err != nil {
			return ReservationListResult{}, err
		}
		if err := s.applyReservationReferences(ctx, &items[i]); err != nil {
			return ReservationListResult{}, err
		}
	}

	return ReservationListResult{
		Items:  items,
		Limit:  normalized.Limit,
		Offset: normalized.Offset,
	}, nil
}

func (s *service) GetReservation(ctx context.Context, reservationID int64) (*Reservation, error) {
	if reservationID <= 0 {
		return nil, apperr.NewValidationError("id must be greater than zero")
	}

	reservation, err := s.repo.GetReservation(ctx, reservationID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrReservationNotFound
		}
		return nil, err
	}
	if err := applyReservationQuantities(reservation); err != nil {
		return nil, err
	}

	items, err := s.repo.ListReservationItems(ctx, reservationID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if err := applyReservationItemQuantities(&items[i]); err != nil {
			return nil, err
		}
	}
	reservation.Items = items

	if err := s.applyReservationReferences(ctx, reservation); err != nil {
		return nil, err
	}

	return reservation, nil
}

func normalizeListReservationsFilter(filter ListReservationsFilter) (ListReservationsFilter, error) {
	if filter.WarehouseID != nil && *filter.WarehouseID <= 0 {
		return ListReservationsFilter{}, apperr.NewValidationError("warehouse_id must be greater than zero")
	}
	if filter.SKUID != nil && *filter.SKUID <= 0 {
		return ListReservationsFilter{}, apperr.NewValidationError("sku_id must be greater than zero")
	}
	if filter.Status != nil && *filter.Status != "active" && *filter.Status != "closed" {
		return ListReservationsFilter{}, apperr.NewValidationError("status must be active or closed")
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

// applyReservationQuantities 归一化数量并推导剩余占用数量。
func applyReservationQuantities(reservation *Reservation) error {
	total, err := normalizeQuantity(reservation.TotalQty)
	if err != nil {
		return err
	}
	released, err := normalizeQuantity(reservation.ReleasedQty)
	if err != nil {
		return err
	}
	consumed, err := normalizeQuantity(reservation.ConsumedQty)
	if err != nil {
		return err
	}
	remaining, err := subtractQuantity(total, released)
	if err != nil {
		return err
	}
	remaining, err = subtractQuantity(remaining, consumed)
	if err != nil {
		return err
	}

	reservation.TotalQty = total
	reservation.ReleasedQty = released
	reservation.ConsumedQty = consumed
	reservation.RemainingQty = remaining
	return nil
}

func applyReservationItemQuantities(item *ReservationItem) error {
	reserved, err := normalizeQuantity(item.ReservedQty)
	if err != nil {
		return err
	}
	released, err := normalizeQuantity(item.ReleasedQty)
	if err != nil {
		return err
	}
	consumed, err := normalizeQuantity(item.ConsumedQty)
	if err != nil {
		return err
	}
	remaining, err := subtractQuantity(reserved, released)
	if err != nil {
		return err
	}
	remaining, err = subtractQuantity(remaining, consumed)
	if err != nil {
		return err
	}

	item.ReservedQty = reserved
	item.ReleasedQty = released
	item.ConsumedQty = consumed
	item.RemainingQty = remaining
	return nil
}

func (s *service) applyReservationReferences(ctx context.Context, reservation *Reservation) error {
	warehouseReference, err := s.getWarehouseReference(ctx, reservation.WarehouseID)
	if err != nil {
		return err
	}
	skuReference, err := s.getSKUReference(ctx, reservation.SKUID)
	if err != nil {
		return err
	}

	reservation.Warehouse = warehouseReference
	reservation.SKU = skuReference
	return nil
}
