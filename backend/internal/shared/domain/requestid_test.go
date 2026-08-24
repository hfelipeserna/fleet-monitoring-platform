package domain_test

import (
	"context"
	"testing"

	shared "fleetmonitoring/backend/internal/shared/domain"
)

// Covers [SPEC-003: FR-001, BR-005] — requestID propagation via context (X-Request-ID)

func TestWithRequestID(t *testing.T) {
	t.Run("stores and retrieves requestID", func(t *testing.T) {
		// Covers [SPEC-003: FR-001]
		// Arrange
		ctx := context.Background()
		id := "550e8400-e29b-41d4-a716-446655440000"

		// Act
		ctx2 := shared.WithRequestID(ctx, id)
		got, ok := shared.RequestIDFromCtx(ctx2)

		// Assert
		if !ok {
			t.Fatal("expected ok true")
		}
		if got != id {
			t.Fatalf("expected %q got %q", id, got)
		}
	})

	t.Run("returns false when not set", func(t *testing.T) {
		// Covers [SPEC-003: FR-001]
		// Arrange
		ctx := context.Background()

		// Act
		_, ok := shared.RequestIDFromCtx(ctx)

		// Assert
		if ok {
			t.Fatal("expected ok false for empty ctx")
		}
	})

	t.Run("overwrites previous value", func(t *testing.T) {
		// Covers [SPEC-003: FR-001]
		// Arrange
		ctx := shared.WithRequestID(context.Background(), "id-1")

		// Act
		ctx2 := shared.WithRequestID(ctx, "id-2")
		got, ok := shared.RequestIDFromCtx(ctx2)

		// Assert
		if !ok || got != "id-2" {
			t.Fatalf("expected id-2 got %q ok %v", got, ok)
		}
	})

	t.Run("isolates different contexts", func(t *testing.T) {
		// Covers [SPEC-003: FR-001]
		// Arrange
		ctx1 := shared.WithRequestID(context.Background(), "a")
		ctx2 := context.Background()

		// Act
		_, ok1 := shared.RequestIDFromCtx(ctx1)
		_, ok2 := shared.RequestIDFromCtx(ctx2)

		// Assert
		if !ok1 {
			t.Fatal("expected ctx1 ok")
		}
		if ok2 {
			t.Fatal("expected ctx2 not ok")
		}
	})

	t.Run("handles empty string", func(t *testing.T) {
		// Covers [SPEC-003: FR-001]
		// Arrange
		ctx := shared.WithRequestID(context.Background(), "")

		// Act
		got, ok := shared.RequestIDFromCtx(ctx)

		// Assert
		if !ok {
			t.Fatal("expected ok true for empty string value")
		}
		if got != "" {
			t.Fatalf("expected empty got %q", got)
		}
	})
}
