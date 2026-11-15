package conversion

import (
	"time"

	"github.com/Mercer08572/stock-flow/internal/shared/reference"
)

const (
	DefaultListLimit int32 = 20
	MaxListLimit     int32 = 100
)

type MaterialUnitConversion struct {
	ID         int64     `json:"id"`
	MaterialID int64     `json:"material_id"`
	FromUnitID int64     `json:"from_unit_id"`
	ToUnitID   int64     `json:"to_unit_id"`
	Factor     string    `json:"factor"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	// 两端单位的引用快照，供客户端直接展示换算关系，无需再按 ID 二次查询。
	FromUnit *reference.Reference `json:"from_unit,omitempty"`
	ToUnit   *reference.Reference `json:"to_unit,omitempty"`
}

type CreateInput struct {
	MaterialID int64
	FromUnitID int64
	ToUnitID   int64
	Factor     string
}

type UpdateInput struct {
	ID         int64
	MaterialID int64
	FromUnitID int64
	ToUnitID   int64
	Factor     string
}

type ListFilter struct {
	MaterialID int64
	FromUnitID *int64
	ToUnitID   *int64
	Limit      int32
	Offset     int32
}

type ListResult struct {
	Items  []MaterialUnitConversion `json:"items"`
	Total  int64                    `json:"total"`
	Limit  int32                    `json:"limit"`
	Offset int32                    `json:"offset"`
}
