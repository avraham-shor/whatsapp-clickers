package game

import (
	"context"
	"errors"
	"strings"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// ErrInvalidJoinCode means no game matches the JOIN code sent.
var ErrInvalidJoinCode = errors.New("game: invalid join code")

// ErrGameStarted means the code is valid but the game has moved past lobby.
// Story 2.5 turns this into spectator registration; until then it is an
// intentional, documented gap — see story 2.4 Dev Notes.
var ErrGameStarted = errors.New("game: already started")

// ErrParticipantNotFound means a name-change sender has no participant row
// anywhere.
var ErrParticipantNotFound = errors.New("game: participant not found")

// JoinOutcome distinguishes the two non-error paths Join can take.
type JoinOutcome string

const (
	JoinWelcome  JoinOutcome = "welcome"
	JoinPreLobby JoinOutcome = "pre_lobby"
)

// JoinResult is Join's success return. Snapshot is the zero value unless
// Created is true AND the post-insert snapshot build succeeded — callers
// broadcast only when Snapshot.GameID != "".
type JoinResult struct {
	Outcome     JoinOutcome
	GameID      string
	DisplayName string
	Created     bool
	Snapshot    Snapshot
}

// RenameResult is Rename's success return.
type RenameResult struct {
	DisplayName string
}

// fallbackParticipantName returns the last 4 characters of phone (or phone
// itself if shorter). [ASSUMPTION — not specified in PRD/UX; EXPERIENCE.md
// A3 assumes a WhatsApp profile name always exists, but contacts[] can be
// absent (wa/webhook.go's profileNameFor already documents this).] Digits
// only, gender-neutral, and immediately correctable via the name-change
// command — deliberately not a gendered placeholder noun (would violate
// UX-DR/A2's gender-neutral-Hebrew rule).
func fallbackParticipantName(phone string) string {
	if len(phone) <= 4 {
		return phone
	}
	return phone[len(phone)-4:]
}

// Join registers phone as a Participant of the game identified by joinCode,
// or reports why it could not. joinCode arrives already normalized (trimmed,
// uppercased) — that is wa's job; Join does not re-normalize (validate at
// boundaries).
func (e *Engine) Join(ctx context.Context, joinCode, phone, profileName string) (JoinResult, error) {
	g, err := e.store.GetGameByJoinCode(ctx, joinCode)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return JoinResult{}, ErrInvalidJoinCode
		}
		return JoinResult{}, err
	}
	switch g.State {
	case StateDraft:
		return JoinResult{Outcome: JoinPreLobby, GameID: g.ID}, nil
	case StateLobby:
		return e.joinLobby(ctx, g, phone, profileName)
	default:
		return JoinResult{}, ErrGameStarted
	}
}

func (e *Engine) joinLobby(ctx context.Context, g gen.Game, phone, profileName string) (JoinResult, error) {
	displayName := strings.TrimSpace(profileName)
	if displayName == "" {
		displayName = fallbackParticipantName(phone)
	}
	p, created, err := e.store.CreateParticipant(ctx, g.ID, phone, displayName, RolePlayer)
	if err != nil {
		return JoinResult{}, err
	}
	result := JoinResult{Outcome: JoinWelcome, GameID: g.ID, DisplayName: p.DisplayName, Created: created}
	if !created {
		// Idempotent repeat (AC-3): the count didn't change, nothing to
		// broadcast.
		return result, nil
	}
	snap, err := e.buildSnapshot(ctx, g)
	if err != nil {
		// The participant row already committed — this join must not fail.
		// Unlike OpenLobby's degrade-to-empty-snapshot, there is no safe
		// placeholder count here: it would undercount by at least this
		// participant. Skip the broadcast entirely; the next successful join
		// or a dashboard reconnect self-heals it.
		e.logger.Warn("post-join snapshot build failed, skipping broadcast", "game_id", g.ID, "error", err)
		return result, nil
	}
	result.Snapshot = snap
	return result, nil
}

// Rename updates phone's most-recently-joined participant row's display
// name (scoped across all games — see story 2.4 Dev Notes' name-command
// scoping decision). No broadcast — a deliberate scope decision, not an
// oversight.
func (e *Engine) Rename(ctx context.Context, phone, displayName string) (RenameResult, error) {
	p, err := e.store.UpdateParticipantNameByPhone(ctx, phone, displayName)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return RenameResult{}, ErrParticipantNotFound
		}
		return RenameResult{}, err
	}
	return RenameResult{DisplayName: p.DisplayName}, nil
}
