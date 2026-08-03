// Package game owns the canonical game-state enum and is the single write
// path for games.state — no other package mutates it directly.
package game

// State is a plain string alias, not a distinct type: gen.Game.State is
// sqlc-generated as a plain string, and a distinct type would force casts
// at every comparison against a gen.Game value for no real safety gain.
type State = string

// The seven canonical states — identical strings in Go/TS/DB (the DB CHECK
// on games.state and web/src/lib/types.ts's GameState union).
const (
	StateDraft          State = "draft"
	StateLobby          State = "lobby"
	StateQuestionOpen   State = "question_open"
	StateQuestionClosed State = "question_closed"
	StateRevealed       State = "revealed"
	StateLeaderboard    State = "leaderboard"
	StateFinished       State = "finished"
)

// Role is a plain string alias, not a distinct type — same reasoning as
// State: gen.Participant.Role is sqlc-generated as a plain string.
type Role = string

// The two canonical participant roles (DB CHECK on participants.role).
// Only RolePlayer has a code path this story — RoleSpectator is Story 2.5's.
const (
	RolePlayer    Role = "player"
	RoleSpectator Role = "spectator"
)
