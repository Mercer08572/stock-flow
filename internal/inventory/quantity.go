package inventory

import (
	"math/big"
	"regexp"
	"strings"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

// quantityPattern 限制请求数量：最多 14 位整数、最多 6 位小数，匹配 NUMERIC(20,6)。
var quantityPattern = regexp.MustCompile(`^[0-9]{1,14}(\.[0-9]{1,6})?$`)

// parseQuantity 校验并解析请求中的正数量。
func parseQuantity(value string) (*big.Rat, error) {
	trimmed := strings.TrimSpace(value)
	if !quantityPattern.MatchString(trimmed) {
		return nil, apperr.NewValidationError("qty must be a decimal string with at most 14 integer digits and 6 decimal places")
	}

	quantity, ok := new(big.Rat).SetString(trimmed)
	if !ok {
		return nil, apperr.NewValidationError("qty must be a decimal string")
	}
	if quantity.Sign() <= 0 {
		return nil, apperr.NewValidationError("qty must be greater than zero")
	}

	return quantity, nil
}

// parseStoredQuantity 解析数据库中的非负数量，不做精度位数限制。
func parseStoredQuantity(value string) (*big.Rat, error) {
	quantity, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok || quantity.Sign() < 0 {
		return nil, apperr.NewValidationError("inventory quantity is invalid")
	}

	return quantity, nil
}

// normalizeQuantity 返回去除末尾零的数量字符串，用于数据库与响应。
func normalizeQuantity(value string) (string, error) {
	quantity, err := parseStoredQuantity(value)
	if err != nil {
		return "", err
	}

	return formatQuantity(quantity), nil
}

// canonicalQuantity 返回固定 6 位小数的数量字符串，用于幂等请求哈希。
func canonicalQuantity(quantity *big.Rat) string {
	return quantity.FloatString(6)
}

func formatQuantity(quantity *big.Rat) string {
	return trimDecimalZeros(quantity.FloatString(6))
}

func addQuantity(left string, right string) (string, error) {
	leftQty, err := parseStoredQuantity(left)
	if err != nil {
		return "", err
	}
	rightQty, err := parseStoredQuantity(right)
	if err != nil {
		return "", err
	}

	return formatQuantity(new(big.Rat).Add(leftQty, rightQty)), nil
}

func subtractQuantity(left string, right string) (string, error) {
	leftQty, err := parseStoredQuantity(left)
	if err != nil {
		return "", err
	}
	rightQty, err := parseStoredQuantity(right)
	if err != nil {
		return "", err
	}

	result := new(big.Rat).Sub(leftQty, rightQty)
	if result.Sign() < 0 {
		return "", apperr.NewValidationError("inventory quantity would become negative")
	}

	return formatQuantity(result), nil
}

func compareQuantity(left string, right string) (int, error) {
	leftQty, err := parseStoredQuantity(left)
	if err != nil {
		return 0, err
	}
	rightQty, err := parseStoredQuantity(right)
	if err != nil {
		return 0, err
	}

	return leftQty.Cmp(rightQty), nil
}

func minQuantity(left *big.Rat, right *big.Rat) *big.Rat {
	if left.Cmp(right) <= 0 {
		return new(big.Rat).Set(left)
	}
	return new(big.Rat).Set(right)
}
