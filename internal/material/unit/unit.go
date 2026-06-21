package unit

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

type UnitType string

const (
	UnitTypeCount   UnitType = "count"
	UnitTypeWeight  UnitType = "weight"
	UnitTypeLength  UnitType = "length"
	UnitTypeArea    UnitType = "area"
	UnitTypeVolume  UnitType = "volume"
	UnitTypePackage UnitType = "package"
	UnitTypeTime    UnitType = "time"
	UnitTypeOther   UnitType = "other"
)

type Unit struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Symbol    string    `json:"symbol"`
	UnitType  UnitType  `json:"unit_type"`
	Precision int32     `json:"precision"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateUnitInput struct {
	Code      string
	Name      string
	Symbol    string
	UnitType  UnitType
	Precision int32
	Status    Status
}

type UpdateUnitInput struct {
	ID        int64
	Code      string
	Name      string
	Symbol    string
	UnitType  UnitType
	Precision int32
	Status    Status
}

type UnitListFilter struct {
	Status   *Status
	UnitType *UnitType
	Limit    int32
	Offset   int32
}

type UnitListResult struct {
	Items  []Unit `json:"items"`
	Limit  int32  `json:"limit"`
	Offset int32  `json:"offset"`
}

func (t UnitType) IsValid() bool {
	switch t {
	case UnitTypeCount, UnitTypeWeight, UnitTypeLength, UnitTypeArea, UnitTypeVolume, UnitTypePackage, UnitTypeTime, UnitTypeOther:
		return true
	default:
		return false
	}
}

func (s Status) IsValid() bool {
	return s == StatusActive || s == StatusInactive
}
