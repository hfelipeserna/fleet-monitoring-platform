package metrics

import (
	"sync"
	"sync/atomic"
)

type CounterVec struct {
	total atomic.Int64
	m     sync.Map
}

func (c *CounterVec) Inc(label string) {
	c.Add(label, 1)
}

func (c *CounterVec) Add(label string, n int64) {
	c.total.Add(n)
	v, _ := c.m.LoadOrStore(label, &atomic.Int64{})
	ctr := v.(*atomic.Int64)
	ctr.Add(n)
}

func (c *CounterVec) Total() int64 {
	return c.total.Load()
}

func (c *CounterVec) Snapshot() map[string]int64 {
	snap := make(map[string]int64)
	c.m.Range(func(k, v any) bool {
		snap[k.(string)] = v.(*atomic.Int64).Load()
		return true
	})
	return snap
}

func (c *CounterVec) Range(fn func(label string, value int64) bool) {
	c.m.Range(func(k, v any) bool {
		return fn(k.(string), v.(*atomic.Int64).Load())
	})
}
