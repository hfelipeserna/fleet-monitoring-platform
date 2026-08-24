package domain_test

import (
	"errors"
	"testing"

	shared "fleetmonitoring/backend/internal/shared/domain"
)

func strPtr(s string) *string { return &s }

// Covers [SPEC-003: AC-001, BR-001, BR-004, FR-005]

func TestValidateStoppedParams(t *testing.T) {
	t.Run("accepts minMinutes 1..1440 and clamps limit", func(t *testing.T) {
		// Covers [SPEC-003: AC-001, BR-001]
		// Arrange
		zone := "550e8400-e29b-41d4-a716-446655440000"

		// Act
		limit, err := shared.ValidateStoppedParams(20, 20, &zone)

		// Assert
		if err != nil {
			t.Fatalf("expected valid, got %v", err)
		}
		if limit != 20 {
			t.Fatalf("expected 20 got %d", limit)
		}
	})

	t.Run("rejects minMinutes out of range", func(t *testing.T) {
		// Covers [SPEC-003: AC-009, BR-009]
		// Arrange
		cases := []int{0, -1, 1441, 2000}
		for _, v := range cases {
			// Act
			_, err := shared.ValidateStoppedParams(v, 10, nil)

			// Assert
			if err == nil {
				t.Fatalf("expected error for minMinutes %d", v)
			}
			if !errors.Is(err, shared.ErrValidation) {
				t.Fatalf("expected ErrValidation for %d got %v", v, err)
			}
		}
	})

	t.Run("validates zoneID UUID", func(t *testing.T) {
		// Covers [SPEC-003: BR-009]
		// Arrange
		bad := strPtr("not-a-uuid")
		good := strPtr("550e8400-e29b-41d4-a716-446655440000")

		// Act
		_, errBad := shared.ValidateStoppedParams(20, 10, bad)
		_, errGood := shared.ValidateStoppedParams(20, 10, good)
		_, errNil := shared.ValidateStoppedParams(20, 10, nil)

		// Assert
		if errBad == nil || !errors.Is(errBad, shared.ErrValidation) {
			t.Fatalf("expected ErrValidation for bad uuid, got %v", errBad)
		}
		if errGood != nil {
			t.Fatalf("expected valid uuid, got %v", errGood)
		}
		if errNil != nil {
			t.Fatalf("expected nil zoneID ok, got %v", errNil)
		}
	})

	t.Run("clamps limit 0 to default 20", func(t *testing.T) {
		// Covers [SPEC-003: AC-005, BR-004]
		// Arrange
		// Act
		limit, err := shared.ValidateStoppedParams(20, 0, nil)

		// Assert
		if err != nil {
			t.Fatalf("expected valid, got %v", err)
		}
		if limit != shared.StoppedLimitDefault {
			t.Fatalf("expected default %d got %d", shared.StoppedLimitDefault, limit)
		}
	})

	t.Run("clamps limit >20 to 20", func(t *testing.T) {
		// Covers [SPEC-003: AC-005, BR-004]
		// Arrange
		// Act
		limit, err := shared.ValidateStoppedParams(20, 30, nil)

		// Assert
		if err != nil {
			t.Fatalf("expected valid, got %v", err)
		}
		if limit != 20 {
			t.Fatalf("expected 20 got %d", limit)
		}
	})

	t.Run("accepts limit boundaries 1 and 20", func(t *testing.T) {
		// Covers [SPEC-003: BR-004]
		// Arrange
		// Act
		l1, err1 := shared.ValidateStoppedParams(20, 1, nil)
		l20, err20 := shared.ValidateStoppedParams(20, 20, nil)

		// Assert
		if err1 != nil || l1 != 1 {
			t.Fatalf("expected 1 got %d err %v", l1, err1)
		}
		if err20 != nil || l20 != 20 {
			t.Fatalf("expected 20 got %d err %v", l20, err20)
		}
	})

	t.Run("minMinutes boundaries 1 and 1440", func(t *testing.T) {
		// Covers [SPEC-003: BR-009]
		// Arrange
		// Act
		_, err1 := shared.ValidateStoppedParams(1, 10, nil)
		_, err1440 := shared.ValidateStoppedParams(1440, 10, nil)

		// Assert
		if err1 != nil {
			t.Fatalf("expected 1 valid, got %v", err1)
		}
		if err1440 != nil {
			t.Fatalf("expected 1440 valid, got %v", err1440)
		}
	})
}
