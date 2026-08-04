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

	"github.com/avraham-shor/whatsapp-clickers/internal/auth"
	"github.com/avraham-shor/whatsapp-clickers/internal/config"
	"github.com/avraham-shor/whatsapp-clickers/internal/game"
	"github.com/avraham-shor/whatsapp-clickers/internal/httpapi"
	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/wa"
	"github.com/avraham-shor/whatsapp-clickers/internal/webdist"
	"github.com/avraham-shor/whatsapp-clickers/internal/ws"
	"github.com/avraham-shor/whatsapp-clickers/migrations"
)

const (
	// httpShutdownTimeout bounds draining in-flight HTTP requests.
	httpShutdownTimeout = 15 * time.Second
	// dispatcherDrainTimeout bounds emptying the outbound queue AFTER the HTTP
	// server has stopped — by then every handler has returned, so everything
	// they queued is already in the queue.
	dispatcherDrainTimeout = 5 * time.Second
	// dispatcherStopGrace lets Run return after its context is cancelled, so
	// its "shut down with unsent messages" summary WARN reaches the log before
	// the process exits. The three sum to a 22s worst case — and Railway's
	// default SIGTERM→SIGKILL grace is ZERO seconds, so none of this runs on
	// a redeploy unless RAILWAY_DEPLOYMENT_DRAINING_SECONDS is set on the
	// service to at least that worst case (30 leaves margin). See
	// .env.example for the ops note.
	dispatcherStopGrace = 2 * time.Second
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
	authSvc := auth.NewService(st, cfg.SessionSecret)

	var clientOpts []wa.ClientOption
	if cfg.WhatsAppAPIBaseURL != "" {
		// Test-harness only: points the client at a local fake provider so an
		// E2E run can assert replies are actually sent. Unset in production.
		logger.Info("WhatsApp API base URL overridden", "base_url", cfg.WhatsAppAPIBaseURL)
		clientOpts = append(clientOpts, wa.WithBaseURL(cfg.WhatsAppAPIBaseURL))
	}
	waClient := wa.NewClient(cfg.WhatsAppAccessToken, cfg.WhatsAppPhoneNumberID, clientOpts...)
	dispatcher := wa.NewDispatcher(waClient, logger)

	// The dispatcher runs on its own context, NOT the signal context: on
	// SIGTERM, srv.Shutdown keeps serving in-flight webhook requests for up to
	// httpShutdownTimeout, and every reply those requests enqueue needs a live
	// worker pool to pick it up. Sharing ctx would stop the workers first and
	// swallow the queue with no drop log at all — "the chat goes silent on
	// redeploy", which is every merge to main.
	dispatchCtx, stopDispatcher := context.WithCancel(context.Background())
	defer stopDispatcher()
	dispatcherDone := make(chan struct{})
	go func() {
		defer close(dispatcherDone)
		dispatcher.Run(dispatchCtx)
	}()

	// st satisfies game.Store (GetGameForOrganizer, OpenGameLobby, ListParticipants,
	// GetGameByJoinCode, CreateParticipant, UpdateParticipantNameByPhone,
	// ListQuestionsByGame, StartGameFirstQuestion, CloseCurrentQuestion,
	// RevealCurrentQuestion, OpenNextQuestion, FinishGame).
	engine := game.NewEngine(st, cfg.WhatsAppDisplayNumber, logger)
	hub := ws.NewHub(logger)
	wsHandler := ws.NewHandler(authSvc, engine, hub, logger)

	// *game.Engine satisfies wa.Registrar, *ws.Hub satisfies wa.SnapshotBroadcaster.
	inboundRouter := wa.NewInboundRouter(dispatcher, engine, hub, logger)
	// st satisfies Deduper directly (MarkWaMessageProcessed).
	webhookHandler := wa.NewWebhookHandler(cfg.WhatsAppAppSecret, cfg.WhatsAppVerifyToken, cfg.WhatsAppPhoneNumberID, st, inboundRouter, logger)

	// st satisfies both Pinger and GameStore.
	router := httpapi.NewRouter(st, authSvc, st, webdist.FS(), webhookHandler, engine, hub, wsHandler)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// WriteTimeout must not outlive long-lived connections; /ws
		// hijacks the conn, which clears these deadlines.
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	logger.Info("server listening", "port", cfg.Port)

	// drainAndStop is the dispatcher teardown for EVERY exit from run, not
	// just the clean one: drain the queue, stop the workers, wait for Run so
	// its unsent-message summary WARN is logged rather than lost to process
	// exit. The error paths need it most — they fire when handlers were
	// still busy (Shutdown timeout) or the listener died, which is exactly
	// when the queue is fullest; skipping it there would silently discard
	// every queued reply, the failure mode this wiring exists to prevent.
	// On paths where handlers may still be running, the drain is best-effort
	// and bounded by dispatcherDrainTimeout. Returns true when the
	// dispatcher stopped within its grace.
	drainAndStop := func() bool {
		drainCtx, cancelDrain := context.WithTimeout(context.Background(), dispatcherDrainTimeout)
		dispatcher.Drain(drainCtx)
		cancelDrain()
		stopDispatcher()
		select {
		case <-dispatcherDone:
			return true
		case <-time.After(dispatcherStopGrace):
			logger.Warn("dispatcher did not stop within the shutdown grace")
			return false
		}
	}

	select {
	case err := <-errCh:
		drainAndStop()
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			drainAndStop()
			return err
		}
		if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
			drainAndStop()
			return err
		}

		// Every handler has returned, so the queue now holds everything
		// they enqueued; this drain is the complete one, and only a clean
		// dispatcher join earns the all-clear line — log-based alerting
		// must never see a warning and an all-clear for the same shutdown.
		if drainAndStop() {
			logger.Info("server stopped cleanly")
		}
		return nil
	}
}
