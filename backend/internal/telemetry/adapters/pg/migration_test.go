//go:build integration

package pg_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Covers [SPEC-001: AC-009, FR-010, BR-008]
// TEST-009 fragment: verifies hypertable + PK + index + geom GENERATED.
// Uses build tag integration as it touches DB migration file / real DB.

func TestTelemetryMigration(t *testing.T) {
	t.Run("migration file contains required DDL fragments", func(t *testing.T) {
		// Arrange
		// Resolve migration file relative to this test file.
		// Prefer backend/migrations/0001_telemetry.sql
		candidates := []string{
			filepath.Join("..", "..", "..", "..", "migrations", "0001_telemetry.sql"),
			filepath.Join("..", "..", "..", "migrations", "0001_telemetry.sql"),
			filepath.Join("..", "migrations", "0001_telemetry.sql"),
			"../../../../migrations/0001_telemetry.sql",
		}
		var content string
		var found bool
		for _, p := range candidates {
			b, err := os.ReadFile(p)
			if err == nil {
				content = string(b)
				found = true
				break
			}
		}
		if !found {
			// fallback: try working dir
			b, err := os.ReadFile(filepath.Join("migrations", "0001_telemetry.sql"))
			if err == nil {
				content = string(b)
				found = true
			}
		}
		if !found {
			t.Fatalf("migration file not found; candidates tried: %v", candidates)
		}
		lower := strings.ToLower(content)

		// Act
		checks := map[string]bool{
			"postgis extension":        strings.Contains(lower, "create extension") && strings.Contains(lower, "postgis"),
			"telemetry table":          strings.Contains(lower, "create table") && strings.Contains(lower, "telemetry"),
			"client_event_id uuid":     strings.Contains(lower, "client_event_id") && strings.Contains(lower, "uuid"),
			"plate check regex":        strings.Contains(content, "^[A-Z]{3}[0-9]{3}$"),
			"received_at timestamptz":  strings.Contains(lower, "received_at") && strings.Contains(lower, "timestamptz"),
			"primary key composite":    strings.Contains(lower, "primary key") && strings.Contains(lower, "client_event_id") && strings.Contains(lower, "received_at"),
			"create_hypertable":        strings.Contains(lower, "create_hypertable") && strings.Contains(lower, "received_at"),
			"chunk 1 day":              strings.Contains(lower, "chunk_time_interval") && strings.Contains(lower, "1 day"),
			"index plate received_at":  strings.Contains(lower, "create index") && strings.Contains(lower, "plate") && strings.Contains(lower, "received_at"),
			"index desc":               strings.Contains(lower, "desc"),
			"lat double precision":     strings.Contains(lower, "lat") && strings.Contains(lower, "double precision"),
			"lon double precision":     strings.Contains(lower, "lon") && strings.Contains(lower, "double precision"),
			"lat check between -90 90": strings.Contains(lower, "lat") && strings.Contains(lower, "between -90 and 90"),
			"lon check between -180 180": strings.Contains(lower, "lon") && strings.Contains(lower, "between -180 and 180"),
			"speed int check >=0":      strings.Contains(lower, "speed") && strings.Contains(lower, "check") && strings.Contains(lower, "speed >= 0"),
			"geom geography":           strings.Contains(lower, "geom") && strings.Contains(lower, "geography") && strings.Contains(lower, "point,4326"),
			"geom generated always":    strings.Contains(lower, "generated always as"),
			"geom stored":              strings.Contains(lower, "stored"),
			"st_makepoint":             strings.Contains(lower, "st_makepoint"),
		}

		// Assert
		for name, ok := range checks {
			if !ok {
				t.Errorf("missing DDL fragment: %s", name)
			}
		}
	})

	t.Run("migration SQL is not empty stub", func(t *testing.T) {
		// Arrange
		paths := []string{
			filepath.Join("..", "..", "..", "..", "migrations", "0001_telemetry.sql"),
			filepath.Join("..", "..", "..", "migrations", "0001_telemetry.sql"),
		}
		var content string
		var found bool
		for _, p := range paths {
			b, err := os.ReadFile(p)
			if err == nil {
				content = strings.TrimSpace(string(b))
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("migration file not found; tried %v", paths)
		}
		lower := strings.ToLower(content)

		// Act
		isStub := strings.Contains(lower, "todo") || !strings.Contains(lower, "create_hypertable") || len(content) < 500

		// Assert
		if isStub {
			t.Fatalf("migration appears to be stub (len %d): %q", len(content), content)
		}
	})
}
