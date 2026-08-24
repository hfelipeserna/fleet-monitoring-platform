package application

import (
	"context"
	"time"

	"fleetmonitoring/backend/internal/assistant/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

type ByZoneCount struct {
	ZoneName string
	Count    int
}

type FleetSummary struct {
	Total  int
	Moving int
	Idle   int
	Alert  int
	ByZone []ByZoneCount
}

type VehicleStatus struct {
	Plate      shared.Plate
	Lat        *float64
	Lon        *float64
	Speed      float64
	ReceivedAt time.Time
	Status     string
	ZoneName   *string
}

type Alert struct {
	Plate     shared.Plate
	AlertType string
	ZoneName  *string
	CreatedAt time.Time
}

type FleetQuerier interface {
	FindStoppedInZones(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error)
	GetFleetSummary(ctx context.Context) (FleetSummary, error)
	GetVehicleStatus(ctx context.Context, plate shared.Plate) (VehicleStatus, error)
	GetActiveAlerts(ctx context.Context, limit int) ([]Alert, error)
}
