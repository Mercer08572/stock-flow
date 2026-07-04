package sku

import "time"

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
}

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
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

func (s Status) IsValid() bool {
	return s == StatusActive || s == StatusInactive
}
