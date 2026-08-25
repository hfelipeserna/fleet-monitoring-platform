package breaker

import (
	"reflect"
	"strings"
)

type Breaker interface {
	State() string
	IsOpen() bool
}

func IsOpen(b any) bool {
	if isNil(b) {
		return false
	}
	if o, ok := b.(interface{ IsOpen() bool }); ok {
		return o.IsOpen()
	}
	if s, ok := b.(interface{ State() string }); ok {
		return strings.EqualFold(s.State(), "open")
	}
	return false
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
