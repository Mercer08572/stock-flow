package warehouse

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

type Type string

const (
	TypeNormal  Type = "normal"
	TypeVirtual Type = "virtual"
)

type Warehouse struct {
	ID           int64     `json:"id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	Type         Type      `json:"type"`
	Status       Status    `json:"status"`
	Location     *string   `json:"location"`
	ContactName  *string   `json:"contact_name"`
	ContactPhone *string   `json:"contact_phone"`
	Remark       *string   `json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Reference struct {
	ID      int64  `json:"id"`
	Code    string `json:"code"`
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
}

type CreateInput struct {
	Code         string
	Name         string
	Type         Type
	Status       Status
	Location     *string
	ContactName  *string
	ContactPhone *string
	Remark       *string
}

type UpdateInput struct {
	ID           int64
	Code         string
	Name         string
	Type         Type
	Status       Status
	Location     *string
	ContactName  *string
	ContactPhone *string
	Remark       *string
}

type ListFilter struct {
	Status *Status
	Type   *Type
	Limit  int32
	Offset int32
}

type ListResult struct {
	Items  []Warehouse `json:"items"`
	Limit  int32       `json:"limit"`
	Offset int32       `json:"offset"`
}

func (s Status) IsValid() bool {
	return s == StatusActive || s == StatusInactive
}

func (t Type) IsValid() bool {
	return t == TypeNormal || t == TypeVirtual
}
