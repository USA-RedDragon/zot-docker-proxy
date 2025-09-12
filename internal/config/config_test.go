package config

import (
	"errors"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     Config
		wantErr error
	}{
		{
			name:    "valid config",
			cfg:     Config{LogLevel: LogLevelInfo, Port: 8080, CORSAllowedOrigins: []string{"*"}, MyURL: "http://localhost:8080", ZotURL: "http://localhost:5000", Secret: "supersecret"},
			wantErr: nil,
		},
		{
			name:    "invalid log level",
			cfg:     Config{LogLevel: "bad", Port: 8080, CORSAllowedOrigins: []string{"*"}, MyURL: "http://localhost:8080", ZotURL: "http://localhost:5000", Secret: "supersecret"},
			wantErr: ErrInvalidLogLevel,
		},
		{
			name:    "invalid port",
			cfg:     Config{LogLevel: LogLevelInfo, Port: 0, CORSAllowedOrigins: []string{"*"}, MyURL: "http://localhost:8080", ZotURL: "http://localhost:5000", Secret: "supersecret"},
			wantErr: ErrInvalidPort,
		},
		{
			name:    "missing zot url",
			cfg:     Config{LogLevel: LogLevelInfo, Port: 8080, CORSAllowedOrigins: []string{"*"}, MyURL: "http://localhost:8080", ZotURL: "", Secret: "supersecret"},
			wantErr: ErrZotURLRequired,
		},
		{
			name:    "invalid zot url",
			cfg:     Config{LogLevel: LogLevelInfo, Port: 8080, CORSAllowedOrigins: []string{"*"}, MyURL: "http://localhost:8080", ZotURL: "ftp://bad", Secret: "supersecret"},
			wantErr: ErrInvalidZotURL,
		},
		{
			name:    "missing my url",
			cfg:     Config{LogLevel: LogLevelInfo, Port: 8080, CORSAllowedOrigins: []string{"*"}, MyURL: "", ZotURL: "http://localhost:5000", Secret: "supersecret"},
			wantErr: ErrMyURLRequired,
		},
		{
			name:    "invalid my url",
			cfg:     Config{LogLevel: LogLevelInfo, Port: 8080, CORSAllowedOrigins: []string{"*"}, MyURL: "ftp://bad", ZotURL: "http://localhost:5000", Secret: "supersecret"},
			wantErr: ErrInvalidMyURL,
		},
		{
			name:    "missing secret",
			cfg:     Config{LogLevel: LogLevelInfo, Port: 8080, CORSAllowedOrigins: []string{"*"}, MyURL: "http://localhost:8080", ZotURL: "http://localhost:5000"},
			wantErr: ErrSecretRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.wantErr == nil && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}
