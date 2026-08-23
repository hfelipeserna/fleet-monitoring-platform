package breaker

import (
	"errors"
	"fmt"
	"time"

	"github.com/sony/gobreaker"
)

type Breaker struct {
	cb *gobreaker.CircuitBreaker
}

func NewBreaker() *Breaker {
	return NewBreakerWithSettings("telemetry-publish", 10, 30*time.Second, 0.5, 10)
}

func NewBreakerWithSettings(name string, maxRequests uint32, timeout time.Duration, failureRatio float64, requestsThreshold uint32) *Breaker {
	if name == "" {
		name = "telemetry-publish"
	}
	if maxRequests == 0 {
		maxRequests = 10
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if failureRatio <= 0 || failureRatio > 1 {
		failureRatio = 0.5
	}
	if requestsThreshold == 0 {
		requestsThreshold = 10
	}
	st := gobreaker.Settings{
		Name:        name,
		MaxRequests: maxRequests,
		Interval:    30 * time.Second,
		Timeout:     timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < requestsThreshold {
				return false
			}
			failureRatioCalc := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatioCalc >= failureRatio
		},
	}
	cb := gobreaker.NewCircuitBreaker(st)
	return &Breaker{cb: cb}
}

func (b *Breaker) State() string {
	switch b.cb.State() {
	case gobreaker.StateOpen:
		return "open"
	case gobreaker.StateHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

func (b *Breaker) IsOpen() bool {
	return b.cb.State() == gobreaker.StateOpen
}

func (b *Breaker) Allow() error {
	if b.cb.State() == gobreaker.StateOpen {
		return fmt.Errorf("circuit breaker open: %w", gobreaker.ErrOpenState)
	}
	return nil
}

func (b *Breaker) RecordSuccess() {
	_, _ = b.cb.Execute(func() (any, error) { return nil, nil })
}

func (b *Breaker) RecordFailure() {
	_, _ = b.cb.Execute(func() (any, error) { return nil, errors.New("failure") })
}

func (b *Breaker) TripOnError(err error) {
	if err != nil {
		b.RecordFailure()
		return
	}
	b.RecordSuccess()
}
