package config

import (
	"errors"
	"net/url"
)

var (
	ErrInvalidLogLevel = errors.New("invalid log level provided")
	ErrInvalidPort     = errors.New("port must be between 1 and 65535")
	ErrZotURLRequired  = errors.New("zot-url is required")
	ErrInvalidZotURL   = errors.New("zot-url must be a valid URL starting with http:// or https://")
	ErrMyURLRequired   = errors.New("my-url is required if cors-allowed-origins is not set to default")
	ErrInvalidMyURL    = errors.New("my-url must be a valid URL starting with http:// or https://")
)

type Config struct {
	LogLevel           LogLevel `name:"log-level" description:"Logging level for the application. One of debug, info, warn, or error" default:"info"`
	Port               int      `name:"port" description:"Port to listen on" default:"8080"`
	CORSAllowedOrigins []string `name:"cors-allowed-origins" description:"CORS allowed origins" default:"https://*,http://*"`
	MyURL              string   `name:"my-url" description:"The protocol, host (and port if necessary) where this proxy is running."`
	ZotURL             string   `name:"zot-url" description:"The protocol, host (and port if necessary) where the Zot registry is running"`
}

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

func (c Config) Validate() error {
	if c.LogLevel != LogLevelDebug &&
		c.LogLevel != LogLevelInfo &&
		c.LogLevel != LogLevelWarn &&
		c.LogLevel != LogLevelError {
		return ErrInvalidLogLevel
	}

	if c.Port < 1 || c.Port > 65535 {
		return ErrInvalidPort
	}

	if c.ZotURL == "" {
		return ErrZotURLRequired
	}

	url, err := url.Parse(c.ZotURL)
	if err != nil || (url.Scheme != "http" && url.Scheme != "https") || url.Host == "" {
		return ErrInvalidZotURL
	}

	if c.MyURL == "" {
		return ErrMyURLRequired
	}

	url, err = url.Parse(c.MyURL)
	if err != nil || (url.Scheme != "http" && url.Scheme != "https") || url.Host == "" {
		return ErrInvalidMyURL
	}

	return nil
}
