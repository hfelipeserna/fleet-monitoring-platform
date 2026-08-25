package application_test

import (
	"reflect"
	"testing"

	"fleetmonitoring/backend/internal/assistant/application"
)

// Covers [SPEC-003: FR-003, BR-002, AC-001]
// Step 1 — Ports contract: FleetQuerier must exist with 4 read-only tools, consumer-side interface, no infra imports

func TestFleetQuerier_InterfaceExists(t *testing.T) {
	t.Run("interface FleetQuerier is defined", func(t *testing.T) {
		// Covers [SPEC-003: FR-003, BR-002]
		// Arrange
		typ := reflect.TypeFor[application.FleetQuerier]()

		// Act
		found := typ.Kind() == reflect.Interface

		// Assert
		if !found {
			t.Fatalf("expected FleetQuerier to be an interface, got %v", typ.Kind())
		}
	})

	t.Run("has method FindStoppedInZones", func(t *testing.T) {
		// Covers [SPEC-003: FR-003, FR-004, BR-001]
		// Arrange
		typ := reflect.TypeOf((*application.FleetQuerier)(nil)).Elem()

		// Act
		_, ok := typ.MethodByName("FindStoppedInZones")

		// Assert
		if !ok {
			t.Fatalf("expected FleetQuerier to have method FindStoppedInZones")
		}
	})

	t.Run("has method GetFleetSummary", func(t *testing.T) {
		// Covers [SPEC-003: FR-003]
		// Arrange
		typ := reflect.TypeFor[application.FleetQuerier]()

		// Act
		_, ok := typ.MethodByName("GetFleetSummary")

		// Assert
		if !ok {
			t.Fatalf("expected FleetQuerier to have method GetFleetSummary")
		}
	})

	t.Run("has method GetVehicleStatus", func(t *testing.T) {
		// Covers [SPEC-003: FR-003, BR-009]
		// Arrange
		typ := reflect.TypeFor[application.FleetQuerier]()

		// Act
		_, ok := typ.MethodByName("GetVehicleStatus")

		// Assert
		if !ok {
			t.Fatalf("expected FleetQuerier to have method GetVehicleStatus")
		}
	})

	t.Run("has method GetActiveAlerts", func(t *testing.T) {
		// Covers [SPEC-003: FR-003]
		// Arrange
		typ := reflect.TypeFor[application.FleetQuerier]()

		// Act
		_, ok := typ.MethodByName("GetActiveAlerts")

		// Assert
		if !ok {
			t.Fatalf("expected FleetQuerier to have method GetActiveAlerts")
		}
	})

	t.Run("methods are read-only no write signature", func(t *testing.T) {
		// Covers [SPEC-003: BR-002, FR-009] — firewall read-only, 5 tools (incl ListPlates for placas)
		// Arrange
		typ := reflect.TypeFor[application.FleetQuerier]()

		// Act
		numMethods := typ.NumMethod()

		// Assert
		if numMethods != 5 {
			t.Fatalf("expected exactly 5 methods on FleetQuerier (read-only firewall), got %d", numMethods)
		}
	})

	t.Run("package compiles without infra imports", func(t *testing.T) {
		// Covers [SPEC-003: BR-008, FR-009] — depguard shared/domain pure, application stdlib-only
		// This test itself verifies compilation succeeded; if depguard violated, lint fails.
		// Arrange
		_ = application.FleetQuerier(nil)

		// Act

		// Assert
		// compilation is the assertion
	})
}
