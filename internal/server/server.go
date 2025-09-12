package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/USA-RedDragon/zot-docker-proxy/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type contextKey uint8

const (
	Config_ContextKey contextKey = iota
)

func NewRouter(cfg *config.Config) (*chi.Mux, error) {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(1 * time.Hour))
	if len(cfg.CORSAllowedOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins: cfg.CORSAllowedOrigins,
			AllowedMethods: []string{http.MethodGet, http.MethodOptions, http.MethodHead},
			AllowedHeaders: []string{"Accept", "Content-Type"},
		}))
	}

	r.Use(dockerAuthMiddleware(cfg))

	url, err := url.Parse(cfg.ZotURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse zot URL: %w", err)
	}

	handler := newReverseProxy(url)

	// Catch-all: proxy everything
	r.Handle("/*", handler)

	return r, nil
}

func newReverseProxy(upstream *url.URL) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(upstream)

	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		slog.Debug("Proxying request", "method", req.Method, "url", req.URL.String(), "host", req.Host, "to", upstream.String())
		origDirector(req)
		req.Host = upstream.Host
	}

	return proxy
}
