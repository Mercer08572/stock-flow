package audit

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultQueueSize      = 8192
	defaultBatchSize      = 500
	defaultFlushInterval  = 500 * time.Millisecond
	defaultEnqueueTimeout = 100 * time.Millisecond
	defaultFlushTimeout   = 10 * time.Second
)

type WriterOptions struct {
	QueueSize      int
	BatchSize      int
	FlushInterval  time.Duration
	EnqueueTimeout time.Duration
	FlushTimeout   time.Duration
	Logger         *log.Logger
}

// Writer decouples request latency from audit persistence.
//
// Requests only do a non-blocking (or briefly blocking) channel send. One
// goroutine batches entries and calls BatchInserter.InsertBatch, which the
// PostgreSQL implementation executes as a single pgx.CopyFrom. This turns
// N commits/fsyncs into roughly one per batch.
type Writer struct {
	inserter BatchInserter
	queue    chan Entry
	stop     chan struct{}
	stopped  chan struct{}
	stopOnce sync.Once

	batchSize      int
	flushInterval  time.Duration
	enqueueTimeout time.Duration
	flushTimeout   time.Duration
	logger         *log.Logger

	dropped atomic.Int64
	failed  atomic.Int64
}

func NewWriter(inserter BatchInserter, options WriterOptions) *Writer {
	if options.QueueSize <= 0 {
		options.QueueSize = defaultQueueSize
	}
	if options.BatchSize <= 0 {
		options.BatchSize = defaultBatchSize
	}
	if options.FlushInterval <= 0 {
		options.FlushInterval = defaultFlushInterval
	}
	if options.EnqueueTimeout <= 0 {
		options.EnqueueTimeout = defaultEnqueueTimeout
	}
	if options.FlushTimeout <= 0 {
		options.FlushTimeout = defaultFlushTimeout
	}
	if options.Logger == nil {
		options.Logger = log.Default()
	}

	writer := &Writer{
		inserter:       inserter,
		queue:          make(chan Entry, options.QueueSize),
		stop:           make(chan struct{}),
		stopped:        make(chan struct{}),
		batchSize:      options.BatchSize,
		flushInterval:  options.FlushInterval,
		enqueueTimeout: options.EnqueueTimeout,
		flushTimeout:   options.FlushTimeout,
		logger:         options.Logger,
	}
	go writer.run()
	return writer
}

// Enqueue never blocks indefinitely. If the queue stays full after a short
// wait, the entry is dropped and counted; the request is not failed because of
// an audit backlog.
func (w *Writer) Enqueue(entry Entry) bool {
	if w == nil {
		return false
	}

	select {
	case <-w.stopped:
		return false
	default:
	}

	select {
	case w.queue <- entry:
		return true
	default:
	}

	timer := time.NewTimer(w.enqueueTimeout)
	defer timer.Stop()

	select {
	case w.queue <- entry:
		return true
	case <-w.stopped:
		return false
	case <-timer.C:
		dropped := w.dropped.Add(1)
		if dropped == 1 || dropped%100 == 0 {
			w.logger.Printf("audit queue full, dropped %d entries so far", dropped)
		}
		return false
	}
}

func (w *Writer) Close(ctx context.Context) error {
	if w == nil {
		return nil
	}
	w.stopOnce.Do(func() { close(w.stop) })
	select {
	case <-w.stopped:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *Writer) Dropped() int64 { return w.dropped.Load() }
func (w *Writer) Failed() int64  { return w.failed.Load() }

func (w *Writer) run() {
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	batch := make([]Entry, 0, w.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		w.flush(batch)
		batch = batch[:0]
	}

	for {
		select {
		case entry := <-w.queue:
			batch = append(batch, entry)
			if len(batch) >= w.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-w.stop:
			// Drain whatever is already queued, then flush before signalling done.
			for {
				select {
				case entry := <-w.queue:
					batch = append(batch, entry)
					if len(batch) >= w.batchSize {
						flush()
					}
				default:
					flush()
					close(w.stopped)
					return
				}
			}
		}
	}
}

func (w *Writer) flush(batch []Entry) {
	ctx, cancel := context.WithTimeout(context.Background(), w.flushTimeout)
	defer cancel()

	if err := w.inserter.InsertBatch(ctx, batch); err != nil {
		w.failed.Add(int64(len(batch)))
		w.logger.Printf("audit batch insert failed (%d entries): %v", len(batch), err)
	}
}
