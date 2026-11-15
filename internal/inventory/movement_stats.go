package inventory

import (
	"context"
	"time"

	"github.com/Mercer08572/stock-flow/internal/shared/statsbucket"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

const (
	// StatsGranularityHour 与 StatsGranularityDay 是趋势统计支持的时间桶粒度。
	StatsGranularityHour = "hour"
	StatsGranularityDay  = "day"

	// DefaultStatsTimezone 是统计桶切分使用的默认时区；与前端展示时区保持一致。
	DefaultStatsTimezone = "Asia/Shanghai"

	// maxStatsRange 限制单次统计的时间跨度，避免一次查询扫过多分区/行。
	maxStatsRange = 90 * 24 * time.Hour
)

// StatsOperationTypes 是趋势统计返回的序列顺序，与前端图例一致。
//
// 顺序是业务语义顺序：增 → 减 → 占用 → 占用扣减 → 占用释放。
var StatsOperationTypes = []string{
	"increase",
	"decrease_available",
	"reserve",
	"decrease_reserved",
	"release_reserved",
}

// MovementStatsFilter 是趋势统计的查询条件。
type MovementStatsFilter struct {
	Granularity string
	Timezone    string
	From        time.Time
	To          time.Time
	WarehouseID *int64
	SKUID       *int64
}

// MovementStatsSeries 是一个操作类型在各时间桶上的调用次数。
type MovementStatsSeries struct {
	OperationType string  `json:"operation_type"`
	Calls         []int64 `json:"calls"`
}

// MovementStats 是趋势统计的完整结果。
//
// Buckets 与每个 Series 的 Calls 等长：无数据的时间桶补 0，而不是缺项，
// 否则折线会在空档处直接连线，视觉上制造出不存在的趋势。
type MovementStats struct {
	Granularity string                `json:"granularity"`
	Timezone    string                `json:"timezone"`
	From        time.Time             `json:"from"`
	To          time.Time             `json:"to"`
	Buckets     []string              `json:"buckets"`
	Series      []MovementStatsSeries `json:"series"`
}

// MovementStatsBucket 是仓储层返回的一个聚合点：某个时间桶内某操作类型的调用次数。
type MovementStatsBucket struct {
	Bucket        time.Time
	OperationType string
	Calls         int64
}

// GetMovementStats 返回库存操作的趋势统计。
func (s *service) GetMovementStats(ctx context.Context, filter MovementStatsFilter) (MovementStats, error) {
	normalized, err := normalizeMovementStatsFilter(filter)
	if err != nil {
		return MovementStats{}, err
	}
	if normalized.WarehouseID != nil {
		if err := s.validateWarehouseForQuery(ctx, *normalized.WarehouseID); err != nil {
			return MovementStats{}, err
		}
	}
	if normalized.SKUID != nil {
		if err := s.validateSKUForQuery(ctx, *normalized.SKUID); err != nil {
			return MovementStats{}, err
		}
	}

	rows, err := s.repo.AggregateMovements(ctx, normalized)
	if err != nil {
		return MovementStats{}, err
	}

	return buildMovementStats(normalized, rows)
}

func normalizeMovementStatsFilter(filter MovementStatsFilter) (MovementStatsFilter, error) {
	if filter.Granularity == "" {
		filter.Granularity = StatsGranularityHour
	}
	switch filter.Granularity {
	case StatsGranularityHour, StatsGranularityDay:
	default:
		return MovementStatsFilter{}, apperr.NewValidationError("granularity must be hour or day")
	}

	if filter.Timezone == "" {
		filter.Timezone = DefaultStatsTimezone
	}
	if _, err := time.LoadLocation(filter.Timezone); err != nil {
		return MovementStatsFilter{}, apperr.NewValidationError("timezone is invalid")
	}

	if filter.From.IsZero() || filter.To.IsZero() {
		return MovementStatsFilter{}, apperr.NewValidationError("from and to are required")
	}
	if !filter.To.After(filter.From) {
		return MovementStatsFilter{}, apperr.NewValidationError("to must be after from")
	}
	if filter.To.Sub(filter.From) > maxStatsRange {
		return MovementStatsFilter{}, apperr.NewValidationError("time range must not exceed 90 days")
	}
	if filter.WarehouseID != nil && *filter.WarehouseID <= 0 {
		return MovementStatsFilter{}, apperr.NewValidationError("warehouse_id must be greater than zero")
	}
	if filter.SKUID != nil && *filter.SKUID <= 0 {
		return MovementStatsFilter{}, apperr.NewValidationError("sku_id must be greater than zero")
	}

	return filter, nil
}

// buildMovementStats 把稀疏的聚合结果补齐成等长的连续时间序列。
func buildMovementStats(filter MovementStatsFilter, rows []MovementStatsBucket) (MovementStats, error) {
	loc, err := time.LoadLocation(filter.Timezone)
	if err != nil {
		return MovementStats{}, apperr.NewValidationError("timezone is invalid")
	}

	buckets, err := statsbucket.Build(filter.From, filter.To, filter.Granularity, loc)
	if err != nil {
		return MovementStats{}, err
	}

	bucketIndex := make(map[int64]int, len(buckets))
	for index, bucket := range buckets {
		bucketIndex[bucket.Unix()] = index
	}

	calls := make(map[string][]int64, len(StatsOperationTypes))
	for _, operationType := range StatsOperationTypes {
		calls[operationType] = make([]int64, len(buckets))
	}

	for _, row := range rows {
		index, ok := bucketIndex[row.Bucket.Unix()]
		if !ok {
			// 落在 [from, to) 之外或非整桶边界的数据不计入，避免污染首尾桶。
			continue
		}
		series, ok := calls[row.OperationType]
		if !ok {
			continue
		}
		series[index] += row.Calls
	}

	series := make([]MovementStatsSeries, 0, len(StatsOperationTypes))
	for _, operationType := range StatsOperationTypes {
		series = append(series, MovementStatsSeries{
			OperationType: operationType,
			Calls:         calls[operationType],
		})
	}

	labels := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		labels = append(labels, bucket.Format(time.RFC3339))
	}

	return MovementStats{
		Granularity: filter.Granularity,
		Timezone:    filter.Timezone,
		From:        filter.From,
		To:          filter.To,
		Buckets:     labels,
		Series:      series,
	}, nil
}
