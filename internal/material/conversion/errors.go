package conversion

import "errors"

var (
	ErrNotFound         = errors.New("material unit conversion not found")
	ErrDuplicatePair    = errors.New("material unit conversion already exists")
	ErrReversePair      = errors.New("reverse material unit conversion already exists")
	ErrMaterialNotFound = errors.New("material not found")
	ErrFromUnitNotFound = errors.New("from unit not found")
	ErrToUnitNotFound   = errors.New("to unit not found")
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
