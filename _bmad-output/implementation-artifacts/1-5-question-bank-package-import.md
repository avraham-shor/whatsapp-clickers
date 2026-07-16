---
baseline_commit: 7f92258
---

# Story 1.5: Question Bank Package Import

Status: in-progress

## Story

As an Organizer,
I want to browse the Question Bank and import a Question Package into my Game,
so that I can run a ready-made quiz and mix it with my own questions.

## Acceptance Criteria

1. **Given** the Question Bank contains at least one Package (`question_packages` tables created and a sample pack seeded in this story; pilot content sourcing remains OQ-4), **when** I browse the bank, **then** I see each Package's name and question count.
2. **When** I import a Package into my Game, **then** all its Questions are **copied** and appended in order to my Game (FR-12).
3. **Given** an imported Question, **when** I edit or delete it in my Game, **then** the Question Bank Package is unchanged (copy semantics), **and** imported and custom Questions can be mixed and reordered freely.
4. **Given** the bank browse view, **then** each Package card shows title, question count, and a question preview (A13), imported copies carry a "מהמאגר" badge, a Game with no Questions prompts "הוסיפו שאלה ראשונה — או ייבאו חבילה מהמאגר" with both CTAs, and an empty Question Bank states that packages are on the way rather than showing a bare list (EXPERIENCE.md empty states).

## Tasks / Subtasks

- [x] Task 1: Schema + seed — migration `00005_question_packages.sql` (AC: 1, 2, 4)
  - [x] `server/migrations/00005_question_packages.sql` (goose Up/Down — next number after 00004; repo numbering remains offset from the architecture's illustrative list):
    - `question_packages`: `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`, `title TEXT NOT NULL`, `created_at`/`updated_at timestamptz NOT NULL DEFAULT now()`. No description column — A13 needs title, count, and preview only; don't invent fields.
    - `package_questions`: `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`, `package_id UUID NOT NULL REFERENCES question_packages(id) ON DELETE CASCADE`, `position INTEGER NOT NULL`, then the **exact content-column shape of `questions`** (copy from 00003 verbatim): `type TEXT NOT NULL CHECK (type IN ('mcq','free_text'))`, `text TEXT NOT NULL`, `options TEXT[] NOT NULL DEFAULT '{}'`, `correct_option INTEGER NOT NULL DEFAULT 0`, `accepted_answers TEXT[] NOT NULL DEFAULT '{}'`, `time_limit_seconds INTEGER NOT NULL DEFAULT 20 CHECK (time_limit_seconds BETWEEN 5 AND 300)`, `created_at`/`updated_at`, plus the same `_type_shape` CHECK as `questions` (renamed `package_questions_type_shape`). Identical shape is what makes the import a column-for-column `INSERT … SELECT`. Index: `idx_package_questions_package_id_position (package_id, position)`.
    - `ALTER TABLE questions ADD COLUMN imported_from_bank BOOLEAN NOT NULL DEFAULT false` — the "מהמאגר" badge marker; `BOOLEAN NOT NULL` emits plain `bool` (no pgtype leak). Existing rows backfill `false` (custom questions), which is correct.
    - Seed one sample Package in the same migration (plain `INSERT` statements; goose runs them at boot): a **fixed UUID literal** for the package row so the question INSERTs can reference it without RETURNING plumbing; ~10 Hebrew questions at positions 1..N, mixed MCQ and Free-Text, mixed difficulty across generations **including Torah-knowledge questions** (the FR-12 "Saba wins" multigenerational mandate), sensible per-question time limits (20–45s). Content quality is dev's craft; phrasing follows the WhatsApp copy rules that will eventually deliver it (gender-neutral, plural imperatives — no "ברוך הבא"). Seeding does NOT close OQ-4 — the real pilot pack is a content task; this pack must still be genuinely playable.
    - Down: `ALTER TABLE questions DROP COLUMN imported_from_bank; DROP TABLE package_questions; DROP TABLE question_packages;`
  - [x] ⚠️ The migration file contains Hebrew — it must be written UTF-8 without BOM (the Write tool does this; do NOT create it via PowerShell `Out-File`, which defaults to UTF-16 on this machine). Verify with `psql` after boot that seeded text renders correctly (the 1.4 Git-Bash-curl mojibake was a shell-input artifact; file-based SQL is safe, but verify anyway).
- [x] Task 2: Store layer — packages queries + import copy (AC: 1, 2, 3)
  - [x] `internal/store/queries/packages.sql`:
    - `ListQuestionPackages :many`: `SELECT p.*, count(pq.id) AS question_count, COALESCE((SELECT pq2.text FROM package_questions pq2 WHERE pq2.package_id = p.id ORDER BY pq2.position, pq2.created_at LIMIT 1), '')::text AS preview FROM question_packages p LEFT JOIN package_questions pq ON pq.package_id = p.id GROUP BY p.id ORDER BY p.created_at, p.title` — preview = first question's text (A13); the `COALESCE(...)::text` keeps sqlc emitting plain `string`, never a nullable.
    - `GetQuestionPackage :one`: `SELECT * FROM question_packages WHERE id = $1` — existence check so a missing package is distinguishable from an empty import.
    - `ImportPackageQuestions :many` — the whole copy in one atomic statement, ownership in the WHERE (1.3 `CreateQuestion` pattern):
      ```sql
      INSERT INTO questions (game_id, position, type, text, options, correct_option, accepted_answers, time_limit_seconds, imported_from_bank)
      SELECT g.id,
             COALESCE((SELECT max(q.position) FROM questions q WHERE q.game_id = g.id), 0)
               + (row_number() OVER (ORDER BY pq.position, pq.created_at))::int,
             pq.type, pq.text, pq.options, pq.correct_option, pq.accepted_answers, pq.time_limit_seconds, true
      FROM games g
      JOIN package_questions pq ON pq.package_id = sqlc.arg(package_id)
      WHERE g.id = sqlc.arg(game_id) AND g.organizer_id = sqlc.arg(organizer_id)
      RETURNING *;
      ```
      `row_number()` (not raw `pq.position`) makes the appended positions MAX+1..MAX+N regardless of gaps in package numbering; the MAX subquery sees the pre-statement snapshot, so all N rows are internally consistent.
  - [x] `internal/store/packages.go`: `ListQuestionPackages(ctx) ([]gen.ListQuestionPackagesRow, error)` (thin pass-through) · `ImportPackageQuestions(ctx, gameID, organizerID, packageID string) ([]gen.Question, error)`: call `GetQuestionPackage` first — `pgx.ErrNoRows` → `store.ErrNotFound` (this is the PACKAGE_NOT_FOUND path); then run the import query and **sort the returned rows by `Position` ascending** before returning (`RETURNING *` row order is not guaranteed); no pgx sentinels escape `store`. No explicit transaction: both statements are single atomic queries, packages have no delete path, and the MAX+1 concurrency class is the same documented deferred item as 1.3's `CreateQuestion` (self-healing via reorder — see Dev Notes). An existing-but-empty package imports zero rows and returns an empty slice — 201 with empty items, no guard (unreachable with the seeded data; packages only enter via migrations).
  - [x] Run `sqlc generate`, commit `gen/`; verify `gen.Question` gains `ImportedFromBank bool`, `ListQuestionPackagesRow` carries `QuestionCount int64` + `Preview string`, and nothing pgtype leaks (all new columns NOT NULL)
- [x] Task 3: HTTP surface — bank browse + import endpoints (AC: 1, 2)
  - [x] New `internal/httpapi/packages.go` (canonical architecture file — FR-12's home):
    - `handleListQuestionPackages`: `GET /api/question-packages` (inside the protected group; still open with the `requireOrganizer` guard per the `handleListGames` precedent — the bank is first-party and identical for every organizer, no ownership scoping). Response `{"items":[{"id","title","questionCount","preview"}]}` — `packagePayload` struct, camelCase, no timestamps on the wire (the browse view doesn't render them).
    - `handleImportPackage`: `POST /api/games/{gameID}/questions/import-package` — body `{"packageId":"<uuid>"}`. Flow pins the 1.4-review-mandated order: `requireOrganizer` → `gameIDParam` → `decodeJSON` → `requireDraftGame` (non-draft → 409 `GAME_NOT_EDITABLE`; foreign/missing → 404 `GAME_NOT_FOUND`) → validate `packageId` with `isUUID` — malformed/missing → 400 `VALIDATION_FAILED` "packageId must be a valid question package id" (**body-ID precedent from reorder, not the path-param 404 precedent**) → `store.ImportPackageQuestions` → `writeStoreError(w, err, "PACKAGE_NOT_FOUND")` → 201 with `{"items":[...questionPayload]}` of the copied questions in position order (created resources; the client refetches the game via invalidation anyway).
    - slog INFO `"package imported"` with `game_id`, `package_id`, `question_count` (canonical keys; zero new ERROR paths on a healthy run).
  - [x] `internal/httpapi/questions.go`: `questionPayload` += `ImportedFromBank bool \`json:"importedFromBank"\`` (no omitempty — always on the wire, TS keeps a non-optional boolean), populated in `newQuestionPayload`. `CreateQuestion`/`UpdateQuestion` queries need **no SQL edits**: the INSERT's explicit column list leaves the new column defaulting to `false` for custom questions, `UpdateQuestion`'s SET never touches it (an imported copy keeps its badge through edits — provenance, not sync state), and every `RETURNING *` picks it up on regeneration.
  - [x] `GameStore` interface += `ListQuestionPackages(ctx context.Context) ([]gen.ListQuestionPackagesRow, error)` + `ImportPackageQuestions(ctx context.Context, gameID, organizerID, packageID string) ([]gen.Question, error)` (grow the one interface — the import handler also needs `GetGameForOrganizer` via `requireDraftGame`, so a separate interface would just split the stub). Router wiring: `protected.Get("/question-packages", handleListQuestionPackages(games))` + `qr.Post("/import-package", handleImportPackage(games))` inside the existing `/questions` group; `cmd/server/main.go` unchanged.
  - [x] Handler tests in the existing stub style (grow the test-file stub `GameStore`; fixtures gain `ImportedFromBank` zero-value automatically): list packages → 200 items with id/title/questionCount/preview · list without session → 401 · import happy path → 201 items in position order, store receives (gameID, organizerID, packageID), payload rows carry `importedFromBank: true` · unknown-but-valid packageId (store returns `ErrNotFound`) → 404 `PACKAGE_NOT_FOUND` · malformed packageId → 400 naming the field, no store call · missing packageId → 400 · non-draft game → 409 `GAME_NOT_EDITABLE` **with a valid body** and also **with a malformed-packageId body** (draft check precedes packageId validation — the 1.4 review regression class; invalid JSON still 400s at decode, which precedes everything) · foreign/missing game → 404 `GAME_NOT_FOUND` · malformed gameID → 404 · invalid JSON → 400 `INVALID_REQUEST` · oversized body → 413 · no session → 401. **Regression guard: all existing tests pass (56 httpapi + 4 store)**
- [x] Task 4: Frontend — import dialog, badge, empty states (AC: 1, 2, 3, 4)
  - [x] `web/src/lib/types.ts`: `Question` += `importedFromBank: boolean`; new `QuestionPackage` interface `{ id: string; title: string; questionCount: number; preview: string }` (+ list shape reuses the `{items}` convention).
  - [x] New `web/src/features/builder/package-import-dialog.tsx` (canonical architecture file): shadcn Dialog (modal — the one shadow-allowed surface, `rounded-xl`), rendered conditionally from the editor like `QuestionEditor` (`{importOpen && <PackageImportDialog gameId={gameId} onClose={...} />}`). Inside: `useQuery` on `['question-packages']` → `GET /api/question-packages`; package cards (white, `host-border`, `rounded-md`, no shadow inside the dialog) each showing title, question count, and the preview line (`text-host-text-secondary`, CSS `line-clamp-2`) + an import Button `h-10` per card. `useMutation` → `POST /api/games/{gameId}/questions/import-package` `{packageId}`; `onSuccess`: invalidate `['games', gameId]` + `['games']`, close the dialog (the appended rows with their badges are the visible confirmation); `isPending` disables every import button (one import at a time). Loading state `strings.common.loading`; query failure and mutation failure each render a UX-DR12 error line (`role="alert"`); empty bank renders the "packages are on the way" message — never a bare list (AC-4; unreachable once seeded, still required).
  - [x] `web/src/features/builder/game-editor-page.tsx`: header gains a secondary outline button "ייבוא מהמאגר" beside "הוספת שאלה"; the empty state becomes the dual-CTA prompt per AC-4 — both action buttons inside the empty-state card ("הוסיפו שאלה ראשונה" opening the question editor + "ייבאו חבילה מהמאגר" opening the import dialog); `QuestionRow` renders a "מהמאגר" badge next to the type badge when `question.importedFromBank` (same pill styling as the type badge). Mixing/reorder/edit/delete need **zero new code** — they are ID-based and agnostic to provenance (AC-3 is satisfied by copy-by-construction; E2E proves it).
  - [x] `web/src/lib/strings.he.ts` += `questionBank` section: dialog title, open-dialog button label ("ייבוא מהמאגר"), per-card import CTA, question-count function, imported badge ("מהמאגר"), empty-bank title/body (packages on the way), load error, import error; update `gameEditor.emptyState*` for the dual-CTA prompt. **No Hebrew literals in components**; copy is dev's craft, UX-DR12 principles binding.
  - [x] Zero new npm dependencies; zero new shadcn components (Dialog/Button already present — re-run the filter-safety greps anyway if any shadcn command is run, which it shouldn't be)
- [x] Task 5: Quality gates + end-to-end verification (all ACs)
  - [x] All local gates: `gofmt` clean, `go vet ./...`, `go test ./...`, `sqlc generate` (empty diff), `eslint`, `tsc -b --noEmit`, `npm run build`, filter-safety greps
  - [x] Manual E2E against local Postgres (Docker, per README): migration `00005` applies at boot on the existing dev DB **and** a fresh DB; seeded pack present with correct Hebrew (`psql \d` + SELECT); `goose down` drops the two tables + column and re-`up` re-seeds → login → editor on an empty game shows the dual-CTA empty state (AC-4) → open the bank dialog → package card shows title, question count, preview (AC-1, A13) → import → questions appended after existing ones in package order, each with the "מהמאגר" badge (AC-2, AC-4) → edit an imported question, delete another → `psql`: `package_questions` rows unchanged (AC-3 copy semantics) → add a custom question, reorder it between imported ones → refresh: mixed order persists (AC-3) → import the same package again → second copy appended (documented design) → curl: bad-format packageId → 400, unknown packageId → 404 `PACKAGE_NOT_FOUND`, foreign game → 404 `GAME_NOT_FOUND`, `psql` state='lobby' → import → 409 `GAME_NOT_EDITABLE` → reset draft → `GET /api/games` list `questionCount` includes imported copies → zero ERROR log lines (NFR-8)

## Dev Notes

### What this story is — and is not

FR-12 complete: two bank tables + one provenance column, a seeded sample Package, one GET + one POST endpoint, one import dialog, one badge, two empty states. It does **not** build: package authoring/CRUD (the bank is first-party, read-only in the pilot — content arrives via migrations; PRD §5 "not a content marketplace"), a standalone Question Bank page or nav entry (see design decisions), per-question category labels (deferred by PRD addendum), game rename/delete, any engine/lobby/WhatsApp code (2.x), Vitest. Zero new Go modules, zero new npm packages, zero new shadcn components. Editing imported copies reuses the 1.3 question editor untouched.

### Design decisions locked for this story (rationale recorded — do not relitigate)

- **Two tables mirroring `questions`' content shape** (`question_packages` + `package_questions`): the epics say "`question_packages` tables" (plural) and copy semantics demand a source of identical shape — a column-for-column `INSERT … SELECT` is the whole import. FK is `package_id` `[documented variance from the strict <singular>_id convention — question_package_id is noise when the child table already lives in the package_ namespace; mirrors the {packageID}-style brevity of the domain]`.
- **`imported_from_bank BOOLEAN`, not a `source_package_id` FK**: the badge needs one bit; a nullable UUID would leak pgtype into every `questions` model (the all-NOT-NULL/sqlc constraint from 1.2/1.3), and copy semantics mean there is nothing to sync back — provenance beyond the badge has no consumer. The badge survives edits by design (it marks origin, not freshness).
- **Copies are physical rows in `questions`**: FR-12's "editing them does not modify the Question Bank" is satisfied by construction, not by guards. Everything downstream (reorder, edit, delete, question dispatch in 3.2) sees ordinary questions.
- **Routes**: `GET /api/question-packages` (plural kebab-case Glossary noun — `packages` alone is ambiguous) and `POST /api/games/{gameID}/questions/import-package` (action on the game's questions collection — the `questions/reorder` precedent, since the effect is "append rows to this game's questions").
- **Malformed body `packageId` → 400, unknown → 404 `PACKAGE_NOT_FOUND`**: two ID-validation precedents exist — path params degrade to domain 404s (`gameIDParam`), body IDs fail validation with 400 (reorder's `questionIds`). `packageId` travels in the body → 400. The bank is shared, so package existence is not an ownership secret; `PACKAGE_NOT_FOUND` follows the `GAME_NOT_FOUND`/`QUESTION_NOT_FOUND` flavor via `writeStoreError`.
- **Import returns 201 `{"items":[...]}` of the copied questions**: created resources in position order; the dialog still invalidates `['games', gameId]` — the response body is for API coherence, not the UI's data path.
- **Re-importing the same package appends another copy**: nothing in FR-12 forbids it, dedupe would need provenance tracking rejected above, and the Organizer can delete rows freely. Documented, not guarded.
- **No standalone bank page/nav entry**: import's only verb needs a target Game, so the browse view lives in the editor's dialog (`package-import-dialog.tsx` is the architecture's canonical file; the tree has no bank page). EXPERIENCE.md's "Question Bank" surface is this dialog.
- **Seed in the migration, not in app code**: migrations already run at boot on deploy (architecture CI/CD); a seed API or Go fixture would be new machinery for static first-party content. Fixed UUID literal for the package row keeps the SQL self-contained.
- **Draft-only import** (409 via existing `requireDraftGame`): consistent with every 1.3/1.4 mutation; the check-then-write TOCTOU remains the documented deferred posture (unreachable until 2.3).

### Previous story intelligence (1.4 + its review — established patterns, follow or rework)

- **Handler order is review-pinned**: `requireOrganizer` → `gameIDParam` → `decodeJSON` → `requireDraftGame` → validate body → store call. The 1.4 review's top finding was validating before the draft/ownership check — a non-draft or foreign game must answer 409/404 even with a garbage body. Write the precedence test up front.
- **Handler conventions**: respond only via `writeJSON`/`writeError`/`writeValidationError`/`writeStoreError`; every DB call bounded by `context.WithTimeout(r.Context(), 5*time.Second)`; organizer ID only from `OrganizerFromContext` via `requireOrganizer`; `decodeJSON` already applies the 64 KiB cap → 413; `isUUID` already exists in errors.go.
- **Store discipline**: `store` is the only pgx importer; wrappers translate `pgx.ErrNoRows` → `store.ErrNotFound`; params stay primitive or in `store`-defined structs; `writeStoreError`'s default branch already logs and maps infrastructure errors to 503.
- **sqlc mechanics**: schema dir is `migrations/` (sqlc parses migration SQL, goose annotations ignored); overrides map uuid→`string`, timestamptz→`time.Time`, valid only because every column is NOT NULL — the new tables and column keep that invariant; v1.31 pinned; commit `gen/`; CI diff-gate includes untracked files.
- **Migrations**: goose v3 Provider API, embedded FS — drop `00005_question_packages.sql` next to the others, the glob picks it up; verify fresh AND existing dev DB, and that Down/re-Up round-trips (1.3/1.4 precedent).
- **Test conventions**: table-free `httptest` cases, stub `GameStore` grown in the test files; co-located `_test.go`; 56 httpapi + 4 store tests exist and must pass (fixtures may gain the new bool field via zero value — compile errors will point at any struct literals that need it).
- **Frontend conventions**: `api.ts` typed fetch (throws typed `ApiError`, hard-redirects on 401); TanStack `isPending`/`isError`/`isSuccess` only; mutations disable their trigger; invalidate both `['games', gameId]` and `['games']` (scoring/question precedents); conditional-render dialogs (`QuestionEditor` precedent); features never import each other.
- **From the 1.4 review (bake in, don't repeat)**: mutations whose refetch can change what's rendered must reconcile via invalidation (the dialog's `onSuccess` invalidation covers it — imported rows render from the game query, not dialog state); stale-form staleness is moot here (the dialog holds no editable fields); error text must be associated where it's announced (`role="alert"` on the dialog's error line).
- **Windows/toolchain notes**: Go 1.26.5 via the `go1.26.5` wrapper (system go is 1.22); local builds may need `-buildvcs=false`; `make` needs Git Bash (README documents the fallback); transient `VirtualAlloc errno=1455` crashes resolve on re-run; **Hebrew must never pass through Git Bash `curl -d`** (1.4 mojibake) — E2E Hebrew assertions go through the browser or psql; all new files lowercase (no case-only renames).
- **Local dev leftovers**: dev DB has test organizers `e2e-org-a`/`e2e-org-b` + a 1.3 E2E game — handy for the foreign-game 404 check.

### Architecture guardrails (violations = rework)

- **Glossary is law**: `Question Bank` / `Question Package` are Glossary terms — `question_packages`, `package_questions`, `packageId`, `QuestionPackage`; never `bank`, `pack`, `library`, `quiz_set` as identifiers.
- **Wire format**: camelCase JSON; lists = `{"items":[...]}`; errors = `{"error":{"code","message"}}`, codes SCREAMING_SNAKE (`PACKAGE_NOT_FOUND` joins the family), messages developer-facing English.
- **Routes**: plural kebab-case nouns + actions as sub-resources (`/api/question-packages`, `/questions/import-package`); chi `{gameID}` param style.
- **Dependency direction**: `httpapi` → `store`; no `game` package exists yet and this story must NOT create it; `store` remains the only pgx importer.
- **DB naming**: plural snake_case tables, `gen_random_uuid()` PKs, `<singular>_id` FKs (documented `package_id` variance above), `timestamptz` UTC, `idx_<table>_<cols>` indexes, enums as TEXT + CHECK.
- **Validation timing**: reject at the boundary (400); the DB CHECKs on `package_questions` guard seed-content mistakes, not runtime input (no write path exists besides migrations).
- **Logging**: slog JSON, canonical keys (`game_id`, `package_id`, `organizer_id`, `question_count`); INFO lifecycle only; zero ERRORs on a healthy run (NFR-8).

### UX guardrails (Host Dashboard — slate register)

- **Dialog**: shadcn Dialog, `rounded-xl` (22px), shadow allowed (modal is the one elevated surface); cards inside stay Level-1 (white, `host-border`, `rounded-md`, no shadow). On-scale spacing stops only (Tailwind 1/2/3/4/6/8/12/16); buttons `h-10`, never `h-9`.
- **Empty states carry a next action** (EXPERIENCE.md): the empty game prompts with **both** CTAs (AC-4 quotes the prompt); the empty bank explains packages are on the way — never a bare "אין נתונים". Empty-state buttons are real actions, not disabled stubs.
- **Badge**: "מהמאגר" as a quiet pill matching the existing type-badge styling (`rounded-sm border host-border`, secondary text) — informational, not semantic-colored; color is never the sole signal and here color isn't a signal at all.
- **Error copy (UX-DR12)**: describe what happened + what to do; never "שגיאה" alone; `role="alert"` on dialog error lines; loading is `strings.common.loading`, no hand-rolled flags.
- **Hebrew previews are plain RTL text** — no bidi isolation needed (the `<bdi>` mandate covers Latin/digit tokens like the JOIN Code, not Hebrew content).
- **Gold never** (UX-DR2): neither of gold's two moments is in the builder.

### Latest tech intelligence

No new dependencies enter this story — the 1.3/1.4 research (verified against the repo 2026-07-16) is current: React 19.2.7, React Router 8.2, TanStack Query 5.101, Tailwind v4.3 + shadcn (Dialog/Button/Input already installed), Vite 8.1, TS ~6.0; Go: chi v5, pgx v5, sqlc 1.31, goose v3; Postgres 17 local (Docker) and Railway managed. Notes that matter here: sqlc 1.31 emits plain `bool` for `BOOLEAN NOT NULL` and `int64` for `count(...)`; `COALESCE(...)::text` guarantees a non-nullable `string` for the preview column; `row_number()` window functions are valid inside `INSERT … SELECT` on Postgres 17; goose runs plain multi-statement SQL without `StatementBegin` annotations as long as no statement contains internal semicolons.

### Project Structure Notes

New files: `server/migrations/00005_question_packages.sql` · `server/internal/store/queries/packages.sql` · `server/internal/store/packages.go` · `server/internal/httpapi/packages.go` (+ `packages_test.go`) · `web/src/features/builder/package-import-dialog.tsx`.

Updated files: `server/internal/store/gen/` (regenerated) · `server/internal/httpapi/questions.go` (payload field) · `server/internal/httpapi/games.go` (GameStore interface) · `server/internal/httpapi/router.go` · `web/src/features/builder/game-editor-page.tsx` · `web/src/lib/types.ts` · `web/src/lib/strings.he.ts`.

**Variances from the canonical architecture tree (documented):** (1) migration numbering continues the repo's offset (00005 here vs. the architecture's illustrative 00004). (2) `package_id` FK naming (rationale in design decisions). (3) No standalone bank page — the canonical tree itself has only `package-import-dialog.tsx`, so this is conformance, recorded because EXPERIENCE.md lists "Question Bank" as a surface: that surface is the dialog. `cmd/server/main.go`, `middleware.go`, `app.tsx` untouched.

### Testing standards

Go stdlib `testing`, co-located, stub interfaces in-test (existing style). This story ships httpapi handler tests only (Task 3 list — both endpoints, the draft-check-precedes-validation regression class, body-ID 400 vs store 404 split, payload propagation incl. `importedFromBank`); the store wrappers are thin sqlc pass-throughs covered by the manual E2E (no real-Postgres tests in CI — 1.1–1.4 posture). The import query's correctness (append positions, order preservation, copy fidelity) is exercised E2E against real Postgres. Frontend: no Vitest (unchanged posture); eslint + tsc strict + build gate, plus the manual E2E browser pass.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-1.5] — story + ACs (verbatim); Epic 1 context; "question_packages tables created and a sample pack seeded in this story"
- [Source: _bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md#4.4] — FR-12 + copy-semantics consequence, the multigenerational/Torah-knowledge pilot-content mandate (OQ-4), Glossary (Question Bank, Question Package), §5 "not a content marketplace"
- [Source: _bmad-output/planning-artifacts/architecture.md#Complete-Project-Directory-Structure] — `packages.sql`, `httpapi/packages.go`, `package-import-dialog.tsx` placement; `question_packages` in the naming list; "FR-12 Bank import: import copies rows, never references"
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation-Patterns-&-Consistency-Rules] — naming, wire format, error envelope, routes, validation timing, logging
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#Game-builder-&-Question-Bank] — A13 package cards (title, count, preview), "מהמאגר" badge, mixing rule, both empty states
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/DESIGN.md] — host-slate palette, modal elevation rule, radius/spacing scale
- [Source: _bmad-output/implementation-artifacts/1-4-configure-scoring-rules.md] — review-pinned handler order, invalidation discipline, Windows/encoding notes, established store/test patterns
- [Source: _bmad-output/implementation-artifacts/1-3-create-a-game-and-author-questions.md] — questions schema shape, `INSERT … SELECT` ownership pattern, sqlc NOT-NULL constraint, dialog conventions
- [Source: _bmad-output/implementation-artifacts/deferred-work.md] — MAX+1 position concurrency class (import inherits the same posture), draft TOCTOU posture

## Dev Agent Record

### Agent Model Used

claude-fable-5 (Claude Fable 5, dev-story workflow)

### Debug Log References

- TDD red phase confirmed: 11 new packages handler tests written first, failed 404/405 (routes absent) before implementation, all green after.
- Full Go suite green: 67 httpapi tests (56 pre-existing unchanged in behavior + 11 new: list packages payload shape incl. no-timestamps, empty bank `[]`, both routes 401 without session, import happy path with position order + `importedFromBank` propagation + store params, unknown package 404 `PACKAGE_NOT_FOUND`, malformed/missing packageId 400 naming the field without store call, non-draft 409 with valid AND malformed bodies (draft check precedes validation — the 1.4 review regression class), foreign game 404, malformed gameID 404 short-circuit, invalid JSON 400, oversized 413) + 4 store tests; `gofmt`/`go vet` clean; `sqlc generate` idempotent (gen/ regenerated: `Question.ImportedFromBank bool`, `ListQuestionPackagesRow{QuestionCount int64, Preview string}`, `QuestionPackage`/`PackageQuestion` models — no pgtype leak, all new columns NOT NULL).
- Frontend gates green: `eslint`, `tsc -b --noEmit`, `npm run build`; filter-safety greps clean on source **and** built bundle.
- API E2E (35/35 checks, Python urllib vs built binary + Docker Postgres 17): bank browse returns the seeded pack with Hebrew title / count 10 / first-question preview / no timestamps → import into a fresh game copies 10 questions at positions 1..10 with full content fidelity (mcq options/correct, free_text accepted answers, per-question time limits) and `importedFromBank: true` → custom question appends at 11 unbadged → re-import appends a second copy at 12..21 (documented design) → edit keeps the badge → delete → reorder mixes custom between imported and persists → malformed/missing packageId 400 naming the field, unknown 404 `PACKAGE_NOT_FOUND`, foreign game 404 `GAME_NOT_FOUND`, no session 401 → psql `state='lobby'` → 409 `GAME_NOT_EDITABLE` with valid AND garbage bodies → reset → list `questionCount` includes imports (20). psql after edits/deletes: `package_questions` still 10 rows, source text unchanged (AC-3 copy semantics). Zero ERROR log lines (NFR-8); INFO `package imported` with `game_id`/`package_id`/`question_count`.
- Visual E2E (18/18 checks, headless Edge via playwright-core, scratchpad install, zero project deps): login → create game → dual-CTA empty state with the AC-4 prompt, both buttons real actions (add opens the question editor) → author a custom question → bank dialog shows the A13 card (title + "10 שאלות" + preview) → import → 11 rows, custom stays first, copies follow in package order, exactly 10 "מהמאגר" pills, custom unbadged → edit an imported copy keeps its badge → delete a copy → move custom between imported → reload: mixed order + badges persist → `dir=rtl`, zero console errors. Screenshots reviewed: modal elevated, Level-1 cards inside (border, no shadow), h-10 buttons, quiet badge pill beside the type badge, bidi-isolated JOIN Code intact.
- Migration check: `00005` applied at boot on the existing dev DB (schema + 10 seeded rows + Hebrew verified via `\d`/SELECT through psql) and on a fresh scratch DB (5 migrations, seed present); `goose down` drops exactly the two tables + `imported_from_bank` column and re-`up` re-seeds (run via a temporary in-module `cmd/goose-e2e` helper using the same embedded FS the server boots with — deleted after the pass). Migration file verified UTF-8 without BOM.

### Completion Notes List

- FR-12 complete: `question_packages` + `package_questions` (content columns mirror `questions` verbatim, `package_questions_type_shape` CHECK), `imported_from_bank BOOLEAN NOT NULL DEFAULT false` on `questions`, a seeded 10-question sample pack (fixed UUID `11111111-1111-4111-8111-111111111111`, mixed MCQ/free-text, mixed difficulty across generations incl. Torah-knowledge "Saba wins" questions, 20–45s limits, gender-neutral plural-imperative phrasing), `GET /api/question-packages`, `POST /api/games/{gameID}/questions/import-package` (one atomic `INSERT … SELECT` with `row_number()` append positions MAX+1..MAX+N), the editor's import dialog with A13 package cards, the "מהמאגר" provenance badge, and both AC-4 empty states.
- Implementation matches the story's locked design decisions with no deviations: copies are physical `questions` rows (AC-3 by construction — mixing/reorder/edit/delete needed zero new code), one provenance bit instead of a source FK, body-ID 400 vs path-param 404 split, re-import appends another copy, draft-only via existing `requireDraftGame`, no standalone bank page, seed in the migration.
- Handler order is the review-pinned sequence: `requireOrganizer` → `gameIDParam` → `decodeJSON` → `requireDraftGame` → `isUUID(packageId)` → store — the precedence tests (409/404 before body validation) were written up front in the red phase.
- Store wrapper sorts the `RETURNING *` rows by `Position` before returning (row order is not guaranteed); `GetQuestionPackage` runs first so a missing package is `ErrNotFound` (404 `PACKAGE_NOT_FOUND`) distinct from an existing-but-empty package (201 with empty items). No explicit transaction — both statements are single atomic queries; the MAX+1 concurrency class remains the documented deferred item shared with 1.3's `CreateQuestion`.
- Seeding does NOT close OQ-4 — the sample pack is genuinely playable but the real pilot pack remains a content task.
- E2E note (test tooling, not app behavior): the `wc_session` cookie is `Secure`, which Python's cookiejar refuses to send over `http://localhost` (browsers and curl exempt localhost as a secure context) — the E2E script manages the cookie header manually. All E2E data (2 games + `e2e-org-15`/`e2e-org-15b` organizers) deleted from the dev DB afterwards; the scratch DB dropped; `webdist/dist` placeholder restored.
- Zero new dependencies (Go and npm); zero new shadcn components; all Hebrew via `strings.he.ts`; no `game` package created; `cmd/server/main.go`, `middleware.go`, `app.tsx` untouched.

### File List

New:
- server/migrations/00005_question_packages.sql
- server/internal/store/queries/packages.sql
- server/internal/store/packages.go
- server/internal/store/gen/packages.sql.go (generated)
- server/internal/httpapi/packages.go
- server/internal/httpapi/packages_test.go
- web/src/features/builder/package-import-dialog.tsx

Modified:
- server/internal/store/gen/models.go (generated)
- server/internal/store/gen/questions.sql.go (generated)
- server/internal/httpapi/games.go
- server/internal/httpapi/games_test.go
- server/internal/httpapi/questions.go
- server/internal/httpapi/router.go
- web/src/features/builder/game-editor-page.tsx
- web/src/lib/types.ts
- web/src/lib/strings.he.ts
- _bmad-output/implementation-artifacts/sprint-status.yaml

## Change Log

- 2026-07-16: Story created by create-story workflow — ultimate context engine analysis completed (epics, PRD FR-12/Glossary/OQ-4, architecture structure + patterns, EXPERIENCE.md bank/A13/empty states, 1.3+1.4 story and review intelligence, live codebase read: migrations 00003/00004, store games/questions + queries, httpapi games/questions/errors/router, game-editor-page, types.ts, strings.he.ts, sqlc.yaml). Status: ready-for-dev.
- 2026-07-16: Story implemented by dev-story workflow (claude-fable-5) — migration 00005 (two bank tables + provenance column + seeded 10-question sample pack), store packages queries + atomic import copy, GET /api/question-packages + POST .../questions/import-package with 11 new handler tests (67 httpapi + 4 store green), import dialog + "מהמאגר" badge + dual-CTA/empty-bank states; all gates green; API E2E 35/35 + visual E2E 18/18 vs built binary + Docker Postgres; migration verified on existing + fresh DB with down/up round-trip. Status: review.
- 2026-07-16: Code review completed (bmad-code-review, 3 adversarial layers) — no AC violations, no High/Critical defects; 2 patch findings left as action items (stale store doc comment, dead PACKAGE_NOT_FOUND mapping in list handler), 4 deferred to deferred-work.md, 6 dismissed. Status: in-progress until patch items are resolved.

## Review Findings

### Code review (2026-07-16) — full adversarial review (Blind Hunter + Edge Case Hunter + Acceptance Auditor)

Outcome: **no acceptance-criteria violations, no High/Critical defects.** The Acceptance Auditor confirmed all four ACs met and that `package_questions` CHECK constraints are byte-identical to `questions` (so the `INSERT … SELECT` copy can never violate a target CHECK). Triage: 2 patch, 4 deferred, 6 dismissed as noise/by-design.

**Patch (unchecked — address before marking done):**

- [ ] [Review][Patch] Inaccurate store doc comment — comment claims a missing/foreign game surfaces "as ErrNotFound to the caller via zero rows", but the code returns `([], nil)` for that case; ownership safety actually rests entirely on the handler's `requireDraftGame`. Fix the comment to describe real behavior. [server/internal/store/packages.go:22-24]
- [ ] [Review][Patch] Dead/misleading `PACKAGE_NOT_FOUND` in the list handler — `ListQuestionPackages` is a plain `:many` pass-through that never returns `store.ErrNotFound`, so the not-found code is unreachable and a real DB failure falls through to the 503 default. Drop or correct the argument. [server/internal/httpapi/packages.go:32]

**Deferred (accepted / out of scope — logged to deferred-work.md):**

- [x] [Review][Defer] Concurrent-import position race — `ImportPackageQuestions` appends via `MAX(position)+row_number()` with no txn and no `UNIQUE(game_id, position)`; two overlapping imports (or import + CreateQuestion) can assign duplicate positions (self-healing via reorder). Same class as the 1.3 `CreateQuestion` posture already tracked; import inherits it. [server/internal/store/queries/packages.sql] — deferred, pre-existing class
- [x] [Review][Defer] Import draft-state TOCTOU — the `INSERT … SELECT` WHERE scopes only on `id`/`organizer_id`, so a draft→lobby transition or a delete landing between `requireDraftGame` and the write is not observed. Same class as the 1.4 draft-only TOCTOU already tracked; revisit with game-start (2.3). [server/internal/httpapi/packages.go:72-81] — deferred, pre-existing class
- [x] [Review][Defer] Import-dialog UX polish — one shared `isPending` disables every card's import button (no per-card affordance), and the dialog stays dismissable mid-import (mutation still resolves → invalidate + onClose after unmount). Harmless today. [web/src/features/builder/package-import-dialog.tsx] — deferred
- [x] [Review][Defer] Hebrew dual-form count — `questionBank.questionsCount` renders `${count} שאלות` for all n≠1, including n=2 (grammatical dual "שתי שאלות"). i18n polish, no path impact. [web/src/lib/strings.he.ts] — deferred

**Dismissed (6):** CHECK-constraint mismatch (false positive — auditor confirmed byte-identical CHECKs); existing-but-empty package → 201 empty (documented design, unreachable — packages enter only via migrations); re-import duplicates questions + no per-game cap (locked design decision "documented, not guarded" + cap out of scope); `correctOption,omitempty` (pre-existing and safe under the 1-based `CHECK correct_option BETWEEN 1 AND 4`); list correlated-subquery preview (spec-dictated, correct, negligible at pilot scale); request body read outside the 5s deadline (bounded by the 64 KiB MaxBytesReader, consistent with existing handlers).
