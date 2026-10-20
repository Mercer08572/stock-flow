package inventory

import (
	"context"
	"errors"
	"math/big"

	sku "github.com/Mercer08572/stock-flow/internal/sku"
	warehouse "github.com/Mercer08572/stock-flow/internal/warehouse"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

type operationPlan struct {
	OperationType string
	Context       OperationContext
	WarehouseID   int64
	SKUID         int64
	BatchID       *int64
	ReservationID *int64
	CanonicalQty  *string
	Execute       func(ctx context.Context, repo Repository) (*StockOperationResult, error)
}

func (s *service) runOperation(ctx context.Context, plan operationPlan) (*StockOperationResult, error) {
	if s.uow == nil {
		return nil, errors.New("inventory unit of work is required")
	}

	payload, hash, err := buildIdempotencyRequest(
		plan.OperationType,
		plan.WarehouseID,
		plan.SKUID,
		plan.BatchID,
		plan.ReservationID,
		plan.CanonicalQty,
		plan.Context,
	)
	if err != nil {
		return nil, err
	}

	var result *StockOperationResult
	err = s.uow.WithinTx(ctx, func(txCtx context.Context, repo Repository) error {
		claim, err := claimIdempotency(txCtx, repo, plan.OperationType, plan.Context, hash, payload)
		if err != nil {
			return err
		}
		if claim.Replay != nil {
			result = claim.Replay
			return nil
		}

		operationResult, err := plan.Execute(txCtx, repo)
		if err != nil {
			return err
		}
		if err := completeIdempotency(txCtx, repo, claim.ID, operationResult); err != nil {
			return err
		}
		result = operationResult

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *service) IncreaseStock(ctx context.Context, req IncreaseStockRequest) (*StockOperationResult, error) {
	operationContext, err := normalizeOperationContext(req.OperationContext)
	if err != nil {
		return nil, err
	}
	qty, err := parseQuantity(req.Qty)
	if err != nil {
		return nil, err
	}
	normalizedQty := formatQuantity(qty)
	if err := s.validateMutationReferences(ctx, req.WarehouseID, req.SKUID, req.BatchID); err != nil {
		return nil, err
	}
	canonicalQty := canonicalQuantity(qty)

	return s.runOperation(ctx, operationPlan{
		OperationType: "increase",
		Context:       operationContext,
		WarehouseID:   req.WarehouseID,
		SKUID:         req.SKUID,
		BatchID:       req.BatchID,
		CanonicalQty:  &canonicalQty,
		Execute: func(ctx context.Context, repo Repository) (*StockOperationResult, error) {
			if err := repo.EnsureStockRow(ctx, req.WarehouseID, req.SKUID); err != nil {
				return nil, err
			}
			if _, err := repo.LockStockRow(ctx, req.WarehouseID, req.SKUID); err != nil {
				return nil, err
			}

			layer, err := repo.InsertStockLayer(ctx, req.WarehouseID, req.SKUID, req.BatchID, normalizedQty)
			if err != nil {
				return nil, err
			}
			if err := repo.AddStockOnHand(ctx, req.WarehouseID, req.SKUID, normalizedQty); err != nil {
				return nil, err
			}
			movement, err := repo.InsertMovement(ctx, MovementInput{
				OperationType:  "increase",
				WarehouseID:    req.WarehouseID,
				SKUID:          req.SKUID,
				BatchID:        req.BatchID,
				StockLayerID:   &layer.ID,
				Qty:            normalizedQty,
				IdempotencyKey: operationContext.IdempotencyKey,
				SourceType:     operationContext.SourceType,
				SourceID:       operationContext.SourceID,
				SourceLineID:   operationContext.SourceLineID,
				Actor:          operationContext.Actor,
			})
			if err != nil {
				return nil, err
			}

			return buildOperationResult(ctx, repo, "increase", req.WarehouseID, req.SKUID, req.BatchID, normalizedQty, nil, []*Movement{movement})
		},
	})
}

func (s *service) ReserveStock(ctx context.Context, req ReserveStockRequest) (*StockOperationResult, error) {
	operationContext, err := normalizeOperationContext(req.OperationContext)
	if err != nil {
		return nil, err
	}
	qty, err := parseQuantity(req.Qty)
	if err != nil {
		return nil, err
	}
	normalizedQty := formatQuantity(qty)
	if err := s.validateMutationReferences(ctx, req.WarehouseID, req.SKUID, req.BatchID); err != nil {
		return nil, err
	}
	canonicalQty := canonicalQuantity(qty)

	return s.runOperation(ctx, operationPlan{
		OperationType: "reserve",
		Context:       operationContext,
		WarehouseID:   req.WarehouseID,
		SKUID:         req.SKUID,
		BatchID:       req.BatchID,
		CanonicalQty:  &canonicalQty,
		Execute: func(ctx context.Context, repo Repository) (*StockOperationResult, error) {
			if err := repo.EnsureStockRow(ctx, req.WarehouseID, req.SKUID); err != nil {
				return nil, err
			}
			stock, err := repo.LockStockRow(ctx, req.WarehouseID, req.SKUID)
			if err != nil {
				return nil, err
			}
			available, err := subtractQuantity(stock.OnHandQty, stock.ReservedQty)
			if err != nil {
				return nil, err
			}
			if comparison, err := compareQuantity(available, normalizedQty); err != nil {
				return nil, err
			} else if comparison < 0 {
				return nil, ErrInsufficientAvailable
			}

			layers, err := repo.ListAvailableStockLayersForUpdate(ctx, req.WarehouseID, req.SKUID, req.BatchID)
			if err != nil {
				return nil, err
			}
			takes, err := allocateAcrossLayers(normalizedQty, layers)
			if err != nil {
				return nil, err
			}

			reservation, err := repo.InsertReservation(ctx, ReservationInput{
				WarehouseID:    req.WarehouseID,
				SKUID:          req.SKUID,
				TotalQty:       normalizedQty,
				IdempotencyKey: operationContext.IdempotencyKey,
				SourceType:     operationContext.SourceType,
				SourceID:       operationContext.SourceID,
				SourceLineID:   operationContext.SourceLineID,
				Actor:          operationContext.Actor,
			})
			if err != nil {
				return nil, err
			}

			allocations := make([]ReservationAllocation, 0, len(takes))
			movements := make([]*Movement, 0, len(takes))
			for _, take := range takes {
				reserved, err := repo.AddStockLayerReserved(ctx, take.Layer.ID, take.Qty)
				if err != nil {
					return nil, err
				}
				if !reserved {
					return nil, ErrInsufficientAvailable
				}

				item, err := repo.InsertReservationItem(ctx, reservation.ID, take.Layer.ID, take.Qty)
				if err != nil {
					return nil, err
				}
				movement, err := repo.InsertMovement(ctx, MovementInput{
					OperationType:     "reserve",
					WarehouseID:       req.WarehouseID,
					SKUID:             req.SKUID,
					BatchID:           take.Layer.BatchID,
					StockLayerID:      &take.Layer.ID,
					ReservationID:     &reservation.ID,
					ReservationItemID: &item.ID,
					Qty:               take.Qty,
					IdempotencyKey:    operationContext.IdempotencyKey,
					SourceType:        operationContext.SourceType,
					SourceID:          operationContext.SourceID,
					SourceLineID:      operationContext.SourceLineID,
					Actor:             operationContext.Actor,
				})
				if err != nil {
					return nil, err
				}

				allocations = append(allocations, ReservationAllocation{
					ReservationItemID: item.ID,
					StockLayerID:      take.Layer.ID,
					BatchID:           take.Layer.BatchID,
					Qty:               take.Qty,
				})
				movements = append(movements, movement)
			}

			reserved, err := repo.ReserveStock(ctx, req.WarehouseID, req.SKUID, normalizedQty)
			if err != nil {
				return nil, err
			}
			if !reserved {
				return nil, ErrInsufficientAvailable
			}

			snapshot, err := reservationSnapshot(reservation, allocations)
			if err != nil {
				return nil, err
			}

			return buildOperationResult(ctx, repo, "reserve", req.WarehouseID, req.SKUID, req.BatchID, normalizedQty, snapshot, movements)
		},
	})
}

func (s *service) ReleaseStock(ctx context.Context, req ReleaseStockRequest) (*StockOperationResult, error) {
	operationContext, err := normalizeOperationContext(req.OperationContext)
	if err != nil {
		return nil, err
	}
	if req.ReservationID <= 0 {
		return nil, apperr.NewValidationError("reservation_id must be greater than zero")
	}

	var (
		normalizedQty string
		canonicalQty  *string
	)
	if req.Qty != nil {
		qty, err := parseQuantity(*req.Qty)
		if err != nil {
			return nil, err
		}
		normalizedQty = formatQuantity(qty)
		value := canonicalQuantity(qty)
		canonicalQty = &value
	}
	if err := s.validateMutationReferences(ctx, req.WarehouseID, req.SKUID, nil); err != nil {
		return nil, err
	}

	return s.runOperation(ctx, operationPlan{
		OperationType: "release_reserved",
		Context:       operationContext,
		WarehouseID:   req.WarehouseID,
		SKUID:         req.SKUID,
		ReservationID: &req.ReservationID,
		CanonicalQty:  canonicalQty,
		Execute: func(ctx context.Context, repo Repository) (*StockOperationResult, error) {
			if err := repo.EnsureStockRow(ctx, req.WarehouseID, req.SKUID); err != nil {
				return nil, err
			}
			if _, err := repo.LockStockRow(ctx, req.WarehouseID, req.SKUID); err != nil {
				return nil, err
			}

			reservation, err := repo.LockReservation(ctx, req.ReservationID)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					return nil, ErrReservationNotFound
				}
				return nil, err
			}
			if reservation.WarehouseID != req.WarehouseID || reservation.SKUID != req.SKUID {
				return nil, ErrReservationMismatch
			}
			if reservation.Status != "active" {
				return nil, ErrReservationNotActive
			}

			remaining, err := reservationRemaining(reservation)
			if err != nil {
				return nil, err
			}
			releaseQty := remaining
			if canonicalQty != nil {
				releaseQty = normalizedQty
				comparison, err := compareQuantity(releaseQty, remaining)
				if err != nil {
					return nil, err
				}
				if comparison > 0 {
					return nil, ErrInsufficientReserved
				}
			}

			items, err := repo.ListReservationItemsForUpdate(ctx, reservation.ID)
			if err != nil {
				return nil, err
			}
			takes, err := allocateAcrossReservationItems(releaseQty, items)
			if err != nil {
				return nil, err
			}

			allocations := make([]ReservationAllocation, 0, len(takes))
			movements := make([]*Movement, 0, len(takes))
			for _, take := range takes {
				released, err := repo.ReleaseStockLayerReserved(ctx, take.Item.StockLayerID, take.Qty)
				if err != nil {
					return nil, err
				}
				if !released {
					return nil, ErrInsufficientReserved
				}
				itemUpdated, err := repo.AddReservationItemReleased(ctx, take.Item.ID, take.Qty)
				if err != nil {
					return nil, err
				}
				if !itemUpdated {
					return nil, ErrInsufficientReserved
				}
				movement, err := repo.InsertMovement(ctx, MovementInput{
					OperationType:     "release_reserved",
					WarehouseID:       req.WarehouseID,
					SKUID:             req.SKUID,
					BatchID:           take.Item.BatchID,
					StockLayerID:      &take.Item.StockLayerID,
					ReservationID:     &reservation.ID,
					ReservationItemID: &take.Item.ID,
					Qty:               take.Qty,
					IdempotencyKey:    operationContext.IdempotencyKey,
					SourceType:        operationContext.SourceType,
					SourceID:          operationContext.SourceID,
					SourceLineID:      operationContext.SourceLineID,
					Actor:             operationContext.Actor,
				})
				if err != nil {
					return nil, err
				}
				allocations = append(allocations, ReservationAllocation{
					ReservationItemID: take.Item.ID,
					StockLayerID:      take.Item.StockLayerID,
					BatchID:           take.Item.BatchID,
					Qty:               take.Qty,
				})
				movements = append(movements, movement)
			}

			updated, err := repo.AddReservationReleased(ctx, reservation.ID, releaseQty)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					return nil, ErrReservationNotActive
				}
				return nil, err
			}
			released, err := repo.ReleaseStockReserved(ctx, req.WarehouseID, req.SKUID, releaseQty)
			if err != nil {
				return nil, err
			}
			if !released {
				return nil, ErrInsufficientReserved
			}

			snapshot, err := reservationSnapshot(updated, allocations)
			if err != nil {
				return nil, err
			}

			return buildOperationResult(ctx, repo, "release_reserved", req.WarehouseID, req.SKUID, nil, releaseQty, snapshot, movements)
		},
	})
}

func (s *service) DecreaseStock(ctx context.Context, req DecreaseStockRequest) (*StockOperationResult, error) {
	operationContext, err := normalizeOperationContext(req.OperationContext)
	if err != nil {
		return nil, err
	}
	qty, err := parseQuantity(req.Qty)
	if err != nil {
		return nil, err
	}
	normalizedQty := formatQuantity(qty)
	canonicalQty := canonicalQuantity(qty)

	if req.ReservationID != nil {
		if *req.ReservationID <= 0 {
			return nil, apperr.NewValidationError("reservation_id must be greater than zero")
		}
		if req.BatchID != nil {
			return nil, apperr.NewValidationError("batch_id must not be provided when reservation_id is set")
		}
		if err := s.validateMutationReferences(ctx, req.WarehouseID, req.SKUID, nil); err != nil {
			return nil, err
		}

		return s.runOperation(ctx, operationPlan{
			OperationType: "decrease_reserved",
			Context:       operationContext,
			WarehouseID:   req.WarehouseID,
			SKUID:         req.SKUID,
			ReservationID: req.ReservationID,
			CanonicalQty:  &canonicalQty,
			Execute: func(ctx context.Context, repo Repository) (*StockOperationResult, error) {
				return s.decreaseReserved(ctx, repo, operationContext, req, normalizedQty)
			},
		})
	}

	if err := s.validateMutationReferences(ctx, req.WarehouseID, req.SKUID, req.BatchID); err != nil {
		return nil, err
	}

	return s.runOperation(ctx, operationPlan{
		OperationType: "decrease_available",
		Context:       operationContext,
		WarehouseID:   req.WarehouseID,
		SKUID:         req.SKUID,
		BatchID:       req.BatchID,
		CanonicalQty:  &canonicalQty,
		Execute: func(ctx context.Context, repo Repository) (*StockOperationResult, error) {
			return s.decreaseAvailable(ctx, repo, operationContext, req, normalizedQty)
		},
	})
}

func (s *service) decreaseAvailable(ctx context.Context, repo Repository, operationContext OperationContext, req DecreaseStockRequest, qty string) (*StockOperationResult, error) {
	if err := repo.EnsureStockRow(ctx, req.WarehouseID, req.SKUID); err != nil {
		return nil, err
	}
	stock, err := repo.LockStockRow(ctx, req.WarehouseID, req.SKUID)
	if err != nil {
		return nil, err
	}
	available, err := subtractQuantity(stock.OnHandQty, stock.ReservedQty)
	if err != nil {
		return nil, err
	}
	if comparison, err := compareQuantity(available, qty); err != nil {
		return nil, err
	} else if comparison < 0 {
		return nil, ErrInsufficientAvailable
	}

	layers, err := repo.ListAvailableStockLayersForUpdate(ctx, req.WarehouseID, req.SKUID, req.BatchID)
	if err != nil {
		return nil, err
	}
	takes, err := allocateAcrossLayers(qty, layers)
	if err != nil {
		return nil, err
	}

	movements := make([]*Movement, 0, len(takes))
	for _, take := range takes {
		decreased, err := repo.DecreaseStockLayerAvailable(ctx, take.Layer.ID, take.Qty)
		if err != nil {
			return nil, err
		}
		if !decreased {
			return nil, ErrInsufficientAvailable
		}
		movement, err := repo.InsertMovement(ctx, MovementInput{
			OperationType:  "decrease_available",
			WarehouseID:    req.WarehouseID,
			SKUID:          req.SKUID,
			BatchID:        take.Layer.BatchID,
			StockLayerID:   &take.Layer.ID,
			Qty:            take.Qty,
			IdempotencyKey: operationContext.IdempotencyKey,
			SourceType:     operationContext.SourceType,
			SourceID:       operationContext.SourceID,
			SourceLineID:   operationContext.SourceLineID,
			Actor:          operationContext.Actor,
		})
		if err != nil {
			return nil, err
		}
		movements = append(movements, movement)
	}

	decreased, err := repo.DecreaseStockAvailable(ctx, req.WarehouseID, req.SKUID, qty)
	if err != nil {
		return nil, err
	}
	if !decreased {
		return nil, ErrInsufficientAvailable
	}

	return buildOperationResult(ctx, repo, "decrease_available", req.WarehouseID, req.SKUID, req.BatchID, qty, nil, movements)
}

func (s *service) decreaseReserved(ctx context.Context, repo Repository, operationContext OperationContext, req DecreaseStockRequest, qty string) (*StockOperationResult, error) {
	if err := repo.EnsureStockRow(ctx, req.WarehouseID, req.SKUID); err != nil {
		return nil, err
	}
	if _, err := repo.LockStockRow(ctx, req.WarehouseID, req.SKUID); err != nil {
		return nil, err
	}

	reservation, err := repo.LockReservation(ctx, *req.ReservationID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrReservationNotFound
		}
		return nil, err
	}
	if reservation.WarehouseID != req.WarehouseID || reservation.SKUID != req.SKUID {
		return nil, ErrReservationMismatch
	}
	if reservation.Status != "active" {
		return nil, ErrReservationNotActive
	}

	remaining, err := reservationRemaining(reservation)
	if err != nil {
		return nil, err
	}
	if comparison, err := compareQuantity(qty, remaining); err != nil {
		return nil, err
	} else if comparison > 0 {
		return nil, ErrInsufficientReserved
	}

	items, err := repo.ListReservationItemsForUpdate(ctx, reservation.ID)
	if err != nil {
		return nil, err
	}
	takes, err := allocateAcrossReservationItems(qty, items)
	if err != nil {
		return nil, err
	}

	allocations := make([]ReservationAllocation, 0, len(takes))
	movements := make([]*Movement, 0, len(takes))
	for _, take := range takes {
		consumed, err := repo.ConsumeStockLayerReserved(ctx, take.Item.StockLayerID, take.Qty)
		if err != nil {
			return nil, err
		}
		if !consumed {
			return nil, ErrInsufficientReserved
		}
		itemUpdated, err := repo.AddReservationItemConsumed(ctx, take.Item.ID, take.Qty)
		if err != nil {
			return nil, err
		}
		if !itemUpdated {
			return nil, ErrInsufficientReserved
		}
		movement, err := repo.InsertMovement(ctx, MovementInput{
			OperationType:     "decrease_reserved",
			WarehouseID:       req.WarehouseID,
			SKUID:             req.SKUID,
			BatchID:           take.Item.BatchID,
			StockLayerID:      &take.Item.StockLayerID,
			ReservationID:     &reservation.ID,
			ReservationItemID: &take.Item.ID,
			Qty:               take.Qty,
			IdempotencyKey:    operationContext.IdempotencyKey,
			SourceType:        operationContext.SourceType,
			SourceID:          operationContext.SourceID,
			SourceLineID:      operationContext.SourceLineID,
			Actor:             operationContext.Actor,
		})
		if err != nil {
			return nil, err
		}
		allocations = append(allocations, ReservationAllocation{
			ReservationItemID: take.Item.ID,
			StockLayerID:      take.Item.StockLayerID,
			BatchID:           take.Item.BatchID,
			Qty:               take.Qty,
		})
		movements = append(movements, movement)
	}

	updated, err := repo.AddReservationConsumed(ctx, reservation.ID, qty)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrReservationNotActive
		}
		return nil, err
	}
	consumed, err := repo.ConsumeStockReserved(ctx, req.WarehouseID, req.SKUID, qty)
	if err != nil {
		return nil, err
	}
	if !consumed {
		return nil, ErrInsufficientReserved
	}

	snapshot, err := reservationSnapshot(updated, allocations)
	if err != nil {
		return nil, err
	}

	return buildOperationResult(ctx, repo, "decrease_reserved", req.WarehouseID, req.SKUID, nil, qty, snapshot, movements)
}

func (s *service) ListMovements(ctx context.Context, filter ListMovementsFilter) (MovementListResult, error) {
	normalized, err := normalizeListMovementsFilter(filter)
	if err != nil {
		return MovementListResult{}, err
	}
	if normalized.WarehouseID != nil {
		if err := s.validateWarehouseForQuery(ctx, *normalized.WarehouseID); err != nil {
			return MovementListResult{}, err
		}
	}
	if normalized.SKUID != nil {
		if err := s.validateSKUForQuery(ctx, *normalized.SKUID); err != nil {
			return MovementListResult{}, err
		}
	}

	items, err := s.repo.ListMovements(ctx, normalized)
	if err != nil {
		return MovementListResult{}, err
	}

	return MovementListResult{Items: items, Limit: normalized.Limit, Offset: normalized.Offset}, nil
}

func (s *service) validateMutationReferences(ctx context.Context, warehouseID int64, skuID int64, batchID *int64) error {
	if err := s.validateWarehouseForMutation(ctx, warehouseID); err != nil {
		return err
	}
	if err := s.validateSKUForMutation(ctx, skuID); err != nil {
		return err
	}
	return s.validateBatchForMutation(ctx, batchID, skuID)
}

// 查询允许停用/已删除的主数据引用，只校验存在性。
func (s *service) validateWarehouseForQuery(ctx context.Context, warehouseID int64) error {
	if warehouseID <= 0 {
		return apperr.NewValidationError("warehouse_id must be greater than zero")
	}
	if _, err := s.warehouseReader.GetReference(ctx, warehouseID); err != nil {
		if errors.Is(err, warehouse.ErrNotFound) {
			return ErrWarehouseNotFound
		}
		if apperr.IsValidationError(err) {
			return apperr.NewValidationError(err.Error())
		}
		return err
	}
	return nil
}

func (s *service) validateSKUForQuery(ctx context.Context, skuID int64) error {
	if skuID <= 0 {
		return apperr.NewValidationError("sku_id must be greater than zero")
	}
	if _, err := s.skuReader.GetReference(ctx, skuID); err != nil {
		if errors.Is(err, sku.ErrNotFound) {
			return ErrSKUNotFound
		}
		if apperr.IsValidationError(err) {
			return apperr.NewValidationError(err.Error())
		}
		return err
	}
	return nil
}

type layerTake struct {
	Layer StockLayer
	Qty   string
}

type reservationItemTake struct {
	Item ReservationItemRecord
	Qty  string
}

func allocateAcrossLayers(qty string, layers []StockLayer) ([]layerTake, error) {
	remaining, err := parseStoredQuantity(qty)
	if err != nil {
		return nil, err
	}

	takes := make([]layerTake, 0, len(layers))
	for _, layer := range layers {
		if remaining.Sign() == 0 {
			break
		}
		availableText, err := subtractQuantity(layer.OnHandQty, layer.ReservedQty)
		if err != nil {
			return nil, err
		}
		available, err := parseStoredQuantity(availableText)
		if err != nil {
			return nil, err
		}
		if available.Sign() == 0 {
			continue
		}

		take := minQuantity(remaining, available)
		takes = append(takes, layerTake{Layer: layer, Qty: formatQuantity(take)})
		remaining.Sub(remaining, take)
	}
	if remaining.Sign() > 0 {
		return nil, ErrInsufficientAvailable
	}

	return takes, nil
}

func allocateAcrossReservationItems(qty string, items []ReservationItemRecord) ([]reservationItemTake, error) {
	remaining, err := parseStoredQuantity(qty)
	if err != nil {
		return nil, err
	}

	takes := make([]reservationItemTake, 0, len(items))
	for _, item := range items {
		if remaining.Sign() == 0 {
			break
		}
		itemRemaining, err := reservationItemRemaining(item)
		if err != nil {
			return nil, err
		}
		if itemRemaining.Sign() == 0 {
			continue
		}

		take := minQuantity(remaining, itemRemaining)
		takes = append(takes, reservationItemTake{Item: item, Qty: formatQuantity(take)})
		remaining.Sub(remaining, take)
	}
	if remaining.Sign() > 0 {
		return nil, ErrInsufficientReserved
	}

	return takes, nil
}

func reservationItemRemaining(item ReservationItemRecord) (*big.Rat, error) {
	reserved, err := parseStoredQuantity(item.ReservedQty)
	if err != nil {
		return nil, err
	}
	released, err := parseStoredQuantity(item.ReleasedQty)
	if err != nil {
		return nil, err
	}
	consumed, err := parseStoredQuantity(item.ConsumedQty)
	if err != nil {
		return nil, err
	}

	remaining := new(big.Rat).Sub(reserved, new(big.Rat).Add(released, consumed))
	if remaining.Sign() < 0 {
		return nil, errors.New("reservation item quantities are inconsistent")
	}
	return remaining, nil
}

func reservationRemaining(reservation *ReservationRecord) (string, error) {
	remaining, err := subtractQuantity(reservation.TotalQty, reservation.ReleasedQty)
	if err != nil {
		return "", err
	}
	return subtractQuantity(remaining, reservation.ConsumedQty)
}

func reservationSnapshot(reservation *ReservationRecord, allocations []ReservationAllocation) (*ReservationSnapshot, error) {
	remaining, err := reservationRemaining(reservation)
	if err != nil {
		return nil, err
	}

	total, err := normalizeQuantity(reservation.TotalQty)
	if err != nil {
		return nil, err
	}
	released, err := normalizeQuantity(reservation.ReleasedQty)
	if err != nil {
		return nil, err
	}
	consumed, err := normalizeQuantity(reservation.ConsumedQty)
	if err != nil {
		return nil, err
	}

	return &ReservationSnapshot{
		ID:           reservation.ID,
		Status:       reservation.Status,
		CloseReason:  reservation.CloseReason,
		TotalQty:     total,
		ReleasedQty:  released,
		ConsumedQty:  consumed,
		RemainingQty: remaining,
		Allocations:  allocations,
	}, nil
}

func buildOperationResult(
	ctx context.Context,
	repo Repository,
	operationType string,
	warehouseID int64,
	skuID int64,
	batchID *int64,
	qty string,
	reservation *ReservationSnapshot,
	movements []*Movement,
) (*StockOperationResult, error) {
	stock, err := repo.GetStock(ctx, warehouseID, skuID)
	if err != nil {
		return nil, err
	}
	quantities, err := stockQuantities(stock)
	if err != nil {
		return nil, err
	}

	refs := make([]MovementRef, 0, len(movements))
	for _, movement := range movements {
		if movement == nil {
			continue
		}
		refs = append(refs, MovementRef{
			ID:           movement.ID,
			StockLayerID: movement.StockLayerID,
			Qty:          movement.Qty,
		})
	}

	return &StockOperationResult{
		OperationType: operationType,
		WarehouseID:   warehouseID,
		SKUID:         skuID,
		BatchID:       batchID,
		Qty:           qty,
		StockAfter:    quantities,
		Reservation:   reservation,
		Movements:     refs,
	}, nil
}

func stockQuantities(stock *StockBalance) (StockQuantities, error) {
	onHand, err := normalizeQuantity(stock.OnHandQty)
	if err != nil {
		return StockQuantities{}, err
	}
	reserved, err := normalizeQuantity(stock.ReservedQty)
	if err != nil {
		return StockQuantities{}, err
	}
	available, err := subtractQuantity(onHand, reserved)
	if err != nil {
		return StockQuantities{}, err
	}

	return StockQuantities{OnHandQty: onHand, ReservedQty: reserved, AvailableQty: available}, nil
}

func normalizeListMovementsFilter(filter ListMovementsFilter) (ListMovementsFilter, error) {
	if filter.WarehouseID != nil && *filter.WarehouseID <= 0 {
		return ListMovementsFilter{}, apperr.NewValidationError("warehouse_id must be greater than zero")
	}
	if filter.SKUID != nil && *filter.SKUID <= 0 {
		return ListMovementsFilter{}, apperr.NewValidationError("sku_id must be greater than zero")
	}
	if filter.OperationType != nil {
		operationType := *filter.OperationType
		switch operationType {
		case "increase", "reserve", "release_reserved", "decrease_available", "decrease_reserved":
		default:
			return ListMovementsFilter{}, apperr.NewValidationError("operation_type is invalid")
		}
		filter.OperationType = &operationType
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
	if filter.CreatedFrom != nil && filter.CreatedTo != nil && filter.CreatedFrom.After(*filter.CreatedTo) {
		return ListMovementsFilter{}, apperr.NewValidationError("created_from must not be after created_to")
	}

	return filter, nil
}
