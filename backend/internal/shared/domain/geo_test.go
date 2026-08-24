package domain

import (
	"testing"
	"time"
)

func TestRound6(t *testing.T) {
	// Covers [SPEC-002: FR-001]
	t.Run("rounds to 6 decimals", func(t *testing.T) {
		// Arrange
		v := 1.123456789

		// Act
		got := Round6(v)

		// Assert
		if got != 1.123457 {
			t.Fatalf("expected 1.123457 got %v", got)
		}
	})
}

func TestEncodeDecodeCursor(t *testing.T) {
	// Covers [SPEC-002: FR-001]
	t.Run("encode decode roundtrip", func(t *testing.T) {
		// Arrange
		plate := "ABC123"
		ts := time.Now().UTC().Truncate(time.Microsecond)

		// Act
		cursor := EncodeCursor(plate, ts)
		gotPlate, gotTime, err := DecodeCursor(cursor)

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if string(gotPlate) != plate {
			t.Fatalf("expected %q got %q", plate, gotPlate)
		}
		if !gotTime.Equal(ts) {
			t.Fatalf("expected %v got %v", ts, gotTime)
		}
	})

	t.Run("decode invalid base64 returns error", func(t *testing.T) {
		// Arrange
		// Act
		_, _, err := DecodeCursor("!!!not-base64")

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("decode invalid format returns error", func(t *testing.T) {
		// Arrange
		cursor := "bm90LXZhbGlk" // base64 of "not-valid" without |
		// Act
		_, _, err := DecodeCursor(cursor)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("decode invalid plate returns error", func(t *testing.T) {
		// Arrange
		cursor := EncodeCursor("BAD", time.Now().UTC())
		// Act
		_, _, err := DecodeCursor(cursor)
		// Assert
		if err == nil {
			t.Fatalf("expected error for invalid plate")
		}
	})
}
