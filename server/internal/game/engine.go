package game

import (
	"context"
	"errors"
	"log/slog"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// ErrNotDraft means the game exists (and is owned by the caller) but is not
// in draft state — the primary, message-accurate rejection for OpenLobby.
var ErrNotDraft = errors.New("game: not in draft state")

// Store is the persistence surface the engine needs; *store.Store satisfies
// it. Consumer-defined here, not in store, per the dependency direction
// (game imports store, never the reverse).
type Store interface {
	GetGameForOrganizer(ctx context.Context, gameID, organizerID string) (gen.Game, error)
	OpenGameLobby(ctx context.Context, gameID, organizerID string) (gen.Game, error)
	ListParticipants(ctx context.Context, gameID string) ([]gen.Participant, error)
}

// Engine is the single write path for games.state (Enforcement Guidelines:
// "no component or handler mutates game state directly"). It builds every
// snapshot fresh from Postgres — nothing worth caching engine-side at this
// story's scale (one transition, no timers, no per-question state).
type Engine struct {
	store          Store
	platformNumber string
	logger         *slog.Logger
}

// NewEngine builds an Engine. platformNumber is the WhatsApp number shown
// on the lobby page (WHATSAPP_DISPLAY_NUMBER). logger may be nil, in which
// case slog.Default() is used.
func NewEngine(st Store, platformNumber string, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{store: st, platformNumber: platformNumber, logger: logger}
}

// OpenLobby transitions gameID from draft to lobby and returns the
// resulting snapshot. A non-draft game (including one that lost a
// concurrent transition race between the read and the write below) is
// ErrNotDraft; a missing/foreign game is store.ErrNotFound.
func (e *Engine) OpenLobby(ctx context.Context, gameID, organizerID string) (Snapshot, error) {
	g, err := e.store.GetGameForOrganizer(ctx, gameID, organizerID)
	if err != nil {
		return Snapshot{}, err
	}
	if g.State != StateDraft {
		return Snapshot{}, ErrNotDraft
	}
	g, err = e.store.OpenGameLobby(ctx, gameID, organizerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// The race-guard WHERE clause (state = 'draft') lost a
			// concurrent race: the game still exists, it just stopped
			// being draft between the read above and this write —
			// ErrNotDraft, not ErrNotFound.
			return Snapshot{}, ErrNotDraft
		}
		return Snapshot{}, err
	}
	// The state transition already committed at this point. A failure
	// building the participant list here must not be reported as a failed
	// transition — the caller would retry into a confusing 409 for a change
	// that already happened, while every WS client stays stuck on the
	// pre-transition snapshot. Degrade to an empty participant list and let
	// the next real read (a reconnect, or a future join broadcast) pick up
	// the true count instead.
	snap, err := e.buildSnapshot(ctx, g)
	if err != nil {
		e.logger.Warn("post-transition snapshot build failed, degrading to empty participants", "game_id", gameID, "error", err)
		return emptySnapshot(e.platformNumber, g), nil
	}
	return snap, nil
}

// Snapshot returns the current live-state snapshot for gameID, scoped to
// organizerID (ownership + existence in one call). This is the read path
// ws.Handler calls on every new connection.
func (e *Engine) Snapshot(ctx context.Context, gameID, organizerID string) (Snapshot, error) {
	g, err := e.store.GetGameForOrganizer(ctx, gameID, organizerID)
	if err != nil {
		return Snapshot{}, err
	}
	return e.buildSnapshot(ctx, g)
}

func (e *Engine) buildSnapshot(ctx context.Context, g gen.Game) (Snapshot, error) {
	participants, err := e.store.ListParticipants(ctx, g.ID)
	if err != nil {
		return Snapshot{}, err
	}
	// Non-nil so the wire always carries [], never null (newGameDetailPayload
	// precedent).
	summaries := make([]ParticipantSummary, 0, len(participants))
	for _, p := range participants {
		summaries = append(summaries, ParticipantSummary{ID: p.ID, DisplayName: p.DisplayName})
	}
	return Snapshot{
		GameID:           g.ID,
		State:            g.State,
		JoinCode:         g.JoinCode,
		PlatformNumber:   e.platformNumber,
		ParticipantCount: len(participants),
		Participants:     summaries,
	}, nil
}

// emptySnapshot builds a snapshot with a non-nil, empty participant list —
// the degraded fallback for OpenLobby when the transition itself committed
// but the follow-up participant read failed.
func emptySnapshot(platformNumber string, g gen.Game) Snapshot {
	return Snapshot{
		GameID:           g.ID,
		State:            g.State,
		JoinCode:         g.JoinCode,
		PlatformNumber:   platformNumber,
		ParticipantCount: 0,
		Participants:     []ParticipantSummary{},
	}
}
