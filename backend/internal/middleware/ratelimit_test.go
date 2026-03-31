package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/time/rate"
)

func TestMakeProxyChecker(t *testing.T) {
	check := makeProxyChecker([]string{"127.0.0.0/8", "172.20.0.0/16"})

	tests := []struct {
		name       string
		addr       string
		wantTrusted bool
	}{
		{"localhost IPv4", "127.0.0.1:12345", true},
		{"localhost other port", "127.0.0.1:8080", true},
		{"loopback range max", "127.255.255.255:9999", true},
		{"docker network", "172.20.0.2:54321", true},
		{"docker network boundary", "172.20.255.255:12345", true},
		{"outside docker network", "172.21.0.1:12345", false},
		{"public IP", "8.8.8.8:12345", false},
		{"private 10.x range", "10.0.0.1:12345", false},
		{"no port provided is still valid trusted IP", "127.0.0.1", true},
		{"invalid IP", "not-an-ip:12345", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := check(tt.addr)
			if got != tt.wantTrusted {
				t.Errorf("check(%q) = %v; want %v", tt.addr, got, tt.wantTrusted)
			}
		})
	}
}

func TestGetRealIP(t *testing.T) {
	trustedChecker := makeProxyChecker([]string{"127.0.0.0/8", "172.20.0.0/16"})
	untrustedChecker := makeProxyChecker([]string{})

	trustedLimiter := &RateLimiter{
		visitors:          make(map[string]*visitor),
		r:                 rate.Limit(10),
		burst:             100,
		trustedProxyCheck: trustedChecker,
	}
	untrustedLimiter := &RateLimiter{
		visitors:          make(map[string]*visitor),
		r:                 rate.Limit(10),
		burst:             100,
		trustedProxyCheck: untrustedChecker,
	}

	tests := []struct {
		name           string
		limiter        *RateLimiter
		remoteAddr     string
		xRealIP        string
		xForwardedFor  string
		wantIP         string
	}{
		{
			name:       "trusted proxy uses X-Real-IP",
			limiter:    trustedLimiter,
			remoteAddr: "172.20.0.2:12345",
			xRealIP:    "203.0.113.50",
			wantIP:     "203.0.113.50",
		},
		{
			name:       "trusted proxy uses X-Forwarded-For leftmost",
			limiter:    trustedLimiter,
			remoteAddr: "172.20.0.2:12345",
			xForwardedFor: "203.0.113.50, 70.41.3.18, 150.172.238.178",
			wantIP:     "203.0.113.50",
		},
		{
			name:       "trusted proxy X-Real-IP takes precedence",
			limiter:    trustedLimiter,
			remoteAddr: "172.20.0.2:12345",
			xRealIP:    "203.0.113.50",
			xForwardedFor: "198.51.100.178, 70.41.3.18",
			wantIP:     "203.0.113.50",
		},
		{
			name:       "trusted proxy falls back to direct TCP when no headers",
			limiter:    trustedLimiter,
			remoteAddr: "172.20.0.2:12345",
			wantIP:     "172.20.0.2",
		},
		{
			name:       "untrusted direct connection ignores X-Real-IP",
			limiter:    untrustedLimiter,
			remoteAddr: "203.0.113.50:12345",
			xRealIP:    "198.51.100.1",
			wantIP:     "203.0.113.50",
		},
		{
			name:       "untrusted direct connection ignores X-Forwarded-For",
			limiter:    untrustedLimiter,
			remoteAddr: "8.8.8.8:12345",
			xForwardedFor: "198.51.100.1, 203.0.113.1",
			wantIP:     "8.8.8.8",
		},
		{
			name:       "localhost trusted uses X-Real-IP",
			limiter:    trustedLimiter,
			remoteAddr: "127.0.0.1:54321",
			xRealIP:    "198.51.100.99",
			wantIP:     "198.51.100.99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}

			got := tt.limiter.getRealIP(req)
			if got != tt.wantIP {
				t.Errorf("getRealIP() = %q; want %q", got, tt.wantIP)
			}
		})
	}
}

func TestRateLimiterStop(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(10), 100, []string{"127.0.0.0/8"})
	rl.Stop() // Should not panic

	// After Stop, the limiter should still be safe to call getLimiter
	// (the map is not modified by cleanup after stop)
	limiter := rl.getLimiter("127.0.0.1")
	if limiter == nil {
		t.Error("expected non-nil limiter after Stop")
	}
}

func TestRateLimiterTrustedProxyIntegration(t *testing.T) {
	// Rate limiter with trusted proxies configured
	rl := NewRateLimiter(rate.Limit(10), 100, []string{"127.0.0.0/8", "172.20.0.0/16"})
	defer rl.Stop()

	// Simulate a request from a trusted proxy (nginx) with spoofed X-Real-IP
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.20.0.2:12345"
	req.Header.Set("X-Real-IP", "1.2.3.4")

	// Should use the X-Real-IP, not the spoofed one
	got := rl.getRealIP(req)
	if got != "1.2.3.4" {
		t.Errorf("expected X-Real-IP to be used from trusted proxy, got %q", got)
	}

	// Now simulate a direct connection (no trusted proxy) with spoofed X-Real-IP
	rl2 := NewRateLimiter(rate.Limit(10), 100, []string{})
	defer rl2.Stop()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "203.0.113.10:12345"
	req2.Header.Set("X-Real-IP", "1.2.3.4")

	got2 := rl2.getRealIP(req2)
	// Should use the direct connection IP, ignoring the spoofed header
	if got2 != "203.0.113.10" {
		t.Errorf("expected direct connection IP to be used for untrusted, got %q", got2)
	}
}
