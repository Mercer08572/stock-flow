package inventory

import (
	"time"

	"github.com/Mercer08572/stock-flow/internal/sku"
	"github.com/Mercer08572/stock-flow/internal/warehouse"
)

const (
	DefaultListLimit int32 = 20
	MaxListLimit     int32 = 100
)

type StockBalance struct {
	WarehouseID  int64                `json:"warehouse_id"`
	SKUID        int64                `json:"sku_id"`
	Warehouse    *warehouse.Reference `json:"warehouse,omitempty"`
	SKU          *sku.Reference       `json:"sku,omitempty"`
	OnHandQty    string               `json:"on_hand_qty"`
	ReservedQty  string               `json:"reserved_qty"`
	AvailableQty string               `json:"available_qty"`
	UpdatedAt    *time.Time           `json:"updated_at"`
	Layers       []StockLayer         `json:"layers,omitempty"`
}

type StockLayer struct {
	ID           int64                `json:"id"`
	WarehouseID  int64                `json:"warehouse_id"`
	SKUID        int64                `json:"sku_id"`
	Warehouse    *warehouse.Reference `json:"warehouse,omitempty"`
	SKU          *sku.Reference       `json:"sku,omitempty"`
	BatchID      *int64               `json:"batch_id"`
	ReceivedAt   time.Time            `json:"received_at"`
	OnHandQty    string               `json:"on_hand_qty"`
	ReservedQty  string               `json:"reserved_qty"`
	AvailableQty string               `json:"available_qty"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
}

type ListStocksFilter struct {
	WarehouseID *int64
	SKUID       *int64
	Limit       int32
	Offset      int32
}

type GetStockQuery struct {
	WarehouseID   int64
	SKUID         int64
	IncludeLayers bool
}

type ListLayersFilter struct {
	WarehouseID int64
	SKUID       int64
	BatchID     *int64
	Limit       int32
	Offset      int32
}

type StockListResult struct {
	Items  []StockBalance `json:"items"`
	Limit  int32          `json:"limit"`
	Offset int32          `json:"offset"`
}

type LayerListResult struct {
	Items  []StockLayer `json:"items"`
	Limit  int32        `json:"limit"`
	Offset int32        `json:"offset"`
}

// Actor 是库存变更的审计身份，来自 auth.Caller。
type Actor struct {
	Type  string
	ID    int64
	Label string
}

// OperationContext 是所有库存变更请求共享的幂等与审计上下文。
type OperationContext struct {
	Actor          *Actor
	IdempotencyKey string
	SourceType     *string
	SourceID       *string
	SourceLineID   *string
}

type IncreaseStockRequest struct {
	OperationContext
	WarehouseID int64
	SKUID       int64
	BatchID     *int64
	Qty         string
}

type ReserveStockRequest struct {
	OperationContext
	WarehouseID int64
	SKUID       int64
	BatchID     *int64
	Qty         string
}

type DecreaseStockRequest struct {
	OperationContext
	WarehouseID   int64
	SKUID         int64
	BatchID       *int64
	ReservationID *int64
	Qty           string
}

type ReleaseStockRequest struct {
	OperationContext
	WarehouseID   int64
	SKUID         int64
	ReservationID int64
	Qty           *string
}

// StockQuantities 是操作完成后的余额快照。
type StockQuantities struct {
	OnHandQty    string `json:"on_hand_qty"`
	ReservedQty  string `json:"reserved_qty"`
	AvailableQty string `json:"available_qty"`
}

type MovementRef struct {
	ID           int64  `json:"id"`
	StockLayerID *int64 `json:"stock_layer_id"`
	Qty          string `json:"qty"`
}

type ReservationAllocation struct {
	ReservationItemID int64  `json:"reservation_item_id"`
	StockLayerID      int64  `json:"stock_layer_id"`
	BatchID           *int64 `json:"batch_id"`
	Qty               string `json:"qty"`
}

type ReservationSnapshot struct {
	ID           int64                   `json:"id"`
	Status       string                  `json:"status"`
	CloseReason  *string                 `json:"close_reason"`
	TotalQty     string                  `json:"total_qty"`
	ReleasedQty  string                  `json:"released_qty"`
	ConsumedQty  string                  `json:"consumed_qty"`
	RemainingQty string                  `json:"remaining_qty"`
	Allocations  []ReservationAllocation `json:"allocations"`
}

// StockOperationResult 是所有库存变更 API 的统一返回体。
type StockOperationResult struct {
	OperationType string               `json:"operation_type"`
	WarehouseID   int64                `json:"warehouse_id"`
	SKUID         int64                `json:"sku_id"`
	BatchID       *int64               `json:"batch_id"`
	Qty           string               `json:"qty"`
	StockAfter    StockQuantities      `json:"stock_after"`
	Reservation   *ReservationSnapshot `json:"reservation,omitempty"`
	Movements     []MovementRef        `json:"movements"`
}

// Movement 是库存流水记录。
type Movement struct {
	ID                int64     `json:"id"`
	OperationType     string    `json:"operation_type"`
	WarehouseID       int64     `json:"warehouse_id"`
	SKUID             int64     `json:"sku_id"`
	BatchID           *int64    `json:"batch_id"`
	StockLayerID      *int64    `json:"stock_layer_id"`
	ReservationID     *int64    `json:"reservation_id"`
	ReservationItemID *int64    `json:"reservation_item_id"`
	Qty               string    `json:"qty"`
	IdempotencyKey    string    `json:"idempotency_key"`
	SourceType        *string   `json:"source_type"`
	SourceID          *string   `json:"source_id"`
	SourceLineID      *string   `json:"source_line_id"`
	ActorType         *string   `json:"actor_type"`
	ActorID           *int64    `json:"actor_id"`
	ActorLabel        *string   `json:"actor_label"`
	CreatedAt         time.Time `json:"created_at"`
}

type ListMovementsFilter struct {
	WarehouseID   *int64
	SKUID         *int64
	OperationType *string
	CreatedFrom   *time.Time
	CreatedTo     *time.Time
	Limit         int32
	Offset        int32
}

type MovementListResult struct {
	Items  []Movement `json:"items"`
	Limit  int32      `json:"limit"`
	Offset int32      `json:"offset"`
}

// 以下类型只在服务层与仓储层之间传递，不直接暴露给 HTTP。

type BatchInfo struct {
	ID               int64
	SKUID            int64
	BatchNo          string
	DispatchPriority int32
	Status           string
}

type ReservationRecord struct {
	ID             int64
	WarehouseID    int64
	SKUID          int64
	TotalQty       string
	ReleasedQty    string
	ConsumedQty    string
	Status         string
	CloseReason    *string
	IdempotencyKey string
	SourceType     *string
	SourceID       *string
	SourceLineID   *string
	ActorType      *string
	ActorID        *int64
	ActorLabel     *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ReservationItemRecord struct {
	ID            int64
	ReservationID int64
	StockLayerID  int64
	ReservedQty   string
	ReleasedQty   string
	ConsumedQty   string
	BatchID       *int64
}

type ReservationInput struct {
	WarehouseID    int64
	SKUID          int64
	TotalQty       string
	IdempotencyKey string
	SourceType     *string
	SourceID       *string
	SourceLineID   *string
	Actor          *Actor
}

type MovementInput struct {
	OperationType     string
	WarehouseID       int64
	SKUID             int64
	BatchID           *int64
	StockLayerID      *int64
	ReservationID     *int64
	ReservationItemID *int64
	Qty               string
	IdempotencyKey    string
	SourceType        *string
	SourceID          *string
	SourceLineID      *string
	Actor             *Actor
}

type IdempotencyClaim struct {
	OperationType  string
	Key            string
	RequestHash    string
	RequestPayload []byte
}

type IdempotencyRecord struct {
	ID              int64
	OperationType   string
	Key             string
	RequestHash     string
	RequestPayload  []byte
	ResponsePayload []byte
	Status          string
}

type IdempotencyCompletion struct {
	ID              int64
	ResponseRefType *string
	ResponseRefID   *int64
	ResponsePayload []byte
}
