package warehouse

import "github.com/Mercer08572/stock-flow/pkg/apperr"

var (
	ErrNotFound              = apperr.NotFound(apperr.CodeWarehouseNotFound, "warehouse not found")
	ErrDuplicateCode         = apperr.Conflict(apperr.CodeWarehouseCodeDuplicate, "warehouse code already exists")
	ErrReferencedByInventory = apperr.Conflict(apperr.CodeWarehouseReferencedByInventory, "warehouse is referenced by inventory and cannot be deleted")
)
