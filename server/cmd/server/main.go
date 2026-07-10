// Command server is the single deployable binary: it validates
// configuration, connects to Postgres, runs migrations, and serves the
// JSON API plus the embedded SPA.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
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

	ctx := context.Background()
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
	}
	logger.Info("server listening", "port", cfg.Port)
	return srv.ListenAndServe()
}
