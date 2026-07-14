package warehouse

import "errors"

var (
	ErrNotFound              = errors.New("warehouse not found")
	ErrDuplicateCode         = errors.New("warehouse code already exists")
	ErrReferencedByInventory = errors.New("warehouse is referenced by inventory and cannot be deleted")
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
