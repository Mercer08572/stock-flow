package inventory

import "time"

const (
	DefaultListLimit int32 = 20
	MaxListLimit     int32 = 100
)

type StockBalance struct {
	WarehouseID  int64        `json:"warehouse_id"`
	SKUID        int64        `json:"sku_id"`
	OnHandQty    string       `json:"on_hand_qty"`
	ReservedQty  string       `json:"reserved_qty"`
	AvailableQty string       `json:"available_qty"`
	UpdatedAt    *time.Time   `json:"updated_at"`
	Layers       []StockLayer `json:"layers,omitempty"`
}

type StockLayer struct {
	ID           int64     `json:"id"`
	WarehouseID  int64     `json:"warehouse_id"`
	SKUID        int64     `json:"sku_id"`
	BatchID      *int64    `json:"batch_id"`
	ReceivedAt   time.Time `json:"received_at"`
	OnHandQty    string    `json:"on_hand_qty"`
	ReservedQty  string    `json:"reserved_qty"`
	AvailableQty string    `json:"available_qty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
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
