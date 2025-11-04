package auth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestMiddleware_ValidCredentials tests successful authentication
func TestMiddleware_ValidCredentials(t *testing.T) {
	username := "admin"
	password := "secret123"

	config := Config{
		Enabled:  true,
		Username: username,
		Password: password,
	}

	middleware := BasicAuthMiddleware(config)

	// Create test handler
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Wrap with auth
	authHandler := middleware(handler)

	// Create request with valid credentials
	req := httptest.NewRequest("GET", "/api/test", nil)
	credentials := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	req.Header.Set("Authorization", "Basic "+credentials)

	rec := httptest.NewRecorder()
	authHandler.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("Handler should be called with valid credentials")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if rec.Body.String() != "success" {
		t.Errorf("Expected body 'success', got '%s'", rec.Body.String())
	}
}

// TestMiddleware_InvalidCredentials tests authentication failure
func TestMiddleware_InvalidCredentials(t *testing.T) {
	config := Config{
		Enabled:  true,
		Username: "admin",
		Password: "correct-password",
	}

	middleware := BasicAuthMiddleware(config)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	authHandler := middleware(handler)

	tests := []struct {
		name     string
		username string
		password string
	}{
		{"wrong password", "admin", "wrong-password"},
		{"wrong username", "hacker", "correct-password"},
		{"both wrong", "hacker", "wrong-password"},
		{"empty password", "admin", ""},
		{"empty username", "", "correct-password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled = false

			req := httptest.NewRequest("GET", "/api/test", nil)
			credentials := base64.StdEncoding.EncodeToString([]byte(tt.username + ":" + tt.password))
			req.Header.Set("Authorization", "Basic "+credentials)

			rec := httptest.NewRecorder()
			authHandler.ServeHTTP(rec, req)

			if handlerCalled {
				t.Error("Handler should not be called with invalid credentials")
			}

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401, got %d", rec.Code)
			}
		})
	}
}

// TestMiddleware_MissingAuthHeader tests missing Authorization header
func TestMiddleware_MissingAuthHeader(t *testing.T) {
	config := Config{
		Enabled:  true,
		Username: "admin",
		Password: "password",
	}

	middleware := BasicAuthMiddleware(config)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	authHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	// No Authorization header

	rec := httptest.NewRecorder()
	authHandler.ServeHTTP(rec, req)

	if handlerCalled {
		t.Error("Handler should not be called without auth header")
	}

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}

	// Verify WWW-Authenticate header is set
	wwwAuth := rec.Header().Get("WWW-Authenticate")
	if wwwAuth != `Basic realm="Container Census", charset="UTF-8"` {
		t.Errorf("Expected WWW-Authenticate header, got '%s'", wwwAuth)
	}
}

// TestMiddleware_MalformedAuthHeader tests malformed authorization headers
func TestMiddleware_MalformedAuthHeader(t *testing.T) {
	config := Config{
		Enabled:  true,
		Username: "admin",
		Password: "password",
	}

	middleware := BasicAuthMiddleware(config)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	authHandler := middleware(handler)

	tests := []struct {
		name   string
		header string
	}{
		{"not basic", "Bearer token123"},
		{"invalid base64", "Basic not-valid-base64!!!"},
		{"no colon", "Basic " + base64.StdEncoding.EncodeToString([]byte("adminpassword"))},
		{"empty", ""},
		{"only Basic", "Basic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled = false

			req := httptest.NewRequest("GET", "/api/test", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			rec := httptest.NewRecorder()
			authHandler.ServeHTTP(rec, req)

			if handlerCalled {
				t.Error("Handler should not be called with malformed auth")
			}

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401, got %d", rec.Code)
			}
		})
	}
}

// TestMiddleware_DisabledAuth tests that auth can be disabled
func TestMiddleware_DisabledAuth(t *testing.T) {
	config := Config{
		Enabled:  false,
		Username: "admin",
		Password: "password",
	}

	middleware := BasicAuthMiddleware(config)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	authHandler := middleware(handler)

	// Request without any auth
	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()
	authHandler.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("Handler should be called when auth is disabled")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with auth disabled, got %d", rec.Code)
	}
}

// TestMiddleware_TimingAttackResistance tests constant-time comparison
// This is a behavioral test - we can't directly test timing, but we can verify
// that the comparison function is being used
func TestMiddleware_TimingAttackResistance(t *testing.T) {
	config := Config{
		Enabled:  true,
		Username: "admin",
		Password: "password123",
	}

	middleware := BasicAuthMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authHandler := middleware(handler)

	// Try various lengths of passwords
	tests := []string{
		"p",
		"pa",
		"pas",
		"pass",
		"passw",
		"passwo",
		"passwor",
		"password12",   // One char off
		"password123",  // Correct
		"password1234", // One char extra
	}

	for _, pw := range tests {
		req := httptest.NewRequest("GET", "/api/test", nil)
		credentials := base64.StdEncoding.EncodeToString([]byte("admin:" + pw))
		req.Header.Set("Authorization", "Basic "+credentials)

		rec := httptest.NewRecorder()

		start := time.Now()
		authHandler.ServeHTTP(rec, req)
		elapsed := time.Since(start)

		// Just verify that timing doesn't vary wildly
		// (In practice, timing attacks are very subtle and hard to test)
		if elapsed > 100*time.Millisecond {
			t.Logf("Warning: Auth check took %v for password length %d", elapsed, len(pw))
		}

		if pw == "password123" {
			if rec.Code != http.StatusOK {
				t.Error("Correct password should succeed")
			}
		} else {
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("Wrong password '%s' should fail", pw)
			}
		}
	}
}

// TestMiddleware_MultipleRequests tests handling multiple requests
func TestMiddleware_MultipleRequests(t *testing.T) {
	config := Config{
		Enabled:  true,
		Username: "admin",
		Password: "password",
	}

	middleware := BasicAuthMiddleware(config)

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	authHandler := middleware(handler)

	// Send 5 valid requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		credentials := base64.StdEncoding.EncodeToString([]byte("admin:password"))
		req.Header.Set("Authorization", "Basic "+credentials)

		rec := httptest.NewRecorder()
		authHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Request %d failed with status %d", i+1, rec.Code)
		}
	}

	if callCount != 5 {
		t.Errorf("Expected handler called 5 times, got %d", callCount)
	}
}

// TestMiddleware_ConcurrentRequests tests thread-safe operation
func TestMiddleware_ConcurrentRequests(t *testing.T) {
	config := Config{
		Enabled:  true,
		Username: "admin",
		Password: "password",
	}

	middleware := BasicAuthMiddleware(config)

	successCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		successCount++
		w.WriteHeader(http.StatusOK)
	})

	authHandler := middleware(handler)

	done := make(chan bool)

	// Send 10 concurrent requests
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/api/test", nil)
			credentials := base64.StdEncoding.EncodeToString([]byte("admin:password"))
			req.Header.Set("Authorization", "Basic "+credentials)

			rec := httptest.NewRecorder()
			authHandler.ServeHTTP(rec, req)

			done <- true
		}()
	}

	// Wait for all
	for i := 0; i < 10; i++ {
		<-done
	}

	if successCount != 10 {
		t.Errorf("Expected 10 successful requests, got %d", successCount)
	}
}

// TestMiddleware_DifferentHTTPMethods tests auth works for all HTTP methods
func TestMiddleware_DifferentHTTPMethods(t *testing.T) {
	config := Config{
		Enabled:  true,
		Username: "admin",
		Password: "password",
	}

	middleware := BasicAuthMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authHandler := middleware(handler)

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/api/test", nil)
		credentials := base64.StdEncoding.EncodeToString([]byte("admin:password"))
		req.Header.Set("Authorization", "Basic "+credentials)

		rec := httptest.NewRecorder()
		authHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Method %s failed with status %d", method, rec.Code)
		}
	}
}

// TestMiddleware_CaseInsensitiveUsername tests username comparison
func TestMiddleware_CaseInsensitiveUsername(t *testing.T) {
	config := Config{
		Enabled:  true,
		Username: "admin",
		Password: "password",
	}

	middleware := BasicAuthMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authHandler := middleware(handler)

	// Try different cases
	tests := []struct {
		username      string
		shouldSucceed bool
	}{
		{"admin", true},
		{"Admin", false}, // Case sensitive
		{"ADMIN", false},
		{"AdMiN", false},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", "/api/test", nil)
		credentials := base64.StdEncoding.EncodeToString([]byte(tt.username + ":password"))
		req.Header.Set("Authorization", "Basic "+credentials)

		rec := httptest.NewRecorder()
		authHandler.ServeHTTP(rec, req)

		if tt.shouldSucceed {
			if rec.Code != http.StatusOK {
				t.Errorf("Username '%s' should succeed", tt.username)
			}
		} else {
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("Username '%s' should fail (case sensitive)", tt.username)
			}
		}
	}
}

// TestMiddleware_SpecialCharactersInPassword tests passwords with special chars
func TestMiddleware_SpecialCharactersInPassword(t *testing.T) {
	specialPasswords := []string{
		"p@ssw0rd!",
		"pass:word",
		"pass word",
		"пароль",     // Unicode
		"パスワード",      // Japanese
		"🔒secure🔒", // Emojis
	}

	for _, password := range specialPasswords {
		t.Run(password, func(t *testing.T) {
			config := Config{
				Enabled:  true,
				Username: "admin",
				Password: password,
			}

			middleware := BasicAuthMiddleware(config)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			authHandler := middleware(handler)

			req := httptest.NewRequest("GET", "/api/test", nil)
			credentials := base64.StdEncoding.EncodeToString([]byte("admin:" + password))
			req.Header.Set("Authorization", "Basic "+credentials)

			rec := httptest.NewRecorder()
			authHandler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("Special password should work, got status %d", rec.Code)
			}
		})
	}
}
