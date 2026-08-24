package idgen

import (
	"regexp"
	"testing"
)

func TestGenerator_NewID(t *testing.T) {
	// Covers [SPEC-001: BR-004, FR-003]
	t.Run("generates UUID v4 via shared generator", func(t *testing.T) {
		// Arrange
		g := New()
		re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

		// Act
		id := g.NewID()

		// Assert
		if !re.MatchString(id) {
			t.Fatalf("expected uuid v4, got %q", id)
		}
	})

	t.Run("GenerateUUID function matches generator", func(t *testing.T) {
		// Arrange
		re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

		// Act
		id := GenerateUUID()

		// Assert
		if !re.MatchString(id) {
			t.Fatalf("expected uuid v4, got %q", id)
		}
	})

	t.Run("New returns non-nil generator", func(t *testing.T) {
		// Arrange
		// Act
		g := New()

		// Assert
		if g == nil {
			t.Fatalf("expected non-nil generator")
		}
	})
}
