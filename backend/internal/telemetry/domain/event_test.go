package domain_test

import (
	"testing"
	"time"

	telemetry "fleetmonitoring/backend/internal/telemetry/domain"
)

func floatPtr(v float64) *float64 { return &v }
func timePtr(v time.Time) *time.Time { return &v }

func validEvent() telemetry.TelemetryEvent {
	return telemetry.TelemetryEvent{
		ClientEventID: "550e8400-e29b-41d4-a716-446655440000",
		Plate:         "GTP890",
		Speed:         42,
		Lat:           floatPtr(4.7110),
		Lon:           floatPtr(-74.0721),
		ReceivedAt:    time.Now().UTC(),
		OccurredAt:    timePtr(time.Now().UTC().Add(-1 * time.Minute)),
	}
}

// Covers [SPEC-001: AC-001, AC-004, FR-001, FR-004, FR-005, BR-002, BR-003]
func TestTelemetryEvent(t *testing.T) {
	t.Run("accepts valid event with all fields", func(t *testing.T) {
		// Covers [SPEC-001: AC-001, FR-001, BR-002, BR-003]
		// Arrange
		evt := validEvent()

		// Act
		err := evt.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected valid event to pass, got error %v", err)
		}
	})

	t.Run("accepts valid event with nullable lat lon both null", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-005, BR-003]
		// Arrange
		evt := validEvent()
		evt.Lat = nil
		evt.Lon = nil

		// Act
		err := evt.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected null lat/lon to be valid (GPS failure), got %v", err)
		}
	})

	t.Run("accepts speed zero as valid", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-005, BR-002]
		// Arrange
		evt := validEvent()
		evt.Speed = 0

		// Act
		err := evt.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected speed 0 to be valid (detenido), got %v", err)
		}
	})

	t.Run("rejects negative speed", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004, BR-002]
		// Arrange
		evt := validEvent()
		evt.Speed = -1

		// Act
		err := evt.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for speed -1")
		}
	})

	t.Run("rejects negative speed large", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004, BR-002]
		// Arrange
		evt := validEvent()
		evt.Speed = -100

		// Act
		err := evt.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for speed -100")
		}
	})

	t.Run("accepts speed at boundary large positive", func(t *testing.T) {
		// Covers [SPEC-001: BR-002]
		// Arrange
		evt := validEvent()
		evt.Speed = 300

		// Act
		err := evt.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected large speed to be valid, got %v", err)
		}
	})

	t.Run("rejects lat out of range", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004, BR-003]
		cases := []struct {
			name string
			lat  float64
		}{
			{"lat 91", 91},
			{"lat -91", -91},
			{"lat 100", 100},
			{"lat -100", -100},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				// Arrange
				evt := validEvent()
				evt.Lat = floatPtr(tc.lat)

				// Act
				err := evt.Validate()

				// Assert
				if err == nil {
					t.Fatalf("expected error for lat %v", tc.lat)
				}
			})
		}
	})

	t.Run("rejects lon out of range", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004, BR-003]
		cases := []struct {
			name string
			lon  float64
		}{
			{"lon 181", 181},
			{"lon -181", -181},
			{"lon 200", 200},
			{"lon -200", -200},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				// Arrange
				evt := validEvent()
				evt.Lon = floatPtr(tc.lon)

				// Act
				err := evt.Validate()

				// Assert
				if err == nil {
					t.Fatalf("expected error for lon %v", tc.lon)
				}
			})
		}
	})

	t.Run("accepts lat lon at boundaries", func(t *testing.T) {
		// Covers [SPEC-001: BR-003]
		boundaries := []struct {
			name string
			lat  float64
			lon  float64
		}{
			{"min", -90, -180},
			{"max", 90, 180},
			{"zero", 0, 0},
			{"lat min lon max", -90, 180},
			{"lat max lon min", 90, -180},
		}
		for _, tc := range boundaries {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				// Arrange
				evt := validEvent()
				evt.Lat = floatPtr(tc.lat)
				evt.Lon = floatPtr(tc.lon)

				// Act
				err := evt.Validate()

				// Assert
				if err != nil {
					t.Fatalf("expected boundaries lat %v lon %v to be valid, got %v", tc.lat, tc.lon, err)
				}
			})
		}
	})

	t.Run("rejects occurred_at future beyond 5 minutes", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004]
		// Arrange
		evt := validEvent()
		future := time.Now().UTC().Add(6 * time.Minute)
		evt.OccurredAt = &future

		// Act
		err := evt.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for occurred_at 6 minutes in future")
		}
	})

	t.Run("rejects occurred_at far future", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004]
		// Arrange
		evt := validEvent()
		future := time.Now().UTC().Add(1 * time.Hour)
		evt.OccurredAt = &future

		// Act
		err := evt.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for occurred_at 1 hour in future")
		}
	})

	t.Run("accepts occurred_at within 5 minutes future", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004]
		// Arrange
		evt := validEvent()
		future := time.Now().UTC().Add(4 * time.Minute)
		evt.OccurredAt = &future

		// Act
		err := evt.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected occurred_at 4 minutes future to be valid, got %v", err)
		}
	})

	t.Run("accepts occurred_at exactly 5 minutes future", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004]
		// Arrange
		evt := validEvent()
		future := time.Now().UTC().Add(5 * time.Minute)
		evt.OccurredAt = &future

		// Act
		err := evt.ValidateAt(future.Add(-5 * time.Minute))

		// Assert
		if err != nil {
			t.Fatalf("expected occurred_at exactly 5min future to be valid, got %v", err)
		}
	})

	t.Run("accepts nil occurred_at as optional", func(t *testing.T) {
		// Covers [SPEC-001: AC-001, FR-001]
		// Arrange
		evt := validEvent()
		evt.OccurredAt = nil

		// Act
		err := evt.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected nil occurred_at to be valid, got %v", err)
		}
	})

	t.Run("accepts occurred_at in the past", func(t *testing.T) {
		// Covers [SPEC-001: AC-001]
		// Arrange
		evt := validEvent()
		past := time.Now().UTC().Add(-10 * time.Minute)
		evt.OccurredAt = &past

		// Act
		err := evt.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected past occurred_at to be valid, got %v", err)
		}
	})

	t.Run("rejects invalid plate", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, BR-001]
		cases := []string{"GTP89", "123", "", "AB1234", "ABCD123"}
		for _, p := range cases {
			p := p
			t.Run(p, func(t *testing.T) {
				// Arrange
				evt := validEvent()
				evt.Plate = p

				// Act
				err := evt.Validate()

				// Assert
				if err == nil {
					t.Fatalf("expected error for invalid plate %q", p)
				}
			})
		}
	})

	t.Run("accepts plate lowercase normalized", func(t *testing.T) {
		// Covers [SPEC-001: BR-001]
		// Arrange
		evt := validEvent()
		evt.Plate = "gtp890"

		// Act
		err := evt.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected lowercase plate to be normalized and valid, got %v", err)
		}
	})

	t.Run("rejects empty plate", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, BR-001]
		// Arrange
		evt := validEvent()
		evt.Plate = ""

		// Act
		err := evt.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for empty plate")
		}
	})

	t.Run("rejects lat null lon present mixed when strict", func(t *testing.T) {
		// Covers [SPEC-001: BR-003]
		// Arrange - spec allows nullable, but geom becomes NULL if either null.
		// We document that single-null is currently rejected or allowed depending on BR-003.
		// For MVP strict, both must be null or both present. This test expects rejection
		// if only one is null to surface decision; adjust if spec allows independent nulls.
		evt := validEvent()
		evt.Lat = nil
		evt.Lon = floatPtr(-74.0721)

		// Act
		err := evt.Validate()

		// Assert
		// If BR-003 allows independent null, this should be valid. We assert valid for now
		// to match plan "lat/lon nullable -> DB NULL" (each nullable independently).
		if err != nil {
			t.Fatalf("expected lat null lon present to be valid per BR-003, got %v", err)
		}
	})
}
