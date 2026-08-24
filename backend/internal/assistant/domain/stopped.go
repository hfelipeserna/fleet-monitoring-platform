package domain

import (
	"errors"
	"fmt"
	"time"

	shared "fleetmonitoring/backend/internal/shared/domain"
)

const (
	LatMin = -90
	LatMax = 90
	LonMin = -180
	LonMax = 180
)

type StoppedVehicle struct {
	Plate        shared.Plate
	ZoneID       string
	ZoneName     string
	DurationMin  int
	Lat          float64
	Lon          float64
	StoppedSince time.Time
}

func (v *StoppedVehicle) Validate() error {
	var errs []error

	if _, err := shared.ParsePlate(string(v.Plate)); err != nil {
		errs = append(errs, fmt.Errorf("plate %q invalid: %w", v.Plate, err))
	}

	if !shared.IsValidUUID(v.ZoneID) {
		errs = append(errs, fmt.Errorf("zoneID %q invalid: must be UUID", v.ZoneID))
	}

	if v.Lat < LatMin || v.Lat > LatMax {
		errs = append(errs, fmt.Errorf("lat %v invalid: must be %d..%d", v.Lat, LatMin, LatMax))
	}

	if v.Lon < LonMin || v.Lon > LonMax {
		errs = append(errs, fmt.Errorf("lon %v invalid: must be %d..%d", v.Lon, LonMin, LonMax))
	}

	if len(errs) > 0 {
		return fmt.Errorf("stopped vehicle validation: %w", errors.Join(append([]error{ErrValidation}, errs...)...))
	}

	v.Lat = shared.Round3(v.Lat)
	v.Lon = shared.Round3(v.Lon)

	return nil
}

func (v StoppedVehicle) Normalized() StoppedVehicle {
	n := v
	n.Lat = shared.Round3(v.Lat)
	n.Lon = shared.Round3(v.Lon)
	return n
}
