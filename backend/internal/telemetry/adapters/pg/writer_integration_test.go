//go:build integration

package pg

import (
	"context"
	"os"
	"testing"
	"time"

	telemetry "fleetmonitoring/backend/internal/telemetry/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPGWriter_Integration_RealDB(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration in short mode")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool creation failed: %v", err)
	}
	defer pool.Close()
	ctx := context.Background()
	receivedAt := time.Now().UTC().Truncate(time.Microsecond)
	events := make([]telemetry.TelemetryEvent, 500)
	for i := 0; i < 500; i++ {
		events[i] = newEvent(validID(i), "GTP890", i%120, floatPtr(4.711), floatPtr(-74.072), receivedAt.Add(time.Duration(i)*time.Millisecond))
	}
	writer := NewWriter(pool)
	n, err := writer.WriteBatch(ctx, events)
	if err != nil {
		t.Fatalf("expected no error for real DB 500 CopyFrom, got %v", err)
	}
	if n != 500 {
		t.Fatalf("expected 500 rows inserted, got %d", n)
	}
	var cnt int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM telemetry WHERE plate='GTP890'`).Scan(&cnt); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if cnt < 500 {
		t.Fatalf("expected at least 500 rows in telemetry, got %d", cnt)
	}
	id := validID(9998)
	evt := newEvent(id, "GTP890", 10, floatPtr(4.7), floatPtr(-74.0), receivedAt.Add(2*time.Second))
	n2, err := writer.WriteBatch(ctx, []telemetry.TelemetryEvent{evt})
	if err != nil {
		t.Fatalf("second insert unexpected error %v", err)
	}
	if n2 != 1 {
		t.Fatalf("expected 1 row for new id, got %d", n2)
	}
	n3, err := writer.WriteBatch(ctx, []telemetry.TelemetryEvent{evt})
	if err != nil {
		t.Fatalf("duplicate insert should be DO NOTHING, got %v", err)
	}
	if n3 != 0 {
		t.Fatalf("expected 0 on duplicate, got %d", n3)
	}
}
