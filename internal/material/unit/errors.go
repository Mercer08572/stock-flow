package unit

import "github.com/Mercer08572/stock-flow/pkg/apperr"

var (
	ErrNotFound      = apperr.NotFound(apperr.CodeMaterialUnitNotFound, "unit not found")
	ErrDuplicateCode = apperr.Conflict(apperr.CodeMaterialUnitCodeDuplicate, "unit code already exists")
)
