package domain

import (
	"fmt"
	"time"

	shared "fleetmonitoring/backend/internal/shared/domain"
)

const (
	SpeedingBucket = 5 * time.Minute
	ZoneBucket     = 20 * time.Minute
	DedupWindow    = 2 * time.Minute
)

type Alert struct {
	EventID   string    `json:"event_id"`
	Plate     string    `json:"plate"`
	AlertType string    `json:"alert_type"`
	ZoneID    *string   `json:"zone_id"`
	ZoneName  *string   `json:"zone_name"`
	Lat       *float64  `json:"lat"`
	Lon       *float64  `json:"lon"`
	Speed     int       `json:"speed"`
	CreatedAt time.Time `json:"created_at"`
}

func BucketFor(alertType string, t time.Time) int64 {
	switch alertType {
	case "zone_enter", "zone_exit":
		return t.Truncate(ZoneBucket).Unix()
	default:
		return t.Truncate(SpeedingBucket).Unix()
	}
}

func (a Alert) MsgID() string {
	bucket := BucketFor(a.AlertType, a.CreatedAt)
	switch a.AlertType {
	case "zone_enter", "zone_exit":
		zid := ""
		if a.ZoneID != nil {
			zid = *a.ZoneID
		}
		return fmt.Sprintf("%s:%s:%s:%d", a.Plate, a.AlertType, zid, bucket)
	default:
		return fmt.Sprintf("%s:%s:%d", a.Plate, a.AlertType, bucket)
	}
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
