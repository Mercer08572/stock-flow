package material

import "errors"

var (
	ErrNotFound         = errors.New("material not found")
	ErrDuplicateCode    = errors.New("material code already exists")
	ErrCategoryNotFound = errors.New("material category not found")
	ErrBaseUnitNotFound = errors.New("material base unit not found")
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
