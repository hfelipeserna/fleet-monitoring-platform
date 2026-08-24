package domain_test

import (
	"errors"
	"math"
	"testing"
	"time"

	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

func vFloatPtr(v float64) *float64 { return &v }

func validVehiclePos() fleet.VehiclePos {
	return fleet.VehiclePos{
		Plate:      "GTP890",
		Lat:        vFloatPtr(4.711),
		Lon:        vFloatPtr(-74.072),
		Speed:      42,
		ReceivedAt: time.Now().UTC(),
		Status:     "moving",
	}
}

// Covers [SPEC-002: AC-001, BR-007, BR-010]
func TestVehiclePosValidate(t *testing.T) {
	t.Run("rejects plate GTP89", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007]
		// Arrange
		v := validVehiclePos()
		v.Plate = "GTP89"

		// Act
		err := v.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for plate GTP89")
		}
		if !errors.Is(err, shared.ErrValidation) {
			t.Fatalf("expected shared.ErrValidation for plate GTP89, got %v", err)
		}
	})

	t.Run("rejects speed -1", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007]
		// Arrange
		v := validVehiclePos()
		v.Speed = -1

		// Act
		err := v.Validate()

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

	t.Run("accepts lat lon nil", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007]
		// Arrange
		v := validVehiclePos()
		v.Lat = nil
		v.Lon = nil

		// Act
		err := v.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected nil for lat/lon nil, got %v", err)
		}
	})

	t.Run("rejects lat NaN", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007, BR-010]
		// Arrange
		v := validVehiclePos()
		nan := math.NaN()
		v.Lat = &nan

		// Act
		err := v.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for lat NaN")
		}
		if !errors.Is(err, fleet.ErrLatOutOfRange) {
			t.Fatalf("expected ErrLatOutOfRange for NaN, got %v", err)
		}
	})

	t.Run("rejects lat 91", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007]
		// Arrange
		v := validVehiclePos()
		v.Lat = vFloatPtr(91)

		// Act
		err := v.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for lat 91")
		}
		if !errors.Is(err, fleet.ErrLatOutOfRange) {
			t.Fatalf("expected ErrLatOutOfRange for 91, got %v", err)
		}
	})

	t.Run("rejects lat -91", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007]
		// Arrange
		v := validVehiclePos()
		v.Lat = vFloatPtr(-91)

		// Act
		err := v.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for lat -91")
		}
		if !errors.Is(err, fleet.ErrLatOutOfRange) {
			t.Fatalf("expected ErrLatOutOfRange for -91, got %v", err)
		}
	})

	t.Run("rejects lon 181", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007]
		// Arrange
		v := validVehiclePos()
		v.Lon = vFloatPtr(181)

		// Act
		err := v.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for lon 181")
		}
		if !errors.Is(err, fleet.ErrLonOutOfRange) {
			t.Fatalf("expected ErrLonOutOfRange for 181, got %v", err)
		}
	})

	t.Run("rejects lon -181", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007]
		// Arrange
		v := validVehiclePos()
		v.Lon = vFloatPtr(-181)

		// Act
		err := v.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for lon -181")
		}
		if !errors.Is(err, fleet.ErrLonOutOfRange) {
			t.Fatalf("expected ErrLonOutOfRange for -181, got %v", err)
		}
	})

	t.Run("rejects status invalid", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007]
		// Arrange
		v := validVehiclePos()
		v.Status = "flying"

		// Act
		err := v.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for invalid status flying")
		}
		if !errors.Is(err, fleet.ErrInvalidStatus) {
			t.Fatalf("expected ErrInvalidStatus, got %v", err)
		}
	})

	t.Run("accepts status empty", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007]
		// Arrange
		v := validVehiclePos()
		v.Status = ""

		// Act
		err := v.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected nil for empty status, got %v", err)
		}
	})

	t.Run("accepts status moving idle alert", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007]
		// Arrange
		cases := []string{"moving", "idle", "alert"}
		for _, s := range cases {
			s := s
			t.Run(s, func(t *testing.T) {
				// Arrange
				v := validVehiclePos()
				v.Status = s

				// Act
				err := v.Validate()

				// Assert
				if err != nil {
					t.Fatalf("expected status %q to be valid, got %v", s, err)
				}
			})
		}
	})

	t.Run("rejects zero ReceivedAt", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007]
		// Arrange
		v := validVehiclePos()
		v.ReceivedAt = time.Time{}

		// Act
		err := v.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for zero ReceivedAt")
		}
		if !errors.Is(err, fleet.ErrZeroTime) {
			t.Fatalf("expected ErrZeroTime, got %v", err)
		}
		if !errors.Is(err, fleet.ErrValidation) {
			t.Fatalf("expected ErrValidation wrapped, got %v", err)
		}
	})
}
