package category

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

type Category struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	ParentID  *int64    `json:"parent_id"`
	Status    Status    `json:"status"`
	Remark    *string   `json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateCategoryInput struct {
	Code     string
	Name     string
	ParentID *int64
	Status   Status
	Remark   *string
}

type UpdateCategoryInput struct {
	ID       int64
	Code     string
	Name     string
	ParentID *int64
	Status   Status
	Remark   *string
}

type CategoryListFilter struct {
	Status   *Status
	ParentID *int64
	Limit    int32
	Offset   int32
}

type CategoryListResult struct {
	Items  []Category `json:"items"`
	Limit  int32      `json:"limit"`
	Offset int32      `json:"offset"`
}

func (s Status) IsValid() bool {
	return s == StatusActive || s == StatusInactive
}
