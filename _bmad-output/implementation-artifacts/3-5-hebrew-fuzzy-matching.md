---
baseline_commit: 8d41b4bd9017cf1c16b29602e3cfe79698f42e8a
---

# Story 3.5: Hebrew Fuzzy Matching

Status: ready-for-dev

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a Participant,
I want my answer accepted despite a typo or spelling variant,
so that spelling doesn't cost me points (FR-16 fuzzy stage).

## ⚠️ Prerequisite: Story 3.4 is not finished

At the time this story was created, **Story 3.4 (Grading Pipeline — MCQ and Exact Match) is fully implemented on disk but uncommitted** — its own story doc status is `review`, sprint-status.yaml still shows `in-progress`. Verified by reading the actual files (not 3.4's story doc): `server/internal/grading/pipeline.go` (`StageMCQ`, `StageExact`, `GradeMCQ`, `GradeExact`), `server/internal/game/answers.go` (`RecordAnswer` grades MCQ/Free-Text inline before persisting), `server/internal/game/engine.go` (`ErrGradingIncomplete`, `Reveal`'s grading-completeness pre-check), `server/migrations/00011_answer_grading.sql` (`answers.is_correct`/`answers.stage`, CHECK `stage IN ('mcq','exact')`), `server/internal/store/queries/answers.sql` + `games.sql` (grading columns threaded through `RecordAnswer`/`GetOpenQuestionForPlayer`, `RevealCurrentQuestion`'s grading gate), and matching test coverage in `game/answers_test.go` + `game/engine_test.go` + `httpapi/control_test.go`. This story's tasks below describe that *current, already-on-disk* shape as the starting point.

**Re-verify these files still match before writing code** — if 3.4 has since been committed (or further changed) by the time this story starts, confirm the shapes below are still accurate.

## Acceptance Criteria

1. **Given** a Free-Text answer that misses the Exact stage, **when** `RecordAnswer` runs, **then** the Fuzzy stage runs next in the same call: it normalizes both the response and every one of the question's `accepted_answers` (trim, final-letter forms ך/ם/ן/ף/ץ → כ/מ/נ/פ/צ, nikud stripping, punctuation), and a match records `stage = 'fuzzy'`, `is_correct = true`. An Exact match never invokes the Fuzzy stage (short-circuit — epic AC-1, FR-16). *(epic AC-1)*
2. **Given** the UJ-3 cases, **then**: `"צרורה "` (trailing space) matches Accepted Answer `"צרורה"`; a response with a single-letter typo relative to an Accepted Answer matches within the Levenshtein threshold; an unrelated word does not match. *(epic AC-2)*
3. **Given** a Free-Text response that misses both Exact and Fuzzy, **when** grading completes, **then** it is graded `is_correct = false` at `stage = 'fuzzy'` — the last stage that ran — never left ungraded (`stage IS NULL`); this extends story 3.4's "graded means the currently-implemented pipeline ran to completion" design decision now that the pipeline has two stages instead of one. *(consequence of epic AC-1, continuity with 3.4 Dev Notes)*
4. **Given** the implementation, **then** it is pure Go: `server/internal/grading/normalize.go` (Hebrew normalization) and `server/internal/grading/fuzzy.go` (Levenshtein distance + threshold matching), with unit tests covering the Hebrew normalization cases (final letters, nikud, punctuation, whitespace) and the Levenshtein/threshold matching cases. *(epic AC-3)*

## Tasks / Subtasks

- [ ] **Task 1: Migration — widen the `stage` CHECK to admit `'fuzzy'`** (AC: 1, 3)
  - [ ] **First, confirm the actual constraint name on the dev DB** — migration 00011 added `stage TEXT CHECK (stage IN ('mcq','exact'))` inline via `ALTER TABLE ... ADD COLUMN`; Postgres's default naming for that shape is `<table>_<column>_check` (i.e. `answers_stage_check`), but this must be confirmed, not assumed (`\d answers` or `SELECT conname FROM pg_constraint WHERE conrelid = 'answers'::regclass` against the dev DB with migration 00011 applied) — same "verify, don't assume" discipline 3.4's Dev Notes applied to `main.go`. Adjust the constraint name below if it differs.
  - [ ] New `server/migrations/00012_answer_grading_fuzzy_stage.sql`:
    ```sql
    -- +goose Up
    -- Widens story 3.4's shape CHECK (answers.stage IN ('mcq','exact')) to
    -- admit 'fuzzy' — this story's Hebrew fuzzy-matching stage. Story 3.6
    -- (AI) widens it again to add 'ai'. is_correct/stage's shape-pairing
    -- CHECK (answers_grading_shape, added in 00011) is untouched — this
    -- migration only widens the allowed stage values.
    ALTER TABLE answers DROP CONSTRAINT answers_stage_check;
    ALTER TABLE answers ADD CONSTRAINT answers_stage_check CHECK (stage IN ('mcq', 'exact', 'fuzzy'));

    -- +goose Down
    ALTER TABLE answers DROP CONSTRAINT answers_stage_check;
    ALTER TABLE answers ADD CONSTRAINT answers_stage_check CHECK (stage IN ('mcq', 'exact'));
    ```
  - [ ] No `sqlc generate` diff is expected from this migration — `stage` stays `TEXT`/`pgtype.Text` in the generated Go; only the DB-side allowed-values list changes, and sqlc's codegen never inspects CHECK constraint bodies. Confirm the diff is empty at Task 5 (unlike story 3.4, which did expect a diff).

- [ ] **Task 2: New `grading` package files — Hebrew normalization + Levenshtein/threshold matching** (AC: 1, 2, 4)
  - [ ] New `server/internal/grading/normalize.go`:
    ```go
    package grading

    import (
        "strings"
        "unicode"
    )

    // finalLetterForms maps each Hebrew final letter to its medial form —
    // ך→כ, ם→מ, ן→נ, ף→פ, ץ→צ (architecture: "final-letter forms ... nikud
    // stripping, punctuation"). A response typed with the wrong form
    // (WhatsApp autocorrect, or a participant unfamiliar with when finals
    // apply) must not cost a Fuzzy match.
    var finalLetterForms = map[rune]rune{
        'ך': 'כ',
        'ם': 'מ',
        'ן': 'נ',
        'ף': 'פ',
        'ץ': 'צ',
    }

    // isNikud reports whether r is a Hebrew niqqud/cantillation mark
    // (Unicode Hebrew "points" sub-block, U+0591-U+05C7) — stripped so
    // "שָׁלוֹם" and "שלום" normalize identically. Hebrew punctuation (geresh
    // U+05F3, gershayim U+05F4) falls outside this range and is instead
    // caught by unicode.IsPunct below.
    func isNikud(r rune) bool {
        return r >= 0x0591 && r <= 0x05C7
    }

    // Normalize prepares s for Fuzzy-stage comparison (FR-16): strips
    // niqqud and punctuation, maps final Hebrew letters to their medial
    // form, and collapses/trims whitespace to single spaces.
    // Self-contained — does not assume the caller already trimmed, so it
    // can be unit-tested and called directly with raw input (epic AC-2's
    // "צרורה " case exercises this independently of game.RecordAnswer's
    // own trim). Applied independently to both the participant's response
    // and each Accepted Answer (GradeFuzzy).
    func Normalize(s string) string {
        var b strings.Builder
        b.Grow(len(s))
        for _, r := range s {
            if isNikud(r) || unicode.IsPunct(r) {
                continue
            }
            if mapped, ok := finalLetterForms[r]; ok {
                r = mapped
            }
            b.WriteRune(r)
        }
        return strings.Join(strings.Fields(b.String()), " ")
    }
    ```
  - [ ] New `server/internal/grading/normalize_test.go`: table-driven `TestNormalize` covering — already-normalized string unchanged; surrounding whitespace trimmed; internal whitespace collapsed (proves punctuation removal can't leave a double space, e.g. a comma with spaces on both sides); each of the five final-letter forms individually; nikud stripped (a vocalized word); punctuation stripped (exclamation mark, comma, geresh `׳`, gershayim `״`); empty string; punctuation-only string (result is empty, not a stray space).
  - [ ] New `server/internal/grading/fuzzy.go`:
    ```go
    package grading

    import "unicode/utf8"

    // Levenshtein returns the edit distance (insertions, deletions,
    // substitutions) between a and b, counted in runes (Hebrew text) —
    // never bytes. Two-row dynamic-programming implementation, O(len(a)*
    // len(b)) time, O(min(len(a),len(b))) space — pure Go, no external
    // dependency (architecture: "pure Go, no external service"; this
    // codebase has no Levenshtein library in go.mod and none is added).
    func Levenshtein(a, b string) int {
        ra, rb := []rune(a), []rune(b)
        if len(ra) > len(rb) {
            ra, rb = rb, ra
        }
        prev := make([]int, len(ra)+1)
        curr := make([]int, len(ra)+1)
        for i := range prev {
            prev[i] = i
        }
        for i := 1; i <= len(rb); i++ {
            curr[0] = i
            for j := 1; j <= len(ra); j++ {
                cost := 1
                if rb[i-1] == ra[j-1] {
                    cost = 0
                }
                curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
            }
            prev, curr = curr, prev
        }
        return prev[len(ra)]
    }

    func min3(a, b, c int) int {
        m := a
        if b < m {
            m = b
        }
        if c < m {
            m = c
        }
        return m
    }

    // levenshteinThreshold returns the maximum edit distance still
    // considered a Fuzzy match for an accepted answer of the given
    // (normalized, rune) length — short answers tolerate one typo, longer
    // answers tolerate proportionally more. [ASSUMPTION — no specific
    // threshold is set in the PRD/architecture; this is a starting point
    // for the pilot. Revisit if Organizer-reported wrong grades (NFR-9,
    // both directions — too lenient or too strict) show it needs tuning;
    // NFR-9 also forbids tuning the *AI* stage toward leniency, but says
    // nothing about Fuzzy, so this threshold is fair game to adjust.]
    func levenshteinThreshold(length int) int {
        switch {
        case length <= 4:
            return 1
        case length <= 10:
            return 2
        default:
            return 3
        }
    }

    // GradeFuzzy reports whether response fuzzily matches any of
    // acceptedAnswers: both sides are normalized (Normalize) and compared
    // by Levenshtein distance against a length-scaled threshold
    // (levenshteinThreshold, keyed on the normalized accepted answer's
    // rune length). Called only after GradeExact misses (game.RecordAnswer)
    // — an Exact hit never reaches here, but GradeFuzzy does not itself
    // depend on that ordering (it renormalizes and would report a match
    // for an Exact hit too).
    func GradeFuzzy(response string, acceptedAnswers []string) bool {
        normResponse := Normalize(response)
        for _, accepted := range acceptedAnswers {
            normAccepted := Normalize(accepted)
            if Levenshtein(normResponse, normAccepted) <= levenshteinThreshold(utf8.RuneCountInString(normAccepted)) {
                return true
            }
        }
        return false
    }
    ```
  - [ ] New `server/internal/grading/fuzzy_test.go`: table-driven tests —
    - `TestLevenshtein`: identical strings (0); both empty (0); empty vs. non-empty (= length of the non-empty side); single substitution (1); single insertion (1); single deletion (1); completely different same-length strings (= length).
    - `TestGradeFuzzy`, covering epic AC-2's UJ-3 cases directly (call `GradeFuzzy` with un-trimmed/un-normalized input, not via `game.RecordAnswer`):
      - trailing-space case: `GradeFuzzy("צרורה ", []string{"צרורה"})` → `true`.
      - one-letter-typo case: an Accepted Answer with a single substituted letter in the response → `true` (e.g. `"ירושלים"` accepted, response `"ירוסלים"` — ש→ס).
      - unrelated-word case: `GradeFuzzy("תל אביב", []string{"ירושלים"})` → `false`.
      - normalization actually engages: `GradeFuzzy("שלום!", []string{"שלום"})` → `true` (punctuation stripped on the response side).
      - multiple accepted answers: matches the second of two when the first doesn't.
      - threshold boundary: a response whose edit distance is *one more* than `levenshteinThreshold` for that accepted answer's length → `false` (proves the threshold is enforced, not just "roughly close" — e.g. a 4-letter accepted answer, threshold 1, with a 2-edit-distance response).

- [ ] **Task 3: `grading/pipeline.go` — add `StageFuzzy`** (AC: 1, 3)
  - [ ] Add the constant and update the package/const-block comments (both currently reference this story by name as future work):
    ```go
    const (
        StageMCQ   Stage = "mcq"
        StageExact Stage = "exact"
        StageFuzzy Stage = "fuzzy"
        // StageAI joins this list in Story 3.6.
    )
    ```
    Update the package doc comment's `"(Fuzzy lands in Story 3.5, AI in Story 3.6; this story is Exact only)"` to reflect that Fuzzy has now landed (AI remains Story 3.6) — keep the sentence, just drop the now-stale "this story is Exact only" framing.

- [ ] **Task 4: `game/answers.go` — run Fuzzy on an Exact miss** (AC: 1, 2, 3)
  - [ ] Extend the `free_text` case in `RecordAnswer`'s type switch (currently a single `grading.GradeExact` call) into a three-way switch so a miss falls through to Fuzzy and a total miss is still graded (not left pending):
    ```go
    case "free_text":
        trimmed := strings.TrimSpace(rawText)
        if utf8.RuneCountInString(trimmed) > maxFreeTextAnswerLength {
            return AnswerResult{Outcome: AnswerTooLong}, nil
        }
        response = trimmed
        switch {
        case grading.GradeExact(response, qc.AcceptedAnswers):
            isCorrect = true
            stage = grading.StageExact
        case grading.GradeFuzzy(response, qc.AcceptedAnswers):
            isCorrect = true
            stage = grading.StageFuzzy
        default:
            isCorrect = false
            stage = grading.StageFuzzy
        }
    ```
    The `case grading.GradeExact(...)` branch's short-circuit is what satisfies AC-1's "an Exact match never invokes the Fuzzy stage" — Go's `switch true` evaluates cases top-to-bottom and stops at the first match, so `GradeFuzzy` is never called once `GradeExact` returns `true`. No other part of `RecordAnswer` changes (the `mcq` case, the outcome-mapping `switch` below the store call, `AnswerResult`/`AnswerOutcome` — all untouched, same as 3.4's scope note).
  - [ ] `server/internal/game/answers_test.go`: update `TestRecordAnswerFreeTextNoMatchSetsIsCorrectFalse` — it currently asserts `Stage == grading.StageExact` for response `"תל אביב"` against accepted `["ירושלים"]`; that response also misses Fuzzy (unrelated word, AC-2), so the assertion becomes `Stage == grading.StageFuzzy` (the pipeline now runs one stage further before giving up). Update the doc comment to explain it's graded across Exact **and** Fuzzy now, still not left ungraded. `TestRecordAnswerFreeTextExactMatchSetsIsCorrectTrue` needs no change — an Exact hit still records `stage = exact` (proves the short-circuit).
    Add:
    - `TestRecordAnswerFreeTextFuzzyMatchSetsIsCorrectTrue` — a one-letter-typo response against a single accepted answer; assert `IsCorrect == true` and `Stage == grading.StageFuzzy`.

- [ ] **Task 5: Quality gates + local E2E** (all ACs)
  - [ ] Local gates: `gofmt -l .` · `go1.26.5 vet ./...` · `go1.26.5 test ./...` (zero regressions) · `sqlc generate` diff — expected **empty** (Task 1 note; unlike story 3.4). No `web/` changes in this story — skip `npm run lint`/`tsc -b` only if genuinely nothing under `web/` changed.
  - [ ] Local Go E2E (`cmd/e2escratch`, deleted after use, same pattern as 2.1–3.4): create a scratch game with one Free-Text question (accepted answers `["ירושלים"]`). Open lobby, join 2 players (A, B). `POST /start`. Drive via signed inbound webhook `POST`s:
    - A replies `"ירוסלים"` (one-letter typo, ש→ס) — after the send, query the scratch DB directly and assert `is_correct = true, stage = 'fuzzy'`.
    - B replies `"תל אביב"` (unrelated word) — assert `is_correct = false, stage = 'fuzzy'` (graded, not left ungraded — proves the widened CHECK constraint from Task 1 actually applies and the migration ran cleanly).
    - `POST /close-question` → `POST /reveal`: must succeed (200) — both recorded answers are graded (this also confirms the migration's CHECK-constraint rename didn't break `RevealCurrentQuestion`'s `NOT EXISTS (... stage IS NULL)` guard from story 3.4).
    - Clean up the scratch game row afterward; delete the harness afterward — same convention as every prior story.

## Dev Notes

### Architecture guardrails (violations = rework)

- **Dependency direction unaffected**: `normalize.go`/`fuzzy.go` join `pipeline.go` in `server/internal/grading/` — still pure functions, zero store/DB/context dependency, matching the package's existing "first package in this codebase with zero I/O in its own tests" posture (3.4 Dev Notes). `game` continues to be the only importer.
- **Persist-before-ack (SM-4) unaffected**: exactly as in 3.4 — the Fuzzy verdict is computed before the single `store.RecordAnswer` INSERT in `RecordAnswer`, so there is no new "persist then separately grade" window.
- **Grade never disclosed before Reveal (FR-15/FR-16, non-negotiable)**: unchanged from 3.4 — `AnswerResult`/`AnswerOutcome` gain no correctness field; `Snapshot`/`CurrentQuestion` are untouched by this story.
- **Glossary**: no new package; `normalize.go`/`fuzzy.go` are the two files architecture's Project Structure section names verbatim for this exact purpose.
- **Logging (NFR-8)**: same restraint as 3.4 — `Levenshtein`/`GradeFuzzy`/`Normalize` are pure, no I/O, nothing to log; do not add per-answer logging.
- **NFR-2 (never block the game loop)**: `Levenshtein` is O(len(a)·len(b)) on short strings (Hebrew words/short phrases, ≤200 runes per FR-5's cap) — microseconds, no blocking risk on the synchronous webhook-request path. This posture changes at Story 3.6 (AI, network I/O), not here.
- **NFR-9 (AI leniency counter-metric) does not apply here** — it specifically targets the *AI Semantic* stage (Story 3.6). The Fuzzy threshold is tunable; flag any threshold change in code review as a deliberate, documented adjustment (not silent drift), matching the audit-trail spirit even though NFR-9's letter is AI-specific.

### Existing code this story modifies — current state, and what must survive

- **[server/internal/grading/pipeline.go](server/internal/grading/pipeline.go)** — as of this story's baseline (3.4 fully on disk): `Stage` type alias, `StageMCQ`/`StageExact` consts, `GradeMCQ`/`GradeExact` pure functions. This story only adds `StageFuzzy` and touches two comments — `GradeMCQ`/`GradeExact` themselves are untouched (still Exact-only, no normalization of their own — that remains correct; normalization is Fuzzy-only per architecture).
- **[server/internal/game/answers.go](server/internal/game/answers.go)** — `RecordAnswer`'s `free_text` case currently calls `grading.GradeExact` once and stops (`stage = grading.StageExact` unconditionally, whether or not it matched — 3.4's "exact miss is graded false, not pending" decision). This story replaces that single call with the three-way `switch true` in Task 4 — the `mcq` case, the `default` (unrecognized type) branch, and everything below the type switch (the `store.RecordAnswer` call, the outcome-mapping `switch`) are untouched.
- **[server/migrations/00011_answer_grading.sql](server/migrations/00011_answer_grading.sql)** — added `is_correct`/`stage` with `stage TEXT CHECK (stage IN ('mcq','exact'))` and the `answers_grading_shape` pairing CHECK. This story's migration widens only the stage-values CHECK (Task 1); `answers_grading_shape` is untouched — a Fuzzy verdict is still always inserted with both columns set together, same as MCQ/Exact.
- **[server/internal/game/answers_test.go](server/internal/game/answers_test.go)** — already has a `"--- Grading (story 3.4) ---"` section with `TestRecordAnswerFreeTextExactMatchSetsIsCorrectTrue`/`TestRecordAnswerFreeTextNoMatchSetsIsCorrectFalse`; the latter's expected `Stage` changes per Task 4 (its behavior is genuinely different now — the pipeline runs one stage further). Add the new Fuzzy-match test into the same section (rename the section comment to `"--- Grading (stories 3.4-3.5) ---"` if convenient, not required).

### Design decisions worth flagging explicitly

- **What "graded" means, extended**: story 3.4 established "graded" as "the currently-implemented pipeline ran to completion for this answer, whatever that pipeline currently contains" (its Dev Notes) rather than leaving a miss `NULL`/pending. This story is the first to actually exercise that principle across two stages: a response missing both Exact and Fuzzy is graded `is_correct = false` at `stage = 'fuzzy'` (AC-3) — the last stage attempted, not the first. When Story 3.6 adds AI, a total miss will similarly record `stage = 'ai'`. Do not special-case "which stage recorded the miss" beyond "whichever ran last" — that is the established, intentional pattern, not an oversight.
- **Levenshtein threshold is an explicit [ASSUMPTION]**: neither the PRD nor architecture.md specifies a numeric threshold — only "Levenshtein distance threshold" as a mechanism. `levenshteinThreshold` (Task 2) is a length-scaled formula (1 for ≤4 runes, 2 for ≤10, 3 beyond) chosen to satisfy epic AC-2's three UJ-3 cases (trailing-space/typo match, unrelated-word miss) without over- or under-matching at the word lengths a Hebrew Free-Text Accepted Answer is likely to have. This is the same category of documented assumption as story 1.4's scoring defaults (`[ASSUMPTION — defaults not fixed in PRD]`) — flag it in code review; NFR-9's audit trail (`stage` recorded per answer) is exactly the mechanism that would surface a wrong-calibration problem in production.
- **Whole-string comparison, not per-word**: `GradeFuzzy` computes one Levenshtein distance over the full normalized string (response vs. each whole Accepted Answer), the same granularity `GradeExact` already uses — not a per-word or token-level fuzzy match. Simpler, consistent with Exact's existing scope, and sufficient for the UJ-3 cases; a per-word approach is not required by any AC and would be scope creep.

### Why no web/`strings.he.ts`/`messages_he.go` changes

Identical reasoning to 3.4: every AC here is server-internal (grading computation only). The participant's ack is already grade-blind (`"התקבל ✓"` regardless of correctness, since story 3.3); a Fuzzy match changes only which row lands in `answers`, never anything a Participant or Organizer observes before Reveal (Story 3.8 is where grade disclosure happens).

### Testing standards

Go stdlib `testing`, co-located `_test.go`, stubs grown in place (`stubStore` — unchanged by this story; no new `Store` interface method is needed since Fuzzy grading happens entirely inside `game.RecordAnswer` using data already fetched via `GetOpenQuestionForPlayer`). `grading`'s tests remain pure unit tests with zero I/O. No real-DB unit tests for the migration itself — Task 5's `cmd/e2escratch` harness is the verification that the widened CHECK constraint round-trips correctly through goose.

### Project Structure Notes

**New:**
- `server/migrations/00012_answer_grading_fuzzy_stage.sql`
- `server/internal/grading/normalize.go`
- `server/internal/grading/normalize_test.go`
- `server/internal/grading/fuzzy.go`
- `server/internal/grading/fuzzy_test.go`

**Modified:**
- `server/internal/grading/pipeline.go` (+`StageFuzzy` const, two comment updates)
- `server/internal/game/answers.go` (`RecordAnswer`'s `free_text` case gains the Fuzzy fallback)
- `server/internal/game/answers_test.go` (one existing assertion updated, one new test added)

**Untouched:** `server/internal/game/engine.go` (no new `Store` method, no new sentinel error — Fuzzy grading is entirely inside `RecordAnswer`, unlike 3.4's `Reveal` gate work) · `server/internal/store/*` (no query changes — `RecordAnswerParams`'s `IsCorrect`/`Stage` fields already accept any string; the DB-side CHECK is the only thing widening) · `server/internal/httpapi/*` (no new domain error, no new endpoint behavior) · `server/internal/game/snapshot.go` (no grade data added to the wire) · `server/internal/wa/*` · `web/*`.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-3.5] — story + all 3 epic ACs verbatim, UJ-3 cases, Epic 3 context, FR-16 fuzzy-stage scope
- [Source: _bmad-output/planning-artifacts/architecture.md#Data-Architecture] — "Hebrew fuzzy stage (FR-16): normalization (trim, final-letter forms ך/ם/ן/ף/ץ → כ/מ/נ/פ/צ, nikud stripping, punctuation) + Levenshtein distance threshold — pure Go, no external service"
- [Source: _bmad-output/planning-artifacts/architecture.md#Project-Structure] — `grading/normalize.go` ("Hebrew normalization (finals, nikud, punctuation)") and `grading/fuzzy.go` ("Levenshtein w/ threshold") as the two planned files this story delivers
- [Source: server/internal/grading/pipeline.go, pipeline_test.go] — current `Stage`/`GradeMCQ`/`GradeExact` shape (story 3.4, on disk) this story extends; the const block's own comment already naming this story as where `StageFuzzy` arrives
- [Source: server/internal/game/answers.go] — current `RecordAnswer` free_text branch (single `GradeExact` call) this story extends into the three-way switch
- [Source: server/migrations/00011_answer_grading.sql] — current `stage` CHECK (`'mcq','exact'`) and `answers_grading_shape` pairing constraint this story's migration widens (values only)
- [Source: server/internal/game/answers_test.go] — `answerStub` helper and the existing `"--- Grading (story 3.4) ---"` test section this story extends/updates
- [Source: _bmad-output/implementation-artifacts/3-4-grading-pipeline-mcq-and-exact-match.md] — previous story's Dev Notes, specifically "A design decision worth flagging explicitly: what happens to a Free-Text miss," which this story directly continues ("most likely, the whole synchronous pipeline (Exact → Fuzzy) simply runs to completion inside one RecordAnswer call")
- [Source: _bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/reconcile-brief.md#M-2] — note that Fuzzy/AI stages handling Hebrew-specific variance (final-letter forms, nikud) was a brief-to-PRD gap this story's normalization closes functionally

## Dev Agent Record

### Agent Model Used

### Debug Log References

### Completion Notes List

### File List
