package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
