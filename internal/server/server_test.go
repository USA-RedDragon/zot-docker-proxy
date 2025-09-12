package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/USA-RedDragon/zot-docker-proxy/internal/config"
	"github.com/USA-RedDragon/zot-docker-proxy/internal/server"
	"github.com/USA-RedDragon/zot-docker-proxy/internal/tokenforge"
)

func mockProxyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("mock proxy"))
		if err != nil {
			http.Error(w, "failed to write response", http.StatusInternalServerError)
		}
	})
}

func TestServerStart(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             "http://localhost:5000",
	}
	router, err := server.NewRouter(cfg, mockProxyHandler())
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v2", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code == 401 || rec.Code == 200 {
		// Acceptable: 401 from dockerPingHandler or 200 from proxy (if no auth logic)
	} else {
		t.Errorf("unexpected status code: %d", rec.Code)
	}
}

func TestDockerAuthMiddleware_AnonymousToken(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             "http://localhost:5000",
	}
	router, err := server.NewRouter(cfg, mockProxyHandler())
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/docker-token", nil)
	req.Header.Set("User-Agent", "docker/24.0.0")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	var resp map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	if err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}
	if _, ok := resp["token"]; !ok {
		t.Errorf("expected token in response, got %s", rec.Body.String())
	}
	valid, err := tokenforge.VerifyToken(cfg.Secret, resp["token"])
	if err != nil || !valid {
		t.Errorf("token=%s", rec.Body.String())
		t.Errorf("expected valid token, got error: %v", err)
	}
}

func TestDockerAuthMiddleware_BasicToken(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             "http://localhost:5000",
	}
	router, err := server.NewRouter(cfg, mockProxyHandler())
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/docker-token", nil)
	req.Header.Set("User-Agent", "docker/24.0.0")
	req.Header.Set("Authorization", "Basic dGVzdDp0ZXN0")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "dGVzdDp0ZXN0") {
		t.Errorf("expected basic token in response, got %s", rec.Body.String())
	}
}

func TestDockerAuthMiddleware_Ping(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             "http://localhost:5000",
	}
	router, err := server.NewRouter(cfg, mockProxyHandler())
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v2", nil)
	req.Header.Set("User-Agent", "docker/24.0.0")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Errorf("expected 401 for docker ping, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("WWW-Authenticate"), "Bearer realm=") {
		t.Errorf("expected WWW-Authenticate header, got %s", rec.Header().Get("WWW-Authenticate"))
	}
}

func TestDockerV2Handler_ValidBearerToken(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             "http://localhost:5000",
		Secret:             "test-secret",
	}

	// Create a valid token using our tokenforge
	validToken, err := tokenforge.MakeToken(cfg.Secret, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Create a mock handler that can verify the Authorization header was removed
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "" {
			t.Errorf("expected Authorization header to be removed, but got: %s", auth)
		}
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("success"))
		if err != nil {
			http.Error(w, "failed to write response", http.StatusInternalServerError)
		}
	})

	router, err := server.NewRouter(cfg, mockHandler)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/manifests/latest", nil)
	req.Header.Set("User-Agent", "docker/24.0.0")
	req.Header.Set("Authorization", "Bearer "+validToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestDockerV2Handler_InvalidBearerToken(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             "http://localhost:5000",
		Secret:             "test-secret",
	}

	invalidToken := "invalid-token-12345"

	// Create a mock handler that can verify the Authorization header was converted to Basic
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("expected Authorization header to be converted to Basic, but got: %s", auth)
		}
		if !strings.Contains(auth, invalidToken) {
			t.Errorf("expected Authorization header to contain the invalid token, but got: %s", auth)
		}
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("success"))
		if err != nil {
			http.Error(w, "failed to write response", http.StatusInternalServerError)
		}
	})

	router, err := server.NewRouter(cfg, mockHandler)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/manifests/latest", nil)
	req.Header.Set("User-Agent", "docker/24.0.0")
	req.Header.Set("Authorization", "Bearer "+invalidToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestDockerV2Handler_EmptyBearerToken(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             "http://localhost:5000",
		Secret:             "test-secret",
	}

	// Create a mock handler that can verify the Authorization header remains unchanged
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer " {
			t.Errorf("expected Authorization header to remain 'Bearer ', but got: %s", auth)
		}
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("success"))
		if err != nil {
			http.Error(w, "failed to write response", http.StatusInternalServerError)
		}
	})

	router, err := server.NewRouter(cfg, mockHandler)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/manifests/latest", nil)
	req.Header.Set("User-Agent", "docker/24.0.0")
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
