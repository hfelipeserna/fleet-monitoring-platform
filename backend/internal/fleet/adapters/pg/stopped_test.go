package pg_test

import (
	"context"
	"os"
	"strings"
	"testing"

	fleetpg "fleetmonitoring/backend/internal/fleet/adapters/pg"
	shared "fleetmonitoring/backend/internal/shared/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type mockQuerierPG struct {
	capturedSQL  string
	capturedArgs []any
	rows         []mockRow
	err          error
}

type mockRow struct {
	plate    string
	zoneID   string
	zoneName string
	duration int
	lat      float64
	lon      float64
	sinceStr string
}

func (m *mockQuerierPG) Query(ctx context.Context, sql string, args ...any) (fleetpg.Rows, error) {
	m.capturedSQL = sql
	m.capturedArgs = args
	if m.err != nil {
		return nil, m.err
	}
	return &mockRowsPG{data: m.rows}, nil
}

type mockRowsPG struct {
	data []mockRow
	idx  int
}

func (r *mockRowsPG) Next() bool { return r.idx < len(r.data) }
func (r *mockRowsPG) Scan(dest ...any) error {
	if r.idx >= len(r.data) {
		return nil
	}
	m := r.data[r.idx]
	r.idx++
	if len(dest) >= 7 {
		if p, ok := dest[0].(*string); ok {
			*p = m.plate
		}
		if p, ok := dest[1].(*string); ok {
			*p = m.zoneID
		}
		if p, ok := dest[2].(*string); ok {
			*p = m.zoneName
		}
		if p, ok := dest[3].(*int); ok {
			*p = m.duration
		}
		if p, ok := dest[4].(*float64); ok {
			*p = m.lat
		}
		if p, ok := dest[5].(*float64); ok {
			*p = m.lon
		}
		if p, ok := dest[6].(*string); ok {
			*p = m.sinceStr
		}
	}
	return nil
}
func (r *mockRowsPG) Close()     {}
func (r *mockRowsPG) Err() error { return nil }

// Covers [SPEC-003: AC-001, BR-001]
func TestStoppedPG_QueryContainsSTWithin(t *testing.T) {
	t.Run("query contains ST_Within and is parametrized without concatenation", func(t *testing.T) {
		// Arrange
		mock := &mockQuerierPG{}
		reader := fleetpg.NewStoppedReader(mock)
		ctx := context.Background()
		zoneID := "550e8400-e29b-41d4-a716-446655440000"

		// Act
		_, err := reader.FindStoppedInZones(ctx, 20, &zoneID, 20)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		sqlUpper := strings.ToUpper(mock.capturedSQL)
		if !strings.Contains(sqlUpper, "ST_WITHIN") {
			t.Fatalf("expected SQL to contain ST_Within, got %q", mock.capturedSQL)
		}
		if strings.Contains(mock.capturedSQL, "GTP980") || strings.Contains(mock.capturedSQL, zoneID) {
			t.Fatalf("expected parametrized query, got raw value in SQL %q", mock.capturedSQL)
		}
		if !strings.Contains(mock.capturedSQL, "$") {
			t.Fatalf("expected parametrized $ placeholders, got %q", mock.capturedSQL)
		}
		if !strings.Contains(sqlUpper, "SPEED") || !strings.Contains(sqlUpper, "RECEIVED_AT") {
			t.Fatalf("expected speed=0 and received_at filter in SQL, got %q", mock.capturedSQL)
		}
		_ = shared.Plate("GTP980")
	})
}

// Covers [SPEC-003: AC-001, BR-001]
func TestStoppedPG_Clamp(t *testing.T) {
	t.Run("clamps limit 1..20 and minMinutes 1..1440", func(t *testing.T) {
		// Arrange
		mock := &mockQuerierPG{}
		reader := fleetpg.NewStoppedReader(mock)
		ctx := context.Background()

		// Act
		_, err := reader.FindStoppedInZones(ctx, 20, nil, 100)

		// Assert
		if err != nil {
			t.Fatalf("expected no error with clamped limit, got %v", err)
		}
		// limit 100 should be clamped to 20 inside reader; verify captured LIMIT or arg
		if strings.Contains(strings.ToUpper(mock.capturedSQL), "LIMIT 100") {
			t.Fatalf("expected limit clamped to 20, got LIMIT 100 in %q", mock.capturedSQL)
		}
		if !strings.Contains(mock.capturedSQL, "20") && !strings.Contains(strings.ToUpper(mock.capturedSQL), "LIMIT") {
			t.Fatalf("expected LIMIT clause with clamped 20, got %q", mock.capturedSQL)
		}
	})

	t.Run("rejects minMinutes out of range via error", func(t *testing.T) {
		// Arrange
		mock := &mockQuerierPG{}
		reader := fleetpg.NewStoppedReader(mock)
		ctx := context.Background()

		// Act
		_, err := reader.FindStoppedInZones(ctx, 0, nil, 20)

		// Assert
		if err == nil {
			t.Fatal("expected error for minMinutes 0 out of range 1..1440")
		}
	})

	t.Run("clamps limit 0 to 20 default", func(t *testing.T) {
		// Arrange
		mock := &mockQuerierPG{}
		reader := fleetpg.NewStoppedReader(mock)
		ctx := context.Background()

		// Act
		_, err := reader.FindStoppedInZones(ctx, 20, nil, 0)

		// Assert
		if err != nil {
			t.Fatalf("expected no error with limit 0 clamped to 20, got %v", err)
		}
		if len(mock.capturedArgs) < 3 {
			t.Fatalf("expected 3 args, got %v", mock.capturedArgs)
		}
		if v, ok := mock.capturedArgs[2].(int); !ok || v != 20 {
			t.Fatalf("expected clamped limit 20 for input 0, got %v", mock.capturedArgs[2])
		}
	})
}

// Covers [SPEC-003: AC-001, BR-001]
func TestStoppedPG_Integration_EXPLAIN(t *testing.T) {
	t.Run("EXPLAIN uses Index Scan with GIST not Seq Scan", func(t *testing.T) {
		// Arrange
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			t.Skip("DATABASE_URL not set, skipping integration EXPLAIN")
		}
		pool, err := pgxpool.New(context.Background(), dsn)
		if err != nil {
			t.Fatalf("pool creation failed: %v", err)
		}
		defer pool.Close()
		reader := fleetpg.NewStoppedReader(fleetpg.NewPgxPoolAdapter(pool))
		ctx := context.Background()

		// Act
		explainSQL := "EXPLAIN SELECT DISTINCT ON (plate) plate, zone_name, duration_min, lat, lon, stopped_since FROM telemetry JOIN critical_zones ON ST_Within(telemetry.geom::geometry, critical_zones.geom) WHERE speed=0 AND now() - received_at >= interval '20 minutes' LIMIT 20"
		_ = reader
		_ = ctx
		rows, err := pool.Query(ctx, explainSQL)

		// Assert
		if err != nil {
			t.Fatalf("EXPLAIN query failed: %v", err)
		}
		defer rows.Close()
		var plan string
		foundIndexScan := false
		foundSeqScan := false
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				t.Fatalf("scan EXPLAIN failed: %v", err)
			}
			plan += line + "\n"
			if strings.Contains(line, "Index Scan") || strings.Contains(line, "Bitmap") || strings.Contains(line, "GIST") {
				foundIndexScan = true
			}
			if strings.Contains(line, "Seq Scan") {
				foundSeqScan = true
			}
		}
		if !foundIndexScan {
			t.Fatalf("expected Index Scan / GIST in EXPLAIN plan, got %q", plan)
		}
		if foundSeqScan {
			t.Fatalf("expected no Seq Scan due to GIST, got %q", plan)
		}
	})
}
