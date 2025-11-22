package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// simple in-memory rate limiter per remote IP.
type rateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	visitors map[string]*visitor
}

type visitor struct {
	count int
	reset time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:    limit,
		window:   window,
		visitors: make(map[string]*visitor),
	}
}

func (r *rateLimiter) allow(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	v, ok := r.visitors[host]
	if !ok || now.After(v.reset) {
		v = &visitor{count: 0, reset: now.Add(r.window)}
		r.visitors[host] = v
	}
	if v.count >= r.limit {
		return false
	}
	v.count++
	return true
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
