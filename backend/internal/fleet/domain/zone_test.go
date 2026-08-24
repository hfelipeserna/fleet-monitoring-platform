package domain_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	fleet "fleetmonitoring/backend/internal/fleet/domain"
)

func validZone() fleet.Zone {
	return fleet.Zone{
		ID:   "550e8400-e29b-41d4-a716-446655440001",
		Name: "Zona Norte",
		Coordinates: [][]float64{
			{-74.07, 4.71},
			{-74.05, 4.71},
			{-74.05, 4.73},
			{-74.07, 4.73},
			{-74.07, 4.71},
		},
	}
}

// Covers [SPEC-002: AC-003, BR-002, BR-007, BR-010]
func TestZoneValidate(t *testing.T) {
	t.Run("accepts valid Polygon cerrado 5 coords", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002]
		// Arrange
		z := validZone()

		// Act
		err := z.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected valid Polygon 5 coords to pass, got %v", err)
		}
	})

	t.Run("rejects not closed first!=last", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002]
		// Arrange
		z := validZone()
		z.Coordinates[4] = []float64{-74.06, 4.72}

		// Act
		err := z.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for not closed polygon first!=last")
		}
		if !errors.Is(err, fleet.ErrNotClosed) {
			t.Fatalf("expected ErrNotClosed, got %v", err)
		}
		if !errors.Is(err, fleet.ErrValidation) {
			t.Fatalf("expected ErrValidation wrapped, got %v", err)
		}
	})

	t.Run("rejects 3 coords", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002]
		// Arrange
		z := validZone()
		z.Coordinates = [][]float64{
			{-74.07, 4.71},
			{-74.05, 4.71},
			{-74.07, 4.71},
		}

		// Act
		err := z.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for 3 coords")
		}
		if !errors.Is(err, fleet.ErrCoordCount) {
			t.Fatalf("expected ErrCoordCount, got %v", err)
		}
		if !errors.Is(err, fleet.ErrValidation) {
			t.Fatalf("expected ErrValidation wrapped, got %v", err)
		}
	})

	t.Run("rejects 102 coords >101", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002]
		// Arrange
		z := validZone()
		coords := make([][]float64, 102)
		for i := 0; i < 101; i++ {
			angle := float64(i) * 2 * math.Pi / 101
			coords[i] = []float64{-74.06 + 0.01*math.Cos(angle), 4.72 + 0.01*math.Sin(angle)}
		}
		coords[101] = coords[0]
		z.Coordinates = coords

		// Act
		err := z.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for 102 coords >101")
		}
		if !errors.Is(err, fleet.ErrCoordCount) {
			t.Fatalf("expected ErrCoordCount for >101, got %v", err)
		}
	})

	t.Run("rejects area zero colineal", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002]
		// Arrange
		z := validZone()
		z.Coordinates = [][]float64{
			{-74.07, 4.71},
			{-74.05, 4.71},
			{-74.03, 4.71},
			{-74.07, 4.71},
		}

		// Act
		err := z.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for zero area colineal")
		}
		if !errors.Is(err, fleet.ErrZeroArea) {
			t.Fatalf("expected ErrZeroArea, got %v", err)
		}
		if !errors.Is(err, fleet.ErrValidation) {
			t.Fatalf("expected ErrValidation wrapped, got %v", err)
		}
	})

	t.Run("rejects self-intersect bowtie", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002]
		// Arrange
		z := validZone()
		z.Coordinates = [][]float64{
			{-74.07, 4.71},
			{-74.05, 4.73},
			{-74.07, 4.73},
			{-74.05, 4.71},
			{-74.07, 4.71},
		}

		// Act
		err := z.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for self-intersect bowtie")
		}
		if !errors.Is(err, fleet.ErrSelfIntersection) {
			t.Fatalf("expected ErrSelfIntersection, got %v", err)
		}
		if !errors.Is(err, fleet.ErrValidation) {
			t.Fatalf("expected ErrValidation wrapped, got %v", err)
		}
	})

	t.Run("rejects name empty", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-007]
		// Arrange
		z := validZone()
		z.Name = ""

		// Act
		err := z.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for empty name")
		}
		if !errors.Is(err, fleet.ErrInvalidName) {
			t.Fatalf("expected ErrInvalidName for empty, got %v", err)
		}
	})

	t.Run("rejects name 101 runes", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-007]
		// Arrange
		z := validZone()
		z.Name = strings.Repeat("a", 101)

		// Act
		err := z.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for name 101 runes")
		}
		if !errors.Is(err, fleet.ErrInvalidName) {
			t.Fatalf("expected ErrInvalidName for 101 runes, got %v", err)
		}
	})

	t.Run("rejects name blank", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-007]
		// Arrange
		z := validZone()
		z.Name = "   "

		// Act
		err := z.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for blank name")
		}
		if !errors.Is(err, fleet.ErrInvalidName) {
			t.Fatalf("expected ErrInvalidName for blank, got %v", err)
		}
	})

	t.Run("rejects uuid malformado", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-007]
		// Arrange
		z := validZone()
		z.ID = "not-a-uuid"

		// Act
		err := z.Validate()

		// Assert
		if err == nil {
			t.Fatal("expected error for malformed uuid")
		}
		if !errors.Is(err, fleet.ErrInvalidUUID) {
			t.Fatalf("expected ErrInvalidUUID, got %v", err)
		}
		if !errors.Is(err, fleet.ErrValidation) {
			t.Fatalf("expected ErrValidation wrapped, got %v", err)
		}
	})

	t.Run("round6 normaliza 4.71111119->4.711111", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-010]
		// Arrange
		z := validZone()
		z.Coordinates[0][1] = 4.71111119
		z.Coordinates[0][0] = -74.07222229
		z.Coordinates[len(z.Coordinates)-1][1] = 4.71111119
		z.Coordinates[len(z.Coordinates)-1][0] = -74.07222229

		// Act
		err := z.Validate()

		// Assert
		if err != nil {
			t.Fatalf("expected round6 normalization to pass, got %v", err)
		}
		if math.Abs(z.Coordinates[0][1]-4.711111) > 1e-9 {
			t.Fatalf("expected lat 4.71111119 rounded to 4.711111, got %v", z.Coordinates[0][1])
		}
		if math.Abs(z.Coordinates[0][0]-(-74.072222)) > 1e-9 {
			t.Fatalf("expected lon -74.07222229 rounded to -74.072222, got %v", z.Coordinates[0][0])
		}
	})
}
