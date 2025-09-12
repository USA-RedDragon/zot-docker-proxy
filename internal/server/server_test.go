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

// createTestBackend creates a test HTTP server that can be used as a backend for proxy tests
func createTestBackend(response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-Called", "true")
		w.Header().Set("X-Backend-Host", r.Host)
		w.Header().Set("X-Backend-URL", r.URL.String())
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(response))
		if err != nil {
			http.Error(w, "failed to write response", http.StatusInternalServerError)
		}
	}))
}

func TestServerStart(t *testing.T) {
	t.Parallel()

	// Create a mock backend server
	backend := createTestBackend("backend response")
	defer backend.Close()

	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             backend.URL,
	}
	router, err := server.NewRouter(cfg)
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

func TestServerWithRealProxy(t *testing.T) {
	t.Parallel()

	// Create a mock backend server
	backend := createTestBackend("backend response")
	defer backend.Close()

	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             backend.URL,
	}

	// Create router without passing a mock handler - this will use NewReverseProxy
	router, err := server.NewRouter(cfg)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

	// Test a non-docker request that should be proxied
	req := httptest.NewRequest(http.MethodGet, "/some/path", nil)
	req.Header.Set("User-Agent", "test-client/1.0")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-Backend-Called") != "true" {
		t.Errorf("expected backend to be called, but X-Backend-Called header not found")
	}
	if !strings.Contains(rec.Body.String(), "backend response") {
		t.Errorf("expected backend response, got %s", rec.Body.String())
	}
}

func TestNewReverseProxy(t *testing.T) {
	t.Parallel()

	// Create a mock backend server
	backend := createTestBackend("proxied content")
	defer backend.Close()

	// Test creating a reverse proxy
	proxy, err := server.NewReverseProxy(backend.URL)
	if err != nil {
		t.Fatalf("failed to create reverse proxy: %v", err)
	}

	// Test that the proxy works
	req := httptest.NewRequest(http.MethodGet, "/test/path", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "proxied content") {
		t.Errorf("expected proxied content, got %s", rec.Body.String())
	}

	// Verify that the request was properly forwarded
	backendHost := rec.Header().Get("X-Backend-Host")
	if backendHost == "" {
		t.Errorf("expected backend host header to be set")
	}

	backendURL := rec.Header().Get("X-Backend-URL")
	if !strings.Contains(backendURL, "/test/path") {
		t.Errorf("expected backend URL to contain /test/path, got %s", backendURL)
	}
}

func TestNewReverseProxy_InvalidURL(t *testing.T) {
	t.Parallel()

	// Test with invalid URL (using invalid characters)
	_, err := server.NewReverseProxy("http://[::1]:%")
	if err == nil {
		t.Errorf("expected error for invalid URL, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse target URL") {
		t.Errorf("expected parse error message, got %v", err)
	}
}

func TestDockerAuthMiddleware_AnonymousToken(t *testing.T) {
	t.Parallel()

	// Create a mock backend server
	backend := createTestBackend("backend response")
	defer backend.Close()

	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             backend.URL,
	}
	router, err := server.NewRouter(cfg)
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

	// Create a mock backend server
	backend := createTestBackend("backend response")
	defer backend.Close()

	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             backend.URL,
	}
	router, err := server.NewRouter(cfg)
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

	// Create a mock backend server
	backend := createTestBackend("backend response")
	defer backend.Close()

	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             backend.URL,
	}
	router, err := server.NewRouter(cfg)
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

	// Create a backend that captures the Authorization header
	var capturedAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("success"))
		if err != nil {
			http.Error(w, "failed to write response", http.StatusInternalServerError)
		}
	}))
	defer backend.Close()

	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             backend.URL,
		Secret:             "test-secret",
	}

	// Create a valid token using our tokenforge
	validToken, err := tokenforge.MakeToken(cfg.Secret, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	router, err := server.NewRouter(cfg)
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

	// Verify that the Authorization header was removed (should be empty)
	if capturedAuth != "" {
		t.Errorf("expected Authorization header to be removed, but got: %s", capturedAuth)
	}
}

func TestDockerV2Handler_InvalidBearerToken(t *testing.T) {
	t.Parallel()

	// Create a backend that captures the Authorization header
	var capturedAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("success"))
		if err != nil {
			http.Error(w, "failed to write response", http.StatusInternalServerError)
		}
	}))
	defer backend.Close()

	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             backend.URL,
		Secret:             "test-secret",
	}

	invalidToken := "invalid-token-12345"

	router, err := server.NewRouter(cfg)
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

	// Verify that the Authorization header was converted to Basic
	if !strings.HasPrefix(capturedAuth, "Basic ") {
		t.Errorf("expected Authorization header to be converted to Basic, but got: %s", capturedAuth)
	}
	if !strings.Contains(capturedAuth, invalidToken) {
		t.Errorf("expected Authorization header to contain the invalid token, but got: %s", capturedAuth)
	}
}

func TestDockerV2Handler_EmptyBearerToken(t *testing.T) {
	t.Parallel()

	// Create a backend that captures the Authorization header
	var capturedAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("success"))
		if err != nil {
			http.Error(w, "failed to write response", http.StatusInternalServerError)
		}
	}))
	defer backend.Close()

	cfg := &config.Config{
		LogLevel:           config.LogLevelInfo,
		Port:               8080,
		CORSAllowedOrigins: []string{"*"},
		MyURL:              "http://localhost:8080",
		ZotURL:             backend.URL,
		Secret:             "test-secret",
	}

	router, err := server.NewRouter(cfg)
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

	// Verify that the Authorization header remains unchanged for empty bearer token
	// Note: HTTP processing may trim trailing spaces, so we check for either form
	if capturedAuth != "Bearer " && capturedAuth != "Bearer" {
		t.Errorf("expected Authorization header to remain 'Bearer ' or 'Bearer', but got: %s", capturedAuth)
	}
}
