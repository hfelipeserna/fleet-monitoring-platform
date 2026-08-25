package breaker

import (
	"log/slog"
	"time"

	"github.com/sony/gobreaker"
)

const (
	DefaultTimeout       = 30 * time.Second
	DefaultInterval      = 30 * time.Second
	MinRequests          = 5
	ConsecutiveThreshold = 3
	FailureRatio         = 0.5
	breakerName          = "gemini"
	maxRequests          = 1
)

func NewSettings(d time.Duration) gobreaker.Settings {
	return gobreaker.Settings{
		Name:        breakerName,
		Interval:    d,
		Timeout:     d,
		MaxRequests: maxRequests,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			if c.Requests < MinRequests {
				return false
			}
			return c.ConsecutiveFailures >= ConsecutiveThreshold || float64(c.TotalFailures)/float64(c.Requests) >= FailureRatio
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			slog.Info("breaker state change", "breaker", name, "from", from.String(), "to", to.String())
		},
	}
}

func newSettings(d time.Duration) gobreaker.Settings {
	return NewSettings(d)
}

func NewAssistantBreaker() *AssistantBreaker {
	return NewAssistantBreakerWithTimeout(DefaultTimeout)
}

func NewAssistantBreakerWithTimeout(d time.Duration) *AssistantBreaker {
	s := NewSettings(d)
	cb := gobreaker.NewCircuitBreaker(s)
	return &AssistantBreaker{cb: cb, timeout: d, interval: d}
}

type AssistantBreaker struct {
	cb       *gobreaker.CircuitBreaker
	timeout  time.Duration
	interval time.Duration
}

func (b *AssistantBreaker) State() string {
	if b == nil || b.cb == nil {
		return gobreaker.StateClosed.String()
	}
	return b.cb.State().String()
}

func (b *AssistantBreaker) IsOpen() bool {
	if b == nil || b.cb == nil {
		return false
	}
	return b.cb.State() == gobreaker.StateOpen
}

func (b *AssistantBreaker) Execute(req func() (any, error)) (any, error) {
	if b == nil || b.cb == nil {
		return req()
	}
	return b.cb.Execute(req)
}

func (b *AssistantBreaker) Timeout() time.Duration {
	if b == nil {
		return 0
	}
	return b.timeout
}

func (b *AssistantBreaker) Interval() time.Duration {
	if b == nil {
		return 0
	}
	return b.interval
}

func (b *AssistantBreaker) Breaker() *gobreaker.CircuitBreaker {
	if b == nil {
		return nil
	}
	return b.cb
}
