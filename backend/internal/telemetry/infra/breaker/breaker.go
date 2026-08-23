package breaker

import (
	"time"

	"github.com/sony/gobreaker"
)

type Breaker struct {
	cb *gobreaker.CircuitBreaker
}

func NewBreaker() *Breaker {
	st := gobreaker.Settings{
		Name:        "telemetry-publish",
		MaxRequests: 1,
		Interval:    30 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < 10 {
				return false
			}
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatio >= 0.5
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

func (b *Breaker) Allow() error {
	_, err := b.cb.Execute(func() (any, error) { return nil, nil })
	return err
}

func (b *Breaker) RecordSuccess() {
	_, _ = b.cb.Execute(func() (any, error) { return nil, nil })
}

func (b *Breaker) RecordFailure() {
	_ = b.cb.State()
}
