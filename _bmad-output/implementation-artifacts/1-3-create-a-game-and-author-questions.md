---
baseline_commit: 4bb7893
---

# Story 1.3: Create a Game and Author Questions

Status: ready-for-dev

## Story

As an Organizer,
I want to create a Game and add, edit, reorder, and delete MCQ and Free-Text Questions,
so that I can prepare my quiz in advance.

## Acceptance Criteria

1. **Given** I am signed in, **when** I create a Game, **then** it appears in my games list with a unique JOIN Code displayed (`games` + `questions` tables created in this story).
2. **Given** a Game in `draft`, **when** I add an MCQ Question, **then** it has exactly four options (א–ד) with exactly one marked correct, and a per-question time limit pre-filled with the default (default 20s per UX revision 2026-07-09 A8; the field carries guidance that 30–45s accommodates screen-reader and slower-motor Participants).
3. **When** I add a Free-Text Question, **then** I can define one or more Accepted Answers and edit them later.
4. **When** I edit, reorder, or delete Questions, **then** the changes and order persist.
5. **Given** a Game owned by another Organizer, **when** I request it by ID, **then** I receive `GAME_NOT_FOUND` (ownership scoping, FR-11), **and** the dashboard follows UX-DR9 (right sidebar 240px, slate palette, 1024px minimum).

## Tasks / Subtasks

- [ ] Task 1: Schema + store layer — `games` and `questions` (AC: 1, 2, 3, 4, 5)
  - [ ] `server/migrations/00003_games_questions.sql` (goose Up/Down — next number after 00002; architecture's illustrative numbering is offset by the 1.1 variance):
    - `games`: `id UUID PK DEFAULT gen_random_uuid()`, `organizer_id UUID NOT NULL REFERENCES organizers(id) ON DELETE CASCADE`, `title TEXT NOT NULL`, `join_code TEXT NOT NULL UNIQUE`, `state TEXT NOT NULL DEFAULT 'draft' CHECK (state IN ('draft','lobby','question_open','question_closed','revealed','leaderboard','finished'))` — the full canonical enum lands in the CHECK now (identical strings in Go/TS/DB per architecture); the engine that drives transitions arrives in 2.3/3.1 — `created_at`/`updated_at timestamptz NOT NULL DEFAULT now()`; index `idx_games_organizer_id`
    - `questions`: `id UUID PK DEFAULT gen_random_uuid()`, `game_id UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE`, `position INTEGER NOT NULL`, `type TEXT NOT NULL CHECK (type IN ('mcq','free_text'))`, `text TEXT NOT NULL`, `options TEXT[] NOT NULL DEFAULT '{}'`, `correct_option INTEGER NOT NULL DEFAULT 0`, `accepted_answers TEXT[] NOT NULL DEFAULT '{}'`, `time_limit_seconds INTEGER NOT NULL DEFAULT 20 CHECK (time_limit_seconds BETWEEN 5 AND 300)`, `created_at`/`updated_at timestamptz NOT NULL DEFAULT now()`; index `idx_questions_game_id_position (game_id, position)`
    - Type-consistency CHECK on `questions`: `(type = 'mcq' AND cardinality(options) = 4 AND correct_option BETWEEN 1 AND 4 AND cardinality(accepted_answers) = 0) OR (type = 'free_text' AND cardinality(options) = 0 AND correct_option = 0 AND cardinality(accepted_answers) >= 1)` — every column NOT NULL with a neutral default so sqlc models stay pgx-free (see Dev Notes / sqlc constraint); no `UNIQUE (game_id, position)` — reorder rewrites positions transactionally instead of fighting a deferred constraint
  - [ ] `internal/store/queries/games.sql`: `CreateGame :one` · `ListGamesByOrganizer :many` (LEFT JOIN question count, ORDER BY created_at DESC) · `GetGameForOrganizer :one` (WHERE id = $1 AND organizer_id = $2 — ownership in the query, always)
  - [ ] `internal/store/queries/questions.sql`: `ListQuestionsByGame :many` (ORDER BY position, created_at) · `CreateQuestion :one` (position = COALESCE(MAX+1) via CTE or separate `NextQuestionPosition :one`) · `UpdateQuestion :one` (sets `updated_at = now()`) · `DeleteQuestion :exec` · `UpdateQuestionPosition :exec` — every question query joins/filters through `games.organizer_id` so a foreign question is unreachable by construction
  - [ ] `internal/store/games.go` + `internal/store/questions.go`: thin wrappers following `store/auth.go` — translate `pgx.ErrNoRows` → `store.ErrNotFound`; `CreateGame(ctx, organizerID, title)` generates the JOIN Code internally and retries ≤5 times on unique-violation (pgconn error 23505 — store is the only package allowed to know pgx sentinels); `ReorderQuestions(ctx, gameID, organizerID, orderedIDs)` runs in a transaction (`q.WithTx`) and rewrites positions 1..N
  - [ ] JOIN Code generator in `store` (pure function + unit test, no DB): 6 chars via crypto/rand from charset `ABCDEFGHJKLMNPQRSTUVWXYZ23456789` (no I/O/0/1 — unambiguous when read aloud across a room), stored uppercase `[ASSUMPTION — PRD says the Organizer "receives" the code, so it is server-generated; COHEN24-style vanity codes in the UX journeys are illustrative only — do NOT build a code editor]`
  - [ ] Run `sqlc generate`, commit `gen/`; verify `TEXT[]` emits `[]string` (pgx/v5 native) and no pgtype leaks into models
- [ ] Task 2: HTTP surface — games + questions CRUD (AC: 1, 2, 3, 4, 5)
  - [ ] `internal/httpapi/errors.go`: the central domain-error → envelope mapper deferred from 1.2 ("arrives with 1.3's richer error surface") — maps `store.ErrNotFound` → 404 (`GAME_NOT_FOUND` / `QUESTION_NOT_FOUND` per resource), validation failures → 400 `VALIDATION_FAILED` (message says which field), non-draft mutation → 409 `GAME_NOT_EDITABLE`, infrastructure → 503 `DB_UNAVAILABLE`; existing `writeError` remains the emitter
  - [ ] `internal/httpapi/games.go`: `GameStore` interface defined here (grow the 1.2 pattern — handlers depend on small interfaces, `*store.Store` satisfies them; `NewRouter` signature grows, update `cmd/server/main.go`). Handlers: `POST /api/games` `{title}` → 201 game payload incl. `joinCode` · `GET /api/games` → `{"items":[...]}` each with `questionCount` · `GET /api/games/{gameID}` → game payload **including its `questions` array** `[design decision — one fetch renders the whole editor; lists stay `{"items":...}` per architecture]`
  - [ ] `internal/httpapi/questions.go`: `POST /api/games/{gameID}/questions` → 201 · `PUT /api/games/{gameID}/questions/{questionID}` → 200 · `DELETE /api/games/{gameID}/questions/{questionID}` → 204 · `POST /api/games/{gameID}/questions/reorder` `{"questionIds":[...]}` → 204 (chi action-as-sub-resource style); reorder body must be an exact permutation of the game's question IDs, else 400
  - [ ] Validation at the boundary (trim first): title 1–120 chars; question text 1–500; MCQ exactly 4 non-empty options ≤200 chars each + `correctOption` 1–4; Free-Text ≥1 accepted answers (≤20 values, each 1–200 chars — mirrors the FR-5 200-char answer cap); `timeLimitSeconds` integer 5–300 `[ASSUMPTION — bounds not fixed in PRD; DB CHECK mirrors them]`; question `type` immutable after creation `[ASSUMPTION]`; `http.MaxBytesReader` 64 KiB on every mutation body (1.2 hardening pattern) → 413 `REQUEST_TOO_LARGE`
  - [ ] Guardrail: all question/game mutations require `state = 'draft'` → else 409 `GAME_NOT_EDITABLE` (trivially true today — every game is draft — but it future-proofs against 2.3+ live states)
  - [ ] Router wiring inside the existing `RequireOrganizer` group; every handler binds DB calls with `context.WithTimeout(r.Context(), 5*time.Second)` (established pattern); ownership = `GetGameForOrganizer` with the context organizer — 404 `GAME_NOT_FOUND`, never 403 (existence must not leak, per AC-5)
  - [ ] slog: INFO on game created (`game_id`, `organizer_id`), question created/updated/deleted (`game_id`, `question_id`); zero new ERROR paths on a healthy run (NFR-8)
  - [ ] `router_test.go` + handler tests in the existing stub-interface style (`stubPinger` precedent): create game returns unique code; list scoped to the session organizer; foreign game → 404 `GAME_NOT_FOUND` envelope; MCQ with 3 options → 400; free-text with 0 accepted answers → 400; `correctOption` 0/5 → 400; reorder with missing/extra/duplicate IDs → 400; reorder happy path persists order; delete → 204; mutations without session → 401; oversized body → 413; non-draft game mutation → 409. **Regression guard: all 18 existing router/auth tests pass unchanged**
- [ ] Task 3: Frontend — dashboard shell per UX-DR9 (AC: 5)
  - [ ] `web/src/components/dashboard-layout.tsx` (documented variance — shared across builder/lobby/live/results later, so it lives above `features/`): right-in-RTL sidebar `aside` 240px (logical start side — RTL puts it right automatically), white Level-1 panel with `host-border` border, brand title, nav link "המשחקים שלי", logout button relocated here from the games-list page (same mutation logic); main region `bg-host-surface` with 24px content padding; render via route layout under `RequireAuth` in `app.tsx`
  - [ ] Do **NOT** add the shadcn sidebar block (it drags sheet/tooltip/skeleton along) — a 240px `aside` is ~20 lines; builder inputs/dialogs stay shadcn defaults per DESIGN.md's inheritance contract
  - [ ] Optimized 1280px+, minimum 1024px; below that the supported single-column collapse (sidebar becomes a top bar) — same code path 200% zoom hits (WCAG 1.4.4); all sizes rem; visible focus rings preserved
  - [ ] `app.tsx` route tree: `RequireAuth` → `DashboardLayout` → `{ index: GamesListPage }`, `{ path: 'games/:gameId', element: GameEditorPage }`; catch-all and login routes untouched
- [ ] Task 4: Frontend — real games list + create game (AC: 1)
  - [ ] `web/src/features/builder/games-list-page.tsx`: replace the 1.2 placeholder — `useQuery` on `GET /api/games`; game cards (white, `host-border`, 12px radius, no shadow) showing title, JOIN Code (bidi-isolated), question count, created date; click → navigate to editor; empty state invites creating the first game (copy from `strings.he.ts`, UX-DR12 principles)
  - [ ] Create-game dialog (shadcn Dialog — modals are the one shadow-allowed surface): title field + submit via `useMutation`; on success invalidate the games query and navigate to `/games/:id`; pending state disables the trigger (architecture rule); errors surfaced via `aria-describedby`, copy from `strings.he.ts`
- [ ] Task 5: Frontend — game editor page (AC: 1, 2, 3, 4)
  - [ ] `web/src/features/builder/game-editor-page.tsx`: `useQuery` on `GET /api/games/{gameId}` (single fetch — game + questions); header shows title + JOIN Code prominently (the code is a Latin/digit LTR token inside RTL — wrap in `<bdi>`/`dir="ltr"` span, per DESIGN.md bidi mandate); questions rendered as an ordered list showing type badge, text, time limit, and for MCQ the correct option
  - [ ] Reorder: per-row up/down buttons (aria-labeled) → optimistic local order + `POST .../questions/reorder`; **no drag-and-drop library** — zero new npm dependencies this story
  - [ ] Delete question behind a shadcn AlertDialog confirm (destructive action; Escape closes, never opens — UX-DR13 spirit)
  - [ ] Empty state: a Game with no Questions prompts adding the first question (single CTA — the "או ייבאו חבילה מהמאגר" second CTA arrives with Story 1.5's Question Bank; don't build a disabled stub for it)
  - [ ] 404/foreign game: `GAME_NOT_FOUND` from the API renders the Hebrew not-found treatment (reuse `strings.notFound` or a builder-specific message) — never a blank screen
- [ ] Task 6: Frontend — question editor (AC: 2, 3, 4)
  - [ ] `web/src/features/builder/question-editor.tsx`: one component for create + edit (dialog or inline panel — dev's choice; dialog matches the modal elevation rule); type picked at creation (MCQ / Free-Text) and immutable when editing
  - [ ] MCQ form: question text (Textarea), exactly four option inputs labeled א/ב/ג/ד **by position** — the letters are presentation, never stored in the option strings (position-as-identity, PRD Glossary) — RadioGroup marks exactly one correct
  - [ ] Free-Text form: question text + Accepted Answers multi-value editor (add/remove/edit rows, order preserved — **the first Accepted Answer is the primary form shown at Reveal on the stage** (A15); hint text says so); AI never invents correctness — the Organizer's list is the whole truth
  - [ ] Time limit field on both: numeric, pre-filled 20, with the A8 guidance line ("30–45 שניות מתאימות לקהל רחב יותר" — final copy dev's craft in `strings.he.ts`, principle binding: mention screen-reader/slower-motor accommodation)
  - [ ] Client-side validation mirrors the server rules (trim, lengths, 4 options, ≥1 accepted answer); errors associated via `aria-describedby`, never floating toasts; submit disabled while pending
  - [ ] `npx shadcn@latest add dialog alert-dialog textarea radio-group` (Button/Input/Label/Card already present); after adding, re-run the filter-safety greps — 1.1 had to strip a fontsource import that shadcn add reintroduced
  - [ ] `web/src/lib/types.ts` created (canonical tree location): `GameState` union (7 canonical strings), `Game`, `GameListItem`, `Question` wire types mirroring the Go payloads (camelCase)
  - [ ] `web/src/lib/strings.he.ts`: all new copy — nav, games list, create dialog, editor, question forms, confirm-delete, validation messages, empty states; **no Hebrew literals in any component** (CI-greppable convention)
- [ ] Task 7: Deferred-work item — spacing-scale enforcement decision (deferred from 1.1 review to "the first real UI story (1.3)")
  - [ ] While building Tasks 3–6, use only on-scale spacing stops (4/8/12/16/24/32/48/64px ↔ Tailwind 1/2/3/4/6/8/12/16) in hand-written classes; buttons stay `h-10` (40px) per DESIGN.md, never shadcn's default `h-9`
  - [ ] Close the item in `deferred-work.md` with the outcome: recommended disposition is "convention + review" (mechanical `--spacing: initial` enforcement stays rejected — it breaks shadcn defaults); record whatever is actually decided
- [ ] Task 8: Quality gates + end-to-end verification (all ACs)
  - [ ] All local gates: `gofmt` clean, `go vet ./...`, `go test ./...`, `sqlc generate` (empty diff), `eslint`, `tsc -b --noEmit`, `npm run build`, filter-safety greps
  - [ ] Manual E2E against local Postgres (Docker, per README): login → create game → JOIN Code visible on list + editor → add MCQ (4 options, correct marked, default 20s shown) → add Free-Text (2 accepted answers) → edit both → reorder via up/down → refresh: order persists (AC-4) → delete a question (confirm dialog) → provision a second organizer, curl their session against the first organizer's game ID → 404 `GAME_NOT_FOUND` envelope (AC-5) → `/games/:id` served by the built binary via SPA fallback → dashboard renders RTL with the 240px sidebar at 1280px and collapses at <1024px → zero ERROR log lines
  - [ ] Migration check: `00003` applies cleanly on a fresh DB **and** on the existing local dev DB; goose Down works

## Dev Notes

### What this story is — and is not

Builder vertical slice: two tables, games+questions CRUD with ownership scoping, JOIN Code generation, the UX-DR9 dashboard shell, real games list, game editor, question editor. It does **not** build: scoring configuration (1.4 — no scoring columns on `games` yet; 1.4 ALTERs them in), Question Bank / packages / "מהמאגר" badge (1.5), lobby open / state transitions / engine / WebSocket (2.3), any WhatsApp code, game rename/delete (not in any AC — out of scope, don't invent), Vitest (architecture: "added when first frontend tests land" — eslint + tsc strict + build still gate; don't introduce it for form markup). The JOIN Code is generated, immutable, and has no vanity editor.

### Previous story intelligence (1.2 — established patterns, follow or rework)

- **Module path** `github.com/avraham-shor/whatsapp-clickers`; Go 1.26.5 via the `go1.26.5` wrapper (system go is 1.22); local builds may need `-buildvcs=false` (sandboxed git exits 128); `make` needs Git Bash — README documents the no-make fallback; transient `VirtualAlloc errno=1455` toolchain crashes on this machine resolve on re-run.
- **Handler conventions**: respond only via `writeJSON`/`writeError`; API-branch 404/405 envelopes already declared in `router.go` — new routes go inside the existing `r.Route("/api")` protected group; every DB call bounded by `context.WithTimeout(r.Context(), 5*time.Second)`; `middleware.go` exposes `OrganizerFromContext(ctx)` — that's your organizer ID source, never a request param.
- **Interface pattern**: handlers depend on small interfaces defined in `httpapi` (`Pinger`, `AuthService` precedents); define `GameStore` there, let `*store.Store` satisfy it, grow `NewRouter`'s signature, update `cmd/server/main.go` wiring (`st` is already constructed there).
- **Store discipline**: `store` is the only pgx importer; wrappers translate `pgx.ErrNoRows` → `store.ErrNotFound` (see `store/auth.go`); callers never see pgx sentinels. The join-code retry needs `pgconn.PgError` 23505 detection — that knowledge stays inside `store`.
- **sqlc constraint (drives the schema design)**: `sqlc.yaml` overrides map uuid→`string`, timestamptz→`time.Time` — valid **only because every column is NOT NULL**. This is why `questions` uses `NOT NULL` arrays + sentinel defaults (`'{}'`, `correct_option = 0`) instead of nullable columns: one nullable column and pgtype leaks into `gen/`. The CHECK constraint carries the real invariants; sentinels never reach the wire (API payloads omit fields irrelevant to the type).
- **sqlc mechanics**: schema dir is `migrations/` (sqlc parses the migration SQL; goose annotations are ignored); v1.31.x pinned in CI; commit `gen/`; CI diff-gate includes untracked files.
- **Migrations**: goose v3 Provider API with `WithSessionLocker`, embedded FS — drop `00003_games_questions.sql` next to the others, the glob picks it up.
- **Test conventions**: table-free `httptest` cases, tiny stub interfaces defined in the test file, co-located `_test.go`. 18 router/auth tests exist — they must pass unchanged.
- **Frontend conventions**: `api.ts` typed fetch (hard-redirects on 401 except the auth probe; throws typed `ApiError`); `queryClient` has `retry: false`; TanStack `isPending`/`isError` only — no hand-rolled loading flags; mutations disable their trigger; features never import each other — shared code is promoted (hence `components/dashboard-layout.tsx`).
- **Windows FS trap**: case-only renames are invisible to git on this machine and break Linux CI — if any file changes case, stage the rename explicitly (`git rm --cached` + `git add`). 1.2 hit this with `App.tsx`→`app.tsx`.
- **Review-hardening classes to preempt** (all 13 findings in 1.2 were these categories — bake them in now): request-body size caps on every mutation, FK/lookup indexes in the migration, differentiated error codes with the underlying cause logged, boundary-value tests (option count 3/4/5, correctOption 0/1/4/5, time limit 4/5/300/301), no silent error discards, Hebrew-only UI on every reachable path.

### Architecture guardrails (violations = rework)

- **Glossary is law**: `games`, `questions`, `organizer_id`, `join_code` — never `quiz`, `quizId`, `user`, `host`. `Question` type values are `mcq` / `free_text` in DB and Go; wire enum `'mcq' | 'free_text'` in TS.
- **Wire format**: camelCase JSON (`joinCode`, `timeLimitSeconds`, `correctOption`, `acceptedAnswers`, `questionCount`); success = direct payload, lists = `{"items":[...]}`; errors = `{"error":{"code","message"}}`, codes SCREAMING_SNAKE, `message` developer-facing English; dates RFC 3339 UTC.
- **Routes**: plural kebab-case nouns, chi `{gameID}` params, actions as sub-resources (`POST /api/games/{gameID}/questions/reorder`).
- **Dependency direction**: `httpapi` → `store`. No `game` engine package exists yet and this story must NOT create it — CRUD handlers calling the store directly is the architecture's stated shape for the builder ("httpapi handlers translate HTTP ↔ engine/store calls").
- **DB naming**: plural snake_case tables, UUID PKs via `gen_random_uuid()`, `<singular>_id` FKs, `timestamptz` UTC, `idx_<table>_<cols>` indexes, enums as TEXT + CHECK (never Postgres enum types).
- **Game-state enum**: the seven canonical strings are fixed (`draft → lobby → question_open → question_closed → revealed → leaderboard → finished`) — the CHECK in this migration and the TS union in `types.ts` must use them verbatim; later stories depend on the exact spelling.
- **Frontend boundaries**: `features/builder/*` imports `lib/` + `components/` only; `display/*` rules don't apply yet; all Hebrew via `strings.he.ts`; all server types via `types.ts`.
- **Validation timing**: reject at the boundary (HTTP 400); DB CHECKs are the last line, not the first — a CHECK violation reaching Postgres is a bug in handler validation.
- **Logging**: slog JSON, canonical keys (`game_id`, `question_id`, `organizer_id`); INFO lifecycle, WARN degradations, ERROR only for invariant violations — zero ERRORs on a healthy run.

### UX guardrails (Host Dashboard surface — slate register, not Festival Green)

- **Palette**: `bg-host-surface`, `text-host-text`, borders `host-border`; green only as accent (primary buttons `bg-green-800`); gold never (UX-DR2 — neither of gold's two moments is in the builder). Tokens already exist in `index.css` — do not redeclare.
- **Elevation**: Level-1 cards = white + `host-border` border, **no shadow**; shadow on modals only (create-game dialog, question dialog, confirm-delete). Radius: cards 12px (`rounded-md`), dialogs 22px (`rounded-xl`).
- **Controls**: buttons 40px (`h-10`, extended hit area to 48px effective), visible labels (never placeholder-as-label), shadcn focus rings preserved, all text rem, functional at 200% zoom.
- **Error copy (UX-DR12)**: describe what happened + what the system is doing; never "שגיאה" alone, never apology, never blame; programmatic association via `aria-describedby`. Host register is direct and terse — the warm-playful voice belongs to WhatsApp, not here.
- **Bidi (DESIGN.md Typography — mandatory)**: the JOIN Code is a Latin/digit LTR token inside RTL Hebrew; render inside `<bdi>` or a `dir="ltr"` span everywhere it appears (list cards, editor header, create-success view). Unisolated codes visually scramble.
- **Empty states carry a next action** (EXPERIENCE.md): games list → create-first-game CTA; empty game → add-first-question CTA. Never a bare "אין נתונים".
- **A8 time-limit guidance**: the field ships with default 20 and visible guidance that 30–45s accommodates screen-reader users and slower-motor Participants — this is an accessibility requirement, not decorative microcopy.

### Latest tech intelligence (verified against repo state 2026-07-15)

- 1.2's research (2026-07-13) is current — no new npm packages and no new Go modules enter this story. Stack: React 19.2.7, React Router 8.2 (`react-router` single package), TanStack Query 5.101, Tailwind v4.3 + shadcn CLI 4.13 (`rtl: true` in `components.json`, radix umbrella package already a dependency), Vite 8.1, TS ~6.0; Go: chi v5, pgx v5, sqlc 1.31, goose v3.
- shadcn adds needed: `dialog alert-dialog textarea radio-group` — all pull from the existing `radix-ui` umbrella; non-interactive-safe; **re-run the filter-safety greps after adding** (fontsource regression precedent from 1.1).
- sqlc 1.31 + pgx/v5: `TEXT[] NOT NULL` columns emit plain `[]string` — no override needed, no pgtype leak. Arrays bind natively through pgx.
- React Router v8 data-mode: layout routes are plain elements with `<Outlet/>` (the existing `RequireAuth` is the precedent — nest `DashboardLayout` inside it); TanStack Query owns fetching, no route loaders.

### Project Structure Notes

New files: `server/migrations/00003_games_questions.sql` · `server/internal/store/queries/{games,questions}.sql` (+ regenerated `gen/`) · `server/internal/store/{games,questions}.go` (+ join-code test) · `server/internal/httpapi/{errors,games,questions}.go` (+ tests) · `web/src/components/dashboard-layout.tsx` · `web/src/features/builder/{game-editor-page,question-editor}.tsx` · `web/src/lib/types.ts` · shadcn `dialog/alert-dialog/textarea/radio-group` under `components/ui/`.

Updated files: `server/internal/httpapi/router.go` (+ test) · `server/cmd/server/main.go` (wire GameStore) · `web/src/app.tsx` · `web/src/features/builder/games-list-page.tsx` (real list; logout moves to the shell) · `web/src/lib/strings.he.ts` · `_bmad-output/implementation-artifacts/deferred-work.md` (Task 7 outcome).

**Variances from the canonical architecture tree (documented):** (1) `web/src/components/dashboard-layout.tsx` — the canonical tree has no shared-components slot besides `ui/`; the shell is shared by builder now and lobby/live/results later, and features may not import each other, so it lives in `components/` (mirrors 1.2's `cmd/provision` precedent for structural additions). (2) `GET /api/games/{gameID}` embeds the questions array — the architecture names the endpoints but not the composition; one fetch renders the editor and avoids a waterfall. (3) Migration numbering continues the 1.1 variance (00003 here vs. the architecture's illustrative 00002).

### Testing standards

Go stdlib `testing`, co-located, stub interfaces in-test (existing style). This story ships: join-code generator unit tests (charset, length, uniqueness across calls), httpapi games/questions handler tests (ownership 404, validation boundaries, reorder permutation checks, 401/409/413 paths). No real-Postgres integration tests in CI (1.1/1.2 posture) — the manual E2E list covers the DB path including reorder persistence and the fresh-DB migration check. Frontend: no Vitest yet; eslint + tsc strict + build gate, plus the manual E2E browser pass.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-1.3] — story + ACs (verbatim); Epic 1 context
- [Source: _bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md#4.4] — FR-11 (builder, ownership), FR-12 boundary (1.5), Glossary, §6.2 out-of-scope
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation-Patterns-&-Consistency-Rules] — naming, wire format, error envelope, state enum, logging, glossary enforcement
- [Source: _bmad-output/planning-artifacts/architecture.md#Complete-Project-Directory-Structure] — games.go/questions.go placement, features/builder/*, lib/types.ts
- [Source: _bmad-output/planning-artifacts/architecture.md#Data-Architecture] — sqlc/pgx/goose, TEXT+CHECK enums, validation-at-boundaries
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#Game-builder-&-Question-Bank] — A8 default 20s + guidance, Accepted Answers editor + primary form (A15), empty states, mixing rule
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/DESIGN.md] — host-slate palette, elevation/radius/spacing, bidi isolation mandate, shadcn inheritance contract
- [Source: _bmad-output/implementation-artifacts/1-2-organizer-sign-in.md] — established code/test patterns, sqlc NOT-NULL constraint, review-hardening classes, Windows notes
- [Source: _bmad-output/implementation-artifacts/deferred-work.md] — spacing-scale enforcement decision lands in this story (Task 7)

## Dev Agent Record

### Agent Model Used

### Debug Log References

### Completion Notes List

### File List

## Change Log

- 2026-07-15: Story created by create-story workflow — ultimate context engine analysis completed (epics, PRD, architecture, UX spine, DESIGN tokens, 1.2 story intelligence, live codebase read: router/middleware/auth handlers/store/sqlc.yaml/migrations/app.tsx/api.ts/strings/index.css). Status: ready-for-dev.
