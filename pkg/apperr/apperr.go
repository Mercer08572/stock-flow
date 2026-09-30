// Package apperr 定义业务错误的统一载体。
//
// 设计目标：把「错误的业务身份」从英文散文里解放出来，变成机器可判别的错误码。
//
//	Status   HTTP 状态，唯一事实来源（响应封装里的数字 code 由它派生）
//	Code     稳定的业务错误码，前端据此选择文案
//	Message  默认英文文案，前端字典未命中时回退到它
//
// 本包只依赖标准库：service / repository 层可以使用它，而不必知道 gin。
// 把错误写成 HTTP 响应的动作在 internal/shared/httperr。
package apperr

import (
	"errors"
	"net/http"
)

// Code 是稳定的业务错误码，与 HTTP 状态解耦。
type Code string

// Error 是业务错误的统一载体。
type Error struct {
	Status  int
	Code    Code
	Message string
}

func (e *Error) Error() string { return e.Message }

// Is 让 errors.Is 按「业务码等价」匹配，而不是按指针身份。
//
// 好处：同一业务码的错误即使在不同模块、不同层被重新构造，也能被 errors.Is 命中；
// 而 Code 为空的错误之间不会互相误匹配。
func (e *Error) Is(target error) bool {
	other, ok := target.(*Error)
	return ok && e.Code != "" && e.Code == other.Code
}

// New 是构造业务错误的原语；下面 6 个具名构造器覆盖常见状态，可读性更好。
func New(status int, code Code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// BadRequest 用于「语义上不合法但请求格式合法」的输入，例如引用了不存在的分类。
func BadRequest(code Code, message string) *Error {
	return New(http.StatusBadRequest, code, message)
}

func Unauthorized(code Code, message string) *Error {
	return New(http.StatusUnauthorized, code, message)
}

func Forbidden(code Code, message string) *Error {
	return New(http.StatusForbidden, code, message)
}

func NotFound(code Code, message string) *Error {
	return New(http.StatusNotFound, code, message)
}

func Conflict(code Code, message string) *Error {
	return New(http.StatusConflict, code, message)
}

func TooManyRequests(code Code, message string) *Error {
	return New(http.StatusTooManyRequests, code, message)
}

// ValidationError 表示「本层输入校验失败」：具体原因在 message 里，没有业务码。
//
// 它与 Error 刻意分开：Error 是「已知的、可安全暴露给客户端的业务错误」，
// ValidationError 是「输入不合法，细节只能靠文案表达」（如 "code is required"）。
// 需要区分两者时用 IsValidationError，不要去看 Code 是否为空。
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func NewValidationError(message string) error {
	return &ValidationError{Message: message}
}

func IsValidationError(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}
