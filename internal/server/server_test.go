package server

import (
	"testing"
)


import (
	"net/http"
	"net/http/httptest"
	"strings"
	"github.com/USA-RedDragon/zot-docker-proxy/internal/config"
)

func mockProxyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("mock proxy"))
	})
}

func TestServerStart(t *testing.T) {
	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             "http://localhost:5000",
	}
	router, err := NewRouter(cfg, mockProxyHandler())
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}
	req := httptest.NewRequest("GET", "/v2", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code == 401 || rec.Code == 200 {
		// Acceptable: 401 from dockerPingHandler or 200 from proxy (if no auth logic)
	} else {
		t.Errorf("unexpected status code: %d", rec.Code)
	}
}

func TestDockerAuthMiddleware_AnonymousToken(t *testing.T) {
	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             "http://localhost:5000",
	}
	router, _ := NewRouter(cfg, mockProxyHandler())
	req := httptest.NewRequest("GET", "/docker-token", nil)
	req.Header.Set("User-Agent", "docker/24.0.0")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "docker-anonymous-token") {
		t.Errorf("expected anonymous token in response, got %s", rec.Body.String())
	}
}

func TestDockerAuthMiddleware_BasicToken(t *testing.T) {
	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             "http://localhost:5000",
	}
	router, _ := NewRouter(cfg, mockProxyHandler())
	req := httptest.NewRequest("GET", "/docker-token", nil)
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
	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             "http://localhost:5000",
	}
	router, _ := NewRouter(cfg, mockProxyHandler())
	req := httptest.NewRequest("GET", "/v2", nil)
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
