package domain

import "fmt"

const (
	StoppedMinMinutesMin     = 1
	StoppedMinMinutesMax     = 1440
	StoppedMinMinutesDefault = 20
	StoppedLimitMin          = 1
	StoppedLimitMax          = 20
	StoppedLimitDefault      = 20
)

func ValidateStoppedParams(minMinutes, limit int, zoneID *string) (int, error) {
	if minMinutes < StoppedMinMinutesMin || minMinutes > StoppedMinMinutesMax {
		return 0, fmt.Errorf("minMinutes %d out of range %d..%d: %w", minMinutes, StoppedMinMinutesMin, StoppedMinMinutesMax, ErrValidation)
	}
	if zoneID != nil && !IsValidUUID(*zoneID) {
		return 0, fmt.Errorf("zoneID %q invalid: must be UUID: %w", *zoneID, ErrValidation)
	}
	if limit <= 0 {
		limit = StoppedLimitDefault
	} else if limit > StoppedLimitMax {
		limit = StoppedLimitMax
	}
	return limit, nil
}
