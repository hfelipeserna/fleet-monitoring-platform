package domain

import (
	"testing"
)

func TestValidateUUID(t *testing.T) {
	// Covers [SPEC-002: BR-004]
	t.Run("valid uuid", func(t *testing.T) {
		// Arrange
		id := "550e8400-e29b-41d4-a716-446655440000"

		// Act
		err := ValidateUUID(id)

		// Assert
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("invalid uuid", func(t *testing.T) {
		// Arrange
		id := "not-a-uuid"

		// Act
		err := ValidateUUID(id)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("empty uuid fails", func(t *testing.T) {
		// Arrange
		// Act
		err := ValidateUUID("")

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}
