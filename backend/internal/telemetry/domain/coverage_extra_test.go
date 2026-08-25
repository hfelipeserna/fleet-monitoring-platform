package domain

import (
	"testing"
	"time"
)

// Covers [SPEC-001: AC-001, AC-004, BR-002]

func TestCoverage_TelemetryValidateAt(t *testing.T) {
	t.Run("ValidateAt future occurredAt -> error and valid", func(t *testing.T) {
		// Arrange
		now := time.Now().UTC()
		future := now.Add(10 * time.Minute)
		evt := TelemetryEvent{
			ClientEventID: "550e8400-e29b-41d4-a716-446655440000",
			Plate:         "GTP890",
			Speed:         10,
			OccurredAt:    &future,
			ReceivedAt:    now,
		}
		// Act
		err := evt.ValidateAt(now)
		// Assert
		if err == nil {
			t.Fatalf("expected error future occurredAt")
		}
		// valid with nil occurredAt
		evt2 := TelemetryEvent{
			ClientEventID: "550e8400-e29b-41d4-a716-446655440001",
			Plate:         "GTP890",
			Speed:         0,
			ReceivedAt:    now,
		}
		if err2 := evt2.ValidateAt(now); err2 != nil {
			t.Fatalf("expected nil valid, got %v", err2)
		}
		// invalid uuid
		evt3 := TelemetryEvent{ClientEventID: "bad", Plate: "GTP890", Speed: 10, ReceivedAt: now}
		if err3 := evt3.ValidateAt(now); err3 == nil {
			t.Fatalf("expected invalid uuid")
		}
		// empty client id
		evt4 := TelemetryEvent{ClientEventID: "", Plate: "GTP890", Speed: 10, ReceivedAt: now}
		if err4 := evt4.ValidateAt(now); err4 == nil {
			t.Fatalf("expected empty client id")
		}
		// negative speed
		evt5 := TelemetryEvent{ClientEventID: "550e8400-e29b-41d4-a716-446655440002", Plate: "GTP890", Speed: -1, ReceivedAt: now}
		if err5 := evt5.ValidateAt(now); err5 == nil {
			t.Fatalf("expected negative speed")
		}
	})
}
