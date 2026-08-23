package rate

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	onlineRate       = 12.0 / 60.0
	onlineBurst      = 20
	batchReqInterval = 5 * time.Second
	batchRate        = 500.0 / 30.0
	batchBurst       = 500
	cleanupInterval  = 10 * time.Minute
	entryTTL         = 30 * time.Minute
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
	stopCh  chan struct{}
	once    sync.Once
}

func NewLimiter() *Limiter {
	return NewLimiterWithContext(context.Background())
}

func NewLimiterWithContext(ctx context.Context) *Limiter {
	l := &Limiter{
		entries: make(map[string]*entry),
		stopCh:  make(chan struct{}),
	}
	go l.cleanupLoop(ctx)
	return l
}

func (l *Limiter) Stop() {
	l.once.Do(func() { close(l.stopCh) })
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
	// O(1) token consumption using AllowN instead of loop O(n).
	return e.batchCount.AllowN(time.Now(), n)
}

func (l *Limiter) getOrCreate(plate string) *entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok := l.entries[plate]; ok {
		e.lastSeen = time.Now()
		return e
	}
	e := &entry{
		online:     rate.NewLimiter(rate.Limit(onlineRate), onlineBurst),
		batchReq:   rate.NewLimiter(rate.Every(batchReqInterval), 1),
		batchCount: rate.NewLimiter(rate.Limit(batchRate), batchBurst),
		lastSeen:   time.Now(),
	}
	l.entries[plate] = e
	return e
}

func (l *Limiter) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.mu.Lock()
			now := time.Now()
			for k, e := range l.entries {
				if now.Sub(e.lastSeen) > entryTTL {
					delete(l.entries, k)
				}
			}
			l.mu.Unlock()
		}
	}
}
