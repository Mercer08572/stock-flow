// Package httperr 是全项目唯一的错误出口。
//
// 业务模块不再各自维护「错误 → HTTP 响应」的映射 switch：领域错误自带状态与业务码
// （见 pkg/apperr），这里负责把它写成统一响应。
package httperr

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
	"github.com/Mercer08572/stock-flow/pkg/response"
)

// Write 把领域错误写成统一响应格式。
//
// 无法识别的错误一律按 500 处理且不暴露内部细节；真实原因附加到 gin 的错误链上
// （而不是静默丢弃），便于日志中间件或排障时取用。
func Write(c *gin.Context, err error) {
	if headers, ok := headersOf(err); ok {
		for name, value := range headers {
			c.Header(name, value)
		}
	}

	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		response.Fail(c, appErr.Status, response.CodeForStatus(appErr.Status), string(appErr.Code), appErr.Message)
		return
	}

	// 输入校验失败：没有业务码，具体原因只能靠文案表达。
	var invalidInput *apperr.ValidationError
	if errors.As(err, &invalidInput) {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "", invalidInput.Error())
		return
	}

	_ = c.Error(err)
	response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
}

// headered 由需要附加响应头的错误实现（目前只有 auth 的登录限流需要 Retry-After）。
// 接口在使用方就地定义，避免把各模块的细节塞进 apperr。
type headered interface {
	Headers() map[string]string
}

func headersOf(err error) (map[string]string, bool) {
	var h headered
	if errors.As(err, &h) {
		return h.Headers(), true
	}
	return nil, false
}
