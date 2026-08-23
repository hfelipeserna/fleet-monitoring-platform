package rate

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type entry struct {
	online     *rate.Limiter
	batchReq   *rate.Limiter
	batchCount *rate.Limiter
	lastSeen   time.Time
}

type Limiter struct {
	mu      sync.Mutex
	entries map[string]*entry
}

func NewLimiter() *Limiter {
	l := &Limiter{entries: make(map[string]*entry)}
	go l.cleanupLoop()
	return l
}

func (l *Limiter) Allow(plate string) bool {
	e := l.getOrCreate(plate)
	return e.online.Allow()
}

func (l *Limiter) AllowBatch(plate string, n int) bool {
	e := l.getOrCreate(plate)
	if !e.batchReq.Allow() {
		return false
	}
	if n <= 0 {
		return true
	}
	for range n {
		if !e.batchCount.Allow() {
			return false
		}
	}
	return true
}

func (l *Limiter) getOrCreate(plate string) *entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok := l.entries[plate]; ok {
		e.lastSeen = time.Now()
		return e
	}
	e := &entry{
		online:     rate.NewLimiter(rate.Limit(12.0/60.0), 20),
		batchReq:   rate.NewLimiter(rate.Every(5*time.Second), 1),
		batchCount: rate.NewLimiter(rate.Limit(500.0/30.0), 500),
		lastSeen:   time.Now(),
	}
	l.entries[plate] = e
	return e
}

func (l *Limiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for k, e := range l.entries {
			if now.Sub(e.lastSeen) > 30*time.Minute {
				delete(l.entries, k)
			}
		}
		l.mu.Unlock()
	}
}
