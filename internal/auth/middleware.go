package auth

import (
	"crypto/subtle"
	"net/http"
)

// Config holds authentication configuration
type Config struct {
	Enabled  bool
	Username string
	Password string
}

// BasicAuthMiddleware creates a middleware that enforces HTTP Basic Authentication
// If auth is not enabled, it passes through without authentication
func BasicAuthMiddleware(config Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If auth is not enabled, skip authentication
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Get credentials from request
			username, password, ok := r.BasicAuth()

			// Check if credentials are provided and valid
			if !ok || !validateCredentials(username, password, config.Username, config.Password) {
				// Send authentication challenge
				w.Header().Set("WWW-Authenticate", `Basic realm="Container Census", charset="UTF-8"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Authentication successful, proceed to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// validateCredentials performs constant-time comparison of credentials
// to prevent timing attacks
func validateCredentials(providedUser, providedPass, validUser, validPass string) bool {
	// Use constant-time comparison to prevent timing attacks
	usernameMatch := subtle.ConstantTimeCompare([]byte(providedUser), []byte(validUser)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(providedPass), []byte(validPass)) == 1

	return usernameMatch && passwordMatch
}
