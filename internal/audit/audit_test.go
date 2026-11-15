package audit

import (
	"context"
	"encoding/base64"
	"io"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	occurredAt := time.Date(2026, 11, 10, 12, 30, 0, 0, time.UTC)
	encoded := EncodeCursor(Log{OccurredAt: occurredAt, ID: 42})
	if encoded == "" {
		t.Fatal("expected a cursor")
	}

	cursor, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if cursor == nil || !cursor.OccurredAt.Equal(occurredAt) || cursor.ID != 42 {
		t.Fatalf("unexpected cursor: %+v", cursor)
	}
}

func TestDecodeCursorRejectsInvalidValues(t *testing.T) {
	values := []string{
		"not-base64!",
		base64.RawURLEncoding.EncodeToString([]byte(`{"occurred_at":"2026-11-10T12:00:00Z","id":0}`)),
		base64.RawURLEncoding.EncodeToString([]byte(`{}`)),
		base64.RawURLEncoding.EncodeToString([]byte(`not-json`)),
	}
	for _, value := range values {
		if _, err := DecodeCursor(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
	if cursor, err := DecodeCursor("  "); err != nil || cursor != nil {
		t.Fatalf("empty cursor should be accepted as nil, got %+v err=%v", cursor, err)
	}
}

func TestTruncateKeepsUTF8Valid(t *testing.T) {
	if got := truncate("中文字符", 2); got != "中文" {
		t.Fatalf("unexpected truncation: %q", got)
	}
	if got := truncate("short", 10); got != "short" {
		t.Fatalf("unexpected unchanged value: %q", got)
	}
	if got := truncate("", 10); got != "" {
		t.Fatalf("expected empty value, got %q", got)
	}
}

func TestPartitionHelpers(t *testing.T) {
	at := time.Date(2026, 11, 10, 23, 59, 59, 0, time.UTC)
	if got := monthStart(at).Format("2006-01-02"); got != "2026-11-01" {
		t.Fatalf("unexpected month start: %s", got)
	}
	if got := partitionName(at); got != "audit_logs_202611" {
		t.Fatalf("unexpected partition name: %s", got)
	}

	parsed, ok := parsePartitionName("audit_logs_202611")
	if !ok || !parsed.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected parsed partition: %v ok=%v", parsed, ok)
	}
	for _, value := range []string{"audit_logs", "audit_logs_2026", "audit_logs_202613", "other_202611"} {
		if _, ok := parsePartitionName(value); ok {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

type captureSink struct {
	mu      sync.Mutex
	entries []Entry
}

func (s *captureSink) Enqueue(entry Entry) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	return true
}

func (s *captureSink) last(t *testing.T) Entry {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.entries) == 0 {
		t.Fatal("expected an audit entry")
	}
	return s.entries[len(s.entries)-1]
}

type fakeInserter struct {
	mu      sync.Mutex
	entries []Entry
	notify  chan struct{}
}

func newFakeInserter() *fakeInserter {
	return &fakeInserter{notify: make(chan struct{}, 1)}
}

func (f *fakeInserter) InsertBatch(_ context.Context, entries []Entry) error {
	f.mu.Lock()
	f.entries = append(f.entries, entries...)
	f.mu.Unlock()
	select {
	case f.notify <- struct{}{}:
	default:
	}
	return nil
}

func (f *fakeInserter) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.entries)
}

type fakeRepository struct {
	inserted     []Entry
	items        []Log
	filters      []QueryFilter
	statsBuckets []StatsBucket
	statsFilters []StatsFilter
}

func (r *fakeRepository) InsertBatch(_ context.Context, entries []Entry) error {
	r.inserted = append(r.inserted, entries...)
	return nil
}

func (r *fakeRepository) List(_ context.Context, filter QueryFilter) ([]Log, error) {
	r.filters = append(r.filters, filter)
	return r.items, nil
}

func (r *fakeRepository) Aggregate(_ context.Context, filter StatsFilter) ([]StatsBucket, error) {
	r.statsFilters = append(r.statsFilters, filter)
	return r.statsBuckets, nil
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

func waitForInserter(t *testing.T, inserter *fakeInserter, want int) {
	t.Helper()
	waitFor(t, func() bool { return inserter.count() >= want })
}

func TestWriterBatchesEntries(t *testing.T) {
	inserter := newFakeInserter()
	writer := NewWriter(inserter, WriterOptions{
		QueueSize:     64,
		BatchSize:     10,
		FlushInterval: 5 * time.Millisecond,
		Logger:        testLogger(),
	})
	t.Cleanup(func() { _ = writer.Close(context.Background()) })

	for index := 0; index < 25; index++ {
		if !writer.Enqueue(Entry{TraceID: "req_" + strings.Repeat("x", index)}) {
			t.Fatal("expected enqueue to succeed")
		}
	}

	waitForInserter(t, inserter, 25)
	if got := inserter.count(); got != 25 {
		t.Fatalf("expected 25 entries, got %d", got)
	}
}

func TestWriterCloseFlushesPendingEntries(t *testing.T) {
	inserter := newFakeInserter()
	writer := NewWriter(inserter, WriterOptions{
		QueueSize:     16,
		BatchSize:     1000,
		FlushInterval: time.Hour,
		Logger:        testLogger(),
	})

	for index := 0; index < 3; index++ {
		writer.Enqueue(Entry{TraceID: "req_close"})
	}
	if err := writer.Close(context.Background()); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if got := inserter.count(); got != 3 {
		t.Fatalf("expected close to flush 3 entries, got %d", got)
	}
	if writer.Enqueue(Entry{TraceID: "req_after_close"}) {
		t.Fatal("enqueue after close must fail")
	}
}

func TestWriterReportsFailedBatches(t *testing.T) {
	writer := NewWriter(failingInserter{}, WriterOptions{
		QueueSize:     8,
		BatchSize:     2,
		FlushInterval: time.Hour,
		Logger:        testLogger(),
	})
	t.Cleanup(func() { _ = writer.Close(context.Background()) })

	writer.Enqueue(Entry{TraceID: "req_fail_1"})
	writer.Enqueue(Entry{TraceID: "req_fail_2"})
	waitFor(t, func() bool { return writer.Failed() >= 2 })
}

type failingInserter struct{}

func (failingInserter) InsertBatch(context.Context, []Entry) error {
	return context.DeadlineExceeded
}

func testLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}
