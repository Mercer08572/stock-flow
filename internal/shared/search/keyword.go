// Package search 提供「列表关键字搜索」的公共归一化逻辑。
//
// 各模块的 List 接口共用同一个查询参数 `q`：按编码或名称做大小写不敏感的子串匹配。
// 归一化集中在这里，避免五个模块各写一份 trim / 长度上限。
package search

import (
	"strings"
	"unicode/utf8"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

// MaxKeywordLength 关键字的最大长度，按 rune 计数（中文名称不会被字节数误伤）。
const MaxKeywordLength = 64

// NormalizeKeyword 归一化查询参数 q：
//   - 未传或只有空白 → nil，表示不过滤；
//   - 超过 MaxKeywordLength → 校验错误（400）。
func NormalizeKeyword(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}

	keyword := strings.TrimSpace(*raw)
	if keyword == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(keyword) > MaxKeywordLength {
		return nil, apperr.NewValidationError("q must be at most 64 characters")
	}

	return &keyword, nil
}
