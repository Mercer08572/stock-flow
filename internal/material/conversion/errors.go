package conversion

import "github.com/Mercer08572/stock-flow/pkg/apperr"

var (
	ErrNotFound         = apperr.NotFound(apperr.CodeMaterialConversionNotFound, "material unit conversion not found")
	ErrDuplicatePair    = apperr.Conflict(apperr.CodeMaterialConversionDuplicate, "material unit conversion already exists")
	ErrReversePair      = apperr.Conflict(apperr.CodeMaterialConversionReverseDuplicate, "reverse material unit conversion already exists")
	ErrMaterialNotFound = apperr.BadRequest(apperr.CodeMaterialConversionMaterialInvalid, "material not found")
	ErrFromUnitNotFound = apperr.BadRequest(apperr.CodeMaterialConversionFromUnitInvalid, "from unit not found")
	ErrToUnitNotFound   = apperr.BadRequest(apperr.CodeMaterialConversionToUnitInvalid, "to unit not found")
)
