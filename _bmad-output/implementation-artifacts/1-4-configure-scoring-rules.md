---
baseline_commit: c9b5ca3
---

# Story 1.4: Configure Scoring Rules

Status: done

## Story

As an Organizer,
I want to set points per correct answer and Speed Bonus values per Game,
so that scoring matches my event.

## Acceptance Criteria

1. **Given** a new Game, **then** scoring defaults are pre-filled: 100 points per correct answer; Speed Bonuses 50/30/20 for 1st/2nd/3rd fastest correct `[ASSUMPTION — defaults not fixed in PRD; consistent with EXPERIENCE.md UJ-2's "+100 נקודות / ⚡ בונוס מהירות +50" example]`.
2. **When** I edit the values, **then** they persist and validate as non-negative integers (zero disables a bonus), **and** the configuration is stored on the Game for the engine to consume (FR-17 configuration half).

## Tasks / Subtasks

- [x] Task 1: Schema + store layer — scoring columns on `games` (AC: 1, 2)
  - [x] `server/migrations/00004_game_scoring.sql` (goose Up/Down — next number after 00003; repo numbering remains offset from the architecture's illustrative list):
    - `ALTER TABLE games ADD COLUMN points_per_correct INTEGER NOT NULL DEFAULT 100 CHECK (points_per_correct BETWEEN 0 AND 10000)`
    - `ALTER TABLE games ADD COLUMN speed_bonus_first INTEGER NOT NULL DEFAULT 50 CHECK (speed_bonus_first BETWEEN 0 AND 10000)`
    - `ALTER TABLE games ADD COLUMN speed_bonus_second INTEGER NOT NULL DEFAULT 30 CHECK (speed_bonus_second BETWEEN 0 AND 10000)`
    - `ALTER TABLE games ADD COLUMN speed_bonus_third INTEGER NOT NULL DEFAULT 20 CHECK (speed_bonus_third BETWEEN 0 AND 10000)`
    - `NOT NULL DEFAULT` backfills every existing game with the canonical defaults — AC-1 is satisfied by the schema, not by client constants. Down: `DROP COLUMN` × 4.
    - Three named columns, NOT an `INTEGER[]`: the PRD fixes bonuses at exactly first/second/third (FR-17), named columns keep CHECKs and the wire self-describing, and `INTEGER NOT NULL` is guaranteed pgtype-free under the sqlc overrides (design decision — see Dev Notes)
  - [x] `internal/store/queries/games.sql` += `UpdateGameScoring :one`: `UPDATE games SET points_per_correct = $3, speed_bonus_first = $4, speed_bonus_second = $5, speed_bonus_third = $6, updated_at = now() WHERE id = $1 AND organizer_id = $2 RETURNING *` — ownership in the WHERE, always (foreign = missing = no row)
  - [x] `internal/store/games.go` += `UpdateGameScoringParams` struct + `UpdateGameScoring` wrapper following the established shape (`store.CreateQuestionParams` precedent): translate `pgx.ErrNoRows` → `store.ErrNotFound`; no pgx sentinels escape `store`
  - [x] Existing queries need **no SQL edits**: `CreateGame`'s `RETURNING *`, `GetGameForOrganizer`'s `SELECT *`, and `ListGamesByOrganizer`'s `g.*` all pick up the new columns on regeneration — new games carry the defaults automatically (AC-1 server-side)
  - [x] Run `sqlc generate`, commit `gen/`; verify the four new `gen.Game` fields are plain `int32` (no pgtype leak — all NOT NULL)
- [x] Task 2: HTTP surface — scoring read + update (AC: 1, 2)
  - [x] `internal/httpapi/games.go`: `gamePayload` += `PointsPerCorrect int32 \`json:"pointsPerCorrect"\``, `SpeedBonusFirst int32 \`json:"speedBonusFirst"\``, `SpeedBonusSecond int32 \`json:"speedBonusSecond"\``, `SpeedBonusThird int32 \`json:"speedBonusThird"\`` — populated in `newGamePayload`, so create/detail responses carry scoring automatically. **No `omitempty` on any scoring field**: 0 is a meaningful value (a disabled bonus) and must serialize — `omitempty` would drop it from the wire (the `questionPayload` omitempty pattern is for type-irrelevant sentinels, which these are not)
  - [x] ⚠️ **`handleListGames` copies `gen.Game` field-by-field** (games.go ~line 163) — add the four new fields to that literal or the list payload silently reports zeros. This is the one easy-to-miss integration point in the story.
  - [x] `handleUpdateScoring` in games.go: `PUT /api/games/{gameID}/scoring` — body `{"pointsPerCorrect": n, "speedBonusFirst": n, "speedBonusSecond": n, "speedBonusThird": n}` with **all four fields as `*int32` and required** (a nil pointer → 400 naming the field; omitted must not silently mean 0, because 0 is a meaningful value that disables a bonus); validate each value 0–10000 `[ASSUMPTION — AC fixes only non-negative; upper bound mirrors the time-limit pattern (bounds in handler + DB CHECK) and keeps worst-case cumulative scores (500 × 100 × 10,000 ≈ 5×10⁸) inside int32]`; flow mirrors `handleUpdateQuestion`: `requireOrganizer` → `gameIDParam` → `decodeJSON` → `requireDraftGame` (non-draft → 409 `GAME_NOT_EDITABLE`) → validate → store call → `writeStoreError(w, err, "GAME_NOT_FOUND")` on failure
  - [x] Response: 200 with the direct scoring payload `{"pointsPerCorrect","speedBonusFirst","speedBonusSecond","speedBonusThird"}` (PUT on the sub-resource returns the sub-resource; the client refetches the game via invalidation); slog INFO `"game scoring updated"` with `game_id`, `organizer_id` (canonical keys, zero new ERROR paths on a healthy run)
  - [x] `GameStore` interface += `UpdateGameScoring(ctx context.Context, arg store.UpdateGameScoringParams) (gen.Game, error)`; router wiring: `gr.Put("/scoring", handleUpdateScoring(games))` inside the existing `/{gameID}` route group; `cmd/server/main.go` unchanged (`*store.Store` already wired)
  - [x] Handler tests in the existing stub style (grow the test-file stub `GameStore`): happy path 200 echoes updated values + store receives validated params · boundary values −1 → 400 / 0 → 200 / 10000 → 200 / 10001 → 400 (each field) · omitted field → 400 naming it · non-integer JSON number (`1.5`) → 400 `INVALID_REQUEST` (decode rejects it) · foreign/missing game → 404 `GAME_NOT_FOUND` · malformed gameID → 404 · non-draft game → 409 `GAME_NOT_EDITABLE` · no session → 401 · oversized body → 413. **Regression guard: all existing tests pass (46 httpapi + 4 store); stub `gen.Game` fixtures may need the new fields — compile errors will point at them**
- [x] Task 3: Frontend — scoring card in the game editor (AC: 1, 2)
  - [x] `web/src/lib/types.ts`: `GameListItem` += `pointsPerCorrect: number`, `speedBonusFirst: number`, `speedBonusSecond: number`, `speedBonusThird: number` (the wire carries them on list and detail alike)
  - [x] New `web/src/features/builder/scoring-editor.tsx` (documented structural addition to the builder feature, `question-editor.tsx` precedent): a Level-1 card (white, `host-border`, `rounded-md`, **no shadow** — not a modal) rendered by `game-editor-page.tsx` **below the questions section**; heading "הגדרות ניקוד"; four numeric fields with visible `Label`s (never placeholder-as-label): points per correct + three Speed Bonus fields (מקום ראשון/שני/שלישי); `Input type="number" min={0} max={10000} step={1}` `h-10 w-32`; a hint line explains that bonuses go to the three fastest correct answers and that 0 disables a bonus
  - [x] Values seeded from `game.data` via `useState` initializer (question-editor precedent — server defaults are the pre-fill; no client-side default constants); `useMutation` PUT on save; `onSuccess` invalidate `['games', gameId]` + `['games']`; submit disabled while `isPending`
  - [x] Saved confirmation: on success render "נשמר ✓" in `text-success` inside an `aria-live="polite"` region (✓ glyph + text — color never the sole signal, UX-DR14); call `save.reset()` when any field changes so a stale confirmation never lingers over unsaved edits
  - [x] Client-side validation mirrors the server (integers 0–10,000 via `Number.isInteger`; no rune counting — numeric fields); error message associated via `aria-describedby`, never a floating toast; server failure shows the UX-DR12-compliant error line
  - [x] `web/src/lib/strings.he.ts` += `scoring` section: title, four field labels, bonus hint (incl. "ערך 0 מבטל את הבונוס"), save, saved-confirmation, validation message, server-error message — **no Hebrew literals in the component**; copy is dev's craft, UX-DR12 principles binding
  - [x] Zero new npm dependencies; no new shadcn components (Button/Input/Label already present)
- [x] Task 4: Quality gates + end-to-end verification (all ACs)
  - [x] All local gates: `gofmt` clean, `go vet ./...`, `go test ./...`, `sqlc generate` (empty diff), `eslint`, `tsc -b --noEmit`, `npm run build`, filter-safety greps
  - [x] Manual E2E against local Postgres (Docker, per README): migration `00004` applies on the existing dev DB (pre-existing games backfilled 100/50/30/20) **and** on a fresh DB; `goose down` removes the four columns and re-`up` succeeds → login → open a game in the editor → scoring card shows 100/50/30/20 pre-filled (AC-1) → edit to e.g. 200/100/0/0 → save → "נשמר ✓" → refresh: values persist (AC-2) → `GET /api/games` list items carry the scoring fields → curl: `-1` → 400 envelope, `10001` → 400, omitted field → 400, second organizer against the first's game → 404 `GAME_NOT_FOUND` → `psql`: `UPDATE games SET state='lobby' WHERE id=...` → curl scoring → 409 `GAME_NOT_EDITABLE` → reset to `draft` → zero ERROR log lines (NFR-8)

## Dev Notes

### What this story is — and is not

The FR-17 **configuration half**, and nothing more: four integer columns on `games`, one `PUT` endpoint, one settings card in the game editor. It does **not** build: any scoring/awarding logic (`game/scoring.go` is Story 3.7 — do not create the `game` package), leaderboard, per-question scoring overrides (scoring is per-Game by design), game rename/delete (still not in any AC), Question Bank (1.5), engine/state transitions (2.3/3.1), Vitest. Zero new Go modules, zero new npm packages, zero new shadcn components. The engine "consumes" the configuration by reading the `games` row — storing it correctly IS the deliverable.

### Design decisions locked for this story (rationale recorded — do not relitigate)

- **Three named columns** (`speed_bonus_first/second/third`), not `INTEGER[]`: the PRD fixes Speed Bonuses at exactly first/second/third (FR-17 — "at most the first, second, and third"), named columns make the DB CHECKs, wire fields, and form labels 1:1 self-describing, and `INTEGER NOT NULL` is guaranteed to emit plain `int32` under the sqlc overrides (the all-NOT-NULL/pgtype-free constraint that shaped 00003). Story 3.7's consumption is trivial: `[]int32{g.SpeedBonusFirst, g.SpeedBonusSecond, g.SpeedBonusThird}`.
- **`PUT /api/games/{gameID}/scoring`** as a sub-resource, not a general game-update endpoint: game rename is out of scope and a generic `PUT /api/games/{gameID}` would invite it. PUT is full-replacement — all four fields required on every call (the form always has all four).
- **Required-pointer decoding** (`*int32`): with plain `int32`, an omitted field silently decodes to 0 — and 0 is a *meaningful* value ("zero disables a bonus"). Absence must therefore be an explicit 400, mirroring how `questionRequest.TimeLimitSeconds` uses a pointer to distinguish omitted-with-default.
- **Bounds 0–10,000** `[ASSUMPTION]`: AC fixes only "non-negative integers"; the cap follows the established bounds pattern (handler check + identical DB CHECK, like `time_limit_seconds BETWEEN 5 AND 300`) and keeps worst-case cumulative int32 scores safe by orders of magnitude.
- **Draft-only mutation** (409 `GAME_NOT_EDITABLE` via the existing `requireDraftGame`): consistent with every 1.3 game/question mutation; trivially true today (every game is `draft` until 2.3). If a live-game scoring edit is ever wanted, that story owns relaxing it.
- **Scoring returned on every game payload** (list + detail): the editor reads it from the existing single fetch — no extra GET endpoint.

### Previous story intelligence (1.3 + its review — established patterns, follow or rework)

- **Handler conventions**: respond only via `writeJSON`/`writeError`/`writeValidationError`/`writeStoreError`; new routes go inside the existing protected `/api/games/{gameID}` group; every DB call bounded by `context.WithTimeout(r.Context(), 5*time.Second)`; organizer ID comes from `OrganizerFromContext` via the `requireOrganizer` helper — never a request param; `gameIDParam` already rejects malformed UUIDs as domain 404s; `decodeJSON` already applies the 64 KiB `MaxBytesReader` cap → 413.
- **Store discipline**: `store` is the only pgx importer; wrappers translate `pgx.ErrNoRows` → `store.ErrNotFound`; params structs live in `store` (`CreateQuestionParams`/`UpdateQuestionParams` precedents in `store/games.go`/`store/questions.go`).
- **sqlc mechanics**: schema dir is `migrations/` (sqlc parses migration SQL); overrides map uuid→`string`, timestamptz→`time.Time`, valid only because every column is NOT NULL — the new columns keep that invariant; v1.31 pinned; commit `gen/`; CI diff-gate includes untracked files.
- **Migrations**: goose v3 Provider API, embedded FS — drop `00004_game_scoring.sql` next to the others, the glob picks it up; verify against fresh AND existing dev DB (1.3 precedent).
- **Test conventions**: table-free `httptest` cases, stub `GameStore` defined in the test files — it grows one method; co-located `_test.go`. 46 httpapi + 4 store tests exist and must pass unchanged (behaviorally — fixtures may gain fields).
- **Frontend conventions**: `api.ts` typed fetch (throws typed `ApiError`, hard-redirects on 401); TanStack `isPending`/`isError`/`isSuccess` only — no hand-rolled flags; mutations disable their trigger; invalidate both `['games', gameId]` and `['games']` (question-editor precedent); features never import each other.
- **From the 1.3 review findings (bake in, don't repeat)**: mutations that can 404 must reconcile — for this story `onSuccess` invalidation suffices (the card doesn't render server rows that can vanish), but keep the invalidation unconditional in spirit; stable keys are moot (fixed four fields); `runeLength` is moot (numeric fields); boundary-value tests on every numeric bound are NOT moot — they were review-mandated in 1.2 and are in Task 2's list.
- **Windows/toolchain notes**: Go 1.26.5 via the `go1.26.5` wrapper (system go is 1.22); local builds may need `-buildvcs=false`; `make` needs Git Bash (README documents the no-make fallback); transient `VirtualAlloc errno=1455` toolchain crashes resolve on re-run; no case-only file renames in this story (all new files lowercase).
- **Local dev leftovers**: dev DB has test organizers `e2e-org-a`/`e2e-org-b` + an E2E game from 1.3 — handy for the foreign-organizer 404 check.

### Architecture guardrails (violations = rework)

- **Glossary is law**: `SpeedBonus` is a PRD Glossary term — `speed_bonus_*` columns, `speedBonus*` wire fields; never `bonus1`/`fastestBonus`/`prize`.
- **Wire format**: camelCase JSON; success = direct payload; errors = `{"error":{"code","message"}}`, codes SCREAMING_SNAKE, messages developer-facing English (user-facing Hebrew lives in `strings.he.ts`).
- **Routes**: actions/sub-resources under the game — `PUT /api/games/{gameID}/scoring` (chi `{gameID}` param style).
- **Dependency direction**: `httpapi` → `store`; no `game` package exists yet and this story must NOT create it.
- **DB naming**: snake_case columns on the existing `games` table; CHECK constraints, not triggers; enums-as-TEXT unaffected.
- **Validation timing**: reject at the boundary (400); the DB CHECK is the last line — a CHECK violation reaching Postgres is a handler bug.
- **Logging**: slog JSON, canonical keys (`game_id`, `organizer_id`); INFO lifecycle only; zero ERRORs on a healthy run.

### UX guardrails (Host Dashboard — slate register)

- **Card**: white `bg-surface-raised`, `border-host-border`, `rounded-md` (12px), **no shadow** (Level-1; shadow is modal-only — this card is not a modal). On-scale spacing stops only (4/8/12/16/24/32/48/64px ↔ Tailwind 1/2/3/4/6/8/12/16; deferred-work convention from 1.3).
- **Controls**: `h-10` buttons/inputs (never shadcn's `h-9`), visible labels above inputs, shadcn focus rings preserved, all text rem.
- **Numbers in RTL**: plain digits in Hebrew text render fine (no bidi isolation needed for numeric `Input`s — the JOIN-Code `<bdi>` mandate applies to mixed Latin/digit tokens, not number fields).
- **Error copy (UX-DR12)**: describe what happened + what to do; never "שגיאה" alone, never apology, never blame; `aria-describedby` association.
- **Success confirmation**: `text-success` token exists in `index.css` (`--color-success: #15803D`, the A18 re-pointed value); ✓ glyph + text so color is never the sole signal (UX-DR14); `aria-live="polite"`.
- **Gold never** (UX-DR2): neither of gold's two moments is in the builder.

### Latest tech intelligence

No new dependencies enter this story — the 1.3 research (verified 2026-07-15 against the repo) is current: React 19.2.7, React Router 8.2, TanStack Query 5.101, Tailwind v4.3 + shadcn (existing Button/Input/Label suffice), Vite 8.1, TS ~6.0; Go: chi v5, pgx v5, sqlc 1.31, goose v3. sqlc 1.31 + pgx/v5 emits plain `int32` for `INTEGER NOT NULL` — no override needed, no pgtype leak.

### Project Structure Notes

New files: `server/migrations/00004_game_scoring.sql` · `web/src/features/builder/scoring-editor.tsx`.

Updated files: `server/internal/store/queries/games.sql` (+ regenerated `gen/`) · `server/internal/store/games.go` · `server/internal/httpapi/games.go` (+ `games_test.go`; stub fixtures in `questions_test.go`/`router_test.go` only if compilation demands) · `server/internal/httpapi/router.go` · `web/src/lib/types.ts` · `web/src/features/builder/game-editor-page.tsx` (render the card) · `web/src/lib/strings.he.ts`.

**Variances from the canonical architecture tree (documented):** (1) `scoring-editor.tsx` — the canonical builder list doesn't name a scoring file; it's a cohesive form component split out of `game-editor-page.tsx` exactly as `question-editor.tsx` is. (2) Migration numbering continues the repo's established offset (00004 here). No other structural changes; `cmd/server/main.go` and `middleware.go` untouched.

### Testing standards

Go stdlib `testing`, co-located, stub interfaces in-test (existing style). This story ships httpapi handler tests only (Task 2 list — validation boundaries, required-field 400s, ownership 404, 409/401/413 paths, list-payload field propagation); the store wrapper is a thin sqlc pass-through covered by the manual E2E (no real-Postgres tests in CI — 1.1/1.2/1.3 posture). Frontend: no Vitest (unchanged posture); eslint + tsc strict + build gate, plus the manual E2E browser pass.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-1.4] — story + ACs (verbatim, incl. the defaults ASSUMPTION); Epic 1 context
- [Source: _bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md#4.6] — FR-17 + consequences (first/second/third, receipt-timestamp ordering — the engine half this story feeds), Glossary (`SpeedBonus`)
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation-Patterns-&-Consistency-Rules] — naming, wire format, error envelope, validation timing, logging
- [Source: _bmad-output/planning-artifacts/architecture.md#Data-Architecture] — sqlc/pgx/goose, validate-at-boundary, CHECK-as-last-line
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#Screen-Inventory] — Game builder includes "configure scoring"; UJ-2 flow shows the +100/+50 values in the wild
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/DESIGN.md] — host-slate palette, elevation/radius/spacing, success token #15803D
- [Source: _bmad-output/implementation-artifacts/1-3-create-a-game-and-author-questions.md] — established code/test patterns, review findings, Windows notes, `requireDraftGame`/`decodeJSON`/`writeStoreError` helpers

## Dev Agent Record

### Agent Model Used

claude-fable-5 (Claude Fable 5, dev-story workflow)

### Debug Log References

- Full Go suite green: 55 httpapi tests (46 pre-existing unchanged in behavior + 9 new scoring tests: persist-and-echo, validation matrix incl. explicit-null, 0/10000 boundaries, non-integer → `INVALID_REQUEST`, foreign 404, malformed-ID short-circuit, non-draft 409, oversized 413, payload propagation detail+list) + 4 store tests; `gofmt`/`go vet` clean; `sqlc generate` idempotent (gen/ committed, four new `int32` fields — no pgtype leak).
- Frontend gates green: `eslint`, `tsc -b --noEmit`, `npm run build`; filter-safety greps clean on source **and** built bundle.
- API E2E (curl vs built binary + Docker Postgres 17): create game returns 100/50/30/20 defaults (AC-1) → PUT scoring 200/100/0/0 → 200 echo with the zero values present on the wire → GET detail + list both persist and carry the fields (AC-2) → −1/10001/omitted-field/float → 400, no session → 401 → boundary 0/10000 → 200 → foreign organizer → 404 `GAME_NOT_FOUND` → `state='lobby'` via psql → 409 `GAME_NOT_EDITABLE` → reset. Zero ERROR log lines; INFO `game scoring updated` with canonical keys.
- Visual E2E (headless Edge via playwright-core, scratchpad install, zero project deps): login → editor → scoring card visible below questions, RTL, pre-filled from server → edit + save → "נשמר ✓" (polite live region) → confirmation clears on further edit → out-of-range 20000 blocked client-side (native constraint validation, no request fired) → reload: saved values persist, unsaved don't → zero console errors. Screenshot reviewed: Level-1 card (border, no shadow), visible labels, h-10 green submit, bidi-isolated JOIN Code intact.
- Migration check: `00004` applied at boot on the existing dev DB (pre-existing 1.3 game backfilled 100/50/30/20; CHECKs verified via `\d games`) and on a fresh scratch DB; `goose down` drops exactly the four columns and re-`up` restores them (run via a temporary in-module `cmd/goose-e2e` helper — the goose CLI needs go.sum entries we don't carry — deleted after the pass).

### Completion Notes List

- FR-17 configuration half complete: four scoring columns on `games` (defaults 100/50/30/20 as schema truth), ownership-scoped `UpdateGameScoring` store wrapper, `PUT /api/games/{gameID}/scoring` with required-pointer decoding (omitted ≠ 0) and 0–10,000 bounds mirrored in DB CHECKs, scoring on every game payload (detail + list), and the "הגדרות ניקוד" card in the game editor.
- Implementation matches the story's locked design decisions with no deviations. Two clarifications recorded:
  - Out-of-range form input is blocked first by native constraint validation (`min`/`max`/`step`/`required` — same behavior as the 1.3 question-editor's time-limit field); the component's `Number.isInteger` bounds check remains as the programmatic safety net and drives `strings.scoring.validation` when native validation is bypassed.
  - The saved-confirmation `aria-live` region is always mounted (empty when idle) so screen readers announce the content change — not conditionally rendered.
- E2E note (test tooling, not app behavior): Hebrew sent through Git Bash `curl -d` arrived mojibake into the create-game title — an input-encoding artifact of the test shell; the browser/SPA path stores Hebrew correctly (1.3 E2E verified that). Fixed via psql for the browser pass; all scoring E2E data (game + `e2e-org-scoring`/`e2e-org-scoring-b` organizers) deleted from the dev DB afterwards.
- Zero new dependencies (Go and npm); no new shadcn components; all Hebrew via `strings.he.ts`; no `game` package created.

### File List

New:
- server/migrations/00004_game_scoring.sql
- web/src/features/builder/scoring-editor.tsx

Modified:
- server/internal/store/queries/games.sql
- server/internal/store/games.go
- server/internal/store/gen/games.sql.go (generated)
- server/internal/store/gen/models.go (generated)
- server/internal/httpapi/games.go
- server/internal/httpapi/games_test.go
- server/internal/httpapi/router.go
- web/src/features/builder/game-editor-page.tsx
- web/src/lib/types.ts
- web/src/lib/strings.he.ts
- _bmad-output/implementation-artifacts/sprint-status.yaml
- _bmad-output/implementation-artifacts/1-4-configure-scoring-rules.md

## Change Log

- 2026-07-16: Story created by create-story workflow — ultimate context engine analysis completed (epics, PRD §4.6/FR-17, architecture patterns + data rules, EXPERIENCE.md builder/UJ-2, DESIGN.md tokens, 1.3 story + review intelligence, live codebase read: migration 00003, store/games.go, httpapi games/questions/errors/router, game-editor-page, question-editor, types.ts, strings.he.ts, text.ts, sqlc.yaml, index.css tokens). Status: ready-for-dev.
- 2026-07-16: Story implemented by dev-story workflow (claude-fable-5) — all 4 tasks complete: migration 00004 + store layer, PUT /scoring endpoint with 9 new handler tests, scoring card in the game editor, full quality gates + API/visual/migration E2E against local Postgres. Status: review.
- 2026-07-16: Adversarial code review (bmad-code-review, 3 layers) — 5 patch findings, all fixed same-session: handler reordered to draft-check-before-validation (+ `TestUpdateScoringDraftCheckPrecedesValidation` covering the 409/404-with-invalid-body gap), `key={game.data.id}` remounts `ScoringEditor` per game, blank-field rejection before `Number()` coercion, validation message cleared on edit, field-level `aria-describedby` error association. 2 deferred to deferred-work.md (409/404 save-error UX + draft TOCTOU — both unreachable until story 2.3), 4 dismissed as noise. Gates re-run green: gofmt/vet/`go test ./...` (56 httpapi + 4 store), eslint, tsc, build. Status: done.

## Review Findings

_Adversarial code review (bmad-code-review, 3 layers: Blind Hunter · Edge Case Hunter · Acceptance Auditor), 2026-07-16. 11 distinct issues after dedup: 5 patch, 2 defer, 4 dismissed as noise._

- [x] [Review][Patch] Scoring handler validates the body **before** the draft/ownership check, reversing the spec-pinned flow (`requireOrganizer → gameIDParam → decodeJSON → requireDraftGame → validate`). A non-draft or foreign game submitted with an out-of-range/omitted value returns `400 VALIDATION_FAILED` instead of `409 GAME_NOT_EDITABLE` / `404 GAME_NOT_FOUND`. `handleUpdateQuestion`/`handleCreateQuestion` both check draft first; the existing 409/404 tests miss this because they use `validScoringBody`. Fix: move the `requireDraftGame` block above the validation loop. [server/internal/httpapi/games.go:257]
- [x] [Review][Patch] `ScoringEditor` `useState` seed goes stale vs refetched game data — the four values seed from props at mount only, with no `key`/remount on the render. Switching games (same `GameEditorPage` instance) or a background refetch (TanStack `refetchOnWindowFocus`) leaves the form showing mount-time values; Save then re-PUTs the stale scoring, silently reverting newer state. Fix: `key={game.data.id}` on `<ScoringEditor>` (covers game-switch), and treat server as source for the refetch case. [web/src/features/builder/scoring-editor.tsx:26 · game-editor-page.tsx:161]
- [x] [Review][Patch] Empty number field coerces to `0` via `Number("") === 0` and passes `Number.isInteger`/bounds — guarded only by the native `required` attribute. If native validation is bypassed (programmatic submit, assistive tech, non-compliant browser) a blanked field persists as a real `0`. Fix: reject empty/blank strings in `submit()` before `Number()`. [web/src/features/builder/scoring-editor.tsx:52]
- [x] [Review][Patch] Stale client validation message never clears on edit — `edit()` calls `save.reset()` (server error only) but not `setValidationMessage(undefined)`, so after an invalid submit the red "0–10,000" text lingers over a corrected field until the next submit. Fix: clear `validationMessage` in the `edit` handler. [web/src/features/builder/scoring-editor.tsx:46]
- [x] [Review][Patch] Error text is associated via `aria-describedby` on the `<form>`, not on the inputs — a screen reader focused on a field won't announce the validation/server error. Fix: add `scoring-error` to each `<Input>`'s `aria-describedby` when an error is present. [web/src/features/builder/scoring-editor.tsx:86]
- [x] [Review][Defer] Save error maps 409/404 to the single generic "try again in a moment" (no `onError`), and the card renders regardless of `game.state` — deferred, not reachable in current product state (every game is `draft` until story 2.3; no game-delete exists, so 404/409 cannot occur yet). Owned by the game-start story. [web/src/features/builder/scoring-editor.tsx:38]
- [x] [Review][Defer] Draft-only guard is check-then-write (TOCTOU) — `UpdateGameScoring`'s `UPDATE … WHERE id = $1 AND organizer_id = $2` omits `state = 'draft'`, so a concurrent draft→lobby transition between the `requireDraftGame` read and the write could still persist scoring — deferred, pre-existing pattern (every existing question mutation relies on the same check-then-write with no SQL state guard; unreachable until game-start (2.3) can leave draft). [server/internal/store/queries/games.sql:19]

**Dismissed as noise (4):** (1) Blind Hunter — "`Game` type never gains the scoring fields" — false positive: `Game extends GameListItem` (types.ts:47), fields inherited, tsc-clean. (2) Non-`ErrNoRows` DB error mapping — `writeStoreError` default → `503 DB_UNAVAILABLE` (not 404), and the CHECK path is unreachable because handler bounds mirror the DB CHECK; consistent with every handler. (3) Redundant `invalidateQueries(['games', id])` + `(['games'])` — follows the spec-mandated question-editor precedent; fuzzy-match makes the first a harmless no-op. (4) `NOT NULL DEFAULT` table-rewrite risk — project runs Postgres 17, where `ADD COLUMN … DEFAULT <const>` is metadata-only.
