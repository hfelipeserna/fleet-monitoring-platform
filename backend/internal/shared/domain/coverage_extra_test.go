package domain

import (
	"strings"
	"testing"
)

// Covers [SPEC-002: BR-002, FR-003]

func TestCoverage_SharedDomain(t *testing.T) {
	t.Run("ValidateMessage empty too long and valid", func(t *testing.T) {
		// Arrange
		// Act
		err1 := ValidateMessage("")
		err2 := ValidateMessage("   ")
		err3 := ValidateMessage(strings.Repeat("a", 4001))
		err4 := ValidateMessage("hello")
		err5 := ValidateMessage(strings.Repeat("a", 4000))
		// Assert
		if err1 == nil || err2 == nil || err3 == nil {
			t.Fatalf("expected errors for empty/long, got %v %v %v", err1, err2, err3)
		}
		if err4 != nil || err5 != nil {
			t.Fatalf("expected nil for valid, got %v %v", err4, err5)
		}
	})

	t.Run("Round3", func(t *testing.T) {
		// Arrange
		// Act
		v := Round3(1.23456)
		// Assert
		if v != 1.235 {
			t.Fatalf("expected 1.235, got %v", v)
		}
	})
}
