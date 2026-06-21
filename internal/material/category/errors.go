package category

import "errors"

var (
	ErrNotFound       = errors.New("material category not found")
	ErrDuplicateCode  = errors.New("material category code already exists")
	ErrParentNotFound = errors.New("parent material category not found")
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
