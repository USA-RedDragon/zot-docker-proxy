package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/USA-RedDragon/configulator"
	"github.com/USA-RedDragon/zot-docker-proxy/internal/config"
	"github.com/USA-RedDragon/zot-docker-proxy/internal/server"
	"github.com/lmittmann/tint"
	"github.com/spf13/cobra"
)

func NewCommand(version, commit string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "zot-docker-proxy",
		Version: fmt.Sprintf("%s - %s", version, commit),
		Annotations: map[string]string{
			"version": version,
			"commit":  commit,
		},
		RunE:              runRoot,
		SilenceErrors:     true,
		DisableAutoGenTag: true,
	}
	return cmd
}

func runRoot(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	fmt.Printf("zot-docker-proxy - %s (%s)\n", cmd.Annotations["version"], cmd.Annotations["commit"])

	c, err := configulator.FromContext[config.Config](ctx)
	if err != nil {
		return fmt.Errorf("failed to get config from context")
	}

	cfg, err := c.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var logger *slog.Logger
	switch cfg.LogLevel {
	case config.LogLevelDebug:
		logger = slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelDebug}))
	case config.LogLevelInfo:
		logger = slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelInfo}))
	case config.LogLevelWarn:
		logger = slog.New(tint.NewHandler(os.Stderr, &tint.Options{Level: slog.LevelWarn}))
	case config.LogLevelError:
		logger = slog.New(tint.NewHandler(os.Stderr, &tint.Options{Level: slog.LevelError}))
	}
	slog.SetDefault(logger)

	r, err := server.NewRouter(cfg)
	if err != nil {
		return fmt.Errorf("failed to create server router: %w", err)
	}

	serverCtx, serverStopCtx := context.WithCancel(ctx)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		ReadHeaderTimeout: 60 * time.Second,
		IdleTimeout:       60 * time.Second,
		Handler:           r,
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		signal := <-sig

		slog.Info("received signal, shutting down gracefully", "signal", signal.String())

		stopCtx, cancel := context.WithTimeout(serverCtx, 30*time.Second)
		defer cancel()

		go func() {
			<-stopCtx.Done()
			if errors.Is(stopCtx.Err(), context.DeadlineExceeded) {
				slog.Error("graceful shutdown timed out, forcing exit")
				os.Exit(1)
			}
		}()

		err := server.Shutdown(stopCtx)
		if err != nil {
			slog.Error("failed to stop server", "error", err)
			os.Exit(1)
		}
		serverStopCtx()
	}()

	slog.Info("server started", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to start server: %w", err)
	}

	<-serverCtx.Done()

	return nil
}
