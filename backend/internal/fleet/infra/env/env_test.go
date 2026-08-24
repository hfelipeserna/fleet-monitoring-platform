package env

import (
	"testing"
)

func TestFleetEnv(t *testing.T) {
	// Covers [SPEC-002: FR-001]
	t.Run("GetDatabaseURL returns empty when not set", func(t *testing.T) {
		// Arrange
		t.Setenv("DATABASE_URL", "")

		// Act
		got := GetDatabaseURL()

		// Assert
		if got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("GetDatabaseURL returns env value", func(t *testing.T) {
		// Arrange
		t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/fleet")

		// Act
		got := GetDatabaseURL()

		// Assert
		if got != "postgres://user:pass@localhost:5432/fleet" {
			t.Fatalf("unexpected %q", got)
		}
	})

	t.Run("GetNATSURL default when not set", func(t *testing.T) {
		// Arrange
		t.Setenv("NATS_URL", "")

		// Act
		got := GetNATSURL()

		// Assert
		if got != "nats://localhost:4222" {
			t.Fatalf("expected default nats url, got %q", got)
		}
	})

	t.Run("GetNATSURL returns env value", func(t *testing.T) {
		// Arrange
		t.Setenv("NATS_URL", "nats://nats:4222")

		// Act
		got := GetNATSURL()

		// Assert
		if got != "nats://nats:4222" {
			t.Fatalf("unexpected %q", got)
		}
	})

	t.Run("GetAPIPort defaults to 8080", func(t *testing.T) {
		// Arrange
		t.Setenv("API_PORT", "")
		t.Setenv("HTTP_PORT", "")
		t.Setenv("PORT", "")

		// Act
		got := GetAPIPort()

		// Assert
		if got != "8080" {
			t.Fatalf("expected 8080, got %q", got)
		}
	})

	t.Run("GetAPIPort prefers API_PORT over HTTP_PORT and PORT", func(t *testing.T) {
		// Arrange
		t.Setenv("API_PORT", "9001")
		t.Setenv("HTTP_PORT", "9002")
		t.Setenv("PORT", "9003")

		// Act
		got := GetAPIPort()

		// Assert
		if got != "9001" {
			t.Fatalf("expected 9001, got %q", got)
		}
	})

	t.Run("GetAPIPort falls back to HTTP_PORT", func(t *testing.T) {
		// Arrange
		t.Setenv("API_PORT", "")
		t.Setenv("HTTP_PORT", "9002")
		t.Setenv("PORT", "")

		// Act
		got := GetAPIPort()

		// Assert
		if got != "9002" {
			t.Fatalf("expected 9002, got %q", got)
		}
	})

	t.Run("GetAPIPort falls back to PORT", func(t *testing.T) {
		// Arrange
		t.Setenv("API_PORT", "")
		t.Setenv("HTTP_PORT", "")
		t.Setenv("PORT", "3000")

		// Act
		got := GetAPIPort()

		// Assert
		if got != "3000" {
			t.Fatalf("expected 3000, got %q", got)
		}
	})

	t.Run("Get returns default when empty", func(t *testing.T) {
		// Arrange
		t.Setenv("CUSTOM_KEY", "")

		// Act
		got := Get("CUSTOM_KEY", "defval")

		// Assert
		if got != "defval" {
			t.Fatalf("expected defval, got %q", got)
		}
	})

	t.Run("Get returns env when set", func(t *testing.T) {
		// Arrange
		t.Setenv("CUSTOM_KEY", "myval")

		// Act
		got := Get("CUSTOM_KEY", "defval")

		// Assert
		if got != "myval" {
			t.Fatalf("expected myval, got %q", got)
		}
	})
}
