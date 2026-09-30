package category

import "github.com/Mercer08572/stock-flow/pkg/apperr"

var (
	ErrNotFound       = apperr.NotFound(apperr.CodeMaterialCategoryNotFound, "material category not found")
	ErrDuplicateCode  = apperr.Conflict(apperr.CodeMaterialCategoryCodeDuplicate, "material category code already exists")
	ErrParentNotFound = apperr.BadRequest(apperr.CodeMaterialCategoryParentNotFound, "parent material category not found")
)
