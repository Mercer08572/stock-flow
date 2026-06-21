package unit

import "errors"

var (
	ErrNotFound      = errors.New("unit not found")
	ErrDuplicateCode = errors.New("unit code already exists")
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
