package audit

import (
	"testing"
	"time"
)

func TestBuildStatsFillsMissingBuckets(t *testing.T) {
	loc, err := time.LoadLocation(DefaultStatsTimezone)
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	from := time.Date(2026, 11, 12, 0, 0, 0, 0, loc)
	to := time.Date(2026, 11, 12, 3, 0, 0, 0, loc)

	rows := []StatsBucket{
		{
			Bucket: time.Date(2026, 11, 12, 1, 0, 0, 0, loc),
			Total:  10,
			Failed: 2,
		},
	}

	stats, err := buildStats(StatsFilter{
		Granularity: StatsGranularityHour,
		Timezone:    DefaultStatsTimezone,
		From:        from,
		To:          to,
		Routes:      DefaultStatsRoutes,
	}, rows)
	if err != nil {
		t.Fatalf("buildStats: %v", err)
	}

	if len(stats.Buckets) != 3 {
		t.Fatalf("buckets = %d, want 3", len(stats.Buckets))
	}
	if len(stats.Total) != 3 || len(stats.Failed) != 3 || len(stats.FailureRate) != 3 {
		t.Fatalf("series lengths = %d/%d/%d, want 3 each",
			len(stats.Total), len(stats.Failed), len(stats.FailureRate))
	}

	if stats.Total[0] != 0 || stats.Total[1] != 10 || stats.Total[2] != 0 {
		t.Fatalf("total = %v, want [0 10 0]", stats.Total)
	}
	if stats.Failed[1] != 2 {
		t.Fatalf("failed[1] = %d, want 2", stats.Failed[1])
	}
	if stats.FailureRate[1] != 0.2 {
		t.Fatalf("failureRate[1] = %v, want 0.2", stats.FailureRate[1])
	}
	// 空桶的失败率必须是 0，而不是 NaN。
	if stats.FailureRate[0] != 0 {
		t.Fatalf("failureRate[0] = %v, want 0", stats.FailureRate[0])
	}
}

func TestNormalizeStatsFilterDefaultsRoutes(t *testing.T) {
	from := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	normalized, err := normalizeStatsFilter(StatsFilter{From: from, To: to})
	if err != nil {
		t.Fatalf("normalizeStatsFilter: %v", err)
	}
	if normalized.Granularity != StatsGranularityHour {
		t.Fatalf("granularity = %q, want %q", normalized.Granularity, StatsGranularityHour)
	}
	if normalized.Timezone != DefaultStatsTimezone {
		t.Fatalf("timezone = %q, want %q", normalized.Timezone, DefaultStatsTimezone)
	}
	if len(normalized.Routes) != len(DefaultStatsRoutes) {
		t.Fatalf("routes = %v, want defaults", normalized.Routes)
	}
}

func TestNormalizeStatsFilterDeduplicatesRoutes(t *testing.T) {
	from := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)

	normalized, err := normalizeStatsFilter(StatsFilter{
		From:   from,
		To:     to,
		Routes: []string{" /a ", "/a", "", "/b"},
	})
	if err != nil {
		t.Fatalf("normalizeStatsFilter: %v", err)
	}
	if len(normalized.Routes) != 2 || normalized.Routes[0] != "/a" || normalized.Routes[1] != "/b" {
		t.Fatalf("routes = %v, want [/a /b]", normalized.Routes)
	}
}

func TestNormalizeStatsFilterRejectsInvalidInput(t *testing.T) {
	valid := StatsFilter{
		From: time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC),
	}

	tooWide := valid
	tooWide.To = tooWide.From.Add(maxStatsRange + time.Hour)

	missingRange := StatsFilter{}

	badGranularity := valid
	badGranularity.Granularity = "minute"

	cases := map[string]StatsFilter{
		"too wide":        tooWide,
		"missing range":   missingRange,
		"bad granularity": badGranularity,
	}

	for name, filter := range cases {
		if _, err := normalizeStatsFilter(filter); err == nil {
			t.Fatalf("%s: expected validation error, got nil", name)
		}
	}
}
