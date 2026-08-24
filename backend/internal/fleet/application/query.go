package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

type FleetReader interface {
	LastPositions(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error)
	History(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error)
}

type QueryService struct {
	reader FleetReader
}

func NewQueryService(reader FleetReader) *QueryService {
	return &QueryService{reader: reader}
}

func validateLimit(limit int) error {
	if limit < 1 || limit > 500 {
		return fmt.Errorf("limit %d out of range 1..500: %w", limit, shared.ErrValidation)
	}
	return nil
}

func validatePlateStr(s *string) (*shared.Plate, error) {
	if s == nil {
		return nil, nil
	}
	p, err := shared.ParsePlate(*s)
	if err != nil {
		return nil, fmt.Errorf("plate %q invalid: %w", *s, errors.Join(shared.ErrInvalidPlate, shared.ErrValidation, err))
	}
	return &p, nil
}

func validateCursor(cursor string) error {
	if cursor == "" {
		return nil
	}
	_, _, err := shared.DecodeCursor(cursor)
	return err
}

func roundPositions(in []fleet.VehiclePos) []fleet.VehiclePos {
	out := make([]fleet.VehiclePos, len(in))
	for i, v := range in {
		nv := v
		if nv.Lat != nil {
			r := shared.Round6(*nv.Lat)
			nv.Lat = &r
		}
		if nv.Lon != nil {
			r := shared.Round6(*nv.Lon)
			nv.Lon = &r
		}
		out[i] = nv
	}
	return out
}

func (s *QueryService) LastPositions(ctx context.Context, plate *string, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
	if err := validateLimit(limit); err != nil {
		return nil, "", err
	}
	p, err := validatePlateStr(plate)
	if err != nil {
		return nil, "", err
	}
	if err := validateCursor(cursor); err != nil {
		return nil, "", err
	}
	positions, _, err := s.reader.LastPositions(ctx, p, limit+1, cursor)
	if err != nil {
		return nil, "", fmt.Errorf("reader LastPositions failed: %w", err)
	}
	rounded := roundPositions(positions)
	if len(rounded) > limit {
		next := shared.EncodeCursor(rounded[limit-1].Plate, rounded[limit-1].ReceivedAt)
		return rounded[:limit], next, nil
	}
	return rounded, "", nil
}

func (s *QueryService) History(ctx context.Context, plate string, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
	if err := validateLimit(limit); err != nil {
		return nil, "", err
	}
	p, err := shared.ParsePlate(plate)
	if err != nil {
		return nil, "", fmt.Errorf("plate %q invalid: %w", plate, errors.Join(shared.ErrInvalidPlate, shared.ErrValidation, err))
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, "", fmt.Errorf("from %v after to %v: %w", *from, *to, shared.ErrValidation)
	}
	if err := validateCursor(cursor); err != nil {
		return nil, "", err
	}
	points, _, err := s.reader.History(ctx, p, from, to, limit+1, cursor)
	if err != nil {
		return nil, "", fmt.Errorf("reader History failed: %w", err)
	}
	rounded := roundPositions(points)
	if len(rounded) > limit {
		next := shared.EncodeCursor(rounded[limit-1].Plate, rounded[limit-1].ReceivedAt)
		return rounded[:limit], next, nil
	}
	return rounded, "", nil
}
