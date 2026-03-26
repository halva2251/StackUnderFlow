package middleware

import "net/http"

// SecurityHeaders adds essential security headers to all responses.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking
			w.Header().Set("X-Frame-Options", "DENY")

			// X-XSS-Protection is deprecated; set to 0 to avoid IE-specific bugs
			w.Header().Set("X-XSS-Protection", "0")

			// Enforce HTTPS for 1 year, include subdomains
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

			// Control referrer information leaked to third parties
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Restrict default fetch directives and forbid framing by any origin
			w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

			// Disable access to sensitive browser features
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

			next.ServeHTTP(w, r)
		})
	}
}
