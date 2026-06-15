package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"marketengine/internal/api/http/gateway"
	"marketengine/internal/config"
	pggateway "marketengine/internal/infra/postgres/gateway"
	"marketengine/internal/storage"
)

var GitSHA = "dev"

func main() {
	configPath := flag.String("config", "configs/gateway.yaml", "YAML config")
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(*configPath); err != nil {
		slog.Error("gateway exit", "err", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	loaded, err := config.Load[config.GatewayConfig](configPath)
	if err != nil {
		return err
	}
	cfg := loaded.Parsed
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:8080"
	}
	slog.Info("gateway config loaded", "listen", cfg.Listen, "config_version", loaded.ConfigVersion)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	pool, err := storage.Open(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()
	h := &gateway.Handler{
		Regime:   pggateway.NewRegimeReader(pool),
		Scores:   pggateway.NewDomainScoreReader(pool),
		Backtest: pggateway.NewBacktestReader(pool),
		Health:   pggateway.NewHealthChecker(pool),
		GitSHA:   GitSHA,
	}
	srv := &http.Server{Addr: cfg.Listen, Handler: h.Routes(), ReadHeaderTimeout: 10 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("gateway listening", "addr", cfg.Listen)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-errCh:
		return err
	}
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	slog.Info("gateway stopped")
	return nil
}
