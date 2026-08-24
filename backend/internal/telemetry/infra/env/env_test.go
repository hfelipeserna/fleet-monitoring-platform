package env

import (
	"testing"
	"time"
)

func TestTelemetryEnv(t *testing.T) {
	// Covers [SPEC-001: FR-001]
	t.Run("Get returns default when missing", func(t *testing.T) {
		// Arrange
		t.Setenv("TELEMETRY_FOO", "")

		// Act
		got := Get("TELEMETRY_FOO", "default")

		// Assert
		if got != "default" {
			t.Fatalf("expected default, got %q", got)
		}
	})

	t.Run("Get returns env value", func(t *testing.T) {
		// Arrange
		t.Setenv("TELEMETRY_FOO", "bar")

		// Act
		got := Get("TELEMETRY_FOO", "default")

		// Assert
		if got != "bar" {
			t.Fatalf("expected bar, got %q", got)
		}
	})

	t.Run("GetInt returns default when missing", func(t *testing.T) {
		// Arrange
		t.Setenv("TELEMETRY_INT", "")

		// Act
		got := GetInt("TELEMETRY_INT", 42)

		// Assert
		if got != 42 {
			t.Fatalf("expected 42, got %d", got)
		}
	})

	t.Run("GetInt parses valid int", func(t *testing.T) {
		// Arrange
		t.Setenv("TELEMETRY_INT", "123")

		// Act
		got := GetInt("TELEMETRY_INT", 42)

		// Assert
		if got != 123 {
			t.Fatalf("expected 123, got %d", got)
		}
	})

	t.Run("GetInt fallback on invalid", func(t *testing.T) {
		// Arrange
		t.Setenv("TELEMETRY_INT", "not-an-int")

		// Act
		got := GetInt("TELEMETRY_INT", 99)

		// Assert
		if got != 99 {
			t.Fatalf("expected 99 fallback, got %d", got)
		}
	})

	t.Run("GetInt64 parses valid int64", func(t *testing.T) {
		// Arrange
		t.Setenv("TELEMETRY_INT64", "1073741824")

		// Act
		got := GetInt64("TELEMETRY_INT64", 1024)

		// Assert
		if got != 1073741824 {
			t.Fatalf("expected 1073741824, got %d", got)
		}
	})

	t.Run("GetInt64 fallback on invalid", func(t *testing.T) {
		// Arrange
		t.Setenv("TELEMETRY_INT64", "bad")

		// Act
		got := GetInt64("TELEMETRY_INT64", 5<<30)

		// Assert
		if got != 5<<30 {
			t.Fatalf("expected fallback, got %d", got)
		}
	})

	t.Run("GetInt64 default when missing", func(t *testing.T) {
		// Arrange
		t.Setenv("TELEMETRY_INT64", "")

		// Act
		got := GetInt64("TELEMETRY_INT64", 1<<30)

		// Assert
		if got != 1<<30 {
			t.Fatalf("expected 1<<30, got %d", got)
		}
	})

	t.Run("GetInt64 ALERTS_MAX_BYTES 1GB parsed", func(t *testing.T) {
		// Arrange
		t.Setenv("ALERTS_MAX_BYTES", "1073741824")

		// Act
		got := GetInt64("ALERTS_MAX_BYTES", 1<<30)

		// Assert
		if got != 1073741824 {
			t.Fatalf("expected 1GB, got %d", got)
		}
	})

	t.Run("GetDuration parses valid duration", func(t *testing.T) {
		// Arrange
		t.Setenv("TELEMETRY_DUR", "5s")

		// Act
		got := GetDuration("TELEMETRY_DUR", 3*time.Second)

		// Assert
		if got != 5*time.Second {
			t.Fatalf("expected 5s, got %v", got)
		}
	})

	t.Run("GetDuration fallback on invalid", func(t *testing.T) {
		// Arrange
		t.Setenv("TELEMETRY_DUR", "bad-duration")

		// Act
		got := GetDuration("TELEMETRY_DUR", 3*time.Second)

		// Assert
		if got != 3*time.Second {
			t.Fatalf("expected fallback 3s, got %v", got)
		}
	})

	t.Run("GetDuration default when missing", func(t *testing.T) {
		// Arrange
		t.Setenv("TELEMETRY_DUR", "")

		// Act
		got := GetDuration("TELEMETRY_DUR", 10*time.Second)

		// Assert
		if got != 10*time.Second {
			t.Fatalf("expected 10s, got %v", got)
		}
	})
}
