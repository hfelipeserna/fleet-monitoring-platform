package application

import (
	"context"
	"errors"
	"fmt"

	"fleetmonitoring/backend/internal/assistant/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

var ErrValidation = shared.ErrValidation

type StoppedService struct {
	querier FleetQuerier
}

func NewStoppedService(q FleetQuerier) *StoppedService {
	return &StoppedService{querier: q}
}

func (s *StoppedService) FindStopped(ctx context.Context, minMinutes int, zoneID *string, limit int) ([]domain.StoppedVehicle, error) {
	clamped, err := shared.ValidateStoppedParams(minMinutes, limit, zoneID)
	if err != nil {
		return nil, err
	}
	limit = clamped
	res, err := s.querier.FindStoppedInZones(ctx, minMinutes, zoneID, limit)
	if err != nil {
		return nil, fmt.Errorf("find stopped failed: %w", err)
	}
	for i := range res {
		if vErr := res[i].Validate(); vErr != nil {
			return nil, fmt.Errorf("stopped vehicle validation failed: %w", errors.Join(ErrValidation, vErr))
		}
	}
	if res == nil {
		res = []domain.StoppedVehicle{}
	}
	return res, nil
}

func (s *StoppedService) GetFleetSummary(ctx context.Context) (FleetSummary, error) {
	v, err := s.querier.GetFleetSummary(ctx)
	if err != nil {
		return FleetSummary{}, fmt.Errorf("get fleet summary failed: %w", err)
	}
	return v, nil
}

func (s *StoppedService) GetVehicleStatus(ctx context.Context, plate shared.Plate) (VehicleStatus, error) {
	v, err := s.querier.GetVehicleStatus(ctx, plate)
	if err != nil {
		return VehicleStatus{}, fmt.Errorf("get vehicle status failed: %w", err)
	}
	return v, nil
}

func (s *StoppedService) GetActiveAlerts(ctx context.Context, limit int) ([]Alert, error) {
	v, err := s.querier.GetActiveAlerts(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("get active alerts failed: %w", err)
	}
	return v, nil
}
