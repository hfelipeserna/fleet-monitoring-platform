//go:build integration

package pg_test

import (
	"context"
	"os"
	"strings"
	"testing"

	fleetpg "fleetmonitoring/backend/internal/fleet/adapters/pg"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStoppedPG_Integration_EXPLAIN(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration in short mode")
	}
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
	explainSQL := "EXPLAIN SELECT DISTINCT ON (plate) plate, zone_name, duration_min, lat, lon, stopped_since FROM telemetry JOIN critical_zones ON ST_Within(telemetry.geom::geometry, critical_zones.geom) WHERE speed=0 AND now() - received_at >= interval '20 minutes' LIMIT 20"
	_ = reader
	_ = ctx
	rows, err := pool.Query(ctx, explainSQL)
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
}
