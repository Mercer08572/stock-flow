package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	TraceIDKey     = "trace_id"
	defaultMessage = "success"
)

const (
	CodeSuccess       = 200
	CodeBadRequest    = 1001
	CodeUnauthorized  = 1002
	CodeForbidden     = 1003
	CodeNotFound      = 1004
	CodeConflict      = 1009
	CodeInternalError = 1500
)

// CodeForStatus 把 HTTP 状态映射为响应封装里的粗粒度数字 code。
//
// 收敛到一处，避免每个错误出口各写各的。历史上 429 沿用 1001（CodeBadRequest），
// 这里刻意保持该行为不做「顺手修正」。
func CodeForStatus(status int) int {
	switch status {
	case http.StatusBadRequest:
		return CodeBadRequest
	case http.StatusUnauthorized:
		return CodeUnauthorized
	case http.StatusForbidden:
		return CodeForbidden
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusConflict:
		return CodeConflict
	case http.StatusTooManyRequests:
		return CodeBadRequest
	default:
		return CodeInternalError
	}
}

type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	// ErrorCode 是稳定的业务错误码（见 pkg/apperr）。成功响应与无业务身份的错误不出现。
	ErrorCode string `json:"error_code,omitempty"`
	Data      any    `json:"data"`
	TraceID   string `json:"trace_id"`
	Timestamp int64  `json:"timestamp"`
}

func Success(c *gin.Context, data any) {
	JSON(c, http.StatusOK, CodeSuccess, defaultMessage, data)
}

func Created(c *gin.Context, data any) {
	JSON(c, http.StatusCreated, CodeSuccess, defaultMessage, data)
}

func NoContent(c *gin.Context) {
	JSON(c, http.StatusOK, CodeSuccess, defaultMessage, nil)
}

func Error(c *gin.Context, httpStatus int, code int, message string) {
	write(c, httpStatus, code, "", message, nil)
}

// Fail 是业务错误的统一出口：在信封里带上稳定的业务错误码（见 pkg/apperr）。
// 无业务身份的错误（如请求格式非法、内部错误）继续用 Error。
func Fail(c *gin.Context, httpStatus int, code int, errorCode string, message string) {
	write(c, httpStatus, code, errorCode, message, nil)
}

func JSON(c *gin.Context, httpStatus int, code int, message string, data any) {
	write(c, httpStatus, code, "", message, data)
}

func write(c *gin.Context, httpStatus int, code int, errorCode string, message string, data any) {
	if message == "" {
		message = http.StatusText(httpStatus)
	}

	c.JSON(httpStatus, Body{
		Code:      code,
		Message:   message,
		ErrorCode: errorCode,
		Data:      data,
		TraceID:   TraceID(c),
		Timestamp: time.Now().UnixMilli(),
	})
}

func TraceID(c *gin.Context) string {
	if c == nil {
		return ""
	}

	if value, exists := c.Get(TraceIDKey); exists {
		if traceID, ok := value.(string); ok {
			return traceID
		}
	}

	return c.GetHeader("X-Trace-ID")
}
