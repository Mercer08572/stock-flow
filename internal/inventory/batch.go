package inventory

import (
	"strings"
	"time"

	"github.com/Mercer08572/stock-flow/internal/sku"
	"github.com/Mercer08572/stock-flow/internal/warehouse"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

// DateLayout 是批次效期类字段的对外日期格式。
//
// 这些字段是日历日而不是时间点，因此接口层统一用 `YYYY-MM-DD` 字符串，
// 避免客户端按本地时区解析 RFC3339 时出现「差一天」。
const DateLayout = "2006-01-02"

const (
	// MaxBatchNoLength 是批号的最大字符数（按 rune 计）。
	MaxBatchNoLength = 64
	// DefaultDispatchPriority 与数据库默认值一致：10 表示不提升也不延后。
	DefaultDispatchPriority int32 = 10
)

// BatchStatus 是批次状态。
type BatchStatus string

const (
	BatchStatusActive   BatchStatus = "active"
	BatchStatusInactive BatchStatus = "inactive"
)

// IsValid 判断状态是否属于允许的枚举值。
func (s BatchStatus) IsValid() bool {
	return s == BatchStatusActive || s == BatchStatusInactive
}

// Batch 是批次主数据。
//
// 它没有数量字段，也没有 warehouse_id：批次库存只能通过在库存层上按 batch_id
// 过滤得到。ProductionDate / ExpirationDate 使用 `YYYY-MM-DD` 字符串。
type Batch struct {
	ID               int64          `json:"id"`
	SKUID            int64          `json:"sku_id"`
	SKU              *sku.Reference `json:"sku,omitempty"`
	BatchNo          string         `json:"batch_no"`
	FirstReceivedAt  *time.Time     `json:"first_received_at"`
	ProductionDate   *string        `json:"production_date"`
	ExpirationDate   *string        `json:"expiration_date"`
	Status           BatchStatus    `json:"status"`
	DispatchPriority int32          `json:"dispatch_priority"`
	Remark           *string        `json:"remark"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// BatchWarehouseStock 是某批次在一个仓库里的库存分布。
//
// 批次库存在库存层上按 batch_id 归组得到，不是批次表上的字段。
type BatchWarehouseStock struct {
	WarehouseID  int64                `json:"warehouse_id"`
	Warehouse    *warehouse.Reference `json:"warehouse,omitempty"`
	OnHandQty    string               `json:"on_hand_qty"`
	ReservedQty  string               `json:"reserved_qty"`
	AvailableQty string               `json:"available_qty"`
	LayerCount   int64                `json:"layer_count"`
}

// BatchDetail 是批次详情：主数据 + 按仓库聚合的库存分布。
type BatchDetail struct {
	Batch
	Stocks []BatchWarehouseStock `json:"stocks"`
}

// ListBatchesFilter 是批次列表的查询条件。
type ListBatchesFilter struct {
	SKUID   *int64
	Status  *BatchStatus
	Keyword *string
	Limit   int32
	Offset  int32
}

// BatchListResult 是批次列表的分页结果。
type BatchListResult struct {
	Items  []Batch `json:"items"`
	Total  int64   `json:"total"`
	Limit  int32   `json:"limit"`
	Offset int32   `json:"offset"`
}

// CreateBatchInput 是新增批次的输入。
type CreateBatchInput struct {
	SKUID            int64
	BatchNo          string
	ProductionDate   *string
	ExpirationDate   *string
	Status           BatchStatus
	DispatchPriority *int32
	Remark           *string
}

// UpdateBatchInput 是编辑批次的输入。
//
// SKUID 与 BatchNo 不在可更新字段内：它们是批次的业务标识，创建后不可修改。
type UpdateBatchInput struct {
	ID               int64
	ProductionDate   *string
	ExpirationDate   *string
	Status           *BatchStatus
	DispatchPriority *int32
	Remark           *string
}

// normalizeBatchNo 负责去空白、必填与长度校验。
func normalizeBatchNo(raw string) (string, error) {
	batchNo := strings.TrimSpace(raw)
	if batchNo == "" {
		return "", apperr.NewValidationError("batch_no is required")
	}
	if len([]rune(batchNo)) > MaxBatchNoLength {
		return "", apperr.NewValidationError("batch_no must be at most 64 characters")
	}
	return batchNo, nil
}

// normalizeOptionalDate 校验 `YYYY-MM-DD` 并返回规范化后的值；空值表示清空。
func normalizeOptionalDate(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(DateLayout, value)
	if err != nil {
		return nil, apperr.NewValidationError("date must use YYYY-MM-DD format")
	}
	normalized := parsed.Format(DateLayout)
	return &normalized, nil
}

// validateBatchDateRange 与数据库约束保持一致：有效期不得早于生产日期。
func validateBatchDateRange(productionDate, expirationDate *string) error {
	if productionDate == nil || expirationDate == nil {
		return nil
	}
	if *expirationDate < *productionDate {
		return apperr.NewValidationError("expiration_date must not be earlier than production_date")
	}
	return nil
}

func normalizeBatchStatus(raw BatchStatus) (BatchStatus, error) {
	status := BatchStatus(strings.TrimSpace(string(raw)))
	if status == "" {
		return BatchStatusActive, nil
	}
	if !status.IsValid() {
		return "", apperr.NewValidationError("status must be active or inactive")
	}
	return status, nil
}

func normalizeDispatchPriority(raw *int32) (int32, error) {
	if raw == nil {
		return DefaultDispatchPriority, nil
	}
	if *raw < 0 {
		return 0, apperr.NewValidationError("dispatch_priority must be greater than or equal to zero")
	}
	return *raw, nil
}

func normalizeRemark(raw *string) *string {
	if raw == nil {
		return nil
	}
	remark := strings.TrimSpace(*raw)
	if remark == "" {
		return nil
	}
	return &remark
}
