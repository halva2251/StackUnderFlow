package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// trustedProxyChecker determines if the direct TCP connection is from a trusted proxy.
// This prevents X-Real-IP / X-Forwarded-For spoofing when the server is exposed directly.
type trustedProxyChecker func(directRemoteAddr string) bool

// visitor stores the rate limiter and the last time it was seen.
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter provides IP-based rate limiting with a cleanup mechanism.
type RateLimiter struct {
	visitors          map[string]*visitor
	mu                sync.RWMutex
	r                 rate.Limit
	burst             int
	trustedProxyCheck trustedProxyChecker
	stop              chan struct{}
}

// NewRateLimiter creates a new rate limiter with the specified rate and burst.
// It also starts a background cleanup routine to remove old visitors every minute.
// trustedProxies is a list of CIDR notations (e.g. "127.0.0.0/8", "172.20.0.0/16") that are
// trusted reverse proxies. Only requests from trusted proxies will have their
// X-Real-IP / X-Forwarded-For headers used for rate limiting.
// The returned RateLimiter must be stopped via Stop() when no longer needed.
func NewRateLimiter(r rate.Limit, burst int, trustedProxies []string) *RateLimiter {
	rl := &RateLimiter{
		visitors:          make(map[string]*visitor),
		r:                 r,
		burst:             burst,
		trustedProxyCheck: makeProxyChecker(trustedProxies),
		stop:              make(chan struct{}),
	}

	go rl.cleanup()

	return rl
}

// Stop signals the cleanup goroutine to exit and waits for it to finish.
// After Stop is called, the RateLimiter should not be used.
func (rl *RateLimiter) Stop() {
	close(rl.stop)
}

// makeProxyChecker returns a function that checks if the direct TCP connection
// originates from a trusted proxy by matching against the given CIDR ranges.
func makeProxyChecker(cidrs []string) trustedProxyChecker {
	var nets []*net.IPNet
	for _, cidr := range cidrs {
		if _, ipNet, err := net.ParseCIDR(cidr); err == nil {
			nets = append(nets, ipNet)
		}
	}

	return func(directRemoteAddr string) bool {
		ipStr, _, err := net.SplitHostPort(directRemoteAddr)
		if err != nil {
			ipStr = directRemoteAddr
		}
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return false
		}
		for _, n := range nets {
			if n.Contains(ip) {
				return true
			}
		}
		return false
	}
}

func (rl *RateLimiter) cleanup() {
	for {
		select {
		case <-rl.stop:
			return
		case <-time.After(time.Minute):
		}

		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if v, exists := rl.visitors[ip]; exists {
		v.lastSeen = time.Now()
		return v.limiter
	}

	limiter := rate.NewLimiter(rl.r, rl.burst)
	rl.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
	return limiter
}

// getRealIP extracts the client IP for rate limiting. It only trusts X-Real-IP
// and X-Forwarded-For headers when the direct TCP connection is from a trusted proxy.
// This prevents IP spoofing when the server is exposed directly to the internet.
func (rl *RateLimiter) getRealIP(r *http.Request) string {
	// Get the direct TCP connection address (set by the kernel, not spoofable)
	directAddr := r.RemoteAddr

	// If the connection is from a trusted proxy, extract the real client IP
	if rl.trustedProxyCheck(directAddr) {
		// X-Real-IP is set by Nginx to the actual client IP
		if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			return strings.TrimSpace(realIP)
		}
		// Fallback to X-Forwarded-For (leftmost IP is the client)
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			if idx := strings.Index(fwd, ","); idx != -1 {
				return strings.TrimSpace(fwd[:idx])
			}
			return strings.TrimSpace(fwd)
		}
	}

	// For direct connections or untrusted proxies, use the direct TCP address.
	// This is the kernel-assigned peer address and cannot be spoofed.
	ip, _, err := net.SplitHostPort(directAddr)
	if err != nil {
		return directAddr
	}
	return ip
}

// Limit returns a middleware that rate limits requests by IP address.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rl.getRealIP(r)

		limiter := rl.getLimiter(ip)

		if !limiter.Allow() {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}
