package sku

import (
	"time"

	"github.com/Mercer08572/stock-flow/internal/shared/reference"
)

const (
	DefaultListLimit int32 = 20
	MaxListLimit     int32 = 100
)

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

type SKU struct {
	ID         int64     `json:"id"`
	MaterialID int64     `json:"material_id"`
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	UnitID     int64     `json:"unit_id"`
	Status     Status    `json:"status"`
	Remark     *string   `json:"remark"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	// 列表与详情接口内联的引用快照，供客户端直接展示，无需再按 ID 二次查询。
	// 写入接口不返回这两个字段。
	Material *reference.Reference `json:"material,omitempty"`
	Unit     *reference.Reference `json:"unit,omitempty"`
}

// Reference 是 SKU 的最小可读快照，形状由 shared/reference 统一。
type Reference = reference.Reference

type CreateInput struct {
	MaterialID int64
	Code       string
	Name       string
	UnitID     int64
	Status     Status
	Remark     *string
}

type UpdateInput struct {
	ID         int64
	MaterialID int64
	Code       string
	Name       string
	UnitID     int64
	Status     Status
	Remark     *string
}

type ListFilter struct {
	Status     *Status
	MaterialID *int64
	UnitID     *int64
	Limit      int32
	Offset     int32
}

type ListResult struct {
	Items  []SKU `json:"items"`
	Total  int64 `json:"total"`
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

func (s Status) IsValid() bool {
	return s == StatusActive || s == StatusInactive
}
