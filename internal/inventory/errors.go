package inventory

import "github.com/Mercer08572/stock-flow/pkg/apperr"

var (
	ErrNotFound          = apperr.NotFound(apperr.CodeInventoryStockNotFound, "inventory stock not found")
	ErrWarehouseNotFound = apperr.BadRequest(apperr.CodeInventoryWarehouseNotFound, "inventory warehouse not found")
	ErrWarehouseInactive = apperr.Conflict(apperr.CodeInventoryWarehouseInactive, "inventory warehouse is inactive")
	ErrSKUNotFound       = apperr.BadRequest(apperr.CodeInventorySKUNotFound, "inventory sku not found")
	ErrBatchNotFound     = apperr.BadRequest(apperr.CodeInventoryBatchNotFound, "inventory batch not found for sku")
	ErrBatchInactive     = apperr.BadRequest(apperr.CodeInventoryBatchInactive, "inventory batch is inactive")

	ErrInsufficientAvailable = apperr.Conflict(apperr.CodeInventoryInsufficientAvailable, "insufficient available inventory")
	ErrInsufficientReserved  = apperr.Conflict(apperr.CodeInventoryInsufficientReserved, "insufficient reserved inventory")

	ErrReservationNotFound  = apperr.NotFound(apperr.CodeInventoryReservationNotFound, "inventory reservation not found")
	ErrReservationNotActive = apperr.Conflict(apperr.CodeInventoryReservationNotActive, "inventory reservation is not active")
	ErrReservationMismatch  = apperr.BadRequest(apperr.CodeInventoryReservationMismatch, "inventory reservation does not match warehouse and sku")

	ErrIdempotencyConflict = apperr.Conflict(apperr.CodeInventoryIdempotencyConflict, "idempotency key was already used with a different request")
	ErrOperationInProgress = apperr.Conflict(apperr.CodeInventoryOperationInProgress, "inventory operation with the same idempotency key is still in progress")
)
