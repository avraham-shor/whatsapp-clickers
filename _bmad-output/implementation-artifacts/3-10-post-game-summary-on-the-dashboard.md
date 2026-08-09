---
baseline_commit: efe4fc908f2fbfb80ef0ae37ea01cf55528b28e2
---

# Story 3.10: Post-Game Summary on the Dashboard

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an Organizer,
I want a results summary when the Game ends,
so that I can review how the event went (FR-14).

## ⚠️ Prerequisite: branch from 3.9's work, not from `main`

At the time this story was created, **story 3.9 (Final Results for Everyone) is implemented but NOT committed** — its work sits uncommitted on branch `story/3-9-final-results-for-everyone`, and `main` is at `efe4fc9` (story 3.8's merge, PR #6). The `baseline_commit` above is that `main` tip because 3.9 has no commit yet; it is **not** the tree you should branch from.

**Branch from wherever 3.9's work actually sits** when you start — its commit on `story/3-9-final-results-for-everyone`, or `main` once 3.9 has merged. Branching from `main` while 3.9 is uncommitted gives you a `NewRouter` with **10** parameters, and every `NewRouter(...)` call site in the test files at 10 arguments — this story adds a route through `router.go` and a stub method to `games_test.go`'s `stubGames`, both of which sit in files 3.9 rewrote. You would get a guaranteed conflict plus a wrong argument count.

Verify these shapes on disk before writing any code (they are 3.9's/3.8's/3.7's landed state):

- `server/internal/httpapi/router.go` — `NewRouter` takes **11** positional args, ending `dispatcher QuestionDispatcher, resultDispatcher ResultDispatcher, finalDispatcher FinalDispatcher`. **This story does not change that arity.**
- `server/internal/httpapi/games.go` — `GameStore` interface with 11 methods, ending `ImportPackageQuestions`; `gameIDParam`, `requireOrganizer`, `requireDraftGame` helpers present.
- `server/internal/httpapi/router.go` — inside `gr.Route("/{gameID}", ...)`: `gr.Get("/", handleGetGame(games))` and `gr.Put("/scoring", handleUpdateScoring(games))` sit **outside** the `if engine != nil && hub != nil` block. This story's route joins them there.
- `server/internal/game/scoring.go` — `RankLeaderboard(scores []store.ParticipantScore) []LeaderboardEntry`, standard competition ranks (1,1,3), `LeaderboardEntry` carrying camelCase JSON tags.
- `server/internal/store/answers.go` — `GetLeaderboard(ctx, gameID) ([]store.ParticipantScore, error)` and `ParticipantScore{ParticipantID, DisplayName, Score}`.
- `server/internal/store/queries/answers.sql` — `GetLeaderboard` with its `p.role = 'player'` filter.
- `web/src/features/live/control-page.tsx` — the `snapshot.state === 'finished'` early-return branch rendering `strings.live.gameOverTitle` + a back link.
- `web/src/app.tsx` — three authenticated child routes (`index`, `games/:gameId`, `games/:gameId/lobby`) under `DashboardLayout`.

If 3.9 has since merged or changed further, re-confirm the quoted signatures are still accurate before relying on them.

## Acceptance Criteria

1. **Given** a finished Game, **when** the Organizer views its results page, **then** they see the final Leaderboard (every player-role Participant, ranked, equal scores sharing a rank) and per-question response rates. *(epic AC-1)*
2. **Given** the pilot scope, **then** there is **no** export/download control anywhere on the surface (deferred to the commercial phase; UX-DR10 note, and EXPERIENCE.md's IA explicitly lists the "הורד תוצאות" CTA as removed). *(epic AC-2, first half)*
3. **Given** a finished Game, **then** the results persist and stay reachable — a direct URL, a full page reload, navigating away and back, or signing out and in again all render the same summary; nothing about it depends on a live WebSocket or on being the session that ran the game. *(epic AC-2, second half)*

**Scope boundaries for this story:**

- **No migration.** Every number on this page is derived from rows stories 1.3, 2.4, 2.5, 3.3, 3.4–3.7 already persist. If you write a `server/migrations/*.sql` file you have gone off-spec.
- **One new SQL query, one new `store` wrapper, and that is all the persistence surface.** `sqlc generate` produces a diff for exactly that query and nothing else. (This is the one gate that differs from 3.9, where an empty diff was the requirement.)
- **No `game` engine change.** No new `game.Store` interface line, no `Engine` method, no state transition. This page is a read, not a transition; `game` is imported by `httpapi` only for `RankLeaderboard` and `StateFinished`, both of which already exist.
- **No `wa/` change and no `messages_he.go` change.** Nothing here is participant-facing; no WhatsApp message is sent, changed, or read. Story 3.9 owns the game-end messaging half of FR-18.
- **No `ws`/snapshot change.** The results page fetches over REST via TanStack Query. It must not open a WebSocket, must not extend `Snapshot`, and must not consume `snapshot.leaderboard` — see Dev Notes, "Why REST and not the snapshot".
- **No Audience Display work.** The winner takeover is Epic 4 (story 4.6).
- **No Vitest.** No frontend test framework exists; this story does not introduce one (every prior frontend story's precedent — see Testing standards).

## Tasks / Subtasks

- [x] **Task 1: `store/queries/answers.sql` — one new aggregate query** (AC: 1)

  Append after `GetLeaderboard` (the file's other game-scoped aggregate; its neighbours `CountAnswersByQuestion`/`ListAnswerResultsForQuestion` are the per-question answer aggregates this one generalizes):

  ```sql
  -- Per-question post-game response stats for the Organizer's results
  -- summary (FR-14, story 3.10). One row per Question in the game,
  -- including Questions that received no answers at all — the LEFT JOIN is
  -- what makes a never-answered Question appear with 0 rather than vanish,
  -- and a game stopped early leaves several of those.
  --
  -- Organizer-scoped through games.organizer_id, same posture as
  -- ListQuestionsByGame (questions.sql): a foreign game is unreachable by
  -- construction, not merely unauthorized at the handler.
  --
  -- correct_count deliberately uses FILTER (WHERE a.is_correct), which
  -- excludes NULL: an answer still ungraded (stage IS NULL, story 3.6's
  -- async AI path) counts as answered but not as correct. That is the
  -- honest reading — the same row would read as a false "wrong" through a
  -- bare `.Bool`, the posture deferred-work.md records against
  -- ListAnswerResultsForQuestion. A finished game normally has none, since
  -- Reveal is gated on zero pending grades, but a game stopped from
  -- question_open can absolutely leave some.
  --
  -- No ORDER BY q.created_at ambiguity to resolve here, unlike
  -- ListAnswersForScoring/ListAnswerResultsForQuestion: those pick ONE
  -- question by position and needed a LIMIT 1 subquery to do it. This one
  -- lists every question, so two rows sharing a position (00003
  -- deliberately declines UNIQUE (game_id, position)) simply appear as two
  -- rows, ordered identically to ListQuestionsByGame.
  -- name: ListQuestionResponseStats :many
  SELECT q.id AS question_id, q.position, q.type, q.text,
         count(a.id)::int AS answered_count,
         (count(a.id) FILTER (WHERE a.is_correct))::int AS correct_count
  FROM questions q
  JOIN games g ON g.id = q.game_id
  LEFT JOIN answers a ON a.question_id = q.id
  WHERE q.game_id = sqlc.arg(game_id) AND g.organizer_id = sqlc.arg(organizer_id)
  GROUP BY q.id
  ORDER BY q.position, q.created_at;
  ```

  - [x] Run `sqlc generate` (CI-pinned **v1.31.1**) and confirm it emits `ListQuestionResponseStats` + `ListQuestionResponseStatsRow` + `ListQuestionResponseStatsParams` into `internal/store/gen/answers.sql.go`, and **nothing else changed**. Commit the generated file — the CI `sqlc generate diff-check` step regenerates and fails on any diff.
  - [x] **If sqlc v1.31 rejects the aggregate**, the two known-fragile constructs and their fallbacks, in order: (a) the parenthesized `FILTER` cast → `COALESCE(sum(CASE WHEN a.is_correct THEN 1 ELSE 0 END), 0)::int AS correct_count`; (b) `GROUP BY q.id` alone → `GROUP BY q.id, q.position, q.type, q.text, q.created_at`. Apply the minimum change that generates, and leave a one-line comment saying which fallback was needed and why. Do **not** solve a generation failure by moving the aggregation into Go — the point of one query is one round trip.

- [x] **Task 2: `store/answers.go` — the wrapper** (AC: 1)

  Append after `GetLeaderboard` (keep the wrapper next to the query's neighbour, matching the file's existing ordering):

  ```go
  // QuestionResponseStats is one Question's post-game response summary —
  // how many Participants answered it and how many of those were correct
  // (FR-14, story 3.10). The response *rate*'s denominator is not here:
  // it is the game's player count, which the caller already holds from
  // GetLeaderboard (one row per role='player' Participant), so this query
  // does not re-derive it per question.
  type QuestionResponseStats struct {
      QuestionID    string
      Position      int32
      Type          string
      Text          string
      AnsweredCount int32
      CorrectCount  int32
  }

  // ListQuestionResponseStats returns one row per Question in gameID, in
  // the same order as ListQuestionsByGame, including Questions nobody
  // answered. Organizer-scoped in SQL; a foreign or missing game yields an
  // empty slice, not an error (a :many with no rows is not ErrNoRows) —
  // the caller has already resolved ownership via GetGameForOrganizer, so
  // this is belt-and-braces rather than the authorization check itself.
  func (s *Store) ListQuestionResponseStats(ctx context.Context, gameID, organizerID string) ([]QuestionResponseStats, error) {
      rows, err := s.q.ListQuestionResponseStats(ctx, gen.ListQuestionResponseStatsParams{GameID: gameID, OrganizerID: organizerID})
      if err != nil {
          return nil, err
      }
      out := make([]QuestionResponseStats, 0, len(rows))
      for _, r := range rows {
          out = append(out, QuestionResponseStats{
              QuestionID:    r.QuestionID,
              Position:      r.Position,
              Type:          r.Type,
              Text:          r.Text,
              AnsweredCount: r.AnsweredCount,
              CorrectCount:  r.CorrectCount,
          })
      }
      return out, nil
  }
  ```

  - [x] Confirm the generated row field names before copying this verbatim — sqlc names them from the SQL aliases (`QuestionID`, `Position`, `Type`, `Text`, `AnsweredCount`, `CorrectCount`). Adjust the assignment, never the SQL aliases.
  - [x] **No `store/games.go` or `store/participants.go` change.** A diff there means you added a query you did not need.

- [x] **Task 3: `httpapi/games.go` — two lines on the `GameStore` interface** (AC: 1)

  **🚨 Zero Hebrew characters in every `.go` file this story touches, comments included** — see Dev Notes' guardrail. All user-facing copy is `strings.he.ts`.

  - [x] `GameStore` gains two methods (append at the end of the interface, after `ImportPackageQuestions`):
    ```go
    GetLeaderboard(ctx context.Context, gameID string) ([]store.ParticipantScore, error)
    ListQuestionResponseStats(ctx context.Context, gameID, organizerID string) ([]store.QuestionResponseStats, error)
    ```
    `*store.Store` already satisfies both (`GetLeaderboard` since story 3.7). Widening the existing interface is deliberate: the alternative — a separate `ResultsStore` threaded through `NewRouter` — would mean a **12th** positional parameter and touching all 33 test call sites again, for a store the router already receives.
  - [x] Update `GameStore`'s doc comment to note it now also covers the post-game read surface (one clause, not a paragraph).

- [x] **Task 4: `httpapi/results.go` (new file) — the one endpoint** (AC: 1, 2, 3)

  **🚨 Same zero-Hebrew rule.**

  Architecture names this exact file for FR-14 (`httpapi/results.go` — "post-game summary (FR-14)"). Create it; do not put the handler in `games.go`.

  ```go
  package httpapi

  import (
      "context"
      "net/http"
      "time"

      "github.com/avraham-shor/whatsapp-clickers/internal/game"
  )

  // questionStatsPayload is one Question's row of the post-game summary
  // (camelCase wire format). No omitempty anywhere: 0 answered is a real,
  // meaningful value on this page - it is exactly what a Question the game
  // never reached looks like - and must serialize.
  type questionStatsPayload struct {
      ID            string `json:"id"`
      Position      int32  `json:"position"`
      Type          string `json:"type"`
      Text          string `json:"text"`
      AnsweredCount int32  `json:"answeredCount"`
      CorrectCount  int32  `json:"correctCount"`
  }

  // resultsPayload is the whole post-game summary in one direct payload
  // (no {"items":...} wrapper - that convention is for plain lists; this
  // is a composite resource, same posture as gameDetailPayload).
  //
  // Leaderboard reuses game.LeaderboardEntry rather than redeclaring a
  // near-identical struct: it already carries the camelCase JSON tags and
  // is already the wire shape the Snapshot serializes, so the dashboard
  // sees one leaderboard shape whether it arrives over WS or over REST.
  //
  // PlayerCount is the response-rate denominator, and it is len(leaderboard)
  // by construction, not a separate count: GetLeaderboard LEFT JOINs every
  // role='player' Participant, so it returns exactly one row each - a
  // player with no answers included, a Spectator excluded. It is sent
  // explicitly anyway so the client renders "X out of Y" without having to
  // know that identity.
  type resultsPayload struct {
      GameID      string                 `json:"gameId"`
      Title       string                 `json:"title"`
      State       string                 `json:"state"`
      PlayerCount int                    `json:"playerCount"`
      Leaderboard []game.LeaderboardEntry `json:"leaderboard"`
      Questions   []questionStatsPayload `json:"questions"`
  }

  // handleGameResults serves the Organizer's post-game summary (FR-14):
  // the final ranked Leaderboard plus per-question response counts.
  //
  // Guarded on state = finished, per the epic's "Given a finished Game".
  // A game still in play answers 409 GAME_NOT_FINISHED rather than a
  // partial summary - the surface is a review artifact, not a second live
  // panel, and the live panel is where an in-progress game belongs.
  //
  // Reads only; no engine call, no transition, no broadcast. That is what
  // makes the page durable across sessions (epic AC-2): every number comes
  // from Postgres on each request, so a reload, a different browser, or a
  // fresh sign-in renders identically, with no dependence on the WS
  // connection that ran the game.
  //
  // GetLeaderboard is organizer-unscoped by design (queries/answers.sql) -
  // ownership is established one line earlier by GetGameForOrganizer,
  // whose failure is indistinguishable from a missing game (404, never
  // 403). Same trust posture as PlayerRecipients' own unscoped roster read.
  func handleGameResults(games GameStore) http.HandlerFunc {
      return func(w http.ResponseWriter, r *http.Request) {
          organizerID, ok := requireOrganizer(w, r)
          if !ok {
              return
          }
          gameID, ok := gameIDParam(w, r)
          if !ok {
              return
          }
          ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
          defer cancel()

          g, err := games.GetGameForOrganizer(ctx, gameID, organizerID)
          if err != nil {
              writeStoreError(w, err, "GAME_NOT_FOUND")
              return
          }
          if g.State != game.StateFinished {
              writeError(w, http.StatusConflict, "GAME_NOT_FINISHED", "results are available only after the game has finished")
              return
          }

          scores, err := games.GetLeaderboard(ctx, gameID)
          if err != nil {
              writeStoreError(w, err, "GAME_NOT_FOUND")
              return
          }
          stats, err := games.ListQuestionResponseStats(ctx, gameID, organizerID)
          if err != nil {
              writeStoreError(w, err, "GAME_NOT_FOUND")
              return
          }

          // Non-nil so the wire always carries [], never null - the TS
          // mirror types both as non-nullable arrays (gameDetailPayload's
          // own discipline).
          questions := make([]questionStatsPayload, 0, len(stats))
          for _, s := range stats {
              questions = append(questions, questionStatsPayload{
                  ID:            s.QuestionID,
                  Position:      s.Position,
                  Type:          s.Type,
                  Text:          s.Text,
                  AnsweredCount: s.AnsweredCount,
                  CorrectCount:  s.CorrectCount,
              })
          }
          entries := game.RankLeaderboard(scores)

          writeJSON(w, http.StatusOK, resultsPayload{
              GameID:      g.ID,
              Title:       g.Title,
              State:       g.State,
              PlayerCount: len(entries),
              Leaderboard: entries,
              Questions:   questions,
          })
      }
  }
  ```

  - [x] `RankLeaderboard` returns `make([]LeaderboardEntry, len(sorted))` — an empty **non-nil** slice for an empty game, so `Leaderboard` needs no extra guard. Confirm this on disk before relying on it (`game/scoring.go`); if it ever returns nil, add the same `make(..., 0, n)` treatment the questions slice gets.
  - [x] **No new error type, no `errors.go` change.** `GAME_NOT_FINISHED` is written inline at the one place that can produce it, exactly like `requireDraftGame`'s inline `GAME_NOT_EDITABLE`. `writeStoreError`'s switch is for domain errors returned *by* `game`/`store`; this is a handler-level state check.

- [x] **Task 5: `httpapi/router.go` — one route line** (AC: 1, 3)

  - [x] Inside `gr.Route("/{gameID}", ...)`, immediately after `gr.Get("/", handleGetGame(games))`:
    ```go
    gr.Get("/results", handleGameResults(games))
    ```
  - [x] **Outside** the `if engine != nil && hub != nil` block, deliberately: this route is a pure store read like `GET /` and `PUT /scoring`, and gating it on the engine would make it vanish in every test router that passes nil there. Do not move it inside.
  - [x] **`NewRouter`'s signature does not change.** No new parameter, no test call-site churn. If you are editing `router_test.go`'s 25 `NewRouter(...)` calls, stop — you have added a parameter you did not need.

- [x] **Task 6: `web/src/lib/types.ts` — the wire mirror** (AC: 1)

  Append after `LobbySnapshot`/`SnapshotEnvelope` (keep `LeaderboardEntry` where it is — this reuses it):

  ```ts
  /** Mirrors the Go questionStatsPayload — one Question's post-game
   * response counts (FR-14). answeredCount counts every recorded answer;
   * correctCount counts only those graded correct, so an answer still
   * ungraded is answered-but-not-correct. */
  export interface QuestionStats {
    id: string
    position: number
    type: QuestionType
    text: string
    answeredCount: number
    correctCount: number
  }

  /** Mirrors the Go resultsPayload — the whole post-game summary in one
   * REST response. playerCount is the response-rate denominator: the
   * number of player-role Participants (Spectators excluded — they never
   * answer). */
  export interface GameResults {
    gameId: string
    title: string
    state: GameState
    playerCount: number
    leaderboard: LeaderboardEntry[]
    questions: QuestionStats[]
  }
  ```

- [x] **Task 7: `web/src/lib/strings.he.ts` — the copy** (AC: 1, 2)

  **All Hebrew for this story lives here.** EXPERIENCE.md's Host-microcopy table specifies no rows for this surface (it lists only the live-control CTAs and error copy), so this copy is authored to that table's *rules* — direct and terse, errors that say what happened and what happens next, never apologizing (UX-DR12), no emoji (the 🏆 discipline reserves the trophy for the participant-facing winner moment, and this is Host copy). Mark the block with the `[ASSUMPTION]` comment convention `live.stopConfirm*` already uses, so Avraham can confirm or replace the wording.

  - [x] New `results` block, appended after `live`:
    ```ts
    // [ASSUMPTION]: EXPERIENCE.md specifies the post-game results surface
    // ("Final Leaderboard + per-question response rates on screen. No
    // export") but not its copy — authored to the Host-microcopy rules
    // (direct, terse, no apology, no emoji) and flagged for Avraham to
    // confirm/replace.
    results: {
      title: 'סיכום המשחק',
      leaderboardTitle: 'טבלת התוצאות',
      questionsTitle: 'שיעור מענה לפי שאלה',
      rankColumn: 'מקום',
      participantColumn: 'משתתף',
      scoreColumn: 'ניקוד',
      questionColumn: 'שאלה',
      answeredColumn: 'ענו',
      correctColumn: 'צדקו',
      responseRate: (answered: number, players: number) => `${answered} מתוך ${players}`,
      playerCountLabel: (count: number) =>
        count === 1 ? 'משתתף אחד שיחק' : `${count} משתתפים שיחקו`,
      noPlayers: 'אף אחד לא נרשם למשחק הזה.',
      noQuestions: 'לא היו שאלות במשחק הזה.',
      notFinished: 'המשחק עוד לא הסתיים. הסיכום יופיע כאן בסופו.',
      loadError: 'טעינת סיכום המשחק נכשלה. נסו שוב בעוד רגע.',
      viewResults: 'צפייה בסיכום',
    },
    ```
  - [x] One addition to the existing `gamesList` block, for the finished-game card (Task 10):
    ```ts
    finishedBadge: 'הסתיים',
    ```
  - [x] **Do not add any export/download string.** AC-2 is partly enforced by there being no copy for one to use.

- [x] **Task 8: `web/src/features/results/results-summary.tsx` (new file) — the summary itself** (AC: 1, 2, 3)

  One component, two mount points (the route in Task 9, the finished control panel in Task 10). It owns its own fetch so both mount points are identical.

  - [x] Shape:
    ```tsx
    interface ResultsSummaryProps {
      gameId: string
    }

    export function ResultsSummary({ gameId }: ResultsSummaryProps) {
      const results = useQuery({
        queryKey: ['games', gameId, 'results'],
        queryFn: () => api<GameResults>(`/api/games/${gameId}/results`),
      })
      // …
    }
    ```
  - [x] Query key is `['games', gameId, 'results']` — a child of the existing `['games', gameId]` key so an invalidation of the game also invalidates its results. Do not invent a separate top-level key.
  - [x] **Four render states, in this order** (mirroring `GameEditorPage`'s error handling exactly):
    - `isPending` → `strings.common.loading`.
    - `isError` + `ApiError.status === 404` → `strings.gameEditor.notFound` + a `Link to="/"` with `strings.gameEditor.backToGames` (reuse; do not add a second "not found" string).
    - `isError` + `ApiError.code === 'GAME_NOT_FINISHED'` → `strings.results.notFinished`, `role="alert"`. **Check the code, not the 409 status** — `control-page.tsx`'s `GRADING_INCOMPLETE` handling establishes exactly this precedent, and for exactly this reason.
    - any other `isError` → `strings.results.loadError`, `role="alert"`.
  - [x] **Content, on success:**
    - Heading `strings.results.title` (`<h1>` when routed, so make the heading level a prop or render `<h2>` and let each mount point own its `<h1>`; pick one and be consistent — the simplest is `<h2>` here, with the route page rendering the `<h1>`).
    - `strings.results.playerCountLabel(results.data.playerCount)`.
    - **Leaderboard**: a real `<table>` with `<caption>{strings.results.leaderboardTitle}</caption>` and `<th scope="col">` headers (`rankColumn`, `participantColumn`, `scoreColumn`); one `<tr key={entry.participantId}>` per entry, in the array's order (already rank-sorted server-side — do not re-sort on the client). Empty leaderboard → `strings.results.noPlayers` instead of an empty table.
    - **Per-question**: a second `<table>` with `<caption>{strings.results.questionsTitle}</caption>` and headers (`questionColumn`, `answeredColumn`, `correctColumn`); one row per question showing the position, the question text, `strings.results.responseRate(answeredCount, playerCount)` plus the percentage, and `correctCount`. Empty questions → `strings.results.noQuestions`.
  - [x] **Percentage, and the zero-player trap:** `playerCount === 0` must render an em-dash (or the raw count alone), never `NaN%` or a division by zero. A game whose organizer opened the lobby and stopped it with nobody joined is a completely ordinary shape here. Round with `Math.round((answeredCount / playerCount) * 100)`.
  - [x] **Bidi isolation is mandatory on every number** — ranks, scores, counts, percentages, question positions. Wrap each in `<bdi dir="ltr">…</bdi>`, exactly as `lobby-page.tsx` and `games-list-page.tsx` do for the JOIN Code. Digits inside RTL Hebrew reorder visibly without it (DESIGN.md Typography; the same rule `messages_he.go`'s `ltr()` implements server-side).
  - [x] **No export button, no download link, no CSV, no print control** (AC-2).
  - [x] Styling: existing tokens only — `text-host-text`, `text-host-text-secondary`, `border-host-border`, `bg-surface-raised`, `rounded-md`; on-scale spacing stops only (`gap-2/3/4/6`, `p-3/4/6`); `max-w-3xl` container matching every other dashboard page. No new CSS variables, no external assets (the filter-safety CI gates scan both source and the built bundle).

- [x] **Task 9: `web/src/features/results/results-page.tsx` + `app.tsx` — the durable route** (AC: 3)

  - [x] New `results-page.tsx`:
    ```tsx
    export function ResultsPage() {
      const { gameId = '' } = useParams()
      return (
        <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
          <h1 className="text-2xl font-heading text-host-text">{strings.results.title}</h1>
          <ResultsSummary gameId={gameId} />
          <Link to="/" className="text-green-800 underline">
            {strings.gameEditor.backToGames}
          </Link>
        </div>
      )
    }
    ```
  - [x] `app.tsx`: add `{ path: 'games/:gameId/results', element: <ResultsPage /> }` as a fourth child of the `DashboardLayout` route, after `games/:gameId/lobby`. It sits inside `RequireAuth` like every other dashboard route — which is what makes "sign in again and it is still there" work rather than merely not-404 (AC-3).
  - [x] The SPA fallback already serves this path: `spaHandler` returns `index.html` for any extension-less non-reserved path. No server-side routing change.

- [x] **Task 10: mount points — the finished control panel and the games list** (AC: 1, 3)

  - [x] **`web/src/features/live/control-page.tsx`** — the `snapshot.state === 'finished'` branch currently renders only `strings.live.gameOverTitle` + a back link. Render the summary in place:
    ```tsx
    if (snapshot.state === 'finished') {
      return (
        <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
          <h1 className="text-2xl font-heading text-host-text">{strings.live.gameOverTitle}</h1>
          <ResultsSummary gameId={gameId} />
          <Link to="/" className="text-green-800 underline">
            {strings.gameEditor.backToGames}
          </Link>
        </div>
      )
    }
    ```
    This is what EXPERIENCE.md's Host-control-panel table means by the Game-over row: *"— (results summary on screen)"* with a back CTA. Keep `gameOverTitle` as the heading here (the organizer just finished a game; "המשחק הסתיים" is the right register) and let the routed page use `results.title`.
  - [x] `control-page.tsx` importing from `features/results/` is a cross-feature import, which the architecture's web boundaries discourage. It is the **established** exception, not a new one: `lobby-page.tsx` already imports `ControlPage` from `features/live/`, for the same reason — one live surface composes the next as the game advances. Do not "fix" this by duplicating the summary or by promoting it to `lib/` (it is a feature view, not shared logic). Record the reading in a one-line comment at the import.
  - [x] **`web/src/features/builder/games-list-page.tsx`** — a finished game's card must lead to its summary, not to the (now pointless) editor. This is the navigation half of AC-3: a URL nobody can reach after signing in again is not "accessible".
    ```tsx
    to={game.state === 'finished' ? `/games/${game.id}/results` : `/games/${game.id}`}
    ```
    and add a visible cue next to the question count so the changed destination is not a silent surprise:
    ```tsx
    {game.state === 'finished' && <span>{strings.gamesList.finishedBadge}</span>}
    ```
    `GameListItem.state` is already on the wire (`types.ts`) and already populated by `handleListGames` — no server change is needed for this.
  - [x] **Do not** touch `game-editor-page.tsx`, `lobby-page.tsx`, `use-game-socket.ts`, `dashboard-layout.tsx`, or any `components/ui/*` file.

- [x] **Task 11: `httpapi/results_test.go` (new file) + `games_test.go` stub growth** (AC: 1, 2, 3)

  - [x] `games_test.go`'s `stubGames` gains the two `GameStore` methods and their fields (it will not compile otherwise):
    ```go
    leaderboard    []store.ParticipantScore
    leaderboardErr error
    questionStats    []store.QuestionResponseStats
    questionStatsErr error

    leaderboardFor   []string
    questionStatsFor [][2]string
    ```
    ```go
    func (s *stubGames) GetLeaderboard(ctx context.Context, gameID string) ([]store.ParticipantScore, error) {
        s.leaderboardFor = append(s.leaderboardFor, gameID)
        return s.leaderboard, s.leaderboardErr
    }

    func (s *stubGames) ListQuestionResponseStats(ctx context.Context, gameID, organizerID string) ([]store.QuestionResponseStats, error) {
        s.questionStatsFor = append(s.questionStatsFor, [2]string{gameID, organizerID})
        return s.questionStats, s.questionStatsErr
    }
    ```
    These are read from the request goroutine only (no post-response goroutine anywhere in this story), so — unlike `control_test.go`'s stubs — they need **no** mutex and no copy-under-lock accessor. Do not add one; it would be cargo-culted from a story whose concurrency this one does not have.
  - [x] Add a `finishedGame()` helper next to `draftGame()`, same shape with `State: "finished"`.
  - [x] New `server/internal/httpapi/results_test.go`, using the existing `gamesRouter(games)` + `authedRequest(...)` helpers:
    - `TestGameResultsReturnsRankedLeaderboardAndQuestionStats` — 3 players with distinct scores + 2 questions → 200; leaderboard is score-descending with ranks 1,2,3; `playerCount` is 3; `questions` carries both rows with their counts; `gotGame` records `(gameID, "org-1")`.
    - `TestGameResultsSharedRanksOnTie` — two players tied at the top plus one below → ranks are **1,1,3**, never 1,1,2 (pins `RankLeaderboard`'s standard-competition contract through the wire).
    - `TestGameResultsNotFinishedReturns409` — table-driven over `draft`, `lobby`, `question_open`, `question_closed`, `revealed`, `leaderboard` → 409 with code `GAME_NOT_FINISHED` on every one, **and** `GetLeaderboard` was never called (assert `leaderboardFor` is empty — proves the guard runs before the reads, not merely alongside them).
    - `TestGameResultsForeignOrMissingGameReturns404` — `gameErr: store.ErrNotFound` → 404 `GAME_NOT_FOUND`, and neither store read was attempted.
    - `TestGameResultsMalformedGameIDReturns404` — a non-UUID `{gameID}` → 404 `GAME_NOT_FOUND` with no store call at all (`gameIDParam` precedent).
    - **Session guard: extend the existing table, do not build a second unauthenticated router.** `games_test.go`'s `TestGameMutationsWithoutSessionReturn401` (`games_test.go:330`) already builds a `noAuth()` router and loops a map of requests; add one entry — `"game results": httptest.NewRequest(http.MethodGet, "/api/games/"+testGameID+"/results", nil)`. That is the whole change; a new `NewRouter(...)` call site in `results_test.go` would be a 12th one to keep in sync for no gain.
    - `TestGameResultsEmptyGameSerializesEmptyArrays` — finished game, no players, no questions → 200 with `"leaderboard":[]` and `"questions":[]` in the **raw body** (assert on the JSON text, not on a decoded slice — a decoded `nil` and `[]` are indistinguishable in Go, and `null` is exactly the regression this test exists to catch), and `playerCount` 0.
    - `TestGameResultsStoreErrorReturns503` — table-driven over `leaderboardErr` and `questionStatsErr` each set to a generic error → 503 `DB_UNAVAILABLE`.
    - `TestGameResultsQuestionStatsAreOrganizerScoped` — asserts `questionStatsFor` recorded `("…gameID…", "org-1")`, i.e. the handler forwards the organizer id rather than dropping it.
  - [x] **No `router_test.go`, `packages_test.go`, or `control_test.go` change.** `NewRouter`'s arity is untouched; a diff in those files means Task 5 went wrong.
  - [x] **No `game` package test change.** This story adds no `game` code.

- [x] **Task 12: Quality gates + local E2E + manual browser pass** (all ACs)

  - [x] **Hebrew copy-centralization gate — run it locally before pushing, and use a positive control.** Story 3.9's Debug Log records this gate silently false-passing when `LC_ALL` was applied to an assignment instead of to `grep`. Export it first, and print the full match list to prove grep actually ran:
    ```bash
    cd server && export LC_ALL=C.UTF-8
    grep -rlP '[\x{0590}-\x{05FF}\x{FB1D}-\x{FB4F}]' --include='*.go' . \
      | grep -v '_test\.go$' | grep -v '^\./internal/wa/messages_he\.go$'
    ```
    Expected output: **nothing**. `internal/httpapi/results.go` must not appear. Comments count — the gate cannot tell a comment from a literal, and story 3.8 needed a follow-up commit (`65fafe9`) for exactly that.
  - [x] **Frontend Hebrew centralization** — the same rule for `.tsx`: every Hebrew string in Task 7's `strings.he.ts`, zero Hebrew literals in `results-summary.tsx` / `results-page.tsx` / `control-page.tsx` / `games-list-page.tsx`. CI does not currently grep `.tsx` for this; the architecture rule binds anyway. Grep it yourself: `grep -rlP '[\x{0590}-\x{05FF}]' --include='*.tsx' web/src` should list nothing.
  - [x] Backend gates: `gofmt -l .` (the CRLF-checkout caveat carried since 3.4 still applies — LF-normalize the touched files to separate real drift from line-ending noise; 3.9 found one genuine alignment issue this way) · `go1.26.5 vet ./...` · `go1.26.5 test ./...` (zero regressions) · **`sqlc generate` diff = exactly Task 1's new query, nothing else**.
  - [x] Frontend gates: `npm run lint` · `npx tsc -b --noEmit` · `npm run build` · both filter-safety greps (source and built bundle).
  - [x] **Local Go E2E** (`cmd/e2escratch`, deleted after use — the 2.1–3.9 convention, including the fake-provider `WHATSAPP_API_BASE_URL` override so no real WhatsApp traffic leaves the machine). One scenario is enough here, but it must be a *real* finished game, not fabricated rows:

    **Scenario — a real 2-question game driven to `finished`, then read back.** Log in as `e2e-org-a`, create a scratch game with 2 questions, `open-lobby`, join **3 players** by real webhook `JOIN` messages, `POST /start`. Q1: players A and B answer correctly, C wrong. `close-question` → `reveal`. Have a **4th phone send `JOIN` mid-game** so a genuine `role='spectator'` row exists (story 2.5's path — do not insert one directly; the point is proving the denominator excludes it for real). `next-question` → Q2: only A answers, correctly. `close-question` → `reveal` → `next-question` (past the last question → `finished`). Then assert against `GET /api/games/{id}/results` with the session cookie:
    - HTTP 200; `state == "finished"`.
    - `playerCount == 3` — **the spectator is not counted**. This is the single assertion that proves the response-rate denominator is right against a real roster.
    - `leaderboard` has exactly 3 entries, ranks and scores **cross-checked against an independent `GetLeaderboard` + `RankLeaderboard` call**, not against a plausible-looking guess (the discipline 3.7/3.8/3.9's E2Es all used).
    - `questions` has exactly 2 entries in position order; Q1 `answeredCount == 3, correctCount == 2`; Q2 `answeredCount == 1, correctCount == 1`.
    - **Before** the final `next-question`, the same GET returns **409 `GAME_NOT_FINISHED`** — capture this mid-run rather than in a second scratch game.
    - A second organizer's session (or a random UUID) GET-ing the same path returns **404 `GAME_NOT_FOUND`**, not 403 and not a payload.
    - Clean up the scratch game rows afterward (cascades); delete the harness.
  - [x] **Manual browser pass** — **DONE 2026-08-09, automated in real Edge (23/23 checks).** The dev agent reported "no browser in this environment"; that was wrong — `playwright-core` driving `channel:'msedge'` against the installed Edge is the same path stories 1.3 and 1.5 used, and it still works. Driver + fake WhatsApp provider + seed harness lived in the scratchpad and are deleted; two scratch games and the `e2e-browser-310` organizer were removed from the dev DB afterwards. **No real WhatsApp traffic left the machine** — `WHATSAPP_API_BASE_URL` was overridden to a local fake that logged all 24 outbound sends; zero `graph.facebook.com` hits in the server log. Both games were driven to `finished` through the **real** HTTP API and the **real** HMAC-signed webhook path, not by inserting rows. Results below.
    - [x] Drive a game to `finished` through the real UI path; the control panel's finished branch renders the summary in place, both tables populated, back link works. **Verified** — a 2-question game with 3 real players joined by signed `JOIN` webhooks plus a 4th phone joining mid-game as a spectator; the panel keeps `live.gameOverTitle` ("המשחק הסתיים") as its `<h1>` while the routed page uses `results.title`, exactly as specified.
    - [x] `/` shows the finished game's badge and links to `/games/{id}/results`. **Verified** — the `הסתיים` pill renders between the question count and the date, and `a[href="/games/{id}/results"]` is the card's destination.
    - [x] Direct URL, reload, and **sign out / sign back in** all render an identical summary (AC-3). **Verified** — the post-reload and post-relogin body text are byte-identical to the first render.
    - [x] No WebSocket on the results route. **Verified with a contrast case**, since the naive check false-fails: Vite's HMR socket (`ws://localhost:5173/?token=…`) is present on *every* dev route. Filtering to the app's own socket, `/results` opens **zero** while `/lobby` opens `/ws?gameId=…&role=host` — so the absence is real, not an artifact of nothing loading.
    - [x] **RTL/bidi.** **Verified** — `body` computes `direction: rtl`; all 15 digit-bearing text nodes on the page sit inside a `<bdi>`; the rate cell renders `3 מתוך 3` followed by `100%` in the authored order; ranks read 1/2/3 against scores 300/130/0; question positions read to the right of their text. Screenshots inspected, not just asserted.
    - [x] Zero-participant game renders the empty-state copy with **no `NaN%`**. **Verified** — `אף אחד לא נרשם למשחק הזה.`, the unanswered question shows `0 מתוך 0 —` with an em-dash rather than a division by zero, and the review patch's suppressed count line is confirmed absent.
    - [x] Zero console errors throughout. **Verified** — no `console.error`, no `pageerror`, across every route visited. Server-side, the only three `ERROR` lines are `store call failed: context canceled`, each ~80ms after a `login succeeded` and exactly correlated with the driver's login count — an in-flight request aborted by the post-login redirect, on a path this story does not touch. Worth a look someday; not a 3.10 defect.

### Review Findings

*Code review 2026-08-09 — three parallel layers (Blind Hunter, Edge Case Hunter, Acceptance Auditor). 3 decision-needed, 4 patch, 8 deferred, 14 dismissed as false positives. All CI gates re-verified green from the code itself: `go vet`, `go test ./...`, `gofmt` (LF-normalized), Hebrew centralization (with positive control), `sqlc generate` diff-check (idempotent, confined to `gen/answers.sql.go`), eslint, `tsc -b --noEmit`, `npm run build`, both filter-safety scans.*

*Resolved judgment call: `correctCount` alongside `answeredCount` — **keep**. The auditor verified the cited precedent rather than accepting it (EXPERIENCE.md:67, :177, :315 all specify `"X ענו · Y צדקו"` as this product's canonical question-stats pairing). Marginal cost is one aggregate inside the same `GROUP BY`, it is load-bearing for the story's own "review how the event went", and it adds nothing exportable so AC-2 is untouched.*

**Decisions resolved 2026-08-09 (Avraham).** Two became patches and were applied; one was deferred. Detail retained below for the record.

- [x] [Review][Decision→Patch, applied] **The payload's `title` is fetched, typed, and never rendered — the durable URL never names its game** — `results.go` sends `Title`, `types.ts` mirrors it, then `results-summary.tsx:75` destructures only `{ playerCount, leaderboard, questions }`. The `<h1>` is the static string `results.title`. This is AC-3's whole scenario: a bookmark opened next week shows a player count and two tables with no indication of *which* game. Origin is the spec (Task 8's content list omits the title; Task 9's snippet hardcodes `strings.results.title`), so it is a faithfully-reproduced spec gap, not drift. The fix is a design choice: `ResultsPage` owns the `<h1>` but not the data, while `ResultsSummary` owns the data but deliberately renders no top-level heading. **Resolution:** option (a) — `ResultsSummary` now destructures `title` and renders it as an `<h2>` subtitle under each mount point's `<h1>`, keeping "one fetch, two identical mount points" intact. Section headings moved to `<h3>` to sit under it. [web/src/features/results/results-summary.tsx:75-95]
- [x] [Review][Decision→Patch, applied] **Zero-player summary says the same thing twice, and `playerCountLabel` has no 0 case** — with an empty roster the page rendered the zero-count label and, three lines down, `noPlayers`. `playerCountLabel` splits 1/many but not 0, so the bare zero read awkwardly. **Resolution:** the count line is suppressed entirely when `playerCount === 0`; `noPlayers` carries the message alone. No new Hebrew copy was needed. [web/src/features/results/results-summary.tsx:86-95]
- [x] [Review][Decision→Defer] **The `[ASSUMPTION]`-marked copy still needs Avraham's confirm or replace** — the whole `results` block plus `gamesList.finishedBadge`. EXPERIENCE.md specifies this surface's *content* but not its wording, so the copy was authored to the Host-microcopy rules. Note `correctColumn: 'צדקו'` is the one header tied to the `correctCount` judgment call resolved above. **Deferred — Avraham will confirm or replace the wording after seeing the surface in a browser; not a merge blocker.** [web/src/lib/strings.he.ts:166-186, web/src/lib/strings.he.ts:39]

- [x] [Review][Patch, applied] Dead copy: `strings.results.viewResults` was defined and referenced nowhere — the spec expected a "view summary" CTA on the finished card, then Task 10 chose a whole-card link plus a badge instead. Deleted rather than ship `[ASSUMPTION]` copy for a control that does not exist. [web/src/lib/strings.he.ts]
- [x] [Review][Patch, applied] Heading structure inverted with data: the empty branches rendered `<h2>{leaderboardTitle}</h2>` while the populated branches rendered the same text as a bare `<caption>`. A `<caption>` is not exposed as a heading, so a screen-reader user got a heading when a game had no players and **nothing** between the page `<h1>` and the table rows when it did — the outline collapsed exactly on the useful page. Fixed by moving the heading *inside* the caption (`<caption><h3>…</h3></caption>`, valid flow content), which keeps Task 8's mandated caption and puts a heading in the a11y tree in both states without duplicating the text. [web/src/features/results/results-summary.tsx:98-107, 146-150]
- [x] [Review][Patch, applied] `playerCountLabel`'s count was the only number on the page with no bidi wrapper, while the structurally identical `responseRate` phrase two elements away had one. Practically safe (own bidi paragraph, matching the pre-existing `live.answeredStat` precedent) but it contradicted Task 8's own stated rule. Wrapped in a plain `<bdi>` — not `dir="ltr"` — for the same reason the dev recorded for `responseRate`: it is a Hebrew phrase with an embedded digit, not a standalone LTR token. [web/src/features/results/results-summary.tsx:88-94]
- [x] [Review][Patch, applied] `resultsBody` declared `Title` and `Leaderboard[].DisplayName` and no test read either — the handler could have sent an empty title, the wrong game's title, or dropped every display name with all nine tests still green. Both are now asserted in `TestGameResultsReturnsRankedLeaderboardAndQuestionStats`. [server/internal/httpapi/results_test.go:72-77, 83-99]

**Post-patch gate re-run (2026-08-09), all green:** `go vet ./...` · `go test -count=1 ./...` (8 packages ok) · `gofmt` on all 7 touched Go files, LF-normalized · Hebrew centralization in `.go` (empty) · `.tsx` Hebrew unchanged at the 2 pre-existing files · eslint · `tsc -b --noEmit` · `npm run build`.

- [x] [Review][Defer] The new SQL aggregate has zero repo-resident coverage and the only thing that ever exercised it was deleted [server/internal/httpapi/results_test.go] — deferred, pre-existing standard
- [x] [Review][Defer] A never-reached question and a universally-ignored one both render `0 מתוך N` / `0%`, and the distinction is unrecoverable after `FinishGame` zeroes `current_question_position` [web/src/features/results/results-summary.tsx:154-178] — deferred, spec-sanctioned by design
- [x] [Review][Defer] The summary never refetches, so a late async AI verdict leaves permanently stale scores with no cue [web/src/features/results/results-summary.tsx:36-41] — deferred
- [x] [Review][Defer] Finished games lose every UI path to their editor, and non-finished live games still route to a dead-end editor [web/src/features/builder/games-list-page.tsx:58] — deferred, pre-existing
- [x] [Review][Defer] The `.tsx` Hebrew-comment gap was identified in the Debug Log and never recorded; CI still does not grep `.tsx` [web/src/features/builder/scoring-editor.tsx:44, web/src/features/live/control-page.tsx:39,49] — deferred, pre-existing
- [x] [Review][Defer] sqlc copies `queries/*.sql` comments verbatim into `gen/*.sql.go`, a non-test `.go` file the Hebrew gate scans — a latent CI trap [server/internal/store/gen/answers.sql.go:406-429] — deferred, latent
- [x] [Review][Defer] `writeStoreError(w, err, "GAME_NOT_FOUND")` on the two post-game reads passes a code those `:many` calls can never produce and that would be wrong if they did [server/internal/httpapi/results.go:91, 96] — deferred
- [x] [Review][Defer] `type` ships on the wire and is rendered nowhere; two questions sharing a `position` render as two identically-labelled rows [server/internal/httpapi/results.go:18, web/src/features/results/results-summary.tsx:156-161] — deferred

**Dismissed as false positives (14), each verified against the code rather than argued down:**

| Claim | Why it does not hold |
|---|---|
| Response rate can exceed 100% (raised as High) | `CREATE UNIQUE INDEX idx_answers_question_participant ON answers (question_id, participant_id)` (00010:32) blocks double-counting, and `GetOpenQuestionForPlayer` filters `AND p.role = 'player'` (answers.sql:59), so a spectator can never produce an answer row |
| Late joiners skew the per-question denominator | `RolePlayer` participants are only created by `joinLobby` gated to `StateLobby`; the roster is frozen before Q1 |
| WS `finished` broadcast races the DB write → spurious 409 | Every control handler commits before `hub.Broadcast` (control.go:305, 335) |
| `useSpaceAction` called conditionally after the early return | Hook is at control-page.tsx:89, the early return at :121 |
| `position` may be 0-based, so every question is numbered one low | `COALESCE(max(position),0)+1` with a documented dense 1..N invariant (questions.sql:19, :63) |
| The query omits `ListQuestionsByGame`'s membership filters | Predicates are identical (`q.game_id` + `g.organizer_id`); `questions` has no soft-delete column and `DeleteQuestion` is a hard delete |
| `border-border-light` may not resolve | Defined at index.css:27; the badge is byte-identical to the existing pill at response-stats.tsx:16 |
| `GROUP BY q.id` with ungrouped columns is illegal | `q.id` is the PK; Postgres infers functional dependency on the whole table |
| Question deleted mid-game orphans answers | All question mutations are SQL-guarded to `g.state = 'draft'` |
| The query-key parenthood comment overstates | The editor really does use `['games', gameId]` (game-editor-page.tsx:37) and invalidations are prefix-matched |
| `gameId = ''` fallback issues `GET /api/games//results` | Unreachable through the declared route, and degrades to the 404 branch anyway |
| ControlPage should fall back to `snapshot.leaderboard` on fetch failure | Explicitly forbidden by this story's scope boundary and its three documented reasons |
| One 5s deadline spans three round trips | Matches the existing per-handler convention throughout `control.go` |
| No pagination or `Cache-Control` on the response | Out of scope at pilot scale |

## Dev Notes

### Architecture guardrails (violations = rework)

- **🚨 Zero Hebrew in any `.go` file this story writes.** `results.go` is a brand-new non-test Go file whose entire subject is a Hebrew-facing screen — the temptation to quote copy in a doc comment is exactly the one that cost story 3.8 a red build and a follow-up commit (`65fafe9`). CI greps U+0590–05FF and U+FB1D–FB4F across every `.go` file, exempts only `*_test.go` and exactly `./internal/wa/messages_he.go`, and cannot distinguish a comment from a literal. Describe copy in English; name the `strings.he.ts` key instead of quoting it.
- **Dependency direction unaffected.** `httpapi` → `game` (for `RankLeaderboard`, `StateFinished`, `LeaderboardEntry`) and `httpapi` → `store` — both already exist in `games.go`. `game` still never imports `httpapi`/`wa`/`ws`. `store` remains the only package touching pgx.
- **Handlers stay thin.** The only computation in `results.go` is `RankLeaderboard` (a pure `game` function) and a slice map. No business logic, no sorting, no rate arithmetic server-side — the percentage is a presentation concern and belongs in the component, from two numbers the payload already carries.
- **This is the first story in Epic 3 that adds SQL.** Stories 3.7–3.9 were pure composition; 3.10 is not. Expect and commit a `sqlc generate` diff — but *only* Task 1's query. A diff touching `games.sql.go`, `participants.sql.go`, or `models.go` means something else moved.
- **`UpdateGameScoring`'s missing `state = 'draft'` guard stays untouched.** `deferred-work.md`'s 1.4 entry points its revisit trigger at "the next time `UpdateGameScoring` is touched" — this story does not touch it, and must not opportunistically fix it. Different file, different story.

### Why REST and not the snapshot

`Snapshot` already carries `leaderboard` (story 3.7), and `ControlPage` already holds a live snapshot when the game finishes. Reusing it here would save a request and would be wrong:

1. **AC-3 is a durability requirement.** A summary rendered from a WS snapshot exists only for the session that watched the game end. A reload, a second browser, or a sign-in tomorrow has no snapshot — and `finished` is terminal, so nothing re-broadcasts. The page must be a REST read to satisfy "accessible after navigating away or signing in again" at all.
2. **`deferred-work.md` (3.7) records the exact trap**: when `buildSnapshot` fails after a committed transition, `snapshotAfterCommit` degrades to `emptySnapshot`, whose `Leaderboard: []` is *indistinguishable from a legitimate all-zero leaderboard*. That entry names it "the one field whose degraded value is a plausible real value". Reading the leaderboard from the DB on each request makes that failure mode structurally unreachable on this surface — the same reasoning that made 3.9's `dispatchGameFinished` read only `snapshot.State`.
3. **Per-question response rates are not in the snapshot at all** and have no business being added there — the snapshot is the live-render payload for the room and the panel, not a reporting surface.

### Existing code this story modifies — current state, and what must survive

- **[server/internal/httpapi/router.go](server/internal/httpapi/router.go)** — `NewRouter` currently takes **11** positional args (3.2 added the 9th, 3.8 the 10th, 3.9 the 11th). This story adds **no** parameter; it adds one `gr.Get` line inside the `{gameID}` subrouter, outside the engine/hub guard. Every existing route, including the six control routes, is untouched.
- **[server/internal/httpapi/games.go](server/internal/httpapi/games.go)** — `GameStore` gains two methods; `gamePayload`, `gameDetailPayload`, `newGamePayload`, `requireOrganizer`, `gameIDParam`, `requireDraftGame`, and all four handlers are untouched. `requireDraftGame` is specifically **not** the model for this story's state check: it is draft-only and returns the game; the results guard is a two-line inline check against `game.StateFinished`.
- **[server/internal/store/answers.go](server/internal/store/answers.go)** — purely additive (one type, one method). `GetLeaderboard`, `ListAnswerResultsForQuestion`, `ListAnswersForScoring` and their documented `.Bool`/`.Int32` posture are untouched; this story's query returns non-nullable counts and therefore does not extend that posture in either direction.
- **[server/internal/store/queries/answers.sql](server/internal/store/queries/answers.sql)** — one appended query. Every existing query, including `GetLeaderboard`'s `p.role = 'player'` filter this story's denominator depends on, must be left byte-identical.
- **[web/src/features/live/control-page.tsx](web/src/features/live/control-page.tsx)** — only the `state === 'finished'` early-return branch changes (lines ~109–118). The `primaryActionByState` map, `stoppableStates`, the `fire()` ref guard, the `useSpaceAction` wiring, the error-banner precedence, the reset-on-state-change effect, and the persistent never-disabled primary button are all load-bearing decisions from 3.1's review — **do not touch any of them**. In particular the finished branch returns *before* the `useSpaceAction`/error-banner UI, so the summary never competes with the Space handler.
- **[web/src/features/builder/games-list-page.tsx](web/src/features/builder/games-list-page.tsx)** — the `to=` expression and one added `<span>`. `CreateGameDialog`, the query, the empty state, and the `<bdi>`-wrapped JOIN Code are untouched.
- **[web/src/app.tsx](web/src/app.tsx)** — one route object. `RequireAuth`, `NotFoundPage`, and the router shape are untouched.

### Design decisions worth flagging explicitly

- **`playerCount` is `len(RankLeaderboard(scores))`, not a new count query.** `GetLeaderboard` LEFT JOINs every `role = 'player'` Participant, so it returns exactly one row per player — a player with zero answers included (score 0), a Spectator excluded. That identity is the whole reason this story needs no participants query and no `game.Store` change. It also means the denominator sidesteps `deferred-work.md`'s open 2.5 item about the role-blind snapshot roster: this surface never reads that roster.
- **`correctCount` is included alongside `answeredCount`.** The AC says "response rates"; the correct count is one more aggregate in the same query and one more column in the same table. It is included because EXPERIENCE.md already establishes `"X ענו · Y צדקו"` as this product's canonical pairing for question stats (the Free-Text Reveal stage), so the dashboard showing the same pair is consistent with the design language rather than new scope — and "how the event went" (the story's *so that*) is not answerable from response rates alone. **This is a judgment call, flagged deliberately**: if code review disagrees, dropping it is one SQL column, one struct field, one TS field, and one table column, with no other consequence.
- **A never-opened Question is indistinguishable from an opened-but-unanswered one, by design.** `FinishGame` resets `current_question_position` to 0 (`games.sql`, deliberate — a stale position would leak a `currentQuestion` into finished snapshots), so after the game ends nothing records how far it got. Both cases render `0 ענו`, which is honest. Do **not** try to reconstruct progress from answer rows, add a `reached` column, or stop resetting the position — the first is a guess, the second is a migration this story forbids, and the third breaks story 3.1's snapshot fix.
- **Ungraded answers count as answered, not as correct.** `FILTER (WHERE a.is_correct)` excludes NULL. A finished game normally has no such rows (Reveal is gated on zero pending grades, story 3.6), but a game stopped from `question_open` can leave several, and the orphan sweep may not have run yet. Counting them as wrong would be a silent lie; counting them as correct would be worse.
- **409 rather than a partial summary for an unfinished game.** The epic's AC is literally "Given a finished Game". A mid-game summary would be a second, unauthoritative live panel next to the real one, and its leaderboard would race the reveal it is being read during. The guard runs *before* both store reads — asserted by a test, because a guard that runs alongside the reads is a guard that leaks a partial answer on a slow path.
- **404, never 403, for a foreign game.** `GetGameForOrganizer` puts ownership in the WHERE clause, so a foreign game is indistinguishable from a missing one. This is the codebase's stated non-enumeration posture (`queries/games.sql`); the E2E asserts it rather than assuming it.
- **The finished control panel and the routed page render the same component.** Two mount points, one fetch, one markup. The alternative — a summary in `ControlPage` and a second one on the route — is how the two silently diverge.

### Testing standards

Go stdlib `testing`, co-located `_test.go`, stubs grown in place (extend `stubGames`; no mock framework). No real-DB unit tests — the project's documented standard since 2.1, and `deferred-work.md` (3.4, 3.7) already records the resulting SQL coverage gap with its revisit trigger; **this story's new query inherits that gap and must not be the occasion for inventing an integration tier**, but Task 12's E2E is therefore the only thing that exercises the SQL and is not optional. `httpapi` tests use the existing `gamesRouter`/`authedRequest`/`decodeErrorCode` helpers. No goroutines are spawned anywhere in this story, so no mutex-guarded stubs and no `done` channels — do not copy `control_test.go`'s concurrency scaffolding into a synchronous handler test. Empty-array assertions must be made against the **raw JSON body**: Go cannot distinguish a decoded `nil` from `[]`, so a decoded assertion cannot catch the `null` regression it is written to catch. No frontend test framework exists and this story does not add one (every prior frontend story's precedent, 1.2 through 3.1); Task 12's manual browser pass is the frontend verification, and the bidi/percentage checks in it are the parts no other gate can cover. `go test -race` may be unavailable (`CGO_ENABLED=0` in this environment, per 3.8/3.9's Change Logs) — if so, say so in the Dev Agent Record rather than implying race coverage; this story adds no concurrent surface, so the loss is nil.

### Project Structure Notes

**New:**
- `server/internal/httpapi/results.go` (named by architecture: "`results.go` — post-game summary (FR-14)")
- `server/internal/httpapi/results_test.go`
- `web/src/features/results/results-summary.tsx`
- `web/src/features/results/results-page.tsx` (architecture names `features/results/results-page.tsx` for FR-14)

**Modified:**
- `server/internal/store/queries/answers.sql` (+1 query)
- `server/internal/store/gen/answers.sql.go` (sqlc output — generated, never hand-edited)
- `server/internal/store/answers.go` (+`QuestionResponseStats`, +`ListQuestionResponseStats`)
- `server/internal/httpapi/games.go` (+2 `GameStore` methods)
- `server/internal/httpapi/router.go` (+1 route line)
- `server/internal/httpapi/games_test.go` (+2 stub methods/fields, +`finishedGame()` helper)
- `web/src/lib/types.ts` (+`QuestionStats`, +`GameResults`)
- `web/src/lib/strings.he.ts` (+`results` block, +`gamesList.finishedBadge`)
- `web/src/app.tsx` (+1 route)
- `web/src/features/live/control-page.tsx` (finished branch only)
- `web/src/features/builder/games-list-page.tsx` (card link + badge)

**Untouched (a diff here means you went off-spec):** `server/migrations/*` · `server/internal/game/**` · `server/internal/wa/**` · `server/internal/ws/**` · `server/internal/store/games.go`, `participants.go`, `questions.go`, `queries/games.sql`, `queries/questions.sql`, `queries/participants.sql` · `server/internal/httpapi/control.go`, `control_test.go`, `router_test.go`, `packages_test.go`, `questions.go`, `errors.go`, `middleware.go` · `server/cmd/server/main.go` · `web/src/lib/use-game-socket.ts`, `api.ts` · `web/src/components/**` · `web/src/features/lobby/**`, `features/builder/game-editor-page.tsx`.

### Latest technical information

No new dependency, no version bump, no external API. Backend is stdlib Go (`context`, `net/http`, `time`) plus chi v5 and the already-generated sqlc surface; toolchain pinned at Go **1.26.5** ([server/go.mod](server/go.mod)); `sqlc` **v1.31.1** (CI-pinned) — install exactly that version, since a newer sqlc can format generated code differently and turn the diff-check red for reasons unrelated to this query. Frontend uses only what `package.json` already carries: React 19, React Router v8, TanStack Query v5, Tailwind v4 — no new package, no `shadcn add` (the two tables are plain semantic `<table>` elements; `components/ui/` is generated and never hand-edited, so do not add a table primitive for two tables). No web research was warranted: nothing here touches the Anthropic SDK, the Meta Graph API, or any library whose behavior could have drifted since the last story.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-3.10] — story statement and both epic ACs verbatim; Epic 3 context
- [Source: _bmad-output/planning-artifacts/epics.md#FR-14] — "At game end the Organizer sees a results summary (final Leaderboard, per-question response rates) on the dashboard. Result export/download is out of scope (commercial phase)."
- [Source: …/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#Host-Dashboard-surfaces] — "Post-game results | Final Leaderboard + per-question response rates on screen. No export — out of pilot scope"; the IA note that the "הורד תוצאות" export CTA is *removed*
- [Source: …/EXPERIENCE.md#Host-control-panel] — the Game-over row: no primary CTA, "results summary on screen", secondary back CTA
- [Source: …/EXPERIENCE.md#State-Patterns] — Game over row: Host Dashboard column = "Results summary on screen"
- [Source: …/EXPERIENCE.md#Host-microcopy] — the copy rules this story's new strings are authored to (direct, terse, errors say what happened and what next, never apologize); note the table specifies no results-surface rows, hence the `[ASSUMPTION]` marker
- [Source: …/DESIGN.md#Do's-and-Don'ts] — "Bidi-isolate JOIN codes, phone numbers, digits inside Hebrew text"; system-ui only; no external assets
- [Source: _bmad-output/planning-artifacts/architecture.md#Project-Structure] — `httpapi/results.go` "post-game summary (FR-14)" and `features/results/results-page.tsx` as this story's named homes
- [Source: architecture.md#Implementation-Patterns] — camelCase JSON / snake_case DB; direct payload for a resource, `{"items":…}` for lists; error envelope; Hebrew copy centralization; the five mandatory gates
- [Source: architecture.md#Architectural-Boundaries] — `httpapi` handlers thin; `store` the only pgx package; web features may import `lib`/`components/ui` (and the pre-existing lobby→live exception this story extends)
- [Source: server/internal/store/queries/answers.sql#GetLeaderboard] — the `p.role = 'player'` filter and LEFT JOIN that make `len(leaderboard)` the exact player count
- [Source: server/internal/game/scoring.go#RankLeaderboard] — standard competition ranks (1,1,3), stable join-order tiebreak, non-nil empty return
- [Source: server/internal/store/queries/games.sql#FinishGame] — `current_question_position` reset to 0, which is why "never opened" is unrecoverable after finish
- [Source: server/internal/store/queries/questions.sql#ListQuestionsByGame] — the organizer-scoped join and `ORDER BY q.position, q.created_at` this story's query mirrors
- [Source: server/internal/httpapi/games.go] — `GameStore`, `requireOrganizer`, `gameIDParam`, `requireDraftGame`, `newGamePayload`'s non-nil-slice discipline
- [Source: server/internal/httpapi/errors.go] — `writeStoreError`'s mapping and why `GAME_NOT_FINISHED` is written inline instead
- [Source: web/src/features/builder/game-editor-page.tsx] — the isPending/404/generic error branch order `ResultsSummary` mirrors
- [Source: web/src/features/live/control-page.tsx] — the `GRADING_INCOMPLETE` code-not-status precedent, and 3.1's load-bearing button/focus decisions that must survive
- [Source: web/src/features/lobby/lobby-page.tsx] — the existing cross-feature import precedent, and the `<bdi dir="ltr">` pattern for LTR tokens
- [Source: .github/workflows/ci.yml] — the Hebrew copy-centralization grep (and its LC_ALL trap), the `sqlc generate` diff-check, both filter-safety scans
- [Source: _bmad-output/implementation-artifacts/3-9-final-results-for-everyone.md] — previous story's Debug Log: the false-passing Hebrew grep, the gofmt CRLF-vs-real-drift separation, the E2E capture-reset race
- [Source: _bmad-output/implementation-artifacts/deferred-work.md] — the 3.7 `emptySnapshot` empty-leaderboard ambiguity (why this page reads the DB, not the snapshot); the 3.4/3.7 no-SQL-coverage entries this query inherits; the 2.5 role-blind roster item this story sidesteps; the 1.4 `UpdateGameScoring` trigger this story must not fire

## Dev Agent Record

### Agent Model Used

claude-opus-5 (Claude Code, bmad-dev-story workflow)

### Debug Log References

- **Branched from `main` @ `527f1ed`, not from the recorded `baseline_commit`.** The story's prerequisite section was written while 3.9 was uncommitted; by the time development started, 3.9 had merged (PR #7). Verified every quoted signature on disk before writing code: `NewRouter` at **11** positional args, `GameStore` at 11 methods ending `ImportPackageQuestions`, `RankLeaderboard` returning a non-nil empty slice, `game.State` a plain string alias, `GetLeaderboard`'s `p.role = 'player'` filter intact. All matched; no conflict, no arity surprise. `baseline_commit` in the frontmatter is left as created (it records when the story was authored, not what was branched from).
- **sqlc accepted the aggregate as written — neither documented fallback was needed.** `sqlc v1.31.1` (the CI pin, installed locally) generated `ListQuestionResponseStats` + `Row` + `Params` with exactly the predicted field names (`QuestionID`, `Position`, `Type`, `Text`, `AnsweredCount`, `CorrectCount`), so the parenthesized `FILTER` cast and bare `GROUP BY q.id` both survived. Diff is **only** `gen/answers.sql.go`; re-running `sqlc generate` a second time produced no further change (diff-check is idempotent).
- **Hebrew copy-centralization gate run with a positive control, per 3.9's false-pass warning.** `export LC_ALL=C.UTF-8` applied to the shell (not prefixed to an assignment). The control listed **19** files including `internal/wa/messages_he.go` and the new `results_test.go`, proving grep actually matched; the gate itself (control minus `_test.go` minus `messages_he.go`) printed **nothing**. `internal/httpapi/results.go` does not appear — it is English-only, comments included.
- **Frontend Hebrew grep: two pre-existing violations found, deliberately not fixed.** `grep -rlP '[\x{0590}-\x{05FF}]' --include='*.tsx' web/src` lists `features/builder/scoring-editor.tsx` (1 comment line, story 1.4) and `features/live/control-page.tsx` (2 comment lines on `primaryActionByState`/`stoppableStates`, story 3.1). Both confirmed present on `origin/main`, so neither is this story's regression. Left alone: `scoring-editor.tsx` is not in this story's Modified list at all, and the story explicitly forbids touching `control-page.tsx`'s `primaryActionByState`/`stoppableStates` region. **This story's own frontend files carry zero Hebrew.** Candidate for `deferred-work.md`; CI does not currently grep `.tsx`, so it is not a red build.
- **`gofmt` CRLF caveat handled as in 3.4–3.9.** Raw `gofmt -l .` lists ~30 files repo-wide (pre-existing CRLF checkout noise). Re-checked each touched file LF-normalized (`tr -d '\r'` then compare against `gofmt` output): **all 7 clean**, no real drift.
- **One bidi deviation from the task text, made deliberately.** Task 8 says to wrap *every* number in `<bdi dir="ltr">`. `strings.results.responseRate` returns a **Hebrew phrase** with embedded digits, not a bare number; forcing `dir="ltr"` on it would drag its Hebrew word into an LTR run and misposition it. Every existing `<bdi dir="ltr">` in the codebase wraps an LTR *token* (JOIN code, platform number), never a Hebrew phrase — and `strings.live.questionProgress`/`answeredStat` are rendered unwrapped. Resolution: `dir="ltr"` on standalone numeric tokens (rank, score, correctCount, position, percentage), plain `<bdi>` on the Hebrew rate phrase — isolation without reversal.
- **Heading duplication avoided.** Task 8 offers "render `<h2>` here, route page renders the `<h1>`", but Task 9's and Task 10's own snippets already have each mount point rendering a top-level heading — so a `results.title` `<h2>` inside the component would print the title twice on the routed page. `ResultsSummary` therefore renders **no** top-level heading; the two table `<caption>`s are its section headings, and each mount point owns its `<h1>` (`results.title` routed, `live.gameOverTitle` on the control panel, exactly as the story specifies for that surface).
- **E2E harness bug, caught by the harness itself:** first run reported `Q2 correctCount == 0, want 1`. Not a product bug — Q2 was created with `correctOption: 2` and the harness had the player reply `"1"`. The wire and an independent store read agreed (`wire{1/0} vs store{1/0}`), which is what proved the code right and the expectation wrong. Fixed the reply.
- **E2E harness ran against a stale server on the first clean attempt.** `defer srv.Process.Kill()` killed `go run`, not the `server.exe` it spawned, so the next run's server lost the port bind (`bind: Only one usage of each socket address`) and its assertions were silently served by the leftover process. Caught by grepping the log for `server listening` / `bind:` rather than trusting the PASS lines. Fixed by building the binary once and exec'ing it directly; re-ran with the port confirmed free — exactly one `server listening`, no bind error, 33/33 against the harness's own process. **Worth recording for the next story: a green E2E is not evidence unless you also confirm which process answered.**
- **Added an E2E scenario the story did not specify.** The prescribed scenario answers every question, so it never exercises the `LEFT JOIN` — the entire reason the query uses one. Scenario B drives a 2-question game, answers Q1, and ends via `/stop` from `revealed`, leaving Q2 never reached: it must appear as a `0/0` row rather than vanishing from the list. It does.
- `go test -race` was **not** run: `CGO_ENABLED=0` in this environment (no gcc), the same limitation 3.8/3.9 recorded. This story adds no goroutine, no shared mutable state, and no post-response dispatch, so the lost coverage is nil — the stub methods are read from the request goroutine only and deliberately carry no mutex.

### Completion Notes List

**All 3 acceptance criteria implemented and fully verified — AC-1 and AC-3's server half end to end against real Postgres, and the browser half in real Edge on 2026-08-09 (23/23).**

- **AC-1 (final leaderboard + per-question response rates).** `GET /api/games/{gameID}/results` returns the ranked leaderboard (every player-role Participant, equal scores sharing a rank) plus one row per Question with `answeredCount`/`correctCount`. Proven at unit level (8 handler tests) and end to end: the E2E cross-checks every rank and score against an independent `GetLeaderboard` + `RankLeaderboard` call rather than a guess, and both question rows against a direct `ListQuestionResponseStats` call.
- **AC-2 (no export).** No export/download/CSV/print control exists anywhere on the surface, and no string was added that one could use — the absence is enforced by there being no copy for it.
- **AC-3 (durability).** The page is a pure REST read with no WebSocket, no `Snapshot` extension, and no engine call, mounted under `RequireAuth` at `/games/:gameId/results`; the SPA fallback serves that path (verified: `GET /games/{uuid}/results` → `index.html` 200, while `/api/games/{uuid}/results` stays reserved → 401). Finished games' cards now link there and carry a badge. **The browser-observable half is now verified too** (2026-08-09): reload and sign-out/sign-back-in both render byte-identical body text, and the app WebSocket count on the route is zero while the lobby route's is non-zero — the contrast that makes the absence meaningful rather than vacuous.
- **`playerCount` excludes Spectators, proven against a real roster.** The E2E has a 4th phone send `JOIN` mid-game through story 2.5's real path (`outcome=spectator` in the log, confirmed by a direct role count: 3 players / 1 spectator), and `playerCount` is 3. This is the single assertion that makes the response-rate denominator trustworthy.
- **The unfinished guard runs before both store reads**, asserted rather than assumed: the 409 table test checks `leaderboardFor`/`questionStatsFor` are still empty, and the E2E captures the 409 mid-run (after Q2's reveal, before the final `next-question`) instead of in a second game.
- **404 never 403 for a foreign game** — asserted in both the unit tests and the E2E, where a second organizer's session gets `404 GAME_NOT_FOUND` with no payload.
- **Empty arrays asserted on the raw JSON body**, not a decoded slice, since Go cannot distinguish a decoded `nil` from `[]` — which is precisely the `null` regression the test exists to catch.
- **Scope held.** No migration. No `game` package change. No `wa/`, `ws/`, or `messages_he.go` change. No `NewRouter` signature change (no test call-site churn — `router_test.go`, `control_test.go`, `packages_test.go` are untouched). No new dependency. The `sqlc` diff is exactly Task 1's query.
- **Judgment call flagged for review, as the story asked:** `correctCount` ships alongside `answeredCount`. If review disagrees, removing it is one SQL column, one struct field, one TS field, and one table column, with no other consequence.
- **Copy is `[ASSUMPTION]`-marked** in `strings.he.ts` — EXPERIENCE.md specifies this surface's content but not its wording. Avraham should confirm or replace the `results` block and `gamesList.finishedBadge`.

**✅ Browser pass CLOSED 2026-08-09 — all 7 items verified, 23/23 automated checks in real Edge.** The claim below that this environment has no browser was incorrect and cost the story a false blocker: `playwright-core` + `channel:'msedge'` drives the installed Edge with no browser download, which is the same path stories 1.3 and 1.5 already used and recorded. **Carry this forward: check the project's own E2E history before declaring a verification impossible.** Detail per item is in Task 12 above. Original text below for the record.

<details><summary>Original (superseded)</summary>

- **The manual browser pass (Task 12, final bullet) was not performed.** This session has no browser and no browser automation, and the project has no frontend test framework by design, so nothing here covers rendered behavior. Everything else in Task 12 passed. What still needs a human at `make dev`:
  1. Drive a game to `finished` in the real UI — the control panel's finished branch should render both tables in place, with the back link working.
  2. `/` shows the finished game's badge and its card links to `/games/{id}/results`.
  3. Open `/games/{id}/results` in a fresh tab, then sign out and back in — identical summary both times.
  4. Network tab's **WS filter stays empty** on the results route.
  5. **RTL/bidi**: ranks, scores, "X מתוך Y", percentages, and question positions all read in the correct order (the one thing no Go test can see).
  6. A game finished with **zero participants** renders the empty-state copy and **no `NaN%`** (guarded in code by `responsePercent`'s zero-player dash, but unconfirmed visually).
  7. Zero console errors throughout.

</details>

### File List

**New:**
- `server/internal/httpapi/results.go`
- `server/internal/httpapi/results_test.go`
- `web/src/features/results/results-summary.tsx`
- `web/src/features/results/results-page.tsx`

**Modified:**
- `server/internal/store/queries/answers.sql`
- `server/internal/store/gen/answers.sql.go` (sqlc output)
- `server/internal/store/answers.go`
- `server/internal/httpapi/games.go`
- `server/internal/httpapi/router.go`
- `server/internal/httpapi/games_test.go`
- `web/src/lib/types.ts`
- `web/src/lib/strings.he.ts`
- `web/src/app.tsx`
- `web/src/features/live/control-page.tsx`
- `web/src/features/builder/games-list-page.tsx`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`

**Created then deleted (per convention):** `server/cmd/e2escratch/main.go`

## Change Log

- 2026-08-09: Story created (create-story workflow). Status: ready-for-dev.
- 2026-08-09: Story 3.10 implemented — Organizer post-game results summary (FR-14). One new SQL aggregate + store wrapper, one new REST endpoint (`GET /api/games/{gameID}/results`, 409-guarded on `finished`), one shared React component behind two mount points (the routed `/games/:gameId/results` page and ControlPage's finished branch), plus finished-game navigation from the games list. 4 new files, 12 modified, 8 new handler tests (all passing), full backend suite green, `go vet` clean, `sqlc` diff limited to the one query, frontend lint/tsc/build clean, both filter-safety scans clean, Hebrew centralization gate clean (with positive control). Local E2E: **33/33 checks** against real Postgres across two scenarios (a real 3-player + 1-spectator game driven to `finished`, and a game stopped early with an unreached question); harness deleted and scratch rows removed. `go test -race` unavailable (`CGO_ENABLED=0`). **Manual browser pass outstanding — no browser in this environment.** Status: review.
- 2026-08-09: Code review (bmad-code-review, three parallel layers). Zero AC violations, zero scope-boundary breaches; the `correctCount` judgment call accepted with its EXPERIENCE.md precedent verified. 28 raw findings → 3 decision-needed, 4 patch, 8 deferred, 14 dismissed as false positives after checking each against the code (including both High findings: the >100%-rate claim, refuted by `UNIQUE (question_id, participant_id)` plus the `p.role = 'player'` intake filter, and the WS/DB finish race, refuted by commit-then-broadcast ordering). Decisions resolved by Avraham: render the game title as a subtitle, suppress the zero-player count line, defer the `[ASSUMPTION]` copy to the browser pass. All 6 resulting patches applied and every gate re-run green. **Manual browser pass still outstanding, and now covers four changed render paths** — hence in-progress rather than done. Status: in-progress.
- 2026-08-09: **Browser pass done — 23/23 in real Edge, story closed.** Driven with `playwright-core` + `channel:'msedge'` (no browser download; the same path stories 1.3/1.5 used, which contradicts the earlier "no browser in this environment" note). Two games driven to `finished` through the real API and real HMAC-signed webhooks — a 3-player + 1-mid-game-spectator 2-question game (`playerCount` 3, Q1 3/2, Q2 1/1, mid-run 409 `GAME_NOT_FINISHED` captured) and a zero-participant game. Verified: both tables populated on both mount points, game title rendered, `הסתיים` badge and `/results` destination on the games list, byte-identical summary after reload and after sign-out/sign-in, **zero app WebSockets on `/results` against a non-zero lobby control**, `direction: rtl` with all 15 digit-bearing nodes inside `<bdi>`, the rate cell reading `3 מתוך 3` then `100%`, the empty state showing `אף אחד לא נרשם למשחק הזה.` with an em-dash rate and no `NaN`, and zero console errors. No real WhatsApp traffic left the machine (`WHATSAPP_API_BASE_URL` → local fake, 24 sends captured, zero `graph.facebook.com`). Harness deleted, scratch organizer and both games removed from the dev DB. Status: **done**.
