package inventory

import (
	"testing"
	"time"
)

func mustLoadLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load location %q: %v", name, err)
	}
	return loc
}

func TestBuildMovementStatsFillsMissingBucketsWithZero(t *testing.T) {
	loc := mustLoadLocation(t, DefaultStatsTimezone)
	from := time.Date(2026, 11, 12, 0, 0, 0, 0, loc)
	to := time.Date(2026, 11, 12, 3, 0, 0, 0, loc)

	// 只有 reserve 在第二个小时桶有数据，其余桶与操作类型都必须补 0。
	rows := []MovementStatsBucket{
		{
			Bucket:        time.Date(2026, 11, 12, 1, 0, 0, 0, loc),
			OperationType: "reserve",
			Calls:         7,
		},
	}

	stats, err := buildMovementStats(MovementStatsFilter{
		Granularity: StatsGranularityHour,
		Timezone:    DefaultStatsTimezone,
		From:        from,
		To:          to,
	}, rows)
	if err != nil {
		t.Fatalf("buildMovementStats: %v", err)
	}

	if len(stats.Buckets) != 3 {
		t.Fatalf("buckets = %d, want 3", len(stats.Buckets))
	}
	if len(stats.Series) != len(StatsOperationTypes) {
		t.Fatalf("series = %d, want %d", len(stats.Series), len(StatsOperationTypes))
	}
	for index, operationType := range StatsOperationTypes {
		if stats.Series[index].OperationType != operationType {
			t.Fatalf("series[%d] = %q, want %q", index, stats.Series[index].OperationType, operationType)
		}
		if len(stats.Series[index].Calls) != len(stats.Buckets) {
			t.Fatalf("series[%d] calls length = %d, want %d",
				index, len(stats.Series[index].Calls), len(stats.Buckets))
		}
	}

	reserve := callsFor(t, stats, "reserve")
	if reserve[0] != 0 || reserve[1] != 7 || reserve[2] != 0 {
		t.Fatalf("reserve calls = %v, want [0 7 0]", reserve)
	}
	if increase := callsFor(t, stats, "increase"); increase[0] != 0 || increase[1] != 0 || increase[2] != 0 {
		t.Fatalf("increase calls = %v, want all zero", increase)
	}
}

func TestBuildMovementStatsIgnoresOutOfRangeRows(t *testing.T) {
	loc := mustLoadLocation(t, DefaultStatsTimezone)
	from := time.Date(2026, 11, 12, 0, 0, 0, 0, loc)
	to := time.Date(2026, 11, 12, 2, 0, 0, 0, loc)

	rows := []MovementStatsBucket{
		{
			Bucket:        time.Date(2026, 11, 12, 5, 0, 0, 0, loc), // 超出 [from, to)
			OperationType: "increase",
			Calls:         99,
		},
		{
			Bucket:        time.Date(2026, 11, 12, 1, 0, 0, 0, loc),
			OperationType: "unknown_type",
			Calls:         5,
		},
	}

	stats, err := buildMovementStats(MovementStatsFilter{
		Granularity: StatsGranularityHour,
		Timezone:    DefaultStatsTimezone,
		From:        from,
		To:          to,
	}, rows)
	if err != nil {
		t.Fatalf("buildMovementStats: %v", err)
	}

	for _, series := range stats.Series {
		for index, calls := range series.Calls {
			if calls != 0 {
				t.Fatalf("%s calls[%d] = %d, want 0", series.OperationType, index, calls)
			}
		}
	}
}

func TestNormalizeMovementStatsFilterRejectsInvalidInput(t *testing.T) {
	valid := MovementStatsFilter{
		Granularity: StatsGranularityHour,
		Timezone:    DefaultStatsTimezone,
		From:        time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC),
		To:          time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC),
	}

	tooWide := valid
	tooWide.To = tooWide.From.Add(maxStatsRange + time.Hour)

	reversed := valid
	reversed.From, reversed.To = reversed.To, reversed.From

	badGranularity := valid
	badGranularity.Granularity = "minute"

	badTimezone := valid
	badTimezone.Timezone = "Mars/Olympus"

	missingRange := valid
	missingRange.From = time.Time{}
	missingRange.To = time.Time{}

	cases := map[string]MovementStatsFilter{
		"too wide":        tooWide,
		"reversed":        reversed,
		"bad granularity": badGranularity,
		"bad timezone":    badTimezone,
		"missing range":   missingRange,
	}

	for name, filter := range cases {
		if _, err := normalizeMovementStatsFilter(filter); err == nil {
			t.Fatalf("%s: expected validation error, got nil", name)
		}
	}
}

func TestNormalizeMovementStatsFilterDefaults(t *testing.T) {
	from := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	normalized, err := normalizeMovementStatsFilter(MovementStatsFilter{
		From: from,
		To:   to,
	})
	if err != nil {
		t.Fatalf("normalizeMovementStatsFilter: %v", err)
	}
	if normalized.Granularity != StatsGranularityHour {
		t.Fatalf("granularity = %q, want %q", normalized.Granularity, StatsGranularityHour)
	}
	if normalized.Timezone != DefaultStatsTimezone {
		t.Fatalf("timezone = %q, want %q", normalized.Timezone, DefaultStatsTimezone)
	}
}

func callsFor(t *testing.T, stats MovementStats, operationType string) []int64 {
	t.Helper()
	for _, series := range stats.Series {
		if series.OperationType == operationType {
			return series.Calls
		}
	}
	t.Fatalf("series %q not found", operationType)
	return nil
}
