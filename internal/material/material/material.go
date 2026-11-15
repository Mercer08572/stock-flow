package material

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

type Material struct {
	ID         int64     `json:"id"`
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	CategoryID int64     `json:"category_id"`
	BaseUnitID int64     `json:"base_unit_id"`
	Status     Status    `json:"status"`
	Remark     *string   `json:"remark"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	// 列表与详情接口内联的引用快照，供客户端直接展示，无需再按 ID 二次查询。
	// 写入接口不返回这两个字段。
	Category *reference.Reference `json:"category,omitempty"`
	BaseUnit *reference.Reference `json:"base_unit,omitempty"`
}

type CreateInput struct {
	Code       string
	Name       string
	CategoryID int64
	BaseUnitID int64
	Status     Status
	Remark     *string
}

type UpdateInput struct {
	ID         int64
	Code       string
	Name       string
	CategoryID int64
	BaseUnitID int64
	Status     Status
	Remark     *string
}

type ListFilter struct {
	Status     *Status
	CategoryID *int64
	Limit      int32
	Offset     int32
}

type ListResult struct {
	Items  []Material `json:"items"`
	Total  int64      `json:"total"`
	Limit  int32      `json:"limit"`
	Offset int32      `json:"offset"`
}

func (s Status) IsValid() bool {
	return s == StatusActive || s == StatusInactive
}
