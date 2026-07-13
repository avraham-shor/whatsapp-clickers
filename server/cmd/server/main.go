// Command server is the single deployable binary: it validates
// configuration, connects to Postgres, runs migrations, and serves the
// JSON API plus the embedded SPA.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/config"
	"github.com/avraham-shor/whatsapp-clickers/internal/httpapi"
	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/webdist"
	"github.com/avraham-shor/whatsapp-clickers/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("server exiting", "error", err.Error())
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger.Info("configuration loaded")

	// Railway sends SIGTERM on every redeploy; drain in-flight requests
	// instead of dropping them.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return err
	}
	logger.Info("database connected")

	migrateCtx, cancelMigrate := context.WithTimeout(ctx, time.Minute)
	defer cancelMigrate()
	applied, err := store.Migrate(migrateCtx, cfg.DatabaseURL, migrations.FS)
	if err != nil {
		return err
	}
	logger.Info("migrations applied", "count", applied)

	st := store.New(pool)
	router := httpapi.NewRouter(st, webdist.FS())

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// WriteTimeout must not outlive long-lived connections; /ws
		// (story 2.3) hijacks the conn, which clears these deadlines.
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	logger.Info("server listening", "port", cfg.Port)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		logger.Info("server stopped cleanly")
		return nil
	}
}
