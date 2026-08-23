package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fleetmonitoring/backend/internal/shared/domain"
	sharedidgen "fleetmonitoring/backend/internal/shared/idgen"
	telemetry "fleetmonitoring/backend/internal/telemetry/domain"
)

var (
	ErrValidation   = errors.New("validation")
	ErrRateLimited  = errors.New("rate_limited")
	ErrBackpressure = errors.New("backpressure")
)

const (
	MaxBatchSize         = 500
	highWatermarkPercent = 80
	maxFutureSkew        = 5 * time.Minute
)

type Publisher interface {
	Publish(ctx context.Context, evt telemetry.TelemetryEvent) error
	PublishBatch(ctx context.Context, evts []telemetry.TelemetryEvent) error
}

type RateLimiter interface {
	Allow(plate string) bool
	AllowBatch(plate string, n int) bool
}

type Breaker interface {
	State() string
	IsOpen() bool
	Allow() error
}

type JetStreamInfo interface {
	Bytes() (uint64, uint64)
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}

type RawValidator interface {
	Validate(RawEvent, time.Time) error
}

type RawEvent struct {
	Plate         string
	Speed         *int
	Lat           *float64
	Lon           *float64
	ClientEventID string
	OccurredAt    *time.Time
}

type IngestService struct {
	pub       Publisher
	limiter   RateLimiter
	breaker   Breaker
	js        JetStreamInfo
	clock     Clock
	idGen     IDGenerator
	validator RawValidator
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type funcClock struct {
	fn func() time.Time
}

func (f funcClock) Now() time.Time { return f.fn() }

type defaultIDGenerator struct{}

func (defaultIDGenerator) NewID() string { return sharedidgen.GenerateUUID() }

type DefaultRawValidator struct{}

func (DefaultRawValidator) Validate(raw RawEvent, now time.Time) error {
	if err := validatePlate(raw.Plate); err != nil {
		return err
	}
	if err := validateSpeed(raw.Speed); err != nil {
		return err
	}
	if err := validateCoords(raw.Lat, raw.Lon); err != nil {
		return err
	}
	if err := validateClientID(raw.ClientEventID); err != nil {
		return err
	}
	if err := validateOccurredAt(raw.OccurredAt, now); err != nil {
		return err
	}
	return nil
}

func NewIngestService(pub Publisher, limiter RateLimiter, breaker Breaker, js JetStreamInfo, now func() time.Time) *IngestService {
	var clk Clock
	if now != nil {
		clk = funcClock{fn: now}
	} else {
		clk = systemClock{}
	}
	return NewIngestServiceWithDeps(pub, limiter, breaker, js, clk, &defaultIDGenerator{}, DefaultRawValidator{})
}

func NewIngestServiceWithDeps(pub Publisher, limiter RateLimiter, breaker Breaker, js JetStreamInfo, clock Clock, idGen IDGenerator, validator RawValidator) *IngestService {
	if clock == nil {
		clock = systemClock{}
	}
	if idGen == nil {
		idGen = &defaultIDGenerator{}
	}
	if validator == nil {
		validator = DefaultRawValidator{}
	}
	return &IngestService{pub: pub, limiter: limiter, breaker: breaker, js: js, clock: clock, idGen: idGen, validator: validator}
}

func (s *IngestService) IngestSingle(ctx context.Context, raw RawEvent) (telemetry.TelemetryEvent, error) {
	if err := s.checkContext(ctx); err != nil {
		return telemetry.TelemetryEvent{}, err
	}
	now := s.clock.Now()
	evt, err := s.processOne(raw, now)
	if err != nil {
		return telemetry.TelemetryEvent{}, err
	}
	if err := s.checkRateSingle(evt.Plate); err != nil {
		return telemetry.TelemetryEvent{}, err
	}
	if err := s.checkBackpressure(); err != nil {
		return telemetry.TelemetryEvent{}, err
	}
	if err := s.publishSingle(ctx, evt); err != nil {
		return telemetry.TelemetryEvent{}, err
	}
	return evt, nil
}

func (s *IngestService) IngestBatch(ctx context.Context, raws []RawEvent) ([]telemetry.TelemetryEvent, error) {
	if err := s.checkContext(ctx); err != nil {
		return nil, err
	}
	if err := s.validateBatchSize(raws); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	evts := make([]telemetry.TelemetryEvent, 0, len(raws))
	for i, raw := range raws {
		evt, err := s.processOne(raw, now)
		if err != nil {
			return nil, fmt.Errorf("batch item %d: %w", i, err)
		}
		evts = append(evts, evt)
	}
	if err := s.checkRateBatch(evts); err != nil {
		return nil, err
	}
	if err := s.checkBackpressure(); err != nil {
		return nil, err
	}
	if err := s.publishBatch(ctx, evts); err != nil {
		return nil, err
	}
	return evts, nil
}

func (s *IngestService) processOne(raw RawEvent, now time.Time) (telemetry.TelemetryEvent, error) {
	if err := s.validator.Validate(raw, now); err != nil {
		return telemetry.TelemetryEvent{}, fmt.Errorf("validation failed: %w", errors.Join(ErrValidation, err))
	}
	evt, err := s.enrich(raw, now)
	if err != nil {
		return telemetry.TelemetryEvent{}, fmt.Errorf("enrich failed: %w", errors.Join(ErrValidation, err))
	}
	if err := evt.ValidateAt(now); err != nil {
		return telemetry.TelemetryEvent{}, fmt.Errorf("domain validation failed: %w", errors.Join(ErrValidation, err))
	}
	return evt, nil
}

func (s *IngestService) enrich(raw RawEvent, now time.Time) (telemetry.TelemetryEvent, error) {
	plateNorm, err := domain.ParsePlate(raw.Plate)
	if err != nil {
		return telemetry.TelemetryEvent{}, err
	}
	cid := strings.TrimSpace(raw.ClientEventID)
	if cid == "" {
		cid = s.idGen.NewID()
	}
	return telemetry.TelemetryEvent{
		ClientEventID: cid,
		Plate:         string(plateNorm),
		Speed:         *raw.Speed,
		Lat:           raw.Lat,
		Lon:           raw.Lon,
		ReceivedAt:    now,
		OccurredAt:    raw.OccurredAt,
	}, nil
}

func (s *IngestService) checkContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", errors.Join(ErrBackpressure, err))
	}
	return nil
}

func (s *IngestService) validateBatchSize(raws []RawEvent) error {
	if len(raws) == 0 {
		return fmt.Errorf("empty batch: %w", ErrValidation)
	}
	if len(raws) > MaxBatchSize {
		return fmt.Errorf("batch size %d exceeds %d: %w", len(raws), MaxBatchSize, ErrValidation)
	}
	return nil
}

func (s *IngestService) checkRateSingle(plate string) error {
	if s.limiter == nil {
		return nil
	}
	if !s.limiter.Allow(plate) {
		return fmt.Errorf("rate limited for plate %s: %w", plate, ErrRateLimited)
	}
	return nil
}

func (s *IngestService) checkRateBatch(evts []telemetry.TelemetryEvent) error {
	if s.limiter == nil {
		return nil
	}
	distinct := make(map[string]int, len(evts))
	for _, e := range evts {
		distinct[e.Plate]++
	}
	for plate, cnt := range distinct {
		if !s.limiter.AllowBatch(plate, cnt) {
			return fmt.Errorf("batch rate limited for plate %s count %d: %w", plate, cnt, ErrRateLimited)
		}
	}
	return nil
}

func (s *IngestService) checkBackpressure() error {
	if err := s.checkBreaker(); err != nil {
		return err
	}
	if err := s.checkJetStream(); err != nil {
		return err
	}
	return nil
}

func (s *IngestService) checkBreaker() error {
	if s.breaker == nil {
		return nil
	}
	if s.breaker.IsOpen() {
		return fmt.Errorf("breaker open: %w", ErrBackpressure)
	}
	if err := s.breaker.Allow(); err != nil {
		return fmt.Errorf("breaker open: %w", errors.Join(ErrBackpressure, err))
	}
	return nil
}

func (s *IngestService) checkJetStream() error {
	if s.js == nil {
		return nil
	}
	used, max := s.js.Bytes()
	if max > 0 && used*100 >= max*highWatermarkPercent {
		return fmt.Errorf("jetstream bytes %d/%d >=%d%%: %w", used, max, highWatermarkPercent, ErrBackpressure)
	}
	return nil
}

func (s *IngestService) publishSingle(ctx context.Context, evt telemetry.TelemetryEvent) error {
	if s.pub == nil {
		return nil
	}
	if err := s.pub.Publish(ctx, evt); err != nil {
		return s.classifyPublishError("publish", err)
	}
	return nil
}

func (s *IngestService) publishBatch(ctx context.Context, evts []telemetry.TelemetryEvent) error {
	if s.pub == nil {
		return nil
	}
	if err := s.pub.PublishBatch(ctx, evts); err != nil {
		return s.classifyPublishError("publish batch", err)
	}
	return nil
}

func (s *IngestService) classifyPublishError(prefix string, err error) error {
	if isBackpressureError(err) {
		return fmt.Errorf("%s backpressure: %w", prefix, errors.Join(ErrBackpressure, err))
	}
	return fmt.Errorf("%s failed: %w", prefix, err)
}

func validatePlate(plate string) error {
	if strings.TrimSpace(plate) == "" {
		return fmt.Errorf("plate required")
	}
	if _, err := domain.ParsePlate(plate); err != nil {
		return fmt.Errorf("plate %q invalid: %w", plate, err)
	}
	return nil
}

func validateSpeed(speed *int) error {
	if speed == nil {
		return fmt.Errorf("speed required")
	}
	if *speed < 0 {
		return fmt.Errorf("speed %d negative", *speed)
	}
	return nil
}

func validateCoords(lat, lon *float64) error {
	if lat != nil && (*lat < -90 || *lat > 90) {
		return fmt.Errorf("lat %v out of range", *lat)
	}
	if lon != nil && (*lon < -180 || *lon > 180) {
		return fmt.Errorf("lon %v out of range", *lon)
	}
	return nil
}

func validateClientID(cid string) error {
	if cid == "" {
		return nil
	}
	if !isValidUUID(cid) {
		return fmt.Errorf("client_event_id %q invalid uuid", cid)
	}
	return nil
}

func validateOccurredAt(ts *time.Time, now time.Time) error {
	if ts == nil {
		return nil
	}
	if ts.After(now.Add(maxFutureSkew)) {
		return fmt.Errorf("occurred_at %v too far in future", *ts)
	}
	return nil
}

func isHexChar(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !isHexChar(c) {
			return false
		}
	}
	return true
}

func isBackpressureError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrBackpressure) {
		return true
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "backpressure") || strings.Contains(msg, "max_pending") || strings.Contains(msg, "max pending") || strings.Contains(msg, "pending exceeded")
}
