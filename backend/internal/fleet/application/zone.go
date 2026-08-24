package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
	"fleetmonitoring/backend/internal/shared/idgen"
)

type ZoneRepository interface {
	List(ctx context.Context) ([]fleet.Zone, error)
	Create(ctx context.Context, z fleet.Zone) (fleet.Zone, error)
	Update(ctx context.Context, id string, z fleet.Zone) (fleet.Zone, error)
	Delete(ctx context.Context, id string) error
}

type ZoneService struct {
	repo ZoneRepository
}

func NewZoneService(repo ZoneRepository) *ZoneService {
	return &ZoneService{repo: repo}
}

func (s *ZoneService) Create(ctx context.Context, name string, coords [][]float64) (fleet.Zone, error) {
	id := idgen.GenerateUUID()
	z := fleet.Zone{ID: id, Name: name, Coordinates: coords}
	if err := z.Validate(); err != nil {
		return fleet.Zone{}, fmt.Errorf("zone validation failed: %w", errors.Join(shared.ErrValidation, err))
	}
	created, err := s.repo.Create(ctx, z)
	if err != nil {
		return fleet.Zone{}, fmt.Errorf("create zone failed: %w", err)
	}
	return created, nil
}

// test-only: timeout wrapper for tests without ctx; production uses Create with context and breaker
func (s *ZoneService) CreateZone(name string, coords [][]float64) (fleet.Zone, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.Create(ctx, name, coords)
}

func (s *ZoneService) List(ctx context.Context) ([]fleet.Zone, error) {
	zones, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list zones failed: %w", err)
	}
	return zones, nil
}

// test-only: timeout wrapper for tests without ctx; production uses List with context and breaker
func (s *ZoneService) ListZones() ([]fleet.Zone, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.List(ctx)
}

func (s *ZoneService) Update(ctx context.Context, id string, name string, coords [][]float64) (fleet.Zone, error) {
	z := fleet.Zone{ID: id, Name: name, Coordinates: coords}
	if err := z.Validate(); err != nil {
		return fleet.Zone{}, fmt.Errorf("zone validation failed: %w", errors.Join(shared.ErrValidation, err))
	}
	updated, err := s.repo.Update(ctx, id, z)
	if err != nil {
		if errors.Is(err, fleet.ErrNotFound) {
			return fleet.Zone{}, fmt.Errorf("zone %s not found: %w", id, err)
		}
		return fleet.Zone{}, fmt.Errorf("update zone %s failed: %w", id, err)
	}
	return updated, nil
}

// test-only: timeout wrapper for tests without ctx; production uses Update with context and breaker
func (s *ZoneService) UpdateZone(id string, name string, coords [][]float64) (fleet.Zone, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.Update(ctx, id, name, coords)
}

func (s *ZoneService) Delete(ctx context.Context, id string) error {
	if err := fleet.ValidateUUID(id); err != nil {
		return fmt.Errorf("zone validation failed: %w", errors.Join(shared.ErrValidation, err))
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, fleet.ErrNotFound) {
			return fmt.Errorf("zone %s not found: %w", id, err)
		}
		return fmt.Errorf("delete zone %s failed: %w", id, err)
	}
	return nil
}

// test-only: timeout wrapper for tests without ctx; production uses Delete with context and breaker
func (s *ZoneService) DeleteZone(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.Delete(ctx, id)
}
