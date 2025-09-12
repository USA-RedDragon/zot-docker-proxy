package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/USA-RedDragon/zot-docker-proxy/internal/config"
	"github.com/USA-RedDragon/zot-docker-proxy/internal/tokenforge"
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
					dockerTokenHandler(cfg, w, r)
					return
				case (path == "/v2" || path == "/v2/") && auth == "":
					dockerPingHandler(cfg, w, r)
					return
				case path == "/v2" || strings.HasPrefix(path, "/v2/"):
					dockerV2Handler(cfg, w, r)
				}

				next.ServeHTTP(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func dockerTokenHandler(cfg *config.Config, w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	token := ""
	if auth != "" && strings.HasPrefix(auth, "Basic ") {
		b64 := strings.TrimSpace(auth[len("Basic "):])
		if b64 != "" {
			token = b64
		}
	}
	if len(token) == 0 {
		var err error
		token, err = tokenforge.MakeToken(cfg.Secret, 1*time.Hour)
		if err != nil {
			slog.Error("Failed to generate anonymous token", "error", err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	tokenBytes, err := json.Marshal(map[string]string{
		"token": token,
	})
	if err != nil {
		slog.Error("Failed to marshal token response", "error", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	written, err := w.Write(tokenBytes)
	if err != nil {
		slog.Error("Failed to write token response", "error", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if written != len(tokenBytes) {
		slog.Warn("Wrote fewer bytes than expected in token response", "expected", len(tokenBytes), "written", written)
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
		w.Header().Set("WWW-Authenticate", "Bearer realm="+strconv.Quote(tokenURL))
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
}

func dockerV2Handler(cfg *config.Config, _ http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		tok := strings.TrimSpace(auth[len("Bearer "):])
		validated, err := tokenforge.VerifyToken(cfg.Secret, tok)
		if err != nil {
			// This can happen normally if the token is expired or invalid, or if
			// docker is actually logged in with a real token.
			slog.Debug("Failed to verify token", "error", err.Error(), "token", tok)
		}
		if validated {
			slog.Debug("Verified token")
			r.Header.Del("Authorization")
		} else if tok != "" {
			r.Header.Set("Authorization", "Basic "+tok)
		}
	}
}
