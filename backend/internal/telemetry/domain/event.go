package domain

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	shared "fleetmonitoring/backend/internal/shared/domain"
)

var (
	uuidRegex           = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	ErrValidation       = errors.New("validation")
	ErrEmptyClientID    = errors.New("empty client_event_id")
	ErrInvalidUUID      = errors.New("invalid uuid format")
	ErrNegativeSpeed    = errors.New("negative speed")
	ErrLatOutOfRange    = errors.New("lat out of range")
	ErrLonOutOfRange    = errors.New("lon out of range")
	ErrOccurredInFuture = errors.New("occurred_at in future")
)

type TelemetryEvent struct {
	ClientEventID string
	Plate         string
	Speed         int
	Lat           *float64
	Lon           *float64
	ReceivedAt    time.Time
	OccurredAt    *time.Time
}

func (e TelemetryEvent) Validate() error {
	return e.ValidateAt(time.Now().UTC())
}

func (e TelemetryEvent) ValidateAt(now time.Time) error {
	if e.ClientEventID == "" {
		return fmt.Errorf("client_event_id required: %w", errors.Join(ErrEmptyClientID, ErrValidation))
	}
	if !uuidRegex.MatchString(e.ClientEventID) {
		return fmt.Errorf("client_event_id %q invalid uuid: %w", e.ClientEventID, errors.Join(ErrInvalidUUID, ErrValidation))
	}
	if _, err := shared.ParsePlate(e.Plate); err != nil {
		return fmt.Errorf("plate validation failed: %w", err)
	}
	if e.Speed < 0 {
		return fmt.Errorf("speed %d invalid: must be >= 0: %w", e.Speed, errors.Join(ErrNegativeSpeed, ErrValidation))
	}
	if e.Lat != nil {
		if *e.Lat < -90 || *e.Lat > 90 {
			return fmt.Errorf("lat %v out of range [-90,90]: %w", *e.Lat, errors.Join(ErrLatOutOfRange, ErrValidation))
		}
	}
	if e.Lon != nil {
		if *e.Lon < -180 || *e.Lon > 180 {
			return fmt.Errorf("lon %v out of range [-180,180]: %w", *e.Lon, errors.Join(ErrLonOutOfRange, ErrValidation))
		}
	}
	if e.OccurredAt != nil {
		if e.OccurredAt.After(now.Add(5 * time.Minute)) {
			return fmt.Errorf("occurred_at %v too far in future (>5m): %w", *e.OccurredAt, errors.Join(ErrOccurredInFuture, ErrValidation))
		}
	}
	return nil
}
