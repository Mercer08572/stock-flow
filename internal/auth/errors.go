package auth

import "errors"

var (
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrUnauthorized            = errors.New("authentication required")
	ErrSessionExpired          = errors.New("admin session expired")
	ErrAmbiguousAuth           = errors.New("request must use only one authentication method")
	ErrAdminNotFound           = errors.New("admin user not found")
	ErrAdminNotInitialized     = errors.New("admin password is not initialized")
	ErrAdminAlreadyInitialized = errors.New("admin password is already initialized")
	ErrPasswordChangeRequired  = errors.New("administrator password change required")
	ErrRateLimited             = errors.New("too many login attempts")
	ErrAPIAppNotFound          = errors.New("api app not found")
	ErrAPISecretNotFound       = errors.New("api secret not found")
	ErrDuplicateAppID          = errors.New("api app identifier already exists")
	ErrDuplicateSecretID       = errors.New("api secret identifier already exists")
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
