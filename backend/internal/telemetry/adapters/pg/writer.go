package pg

import (
	"context"
	"errors"
	"fmt"
	"sync"

	telemetry "fleetmonitoring/backend/internal/telemetry/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sony/gobreaker"
)

const maxBatchChunk = 1000

var (
	ErrEmptyBatch = errors.New("empty batch")
	ErrValidation = errors.New("validation")
)

type Writer struct {
	pool    *pgxpool.Pool
	breaker *gobreaker.CircuitBreaker
	mu      sync.Mutex
	store   map[string]telemetry.TelemetryEvent // in-memory only for tests
}

func NewWriter(pool *pgxpool.Pool) *Writer {
	return NewWriterWithBreaker(pool, nil)
}

func NewWriterWithBreaker(pool *pgxpool.Pool, cb *gobreaker.CircuitBreaker) *Writer {
	w := &Writer{
		pool:    pool,
		breaker: cb,
		store:   make(map[string]telemetry.TelemetryEvent),
	}
	return w
}

func (w *Writer) WriteBatch(ctx context.Context, evts []telemetry.TelemetryEvent) (int64, error) {
	if len(evts) == 0 {
		return 0, fmt.Errorf("empty batch: %w", errors.Join(ErrEmptyBatch, ErrValidation))
	}
	if len(evts) > maxBatchChunk {
		return w.writeChunked(ctx, evts)
	}
	if w.pool == nil {
		return w.writeInMemory(ctx, evts)
	}
	return w.writeWithPool(ctx, evts)
}

func (w *Writer) writeChunked(ctx context.Context, evts []telemetry.TelemetryEvent) (int64, error) {
	var total int64
	for i := 0; i < len(evts); i += maxBatchChunk {
		end := i + maxBatchChunk
		if end > len(evts) {
			end = len(evts)
		}
		var n int64
		var err error
		if w.pool == nil {
			n, err = w.writeInMemory(ctx, evts[i:end])
		} else {
			n, err = w.writeWithPool(ctx, evts[i:end])
		}
		if err != nil {
			return total, fmt.Errorf("chunk %d failed: %w", i/maxBatchChunk, err)
		}
		total += n
	}
	return total, nil
}

func (w *Writer) writeInMemory(_ context.Context, evts []telemetry.TelemetryEvent) (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var n int64
	batchSeen := make(map[string]bool, len(evts))
	for _, e := range evts {
		if batchSeen[e.ClientEventID] {
			continue
		}
		batchSeen[e.ClientEventID] = true
		if _, exists := w.store[e.ClientEventID]; exists {
			continue
		}
		w.store[e.ClientEventID] = e
		n++
	}
	return n, nil
}

func (w *Writer) writeWithPool(ctx context.Context, evts []telemetry.TelemetryEvent) (int64, error) {
	exec := func() (int64, error) {
		tx, err := w.pool.Begin(ctx)
		if err != nil {
			return 0, fmt.Errorf("begin tx failed: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if err := ensureStaging(ctx, tx); err != nil {
			return 0, err
		}
		if _, err := copyToStaging(ctx, tx, evts); err != nil {
			return 0, err
		}
		n, err := insertFromStaging(ctx, tx)
		if err != nil {
			return 0, err
		}
		if err := tx.Commit(ctx); err != nil {
			return 0, fmt.Errorf("commit failed: %w", err)
		}
		return n, nil
	}
	if w.breaker == nil {
		return exec()
	}
	res, err := w.breaker.Execute(func() (any, error) { return exec() })
	if err != nil {
		return 0, fmt.Errorf("circuit breaker or db failed: %w", err)
	}
	return res.(int64), nil
}

func ensureStaging(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `CREATE TEMP TABLE telemetry_staging (
		client_event_id UUID,
		plate TEXT,
		received_at TIMESTAMPTZ,
		occurred_at TIMESTAMPTZ,
		lat DOUBLE PRECISION,
		lon DOUBLE PRECISION,
		speed INT
	) ON COMMIT DROP`)
	if err != nil {
		return fmt.Errorf("create staging failed: %w", err)
	}
	return nil
}

func copyToStaging(ctx context.Context, tx pgx.Tx, evts []telemetry.TelemetryEvent) (int64, error) {
	cols := []string{"client_event_id", "plate", "received_at", "occurred_at", "lat", "lon", "speed"}
	rows := make([][]any, 0, len(evts))
	for _, e := range evts {
		rows = append(rows, []any{e.ClientEventID, e.Plate, e.ReceivedAt, e.OccurredAt, e.Lat, e.Lon, e.Speed})
	}
	copied, err := tx.CopyFrom(ctx, pgx.Identifier{"telemetry_staging"}, cols, pgx.CopyFromRows(rows))
	if err != nil {
		return 0, fmt.Errorf("copyfrom staging failed: %w", err)
	}
	return copied, nil
}

func insertFromStaging(ctx context.Context, tx pgx.Tx) (int64, error) {
	tag, err := tx.Exec(ctx, `
		WITH new_ids AS (
			INSERT INTO telemetry_dedup(client_event_id)
			SELECT DISTINCT client_event_id FROM telemetry_staging
			ON CONFLICT DO NOTHING
			RETURNING client_event_id
		)
		INSERT INTO telemetry (client_event_id, plate, received_at, occurred_at, lat, lon, speed)
		SELECT s.client_event_id, s.plate, s.received_at, s.occurred_at, s.lat, s.lon, s.speed
		FROM telemetry_staging s
		JOIN new_ids USING (client_event_id)`)
	if err != nil {
		return 0, fmt.Errorf("insert via dedup failed: %w", err)
	}
	return tag.RowsAffected(), nil
}
