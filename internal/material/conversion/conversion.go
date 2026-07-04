package conversion

import "time"

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
	Limit  int32                    `json:"limit"`
	Offset int32                    `json:"offset"`
}
