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

	proxyHandler, err := newReverseProxy(cfg.ZotURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create reverse proxy: %w", err)
	}

	// Catch-all: proxy everything
	r.Handle("/*", proxyHandler)

	return r, nil
}

func newReverseProxy(targetRaw string) (http.Handler, error) {
	target, err := url.Parse(targetRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target URL: %w", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)

	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		slog.Debug("Proxying request", "method", req.Method, "url", req.URL.String(), "host", req.Host, "to", target.String())
		origDirector(req)
		req.Host = target.Host
	}

	return proxy, nil
}
