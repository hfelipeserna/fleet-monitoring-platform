package breaker

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/sony/gobreaker"
)

type Breaker struct {
	cb *gobreaker.CircuitBreaker
}

const (
	defaultBreakerRequests   = 10
	defaultBreakerTimeout    = 30 * time.Second
	defaultBreakerInterval   = 30 * time.Second
	defaultFailureRatio      = 0.5
	defaultRequestsThreshold = 10
)

func NewBreaker() *Breaker {
	return NewBreakerWithSettings("telemetry-publish", defaultBreakerRequests, defaultBreakerTimeout, defaultFailureRatio, defaultRequestsThreshold)
}

func NewBreakerWithSettings(name string, maxRequests uint32, timeout time.Duration, failureRatio float64, requestsThreshold uint32) *Breaker {
	if name == "" {
		name = "telemetry-publish"
	}
	if maxRequests == 0 {
		maxRequests = defaultBreakerRequests
	}
	if timeout == 0 {
		timeout = defaultBreakerTimeout
	}
	if failureRatio <= 0 || failureRatio > 1 {
		failureRatio = defaultFailureRatio
	}
	if requestsThreshold == 0 {
		requestsThreshold = defaultRequestsThreshold
	}
	st := gobreaker.Settings{
		Name:        name,
		MaxRequests: maxRequests,
		Interval:    defaultBreakerInterval,
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
	if b == nil || b.cb == nil {
		return "closed"
	}
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
	if b == nil || b.cb == nil {
		return false
	}
	return b.cb.State() == gobreaker.StateOpen
}

// IsOpenBreaker is a nil-safe helper that reports whether b is open.
// Centralized to avoid duplicated isBreakerOpen helpers across adapters/application.
func IsOpenBreaker(b *Breaker) bool {
	if b == nil {
		return false
	}
	return b.IsOpen()
}

func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

// IsOpen reports whether any Breaker implementation is open, nil-safe with fallback to State() string compare.
func IsOpen(b interface {
	State() string
	IsOpen() bool
}) bool {
	if isNil(b) {
		return false
	}
	return b.IsOpen()
}

func (b *Breaker) Allow() error {
	if b.IsOpen() {
		return fmt.Errorf("circuit breaker open: %w", gobreaker.ErrOpenState)
	}
	return nil
}

func (b *Breaker) RecordSuccess() {
	if b == nil || b.cb == nil {
		return
	}
	_, _ = b.cb.Execute(func() (any, error) { return nil, nil })
}

func (b *Breaker) RecordFailure() {
	if b == nil || b.cb == nil {
		return
	}
	_, _ = b.cb.Execute(func() (any, error) { return nil, errors.New("failure") })
}

func (b *Breaker) TripOnError(err error) {
	if err != nil {
		b.RecordFailure()
		return
	}
	b.RecordSuccess()
}
