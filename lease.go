package uniqidx

import (
	"sync"
	"time"
)

// DeadlineLeaseGuard is a small adapter for coordinator clients: renew it only
// after an acknowledged keepalive, and generation stops at the deadline.
type DeadlineLeaseGuard struct {
	mu       sync.RWMutex
	deadline time.Time
}

func NewDeadlineLeaseGuard(deadline time.Time) *DeadlineLeaseGuard {
	return &DeadlineLeaseGuard{deadline: deadline}
}
func (g *DeadlineLeaseGuard) Renew(deadline time.Time) {
	g.mu.Lock()
	g.deadline = deadline
	g.mu.Unlock()
}
func (g *DeadlineLeaseGuard) Valid(now time.Time) bool {
	g.mu.RLock()
	d := g.deadline
	g.mu.RUnlock()
	return now.Before(d)
}
