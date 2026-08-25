package pg

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	telemetry "fleetmonitoring/backend/internal/telemetry/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sony/gobreaker"
)

// ---------------------------------------------------------------------------
// fakes for DBPool / Tx — consumer-side interfaces, no external DB
// ---------------------------------------------------------------------------

type fakeTx struct {
	execCalls      []string
	execFunc       func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	execErr        error
	execTag        pgconn.CommandTag
	copyFromFunc   func(ctx context.Context, tableName pgx.Identifier, cols []string, src pgx.CopyFromSource) (int64, error)
	copyFromErr    error
	copyFromRows   int64
	copyFromCalls  int
	capturedTable  pgx.Identifier
	capturedCols   []string
	commitErr      error
	rollbackErr    error
	commitCalls    int
	rollbackCalls  int
	beginTx        pgx.Tx
	beginErr       error
}

func (f *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.execCalls = append(f.execCalls, sql)
	if f.execFunc != nil {
		return f.execFunc(ctx, sql, args...)
	}
	if f.execErr != nil {
		return pgconn.NewCommandTag(""), f.execErr
	}
	if f.execTag.String() != "" {
		return f.execTag, nil
	}
	// default: CREATE TEMP returns 0, INSERT returns f.execTag if set else INSERT 0 N
	return pgconn.NewCommandTag(""), nil
}

func (f *fakeTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	f.copyFromCalls++
	f.capturedTable = tableName
	f.capturedCols = columnNames
	if f.copyFromFunc != nil {
		return f.copyFromFunc(ctx, tableName, columnNames, rowSrc)
	}
	if f.copyFromErr != nil {
		return 0, f.copyFromErr
	}
	return f.copyFromRows, nil
}

func (f *fakeTx) Commit(ctx context.Context) error {
	f.commitCalls++
	if f.commitErr != nil {
		return f.commitErr
	}
	return nil
}

func (f *fakeTx) Rollback(ctx context.Context) error {
	f.rollbackCalls++
	if f.rollbackErr != nil {
		return f.rollbackErr
	}
	return nil
}

func (f *fakeTx) Begin(ctx context.Context) (pgx.Tx, error) {
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	if f.beginTx != nil {
		return f.beginTx, nil
	}
	return f, nil
}

func (f *fakeTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (f *fakeTx) LargeObjects() pgx.LargeObjects                               { return pgx.LargeObjects{} }
func (f *fakeTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (f *fakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (f *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row { return nil }
func (f *fakeTx) Conn() *pgx.Conn                                               { return nil }

type fakePool struct {
	beginCalls int
	tx         pgx.Tx
	err        error
	beginFunc  func(ctx context.Context) (pgx.Tx, error)
}

func (f *fakePool) Begin(ctx context.Context) (pgx.Tx, error) {
	f.beginCalls++
	if f.beginFunc != nil {
		return f.beginFunc(ctx)
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.tx, nil
}

// ---------------------------------------------------------------------------
// helpers for pool-backed writer tests
// ---------------------------------------------------------------------------

func newFakeTxSuccess(inserted int64) *fakeTx {
	// Arrange a Tx that succeeds all stages: CREATE TEMP ok, CopyFrom copies N, INSERT returns tag with N
	return &fakeTx{
		copyFromRows: inserted,
		execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "CREATE TEMP TABLE") {
				return pgconn.NewCommandTag("CREATE TABLE"), nil
			}
			if strings.Contains(sql, "INSERT INTO telemetry") {
				return pgconn.NewCommandTag(strings.Join([]string{"INSERT 0", itoa(int(inserted))}, " ")), nil
			}
			return pgconn.NewCommandTag(""), nil
		},
	}
}

func newFakeTxWithCopied(copied int64, inserted int64) *fakeTx {
	return &fakeTx{
		copyFromRows: copied,
		execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "CREATE TEMP TABLE") {
				return pgconn.NewCommandTag("CREATE TABLE"), nil
			}
			if strings.Contains(sql, "INSERT INTO telemetry") {
				return pgconn.NewCommandTag(strings.Join([]string{"INSERT 0", itoa(int(inserted))}, " ")), nil
			}
			return pgconn.NewCommandTag(""), nil
		},
	}
}

// ---------------------------------------------------------------------------
// Suite: WriteBatch via pool — Covers [SPEC-001: AC-009, AC-003, FR-010, BR-008, BR-009]
// ---------------------------------------------------------------------------

func TestWriter_WriteWithPool_Success(t *testing.T) {
	// Covers [SPEC-001: AC-009, FR-010, BR-008, BR-009]
	t.Run("single batch 2 events via pool succeeds and returns inserted count", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC().Truncate(time.Microsecond)
		evts := []telemetry.TelemetryEvent{
			newEvent(validID(1), "GTP890", 10, floatPtr(4.711), floatPtr(-74.072), now),
			newEvent(validID(2), "GTP890", 20, floatPtr(4.711), floatPtr(-74.072), now.Add(time.Millisecond)),
		}
		tx := newFakeTxSuccess(2)
		pool := &fakePool{tx: tx}
		w := NewWriterWithPool(pool)

		// Act
		n, err := w.WriteBatch(ctx, evts)

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if n != 2 {
			t.Fatalf("expected 2, got %d", n)
		}
		if tx.commitCalls != 1 {
			t.Fatalf("expected Commit 1, got %d", tx.commitCalls)
		}
		if tx.rollbackCalls != 1 {
			t.Fatalf("expected Rollback deferred 1, got %d", tx.rollbackCalls)
		}
	})

	t.Run("CopyFrom receives correct table and columns", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(10), "TTY423", 42, floatPtr(4.7), floatPtr(-74.0), now)}
		tx := newFakeTxSuccess(1)
		pool := &fakePool{tx: tx}
		w := NewWriterWithPool(pool)

		// Act
		_, err := w.WriteBatch(ctx, evts)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if len(tx.capturedTable) != 1 || tx.capturedTable[0] != "telemetry_staging" {
			t.Fatalf("expected telemetry_staging table got %v", tx.capturedTable)
		}
		expectedCols := []string{"client_event_id", "plate", "received_at", "occurred_at", "lat", "lon", "speed"}
		if len(tx.capturedCols) != len(expectedCols) {
			t.Fatalf("expected cols %v got %v", expectedCols, tx.capturedCols)
		}
		for i, c := range expectedCols {
			if tx.capturedCols[i] != c {
				t.Fatalf("col %d expected %s got %s", i, c, tx.capturedCols[i])
			}
		}
		if tx.copyFromCalls != 1 {
			t.Fatalf("expected CopyFrom 1 got %d", tx.copyFromCalls)
		}
	})

	t.Run("ensureStaging executed CREATE TEMP TABLE", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(11), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now)}
		tx := newFakeTxSuccess(1)
		pool := &fakePool{tx: tx}
		w := NewWriterWithPool(pool)

		// Act
		_, err := w.WriteBatch(ctx, evts)

		// Assert
		if err != nil {
			t.Fatalf("got %v", err)
		}
		found := false
		for _, sql := range tx.execCalls {
			if strings.Contains(sql, "CREATE TEMP TABLE telemetry_staging") && strings.Contains(sql, "ON COMMIT DROP") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected CREATE TEMP TABLE telemetry_staging not found in execCalls %v", tx.execCalls)
		}
	})

	t.Run("insertFromStaging uses CTE with ON CONFLICT DO NOTHING", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(12), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now)}
		tx := newFakeTxSuccess(1)
		pool := &fakePool{tx: tx}
		w := NewWriterWithPool(pool)

		// Act
		_, err := w.WriteBatch(ctx, evts)

		// Assert
		if err != nil {
			t.Fatalf("got %v", err)
		}
		found := false
		for _, sql := range tx.execCalls {
			if strings.Contains(sql, "WITH new_ids") && strings.Contains(sql, "ON CONFLICT DO NOTHING") && strings.Contains(sql, "telemetry_dedup") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected CTE dedup ON CONFLICT DO NOTHING not found in execCalls %v", tx.execCalls)
		}
	})
}

func TestWriter_WriteWithPool_Errors(t *testing.T) {
	// Covers [SPEC-001: AC-009, FR-010, BR-004] error paths wrapped with fmt.Errorf %w
	t.Run("begin tx failed returns wrapped error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(20), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now)}
		pool := &fakePool{err: errors.New("pool down")}
		w := NewWriterWithPool(pool)

		// Act
		_, err := w.WriteBatch(ctx, evts)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "begin tx failed") {
			t.Fatalf("expected begin tx failed, got %v", err)
		}
	})

	t.Run("ensureStaging CREATE TEMP TABLE error propagates", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(21), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now)}
		tx := &fakeTx{
			copyFromRows: 1,
			execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "CREATE TEMP TABLE") {
					return pgconn.NewCommandTag(""), errors.New("create staging boom")
				}
				return pgconn.NewCommandTag("INSERT 0 1"), nil
			},
		}
		pool := &fakePool{tx: tx}
		w := NewWriterWithPool(pool)

		// Act
		_, err := w.WriteBatch(ctx, evts)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "create staging failed") {
			t.Fatalf("expected create staging failed got %v", err)
		}
	})

	t.Run("copyToStaging CopyFrom error propagates", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(22), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now)}
		tx := &fakeTx{
			copyFromErr: errors.New("copy boom"),
			execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("CREATE TABLE"), nil
			},
		}
		pool := &fakePool{tx: tx}
		w := NewWriterWithPool(pool)

		// Act
		_, err := w.WriteBatch(ctx, evts)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "copyfrom staging failed") {
			t.Fatalf("expected copyfrom staging failed got %v", err)
		}
	})

	t.Run("insertFromStaging Exec error propagates", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(23), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now)}
		tx := &fakeTx{
			copyFromRows: 1,
			execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "CREATE TEMP TABLE") {
					return pgconn.NewCommandTag("CREATE TABLE"), nil
				}
				if strings.Contains(sql, "INSERT INTO telemetry") {
					return pgconn.NewCommandTag(""), errors.New("insert boom")
				}
				return pgconn.NewCommandTag(""), nil
			},
		}
		pool := &fakePool{tx: tx}
		w := NewWriterWithPool(pool)

		// Act
		_, err := w.WriteBatch(ctx, evts)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "insert via dedup failed") {
			t.Fatalf("expected insert via dedup failed got %v", err)
		}
	})

	t.Run("commit failed returns wrapped error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(24), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now)}
		tx := &fakeTx{
			copyFromRows: 1,
			commitErr:    errors.New("commit boom"),
			execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "CREATE TEMP TABLE") {
					return pgconn.NewCommandTag("CREATE TABLE"), nil
				}
				return pgconn.NewCommandTag("INSERT 0 1"), nil
			},
		}
		pool := &fakePool{tx: tx}
		w := NewWriterWithPool(pool)

		// Act
		_, err := w.WriteBatch(ctx, evts)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("expected commit failed got %v", err)
		}
	})
}

func TestWriter_WriteChunked(t *testing.T) {
	// Covers [SPEC-001: AC-009, FR-010, BR-009] chunking at 1000
	t.Run("in-memory chunked 1500 events splits 1000+500", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC().Truncate(time.Microsecond)
		evts := make([]telemetry.TelemetryEvent, 1500)
		for i := 0; i < 1500; i++ {
			evts[i] = newEvent(validID(i%10000), "GTP890", i%120, floatPtr(4.711), floatPtr(-74.072), now.Add(time.Duration(i)*time.Millisecond))
		}
		w := NewWriterWithPool(nil)
		// pool nil => in-memory path, ensure chunking works via in-memory logic

		// Act
		n, err := w.WriteBatch(ctx, evts)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if n != 1500 {
			t.Fatalf("expected 1500 got %d", n)
		}
	})

	t.Run("in-memory chunked 2500 events splits 1000+1000+500", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC().Truncate(time.Microsecond)
		evts := make([]telemetry.TelemetryEvent, 2500)
		for i := 0; i < 2500; i++ {
			evts[i] = newEvent(validID(i%10000), "TTY423", 42, floatPtr(4.7), floatPtr(-74.0), now.Add(time.Duration(i)*time.Millisecond))
		}
		w := NewWriterWithPool(nil)

		// Act
		n, err := w.WriteBatch(ctx, evts)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if n != 2500 {
			t.Fatalf("expected 2500 got %d", n)
		}
	})

	t.Run("pool chunked 2500 events via CopyFrom 3 chunks all succeed", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC().Truncate(time.Microsecond)
		evts := make([]telemetry.TelemetryEvent, 2500)
		for i := 0; i < 2500; i++ {
			evts[i] = newEvent(validID(i%10000), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now.Add(time.Duration(i)*time.Millisecond))
		}
		// pool that returns a fresh success Tx per Begin
		pool := &fakePool{
			beginFunc: func(ctx context.Context) (pgx.Tx, error) {
				return newFakeTxSuccess(0), nil
			},
		}
		// we need Tx that reports inserted count equal to copied rows; since our fake returns 0 fixed,
		// override to return dynamic: we will make copyFromFunc that inspects rowSrc? Simpler: make Tx Exec return INSERT 0 1000 etc
		// For this test, set Tx to return 1000 for first two chunks and 500 for last — our generic fake returns 0, so total would be 0.
		// To correctly test total, we need Tx that knows event count: we capture copyFrom row count via copyFromRows set above,
		// but our beginFunc currently returns tx with 0. We need to compute inserted per chunk dynamically via Exec -> return INSERT 0 N where N equals copyFromRows.
		// Since fakeTx's Exec for INSERT queries needs to return RowsAffected matching copyFromRows.
		// Our newFakeTxSuccess currently uses fixed inserted. Instead we build Tx that returns copyFromRows as inserted.
		// Override beginFunc to create Tx with custom Exec that parses copyFromRows.
		// For simplicity, make copyFromRows = 1000 for first chunks, but we can approximate by making Exec return INSERT 0 1000 for first two calls and 500 for last by counting calls.
		callIdx := 0
		pool.beginFunc = func(ctx context.Context) (pgx.Tx, error) {
			idx := callIdx
			callIdx++
			inserted := int64(1000)
			if idx == 2 {
				inserted = 500
			}
			tx := newFakeTxSuccess(inserted)
			// adjust CopyFrom to report same as inserted (CopyFrom returns copied, Exec returns inserted)
			tx.copyFromRows = inserted
			// override Exec to ensure correct tag
			tx.execFunc = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "CREATE TEMP TABLE") {
					return pgconn.NewCommandTag("CREATE TABLE"), nil
				}
				if strings.Contains(sql, "INSERT INTO telemetry") {
					return pgconn.NewCommandTag(strings.Join([]string{"INSERT 0", itoa(int(inserted))}, " ")), nil
				}
				return pgconn.NewCommandTag(""), nil
			}
			return tx, nil
		}
		w := NewWriterWithPool(pool)

		// Act
		n, err := w.WriteBatch(ctx, evts)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if n != 2500 {
			t.Fatalf("expected 2500 got %d", n)
		}
		if pool.beginCalls != 3 {
			t.Fatalf("expected 3 Begin calls, got %d", pool.beginCalls)
		}
	})

	t.Run("chunk failure on second chunk returns partial total and chunk index error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC().Truncate(time.Microsecond)
		evts := make([]telemetry.TelemetryEvent, 1500)
		for i := 0; i < 1500; i++ {
			evts[i] = newEvent(validID(i%10000), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now.Add(time.Duration(i)*time.Millisecond))
		}
		callIdx := 0
		pool := &fakePool{
			beginFunc: func(ctx context.Context) (pgx.Tx, error) {
				idx := callIdx
				callIdx++
				if idx == 1 {
					return &fakeTx{
						copyFromRows: 500,
						execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
							if strings.Contains(sql, "CREATE TEMP TABLE") {
								return pgconn.NewCommandTag("CREATE TABLE"), nil
							}
							return pgconn.NewCommandTag(""), errors.New("second chunk insert fail")
						},
					}, nil
				}
				// first chunk succeeds 1000
				tx := newFakeTxSuccess(1000)
				tx.copyFromRows = 1000
				tx.execFunc = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					if strings.Contains(sql, "CREATE TEMP TABLE") {
						return pgconn.NewCommandTag("CREATE TABLE"), nil
					}
					return pgconn.NewCommandTag("INSERT 0 1000"), nil
				}
				return tx, nil
			},
		}
		w := NewWriterWithPool(pool)

		// Act
		n, err := w.WriteBatch(ctx, evts)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "chunk 1 failed") {
			t.Fatalf("expected chunk 1 failed got %v", err)
		}
		if n != 1000 {
			t.Fatalf("expected partial 1000 got %d", n)
		}
	})

	t.Run("exactly 1000 uses single transaction not chunked", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := make([]telemetry.TelemetryEvent, 1000)
		for i := 0; i < 1000; i++ {
			evts[i] = newEvent(validID(i%10000), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now.Add(time.Duration(i)*time.Millisecond))
		}
		tx := newFakeTxSuccess(1000)
		pool := &fakePool{tx: tx}
		w := NewWriterWithPool(pool)

		// Act
		n, err := w.WriteBatch(ctx, evts)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if n != 1000 {
			t.Fatalf("expected 1000 got %d", n)
		}
		if pool.beginCalls != 1 {
			t.Fatalf("expected 1 Begin, got %d", pool.beginCalls)
		}
	})

	t.Run("1001 triggers chunked 2 transactions", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := make([]telemetry.TelemetryEvent, 1001)
		for i := 0; i < 1001; i++ {
			evts[i] = newEvent(validID(i%10000), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now.Add(time.Duration(i)*time.Millisecond))
		}
		callIdx := 0
		pool := &fakePool{
			beginFunc: func(ctx context.Context) (pgx.Tx, error) {
				idx := callIdx
				callIdx++
				inserted := int64(1000)
				if idx == 1 {
					inserted = 1
				}
				tx := newFakeTxSuccess(inserted)
				tx.copyFromRows = inserted
				tx.execFunc = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					if strings.Contains(sql, "CREATE TEMP TABLE") {
						return pgconn.NewCommandTag("CREATE TABLE"), nil
					}
					return pgconn.NewCommandTag(strings.Join([]string{"INSERT 0", itoa(int(inserted))}, " ")), nil
				}
				return tx, nil
			},
		}
		w := NewWriterWithPool(pool)

		// Act
		n, err := w.WriteBatch(ctx, evts)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if n != 1001 {
			t.Fatalf("expected 1001 got %d", n)
		}
		if pool.beginCalls != 2 {
			t.Fatalf("expected 2 Begins, got %d", pool.beginCalls)
		}
	})
}

func TestWriter_CircuitBreaker(t *testing.T) {
	// Covers [SPEC-001: AC-009, FR-010] resiliencia circuit breaker
	t.Run("breaker closed success returns via breaker path", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(30), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now)}
		tx := newFakeTxSuccess(1)
		pool := &fakePool{tx: tx}
		cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:        "test-closed",
			MaxRequests: 5,
			Timeout:     time.Second,
			ReadyToTrip: func(c gobreaker.Counts) bool { return false },
		})
		w := NewWriterWithPoolAndBreaker(pool, cb)

		// Act
		n, err := w.WriteBatch(ctx, evts)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if n != 1 {
			t.Fatalf("expected 1 got %d", n)
		}
		if cb.State() != gobreaker.StateClosed {
			t.Fatalf("expected closed got %v", cb.State())
		}
	})

	t.Run("breaker open returns error wrapped circuit breaker or db failed", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(31), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now)}
		pool := &fakePool{tx: newFakeTxSuccess(1)}
		cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:        "test-open",
			MaxRequests: 1,
			Interval:    0,
			Timeout:     10 * time.Second,
			ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= 1 },
		})
		// trip breaker with a failing call
		failPool := &fakePool{err: errors.New("db fail trip")}
		wFail := NewWriterWithPoolAndBreaker(failPool, cb)
		_, _ = wFail.WriteBatch(ctx, evts)
		// ensure breaker is open
		if cb.State() != gobreaker.StateOpen {
			t.Fatalf("expected open after trip, got %v", cb.State())
		}
		w := NewWriterWithPoolAndBreaker(pool, cb)

		// Act
		_, err := w.WriteBatch(ctx, evts)

		// Assert
		if err == nil {
			t.Fatalf("expected error due to breaker open")
		}
		if !strings.Contains(err.Error(), "circuit breaker or db failed") {
			t.Fatalf("expected circuit breaker or db failed got %v", err)
		}
		if !errors.Is(err, gobreaker.ErrOpenState) && !strings.Contains(err.Error(), "open") {
			t.Fatalf("expected ErrOpenState wrapped got %v", err)
		}
		// verify pool was not hit (Begin not called) because breaker blocked execution
		if pool.beginCalls != 0 {
			t.Fatalf("expected pool not called when breaker open, got %d", pool.beginCalls)
		}
	})

	t.Run("breaker failure wraps db error with circuit breaker or db failed", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(32), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now)}
		pool := &fakePool{err: errors.New("begin boom")}
		cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:        "test-wrap",
			MaxRequests: 5,
			ReadyToTrip: func(c gobreaker.Counts) bool { return false },
		})
		w := NewWriterWithPoolAndBreaker(pool, cb)

		// Act
		_, err := w.WriteBatch(ctx, evts)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "circuit breaker or db failed") {
			t.Fatalf("expected circuit breaker or db failed got %v", err)
		}
		if !strings.Contains(err.Error(), "begin tx failed") {
			t.Fatalf("expected begin tx failed in wrapped error got %v", err)
		}
	})

	t.Run("nil breaker bypasses circuit breaker", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(33), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now)}
		tx := newFakeTxSuccess(1)
		pool := &fakePool{tx: tx}
		w := NewWriterWithPool(pool) // no breaker

		// Act
		n, err := w.WriteBatch(ctx, evts)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if n != 1 {
			t.Fatalf("expected 1 got %d", n)
		}
	})
}

func TestEnsureStaging(t *testing.T) {
	// Covers [SPEC-001: AC-009, FR-010, BR-008] ensureStaging CREATE TEMP TABLE
	t.Run("success creates temp table", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		tx := &fakeTx{
			execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("CREATE TABLE"), nil
			},
		}

		// Act
		err := ensureStaging(ctx, tx)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if len(tx.execCalls) != 1 {
			t.Fatalf("expected 1 exec call got %d", len(tx.execCalls))
		}
		if !strings.Contains(tx.execCalls[0], "telemetry_staging") {
			t.Fatalf("expected telemetry_staging in sql got %v", tx.execCalls[0])
		}
	})

	t.Run("error wrapped as create staging failed", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		tx := &fakeTx{execErr: errors.New("exec fail")}

		// Act
		err := ensureStaging(ctx, tx)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "create staging failed") {
			t.Fatalf("expected create staging failed got %v", err)
		}
	})
}

func TestCopyToStaging(t *testing.T) {
	// Covers [SPEC-001: AC-009, FR-010] CopyFrom staging
	t.Run("success copies rows and returns count", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{
			newEvent(validID(40), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now),
			newEvent(validID(41), "GTP890", 20, nil, nil, now),
		}
		tx := &fakeTx{copyFromRows: 2}

		// Act
		n, err := copyToStaging(ctx, tx, evts)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if n != 2 {
			t.Fatalf("expected 2 got %d", n)
		}
		if tx.copyFromCalls != 1 {
			t.Fatalf("expected 1 copy call got %d", tx.copyFromCalls)
		}
	})

	t.Run("empty events still calls CopyFrom with zero rows", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		tx := &fakeTx{copyFromRows: 0}

		// Act
		n, err := copyToStaging(ctx, tx, nil)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if n != 0 {
			t.Fatalf("expected 0 got %d", n)
		}
	})

	t.Run("error wrapped as copyfrom staging failed", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(42), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now)}
		tx := &fakeTx{copyFromErr: errors.New("copy fail")}

		// Act
		_, err := copyToStaging(ctx, tx, evts)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "copyfrom staging failed") {
			t.Fatalf("expected copyfrom staging failed got %v", err)
		}
	})
}

func TestInsertFromStaging(t *testing.T) {
	// Covers [SPEC-001: AC-003, AC-009, FR-010] CTE dedup ON CONFLICT DO NOTHING
	t.Run("success returns RowsAffected", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		tx := &fakeTx{execTag: pgconn.NewCommandTag("INSERT 0 5")}

		// Act
		n, err := insertFromStaging(ctx, tx)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if n != 5 {
			t.Fatalf("expected 5 got %d", n)
		}
	})

	t.Run("error wrapped as insert via dedup failed", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		tx := &fakeTx{execErr: errors.New("insert fail")}

		// Act
		_, err := insertFromStaging(ctx, tx)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "insert via dedup failed") {
			t.Fatalf("expected insert via dedup failed got %v", err)
		}
	})

	t.Run("sql contains telemetry_dedup and telemetry tables", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		tx := &fakeTx{
			execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				if !strings.Contains(sql, "telemetry_dedup") || !strings.Contains(sql, "INSERT INTO telemetry") {
					return pgconn.NewCommandTag(""), errors.New("missing tables")
				}
				return pgconn.NewCommandTag("INSERT 0 1"), nil
			},
		}

		// Act
		n, err := insertFromStaging(ctx, tx)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if n != 1 {
			t.Fatalf("expected 1 got %d", n)
		}
	})
}

func TestWriter_WriteBatch_EmptyAndNewWriterHelpers(t *testing.T) {
	// Covers [SPEC-001: AC-009, BR-009] empty batch and constructors
	t.Run("empty batch returns ErrEmptyBatch wrapped", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		w := NewWriterWithPool(nil)

		// Act
		_, err := w.WriteBatch(ctx, nil)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !errors.Is(err, ErrEmptyBatch) {
			t.Fatalf("expected ErrEmptyBatch got %v", err)
		}
	})

	t.Run("NewWriter nil pool uses in-memory", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(50), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now)}
		w := NewWriter(nil)

		// Act
		n, err := w.WriteBatch(ctx, evts)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if n != 1 {
			t.Fatalf("expected 1 got %d", n)
		}
	})

	t.Run("NewWriterWithBreaker nil pool still in-memory", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(51), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now)}
		cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "test"})
		w := NewWriterWithBreaker(nil, cb)

		// Act
		n, err := w.WriteBatch(ctx, evts)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if n != 1 {
			t.Fatalf("expected 1 got %d", n)
		}
	})

	t.Run("DBPool interface implemented by fakePool", func(t *testing.T) {
		// Arrange
		var _ DBPool = (*fakePool)(nil)
		pool := &fakePool{tx: newFakeTxSuccess(1)}
		w := NewWriterWithPool(pool)
		ctx := context.Background()
		now := time.Now().UTC()
		evts := []telemetry.TelemetryEvent{newEvent(validID(52), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now)}

		// Act
		n, err := w.WriteBatch(ctx, evts)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if n != 1 {
			t.Fatalf("expected 1 got %d", n)
		}
	})

	t.Run("NewWriter with non-nil pgxpool sets pool branch", func(t *testing.T) {
		// Arrange
		pool := &pgxpool.Pool{}
		cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "ctor-pool"})

		// Act
		w1 := NewWriter(pool)
		w2 := NewWriterWithBreaker(pool, cb)

		// Assert
		if w1 == nil || w1.pool == nil {
			t.Fatalf("expected w1.pool not nil")
		}
		if w2 == nil || w2.pool == nil {
			t.Fatalf("expected w2.pool not nil")
		}
		if w2.breaker == nil {
			t.Fatalf("expected breaker not nil")
		}
	})
}

func TestWriter_WriteWithPool_BreakerAndPoolNilInteraction(t *testing.T) {
	// Covers [SPEC-001: AC-009] ensures pool nil check precedes breaker for chunking
	t.Run("chunked in-memory with breaker still uses in-memory path", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		now := time.Now().UTC().Truncate(time.Microsecond)
		evts := make([]telemetry.TelemetryEvent, 1500)
		for i := 0; i < 1500; i++ {
			evts[i] = newEvent(validID(i%10000), "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), now.Add(time.Duration(i)*time.Millisecond))
		}
		cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "chunk-breaker", ReadyToTrip: func(c gobreaker.Counts) bool { return false }})
		w := NewWriterWithPoolAndBreaker(nil, cb)

		// Act
		n, err := w.WriteBatch(ctx, evts)

		// Assert
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
		if n != 1500 {
			t.Fatalf("expected 1500 got %d", n)
		}
	})
}
