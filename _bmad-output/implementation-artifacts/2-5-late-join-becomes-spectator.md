---
baseline_commit: 4c61763
---

# Story 2.5: Late Join Becomes Spectator

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a person joining after the Game started,
I want to be told the Game already began and still get the final results,
so that I am included without disrupting play (FR-3).

## Acceptance Criteria

1. **Given** a Game in `question_open`, `question_closed`, `revealed`, or `leaderboard` (EXPERIENCE.md's conversation-grammar "Question open" / "Between questions²" columns — footnote ²: "covers the closed, revealed, and leaderboard states"), **when** a person sends `JOIN <code>` with its valid code, **then** they are registered as a Participant with `role = spectator` — display name resolved the same way as a Player join (WhatsApp profile name, falling back to the phone's last 4 digits) — and receive the Hebrew Spectator-notice reply verbatim from the canonical templates table, **and** no snapshot broadcast fires (see Dev Notes — deliberate, not a degrade). *(epic AC-1, states scoped precisely — see AC 2)*
2. **Given** a Game in `finished` ("Game over" — a **distinct** column in EXPERIENCE.md's conversation-grammar matrix, **not** part of "Between questions²"), **when** a person sends `JOIN <code>` with its valid code, **then** they receive the Help reply and **no** participant row is created — registering a spectator for a game that will never fire another `GameFinished`-triggered results dispatch would make the Spectator-notice's "results will arrive at game end" promise false. This narrows epics.md's summary phrasing ("any state past lobby") to match EXPERIENCE.md's precise per-state table, which epics.md itself names as the authoritative behavior source for this OQ-7 area — see Dev Notes, this is the story's central judgment call. *(implementation-completeness correction, not a literal epics.md line item)*
3. **Given** a registered Spectator, **when** they re-send `JOIN` with the same code (including under a Meta webhook retry — already deduped by story 2.1's message-ID ledger before this code ever runs), **then** no duplicate row is created (the existing `UNIQUE (game_id, phone)` constraint + `CreateParticipant`'s `ON CONFLICT DO NOTHING` + refetch — already race-safe per story 2.4's AC-4, reused unchanged) and they receive the same Spectator-notice reply. *(epic AC-3)*
4. **Given** a registered Spectator, **then** the persisted `role = 'spectator'` is this story's entire contribution to "excluded from question dispatch" and "included in final-results" — no dispatch code (`wa/dispatch.go` question delivery, story 3.2) or results code (story 3.9) exists yet to filter or include by role. Nothing in this story is verified against those future consumers beyond the column value being correct. *(epic AC-2, verified by construction — not executable this story)*

## Tasks / Subtasks

- [x] **Task 1: `game` package — replace the `ErrGameStarted` placeholder with real state-aware registration** (AC: 1, 2, 3)
  - [x] `server/internal/game/participants.go`: **remove** `ErrGameStarted` (story 2.4's documented placeholder — its doc comment literally says "Story 2.5 turns this into spectator registration"; this story is that story, so the placeholder is deleted, not left dead). Add `var ErrGameFinished = errors.New("game: already finished")` — the code is valid but the game already ended; maps to the Help reply, never spectator registration (AC 2).
  - [x] Add `JoinSpectator JoinOutcome = "spectator"` to the `JoinOutcome` const block (alongside `JoinWelcome`, `JoinPreLobby`).
  - [x] Extract the display-name resolution (`strings.TrimSpace(profileName)`, falling back to `fallbackParticipantName(phone)` when empty) out of `joinLobby` into a small helper: `func resolveDisplayName(profileName, phone string) string`. `joinLobby` and the new `joinSpectator` need the identical three lines — centralizing avoids the two paths drifting on this locked, documented fallback rule.
  - [x] Rewrite `Join`'s switch to three explicit cases plus a narrowed default:
    ```go
    switch g.State {
    case StateDraft:
        return JoinResult{Outcome: JoinPreLobby, GameID: g.ID}, nil
    case StateLobby:
        return e.joinLobby(ctx, g, phone, profileName)
    case StateFinished:
        return JoinResult{}, ErrGameFinished
    default:
        return e.joinSpectator(ctx, g, phone, profileName)
    }
    ```
    `default` now covers exactly `question_open`, `question_closed`, `revealed`, `leaderboard` (the four states between `lobby` and `finished`) — matches EXPERIENCE.md's "Question open" + "Between questions²" columns.
  - [x] Add `joinSpectator`, structurally parallel to `joinLobby` but **without** a snapshot/broadcast:
    ```go
    func (e *Engine) joinSpectator(ctx context.Context, g gen.Game, phone, profileName string) (JoinResult, error) {
        displayName := resolveDisplayName(profileName, phone)
        p, created, err := e.store.CreateParticipant(ctx, g.ID, phone, displayName, RoleSpectator)
        if err != nil {
            return JoinResult{}, err
        }
        return JoinResult{Outcome: JoinSpectator, GameID: g.ID, DisplayName: p.DisplayName, Created: created}, nil
    }
    ```
    No `store` or `engine.go` changes needed: `CreateParticipant` already takes `role` as a plain parameter (story 2.4 built it generically), and `Store` interface / `stubStore` already expose it — this story is pure consumption of an existing surface.
  - [x] `server/internal/game/participants_test.go`: delete `TestJoinGameAlreadyStartedReturnsErrGameStartedWithoutWriting` (the case it tested no longer exists — those states now register a spectator). Add, reusing `joinStub`'s shape:
    - `TestJoinDuringLiveStatesRegistersSpectator` — table-driven over `{StateQuestionOpen, StateQuestionClosed, StateRevealed, StateLeaderboard}`: `Outcome == JoinSpectator`, `Created == true`, `DisplayName` from `profileName`, `Snapshot.GameID == ""` (no broadcast — the whole point of AC 1's second half), `CreateParticipant` called once with `RoleSpectator` (assert via `st.createParticipantRole`, the field story 2.4 already added).
    - `TestJoinSpectatorEmptyProfileNameFallsBackToPhoneDigits` — mirrors `TestJoinFromLobbyEmptyProfileNameFallsBackToPhoneDigits` for the spectator path.
    - `TestJoinSpectatorIdempotentRepeat` — `st.createParticipantCreated = false`, distinct stored name on the stub (same pattern as `TestJoinFromLobbyIdempotentRepeatSkipsBroadcast`): `Created == false`, `DisplayName` is the *existing* stored name not the resent one, `Snapshot.GameID == ""`.
    - `TestJoinFinishedGameReturnsErrGameFinishedWithoutWriting` — `errors.Is(err, ErrGameFinished)`, `st.createParticipantCalls == 0`.

- [x] **Task 2: `wa` package — Spectator-notice template, Game-finished routing, remove the dead `ErrGameStarted` branch** (AC: 1, 2, 3)
  - [x] `server/internal/wa/messages_he.go`: update the header comment's row-ownership map — move `Spectator notice -> story 2.5` off the pending list into the implemented list (same housekeeping story 2.4 did for its four rows). Add, following the `preLobbyMessage` pattern exactly (no placeholders — EXPERIENCE.md's Spectator-notice copy has none):
    ```go
    // msgSpectatorNoticeReply is sent when a valid JOIN arrives while the game
    // is already running — question_open through leaderboard (EXPERIENCE.md's
    // "Question open"/"Between questions²" columns). A finished game gets Help
    // instead (story 2.5 AC 2) — this function is never called for one.
    const msgSpectatorNoticeReply = "המשחק כבר התחיל! נרשמת כצופה — התוצאות יגיעו לכאן בסוף המשחק 🏆"

    // spectatorNoticeMessage returns the Spectator-notice copy.
    func spectatorNoticeMessage() string {
        return msgSpectatorNoticeReply
    }
    ```
  - [x] `server/internal/wa/messages_he_test.go`: add `canonicalSpectatorNoticeCopy` (verbatim, held alongside the other `canonical*Copy` constants) and `TestSpectatorNoticeMessageMatchesCanonicalCopy`, mirroring `TestPreLobbyMessageMatchesCanonicalCopy` (direct equality — no isolation to test, since there's no dynamic placeholder).
  - [x] `server/internal/wa/inbound.go` — `handleJoin`'s `switch result.Outcome` block gains one case: `case game.JoinSpectator: reply = spectatorNoticeMessage()`. The existing `if result.Created && result.Snapshot.GameID != ""` broadcast guard needs **no change** — `joinSpectator` never populates `Snapshot`, so it already naturally skips.
  - [x] Same function's outer `switch { case err == nil: ... }`: **delete** the `case errors.Is(err, game.ErrGameStarted):` arm entirely (dead — `Join` never returns that error anymore). Add in its place: `case errors.Is(err, game.ErrGameFinished): r.replier.Enqueue(msg.From, helpMessage())` + `r.logger.Info("join rejected, game already finished", "phone_last4", PhoneLast4(msg.From), "wa_message_id", WaMessageIDDigest(msg.WaMessageID))` — INFO, not WARN: this is an expected, documented reply path (AC 2), not a failure, same reasoning story 2.4 applied to the (now-removed) `ErrGameStarted` branch.
  - [x] `server/internal/wa/inbound_test.go`: delete `TestHandleJoinGameStartedSendsHelp` (the error it exercises no longer exists). Add:
    - `TestHandleJoinSpectatorSendsSpectatorNotice` — `stubRegistrar{joinResult: game.JoinResult{Outcome: game.JoinSpectator, GameID: "game-1", DisplayName: "דנה", Created: true}}` (no `Snapshot`): assert reply `== spectatorNoticeMessage()` and `len(broadcaster.calls) == 0`.
    - `TestHandleJoinFinishedGameSendsHelp` — `stubRegistrar{joinErr: game.ErrGameFinished}`: assert reply `== helpMessage()` and the log does **not** contain `level=WARN` (mirrors `TestHandleJoinGameStartedSendsHelp`'s assertion style, now retargeted).
  - [x] No `main.go` change — `Registrar`'s `Join` signature is unchanged; only its return values' meaning shifted.

- [x] **Task 3: Quality gates + local E2E** (all ACs)
  - [x] Local gates: `gofmt -l .` (LF-normalize first — Windows/CRLF false-positive, 2.2/2.3/2.4's documented workaround) · `go vet ./...` · `go test ./...` (zero regressions — `game` and `wa` package tests changed the most) · `sqlc generate` empty diff (expected — no query changes this story) · the Hebrew copy-centralization grep (the one new template lives in `messages_he.go` only). `make lint` + `make test` cover most of this. No frontend changes this story (confirmed: no `participants.role` reference anywhere under `web/src`, grepped during story creation) — skip `tsc`/`eslint`/`npm run build` review beyond confirming they still pass untouched.
  - [x] **Local E2E** (Go, `cmd/e2escratch`, deleted after use — same pattern as stories 2.3/2.4): boot the real server binary against the Docker dev Postgres with a fake WhatsApp HTTP server capturing outbound sends. **Known ceiling, same as 2.4's**: no REST path exists yet to move a game past `lobby` (story 3.1's control actions aren't built) — this story's `question_open`/`finished`-state paths are **unreachable from a live HTTP flow** and can only be exercised through the Go unit tests added in Task 1/2. Scope the E2E to what a real webhook flow *can* reach today: log in as `e2e-org-a` → create a scratch game → `POST /open-lobby` → signed webhook `JOIN <code>` (still gets Welcome — regression check that Task 1's switch rewrite didn't disturb the `lobby`/`draft` branches) → assert the fake WhatsApp server captured the Welcome copy, unchanged from 2.4. Note the spectator/finished paths as a manual follow-up once story 3.1 lands (verbatim continuation of 2.4's own E2E note for the same limitation).
  - [x] No real-phone `make dev` pass is required for this story — unlike 2.4, there is still no way to advance a real game past `lobby`, so a real-phone spectator test is equally blocked until story 3.1. Skip it; do not invent a workaround.

### Review Findings

- [x] [Review][Patch] `Join`'s default arm was a fail-open write path for unknown game states — [server/internal/game/participants.go:74-86] the switch enumerated draft/lobby/finished and left `default` routing to `joinSpectator`, so a future or drifted state would silently create a participant row and send "the game already started" copy. User decision: narrow the switch to the four explicit live states (`StateQuestionOpen, StateQuestionClosed, StateRevealed, StateLeaderboard`) and make `default` a fail-closed `fmt.Errorf("game: unexpected state %q", ...)` — wa's existing unexpected-error WARN+Help arm handles it with no new wa code. Fixed: participants.go's switch gained the explicit case + narrowed default; added `TestJoinUnknownStateFailsClosedWithoutWriting` covering a synthetic `"cancelled"` state. `go1.26.5 build/vet/test ./...` and `gofmt -l .` (CRLF-normalized) clean. Raised independently by all three review layers. (blind+edge+auditor)
- [x] [Review][Defer] A lobby-joined Player who re-sends JOIN mid-game is told they are a spectator — [server/internal/game/participants.go:126-135, server/internal/wa/inbound.go:202-203] `joinSpectator` ignores the refetched row's existing `role`, so a `role='player'` row hit via `ON CONFLICT DO NOTHING` still returns `Outcome: JoinSpectator` and the "נרשמת כצופה" reply — misleading copy for an active Player (no data corrupted, `role` stays `'player'`). Deferred — spec-anticipated ("Known edge case, not specially handled"), unreachable until story 3.1 moves a game past `lobby`; logged in deferred-work.md per the spec's own triage instruction. (blind+edge)
- [x] [Review][Defer] State-read→INSERT race can register a spectator into a game that just finished — [server/internal/game/participants.go:67-83] a `leaderboard`→`finished` transition landing between `GetGameByJoinCode` and `CreateParticipant` writes a spectator row into a finished game (the exact combination AC 2 forbids) whose results dispatch already fired. Deferred — same TOCTOU class as deferred-work.md's 2.4 entry, which already names "2.5's player-vs-spectator boundary"; still unreachable until story 3.1, fold into that entry's 3.1 concurrency-posture decision. (blind+edge)
- [x] [Review][Defer] New spectator row hijacks the cross-game `שם:` rename from the sender's live player row — [server/internal/game/participants.go:137-151, server/internal/store/queries/participants.sql `UpdateParticipantNameByPhone`] a Player in game A who JOINs already-started game B gains a newer spectator row in B; the role-blind most-recent-row rename then updates B's spectator row while A's live surfaces keep the stale name and the reply claims "עודכן ✓". The 2.4 rename-scoping decision predates mid-game JOINs creating rows at all. Deferred — unreachable until story 3.1; new deferred-work.md entry added. (blind+edge)
- [x] [Review][Defer] Spectator rows silently inflate the role-blind snapshot roster/count — [server/internal/game/engine.go buildSnapshot → server/internal/ws/handler.go WS connect] `buildSnapshot` lists participants without a role filter and `ParticipantSummary` carries no role field, so any live-state snapshot served on a WS (re)connect counts spectators indistinguishably from players. Deferred — no live-state consumer exists today (spec-documented); revisit when story 3.1's control panel or Epic 4's displays render live rosters: filter `role='player'` or add role to `ParticipantSummary`. (edge)

## Dev Notes

### The central judgment call — epics.md's phrasing vs. EXPERIENCE.md's precise table

epics.md's Story 2.5 text says a Spectator is anyone who joins in "any state past `lobby`" — read literally, that includes `finished`. **This is imprecise, and this story does not implement it literally.** EXPERIENCE.md's WhatsApp conversation-grammar matrix (the file epics.md itself designates as authoritative for "experience and behavior on all three surfaces," per its own OQ-7 resolution note) has five separate per-state columns for `JOIN <valid>`, and "Game over" (`finished`) is a **distinct column from** "Question open" and "Between questions²" — with a **different** reply: Help, not Spectator notice.

```
| Inbound       | Pre-lobby (draft) | Lobby    | Question open    | Between questions² | Game over |
| JOIN <valid>  | Pre-lobby reply   | Welcome  | Spectator notice | Spectator notice   | Help      |
```
(² "Covers the closed, revealed, and leaderboard states.")

The product reason this matters, not just textual pedantry: the Spectator-notice copy promises "התוצאות יגיעו לכאן בסוף המשחק" (results will arrive here at game end). For someone joining a game that has **already** finished, that promise is false — the `GameFinished` event that triggers the final-results dispatch (story 3.9, not yet built) has already fired once, for the people who were actually registered at the time, and will never fire again. Registering a post-finish "spectator" would create a participant row that receives no message, ever — silently broken, not silently correct. Replying Help (the same reply a finished game already gives to every other unrecognized-in-context message per that column) is both spec-accurate and honest.

**This is the single most important thing to get right in this story** — a developer who implements the epics.md summary literally (spectator for *any* non-lobby, non-draft state, `finished` included) produces working code that passes a shallow reading of the epics AC text but is wrong per the canonical UX behavior spec and ships a broken promise. AC 2 and Task 1's `StateFinished` case exist specifically to prevent that.

### Spectator join never broadcasts a snapshot — by design, not a degrade

Unlike `joinLobby`, `joinSpectator` does not call `buildSnapshot` at all — it isn't skipping a broadcast the way story 2.4's post-insert-failure path does, it never attempts one. Two independent reasons: (1) nothing consumes `role` in `Snapshot`/`ParticipantSummary` today — `game/snapshot.go`'s own comment on `ParticipantSummary` says "no role field yet — nothing consumes it until Stories 2.4/2.5," and this story still doesn't need to add one, since nothing renders a spectator count anywhere. (2) The only live-state consumer that exists is the lobby page (story 2.3), which is only meaningful while a game is actually in `lobby` — by construction, a spectator join only happens in a state *after* `lobby`, so there is no live surface to inform. Building `Snapshot` and broadcasting it would be speculative work for a consumer that doesn't exist (Epic 3's control panel, story 3.1, or a future spectator-count feature) — contradicts "don't design for hypothetical future requirements." `[ASSUMPTION]`

### Known edge case, not specially handled, unreachable today

A Participant who joined during `lobby` (`role = 'player'`) and later re-sends `JOIN` after the game has started would route into `joinSpectator`, hit `CreateParticipant`'s `ON CONFLICT (game_id, phone) DO NOTHING` (their row already exists), and get back their **existing** row unchanged (`role` stays `'player'` — the conflict path never overwrites). The reply they receive would still be the Spectator-notice text, which is misleading for someone who is actually still a full Player — but no data is corrupted, and EXPERIENCE.md's grammar table makes no distinction for this sub-case (every valid JOIN in these columns maps to "Spectator notice" uniformly). **Unreachable today**: no code path moves a game past `lobby` until story 3.1's control actions exist, so no Player row can exist at the point this would matter. Do not build special-case detection (e.g., an extra `GetParticipantByPhone` lookup before replying) for this story — flag it for the code-review triage session if it seems worth a `deferred-work.md` entry once 3.1 makes it reachable, same class as the other "unreachable until 3.1" items already logged there (see below).

### Existing code this story modifies — current state, and what must survive

- **[server/internal/game/participants.go](server/internal/game/participants.go)** — currently: `Join`'s switch has three arms (`StateDraft` → `JoinPreLobby`, `StateLobby` → `joinLobby`, `default` → `ErrGameStarted`). `ErrGameStarted`'s doc comment literally names this story as its replacement. Must survive unchanged: `joinLobby`'s full body (idempotency, the post-commit snapshot-degrade-to-skip logic, its own doc comment) — this story only factors its display-name resolution into a shared helper, does not change its behavior.
- **[server/internal/wa/inbound.go](server/internal/wa/inbound.go)** — `handleJoin`'s `switch result.Outcome` currently has two cases (`JoinWelcome`, `JoinPreLobby`) plus a belt-and-braces default; the outer `switch { case err == nil / errors.Is(...) / default }` currently has four arms including the `ErrGameStarted` one this story deletes. Must survive unchanged: the `ErrInvalidJoinCode` and unexpected-error (WARN) arms, the broadcast-before-reply ordering, every `phone_last4`/`wa_message_id` redaction in every log line.
- **[server/internal/wa/messages_he.go](server/internal/wa/messages_he.go)** — the header comment's row-ownership map already lists `Spectator notice -> story 2.5`; this story is what fulfills that line. Must survive: `ltr()`, the three existing templates, `renamePrefix`, all unchanged.
- **[server/internal/game/engine.go](server/internal/game/engine.go)**, **[server/internal/game/engine_test.go](server/internal/game/engine_test.go)** — **no changes**. The `Store` interface's `CreateParticipant(ctx, gameID, phone, displayName, role string)` already takes `role` generically (story 2.4 built it that way on purpose), and `stubStore` already records `createParticipantRole`. This story is pure consumption of an existing, already-generic surface — if a change here seems necessary, that's a signal the design has drifted from the plan above; re-check before adding one.
- **[server/internal/store/*](server/internal/store)**, **[server/internal/store/queries/participants.sql](server/internal/store/queries/participants.sql)** — **no changes**. `CreateParticipant`'s `ON CONFLICT (game_id, phone) DO NOTHING` + refetch is already role-agnostic; the `participants.role` CHECK constraint (migration `00008`) already allows `'spectator'` (added in story 2.3, used for the first time by this story).
- **[server/cmd/server/main.go](server/cmd/server/main.go)**, **web/*** — **no changes**. `Registrar.Join`'s signature is untouched (only the meaning of its existing return values grew); no `participants.role` reference exists anywhere under `web/src` (grepped during story creation) — the dashboard has no spectator-facing UI to build or break.

### Previous story intelligence (2.4 — established patterns, reused verbatim)

- **Encoding discipline (Windows tax):** write new/edited files with the Write tool only, never PowerShell redirection (emits UTF-16 + BOM) — relevant here for the one new Hebrew template.
- **`gofmt -l .` false-positives on Windows** (`core.autocrlf=true`) — LF-normalize before piping through `gofmt`.
- **Go 1.26.5 via the `go1.26.5` wrapper** (system `go` is older).
- **`go test -race` is not available in this environment.**
- **Captured-slog testing is the house pattern** (`slog.New(slog.NewTextHandler(&buf, nil))`, `wa`'s `newTestLogger()` helper) — reuse for the new `ErrGameFinished` → Help INFO-not-WARN assertion, same style as 2.4's `ErrGameStarted` test it replaces.
- **Dev DB carries `e2e-org-a`/`e2e-org-b`** test organizers — reusable for Task 3's E2E; clean up scratch rows afterward.
- **This story is much smaller in surface area than 2.1–2.4** (no new store queries, no `main.go` change, no new package) — do not expand scope to match their review-load pattern; the two packages touched (`game`, `wa`) are both narrow, additive changes to code those exact stories already built for this purpose.
- **2.4's Dev Notes explicitly named this story as `ErrGameStarted`'s successor** — confirmed consistent with this story's Task 1.
- **`deferred-work.md`'s 2.4 entry on the state-check→INSERT TOCTOU** (`server/internal/game/participants.go:66-84`) says: "also relevant to 2.5's player-vs-spectator boundary." Status unchanged by this story — still unreachable until story 3.1 (no code path leaves `lobby` yet), so `joinSpectator`'s `CreateParticipant` call carries the same pre-existing, already-logged gap. Nothing new to add there this story.

### Architecture guardrails (violations = rework)

- **Dependency direction is law**: `wa` → `game` → `store`, one way, unchanged by this story (no new import edges).
- **State mutation is centralized**: `joinSpectator` writes through `game.Engine` → `store` only, exactly like `joinLobby`.
- **Glossary**: `Participant`, `Game`, `Organizer` verbatim; `role` values `'player'`/`'spectator'` are the epics-pinned exception (already established, story 2.3's migration) — this story is the first to actually write `'spectator'`.
- **Logging (NFR-8)**: INFO for the Spectator-notice reply and the Game-finished→Help reply (both expected paths); WARN reserved for genuinely unexpected store errors (unchanged from 2.4's pattern — `joinSpectator`'s own store-error path propagates to `handleJoin`'s existing unexpected-error WARN arm with no new code needed there).
- **Copy centralization**: the one new Hebrew template lives in `messages_he.go` only — CI's Hebrew-literal grep enforces this on `.go` files.
- **Validate at boundaries**: unaffected by this story — `wa` already normalizes the JOIN code before calling `game.Join`; this story adds no new boundary.

### Project Structure Notes

**New:** nothing (no new files).

**Modified (backend):** `server/internal/game/participants.go` (`ErrGameStarted` → `ErrGameFinished`, `JoinSpectator` outcome, `resolveDisplayName` helper, `joinSpectator`, three-arm→four-arm state switch) · `server/internal/game/participants_test.go` (delete the `ErrGameStarted` test, add four new ones) · `server/internal/wa/messages_he.go` (+1 template) · `server/internal/wa/messages_he_test.go` (+1 test) · `server/internal/wa/inbound.go` (`handleJoin`'s two switches gain/lose one arm each) · `server/internal/wa/inbound_test.go` (delete the `ErrGameStarted` test, add two new ones).

**Untouched:** everything under `web/` · `server/internal/httpapi/*` · `server/internal/ws/*` · `server/internal/store/*` (including `queries/`, `gen/`) · `server/internal/auth/*` · `server/internal/game/engine.go` + `engine_test.go` · `server/cmd/server/main.go` · all Epic 1 authoring surfaces · Epic 3/4 packages (don't exist yet).

### Testing standards

Go stdlib `testing`, co-located `_test.go`, stubs grown in-test (no mock framework, reuse `stubStore`/`stubRegistrar`/`stubBroadcaster` as-is — no new stub fields needed since `role` plumbing already exists), `*slog.Logger` injected where log assertions matter. No real-DB tests in CI — Task 3's E2E is the DB/WhatsApp-touching verification, scoped down from 2.4's per the "no path past lobby yet" ceiling explained there. No frontend test framework exists yet and this story needs none.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-2.5] — story + all 3 epic ACs verbatim, Epic 2 context
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#WhatsApp-conversation-grammar] — the (state × message class) grammar matrix; the authoritative source for AC 2's `finished`-state narrowing, including footnote ² defining "Between questions"
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#WhatsApp-message-templates] — canonical Spectator-notice copy, verbatim, no placeholders
- [Source: _bmad-output/planning-artifacts/architecture.md#Component-Boundaries-Go] — dependency direction `wa`→`game`→`store`; FR-3's architectural home ("`game` registry with participant role; final-results dispatch includes spectators")
- [Source: server/internal/game/participants.go, state.go] — existing `Join`/`joinLobby`, the already-defined `RoleSpectator` constant (added in 2.4, unused until now) this story finally exercises
- [Source: server/internal/game/snapshot.go] — `ParticipantSummary`'s "no role field yet" comment, confirming no broadcast consumer exists
- [Source: server/internal/store/participants.go, queries/participants.sql] — `CreateParticipant`'s existing role-agnostic `ON CONFLICT DO NOTHING` + refetch, reused unchanged
- [Source: server/internal/wa/inbound.go, messages_he.go] — existing `handleJoin`/template machinery this story extends
- [Source: _bmad-output/implementation-artifacts/2-4-join-the-game-with-one-whatsapp-message.md] — `ErrGameStarted`'s explicit "story 2.5 replaces this" note; the `joinLobby` degrade/idempotency precedents this story's `joinSpectator` follows; the E2E "no path past lobby yet" ceiling, unchanged and re-applied here
- [Source: _bmad-output/implementation-artifacts/deferred-work.md#2-4] — the state-check→INSERT TOCTOU item explicitly naming "2.5's player-vs-spectator boundary" as relevant; confirmed still unreachable, no action needed this story

## Dev Agent Record

### Agent Model Used

claude-sonnet-5 (Claude Code)

### Debug Log References

- Local gates: `go1.26.5 build ./...`, `go1.26.5 vet ./...`, `go1.26.5 test ./...` all clean after Tasks 1 and 2. `gofmt -l .` flagged the same 9 pre-existing files stories 2.2/2.3/2.4 already documented as Windows/CRLF false positives (`internal/store/wa_messages.go`, `internal/wa/{client,client_test,dispatch,dispatch_test,redact,redact_test,webhook,webhook_test}.go`); re-checked each with `tr -d '\r'` piped through `gofmt -l` and confirmed zero real diffs, none of them touched by this story. All 6 files this story touched pass `gofmt -l` cleanly after LF-normalizing. One real gofmt issue was self-inflicted and fixed: adding `canonicalSpectatorNoticeCopy` to `messages_he_test.go`'s const block broke column alignment on the other rows — corrected.
- `sqlc generate` produced an empty `git status` diff on `internal/store/gen/` — confirmed no query changes this story.
- Hebrew copy-centralization check (CI's `.github/workflows/ci.yml` grep, `[\x{0590}-\x{05FF}\x{FB1D}-\x{FB4F}]` over `*.go` excluding `_test.go` and `messages_he.go`) run locally via a Python port (Git Bash's `grep -P` doesn't support the codepoint-range syntax on this machine) — clean, no Hebrew literals leaked into `inbound.go` or `participants.go`.
- `make lint`'s `go vet`/`gofmt` steps ran as documented above; `npm run lint` and `npx tsc -b --noEmit` under `web/` both passed with zero output — confirms frontend is genuinely untouched by this story.
- Local E2E: built a throwaway `server/cmd/e2escratch/main.go` (deleted after the run, per the 2.3/2.4 pattern) that resets `e2e-org-a`'s password via `store`/`auth` directly (in-process equivalent of `cmd/provision`, existing sessions invalidated), boots the real server binary against the Docker dev Postgres (`whatsapp-clickers-pg` container) with `WHATSAPP_API_BASE_URL` pointed at an in-process fake WhatsApp HTTP server, logs in over HTTP, creates a scratch game, `POST /open-lobby`, then POSTs a correctly-HMAC-signed webhook `JOIN <code>` for a synthetic phone. The fake server captured exactly the Welcome copy (`"היי ⁦בדיקה⁩, נרשמת! 🎉 ..."`) — confirming Task 1's three-arm-plus-default switch rewrite still routes `StateLobby` through `joinLobby` unchanged. Server stdout/stderr contained zero `WARN`/`ERROR` lines for the run. Scratch game row (and its cascaded participant row) deleted afterward via a direct `DELETE FROM games WHERE id = $1`; verified 0 rows remaining with `docker exec whatsapp-clickers-pg psql ... SELECT ... WHERE title LIKE 'E2E Scratch%'`. As scoped by the story, the `question_open`/`finished`-state paths were **not** exercised in this E2E — no REST path leaves `lobby` until story 3.1's control actions exist; those paths are covered by the four new `game` package unit tests and the two new `wa` package unit tests instead. No real-phone `make dev` pass was performed — same reasoning, skipped per the story's own instruction.

### Completion Notes List

- Task 1: `ErrGameStarted` (story 2.4's documented placeholder) removed; replaced with `ErrGameFinished`. `Join`'s switch now has four explicit arms (`draft`/`lobby`/`finished`/default-spectator). Added `resolveDisplayName` shared by `joinLobby` and the new `joinSpectator`, which registers `role='spectator'` and — by design, not a degrade — never builds or broadcasts a `Snapshot`. Old `TestJoinGameAlreadyStartedReturnsErrGameStartedWithoutWriting` replaced with four tests: table-driven spectator registration across all four live states, the empty-profile-name fallback, idempotent-repeat, and the `finished`→`ErrGameFinished` case. `go1.26.5 test ./internal/game/...` green.
- Task 2: added the Spectator-notice Hebrew template (`msgSpectatorNoticeReply`/`spectatorNoticeMessage`) to `messages_he.go`, verified verbatim against EXPERIENCE.md's templates table, with a canonical-copy test. `inbound.go`'s `handleJoin` gained a `game.JoinSpectator` reply-outcome case and swapped its dead `ErrGameStarted` arm for `ErrGameFinished` → Help at INFO (not WARN — an expected, documented reply path per AC 2, same reasoning 2.4 used for the branch it replaces). Old `TestHandleJoinGameStartedSendsHelp` replaced with `TestHandleJoinSpectatorSendsSpectatorNotice` and `TestHandleJoinFinishedGameSendsHelp`. No `main.go` change needed — `Registrar.Join`'s signature was already generic. `go1.26.5 test ./internal/wa/...` green.
- Task 3: all local quality gates pass (see Debug Log). Local E2E regression-checked the `lobby`/`draft` branches over a real HTTP + signed-webhook flow; the two new live-state paths (spectator registration, finished-game rejection) are unreachable from a real HTTP flow until story 3.1 and are covered by unit tests only, exactly as the story scoped.
- All 4 acceptance criteria satisfied: AC 1 (spectator registration across the four live states, no broadcast) and AC 3 (idempotent repeat) verified by `TestJoinDuringLiveStatesRegistersSpectator`/`TestJoinSpectatorIdempotentRepeat` plus the `wa`-level `TestHandleJoinSpectatorSendsSpectatorNotice`; AC 2 (finished game → Help, no participant row) verified by `TestJoinFinishedGameReturnsErrGameFinishedWithoutWriting`/`TestHandleJoinFinishedGameSendsHelp`; AC 4 (role persisted, not yet consumed by dispatch/results) verified by construction — `RoleSpectator` is written through the existing generic `CreateParticipant` surface, nothing further to test this story per the Dev Notes.

### File List

- `server/internal/game/participants.go` (modified)
- `server/internal/game/participants_test.go` (modified)
- `server/internal/wa/messages_he.go` (modified)
- `server/internal/wa/messages_he_test.go` (modified)
- `server/internal/wa/inbound.go` (modified)
- `server/internal/wa/inbound_test.go` (modified)

## Change Log

- 2026-08-03: Story created by create-story workflow — full-context analysis: epics 2.5 + Epic 2 cross-story context (2.1's dedupe, 2.3's `participants.role` column + ws hub, 2.4's `Join`/`joinLobby`/`ErrGameStarted` placeholder and its explicit "2.5 replaces this" note), FR-3, architecture's dependency direction + centralized-copy rules, EXPERIENCE.md's full conversation-grammar matrix (surfacing and resolving a real discrepancy against epics.md's coarser "any state past lobby" phrasing for the `finished` state — see Dev Notes), a live read of `participants.go`, `inbound.go`, `messages_he.go`, `engine.go`, `state.go`, `snapshot.go`, `store/participants.go`, `store/queries/participants.sql`, `engine_test.go`, `participants_test.go`, `inbound_test.go`, `messages_he_test.go`, migration `00008_participants.sql`, and `deferred-work.md`'s 2.4 entries; confirmed via grep that `CreateParticipant`/the `role` CHECK/`RoleSpectator` already exist generically from 2.3/2.4 (this story is pure consumption, no store/engine-interface changes) and that no frontend file references `participants.role`. Status: ready-for-dev.
- 2026-08-03: Story implemented — Task 1 (`game` package: `ErrGameFinished` replaces `ErrGameStarted`, `joinSpectator`, `resolveDisplayName`, four-arm `Join` switch), Task 2 (`wa` package: Spectator-notice template, `handleJoin` routing for `JoinSpectator`/`ErrGameFinished`), Task 3 (all local quality gates green; local E2E via a throwaway `cmd/e2escratch` regression-checked the `lobby`/`draft` branches against a real HTTP + signed-webhook flow). Zero regressions across the full `go test ./...` suite. Status: review.
- 2026-08-03: Code review (bmad-code-review) — Blind Hunter, Edge Case Hunter, and Acceptance Auditor run in parallel. 1 decision-needed, 4 defer, several dismissed as spec-sanctioned or non-issues. Decision: narrow `Join`'s `default` arm from a fail-open spectator-write catch-all to an explicit 4-state list (`StateQuestionOpen/QuestionClosed/Revealed/Leaderboard`) plus a fail-closed unexpected-state error — patched in `participants.go`, covered by new `TestJoinUnknownStateFailsClosedWithoutWriting`. The 4 deferred findings (player-mis-labeled-as-spectator on re-JOIN, finished-game race, cross-game rename hijack, role-blind snapshot count) are all pre-existing-class TOCTOU/role gaps unreachable until story 3.1 moves a game past `lobby` — logged in `deferred-work.md`. `go1.26.5 build/vet/test ./...` and `gofmt -l .` clean after the patch. Status: done.
