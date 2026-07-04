package warehouse

import "errors"

var (
	ErrNotFound      = errors.New("warehouse not found")
	ErrDuplicateCode = errors.New("warehouse code already exists")
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
