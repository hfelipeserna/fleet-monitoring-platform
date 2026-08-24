package domain

import (
	"errors"
	"fmt"

	shared "fleetmonitoring/backend/internal/shared/domain"
)

const (
	MessageMinRunes    = 1
	MessageMaxRunes    = 4000
	LimitMin           = 1
	LimitMax           = 20
	MinMinutesMin      = 1
	MinMinutesMax      = 1440
	DefaultMinMinutes  = 20
	DefaultLimit       = LimitMax
)

var ErrValidation = shared.ErrValidation

type ChatRequest struct {
	Message    string
	Plate      *shared.Plate
	ZoneID     *string
	Limit      int
	MinMinutes int
	SessionID  *string
}

func NewChatRequest(message string) ChatRequest {
	return ChatRequest{
		Message:    message,
		Limit:      DefaultLimit,
		MinMinutes: DefaultMinMinutes,
	}
}

func (r *ChatRequest) ApplyDefaults() {
	if r.Limit == 0 {
		r.Limit = DefaultLimit
	}
	if r.MinMinutes == 0 {
		r.MinMinutes = DefaultMinMinutes
	}
}

func (r *ChatRequest) Validate() error {
	var errs []error
	var normalizedPlate *shared.Plate

	if err := shared.ValidateMessage(r.Message); err != nil {
		errs = append(errs, err)
	}

	if r.Plate != nil {
		p, err := shared.ParsePlate(string(*r.Plate))
		if err != nil {
			errs = append(errs, fmt.Errorf("plate %q invalid: %w", string(*r.Plate), err))
		} else {
			normalizedPlate = &p
		}
	}

	if r.ZoneID != nil {
		if !shared.IsValidUUID(*r.ZoneID) {
			errs = append(errs, fmt.Errorf("zoneID %q invalid: must be UUID", *r.ZoneID))
		}
	}

	if r.Limit < LimitMin || r.Limit > LimitMax {
		errs = append(errs, fmt.Errorf("limit %d invalid: must be %d..%d", r.Limit, LimitMin, LimitMax))
	}

	if r.MinMinutes < MinMinutesMin || r.MinMinutes > MinMinutesMax {
		errs = append(errs, fmt.Errorf("minMinutes %d invalid: must be %d..%d", r.MinMinutes, MinMinutesMin, MinMinutesMax))
	}

	if r.SessionID != nil {
		if !shared.IsValidUUID(*r.SessionID) {
			errs = append(errs, fmt.Errorf("sessionID %q invalid: must be UUID", *r.SessionID))
		}
	}

	if len(errs) == 0 {
		if normalizedPlate != nil {
			*r.Plate = *normalizedPlate
		}
		return nil
	}

	return fmt.Errorf("chat request validation: %w", errors.Join(append([]error{ErrValidation}, errs...)...))
}
