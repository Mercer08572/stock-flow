package sku

import "github.com/Mercer08572/stock-flow/pkg/apperr"

var (
	ErrNotFound              = apperr.NotFound(apperr.CodeSKUNotFound, "sku not found")
	ErrDuplicateCode         = apperr.Conflict(apperr.CodeSKUCodeDuplicate, "sku code already exists")
	ErrActiveSKUForMaterial  = apperr.Conflict(apperr.CodeSKUActiveExistsForMaterial, "active sku already exists for material")
	ErrReferencedByInventory = apperr.Conflict(apperr.CodeSKUReferencedByInventory, "sku is referenced by inventory and cannot be deleted")
	ErrMaterialNotFound      = apperr.BadRequest(apperr.CodeSKUMaterialNotFound, "sku material not found")
	ErrUnitNotFound          = apperr.BadRequest(apperr.CodeSKUUnitNotFound, "sku unit not found")
	ErrInvalidUnit           = apperr.BadRequest(apperr.CodeSKUUnitNotAllowed, "sku unit is not allowed for material")
)
