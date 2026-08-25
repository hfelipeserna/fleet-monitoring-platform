package domain

import (
	"testing"
)

// Covers [SPEC-003: AC-001]

func TestCoverage_AssistantDomain(t *testing.T) {
	t.Run("NewChatRequest and ApplyDefaults", func(t *testing.T) {
		// Arrange
		// Act
		req := NewChatRequest("hello")
		req.ApplyDefaults()
		// Assert
		if req.Message != "hello" {
			t.Fatalf("expected hello, got %+v", req)
		}
		if req.Limit != DefaultLimit || req.MinMinutes != DefaultMinMinutes {
			t.Fatalf("expected defaults %d %d, got %d %d", DefaultLimit, DefaultMinMinutes, req.Limit, req.MinMinutes)
		}
		// empty request defaults
		empty := NewChatRequest("")
		empty.Limit = 0
		empty.MinMinutes = 0
		empty.ApplyDefaults()
		if empty.Limit != DefaultLimit || empty.MinMinutes != DefaultMinMinutes {
			t.Fatalf("expected defaults for empty, got %d %d", empty.Limit, empty.MinMinutes)
		}
	})

	t.Run("StoppedVehicle Validate edge", func(t *testing.T) {
		// Arrange
		v := StoppedVehicle{Plate: "BAD", ZoneID: "bad", Lat: 100, Lon: 200}
		// Act
		err := v.Validate()
		// Assert
		if err == nil {
			t.Fatalf("expected error bad fields")
		}
		v2 := StoppedVehicle{Plate: "ABC123", ZoneID: "550e8400-e29b-41d4-a716-446655440001", Lat: 4.7, Lon: -74}
		if err2 := v2.Validate(); err2 != nil {
			t.Fatalf("expected nil, got %v", err2)
		}
		if v2.Lat != 4.7 {
			// Round3 should be applied
			if v2.Lat != 4.7 {
				t.Fatalf("expected lat 4.7, got %v", v2.Lat)
			}
		}
		norm := v2.Normalized()
		if norm.Lat != 4.7 {
			t.Fatalf("expected normalized lat 4.7, got %v", norm.Lat)
		}
	})
}
