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
	CodeNotFound      = 1004
	CodeConflict      = 1009
	CodeInternalError = 1500
)

type Body struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
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
	JSON(c, httpStatus, code, message, nil)
}

func JSON(c *gin.Context, httpStatus int, code int, message string, data any) {
	if message == "" {
		message = http.StatusText(httpStatus)
	}

	c.JSON(httpStatus, Body{
		Code:      code,
		Message:   message,
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
