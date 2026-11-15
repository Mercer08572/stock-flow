package statsbucket_test

import (
	"testing"
	"time"

	"github.com/Mercer08572/stock-flow/internal/shared/statsbucket"
)

const shanghai = "Asia/Shanghai"

func mustLoadLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load location %q: %v", name, err)
	}
	return loc
}

func TestBuildHourlyStartsAtLocalHourBoundary(t *testing.T) {
	loc := mustLoadLocation(t, shanghai)
	from := time.Date(2026, 11, 12, 22, 30, 0, 0, loc)
	to := time.Date(2026, 11, 13, 2, 0, 0, 0, loc)

	buckets, err := statsbucket.Build(from, to, statsbucket.Hour, loc)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	want := []time.Time{
		time.Date(2026, 11, 12, 22, 0, 0, 0, loc),
		time.Date(2026, 11, 12, 23, 0, 0, 0, loc),
		time.Date(2026, 11, 13, 0, 0, 0, 0, loc),
		time.Date(2026, 11, 13, 1, 0, 0, 0, loc),
	}
	if len(buckets) != len(want) {
		t.Fatalf("bucket count = %d, want %d (%v)", len(buckets), len(want), buckets)
	}
	for index := range want {
		if !buckets[index].Equal(want[index]) {
			t.Fatalf("bucket[%d] = %v, want %v", index, buckets[index], want[index])
		}
	}
}

func TestBuildDailyUsesLocalCalendarDays(t *testing.T) {
	loc := mustLoadLocation(t, shanghai)
	from := time.Date(2026, 11, 12, 10, 0, 0, 0, loc)
	to := time.Date(2026, 11, 15, 0, 0, 0, 0, loc)

	buckets, err := statsbucket.Build(from, to, statsbucket.Day, loc)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(buckets) != 3 {
		t.Fatalf("bucket count = %d, want 3 (%v)", len(buckets), buckets)
	}
	// 起点必须是本地零点，而不是 from 的时刻。
	wantFirst := time.Date(2026, 11, 12, 0, 0, 0, 0, loc)
	if !buckets[0].Equal(wantFirst) {
		t.Fatalf("first bucket = %v, want %v", buckets[0], wantFirst)
	}
}

func TestBuildAlignsToTargetTimezone(t *testing.T) {
	utc := mustLoadLocation(t, "UTC")
	// UTC 的 [12 日 20:00, 13 日 20:00) 在上海是 [13 日 04:00, 14 日 04:00)，
	// 按上海自然日切桶应得到 13 日与 14 日两个桶。
	from := time.Date(2026, 11, 12, 20, 0, 0, 0, utc)
	to := time.Date(2026, 11, 13, 20, 0, 0, 0, utc)

	loc := mustLoadLocation(t, shanghai)
	buckets, err := statsbucket.Build(from, to, statsbucket.Day, loc)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(buckets) != 2 {
		t.Fatalf("bucket count = %d, want 2 (%v)", len(buckets), buckets)
	}
	if day := buckets[0].Day(); day != 13 {
		t.Fatalf("first bucket day = %d, want 13 (bucket=%v)", day, buckets[0])
	}
}

func TestBuildRejectsInvalidGranularity(t *testing.T) {
	loc := mustLoadLocation(t, shanghai)
	from := time.Date(2026, 11, 12, 0, 0, 0, 0, loc)

	if _, err := statsbucket.Build(from, from.Add(time.Hour), "minute", loc); err == nil {
		t.Fatal("expected validation error for granularity")
	}
}

func TestBuildRejectsTooManyPoints(t *testing.T) {
	loc := mustLoadLocation(t, shanghai)
	from := time.Date(2026, 11, 12, 0, 0, 0, 0, loc)
	to := from.Add(time.Duration(statsbucket.MaxPoints+1) * time.Hour)

	if _, err := statsbucket.Build(from, to, statsbucket.Hour, loc); err == nil {
		t.Fatal("expected validation error for too many points")
	}
}
