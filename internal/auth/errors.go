package auth

import (
	"errors"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

var (
	// 三个 401 错误的对外文案刻意保持扁平化：历史上它们都被写成 "authentication failed"，
	// 这里沿用同一文案以保证 API 可观察输出不变，改用业务码区分具体原因。
	ErrInvalidCredentials = apperr.Unauthorized(apperr.CodeAuthInvalidCredentials, "authentication failed")
	ErrUnauthorized       = apperr.Unauthorized(apperr.CodeAuthRequired, "authentication failed")
	ErrSessionExpired     = apperr.Unauthorized(apperr.CodeAuthSessionExpired, "authentication failed")

	ErrAmbiguousAuth = apperr.BadRequest(apperr.CodeAuthAmbiguousMethod, "request must use only one authentication method")

	// 以下三个错误只由 cmd/admin 引导流程使用，不经 HTTP 出口，因此保持普通错误。
	ErrAdminNotFound           = errors.New("admin user not found")
	ErrAdminNotInitialized     = errors.New("admin password is not initialized")
	ErrAdminAlreadyInitialized = errors.New("admin password is already initialized")

	ErrPasswordChangeRequired = apperr.Forbidden(apperr.CodeAuthPasswordChangeRequired, "administrator password change required")
	ErrRateLimited            = apperr.TooManyRequests(apperr.CodeAuthRateLimited, "too many login attempts")
	ErrAPIAppNotFound         = apperr.NotFound(apperr.CodeAuthAPIAppNotFound, "api app not found")
	ErrAPISecretNotFound      = apperr.NotFound(apperr.CodeAuthAPISecretNotFound, "api secret not found")
	ErrDuplicateAppID         = apperr.Conflict(apperr.CodeAuthAPIAppDuplicate, "api app identifier already exists")
	ErrDuplicateSecretID      = apperr.Conflict(apperr.CodeAuthAPISecretDuplicate, "api secret identifier already exists")
)
