package sku

import "errors"

var (
	ErrNotFound              = errors.New("sku not found")
	ErrDuplicateCode         = errors.New("sku code already exists")
	ErrActiveSKUForMaterial  = errors.New("active sku already exists for material")
	ErrMaterialNotFound      = errors.New("sku material not found")
	ErrUnitNotFound          = errors.New("sku unit not found")
	ErrInvalidUnit           = errors.New("sku unit is not allowed for material")
	ErrReferencedByInventory = errors.New("sku is referenced by inventory and cannot be deleted")
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func NewValidationError(message string) error {
	return &ValidationError{Message: message}
}

func IsValidationError(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}
