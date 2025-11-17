package auth

import (
	"net/http"

	"github.com/gorilla/sessions"
)

var sessionStore *sessions.CookieStore

// InitSessionStore initializes the session store with a secret key
func InitSessionStore(secretKey string) {
	sessionStore = sessions.NewCookieStore([]byte(secretKey))
	sessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   false, // Set to true if using HTTPS
		SameSite: http.SameSiteLaxMode,
	}
}

// SessionMiddleware creates a middleware that checks for valid session or Basic Auth
// Provides backward compatibility with Basic Auth headers
func SessionMiddleware(config Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If auth is not enabled, skip authentication
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Check for valid session cookie
			session, _ := sessionStore.Get(r, "census-session")
			if auth, ok := session.Values["authenticated"].(bool); ok && auth {
				next.ServeHTTP(w, r)
				return
			}

			// Fallback: check Basic Auth for backward compatibility
			username, password, ok := r.BasicAuth()
			if ok && validateCredentials(username, password, config.Username, config.Password) {
				next.ServeHTTP(w, r)
				return
			}

			// Unauthorized - return JSON for API calls, let browser handle redirects
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Unauthorized"}`))
		})
	}
}

// CreateSession creates a new authenticated session
func CreateSession(w http.ResponseWriter, r *http.Request) error {
	session, _ := sessionStore.Get(r, "census-session")
	session.Values["authenticated"] = true
	return session.Save(r, w)
}

// DestroySession destroys the current session
func DestroySession(w http.ResponseWriter, r *http.Request) error {
	session, _ := sessionStore.Get(r, "census-session")
	session.Values["authenticated"] = false
	session.Options.MaxAge = -1 // Delete cookie
	return session.Save(r, w)
}

// GetSession retrieves the current session
func GetSession(r *http.Request) (*sessions.Session, error) {
	return sessionStore.Get(r, "census-session")
}
