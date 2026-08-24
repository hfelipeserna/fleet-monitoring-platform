package domain

import (
	"fmt"
	"time"

	shared "fleetmonitoring/backend/internal/shared/domain"
)

type VehiclePos struct {
	Plate      string
	Lat        *float64
	Lon        *float64
	Speed      int
	ReceivedAt time.Time
	Status     string
}

func (v VehiclePos) Validate() error {
	if _, err := shared.ParsePlate(v.Plate); err != nil {
		return fmt.Errorf("plate validation failed: %w", err)
	}
	if err := validateSpeed(v.Speed); err != nil {
		return err
	}
	if err := validateVehicleLatLon(v.Lat, v.Lon); err != nil {
		return err
	}
	if err := validateStatus(v.Status); err != nil {
		return err
	}
	if err := validateReceivedAt(v.ReceivedAt); err != nil {
		return err
	}
	return nil
}
