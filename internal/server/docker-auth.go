package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/USA-RedDragon/zot-docker-proxy/internal/config"
)

func dockerAuthMiddleware(cfg *config.Config) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ua := r.Header.Get("User-Agent")
			if strings.HasPrefix(ua, "docker/") {
				path := r.URL.Path
				auth := r.Header.Get("Authorization")

				switch {
				case path == "/docker-token":
					dockerTokenHandler(w, r)
					return
				case (path == "/v2" || path == "/v2/") && auth == "":
					dockerPingHandler(cfg, w, r)
					return
				case path == "/v2" || strings.HasPrefix(path, "/v2/"):
					dockerV2Handler(w, r)
				}

				next.ServeHTTP(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func dockerTokenHandler(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	token := "QQ="
	if auth != "" && strings.HasPrefix(auth, "Basic ") {
		b64 := strings.TrimSpace(auth[len("Basic "):])
		if b64 != "" {
			token = b64
		}
	}
	w.Header().Set("Content-Type", "application/json")
	tokenBytes := fmt.Sprintf(`{"token":"%s"}`, token)
	bytes, err := w.Write([]byte(tokenBytes))
	if err != nil {
		slog.Error("Failed to write token response", "error", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if bytes != len(tokenBytes) {
		slog.Warn("Wrote fewer bytes than expected in token response", "expected", len(tokenBytes), "written", bytes)
	}
}

func dockerPingHandler(cfg *config.Config, w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		tokenURL, err := url.JoinPath(cfg.MyURL, "/docker-token")
		if err != nil {
			slog.Error("Failed to build token URL", "error", err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		slog.Debug("Docker ping without Authorization, sending 401 with WWW-Authenticate Bearer", "url", r.URL.String(), "token_url", tokenURL)
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s"`, tokenURL))
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
}

func dockerV2Handler(_ http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		tok := strings.TrimSpace(auth[len("Bearer "):])
		if tok == "QQ=" {
			r.Header.Del("Authorization")
		} else if tok != "" {
			r.Header.Set("Authorization", "Basic "+tok)
		}
	}
}
