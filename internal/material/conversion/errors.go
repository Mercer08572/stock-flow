package conversion

import "github.com/Mercer08572/stock-flow/pkg/apperr"

var (
	ErrNotFound         = apperr.NotFound(apperr.CodeMaterialConversionNotFound, "material unit conversion not found")
	ErrDuplicatePair    = apperr.Conflict(apperr.CodeMaterialConversionDuplicate, "material unit conversion already exists")
	ErrReversePair      = apperr.Conflict(apperr.CodeMaterialConversionReverseDuplicate, "reverse material unit conversion exists; the base unit must be the from unit")
	ErrMaterialNotFound = apperr.BadRequest(apperr.CodeMaterialConversionMaterialInvalid, "material not found")
	ErrFromUnitNotFound = apperr.BadRequest(apperr.CodeMaterialConversionFromUnitInvalid, "from unit not found")
	ErrToUnitNotFound   = apperr.BadRequest(apperr.CodeMaterialConversionToUnitInvalid, "to unit not found")
	ErrUnitTypeMismatch = apperr.BadRequest(apperr.CodeMaterialConversionUnitTypeMismatch, "unit types are not commensurable")
	ErrBaseUnitRequired = apperr.BadRequest(apperr.CodeMaterialConversionBaseUnitRequired, "one side of the conversion must be the material base unit")
)
