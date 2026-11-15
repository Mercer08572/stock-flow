package audit

import (
	"context"
	"strings"
	"time"

	"github.com/Mercer08572/stock-flow/internal/shared/statsbucket"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

const (
	// StatsGranularityHour 与 StatsGranularityDay 是吞吐统计支持的时间桶粒度。
	StatsGranularityHour = statsbucket.Hour
	StatsGranularityDay  = statsbucket.Day

	// DefaultStatsTimezone 与库存趋势图保持一致，保证两张图的时间轴对齐。
	DefaultStatsTimezone = "Asia/Shanghai"

	maxStatsRange      = 90 * 24 * time.Hour
	maxStatsRouteCount = 20
)

// DefaultStatsRoutes 是库存变更接口的路由模板。
//
// 注意 `POST /inventory/decrease` 同时承载 decrease_available 与 decrease_reserved
// 两种业务操作（由请求体是否携带 reservation_id 区分），审计日志的 route 字段
// 无法再细分，因此吞吐图天然是 4 条路由而不是 5 种操作。
var DefaultStatsRoutes = []string{
	"/api/v1/inventory/increase",
	"/api/v1/inventory/decrease",
	"/api/v1/inventory/reserve",
	"/api/v1/inventory/release",
}

// StatsFilter 是吞吐统计的查询条件。
type StatsFilter struct {
	From        time.Time
	To          time.Time
	Granularity string
	Timezone    string
	Routes      []string
}

// StatsBucket 是仓储层返回的一个聚合点。
type StatsBucket struct {
	Bucket time.Time
	Total  int64
	Failed int64
}

// Stats 是库存接口吞吐与失败率统计。
//
// Total / Failed / FailureRate 与 Buckets 等长，无数据的时间桶补 0。
type Stats struct {
	Granularity string    `json:"granularity"`
	Timezone    string    `json:"timezone"`
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	Buckets     []string  `json:"buckets"`
	Total       []int64   `json:"total"`
	Failed      []int64   `json:"failed"`
	FailureRate []float64 `json:"failure_rate"`
}

// Stats 返回库存接口的调用量与失败率趋势。
func (s *service) Stats(ctx context.Context, filter StatsFilter) (Stats, error) {
	normalized, err := normalizeStatsFilter(filter)
	if err != nil {
		return Stats{}, err
	}

	rows, err := s.repo.Aggregate(ctx, normalized)
	if err != nil {
		return Stats{}, err
	}

	return buildStats(normalized, rows)
}

func normalizeStatsFilter(filter StatsFilter) (StatsFilter, error) {
	if filter.Granularity == "" {
		filter.Granularity = StatsGranularityHour
	}
	switch filter.Granularity {
	case StatsGranularityHour, StatsGranularityDay:
	default:
		return StatsFilter{}, apperr.NewValidationError("granularity must be hour or day")
	}

	if filter.Timezone == "" {
		filter.Timezone = DefaultStatsTimezone
	}
	if _, err := time.LoadLocation(filter.Timezone); err != nil {
		return StatsFilter{}, apperr.NewValidationError("timezone is invalid")
	}

	if filter.From.IsZero() || filter.To.IsZero() {
		return StatsFilter{}, apperr.NewValidationError("from and to are required")
	}
	if !filter.To.After(filter.From) {
		return StatsFilter{}, apperr.NewValidationError("to must be after from")
	}
	if filter.To.Sub(filter.From) > maxStatsRange {
		return StatsFilter{}, apperr.NewValidationError("time range must not exceed 90 days")
	}

	routes := make([]string, 0, len(filter.Routes))
	seen := make(map[string]struct{}, len(filter.Routes))
	for _, route := range filter.Routes {
		trimmed := strings.TrimSpace(route)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		routes = append(routes, trimmed)
	}
	if len(routes) == 0 {
		routes = append(routes, DefaultStatsRoutes...)
	}
	if len(routes) > maxStatsRouteCount {
		return StatsFilter{}, apperr.NewValidationError("too many routes")
	}
	filter.Routes = routes

	return filter, nil
}

// buildStats 把稀疏的聚合结果补齐成等长的连续时间序列。
func buildStats(filter StatsFilter, rows []StatsBucket) (Stats, error) {
	loc, err := time.LoadLocation(filter.Timezone)
	if err != nil {
		return Stats{}, apperr.NewValidationError("timezone is invalid")
	}

	buckets, err := statsbucket.Build(filter.From, filter.To, filter.Granularity, loc)
	if err != nil {
		return Stats{}, err
	}

	bucketIndex := make(map[int64]int, len(buckets))
	for index, bucket := range buckets {
		bucketIndex[bucket.Unix()] = index
	}

	total := make([]int64, len(buckets))
	failed := make([]int64, len(buckets))
	for _, row := range rows {
		index, ok := bucketIndex[row.Bucket.Unix()]
		if !ok {
			continue
		}
		total[index] += row.Total
		failed[index] += row.Failed
	}

	failureRate := make([]float64, len(buckets))
	for index := range buckets {
		if total[index] == 0 {
			continue
		}
		failureRate[index] = float64(failed[index]) / float64(total[index])
	}

	labels := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		labels = append(labels, bucket.Format(time.RFC3339))
	}

	return Stats{
		Granularity: filter.Granularity,
		Timezone:    filter.Timezone,
		From:        filter.From,
		To:          filter.To,
		Buckets:     labels,
		Total:       total,
		Failed:      failed,
		FailureRate: failureRate,
	}, nil
}
