package inventory

import "github.com/Mercer08572/stock-flow/pkg/apperr"

var (
	ErrNotFound          = apperr.NotFound(apperr.CodeInventoryStockNotFound, "inventory stock not found")
	ErrWarehouseNotFound = apperr.BadRequest(apperr.CodeInventoryWarehouseNotFound, "inventory warehouse not found")
	ErrSKUNotFound       = apperr.BadRequest(apperr.CodeInventorySKUNotFound, "inventory sku not found")
	ErrBatchNotFound     = apperr.BadRequest(apperr.CodeInventoryBatchNotFound, "inventory batch not found for sku")
)
