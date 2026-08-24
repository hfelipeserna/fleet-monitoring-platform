package domain

import (
	"fmt"
	"time"

	shared "fleetmonitoring/backend/internal/shared/domain"
)

type Alert struct {
	EventID   string
	Plate     string
	AlertType string
	ZoneID    *string
	Lat       *float64
	Lon       *float64
	Speed     int
	CreatedAt time.Time
}

func (a Alert) Validate() error {
	if err := validateUUID(a.EventID); err != nil {
		return err
	}
	if _, err := shared.ParsePlate(a.Plate); err != nil {
		return fmt.Errorf("plate validation failed: %w", err)
	}
	if err := validateAlertType(a.AlertType); err != nil {
		return err
	}
	if err := validateAlertZoneID(a.AlertType, a.ZoneID); err != nil {
		return err
	}
	if err := validateAlertLatLon(a.Lat, a.Lon); err != nil {
		return err
	}
	if err := validateSpeed(a.Speed); err != nil {
		return err
	}
	if err := validateCreatedAt(a.CreatedAt); err != nil {
		return err
	}
	return nil
}
