package breaker

import "strings"

type Breaker interface {
	State() string
	IsOpen() bool
}

func IsOpen(b Breaker) bool {
	if b == nil {
		return false
	}
	if o, ok := any(b).(interface{ IsOpen() bool }); ok {
		return o.IsOpen()
	}
	if s, ok := any(b).(interface{ State() string }); ok {
		return strings.EqualFold(s.State(), "open")
	}
	return false
}
