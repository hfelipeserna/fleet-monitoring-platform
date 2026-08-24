package idgen

import (
	"regexp"
	"testing"
)

func TestGenerateUUID(t *testing.T) {
	// Covers [SPEC-001: BR-004, SPEC-002: BR-004]
	t.Run("generates UUID v4 format", func(t *testing.T) {
		// Arrange
		re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

		// Act
		uuid := GenerateUUID()

		// Assert
		if !re.MatchString(uuid) {
			t.Fatalf("expected uuid v4 format, got %q", uuid)
		}
	})

	t.Run("generates unique values", func(t *testing.T) {
		// Arrange
		// Act
		a := GenerateUUID()
		b := GenerateUUID()

		// Assert
		if a == b {
			t.Fatalf("expected unique uuids, got same %q", a)
		}
	})

	t.Run("has 36 char length with 4 dashes", func(t *testing.T) {
		// Arrange
		// Act
		uuid := GenerateUUID()

		// Assert
		if len(uuid) != 36 {
			t.Fatalf("expected len 36, got %d (%q)", len(uuid), uuid)
		}
		dashes := 0
		for _, c := range uuid {
			if c == '-' {
				dashes++
			}
		}
		if dashes != 4 {
			t.Fatalf("expected 4 dashes, got %d in %q", dashes, uuid)
		}
	})
}
