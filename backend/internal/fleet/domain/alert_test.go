package domain_test

import (
	"errors"
	"testing"
	"time"

	fleet "fleetmonitoring/backend/internal/fleet/domain"
)

func aFloatPtr(v float64) *float64 { return &v }

func strPtr(s string) *string { return &s }

func validAlert() fleet.Alert {
	return fleet.Alert{
		EventID:   "550e8400-e29b-41d4-a716-446655440003",
		Plate:     "GTP890",
		AlertType: "zone_enter",
		ZoneID:    strPtr("550e8400-e29b-41d4-a716-446655440002"),
		Lat:       aFloatPtr(4.711),
		Lon:       aFloatPtr(-74.072),
		Speed:     42,
		CreatedAt: time.Now().UTC(),
	}
}

// Covers [SPEC-002: AC-005, BR-004, BR-007, BR-011]
func TestAlertValidate(t *testing.T) {
	t.Run("rejects zone_enter sin zone_id", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-004, BR-007]
		// Arrange
		a := validAlert()
		a.AlertType = "zone_enter"
		a.ZoneID = nil

		// Act
		err := a.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for zone_enter without zone_id")
		}
		if !errors.Is(err, fleet.ErrMissingZone) {
			t.Fatalf("expected ErrMissingZone, got %v", err)
		}
		if !errors.Is(err, fleet.ErrValidation) {
			t.Fatalf("expected ErrValidation wrapped, got %v", err)
		}
	})

	t.Run("rejects zone_exit sin zone_id", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-004, BR-007]
		// Arrange
		a := validAlert()
		a.AlertType = "zone_exit"
		a.ZoneID = nil

		// Act
		err := a.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for zone_exit without zone_id")
		}
		if !errors.Is(err, fleet.ErrMissingZone) {
			t.Fatalf("expected ErrMissingZone for zone_exit, got %v", err)
		}
	})

	t.Run("rejects speeding_on con zone_id", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-004, BR-007, BR-011]
		// Arrange
		a := validAlert()
		a.AlertType = "speeding_on"
		a.ZoneID = strPtr("550e8400-e29b-41d4-a716-446655440002")

		// Act
		err := a.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for speeding_on with zone_id")
		}
		if !errors.Is(err, fleet.ErrUnexpectedZone) {
			t.Fatalf("expected ErrUnexpectedZone, got %v", err)
		}
		if !errors.Is(err, fleet.ErrValidation) {
			t.Fatalf("expected ErrValidation wrapped, got %v", err)
		}
	})

	t.Run("rejects speeding_off con zone_id", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-004, BR-007, BR-011]
		// Arrange
		a := validAlert()
		a.AlertType = "speeding_off"
		a.ZoneID = strPtr("550e8400-e29b-41d4-a716-446655440002")

		// Act
		err := a.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for speeding_off with zone_id")
		}
		if !errors.Is(err, fleet.ErrUnexpectedZone) {
			t.Fatalf("expected ErrUnexpectedZone for speeding_off, got %v", err)
		}
	})

	t.Run("rejects zone_enter bad uuid", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-007]
		// Arrange
		a := validAlert()
		a.AlertType = "zone_enter"
		a.ZoneID = strPtr("not-a-uuid")

		// Act
		err := a.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for zone_enter bad uuid")
		}
		if !errors.Is(err, fleet.ErrInvalidUUID) {
			t.Fatalf("expected ErrInvalidUUID for zone_id, got %v", err)
		}
	})

	t.Run("rejects EventID bad uuid", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-007]
		// Arrange
		a := validAlert()
		a.EventID = "bad-uuid"

		// Act
		err := a.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for bad EventID uuid")
		}
		if !errors.Is(err, fleet.ErrInvalidUUID) {
			t.Fatalf("expected ErrInvalidUUID for EventID, got %v", err)
		}
		if !errors.Is(err, fleet.ErrValidation) {
			t.Fatalf("expected ErrValidation wrapped, got %v", err)
		}
	})

	t.Run("rejects lat 91", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-007]
		// Arrange
		a := validAlert()
		a.Lat = aFloatPtr(91)

		// Act
		err := a.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for lat 91")
		}
		if !errors.Is(err, fleet.ErrLatOutOfRange) {
			t.Fatalf("expected ErrLatOutOfRange, got %v", err)
		}
	})

	t.Run("rejects speed -1", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-007, BR-011]
		// Arrange
		a := validAlert()
		a.Speed = -1

		// Act
		err := a.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for speed -1")
		}
		if !errors.Is(err, fleet.ErrNegativeSpeed) {
			t.Fatalf("expected ErrNegativeSpeed, got %v", err)
		}
		if !errors.Is(err, fleet.ErrValidation) {
			t.Fatalf("expected ErrValidation wrapped, got %v", err)
		}
	})

	t.Run("rejects created_at zero", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-007]
		// Arrange
		a := validAlert()
		a.CreatedAt = time.Time{}

		// Act
		err := a.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for zero CreatedAt")
		}
		if !errors.Is(err, fleet.ErrZeroTime) {
			t.Fatalf("expected ErrZeroTime, got %v", err)
		}
		if !errors.Is(err, fleet.ErrValidation) {
			t.Fatalf("expected ErrValidation wrapped, got %v", err)
		}
	})

	t.Run("accepts 4 tipos validos", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-004, BR-011]
		cases := []struct {
			name      string
			alertType string
			zoneID    *string
		}{
			{"zone_enter", "zone_enter", strPtr("550e8400-e29b-41d4-a716-446655440002")},
			{"zone_exit", "zone_exit", strPtr("550e8400-e29b-41d4-a716-446655440002")},
			{"speeding_on", "speeding_on", nil},
			{"speeding_off", "speeding_off", nil},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				// Arrange
				a := validAlert()
				a.AlertType = tc.alertType
				a.ZoneID = tc.zoneID

				// Act
				err := a.Validate()

				// Assert
				if err != nil {
					t.Fatalf("expected valid alert %q to pass, got %v", tc.name, err)
				}
			})
		}
	})

	t.Run("rejects alert_type invalid", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-007]
		// Arrange
		a := validAlert()
		a.AlertType = "invalid_type"

		// Act
		err := a.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for invalid alert_type")
		}
	})
}
