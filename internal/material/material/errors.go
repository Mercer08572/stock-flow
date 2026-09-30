package material

import (
	"errors"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

var (
	ErrNotFound          = apperr.NotFound(apperr.CodeMaterialNotFound, "material not found")
	ErrDuplicateCode     = apperr.Conflict(apperr.CodeMaterialCodeDuplicate, "material code already exists")
	ErrCategoryNotFound  = apperr.BadRequest(apperr.CodeMaterialCategoryReferenceInvalid, "material category not found")
	ErrBaseUnitNotFound  = apperr.BadRequest(apperr.CodeMaterialBaseUnitReferenceInvalid, "material base unit not found")
	ErrSKUUnitNotAllowed = errors.New("sku unit is not allowed for material")
)
