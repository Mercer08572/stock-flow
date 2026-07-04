package inventory

import "errors"

var (
	ErrNotFound          = errors.New("inventory stock not found")
	ErrWarehouseNotFound = errors.New("inventory warehouse not found")
	ErrSKUNotFound       = errors.New("inventory sku not found")
	ErrBatchNotFound     = errors.New("inventory batch not found for sku")
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
