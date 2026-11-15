// Package statsbucket 生成按粒度对齐到目标时区的连续时间桶。
//
// 库存趋势图与接口吞吐图必须使用同一套桶边界，否则同一时间范围下两张图的
// 时间轴会对不上。因此这里集中实现，而不是各模块各写一份。
package statsbucket

import (
	"time"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

const (
	// Hour 与 Day 是支持的时间桶粒度。
	Hour = "hour"
	Day  = "day"

	// MaxPoints 限制单次生成的桶数量，防止 hour 粒度下跨度太大拖垮前端。
	MaxPoints = 3000
)

// Build 返回 [from, to) 内所有时间桶的起点。
//
// 桶边界按目标时区的自然小时/自然日切分，而不是对 UTC 时间做固定步长截断：
// 后者在非整小时偏移的时区（例如 +05:30）会把桶切错位，也会让跨日统计落到错误的日期。
func Build(from, to time.Time, granularity string, loc *time.Location) ([]time.Time, error) {
	if loc == nil {
		return nil, apperr.NewValidationError("timezone is required")
	}
	switch granularity {
	case Hour, Day:
	default:
		return nil, apperr.NewValidationError("granularity must be hour or day")
	}

	localFrom := from.In(loc)
	localTo := to.In(loc)

	var current time.Time
	if granularity == Day {
		current = time.Date(localFrom.Year(), localFrom.Month(), localFrom.Day(), 0, 0, 0, 0, loc)
	} else {
		current = time.Date(localFrom.Year(), localFrom.Month(), localFrom.Day(), localFrom.Hour(), 0, 0, 0, loc)
	}

	buckets := make([]time.Time, 0, 32)
	for current.Before(localTo) {
		if len(buckets) >= MaxPoints {
			return nil, apperr.NewValidationError("time range produces too many data points")
		}
		buckets = append(buckets, current)
		if granularity == Day {
			// 用 AddDate 而不是 Add(24h)：跨夏令时切换时"下一天"仍是本地零点。
			current = current.AddDate(0, 0, 1)
		} else {
			current = current.Add(time.Hour)
		}
	}

	return buckets, nil
}
