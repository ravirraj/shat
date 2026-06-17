package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu       sync.Mutex
	rate     int
	burst    int
	tokens   int
	lastTime time.Time
}

func New(rate, burst int) *Limiter {
	return &Limiter{
		rate:     rate,
		burst:    burst,
		tokens:   burst,
		lastTime: time.Now(),
	}
}

func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.lastTime)
	l.lastTime = now

	l.tokens += int(elapsed.Seconds()) * l.rate
	if l.tokens > l.burst {
		l.tokens = l.burst
	}

	if l.tokens <= 0 {
		return false
	}

	l.tokens--
	return true
}

type Manager struct {
	mu       sync.RWMutex
	limiters map[string]*Limiter
	rate     int
	burst    int
}

func NewManager(rate, burst int) *Manager {
	return &Manager{
		limiters: make(map[string]*Limiter),
		rate:     rate,
		burst:    burst,
	}
}

func (m *Manager) GetLimiter(key string) *Limiter {
	m.mu.RLock()
	if l, ok := m.limiters[key]; ok {
		m.mu.RUnlock()
		return l
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if l, ok := m.limiters[key]; ok {
		return l
	}

	l := New(m.rate, m.burst)
	m.limiters[key] = l
	return l
}

func (m *Manager) Allow(key string) bool {
	return m.GetLimiter(key).Allow()
}
