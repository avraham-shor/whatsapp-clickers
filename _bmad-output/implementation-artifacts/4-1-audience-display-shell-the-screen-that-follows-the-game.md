---
baseline_commit: 527f1edbd88f92a4f1a2107da87c0726383e124d
---

# Story 4.1: Audience Display Shell — the Screen That Follows the Game

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an Organizer,
I want to open the Audience Display in a separate window that follows the Game live,
so that the projector always shows the current state with zero interaction (FR-9).

## ⚠️ Prerequisite: branch from 3.10's work, not from `main`

At the time this story was created, **story 3.10 (Post-Game Summary on the Dashboard) is implemented and reviewed but NOT committed** — its work sits uncommitted on branch `story/3-10-post-game-summary-on-the-dashboard`, and `main` is at `527f1ed` (story 3.9's merge, PR #7). The `baseline_commit` above is that `main` tip because 3.10 has no commit yet; it is **not** the tree you should branch from.

**Branch from wherever 3.10's work actually sits** when you start — its commit on `story/3-10-post-game-summary-on-the-dashboard`, or `main` once 3.10 has merged. This story edits five files 3.10 also edited (`web/src/app.tsx`, `web/src/lib/types.ts`, `web/src/lib/strings.he.ts`, `web/src/features/live/control-page.tsx`, `server/internal/httpapi/router.go`) — branching from `main` while 3.10 is uncommitted guarantees conflicts in all five.

Verify these shapes on disk before writing any code (they are 3.10's/3.9's/3.1's landed state):

- `server/internal/httpapi/router.go` — `NewRouter` takes **11** positional args, ending `dispatcher QuestionDispatcher, resultDispatcher ResultDispatcher, finalDispatcher FinalDispatcher`. **This story does not change that arity.**
- `server/internal/httpapi/control.go` — `ControlEngine` interface with **9** methods, ending `ResultsForFinishedGame`; `SnapshotBroadcaster` with the single `Broadcast(gameID string, snapshot game.Snapshot)`.
- `server/internal/game/engine.go` — `Store` interface with **22** methods, ending `UpdateAnswerGrade`; `OpenLobby` at ~line 189 ending `return e.snapshotAfterCommit(ctx, g), nil`; `buildSnapshot` and `emptySnapshot` at the bottom of the file.
- `server/internal/ws/handler.go` — the `role` query param is already validated as `host`/`display` and a `role=display` handshake already succeeds. **You are not adding WS support; you are adding its first consumer.**
- `web/src/lib/use-game-socket.ts` — `useGameSocket(gameId, role: 'host' | 'display')` already accepts `'display'`. **Do not modify this file** (see Dev Notes, "The socket hook is already correct").
- `web/src/app.tsx` — four authenticated child routes under `DashboardLayout` (`index`, `games/:gameId`, `games/:gameId/lobby`, `games/:gameId/results`), plus `/login` and the `*` catch-all.
- `web/src/features/live/control-page.tsx` — the `snapshot.state === 'finished'` early-return branch rendering `strings.live.gameOverTitle` + `<ResultsSummary>` + a back link.
- `web/src/index.css` — the `@theme` block with the Festival Green tokens (`--color-green-800`, `--color-surface-base`, `--color-gold`, …), the radius/spacing scales, and the four font weights. Gold is defined and, as of 3.10, still applied nowhere.

If 3.10 has since merged or changed further, re-confirm the quoted signatures are still accurate before relying on them.

## Acceptance Criteria

1. **Given** an authenticated dashboard session, **when** the Organizer launches the Audience Display, **then** a separate browser window opens at `/display/:gameId` — same-origin, inheriting the session cookie, no pairing flow — full-screen, output-only, `dir="rtl"`. *(epic AC-1)*
2. **Given** the display connects to `/ws?gameId=…&role=display`, **then** it renders the current state entirely from the first snapshot (no REST query is the data source), and a stage-switcher renders one component per game state (UX-DR16). *(epic AC-2)*
3. **Given** an Organizer action, **then** the display transitions to the new state within 1 second (NFR-1). *(epic AC-3)*
4. **Given** a dropped connection, **then** it auto-reconnects with exponential backoff and re-renders current state with no Organizer intervention; the connecting copy appears **only** while the socket is down (UX-DR12), **and** the surface uses the Festival Green palette and the system-ui stack with zero external asset requests (UX-DR1, UX-DR3, UX-DR4). *(epic AC-4)*
5. **Given** the dashboard's "הפחת אנימציות" toggle (EXPERIENCE.md Display controls), **when** the Organizer enables it, **then** the Audience Display renders the static motion equivalents for the whole room — the setting rides the game snapshot (`displaySettings` field), requiring no display interaction, because the audience cannot set `prefers-reduced-motion` on a projector. *(epic AC-5)*

### Scope boundaries for this story

- **No stage content.** This story builds the *shell*: the route, the socket wiring, the stage-switcher contract, the projection type ramp, the reconnect overlay, the state announcer, and the reduced-motion plumbing. The lobby counter (4.2), the timer ring and options (4.3), the reveal distribution (4.4), the leaderboard (4.5) and the winner takeover (4.6) are **not** built here. Every state maps to one shared placeholder that later stories replace one entry at a time.
- **No gold anywhere.** Gold's two permitted moments are the ≤5s timer (4.3) and the winner (4.6). If a `text-gold`/`bg-gold`/`border-gold` class appears in this story's diff, it is wrong (UX-DR2).
- **No change to `web/src/lib/use-game-socket.ts`.** It already does everything this story needs. See Dev Notes.
- **No change to `server/internal/ws/`.** `role=display` already handshakes and already receives the same snapshot envelope. Not one line.
- **No `wa/` change, no `messages_he.go` change.** Nothing here is participant-facing.
- **No display auth redesign.** `deferred-work.md`'s 2.3 entry points its revisit trigger at this story; the epic AC and the architecture both mandate the session-cookie approach. See Task 12 — you re-triage the entry in writing, you do not build a display token.
- **No Vitest, no frontend test framework.** Every prior frontend story's precedent (1.2 through 3.10). The manual browser pass in Task 11 is the frontend verification.
- **No new npm package and no `shadcn add`.** `components/ui/` is generated and never hand-edited.

## Tasks / Subtasks

- [x] **Task 1: `server/migrations/00016_game_display_settings.sql` (new) — the one column** (AC: 5)

  The display-settings flag must survive a display reconnect, and a reconnect rebuilds the snapshot from Postgres (`Engine.Snapshot` → `buildSnapshot`). An in-memory flag would be silently lost by the exact event AC-4 exists to make invisible, so this is a real column.

  ```sql
  -- +goose Up
  -- Room-level display setting (FR-9, story 4.1). The audience cannot set
  -- prefers-reduced-motion on a projector, so the Organizer sets it for the
  -- room and it rides the game snapshot. Column, not memory: a display
  -- reconnect rebuilds its snapshot from this table, and losing the setting
  -- on reconnect would be the one failure the auto-reconnect exists to hide.
  ALTER TABLE games ADD COLUMN reduced_motion BOOLEAN NOT NULL DEFAULT false;

  -- +goose Down
  ALTER TABLE games DROP COLUMN reduced_motion;
  ```

  - [x] `NOT NULL DEFAULT false` backfills every existing game, exactly as 00004 and 00009 did for their `games` columns — no two-phase migration, no pgtype in the sqlc model.
  - [x] **This Down block is genuinely runnable**, unlike 00011–00013's (see `deferred-work.md`): dropping a boolean column validates nothing. Do not add a CHECK constraint that would change that.
  - [x] Migrations run at boot via goose (`store/migrate.go`); no wiring change in `main.go`.

- [x] **Task 2: `store/queries/games.sql` + `store/games.go` — one query, one wrapper** (AC: 5)

  - [x] Append to `queries/games.sql`, immediately after `UpdateGameScoring` (its nearest sibling — same shape, same ownership posture):
    ```sql
    -- Room-level display settings (FR-9). Deliberately NOT state-guarded,
    -- unlike OpenGameLobby and the question mutations: this is legal in
    -- every state from lobby through finished (EXPERIENCE.md's Display
    -- controls row), so a state predicate here would be wrong, not missing.
    -- name: UpdateGameDisplaySettings :one
    UPDATE games
    SET reduced_motion = $3,
        updated_at = now()
    WHERE id = $1 AND organizer_id = $2
    RETURNING *;
    ```
  - [x] `store/games.go`, appended after `UpdateGameScoring` (keep the wrapper next to its query's neighbour, matching the file's ordering):
    ```go
    // UpdateGameDisplaySettings sets the room-level display settings;
    // ownership is in the WHERE clause, so a foreign game is ErrNotFound
    // like a missing one.
    func (s *Store) UpdateGameDisplaySettings(ctx context.Context, gameID, organizerID string, reducedMotion bool) (gen.Game, error) {
        game, err := s.q.UpdateGameDisplaySettings(ctx, gen.UpdateGameDisplaySettingsParams{ID: gameID, OrganizerID: organizerID, ReducedMotion: reducedMotion})
        if errors.Is(err, pgx.ErrNoRows) {
            return gen.Game{}, ErrNotFound
        }
        return game, err
    }
    ```
  - [x] Run `sqlc generate` (CI-pinned **v1.31.1** — install exactly that version; a newer sqlc formats generated code differently and turns the diff-check red for unrelated reasons). Expect a diff in **`gen/games.sql.go`** (the new query) **and `gen/models.go`** (the new `Game.ReducedMotion` field, which also appends a `&i.ReducedMotion` scan line to every `RETURNING *` / `SELECT *` query on `games`). That models-wide churn is expected here and is the one difference from 3.10's "one file only" gate — a `games` column necessarily touches every generated scan of that table. Commit the generated files.
  - [x] **Record the exact changed-file list in the Debug Log**, and check each entry actually scans `games`. A diff in a file that does not (`answers.sql.go`, `participants.sql.go`, `questions.sql.go`, `packages.sql.go` — none of them return `games.*`) means you touched a query you did not need to. Re-run `sqlc generate` a second time and confirm it is idempotent, as 3.10 did.
  - [x] **No `store/answers.go`, `participants.go`, `questions.go` or `packages.go` change.**

- [x] **Task 3: `game/snapshot.go` — the wire field** (AC: 5)

  - [x] Add the settings type and the `Snapshot` field:
    ```go
    // DisplaySettings carries the room-level rendering settings the Audience
    // Display obeys (FR-9, story 4.1). It rides the snapshot rather than a
    // display-side control because the room shares one projector and nobody
    // interacts with it — the Organizer decides for everyone.
    type DisplaySettings struct {
        ReducedMotion bool `json:"reducedMotion"`
    }
    ```
    and on `Snapshot`, appended after `Leaderboard`:
    ```go
    DisplaySettings DisplaySettings `json:"displaySettings"`
    ```
  - [x] A struct value, not a pointer, and no `omitempty`: `false` is a real, meaningful value and must serialize. The dashboard receives the same field over the same envelope — one payload, both roles, per `Snapshot`'s own doc comment.
  - [x] **Do not** add anything else to `Snapshot`. In particular, do **not** add participant roles (`deferred-work.md`'s open 2.5 item) — this shell renders no roster.

- [x] **Task 4: `game/engine.go` — one `Store` method, one `Engine` method, two snapshot builders** (AC: 5)

  - [x] `Store` interface gains one line, appended after `UpdateAnswerGrade`:
    ```go
    UpdateGameDisplaySettings(ctx context.Context, gameID, organizerID string, reducedMotion bool) (gen.Game, error)
    ```
  - [x] `buildSnapshot` populates the new field from the row it already holds:
    ```go
    DisplaySettings: DisplaySettings{ReducedMotion: g.ReducedMotion},
    ```
  - [x] **`emptySnapshot` populates it too**, from the same `g`. It already carries `State`/`JoinCode`/`PlatformNumber` from the row for exactly this reason: the degraded fallback must not silently flip the room's motion setting back to animated at the worst possible moment. This is the one line in this story that a careless reading skips.
  - [x] New `Engine` method, placed next to `OpenLobby` (read `OpenLobby` first and mirror its shape — store call, then `e.snapshotAfterCommit(ctx, g)`):
    ```go
    // SetDisplaySettings updates the room-level display settings for gameID
    // and returns the resulting snapshot for broadcast.
    //
    // Not a state transition: games.state is neither read as a precondition
    // nor written. It lives on the Engine anyway because the snapshot is the
    // Engine's to build (Enforcement Guidelines: state changes are emitted
    // only through the engine, and this changes what every WS client
    // renders), and because httpapi must not learn to build snapshots.
    //
    // Legal from lobby through finished, and from draft too: a settings
    // write that arrives before the display is ever launched is harmless and
    // guarding it would add a failure mode with no beneficiary.
    func (e *Engine) SetDisplaySettings(ctx context.Context, gameID, organizerID string, reducedMotion bool) (Snapshot, error) {
        g, err := e.store.UpdateGameDisplaySettings(ctx, gameID, organizerID, reducedMotion)
        if err != nil {
            return Snapshot{}, err
        }
        return e.snapshotAfterCommit(ctx, g), nil
    }
    ```
  - [x] **No state machine change.** No new state, no new transition, no change to `OpenLobby`/`StartGame`/`CloseQuestion`/`Reveal`/`NextQuestion`/`StopGame`, no change to `state.go`.
  - [x] `engine_test.go`'s `stubStore` gains the one method (it will not compile otherwise) plus the fields to observe it. Follow whatever convention that stub already uses for its recorded calls — read it before adding.

- [x] **Task 5: `httpapi/display.go` (new) + one line in `control.go` + one route** (AC: 5)

  **🚨 Zero Hebrew characters in every `.go` file this story touches, comments included.** CI greps U+0590–05FF and U+FB1D–FB4F across every non-test `.go` file and exempts only `internal/wa/messages_he.go`; it cannot distinguish a comment from a literal. Story 3.8 needed a follow-up commit (`65fafe9`) for exactly this. Name the `strings.he.ts` key, never quote the copy.

  - [x] `control.go`: `ControlEngine` gains one method, appended at the end of the interface:
    ```go
    SetDisplaySettings(ctx context.Context, gameID, organizerID string, reducedMotion bool) (game.Snapshot, error)
    ```
    Widening the existing interface is deliberate: a separate `DisplayEngine` threaded through `NewRouter` would be a **12th** positional parameter and would touch all ~25 `NewRouter(...)` test call sites, for an engine the router already receives. **Nothing else in `control.go` changes** — not `handleOpenLobby`, not the three dispatch helpers, not the six existing handlers.
  - [x] New `server/internal/httpapi/display.go`:
    ```go
    package httpapi

    import (
        "context"
        "log/slog"
        "net/http"
        "time"
    )

    // displaySettingsPayload mirrors the request body; the response is the
    // full snapshot, like every other engine+hub route in this package.
    // Pointer field and required: with a plain bool an omitted key decodes
    // to false, and false is meaningful here - absence must be an explicit
    // 400, never an accidental "turn animations back on" (the same reasoning
    // handleUpdateScoring documents for its four pointers).
    type displaySettingsRequest struct {
        ReducedMotion *bool `json:"reducedMotion"`
    }

    // handleUpdateDisplaySettings sets the room-level display settings and
    // broadcasts the resulting snapshot (FR-9). PUT, not PATCH: it replaces
    // the whole settings sub-resource, matching PUT /scoring.
    //
    // Broadcast before writing the REST response so an already-open display
    // renders at least as promptly as the organizer's own dashboard - the
    // same ordering handleOpenLobby documents.
    func handleUpdateDisplaySettings(engine ControlEngine, hub SnapshotBroadcaster) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            organizerID, ok := requireOrganizer(w, r)
            if !ok {
                return
            }
            gameID, ok := gameIDParam(w, r)
            if !ok {
                return
            }
            var req displaySettingsRequest
            if !decodeJSON(w, r, &req) {
                return
            }
            if req.ReducedMotion == nil {
                writeValidationError(w, "reducedMotion is required")
                return
            }
            ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
            defer cancel()
            snapshot, err := engine.SetDisplaySettings(ctx, gameID, organizerID, *req.ReducedMotion)
            if err != nil {
                writeStoreError(w, err, "GAME_NOT_FOUND")
                return
            }
            hub.Broadcast(gameID, snapshot)
            slog.Info("display settings updated", "game_id", gameID, "organizer_id", organizerID, "reduced_motion", *req.ReducedMotion)
            writeJSON(w, http.StatusOK, snapshot)
        }
    }
    ```
  - [x] `router.go`: one line **inside** the `if engine != nil && hub != nil` block, after `gr.Post("/stop", …)`:
    ```go
    gr.Put("/display-settings", handleUpdateDisplaySettings(engine, hub))
    ```
    Inside the guard, deliberately — unlike 3.10's `/results`, this route genuinely needs both the engine and the hub, so a test router passing nil for either must not expose it.
  - [x] **`NewRouter`'s signature does not change.** If you are editing `router_test.go`'s `NewRouter(...)` calls, stop — you added a parameter you did not need.
  - [x] **No `errors.go` change.** Every error this handler can produce is already mapped (`writeStoreError`, `writeValidationError`, `gameIDParam`'s 404).

- [x] **Task 6: `web/src/index.css` — the projection type ramp** (AC: 1, 2, 4)

  DESIGN.md A19 fixes the ramp at 1080p and requires "all sizes in rem so the whole ramp scales with viewport width". The stage is 16:9 and never scrolls, so viewport-width units give the *exact* A19 numbers at 1920 and scale proportionally at any other width. Define the ramp once here so stories 4.2–4.6 inherit it instead of each inventing sizes.

  - [x] Append a scoped block after the `@theme` blocks (not inside `@theme` — these are stage-local, not global Tailwind tokens):
    ```css
    /*
     * Audience Display projection ramp (DESIGN.md A19, UX-DR3). Sizes are
     * viewport-width relative so the stage keeps its proportions from the
     * 1280x720 minimum to 1920x1080 and beyond; at 1920 each value lands on
     * A19's exact pixel figure (1vw = 19.2px):
     *   display 5vw = 96px    heading 3.3334vw = 64px
     *   ui      2.5vw = 48px  body    2.0834vw = 40px
     *   safe margin 2.5vw = 48px (projector overscan tolerance)
     * Nothing on the stage sits below the body step.
     */
    .stage-root {
      --stage-display: 5vw;
      --stage-heading: 3.3334vw;
      --stage-ui: 2.5vw;
      --stage-body: 2.0834vw;
      --stage-margin: 2.5vw;
    }
    /* The stage cross-fade (EXPERIENCE.md: <=300ms, instant when reduced).
       Two independent sources: the viewer's own OS setting, and the
       Organizer's room-level toggle riding the snapshot - the audience
       cannot set the former on a projector, which is why the latter exists. */
    .stage-fade {
      animation: stage-fade-in 300ms ease-out;
    }
    @keyframes stage-fade-in {
      from { opacity: 0; }
      to   { opacity: 1; }
    }
    @media (prefers-reduced-motion: reduce) {
      .stage-fade { animation: none; }
    }
    .stage-root[data-reduced-motion='true'] .stage-fade { animation: none; }
    ```
  - [x] **No new `@theme` color token.** Every color this story needs (`green-800`, `green-900`, `surface-base`, `surface-raised`, `ink-on-dark`, `ink-on-dark-muted`, `text-primary`) already exists. **Do not touch `--color-gold`** — defined, still unused, and it stays that way until 4.3.
  - [x] **No external URL, no `@import` of a font, no `url(//…)`.** Both CI filter-safety scans (source and built bundle) cover this file.

- [x] **Task 7: `web/src/lib/types.ts` + `web/src/lib/strings.he.ts` — the mirror and the copy** (AC: 1, 4, 5)

  - [x] `types.ts`, appended near `LobbySnapshot` (keep every existing interface where it is):
    ```ts
    /** Mirrors the Go game.DisplaySettings — room-level rendering settings
     * the Audience Display obeys. Set by the Organizer on the dashboard
     * because the audience cannot set prefers-reduced-motion on a projector. */
    export interface DisplaySettings {
      reducedMotion: boolean
    }
    ```
    and on `LobbySnapshot`, after `leaderboard`:
    ```ts
    displaySettings: DisplaySettings
    ```
    Non-optional — the server always sends it (no `omitempty`).
  - [x] `strings.he.ts`: **all Hebrew for this story lives here.** EXPERIENCE.md's Host-microcopy table gives one exact row for this surface — the launch CTA — and EXPERIENCE.md's Display-controls line names the toggle. Everything else is authored to the table's *rules* (direct, terse, says what happened and what next, never apologizes, no emoji) and marked `[ASSUMPTION]` in the same convention `live.stopConfirm*` and `results` already use.
    - [x] Two additions to the existing `live` block (host-side controls belong with the other host CTAs):
      ```ts
      // "פתח מסך קהל" is EXPERIENCE.md's Host-microcopy row verbatim;
      // "הפחת אנימציות" is its Display-controls wording verbatim. The
      // hint below is [ASSUMPTION] — the spec names the control, not its
      // explanation.
      openDisplayCta: 'פתח מסך קהל',
      reduceMotionLabel: 'הפחת אנימציות',
      reduceMotionHint: 'מבטל תנועה במסך הקהל עבור כל החדר.',
      reduceMotionError: 'שינוי ההגדרה לא נשמר. נסו שוב בעוד רגע.',
      ```
    - [x] New `display` block, appended after `results`:
      ```ts
      // [ASSUMPTION]: EXPERIENCE.md specifies the display's resilience
      // behavior ("מתחבר מחדש..." over the last rendered state) and its
      // stage inventory, but no copy for a stage that has not been built
      // yet or for a display opened on an unknown game — authored to the
      // Host-microcopy rules and flagged for Avraham to confirm/replace.
      display: {
        // First connect, nothing rendered yet. Matches common.connecting
        // deliberately: the epic AC quotes "מתחבר...".
        connecting: 'מתחבר…',
        // Reconnect, over the last rendered stage — EXPERIENCE.md's
        // Resilience row wording.
        reconnecting: 'מתחבר מחדש…',
        notFound: 'המשחק הזה לא נמצא.',
        // Every stage until its own story lands (4.2–4.6), plus `draft`
        // permanently — a display opened before the lobby is.
        waiting: 'המסך מוכן — ממתינים למארגן.',
      },
      ```
    - [x] Screen-reader state names for the `aria-live` announcer (AC-2/UX-DR14). Inside the `display` block:
      ```ts
      stateAnnouncement: {
        draft: 'המשחק עוד לא נפתח',
        lobby: 'הרשמה פתוחה',
        question_open: 'שאלה פתוחה',
        question_closed: 'השאלה נסגרה',
        revealed: 'התשובה נחשפה',
        leaderboard: 'טבלת התוצאות',
        finished: 'המשחק הסתיים',
      },
      ```
      Keyed by the canonical `GameState` strings so the switcher and the announcer can never drift apart. Because `strings` is `as const`, indexing this object with a `GameState`-typed value is itself the exhaustiveness check — a missing key is a compile error, exactly like the `Record<GameState, …>` on the switcher. **Do not add an `?? ''` fallback**; it would disarm that check.

- [x] **Task 8: `web/src/features/display/` (new) — the shell** (AC: 1, 2, 3, 4, 5)

  Architecture names this directory and `display-page.tsx` for FR-9/FR-10. Two new files.

  - [x] **`web/src/features/display/stage-placeholder.tsx`** — one component the switcher points at until each stage's own story lands:
    ```tsx
    import { strings } from '@/lib/strings.he'
    import type { StageProps } from './display-page'

    // Every state maps here until its own story (4.2–4.6) replaces that one
    // entry in display-page's stageByState map; `draft` keeps it permanently
    // (the launch CTA only appears from lobby onward, so a draft display is
    // only ever reached by a hand-typed URL).
    export function StagePlaceholder(_: StageProps) {
      return (
        <p className="text-[length:var(--stage-heading)] font-heading text-text-secondary">
          {strings.display.waiting}
        </p>
      )
    }
    ```
    - [x] **`import type`, not a value import**, for `StageProps` — `display-page.tsx` imports this component, so a value import would be a runtime cycle. A type-only import is erased at compile time and is not.
    - [x] `text-[length:var(--stage-heading)]` is the arbitrary-value form Tailwind v4 still accepts (the bare `text-[var(…)]` is ambiguous between font-size and color). If it does not resolve in this Tailwind version, use `style={{ fontSize: 'var(--stage-heading)' }}` rather than hardcoding a size — the ramp variable must stay the single source.
    - [x] `text-text-secondary` on `surface-base` is 10.6:1 per DESIGN.md's contrast table. **Do not use `ink-on-dark-muted` here** — that token is for the dark green hero band, and this placeholder sits on the light ground.
  - [x] **`web/src/features/display/display-page.tsx`** — the shell. Required behavior, each item load-bearing for an AC:
    - [x] **Route data source is the socket alone**: `const { snapshot, notFound } = useGameSocket(gameId, 'display')`. **No `useQuery`, no `api()` call, no TanStack Query import in this file** (architecture: "`display/*` renders exclusively from the WS snapshot (no TanStack Query) — it must work with nothing but a `gameId` and a socket"). AC-2.
    - [x] **Last-snapshot retention** (AC-4). `useGameSocket` nulls `snapshot` the moment the socket closes, which is right for the dashboard but would blank a projector mid-room. Keep the last non-null snapshot and render it *under* the reconnect overlay:
      ```tsx
      const lastSnapshot = useRef<LobbySnapshot | null>(null)
      useEffect(() => {
        if (snapshot) lastSnapshot.current = snapshot
      }, [snapshot])
      const rendered = snapshot ?? lastSnapshot.current
      ```
      Sync the ref **inside an effect, never during render** — `control-page.tsx` documents that the lint rules here forbid the latter. The ordering works: when `snapshot` flips to null, this render still sees the previous value in `lastSnapshot.current`, and the effect that follows does not overwrite it.
    - [x] **Overlay copy, three cases, in this order** (AC-4, UX-DR12):
      - `notFound` → `strings.display.notFound`, alone, no overlay (a dead end, not a transient one). Stop rendering stages.
      - `!snapshot && !rendered` (first connect) → `strings.display.connecting`, centered on the stage background.
      - `!snapshot && rendered` (reconnect) → the last stage still rendered, with `strings.display.reconnecting` in a fixed corner band over it.
      - `snapshot` present → no overlay at all. **The connecting copy must never appear while the socket is up** — that is the literal AC.
    - [x] **Stage switcher** (AC-2), exhaustive over the seven canonical states so TypeScript fails a missing one:
      ```tsx
      export interface StageProps {
        snapshot: LobbySnapshot
        reducedMotion: boolean
      }

      // One component per game state (UX-DR16). Stories 4.2–4.6 each
      // replace exactly one entry; `draft` keeps the placeholder.
      const stageByState: Record<GameState, ComponentType<StageProps>> = {
        draft: StagePlaceholder,
        lobby: StagePlaceholder,            // story 4.2 — lobby stage
        question_open: StagePlaceholder,    // story 4.3 — question stage
        question_closed: StagePlaceholder,  // story 4.3 — question stage
        revealed: StagePlaceholder,         // story 4.4 — reveal stage
        leaderboard: StagePlaceholder,      // story 4.5 — leaderboard stage
        finished: StagePlaceholder,         // story 4.6 — winner takeover
      }
      ```
      A `Record<GameState, …>` and not a lookup with a fallback: the whole point is that adding a state to the union breaks the build here.
    - [x] **Effective reduced motion** is the OR of two sources: the viewer's `prefers-reduced-motion: reduce` (read once via `window.matchMedia`, subscribed for changes) and `rendered.displaySettings.reducedMotion` from the snapshot (AC-5). Pass the result as `StageProps.reducedMotion` **and** set `data-reduced-motion={String(effective)}` on the `.stage-root` container so Task 6's CSS rule can kill the fade without any stage having to opt in. Both channels, because 4.3–4.6 will need each.
    - [x] **The cross-fade** (EXPERIENCE.md, ≤300ms): give the stage wrapper `className="stage-fade"` plus `key={rendered.state}` so a state change remounts it and replays the animation. Under reduced motion the CSS makes it instant — no JS branch needed.
    - [x] **State announcer** (UX-DR14): a visually-hidden `<p aria-live="assertive" role="status">{strings.display.stateAnnouncement[rendered.state]}</p>`. Assertive is correct here and only here — the display is projected, but it is still a page a screen-reader user may open, and a state change is exactly the interruption-worthy event UX-DR14 reserves assertive for. Do **not** put counters or the timer numeral in a live region (that is 4.2/4.3's problem, and the timer is explicitly excluded).
    - [x] **Surface** (AC-1, AC-4): the root container is `dir="rtl"` (belt-and-braces over `index.html`'s document-level `dir`, since this window may be opened standalone), `min-h-svh w-full`, `bg-surface-base`, `text-text-primary`, `overflow-hidden` (a projector never scrolls), padding `var(--stage-margin)`, and `className="stage-root …"`. Flat — **no `shadow-*` class anywhere** (DESIGN.md: "Audience Display: flat").
    - [x] **Output-only** (AC-1): no `<button>`, no `<a>`, no `<input>`, no `onClick`, nothing focusable that the room could tab into. Nobody touches the projector.
    - [x] `const { gameId = '' } = useParams()` — same pattern as every other routed page.

- [x] **Task 9: `web/src/app.tsx` — the route** (AC: 1)

  - [x] Add `{ path: '/display/:gameId', element: <DisplayPage /> }` as a **top-level** route, sibling to `/login`, before the `*` catch-all — **not** under `RequireAuth` and **not** under `DashboardLayout`.
  - [x] Both exclusions are deliberate and both are ACs:
    - `DashboardLayout` renders the 240px sidebar, the brand, and the logout button. A projector showing dashboard chrome is not the surface FR-9 describes.
    - `RequireAuth` runs a REST session probe (`GET /api/auth/me`) before rendering. Putting the display behind it would make a REST call the precondition for the first paint, which AC-2 rules out. The WS handshake is already the auth gate: `ws/handler.go` rejects a missing/invalid session **before** the upgrade, and `use-game-socket`'s post-handshake probe lets `api()`'s default 401 handling redirect the window to `/login`. An unauthenticated display therefore self-heals to the login page with no route guard — **verify this in the browser pass, do not assume it.**
  - [x] The SPA fallback already serves this path: `spaHandler` returns `index.html` for any extension-less non-reserved path, and its own doc comment already names `/display/:gameId` as the example. **No server-side routing change.**

- [x] **Task 10: `web/src/features/live/display-controls.tsx` (new) + two mount points** (AC: 1, 5)

  One component owning both dashboard-side display controls — the launch CTA and the reduced-motion toggle — so the two surfaces that show them cannot diverge.

  - [x] **New `web/src/features/live/display-controls.tsx`:**
    ```tsx
    interface DisplayControlsProps {
      gameId: string
      reducedMotion: boolean
    }
    ```
    - [x] **Launch CTA**: a secondary `Button` (`variant="outline"`, `h-10` per the project's button convention) labelled `strings.live.openDisplayCta`, calling `window.open('/display/' + gameId, '_blank', 'noopener')`. A **new window**, per the AC and Flow 1 ("חלון חדש נגרר למקרן") — not a `<Link>`, which would navigate the dashboard away from the live panel mid-game. `noopener` because the opened window has no business reaching back into the opener.
    - [x] **Reduced-motion toggle**: a native `<input type="checkbox">` bound to `reducedMotion`, paired with the existing `Label` component (`components/ui/label.tsx`) via `htmlFor`/`id`, labelled `strings.live.reduceMotionLabel`, with `strings.live.reduceMotionHint` as an `aria-describedby` hint (UX-DR12: errors and hints are programmatically associated, never floating). **Not a shadcn Switch** — `components/ui/` has no switch primitive, `shadcn add` is out of scope for this story, and a checkbox is semantically exact for a boolean with free native keyboard and AT support. Style it with existing tokens only (`accent-green-800`, `size-5`); do not hand-roll a custom toggle.
    - [x] The mutation is `useMutation({ mutationFn: (next: boolean) => api<unknown>('/api/games/' + gameId + '/display-settings', { method: 'PUT', body: JSON.stringify({ reducedMotion: next }) }) })`. **Do not invalidate any query and do not hold local state for the checked value** — the checkbox is driven by `props.reducedMotion`, which comes from the snapshot, which the broadcast updates. That round trip is the AC's "rides the game snapshot" made visible on the dashboard too.
    - [x] On error render `strings.live.reduceMotionError` with `role="alert"`. Keep it inside this component — do **not** feed it into `control-page.tsx`'s error banner, which `deferred-work.md` already records as over-multiplexed with two mutations (3.10 entry); a third would make that entry worse.
  - [x] **`web/src/features/lobby/lobby-page.tsx`** — render `<DisplayControls gameId={gameId} reducedMotion={snapshot.displaySettings.reducedMotion} />` in the **`lobby` branch only**, near the "התחל משחק" button. **Not** in the `draft` branch: EXPERIENCE.md's State Patterns puts the display at "— (not yet launched)" pre-lobby, and its Host-control-panel table makes "פתח מסך קהל" available "from Lobby through Game over".
  - [x] **`web/src/features/live/control-page.tsx`** — render the same component in **both** return paths: the `finished` early-return branch (Game over is inside the CTA's window) and the main return, next to the primary CTA row. **Everything else in this file is off limits**: the `primaryActionByState` map, `stoppableStates`, the `fire()` ref guard, the `useSpaceAction` wiring, the error-banner precedence, the reset-on-state-change effect, and the never-disabled persistent primary button are all load-bearing decisions from 3.1's and 3.10's reviews.
  - [x] Both mount points import from `features/live/` — for `lobby-page.tsx` that is the pre-existing cross-feature exception it already uses for `ControlPage`, unchanged in kind. Record the reading in a one-line comment at the import.
  - [x] **Do not** touch `game-editor-page.tsx`, `games-list-page.tsx`, `results-*.tsx`, `dashboard-layout.tsx`, `use-game-socket.ts`, `api.ts`, or any `components/ui/*` file.

- [x] **Task 11: tests + quality gates + local E2E + manual browser pass** (all ACs)

  - [x] **Go tests.**
    - [x] `server/internal/game/engine_test.go` — `stubStore` grows `UpdateGameDisplaySettings` (compile requirement) plus a recorded-call field. Add `TestSetDisplaySettingsPersistsAndSnapshots`: the store returns a `gen.Game` with `ReducedMotion: true` → the returned `Snapshot.DisplaySettings.ReducedMotion` is true and the stub recorded `(gameID, organizerID, true)`. Add `TestSetDisplaySettingsStoreErrorPropagates` (`store.ErrNotFound` → the same error out, empty snapshot).
    - [x] **`TestEmptySnapshotCarriesDisplaySettings`** — build the degraded snapshot from a `gen.Game{ReducedMotion: true}` and assert the field survives. This is the one-line-easy-to-miss item from Task 4, and it is the only test that can catch it.
    - [x] `server/internal/httpapi/display_test.go` (new), using the existing `control_test.go` helpers (read them first — `stubControlEngine` there **is** mutex-guarded because 3.2/3.8 spawn post-response goroutines; this story spawns none, but you are extending that same stub, so follow its existing convention rather than introducing a second one):
      - `TestUpdateDisplaySettingsBroadcastsAndReturnsSnapshot` — 200, body is the snapshot with `displaySettings.reducedMotion` true, and the hub stub recorded exactly one `Broadcast` for this gameID.
      - `TestUpdateDisplaySettingsMissingFieldReturns400` — body `{}` → 400 `VALIDATION_ERROR` (or whatever code `writeValidationError` emits — read `errors.go`, do not guess), **and the engine was never called** (assert the recorded-call slice is empty: proves absence is rejected before it can be read as `false`).
      - `TestUpdateDisplaySettingsFalseIsAccepted` — body `{"reducedMotion": false}` → 200 and the engine recorded `false`. Pins that the pointer guard rejects *absence*, not the value.
      - `TestUpdateDisplaySettingsForeignOrMissingGameReturns404` — `store.ErrNotFound` from the engine → 404 `GAME_NOT_FOUND`, no broadcast.
      - `TestUpdateDisplaySettingsMalformedGameIDReturns404` — non-UUID `{gameID}` → 404 with no engine call (`gameIDParam` precedent).
      - `TestUpdateDisplaySettingsWithoutSessionReturns401` — **extend the existing unauthenticated table in `games_test.go`** (`TestGameMutationsWithoutSessionReturn401`) with one entry rather than building a second `noAuth()` router. If that table's router passes nil for engine/hub, the route will not exist there — in that case add the case to `control_test.go`'s equivalent table instead, or say in the Debug Log why neither fit.
      - `TestUpdateDisplaySettingsRouteAbsentWithoutEngine` — a router built with nil engine/hub answers 404 on this path, proving the route really is inside the guard.
    - [x] **No `router_test.go` / `packages_test.go` / `questions_test.go` change.** `NewRouter`'s arity is untouched; a diff there means Task 5 went wrong.
  - [x] **Hebrew copy-centralization gate — run it locally with a positive control.** Story 3.9's Debug Log records this gate silently false-passing when `LC_ALL` was applied to an assignment instead of to `grep`. Export it first, and print the full match list to prove grep actually ran:
    ```bash
    cd server && export LC_ALL=C.UTF-8
    grep -rlP '[\x{0590}-\x{05FF}\x{FB1D}-\x{FB4F}]' --include='*.go' . \
      | grep -v '_test\.go$' | grep -v '^\./internal/wa/messages_he\.go$'
    ```
    Expected output: **nothing**. `internal/httpapi/display.go` must not appear. Note `deferred-work.md`'s open 3.10 entry: sqlc copies `queries/*.sql` comments verbatim into `gen/*.sql.go`, which this gate scans — Task 2's SQL comment must stay English for the same reason.
  - [x] **Frontend Hebrew centralization**: `grep -rlP '[\x{0590}-\x{05FF}]' --include='*.tsx' web/src` must list only the two pre-existing violations (`features/builder/scoring-editor.tsx`, `features/live/control-page.tsx` — both comments, both on `origin/main`, both already recorded in `deferred-work.md`). **This story's own new files must carry zero Hebrew**, comments included.
  - [x] Backend gates: `gofmt -l .` (the CRLF-checkout caveat carried since 3.4 still applies — LF-normalize the touched files to separate real drift from line-ending noise) · `go vet ./...` · `go test ./...` (zero regressions) · `sqlc generate` diff confined to `gen/games.sql.go` + `gen/models.go` + the `games`-scanning query files, and idempotent on a second run.
  - [x] Frontend gates: `npm run lint` · `npx tsc -b --noEmit` · `npm run build` · both filter-safety greps (source and built bundle). `go test -race` may be unavailable (`CGO_ENABLED=0` in this environment, per 3.8–3.10's Change Logs) — if so, **say so in the Dev Agent Record** rather than implying race coverage.
  - [x] **Local Go E2E** (`cmd/e2escratch`, deleted after use — the 2.1–3.10 convention, including the fake-provider `WHATSAPP_API_BASE_URL` override so no real WhatsApp traffic leaves the machine). This is the first E2E in the project that needs a WebSocket *client*: use `websocket.Dial` from **`github.com/coder/websocket`**, already a direct dependency (`server/go.mod`) — do not add a client library, and pass the session cookie on the dial request's header, since the handshake authenticates before upgrading. Two things only a real server proves:
    - **Scenario A — the setting round-trips through the snapshot.** Log in, create a scratch game, `open-lobby`. Open a **real `role=display` WebSocket** with the session cookie and read the initial frame: `displaySettings.reducedMotion` is `false`. `PUT /api/games/{id}/display-settings {"reducedMotion":true}` → 200, and the **already-open display socket receives a broadcast frame** carrying `true` (this is the whole of AC-5's server half; assert on the pushed frame, not on the PUT response). Close and reopen the display socket: the *initial* frame still carries `true` — proving the reconnect path reads the column, not memory. Then `PUT {"reducedMotion":false}` and confirm the pushed frame flips back.
    - **Scenario B — the display socket sees every transition.** With the display socket open, drive `start` → `close-question` → `reveal` → `next-question` (past the last question → `finished`) and assert the display received one frame per transition, each with the expected `state`, in order and with strictly non-decreasing `seq`. Cross-check the first frame after each action against an independent `Engine.Snapshot` read rather than a plausible-looking guess (the discipline 3.7–3.10's E2Es all used).
    - Also assert a second organizer's session opening `/ws?gameId=…&role=display` for this game is rejected **before** the upgrade with 404 `GAME_NOT_FOUND` (not 403, not a snapshot).
    - **A green E2E is not evidence unless you confirm which process answered** — 3.10's Debug Log records a run silently served by a leftover server from the previous attempt. Build the binary once and exec it directly; grep the log for exactly one `server listening` and no `bind:` error.
    - Clean up the scratch game rows afterward (cascades); delete the harness.
  - [x] **Manual browser pass** (`make dev`, real browser — none of this story's frontend behavior is covered by any Go test, and no frontend test framework exists). This is the *primary* verification for four of the five ACs:
    1. **AC-1** — from the lobby, click "פתח מסך קהל": a **new window** opens at `/display/{id}`, full-bleed, RTL, no dashboard sidebar, no focusable control. F11 goes true full-screen. The dashboard window stays on the lobby.
    2. **AC-3** — with both windows visible, drive `start` / `close` / `reveal` / `next` from the dashboard and confirm the display's state announcement and stage change **within a second**, every time.
    3. **AC-4, reconnect** — kill the Go server (`Ctrl-C`) with the display open: the last stage stays on screen with "מתחבר מחדש…" over it, never a blank screen. Restart the server: the display re-renders current state **with no interaction**, and the overlay disappears. Confirm the copy is gone while the socket is up.
    4. **AC-4, first connect** — reload the display: "מתחבר…" appears only for the instant before the first frame.
    5. **AC-5** — tick "הפחת אנימציות" on the dashboard: the display's `data-reduced-motion` flips to `true` (check the DOM) and the cross-fade stops, **with no interaction on the display window**. Reload the display: still on. Untick: off again.
    6. **AC-2, no REST** — on the display route, the Network tab's **Fetch/XHR filter stays empty** and the WS filter shows exactly one connection. (A `GET /api/games/{id}` appears only if the handshake fails — that is the classification probe, not a data source.)
    7. **Unauthenticated display** — sign out in the dashboard window, then reload the display window: it lands on `/login` rather than spinning forever or throwing.
    8. **Unknown game** — open `/display/{a-random-uuid}`: the display shows `strings.display.notFound` and stops retrying.
    9. **Projection scale** — resize the display window between 1280×720 and 1920×1080: the placeholder text scales proportionally, nothing clips, no horizontal scrollbar ever appears.
    10. **Palette and filter-safety** — the stage is Festival Green (`surface-base` ground), **no gold anywhere**, and the Network tab shows **zero** requests to any external host.
    11. Zero console errors throughout, in both windows.

- [x] **Task 12: re-triage the three `deferred-work.md` entries whose trigger is this story** (no code)

  Three open entries name Epic 4 / story 4.1 as their revisit trigger. Each needs an explicit written outcome in `deferred-work.md` — a decision recorded, not a silence. **Do not implement any of them in this story.**

  - [x] **`role=display` authenticates with a full organizer session** (2.3 entry, trigger: *"Epic 4 (audience display shell, story 4.1) — design display auth there"*). **Outcome: re-defer, with the reasoning recorded.** The epic's own AC-1 and the architecture both specify the session-cookie approach explicitly ("same-origin, inheriting my session cookie, no pairing flow"), so a scoped display token would be this story *overruling its own spec*. What is genuinely true and worth writing down: the projector machine is the organizer's own laptop in every pilot scenario (Flow 1), so the credential never leaves their hands; the exposure becomes real only when the display runs on a machine the organizer does not control. **Re-point the trigger to that**, not to a story number.
  - [x] **Live WS connections survive logout and session expiry** (2.3 entry, trigger: *"when the display-auth design lands (Epic 4)"*). Same outcome, same reasoning — the trigger fires only under the same condition. Merge or cross-reference it with the entry above so they cannot drift apart.
  - [x] **Broadcast `seq` is assigned at `Broadcast()`-call time, not DB-commit order** (3.1 entry, trigger: *"before Epic 4 — audience displays make a stuck stale state visible to a whole room"*). This one is a **judgment call for Avraham, and you must surface it rather than deciding silently.** The race needs two organizer sessions acting on one game concurrently, which stays effectively unreachable at pilot scale (one organizer, one tab); the fix (deriving `seq` from something commit-ordered, or serializing broadcast-after-write per game) is a `ws`/`control.go` redesign well outside a shell story. **Recommended: re-defer, re-pointing the trigger at "a second organizer surface, or the first report of a stuck stale state."** Record the recommendation and the reasoning in the Dev Agent Record and let Avraham confirm at code review.
  - [x] Two further entries name Epic 4 but are **not** triggered by this story — note that explicitly so the next Epic 4 story does not have to re-derive it:
    - *Spectator rows inflate the role-blind snapshot roster* (2.5) — trigger is "the first surface rendering a live roster past lobby". This shell renders no roster; 4.2's lobby counter is lobby-only, where spectator rows cannot exist. Still not triggered.
    - *`emptySnapshot`'s empty leaderboard is indistinguishable from all-zeros* (3.7) — trigger is `leaderboard-stage.tsx`, which is story 4.5. Not triggered here. (Task 4's `emptySnapshot` change is additive and does not touch this.)

### Review Findings

Code review 2026-08-09 (bmad-code-review, three parallel layers: Blind Hunter / Edge Case Hunter / Acceptance Auditor). All five quality gates re-run independently and green: `go vet`, `go test -count=1 ./...`, Hebrew centralization gate with a 19-file positive control, `tsc -b`, `npm run lint`, `npm run build`, both filter-safety scans, zero `gold`, zero `shadow-*` on the display, zero focusable elements on the display, zero TanStack/`api()` import in `display-page.tsx`, `NewRouter` arity unchanged, no file from the "Untouched" list touched.

**Decisions — all four resolved by Avraham at the review (2026-08-09):**

- [x] [Review][Decision] Task 12's re-defer of the broadcast-`seq` race rests on a premise this story falsifies — the recorded reasoning is "story 4.1 adds no new writer — the display is strictly a reader" and "the race requires two organizer sessions". Both are now false: `handleUpdateDisplaySettings` is a seventh broadcasting writer [server/internal/httpapi/display.go:52], and `DisplayControls` renders *inside* the live control panel next to the transition CTAs [web/src/features/live/control-page.tsx:220], neither disabling the other. One organizer in one tab who presses "next question" and then ticks the checkbox has two goroutines racing `hub.Broadcast`; the older snapshot can win the higher `seq`, the client's `seq < lastSeq` guard accepts it, and the whole room regresses to a stale stage with no self-heal (a re-press returns 409; on `finished` it is permanent). Options: (a) re-defer anyway with the trigger re-pointed and the corrected reasoning recorded; (b) serialize the settings write against transitions in the UI (cheap, partial); (c) fix `seq` ordering properly in its own story before Epic 4 ships to a real room. — **Resolved: (a).** `deferred-work.md`'s 3.1 `seq` entry now carries the corrected reasoning (the wrong premises struck through, not quietly rewritten) and a trigger of "a second organizer surface, the first report of a stuck stale state, or Epic 4 shipping to a real room". The checkbox's own self-race was closed in passing by the in-flight guard patch below; the cross-race with a transition is what stays deferred.
- [x] [Review][Decision] The projection ramp uses `vw`, overriding DESIGN.md A19's literal "all sizes in rem" and forfeiting UX-DR14's 200%-zoom clause (WCAG 1.4.4) on this surface [web/src/index.css:191-197]. The pixel math is exact at 1920 (96/64/48/40/48), and Task 6's reasoning for `vw` is sound for a projector — but `vw` text does not respond to browser zoom at all, and the display is the only surface in the app off the rem scale. Neither the CSS comment nor the Dev Agent Record records this as a deviation. Options: (a) keep `vw`, record the deviation and the traded-away clause explicitly; (b) `clamp()` with a rem floor so zoom still works; (c) revert to rem and accept a fixed ramp. — **Resolved: (a).** `index.css`'s ramp comment now states the deviation, why bare rem cannot deliver A19's own stated intent, and that UX-DR14's 200%-zoom clause is knowingly forfeited on this one surface (with clamp-over-a-rem-floor named as the revisit if the display ever becomes a readable page). Also records that `--stage-display`/`--stage-ui` are unreferenced by design.
- [x] [Review][Decision] AC-5's persistence half has no coverage that will ever run again, and the three deferred entries whose trigger is exactly this (3.4 / 3.7 / 3.9, "Epic 4, or the first defect that reaches a real game — introduce a Postgres-backed integration tier") have now fired un-noted. Deleting `SET reduced_motion = $3` from `queries/games.sql`, dropping the `AND organizer_id = $2` cross-tenant guard, or never applying migration 00016 all leave the entire Go suite green — every new test drives a stub, and `cmd/e2escratch` was deleted per convention. Options: (a) re-defer again with the trigger re-pointed; (b) introduce the Postgres-backed tier now as its own story; (c) keep `cmd/e2escratch` as a committed, CI-optional harness instead of deleting it each story. — **Resolved: (a).** The whole four-entry family (3.4 / 3.7 / 3.10 / 00014's NULL path) is re-deferred together under one new trigger that no longer names an epic: "the first scoring, grading, results or settings defect that reaches a real game, or the first sqlc/migration change made by someone who did not write the original query". Recorded on the 3.4 entry, with the others cross-referencing it.
- [x] [Review][Decision] Confirm or replace the new Hebrew copy. Everything in the `display` block plus `live.reduceMotionHint` / `live.reduceMotionError` is authored `[ASSUMPTION]`, and `openDisplayCta: 'פתח מסך קהל'` — the most prominent new string on the dashboard — is presented in the code comment as "EXPERIENCE.md's Host-microcopy row verbatim" when EXPERIENCE.md:141 itself marks that row `[ASSUMPTION]` and records a rejected alternative ("הצג מצב מקרן") [web/src/lib/strings.he.ts:165-171, 196-228]. — **Resolved: approved as written.** The `[ASSUMPTION]` markers on this story's strings are removed and replaced with a note recording Avraham's confirmation at this review; the launch-CTA comment no longer presents "verbatim" as "settled" and now says the source row is itself marked. One string was added by the review patches (`openDisplayBlocked`) and is confirmed under the same decision.

**Patches:**

- [x] [Review][Patch] A cosmetic toggle can broadcast `emptySnapshot`, blanking the room's live data — `SetDisplaySettings` routes through `snapshotAfterCommit`, so a transient `buildSnapshot` failure after the settings UPDATE commits degrades to `emptySnapshot` (`participantCount: 0`, `currentQuestion: nil`, empty leaderboard) and `handleUpdateDisplaySettings` broadcasts it to every socket and returns it as 200. Before this story `emptySnapshot` was only reachable from a deliberate state transition; it is now reachable from a checkbox, including on a `finished` game, which the pre-existing 3.7 deferral assumed could never re-broadcast. Fix: propagate the build error instead of degrading — a settings write has nothing a client must learn about, so a 500 the organizer retries beats a blanked room [server/internal/game/engine.go:224-229, server/internal/httpapi/display.go:52]
- [x] [Review][Patch] Unguarded `snapshot.displaySettings.reducedMotion` deref crashes the page on deploy skew, and there is no error boundary anywhere in `web/src` — `store/migrate.go:23-24` documents two concurrent instances during a redeploy, and `use-game-socket.ts` validates only `envelope.type`/`envelope.seq`, so a new bundle whose socket lands on a still-draining old instance gets a snapshot with no `displaySettings`. React Router's default English "Unexpected Application Error!" page then takes the projector *and* the organizer's live panel. Fix: optional-chain the four deref sites (and/or add an error boundary) [web/src/features/display/display-page.tsx:117, web/src/features/lobby/lobby-page.tsx:141, web/src/features/live/control-page.tsx:128, web/src/features/live/control-page.tsx:220]
- [x] [Review][Patch] `stageByState[rendered.state]` is `undefined` for any state the client's union lacks, throwing "Element type is invalid" onto the projector — the `Record<GameState, …>` is a compile-time check over the *client's* union and gives no runtime protection against server/client skew, which is the exact case its comment claims to cover. A runtime `?? StagePlaceholder` fallback does not disarm the compile-time exhaustiveness check (the Record type still demands every key) [web/src/features/display/display-page.tsx:118]
- [x] [Review][Patch] The launch CTA's window semantics are wrong on three counts at once — `window.open(url, '_blank', 'noopener')` [web/src/features/live/display-controls.tsx:48]: (1) a noopener-only features string is browser-dependent as to whether it yields a *window* or a *tab*, and AC-1 and Flow 1 both require a window; (2) `noopener` forces the target to `_blank` per spec, so every repeat click spawns another display window, each holding its own hub connection — likely, since the first window opens behind the dashboard on a single-monitor laptop; (3) `noopener` also makes `window.open` return `null` unconditionally, so a popup-blocked launch is undetectable even in principle and the organizer gets no feedback in front of a room. Fix: same-origin named target `display-${gameId}` with an explicit `popup=yes,width=,height=`, dropping `noopener` (the opened page is our own output-only code and never touches `window.opener`), and report a null return
- [x] [Review][Patch] The reduced-motion checkbox has no in-flight guard, so double-submission is the expected behavior — `checked` is driven purely by the snapshot (deliberate, per Task 10), so React snaps the DOM back immediately and nothing moves for a full PUT→broadcast round trip; on venue wifi that reads as broken and the organizer clicks again. Two concurrent PUTs then race, each building and broadcasting a full snapshot (feeding the `seq` decision above). Every sibling control in the project guards this with a ref (`control-page.tsx:80`, `lobby-page.tsx:42`); this one has nothing [web/src/features/live/display-controls.tsx:62-64]
- [x] [Review][Patch] The state announcer is unlikely to announce the first state, and mixes contradictory ARIA — `role="status"` carries an implicit `aria-live="polite"` and overriding it to `assertive` on the same element is handled inconsistently by screen readers; more concretely, the region is absent from both the `notFound` and `connecting` early returns, so it is inserted into the DOM *already populated*, and AT does not announce content present when a live region first appears. The single most important announcement — the display just came up in `lobby`/`question_open` — is dropped, and UX-DR14 is satisfied only from the second transition onward. Fix: bare `aria-live="assertive"` (drop `role="status"`) and mount the region in every branch [web/src/features/display/display-page.tsx:133]
- [x] [Review][Patch] The reconnect band is positioned outside the projection safe margin it is most needed inside — `absolute top-0 start-0 end-0` resolves against the root's padding box, i.e. the outer edge, while `--stage-margin` exists as "projector overscan tolerance". On a projector with overscan — the precise scenario the margin was added for — the room's only indication that the screen has gone stale is the one element clipped off. It also intrudes ~16px past the margin into the content box at both 1280×720 and 1920×1080, under `overflow-hidden`, which will silently clip the top of any full-height stage from 4.2 onward [web/src/features/display/display-page.tsx:151-153, web/src/index.css:191-197]
- [x] [Review][Patch] The new stage CSS is unlayered, so it silently outranks every Tailwind utility 4.2–4.6 will use — the rest of the file uses `@layer base`, but `.stage-root`, `.stage-fade`, the `@keyframes`, the `prefers-reduced-motion` block and the `[data-reduced-motion]` override are appended at top level. Unlayered rules beat all layered ones regardless of specificity, and Tailwind v4 utilities live in `@layer utilities` — so a later stage putting any `animation-*` utility on a `stage-fade` element will find it overridden with no visible specificity conflict. Fix: wrap the animation rules in a layer (the custom-property block is fine unlayered) [web/src/index.css:191-224]
- [x] [Review][Patch] `lastSnapshot` survives a `gameId` change, so the display renders and *announces* the previous game's stage while connecting to a new one — navigating `/display/A` → `/display/B` in the same window does not remount `DisplayPage`, so `useState` keeps game A's snapshot while the socket returns `{snapshot: null}` for B; `rendered` falls back to A, with the "מתחבר מחדש…" band on top implying it is B reconnecting. Fix: key the retained snapshot by `gameId` [web/src/features/display/display-page.tsx:87-91]
- [x] [Review][Patch] The mutation error banner is never reset and persists for the rest of the game — one transient 503 on the PUT leaves `setReducedMotion.isError` true, and because `DisplayControls` keeps the same instance across every state transition inside `ControlPage`, "שינוי ההגדרה לא נשמר" stays on the live panel indefinitely. `ControlPage` deliberately resets its own banners on `[snapshot.state]` (`control-page.tsx:111-114`); this component is not wired into that [web/src/features/live/display-controls.tsx:76]
- [x] [Review][Patch] Task 12 is incomplete — seven further open `deferred-work.md` entries name Epic 4 as their revisit trigger and got no written outcome. The story's enumeration of "three triggered, two not" is factually wrong; the un-triaged entries are at deferred-work.md lines 52 (2.4 lobby counts), 65 (3.1 control mutations discard the REST snapshot — the direct sibling of the entry that *was* triaged, and the one this story makes worse), 89 (3.4), 112 (3.7), 125 (3.9) (all three the Postgres integration tier), 121 (3.8 reveal→next pacing) and 128 (3.10 games list). Epic 4 has started with seven fired triggers silently unaddressed [_bmad-output/implementation-artifacts/deferred-work.md]
- [x] [Review][Patch] Three test gaps in the new suite — (a) `TestUpdateDisplaySettingsWithoutSessionReturns401` sends no body, so it passes by luck rather than pinning auth-before-validation ordering, and there is no malformed-body case at all (`{"reducedMotion":"yes"}`), leaving `decodeJSON`'s behavior on this route unspecified; (b) `TestUpdateDisplaySettingsRouteAbsentWithoutEngine` passes `nil` for *both* engine and hub, so it cannot distinguish "gated on engine" from "gated on hub" and does not prove the `&&` its comment claims to lock in; (c) nothing asserts the broadcast-before-response ordering that `display.go`'s doc comment gives as the reason the route is shaped this way [server/internal/httpapi/display_test.go:141, server/internal/httpapi/display_test.go:155]
- [x] [Review][Patch] `strings.live.reduceMotionError` is authored copy with no `[ASSUMPTION]` marker — the comment above it marks only "the hint below" (singular), and the error string appears in no EXPERIENCE.md row. The `display` block's block-level marker correctly covers its own keys [web/src/lib/strings.he.ts:166-171]

**Deferred (pre-existing, recorded in `deferred-work.md`):**

- [x] [Review][Defer] The settings mutation discards the 200 response snapshot, so a dropped WS frame leaves the checkbox permanently lying about persisted state [web/src/features/live/display-controls.tsx:28-33] — deferred, pre-existing: this is the standing 3.1 entry ("control mutations discard the REST response snapshot"), now landing on a controlled input where the failure is worse than for a CTA.
- [x] [Review][Defer] `updated_at` is now advanced by a write that changes no state, poisoning the remedy the `seq` entry itself proposes [server/internal/store/queries/games.sql:38] — deferred, pre-existing: recorded next to the `seq` entry so whoever implements "derive `seq` from `games.updated_at`" knows it is no longer a proxy for "state changed".
- [x] [Review][Defer] Nothing exercises the `StageProps` wiring — `reducedMotion` could silently drop the room-level OR, or `snapshot` could be swapped for the nullable one, and nothing would notice until 4.3 [web/src/features/display/display-page.tsx:139, web/src/features/display/stage-placeholder.tsx:20] — deferred, pre-existing: no frontend test framework exists and this story is forbidden from adding one.

**Dismissed as noise (7):** the WS handshake *is* organizer-scoped, so a display cannot reach a foreign game (`ws/handler.go:82` calls `engine.Snapshot(ctx, gameID, organizer.ID)`, and line 72 already 404s) · the app.tsx/display-page comments about `api()` are both true, not contradictory (the call lives in the hook's `probeThenDecide`) · the render-phase `setState` retention pattern is sound and lint-forced, and `useSyncExternalStore` guarantees the stable reference it depends on · `--stage-display`/`--stage-ui` are unused by design (the ramp is defined once for 4.2–4.6) · `goose down` on 00016 being binary-coupled is generic to every column add and matches the 00004/00009 precedent · the `stateAnnouncement` "as const is the exhaustiveness check" comment is imprecise but the enforcement is real at its one indexing site · the `stubControlEngine` mutex deviation is the *correct* call (only two methods there are guarded) and is documented in a code comment.

## Dev Notes

### Architecture guardrails (violations = rework)

- **🚨 Zero Hebrew in any `.go` file this story writes**, comments included. `httpapi/display.go` is a brand-new non-test Go file whose entire subject is a Hebrew-facing screen — precisely the temptation that cost story 3.8 a red build and a follow-up commit. Describe copy in English; name the `strings.he.ts` key instead of quoting it. Same rule for `queries/games.sql`: sqlc copies its comments verbatim into `gen/games.sql.go`, which the gate scans (`deferred-work.md`, 3.10).
- **Dependency direction unaffected.** `httpapi` → `game` and `httpapi` → `store` both already exist. `game` still never imports `httpapi`/`wa`/`ws`. `store` remains the only package touching pgx. `ws` is not touched at all.
- **Handlers stay thin.** `handleUpdateDisplaySettings` decodes, validates one pointer, calls the engine, broadcasts, writes. No business logic, no snapshot construction in `httpapi`.
- **The engine owns the snapshot.** `SetDisplaySettings` is not a state transition, but it changes what every WS client renders, so it goes through the engine like everything else that does. `httpapi` must not learn to build a `game.Snapshot`.
- **`UpdateGameScoring`'s missing `state = 'draft'` guard stays untouched.** `deferred-work.md`'s 1.4 entry points its trigger at "the next time `UpdateGameScoring` is touched" — this story adds a *sibling* query in the same file and does not touch that one. Appending `UpdateGameDisplaySettings` after it does not fire the trigger; changing `UpdateGameScoring` would.

### The socket hook is already correct — do not touch it

`use-game-socket.ts` was built in story 2.3 with `role: 'host' | 'display'` in its signature from the start, and `ws/handler.go` has accepted and logged `role=display` since the same story. Everything AC-4 asks for — exponential backoff from 500ms to 10s, indefinite retry, `lastSeq` stale-frame dropping, the StrictMode double-mount guard, the module-level store keyed by `gameId:role` — is already there and already reviewed. **This story adds the hook's first `role='display'` caller and nothing else.**

Two behaviors of the hook that matter here and are easy to misread:

1. **It nulls `snapshot` on close.** That is deliberate (UX-DR12, so the dashboard cannot show stale data as if it were live), and it is exactly wrong for a projector — hence Task 8's `lastSnapshot` ref, which lives *in the display page*, not in the hook. Changing the hook would change the dashboard's behavior as a side effect.
2. **`probeThenDecide` makes one REST call**, but only when a handshake never opened, and only to tell "this game does not exist" apart from "the server is down". It is error classification, not a data source, so it does not violate AC-2's "no REST queries — snapshot is the only data source". Do not remove it, and do not add a second REST call to compensate for it.

### Why the display route sits outside `RequireAuth`

Three reasons, in order of weight:

1. **AC-2.** `RequireAuth` blocks the first paint on `GET /api/auth/me`. Making a REST round trip the precondition for rendering is the thing the AC rules out.
2. **The WS handshake is already a strictly stronger gate.** `ws/handler.go` validates the session cookie, the `role` param, the gameID's UUID shape, and the game's ownership — all *before* the upgrade, each with its own status code. A route guard would re-check a subset of that, later.
3. **The failure path already works.** A missing session makes the handshake 401 before opening, `hasOpened` stays false, `probeThenDecide` runs, `api()`'s default `on401: 'redirect'` navigates the window to `/login`. That is the desired outcome, reached without a guard. **It is also the single most likely thing to be subtly wrong, so it is an explicit item in the browser pass.**

### Why the reduced-motion setting is a column and not memory

The three candidate homes, and why only one survives:

- **Display-side (`localStorage`, a URL param, a keyboard shortcut)** — ruled out by the AC's own reasoning: nobody interacts with the projector, and the room cannot set `prefers-reduced-motion` on it. A setting the audience cannot reach must be set by the Organizer.
- **In the hub / engine memory** — survives a *display* reconnect only while the process lives, and dies on every redeploy. `store/migrate.go` documents redeploys running two instances concurrently, so an in-memory flag would also disagree between them. Worse, the loss would happen at reconnect — the exact event AC-4 exists to make invisible to the room.
- **A `games` column** — one boolean, `NOT NULL DEFAULT false`, rebuilt into every snapshot by the code path that already rebuilds everything else. The migration is four lines and its Down is genuinely runnable.

### The stage-switcher contract (what 4.2–4.6 inherit)

This is the story's most consequential deliverable, because five later stories build against it. Get it right and each of them is a one-entry change:

- `StageProps` is `{ snapshot: LobbySnapshot; reducedMotion: boolean }`. A stage gets the whole snapshot (not narrowed props) because the snapshot *is* the store — narrowing would just mean editing the shell every time a stage needs one more field.
- `stageByState` is `Record<GameState, ComponentType<StageProps>>` — exhaustive by type, so adding a state to the union is a build error here rather than a silently blank projector.
- `reducedMotion` is already the OR of the OS setting and the room setting when a stage receives it. **No stage should ever call `matchMedia` itself** — 4.3's ring, 4.5's reshuffle and 4.6's confetti all read this one prop, and the `data-reduced-motion` attribute on the root covers the pure-CSS cases.
- The projection ramp (`--stage-display` / `--stage-heading` / `--stage-ui` / `--stage-body` / `--stage-margin`) is defined once in `index.css`. Later stages use the variables; none of them should hardcode a `px` or invent a size.
- The shell owns the cross-fade, the reconnect overlay, and the `aria-live` announcer. A stage renders content only — it never announces its own arrival and never draws a connection state.

### Existing code this story modifies — current state, and what must survive

- **[server/internal/game/engine.go](server/internal/game/engine.go)** — `Store` gains one method (23rd), `buildSnapshot` and `emptySnapshot` each gain one field assignment, and one new `Engine` method lands next to `OpenLobby`. Every existing transition, every guard, `snapshotAfterCommit`'s `WithoutCancel`+timeout reasoning, and `PlayerRecipients`' documented unscoped posture are untouched.
- **[server/internal/game/snapshot.go](server/internal/game/snapshot.go)** — one new type, one new field. `CurrentQuestion`'s deliberate omission of `CorrectOption`/`AcceptedAnswers` is load-bearing and stays: `Snapshot` is the one payload **both** roles receive, and the display is the reason that matters. Story 4.4 will need the correct answer on the reveal stage and will have to solve that properly; **this story must not open the door by adding it "while we're here".**
- **[server/internal/httpapi/control.go](server/internal/httpapi/control.go)** — one line on the `ControlEngine` interface. Nothing else. The six handlers, the three dispatch helpers and their state guards are all 3.1/3.2/3.8/3.9 review decisions.
- **[server/internal/httpapi/router.go](server/internal/httpapi/router.go)** — one `gr.Put` line inside the existing engine/hub guard. `NewRouter`'s 11 parameters, every existing route, and `spaHandler`/`isReservedPath` are untouched.
- **[server/internal/store/queries/games.sql](server/internal/store/queries/games.sql)** — one appended query. Every existing query, including `GetGameForOrganizer`'s `SELECT *` (which now scans one more column via regenerated code, not via an edit here), must be left byte-identical.
- **[web/src/lib/use-game-socket.ts](web/src/lib/use-game-socket.ts)** — **zero changes.** See above.
- **[web/src/features/live/control-page.tsx](web/src/features/live/control-page.tsx)** — one component rendered in two places. The `primaryActionByState` map, `stoppableStates`, `fire()`'s ref guard, `useSpaceAction`, the error-banner precedence and its reset effect, and the never-disabled persistent primary button are all protected by prior reviews.
- **[web/src/features/lobby/lobby-page.tsx](web/src/features/lobby/lobby-page.tsx)** — one component rendered in the `lobby` branch. The `draft` branch, `fireStart`'s ref guard, the `useSpaceAction` gate and the `<bdi dir="ltr">` JOIN-code header are untouched.
- **[web/src/app.tsx](web/src/app.tsx)** — one top-level route object. `RequireAuth`, `NotFoundPage`, and the authenticated branch are untouched.
- **[web/src/index.css](web/src/index.css)** — one appended scoped block. The `@theme` tokens, the shadcn `:root`/`.dark` variables and the `@layer base` rules are untouched.

### Design decisions worth flagging explicitly

- **The stage placeholder is not throwaway work.** It is the permanent `draft` stage and the temporary occupant of five map entries that shrink one per story. The alternative — five hand-written stub stages now — would be five files that 4.2–4.6 each rewrite from scratch.
- **`aria-live="assertive"` is correct here and nowhere else on this surface.** UX-DR14 reserves assertive for game-state transitions specifically, and the shell owns transitions. Counters (`polite`, throttled) are 4.2's; the timer numeral is **excluded from live regions entirely** and is 4.3's.
- **The reconnect overlay renders over the last stage, not instead of it.** EXPERIENCE.md's Resilience row is explicit ("over the last rendered state"), and the epic AC only constrains *when* the copy appears, not what is behind it. A blank projector mid-game is a worse failure than a stale one, and the stale state is a second old at most.
- **The launch CTA opens a window, not a tab or a route.** Flow 1 has שלמה dragging that window to the projector while keeping the dashboard in front of him. A `<Link>` would take the dashboard with it.
- **`PUT`, not `PATCH`.** One field today, but it is a settings sub-resource replaced whole — the same shape and the same verb as `/scoring`. When 4.3 or a later story adds a second display setting, the route does not change semantics.
- **`reducedMotion` is a required pointer in the request body.** With a plain `bool`, `{}` decodes to `false` and silently turns the room's animations back on — a request that says nothing would overwrite a deliberate setting. Absence is a 400; `false` is a legitimate value. This is the identical reasoning `handleUpdateScoring` documents for its four pointers, and there is a test for each half.
- **No `game` state guard on the settings write.** Legal from `draft` through `finished`. A guard would add a failure mode with no beneficiary — and the query's comment says so, so a future reader does not "fix" the missing predicate the way the draft-only mutations legitimately needed one.

### Testing standards

Go stdlib `testing`, co-located `_test.go`, stubs grown in place — no mock framework. **No real-DB unit tests**: the project's documented standard since 2.1, and `deferred-work.md` (3.4, 3.7, 3.10) already records the resulting SQL coverage gap with its revisit trigger. This story's migration and query inherit that gap, and **must not be the occasion for inventing an integration tier** — but Task 11's E2E is therefore the only thing that exercises the column and the broadcast, and is not optional.

`httpapi` tests use the existing `control_test.go` stubs and helpers; extend them rather than adding a parallel set. That file's `stubControlEngine` carries a mutex because stories 3.2/3.8 spawn post-response dispatch goroutines — this story spawns none, but you are extending that same stub, so keep its convention instead of introducing a second one alongside it.

No frontend test framework exists and this story does not add one (every prior frontend story's precedent, 1.2 through 3.10). **The browser pass in Task 11 is the frontend verification, and for this story it carries unusually much**: four of the five ACs are observable only in a running browser, and two of them (the reconnect overlay behavior, the unauthenticated redirect) have no other check anywhere. Story 3.10's browser pass is still outstanding for the same reason — if you can, do both in one session.

`go test -race` may be unavailable (`CGO_ENABLED=0` in this environment, per 3.8–3.10's Change Logs). If so, say so in the Dev Agent Record rather than implying race coverage. This story adds no goroutine and no shared mutable state of its own.

### Project Structure Notes

**New:**
- `server/migrations/00016_game_display_settings.sql`
- `server/internal/httpapi/display.go`
- `server/internal/httpapi/display_test.go`
- `web/src/features/display/display-page.tsx` (architecture names `features/display/display-page.tsx` — "`/display/:gameId` — full-screen, stage switcher")
- `web/src/features/display/stage-placeholder.tsx` (not in the architecture's listing; it is the temporary occupant of the five stage entries the architecture *does* name, and the permanent `draft` stage)
- `web/src/features/live/display-controls.tsx` (host-side controls, not a stage — `features/display/` is output-only by the architecture's own boundary rule)

**Modified:**
- `server/internal/store/queries/games.sql` (+1 query)
- `server/internal/store/gen/games.sql.go`, `server/internal/store/gen/models.go` (+ the other `games`-scanning generated files — sqlc output, never hand-edited)
- `server/internal/store/games.go` (+`UpdateGameDisplaySettings`)
- `server/internal/game/snapshot.go` (+`DisplaySettings`, +1 field)
- `server/internal/game/engine.go` (+1 `Store` method, +`SetDisplaySettings`, +2 field assignments)
- `server/internal/game/engine_test.go` (+1 stub method, +3 tests)
- `server/internal/httpapi/control.go` (+1 interface line)
- `server/internal/httpapi/router.go` (+1 route line)
- `server/internal/httpapi/control_test.go` (+1 stub method) and/or `games_test.go` (+1 unauthenticated table entry)
- `web/src/index.css` (+1 scoped block)
- `web/src/lib/types.ts` (+`DisplaySettings`, +1 field)
- `web/src/lib/strings.he.ts` (+`display` block, +4 `live` keys)
- `web/src/app.tsx` (+1 route)
- `web/src/features/lobby/lobby-page.tsx` (lobby branch only)
- `web/src/features/live/control-page.tsx` (two render sites only)
- `_bmad-output/implementation-artifacts/deferred-work.md` (Task 12 re-triage)

**Untouched (a diff here means you went off-spec):** `server/internal/ws/**` · `server/internal/wa/**` · `server/internal/grading/**` · `server/internal/auth/**` · `server/internal/game/state.go`, `answers.go`, `scoring.go`, `results.go`, `final.go`, `participants.go` · `server/internal/store/answers.go`, `participants.go`, `questions.go`, `packages.go` and their `queries/*.sql` · `server/internal/httpapi/games.go`, `questions.go`, `packages.go`, `results.go`, `errors.go`, `middleware.go`, `auth_handlers.go`, `router_test.go` · `server/cmd/server/main.go` · `web/src/lib/use-game-socket.ts`, `api.ts`, `text.ts`, `use-space-action.ts` · `web/src/components/**` · `web/src/features/builder/**`, `features/results/**`, `features/auth/**` · `web/index.html`.

### Latest technical information

No new dependency, no version bump, no external API. Backend is stdlib Go (`context`, `net/http`, `time`, `log/slog`) plus chi v5 and the already-generated sqlc surface; toolchain pinned at Go **1.26.5** ([server/go.mod](server/go.mod)); `sqlc` **v1.31.1** (CI-pinned — install exactly that version); goose v3 for the migration, run at boot by `store/migrate.go`. Frontend uses only what `package.json` already carries: React 19, React Router v8, TanStack Query v5 (used by `display-controls.tsx`'s mutation only, never by the display page), Tailwind v4. No web research was warranted: nothing here touches the Anthropic SDK, the Meta Graph API, or any library whose behavior could have drifted since the last story. The two browser APIs in play — `window.open` and `window.matchMedia('(prefers-reduced-motion: reduce)')` with `addEventListener('change', …)` — are both baseline-stable and need no polyfill or fallback.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-4.1] — story statement and all five epic ACs verbatim; Epic 4 context and its "built entirely on Epic 3's WS snapshots" implementation note
- [Source: epics.md#Story-4.2 through #Story-4.6] — what this shell must *not* build, and what each later story replaces in the switcher
- [Source: epics.md#UX-DR1, #UX-DR3, #UX-DR4] — palette, system-ui-only typography, zero external assets
- [Source: epics.md#UX-DR2] — gold's two permitted moments (neither is this story)
- [Source: epics.md#UX-DR12] — error/connection copy principles; "מתחבר..." only while the WS is down
- [Source: epics.md#UX-DR14] — `aria-live="assertive"` on state transitions; color never the sole signal; `dir="rtl"`+`lang="he"`
- [Source: epics.md#UX-DR16] — one component per display state
- [Source: …/ux-designs/…/EXPERIENCE.md#Audience-Display-stages] — the stage inventory table (what each later story owns) and the ≤300ms cross-fade
- [Source: …/EXPERIENCE.md#Audience-Display-stages → Resilience] — auto-reconnect and "מתחבר מחדש..." **over the last rendered state**
- [Source: …/EXPERIENCE.md#Audience-Display-stages → Display controls] — the "הפחת אנימציות" toggle lives on the dashboard and forces the statics for the whole room
- [Source: …/EXPERIENCE.md#Host-microcopy] — "פתח מסך קהל" verbatim; the copy rules this story's new strings are authored to
- [Source: …/EXPERIENCE.md#Host-control-panel] — "פתח מסך קהל" is available from Lobby through Game over
- [Source: …/EXPERIENCE.md#State-Patterns] — the Audience Display column per state, including "— (not yet launched)" pre-lobby and "Auto-reconnect; …over last state" on disconnect
- [Source: …/EXPERIENCE.md#Interaction-Primitives] — "Audience Display: zero interaction. Output-only. Fullscreen via the browser (F11)" (A12: no kiosk mode, no pairing flow)
- [Source: …/EXPERIENCE.md#Accessibility-Floor] — live-region discipline (assertive for transitions; the timer numeral excluded); the motion rule the room-level toggle exists to serve
- [Source: …/EXPERIENCE.md#Key-Flows Flow 1] — the Organizer drags a new window to the projector and checks the back row; Flow 5 for what the room watches
- [Source: …/DESIGN.md#Typography] — A19's projection ramp (96/64/48/40px at 1080p), "all sizes in rem so the whole ramp scales", bidi isolation on LTR tokens
- [Source: …/DESIGN.md#Layout-Spacing] — 16:9, 1920×1080 design target, 1280×720 minimum, 48px safe margin
- [Source: …/DESIGN.md#Elevation-Depth] — "Audience Display: flat. No box-shadows on any stage element."
- [Source: _bmad-output/planning-artifacts/architecture.md#Frontend-Architecture] — `/display/:gameId` as a route of the single SPA; the WS hook with `useSyncExternalStore` and backoff; snapshot-as-store
- [Source: architecture.md#Architectural-Boundaries] — "`display/*` renders exclusively from the WS snapshot (no TanStack Query)"; features never import each other; `store` the only pgx package
- [Source: architecture.md#Project-Structure] — `features/display/display-page.tsx` and the five stage files this story leaves for 4.2–4.6
- [Source: architecture.md#Authentication-Security] — "Audience Display: same-origin route opened from an authenticated dashboard session — inherits the session cookie; no pairing flow"
- [Source: architecture.md#Implementation-Patterns] — camelCase JSON / snake_case DB; direct payload for a resource; Hebrew copy centralization; the five mandatory gates
- [Source: server/internal/ws/handler.go] — `role=display` already validated and accepted; every rejection answers before the upgrade
- [Source: server/internal/ws/hub.go] — the one envelope shape, `seq` assignment, drop-on-full posture
- [Source: web/src/lib/use-game-socket.ts] — backoff, `lastSeq` guard, the null-on-close behavior this story compensates for, and `probeThenDecide`'s role
- [Source: server/internal/game/engine.go#OpenLobby] — the shape `SetDisplaySettings` mirrors; `snapshotAfterCommit`'s degradation contract
- [Source: server/internal/httpapi/control.go#handleOpenLobby] — broadcast-before-response ordering
- [Source: server/internal/httpapi/games.go#handleUpdateScoring] — the required-pointer body pattern and its documented reasoning
- [Source: server/migrations/00004_game_scoring.sql, 00009_game_question_progress.sql] — the `NOT NULL DEFAULT` precedent for a `games` column
- [Source: .github/workflows/ci.yml] — the Hebrew copy-centralization grep (and its LC_ALL trap), the `sqlc generate` diff-check, both filter-safety scans
- [Source: _bmad-output/implementation-artifacts/3-10-post-game-summary-on-the-dashboard.md] — previous story's Debug Log: the stale-server E2E trap, the gofmt CRLF separation, the `.tsx` Hebrew-comment gap, and its still-outstanding browser pass
- [Source: _bmad-output/implementation-artifacts/deferred-work.md] — the three entries Task 12 re-triages (2.3 display auth ×2, 3.1 broadcast `seq`) and the two Epic-4-named entries this story does not trigger (2.5 roster roles, 3.7 empty leaderboard)

## Dev Agent Record

### Agent Model Used

claude-sonnet-5 (Claude Code, dev-story workflow)

### Branch note

Branched from **`a4523ea`** (story 3.10's commit on `story/3-10-post-game-summary-on-the-dashboard`), **not** from the `baseline_commit` in the frontmatter. Per the story's own Prerequisite section: `origin/main` was still at `527f1ed` at implementation time (3.10 committed and pushed but not merged), so `main` would have conflicted in all five shared files. Branch: `story/4-1-audience-display-shell`.

### Debug Log References

**sqlc changed-file list (Task 2).** `sqlc generate` (v1.31.1, matching the CI pin) touched exactly **two** files: `server/internal/store/gen/games.sql.go` and `server/internal/store/gen/models.go`. Both scan `games` — `models.go` gains the `Game.ReducedMotion` field, `games.sql.go` gains the new query plus a `&i.ReducedMotion` scan line on every `RETURNING *`/`SELECT *` over `games`. No diff in `answers.sql.go`, `participants.sql.go`, `questions.sql.go` or `packages.sql.go`, none of which return `games.*` — so the churn is narrower than the story predicted, not wider. Re-ran `sqlc generate` twice more: same two files, byte-identical (idempotent). Also confirmed sqlc **does** copy a query's leading comment verbatim into the generated Go doc comment (`deferred-work.md`'s open 3.10 entry) — the new SQL comment is English, so the Hebrew gate stays clean.

**Hebrew copy-centralization gate run with a positive control (Task 11).** `export LC_ALL=C.UTF-8` applied to the shell, not to an assignment (story 3.9's silent false-pass). The unfiltered grep listed **19** files including `internal/wa/messages_he.go`, proving grep actually ran; the filtered gate printed nothing. `internal/httpapi/display.go` does not appear.

**Caught one of my own violations.** The frontend Hebrew scan initially listed **three** `.tsx` files instead of the two pre-existing ones: my new `display-controls.tsx` carried a Hebrew phrase inside a code comment (a quote from EXPERIENCE.md Flow 1). Rewritten in English; the scan now lists exactly `features/builder/scoring-editor.tsx` and `features/live/control-page.tsx`, both pre-existing and both already recorded in `deferred-work.md`.

**Two lint rules forced a deviation from the story's prescribed `lastSnapshot` code (Task 8).** The story specifies a `useRef` synced in an effect. This project's ESLint config (`react-hooks` flat recommended) rejects **both** halves of that: `react-hooks/refs` forbids reading `ref.current` during render (5 errors), and after switching to `useState` + effect, `react-hooks/set-state-in-effect` forbids the effect body. Implemented instead with React's documented "storing information from previous renders" pattern — state adjusted during render behind a `snapshot !== null && snapshot !== lastSnapshot` guard. **Behaviorally identical** (the guard only ever stores a non-null snapshot, so a null `snapshot` still renders the previous one) and verified in the browser: the reconnect check confirms the last stage stays on screen with the overlay above it. Same class of change in `stage-placeholder.tsx`: the story's `function StagePlaceholder(_: StageProps)` trips `@typescript-eslint/no-unused-vars`, so it is typed as `ComponentType<StageProps>` with no parameter, which keeps it assignable to the switcher's map.

**Tailwind arbitrary-value form verified in the built bundle, not assumed.** `text-[length:var(--stage-heading)]` does resolve in this Tailwind v4 — the built CSS contains `.text-\[length\:var\(--stage-heading\)\]{font-size:var(--stage-heading)}`. No inline-style fallback needed. Also confirmed present in `dist`: `.stage-root` with all five ramp variables, `.stage-fade`, `@keyframes stage-fade-in`, the `prefers-reduced-motion` media override, and `.stage-root[data-reduced-motion=true] .stage-fade{animation:none}`.

**The 401 test could not go where the story suggested.** `games_test.go`'s `TestGameMutationsWithoutSessionReturn401` builds its router with nil engine/hub, so `/display-settings` does not exist on it (the route is inside the guard — which `TestUpdateDisplaySettingsRouteAbsentWithoutEngine` proves) and an entry there would have asserted 401 against a 404. `control_test.go`'s `controlActionCases` table is POST-only and bodyless. Written in `display_test.go` in the same shape as `TestOpenLobbyWithoutSessionReturns401`, as the story's third option allows.

**gofmt CRLF caveat (carried since 3.4).** Bare `gofmt -l .` lists nearly every file in the repo, touched or not, because of the CRLF checkout. Re-checked by copying each touched file through `sed 's/\r$//'` into a temp dir and running `gofmt -l` there: **no real drift** in any of them.

**E2E: 30/30 against real Postgres.** `server/cmd/e2escratch` (created, run, deleted). `WHATSAPP_API_BASE_URL` and `ANTHROPIC_API_BASE_URL` overridden to a local dead address, so **no real WhatsApp traffic left the machine** (0 `graph.facebook.com` in the log; the only matches for the fake URL are the two override-confirmation log lines, no send attempts). Per 3.10's lesson, the server binary was built once and exec'd directly, and the log was checked for exactly **one** `server listening` and **zero** `bind:` errors before any result was trusted. The migration applied at boot (`migrations applied count=1`). Scratch organizers and game removed afterwards.

**Browser pass: 39/39 in real Edge**, `playwright-core` + `channel: 'msedge'` (installed into the scratchpad, never into `web/package.json`); driver deleted. **The driver caught a real defect on its first run**: `DisplayControls` was mounted only in `control-page.tsx`'s `finished` branch, so both controls vanished the moment a game started — the run hung waiting for a checkbox that no longer existed. Fixed by adding the second mount point the story's Task 10 actually specifies (the main return), then re-run green.

**Three browser checks false-failed before being re-scoped; none was a product defect, and each was verified rather than excused:**

1. *"exactly one WebSocket on the display route" saw two.* React StrictMode's dev-only mount/unmount/remount opens a second handshake and closes the first. Confirmed against the server log: the **host** route does the same (2 `ws connected` for `role=host` as well), so this is pre-existing hook behavior from story 2.3, not this story's code — and 4.1 is forbidden from touching the hook. Re-scoped to count **live** sockets: 1 live of 2 handshakes. The lobby contrast control shows the identical pattern.
2. *"zero console errors" saw four.* All four come from the two deliberate negative-path pages (unknown game → 404 handshake; signed-out display → 401 handshake). Browsers log a rejected WebSocket handshake unconditionally and no application code can suppress it — those errors **are** the rejection working. Re-scoped: zero errors in the dashboard and display windows through the whole flow, and the four negative-path errors asserted to be exactly the expected 404/401 handshake reports.
3. *Playwright's `locator.check()` failed with "clicking the checkbox did not change its state".* Correct and by design: the checkbox is driven by `props.reducedMotion` from the snapshot with no local state, so React reverts the DOM until the broadcast lands. `check()` asserts a synchronous flip. Switched to `.click()` plus a poll — and added an assertion that the checkbox ends up checked, which makes the round trip itself a verified behavior rather than an obstacle.

**`go test -race` is unavailable in this environment** — `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1`. **No race coverage was obtained.** This story adds no goroutine and no shared mutable state of its own (the one new handler is fully synchronous and spawns nothing), so the loss is bounded, but it is a real gap and not a claim of safety.

### Completion Notes List

**AC-1 — separate window, output-only, RTL.** The launch CTA calls `window.open('/display/<id>', '_blank', 'noopener')`. Verified in a real browser: a new window opens at `/display/{id}`, the dashboard window stays on the lobby, the root is `dir="rtl"`, and the surface has **zero focusable elements**, zero `nav`/`aside` chrome, zero box-shadows (DESIGN.md: the display is flat), `overflow: hidden`, and fills the viewport. The route is top-level in `app.tsx`, outside both `RequireAuth` and `DashboardLayout`.

**AC-2 — the snapshot is the only data source.** `display-page.tsx` imports no TanStack Query and calls no `api()`. Measured in the browser: **zero** `/api/` fetch/xhr requests on the display route and exactly one live app WebSocket. Proven to be a real absence rather than a dead page by a contrast control in the same run — the lobby route does make `/api/` calls and does open its own socket. The stage switcher is `Record<GameState, ComponentType<StageProps>>`, so adding a state to the union is a build error rather than a blank projector.

**AC-3 — transitions land well inside 1 second.** Four dashboard-driven transitions measured end to end (action fired → display's announcer text changed): **35ms / 25ms / 32ms / 26ms**, against a 1000ms budget. The E2E separately proves the server half: the display socket received exactly one frame per transition across all seven state changes, in order, with non-decreasing `seq`, each cross-checked against an independent `Engine.Snapshot` read rather than a guess.

**AC-4 — reconnect keeps the room's screen alive.** With the display open, the server was killed: `"מתחבר מחדש…"` appeared over the **still-rendered last stage** (never a blank screen), and on restart the display re-rendered current state with **no interaction** and the overlay disappeared. Separately, on a first connect the `"מתחבר…"` copy appears and is then **gone while the socket is up** — the literal AC. The reconnect machinery itself is the untouched story-2.3 hook; this story adds only the last-snapshot retention the hook deliberately does not do. Palette and typography verified on the real surface: ground is `#F0FDF4`, **no gold anywhere**, and **zero requests to any external host** from any window.

**AC-5 — the room-level setting rides the snapshot.** A `games.reduced_motion` column (migration 00016), a PUT route that broadcasts before responding, and the field on `Snapshot` — so the display obeys it with no display-side interaction. Verified both ways: in the browser, ticking the dashboard toggle flipped the display's `data-reduced-motion` to `true` and killed the cross-fade with zero interaction on the display window, a reload kept it on, and unticking flipped it back; in the E2E, an **already-open** display socket received the broadcast, and a **reconnected** socket's *initial* frame still carried `true` — which is only possible if the value came from the column. `emptySnapshot` carries it too (`TestEmptySnapshotCarriesDisplaySettings` is the only thing that can catch that line going missing).

**Absence is a 400, `false` is a value.** `reducedMotion` is a required pointer: `{}` is rejected before the engine is ever called (asserted on the recorded-call slice, not just the status code), while `{"reducedMotion": false}` reaches the engine as a deliberate `false`. Both halves have their own test, plus an E2E check on the real endpoint.

**Scope discipline.** `NewRouter`'s 11-parameter signature is unchanged (no `router_test.go` diff). `use-game-socket.ts`, all of `server/internal/ws/`, all of `wa/`, and `messages_he.go` are untouched. No gold class anywhere in the diff. No new npm package, no `shadcn add`, no frontend test framework. No state-machine change.

**Task 12 outcomes (recorded in `deferred-work.md`, no code).** Both 2.3 display-auth entries **re-deferred and merged**, with the trigger re-pointed away from a story number to the condition that actually matters: *a display running on a machine the organizer does not physically control*. The 3.1 broadcast-`seq` entry is **surfaced for Avraham, not decided** — recommendation is re-defer (the race still needs two concurrent organizer writers, and 4.1 adds a reader, not a writer), with a new trigger of "a second organizer surface, or the first report of a stuck stale state"; **please confirm or overrule at code review.** Two further Epic-4-named entries were checked and explicitly marked **not triggered** (2.5 roster roles — this shell renders no roster; 3.7 empty leaderboard — that trigger is `leaderboard-stage.tsx`, i.e. story 4.5, which must resolve it before rendering a leaderboard to a room).

### Code review outcome (2026-08-09)

Three parallel review layers (Blind Hunter / Edge Case Hunter / Acceptance Auditor). 4 decisions resolved by Avraham, 17 patches applied, 3 items deferred, 7 findings dismissed. All gates re-run green after the patches: `go vet`, `go test -count=1 ./...`, the Hebrew gate with a 19-file positive control, `tsc -b`, `npm run lint`, `npm run build`, both filter-safety scans.

**Four crash-or-corruption paths were closed.** (1) `SetDisplaySettings` no longer routes through `snapshotAfterCommit`: it was the only non-transition write using the degrade-to-`emptySnapshot` path, which meant a transient `buildSnapshot` failure let a *cosmetic toggle* broadcast a blanked roster / null question / empty leaderboard to the whole room — on a `finished` game too, which never re-broadcasts to self-correct. It now propagates the build error (new `detachedSnapshot` helper keeps the `WithoutCancel` + 5s budget for both callers; `snapshotAfterCommit` keeps the degradation, which is right for a committed transition). (2) The four `snapshot.displaySettings.reducedMotion` derefs are optional-chained — `store/migrate.go` documents redeploys running two instances, and there is **no error boundary anywhere in `web/src`**, so a snapshot from a draining old instance put React Router's English crash page on the projector *and* on the organizer's live panel. (3) `stageByState[state]` falls back to the placeholder for a state the client's union lacks; the `Record<GameState, …>` compile-time exhaustiveness check that 4.2–4.6 rely on is untouched. (4) The reduce-motion checkbox got the project's standard `submittingRef` in-flight guard — with no local echo it snaps back for a whole round trip, which reads as broken and reliably earns a second click, and two concurrent PUTs then race to decide both the persisted value and the last broadcast.

**The launch CTA was wrong on three counts at once**, all from one features string. `noopener` forces the target to `_blank` per spec, so the named-target reuse the code appeared to want never happened and every repeat click spawned another socket-holding window; `noopener` also makes `window.open` return `null` unconditionally, so a popup-blocked launch was undetectable *in principle* and the organizer got no feedback in front of a room; and a noopener-only features string is browser-dependent as to whether it yields a window or a tab, which AC-1 and Flow 1 both require to be a window. Now: named target `wc-display-${gameId}`, explicit `popup=yes,width=1280,height=720`, no `noopener` (the opened page is our own same-origin output-only surface and never touches `window.opener`), and a null return raises `strings.live.openDisplayBlocked`.

**Accessibility, corrected on the story's own terms.** The announcer carried both `role="status"` (implicitly polite) and `aria-live="assertive"` — contradictory, and AT-dependent. Worse, it was absent from the `notFound` and `connecting` branches, so it was inserted into the DOM *already holding its first text*, which screen readers do not announce: the single most important announcement, "the display just came up in `lobby`", was silently dropped and UX-DR14 was only satisfied from the second transition onward. The region is now bare `aria-live="assertive"`, rendered in every branch, empty until the first snapshot lands.

**Two things the shell was about to lock in for five later stories.** The reconnect band was `top-0`, which resolves against the padding box — i.e. the outer edge, deliberately outside `--stage-margin`, whose entire purpose is projector overscan tolerance. The one element telling a room its screen has gone stale was the one element a projector could clip; it is now offset by the margin. And the new stage CSS was unlayered: unlayered rules outrank every layered one regardless of specificity, and Tailwind v4 utilities live in `@layer utilities`, so any `animation-*` utility a 4.2–4.6 stage put on a `stage-fade` element would have been silently overridden with no visible specificity conflict. The animation rules now sit in `@layer components` (verified in the built bundle: `@layer components{.stage-fade{…}}`, ahead of `@layer utilities`). The custom properties stay unlayered deliberately.

**Also fixed:** the retained snapshot is now keyed by `gameId`, so navigating `/display/A` → `/display/B` in one window no longer renders *and assertively announces* game A's stage under a band implying B is reconnecting (`DisplayPage` is not remounted on a param change); and the mutation error banner is reset on a live-state change via the same latest-ref pattern `control-page.tsx` uses, so one transient 503 no longer leaves "שינוי ההגדרה לא נשמר" on the panel for the rest of the game. `DisplayControls` gained a `gameState` prop for that reset; the popup-blocked banner expires the same way, stored as the state it was raised in rather than a bare boolean so no effect is needed (`react-hooks/set-state-in-effect` rejects the effect form).

**Three test gaps closed.** `TestUpdateDisplaySettingsWithoutSessionReturns401` now sends a body the handler would otherwise accept — bodyless, it passed whether auth ran before or after validation, so it pinned nothing. `TestUpdateDisplaySettingsRouteAbsentWithoutEngineOrHub` exercises nil-engine and nil-hub separately, which the both-nil version could not distinguish and so never proved the `&&`. `TestUpdateDisplaySettingsBroadcastsBeforeWritingTheResponse` asserts the ordering `display.go`'s doc comment gives as the reason for its shape (the recorder's body length is 0 at `Broadcast` time, with a positive control that the body is non-empty at the end). Plus `TestUpdateDisplaySettingsMalformedBodyReturns400` over four shapes, and `TestSetDisplaySettingsDoesNotDegradeOnBuildFailure` guarding the engine change above.

**Task 12 was materially incomplete and is now finished.** The story enumerated "three triggered, two not"; there were **ten** open `deferred-work.md` entries naming Epic 4. The seven missed ones now each carry a written outcome: 2.4 lobby counts (not triggered — 4.2 triggers it), 3.1 control mutations discarding the REST snapshot (triggered, re-deferred, and *worsened* by this story's controlled checkbox), 3.4 / 3.7 / 3.10 integration tier (triggered, re-deferred as one family under a trigger that no longer names an epic), 3.8 reveal→next pacing (not triggered — 4.4 triggers it), 3.10 games list (not triggered; the epic reference was imprecise and is dropped). The `seq` entry's re-triage had its reasoning **struck through and corrected rather than rewritten**: "4.1 adds no new writer" and "the race needs two organizer sessions" were both false, and the record now says so.

**Note for whoever fixes the `seq` race:** `games.updated_at` is no longer a valid commit-ordered source — `UpdateGameDisplaySettings` bumps it on a write that changes no state. Recorded as its own deferred entry.

**`go test -race` remains unavailable** (`CGO_ENABLED=0`). The review added no goroutine.

### Post-review browser pass (2026-08-09) — 60/60

Run against the patched tree in real Edge (`playwright-core` + `channel: 'msedge'`, installed into the scratchpad, never into `web/package.json`; drivers deleted). Two harnesses: **48/48** for the main flow, **12/12** for reconnect. Real Postgres, real WS, real session cookie. `WHATSAPP_API_BASE_URL` and `ANTHROPIC_API_BASE_URL` overridden to a dead local address — **zero `graph.facebook.com` in the logs**, 4 override confirmations, exactly one `server listening` per process and zero `bind:` errors, so no stale server answered.

Every review patch was verified on the real surface, not argued: the named window is genuinely reused (a second launch click opens no second page, and no popup-blocked banner appears — proving `window.open` returned a real window rather than `noopener`'s unconditional `null`); the in-flight guard collapses a rapid triple-click to **1** PUT; the announcer is `aria-live="assertive"` with **no** `role="status"`, is `sr-only`, and **is mounted on the notFound branch too** (empty, so a later state change is a real mutation the AT will announce); the error banner appears on a stubbed 503 and clears when the state advances underneath the same mounted component; navigating `/display/A` → `/display/B` never renders or announces A's stage; and the cross-fade resolves to `stage-fade-in` normally and `none` under reduced motion, which is the layered CSS working.

AC evidence: **AC-1** — separate window at `/display/:id`, `dir="rtl"`, **0** focusable elements, 0 nav/aside/header, 0 box-shadows, `overflow: hidden`, dashboard stays on the lobby. **AC-2** — **zero** `/api/` requests from the display route, exactly **1 live** app socket of 2 handshakes (StrictMode opens and closes one), confirmed `role=display`, with two controls proving the measurement is real: the lobby route *does* call `/api/`, and the Vite HMR socket is present but correctly excluded. **AC-3** — 76 / 74 / 53 / 53 ms against a 1000 ms budget. **AC-4** — the server was **actually killed** (`Stop-Process`); the socket was asserted closed, the last stage stayed on screen, the announcer held the last state, the first-connect copy never appeared, and the display recovered with **zero interaction** when the server came back. **AC-5** — the dashboard toggle flips `data-reduced-motion` with no display interaction, survives a display reload (so it is read from the column), and unticks. Plus: 64.0013px heading at 1920 (A19's 64px), margin scaling 48px→32px between 1920 and 1280 with no horizontal scrollbar at either, **no gold rendered and the token not even emitted** (Tailwind tree-shakes it — nothing references it), zero external hosts, unauthenticated display self-heals to `/login`, and zero non-WebSocket console errors.

**One driver artifact worth recording so the next story does not repeat it:** `ctx.setOffline(true)` does **not** close an already-established WebSocket, so a reconnect test built on it passes vacuously — the band never appears because the socket never drops. The reconnect harness kills the Go server for real and asserts `ws.on('close')` fired before trusting anything else.

**🔴 The browser pass found a severe PRE-EXISTING bug, recorded in `deferred-work.md` and recommended as the next story:** the live control panel dies after exactly one action. `fire()`'s `submittingRef` is cleared only by the `onSettled` passed to `mutate()`, and the state-change effect's `action.reset()` discards that callback when the WS broadcast beats the HTTP response — which the deliberate broadcast-before-response ordering makes the normal case. Sequence observed: `close-question` works and poisons the ref, `reveal` is then silently swallowed with no request and no error; a reload fixes it. **Reproduced against `HEAD` with `control-page.tsx` restored from `a4523ea`**, so it is not 4.1's regression — but 4.1 makes it worse to experience, because the projector now freezes on the same stage in front of a room. Not patched here: the story explicitly protects `fire()`, the ref guard and the reset effect, and the fix is a genuine design choice.

**Open question for review — resolved.** All new Hebrew copy in `strings.he.ts` outside the two verbatim EXPERIENCE.md rows (`פתח מסך קהל`, `הפחת אנימציות`) is marked `[ASSUMPTION]` in the file, per the convention `live.stopConfirm*` and `results` already use — the reconnect/waiting/notFound/announcer strings are authored to the Host-microcopy rules, not quoted from a spec. Worth a read-through. **Avraham approved the copy as written at the code review (2026-08-09)**; the markers are removed and the launch-CTA comment now records that EXPERIENCE.md's own row is `[ASSUMPTION]`-marked with a rejected alternative.

### File List

**New:**
- `server/migrations/00016_game_display_settings.sql`
- `server/internal/httpapi/display.go`
- `server/internal/httpapi/display_test.go`
- `web/src/features/display/display-page.tsx`
- `web/src/features/display/stage-placeholder.tsx`
- `web/src/features/live/display-controls.tsx`

**Modified:**
- `server/internal/store/queries/games.sql`
- `server/internal/store/gen/games.sql.go` (sqlc output)
- `server/internal/store/gen/models.go` (sqlc output)
- `server/internal/store/games.go`
- `server/internal/game/snapshot.go`
- `server/internal/game/engine.go`
- `server/internal/game/engine_test.go`
- `server/internal/httpapi/control.go`
- `server/internal/httpapi/control_test.go`
- `server/internal/httpapi/router.go`
- `web/src/index.css`
- `web/src/lib/types.ts`
- `web/src/lib/strings.he.ts`
- `web/src/app.tsx`
- `web/src/features/lobby/lobby-page.tsx`
- `web/src/features/live/control-page.tsx`
- `_bmad-output/implementation-artifacts/deferred-work.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `_bmad-output/implementation-artifacts/4-1-audience-display-shell-the-screen-that-follows-the-game.md` (this file)

**Created then deleted (per convention):** `server/cmd/e2escratch/main.go`

**Also modified by the code review (2026-08-09):** `server/internal/game/engine.go` (+`detachedSnapshot`, `SetDisplaySettings` no longer degrades) · `server/internal/game/engine_test.go` (+1 test) · `server/internal/httpapi/display_test.go` (+2 tests, 2 hardened) · `web/src/features/display/display-page.tsx` · `web/src/features/live/display-controls.tsx` · `web/src/features/live/control-page.tsx` · `web/src/features/lobby/lobby-page.tsx` · `web/src/index.css` · `web/src/lib/strings.he.ts` (+`openDisplayBlocked`) · `_bmad-output/implementation-artifacts/deferred-work.md`

## Change Log

- 2026-08-09: Story created (create-story workflow). Status: ready-for-dev.
- 2026-08-09: Story 4.1 implemented — the Audience Display shell (FR-9). One migration + one query + one store wrapper for the room-level `reduced_motion` column; one `Snapshot` field and one non-transition `Engine` method; one PUT route inside the existing engine/hub guard (`NewRouter`'s arity untouched); the projection type ramp and stage cross-fade in `index.css`; the `/display/:gameId` route outside both `RequireAuth` and `DashboardLayout`; the stage-switcher shell with last-snapshot retention, the reconnect overlay, the `aria-live` announcer and the two-source reduced-motion plumbing; and one dashboard controls component behind three mount points. 6 new files, 16 modified. **10 new Go tests** (3 engine, 7 handler) all passing, full backend suite green, `go vet` clean, `sqlc` diff confined to 2 files and idempotent, frontend lint/tsc/build clean, both filter-safety scans clean, Hebrew centralization gate clean in both languages (with positive control). **Local E2E 30/30** against real Postgres (settings round-trip through a real display socket incl. a reconnect, all 7 transitions cross-checked against independent snapshots, foreign-organizer handshake rejected 404 pre-upgrade); harness deleted and scratch rows removed. **Browser pass 39/39 in real Edge** across all 11 specified checks — it caught and closed one real defect (the missing second `DisplayControls` mount point). `go test -race` unavailable (`CGO_ENABLED=0`) — no race coverage obtained. Deviated from the story's prescribed `useRef` retention pattern because two `react-hooks` lint rules reject it; behaviorally identical replacement documented in the Debug Log. Status: review.
- 2026-08-09: Code review (bmad-code-review, three parallel layers). 4 decisions resolved by Avraham, **17 patches applied**, 3 items deferred, 7 findings dismissed. Closed four crash-or-corruption paths (a cosmetic toggle could broadcast a blanked `emptySnapshot` to the whole room; an unguarded `displaySettings` deref crashed the projector *and* the live panel on a rolling deploy, with no error boundary anywhere; an unknown `state` threw "Element type is invalid"; the checkbox had no in-flight guard against a double PUT). Rebuilt the launch CTA's window semantics (`noopener` was defeating its own named target, making the return value permanently `null` and the tab-vs-window outcome browser-dependent). Fixed the announcer's contradictory ARIA and its never-announced first state. Moved the reconnect band inside the projector safe margin and put the stage animation CSS in `@layer components` — both contracts 4.2–4.6 inherit. Keyed the retained snapshot by `gameId`; reset the error banner on a state change. Added 7 Go tests (malformed-body table, split engine/hub route guard, broadcast-ordering assertion, no-degrade-on-build-failure) and hardened the bodyless 401 test. **Finished Task 12**, which had triaged 3 of 10 Epic-4-triggered `deferred-work.md` entries — and corrected the `seq` re-triage, whose two stated premises this story itself falsified. Hebrew copy confirmed; `[ASSUMPTION]` markers removed. All gates re-run green. `go test -race` still unavailable (`CGO_ENABLED=0`). Status: done.
- 2026-08-09: Post-review browser pass in real Edge — **60/60** (48/48 main flow + 12/12 reconnect against a genuinely killed server), zero outbound WhatsApp traffic, every review patch verified on the real surface. Also closes story 4.1's own manual-pass gate. **Found a severe pre-existing defect while driving it: the live control panel goes dead after one action** (`submittingRef` never cleared because the state-change `action.reset()` discards the `onSettled` that clears it, whenever the WS broadcast beats the HTTP response). Reproduced against `HEAD` with 4.1's `control-page.tsx` reverted, so it is a `main` bug, not this story's — recorded in `deferred-work.md` as the top-severity item and recommended as the next story, ahead of 4.2.
