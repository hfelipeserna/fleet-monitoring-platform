package domain_test

import (
	"math"
	"testing"

	shared "fleetmonitoring/backend/internal/shared/domain"
	"fleetmonitoring/backend/internal/assistant/domain"
)

// Covers [SPEC-003: AC-001, AC-005, BR-004, FR-005]
// TEST-005 / Step 1 — Round helpers and StoppedVehicle domain

func TestRound3(t *testing.T) {
	t.Run("rounds 4.71111119 to 4.711 with 3 decimals", func(t *testing.T) {
		// Covers [SPEC-003: AC-001, AC-005, BR-004]
		// Arrange
		v := 4.71111119

		// Act
		got := domain.Round3(v)

		// Assert
		if math.Abs(got-4.711) > 1e-9 {
			t.Fatalf("expected 4.711 got %v", got)
		}
	})

	t.Run("rounds -74.07222229 to -74.072", func(t *testing.T) {
		// Covers [SPEC-003: BR-004]
		// Arrange
		v := -74.07222229

		// Act
		got := domain.Round3(v)

		// Assert
		if math.Abs(got-(-74.072)) > 1e-9 {
			t.Fatalf("expected -74.072 got %v", got)
		}
	})

	t.Run("rounds 0 to 0", func(t *testing.T) {
		// Covers [SPEC-003: BR-004]
		// Arrange
		v := 0.0

		// Act
		got := domain.Round3(v)

		// Assert
		if got != 0 {
			t.Fatalf("expected 0 got %v", got)
		}
	})

	t.Run("rounds half up 1.2345 to 1.235", func(t *testing.T) {
		// Covers [SPEC-003: BR-004]
		// Arrange
		v := 1.2345

		// Act
		got := domain.Round3(v)

		// Assert
		if math.Abs(got-1.235) > 1e-9 {
			t.Fatalf("expected 1.235 got %v", got)
		}
	})

	t.Run("shared Round6 still works control", func(t *testing.T) {
		// Covers [SPEC-002: FR-001, SPEC-003: BR-004]
		// Arrange
		v := 1.123456789

		// Act
		got := shared.Round6(v)

		// Assert
		if got != 1.123457 {
			t.Fatalf("expected shared Round6 1.123457 got %v", got)
		}
	})
}

func TestStoppedVehicle(t *testing.T) {
	t.Run("valid stopped vehicle with 3 dec rounding", func(t *testing.T) {
		// Covers [SPEC-003: AC-001, BR-004, FR-005]
		// Arrange
		v := domain.StoppedVehicle{
			Plate:       shared.Plate("GTP980"),
			ZoneID:      "550e8400-e29b-41d4-a716-446655440000",
			ZoneName:    "Zona Norte",
			DurationMin: 27,
			Lat:         4.71111119,
			Lon:         -74.07222229,
		}

		// Act
		err := v.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected valid stopped vehicle, got %v", err)
		}
		if math.Abs(v.Lat-4.711) > 1e-9 {
			t.Fatalf("expected lat rounded to 4.711, got %v", v.Lat)
		}
		if math.Abs(v.Lon-(-74.072)) > 1e-9 {
			t.Fatalf("expected lon rounded to -74.072, got %v", v.Lon)
		}
		n := v.Normalized()
		if math.Abs(n.Lat-4.711) > 1e-9 || math.Abs(n.Lon-(-74.072)) > 1e-9 {
			t.Fatalf("expected Normalized() to round without mutating on error path")
		}
	})

	t.Run("rejects invalid plate in stopped vehicle", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		v := domain.StoppedVehicle{
			Plate:    shared.Plate("GTP98"),
			ZoneID:   "550e8400-e29b-41d4-a716-446655440000",
			ZoneName: "Zona Norte",
			Lat:      4.71,
			Lon:      -74.07,
		}

		// Act
		err := v.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for invalid plate GTP98")
		}
		if v.Lat != 4.71 {
			t.Fatalf("expected no mutation on error path, got %v", v.Lat)
		}
	})

	t.Run("lat lon precision max 6 dec via Round6 then minimized to 3 dec", func(t *testing.T) {
		// Covers [SPEC-003: BR-004, FR-005] — BR-004 max 6 dec default 3
		// Arrange
		lat6 := shared.Round6(4.71111119)
		lat3 := domain.Round3(lat6)

		// Act
		_ = lat3

		// Assert
		if math.Abs(lat6-4.711111) > 1e-9 {
			t.Fatalf("expected Round6 4.711111, got %v", lat6)
		}
		if math.Abs(lat3-4.711) > 1e-9 {
			t.Fatalf("expected Round3 4.711, got %v", lat3)
		}
	})
}
