package inventory

import (
	"context"
	"errors"
	"math/big"
	"strings"

	sku "github.com/Mercer08572/stock-flow/internal/sku"
	warehouse "github.com/Mercer08572/stock-flow/internal/warehouse"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

type Service interface {
	ListStocks(ctx context.Context, filter ListStocksFilter) (StockListResult, error)
	GetStock(ctx context.Context, query GetStockQuery) (*StockBalance, error)
	ListLayers(ctx context.Context, filter ListLayersFilter) (LayerListResult, error)
	ListReservations(ctx context.Context, filter ListReservationsFilter) (ReservationListResult, error)
	GetReservation(ctx context.Context, reservationID int64) (*Reservation, error)

	IncreaseStock(ctx context.Context, req IncreaseStockRequest) (*StockOperationResult, error)
	ReserveStock(ctx context.Context, req ReserveStockRequest) (*StockOperationResult, error)
	ReleaseStock(ctx context.Context, req ReleaseStockRequest) (*StockOperationResult, error)
	DecreaseStock(ctx context.Context, req DecreaseStockRequest) (*StockOperationResult, error)
	ListMovements(ctx context.Context, filter ListMovementsFilter) (MovementListResult, error)
}

type Repository interface {
	ListStocks(ctx context.Context, filter ListStocksFilter) ([]StockBalance, error)
	CountStocks(ctx context.Context, filter ListStocksFilter) (int64, error)
	GetStock(ctx context.Context, warehouseID int64, skuID int64) (*StockBalance, error)
	ListLayers(ctx context.Context, filter ListLayersFilter) ([]StockLayer, error)
	CountLayers(ctx context.Context, filter ListLayersFilter) (int64, error)
	ListReservations(ctx context.Context, filter ListReservationsFilter) ([]Reservation, error)
	CountReservations(ctx context.Context, filter ListReservationsFilter) (int64, error)
	GetReservation(ctx context.Context, reservationID int64) (*Reservation, error)
	ListReservationItems(ctx context.Context, reservationID int64) ([]ReservationItem, error)
	BatchExistsForSKU(ctx context.Context, batchID int64, skuID int64) (bool, error)
	HasWarehouseReferences(ctx context.Context, warehouseID int64) (bool, error)
	HasSKUReferences(ctx context.Context, skuID int64) (bool, error)

	GetBatch(ctx context.Context, batchID int64, skuID int64) (*BatchInfo, error)

	EnsureStockRow(ctx context.Context, warehouseID int64, skuID int64) error
	LockStockRow(ctx context.Context, warehouseID int64, skuID int64) (*StockBalance, error)
	AddStockOnHand(ctx context.Context, warehouseID int64, skuID int64, qty string) error
	ReserveStock(ctx context.Context, warehouseID int64, skuID int64, qty string) (bool, error)
	ReleaseStockReserved(ctx context.Context, warehouseID int64, skuID int64, qty string) (bool, error)
	DecreaseStockAvailable(ctx context.Context, warehouseID int64, skuID int64, qty string) (bool, error)
	ConsumeStockReserved(ctx context.Context, warehouseID int64, skuID int64, qty string) (bool, error)

	InsertStockLayer(ctx context.Context, warehouseID int64, skuID int64, batchID *int64, qty string) (*StockLayer, error)
	ListAvailableStockLayersForUpdate(ctx context.Context, warehouseID int64, skuID int64, batchID *int64) ([]StockLayer, error)
	AddStockLayerReserved(ctx context.Context, stockLayerID int64, qty string) (bool, error)
	DecreaseStockLayerAvailable(ctx context.Context, stockLayerID int64, qty string) (bool, error)
	ReleaseStockLayerReserved(ctx context.Context, stockLayerID int64, qty string) (bool, error)
	ConsumeStockLayerReserved(ctx context.Context, stockLayerID int64, qty string) (bool, error)

	InsertReservation(ctx context.Context, input ReservationInput) (*ReservationRecord, error)
	LockReservation(ctx context.Context, reservationID int64) (*ReservationRecord, error)
	AddReservationReleased(ctx context.Context, reservationID int64, qty string) (*ReservationRecord, error)
	AddReservationConsumed(ctx context.Context, reservationID int64, qty string) (*ReservationRecord, error)
	InsertReservationItem(ctx context.Context, reservationID int64, stockLayerID int64, qty string) (*ReservationItemRecord, error)
	ListReservationItemsForUpdate(ctx context.Context, reservationID int64) ([]ReservationItemRecord, error)
	AddReservationItemReleased(ctx context.Context, reservationItemID int64, qty string) (bool, error)
	AddReservationItemConsumed(ctx context.Context, reservationItemID int64, qty string) (bool, error)

	InsertMovement(ctx context.Context, input MovementInput) (*Movement, error)
	ListMovements(ctx context.Context, filter ListMovementsFilter) ([]Movement, error)
	CountMovements(ctx context.Context, filter ListMovementsFilter) (int64, error)

	ClaimIdempotencyKey(ctx context.Context, input IdempotencyClaim) (*IdempotencyRecord, bool, error)
	GetIdempotencyKey(ctx context.Context, operationType string, key string) (*IdempotencyRecord, error)
	CompleteIdempotencyKey(ctx context.Context, input IdempotencyCompletion) error
}

type WarehouseReader interface {
	GetReference(ctx context.Context, id int64) (*warehouse.Reference, error)
	Get(ctx context.Context, id int64) (*warehouse.Warehouse, error)
}

type SKUReader interface {
	GetReference(ctx context.Context, id int64) (*sku.Reference, error)
	Get(ctx context.Context, id int64) (*sku.SKU, error)
}

type service struct {
	repo            Repository
	warehouseReader WarehouseReader
	skuReader       SKUReader
	uow             UnitOfWork
}

func NewService(repo Repository, warehouseReader WarehouseReader, skuReader SKUReader, uow UnitOfWork) Service {
	return &service{
		repo:            repo,
		warehouseReader: warehouseReader,
		skuReader:       skuReader,
		uow:             uow,
	}
}

func (s *service) ListStocks(ctx context.Context, filter ListStocksFilter) (StockListResult, error) {
	normalized, err := normalizeListStocksFilter(filter)
	if err != nil {
		return StockListResult{}, err
	}

	if normalized.WarehouseID != nil {
		if _, err := s.getWarehouseReference(ctx, *normalized.WarehouseID); err != nil {
			return StockListResult{}, err
		}
	}
	if normalized.SKUID != nil {
		if _, err := s.getSKUReference(ctx, *normalized.SKUID); err != nil {
			return StockListResult{}, err
		}
	}

	total, err := s.repo.CountStocks(ctx, normalized)
	if err != nil {
		return StockListResult{}, err
	}

	items, err := s.repo.ListStocks(ctx, normalized)
	if err != nil {
		return StockListResult{}, err
	}
	for i := range items {
		if err := applyStockAvailable(&items[i]); err != nil {
			return StockListResult{}, err
		}
		if err := s.applyStockReferences(ctx, &items[i]); err != nil {
			return StockListResult{}, err
		}
	}

	return StockListResult{
		Items:  items,
		Total:  total,
		Limit:  normalized.Limit,
		Offset: normalized.Offset,
	}, nil
}

func (s *service) GetStock(ctx context.Context, query GetStockQuery) (*StockBalance, error) {
	normalized, err := normalizeGetStockQuery(query)
	if err != nil {
		return nil, err
	}
	warehouseReference, err := s.getWarehouseReference(ctx, normalized.WarehouseID)
	if err != nil {
		return nil, err
	}
	skuReference, err := s.getSKUReference(ctx, normalized.SKUID)
	if err != nil {
		return nil, err
	}

	stock, err := s.repo.GetStock(ctx, normalized.WarehouseID, normalized.SKUID)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		stock = zeroStockBalance(normalized.WarehouseID, normalized.SKUID)
	}
	if err := applyStockAvailable(stock); err != nil {
		return nil, err
	}
	stock.Warehouse = warehouseReference
	stock.SKU = skuReference

	if normalized.IncludeLayers {
		layers, err := s.repo.ListLayers(ctx, ListLayersFilter{
			WarehouseID: normalized.WarehouseID,
			SKUID:       normalized.SKUID,
			Limit:       MaxListLimit,
			Offset:      0,
		})
		if err != nil {
			return nil, err
		}
		for i := range layers {
			if err := applyLayerAvailable(&layers[i]); err != nil {
				return nil, err
			}
			layers[i].Warehouse = warehouseReference
			layers[i].SKU = skuReference
		}
		stock.Layers = layers
	}

	return stock, nil
}

func (s *service) ListLayers(ctx context.Context, filter ListLayersFilter) (LayerListResult, error) {
	normalized, err := normalizeListLayersFilter(filter)
	if err != nil {
		return LayerListResult{}, err
	}
	warehouseReference, err := s.getWarehouseReference(ctx, normalized.WarehouseID)
	if err != nil {
		return LayerListResult{}, err
	}
	skuReference, err := s.getSKUReference(ctx, normalized.SKUID)
	if err != nil {
		return LayerListResult{}, err
	}
	if normalized.BatchID != nil {
		exists, err := s.repo.BatchExistsForSKU(ctx, *normalized.BatchID, normalized.SKUID)
		if err != nil {
			return LayerListResult{}, err
		}
		if !exists {
			return LayerListResult{}, ErrBatchNotFound
		}
	}

	total, err := s.repo.CountLayers(ctx, normalized)
	if err != nil {
		return LayerListResult{}, err
	}

	items, err := s.repo.ListLayers(ctx, normalized)
	if err != nil {
		return LayerListResult{}, err
	}
	for i := range items {
		if err := applyLayerAvailable(&items[i]); err != nil {
			return LayerListResult{}, err
		}
		items[i].Warehouse = warehouseReference
		items[i].SKU = skuReference
	}

	return LayerListResult{
		Items:  items,
		Total:  total,
		Limit:  normalized.Limit,
		Offset: normalized.Offset,
	}, nil
}

func (s *service) getWarehouseReference(ctx context.Context, id int64) (*warehouse.Reference, error) {
	if s.warehouseReader == nil {
		return nil, errors.New("inventory warehouse reader is required")
	}
	reference, err := s.warehouseReader.GetReference(ctx, id)
	if err != nil {
		if errors.Is(err, warehouse.ErrNotFound) {
			return nil, ErrWarehouseNotFound
		}
		if apperr.IsValidationError(err) {
			return nil, apperr.NewValidationError(err.Error())
		}
		return nil, err
	}

	return reference, nil
}

func (s *service) getSKUReference(ctx context.Context, id int64) (*sku.Reference, error) {
	if s.skuReader == nil {
		return nil, errors.New("inventory sku reader is required")
	}
	reference, err := s.skuReader.GetReference(ctx, id)
	if err != nil {
		if errors.Is(err, sku.ErrNotFound) {
			return nil, ErrSKUNotFound
		}
		if apperr.IsValidationError(err) {
			return nil, apperr.NewValidationError(err.Error())
		}
		return nil, err
	}

	return reference, nil
}

func (s *service) applyStockReferences(ctx context.Context, stock *StockBalance) error {
	warehouseReference, err := s.getWarehouseReference(ctx, stock.WarehouseID)
	if err != nil {
		return err
	}
	skuReference, err := s.getSKUReference(ctx, stock.SKUID)
	if err != nil {
		return err
	}

	stock.Warehouse = warehouseReference
	stock.SKU = skuReference
	return nil
}

func normalizeListStocksFilter(filter ListStocksFilter) (ListStocksFilter, error) {
	if filter.WarehouseID != nil && *filter.WarehouseID <= 0 {
		return ListStocksFilter{}, apperr.NewValidationError("warehouse_id must be greater than zero")
	}
	if filter.SKUID != nil && *filter.SKUID <= 0 {
		return ListStocksFilter{}, apperr.NewValidationError("sku_id must be greater than zero")
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

func normalizeGetStockQuery(query GetStockQuery) (GetStockQuery, error) {
	if query.WarehouseID <= 0 {
		return GetStockQuery{}, apperr.NewValidationError("warehouse_id must be greater than zero")
	}
	if query.SKUID <= 0 {
		return GetStockQuery{}, apperr.NewValidationError("sku_id must be greater than zero")
	}

	return query, nil
}

func normalizeListLayersFilter(filter ListLayersFilter) (ListLayersFilter, error) {
	if filter.WarehouseID <= 0 {
		return ListLayersFilter{}, apperr.NewValidationError("warehouse_id must be greater than zero")
	}
	if filter.SKUID <= 0 {
		return ListLayersFilter{}, apperr.NewValidationError("sku_id must be greater than zero")
	}
	if filter.BatchID != nil && *filter.BatchID <= 0 {
		return ListLayersFilter{}, apperr.NewValidationError("batch_id must be greater than zero")
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

func zeroStockBalance(warehouseID int64, skuID int64) *StockBalance {
	return &StockBalance{
		WarehouseID:  warehouseID,
		SKUID:        skuID,
		OnHandQty:    "0",
		ReservedQty:  "0",
		AvailableQty: "0",
	}
}

func applyStockAvailable(stock *StockBalance) error {
	available, err := calculateAvailable(stock.OnHandQty, stock.ReservedQty)
	if err != nil {
		return err
	}
	stock.OnHandQty = normalizeDecimalOutput(stock.OnHandQty)
	stock.ReservedQty = normalizeDecimalOutput(stock.ReservedQty)
	stock.AvailableQty = available

	return nil
}

func applyLayerAvailable(layer *StockLayer) error {
	available, err := calculateAvailable(layer.OnHandQty, layer.ReservedQty)
	if err != nil {
		return err
	}
	layer.OnHandQty = normalizeDecimalOutput(layer.OnHandQty)
	layer.ReservedQty = normalizeDecimalOutput(layer.ReservedQty)
	layer.AvailableQty = available

	return nil
}

func calculateAvailable(onHand string, reserved string) (string, error) {
	onHandRat, ok := new(big.Rat).SetString(onHand)
	if !ok {
		return "", apperr.NewValidationError("on_hand_qty is invalid")
	}
	reservedRat, ok := new(big.Rat).SetString(reserved)
	if !ok {
		return "", apperr.NewValidationError("reserved_qty is invalid")
	}

	available := new(big.Rat).Sub(onHandRat, reservedRat)
	scale := max(decimalScale(onHand), decimalScale(reserved))
	return trimDecimalZeros(available.FloatString(scale)), nil
}

func normalizeDecimalOutput(value string) string {
	return trimDecimalZeros(strings.TrimSpace(value))
}

func decimalScale(value string) int {
	trimmed := strings.TrimSpace(value)
	dot := strings.IndexByte(trimmed, '.')
	if dot < 0 {
		return 0
	}
	return len(trimmed) - dot - 1
}

func trimDecimalZeros(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.Contains(trimmed, ".") {
		trimmed = strings.TrimRight(trimmed, "0")
		trimmed = strings.TrimRight(trimmed, ".")
	}
	if trimmed == "" || trimmed == "-0" {
		return "0"
	}
	return trimmed
}
