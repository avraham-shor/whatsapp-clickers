---
baseline_commit: 8d41b4bd9017cf1c16b29602e3cfe79698f42e8a
---

# Story 3.5: Hebrew Fuzzy Matching

Status: done

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

- [x] **Task 1: Migration — widen the `stage` CHECK to admit `'fuzzy'`** (AC: 1, 3)
  - [x] **First, confirm the actual constraint name on the dev DB** — migration 00011 added `stage TEXT CHECK (stage IN ('mcq','exact'))` inline via `ALTER TABLE ... ADD COLUMN`; Postgres's default naming for that shape is `<table>_<column>_check` (i.e. `answers_stage_check`), but this must be confirmed, not assumed (`\d answers` or `SELECT conname FROM pg_constraint WHERE conrelid = 'answers'::regclass` against the dev DB with migration 00011 applied) — same "verify, don't assume" discipline 3.4's Dev Notes applied to `main.go`. Adjust the constraint name below if it differs.
  - [x] New `server/migrations/00012_answer_grading_fuzzy_stage.sql`:
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
  - [x] No `sqlc generate` diff is expected from this migration — `stage` stays `TEXT`/`pgtype.Text` in the generated Go; only the DB-side allowed-values list changes, and sqlc's codegen never inspects CHECK constraint bodies. Confirm the diff is empty at Task 5 (unlike story 3.4, which did expect a diff).

- [x] **Task 2: New `grading` package files — Hebrew normalization + Levenshtein/threshold matching** (AC: 1, 2, 4)
  - [x] New `server/internal/grading/normalize.go`:
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
  - [x] New `server/internal/grading/normalize_test.go`: table-driven `TestNormalize` covering — already-normalized string unchanged; surrounding whitespace trimmed; internal whitespace collapsed (proves punctuation removal can't leave a double space, e.g. a comma with spaces on both sides); each of the five final-letter forms individually; nikud stripped (a vocalized word); punctuation stripped (exclamation mark, comma, geresh `׳`, gershayim `״`); empty string; punctuation-only string (result is empty, not a stray space).
  - [x] New `server/internal/grading/fuzzy.go`:
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
  - [x] New `server/internal/grading/fuzzy_test.go`: table-driven tests —
    - `TestLevenshtein`: identical strings (0); both empty (0); empty vs. non-empty (= length of the non-empty side); single substitution (1); single insertion (1); single deletion (1); completely different same-length strings (= length).
    - `TestGradeFuzzy`, covering epic AC-2's UJ-3 cases directly (call `GradeFuzzy` with un-trimmed/un-normalized input, not via `game.RecordAnswer`):
      - trailing-space case: `GradeFuzzy("צרורה ", []string{"צרורה"})` → `true`.
      - one-letter-typo case: an Accepted Answer with a single substituted letter in the response → `true` (e.g. `"ירושלים"` accepted, response `"ירוסלים"` — ש→ס).
      - unrelated-word case: `GradeFuzzy("תל אביב", []string{"ירושלים"})` → `false`.
      - normalization actually engages: `GradeFuzzy("שלום!", []string{"שלום"})` → `true` (punctuation stripped on the response side).
      - multiple accepted answers: matches the second of two when the first doesn't.
      - threshold boundary: a response whose edit distance is *one more* than `levenshteinThreshold` for that accepted answer's length → `false` (proves the threshold is enforced, not just "roughly close" — e.g. a 4-letter accepted answer, threshold 1, with a 2-edit-distance response).

- [x] **Task 3: `grading/pipeline.go` — add `StageFuzzy`** (AC: 1, 3)
  - [x] Add the constant and update the package/const-block comments (both currently reference this story by name as future work):
    ```go
    const (
        StageMCQ   Stage = "mcq"
        StageExact Stage = "exact"
        StageFuzzy Stage = "fuzzy"
        // StageAI joins this list in Story 3.6.
    )
    ```
    Update the package doc comment's `"(Fuzzy lands in Story 3.5, AI in Story 3.6; this story is Exact only)"` to reflect that Fuzzy has now landed (AI remains Story 3.6) — keep the sentence, just drop the now-stale "this story is Exact only" framing.

- [x] **Task 4: `game/answers.go` — run Fuzzy on an Exact miss** (AC: 1, 2, 3)
  - [x] Extend the `free_text` case in `RecordAnswer`'s type switch (currently a single `grading.GradeExact` call) into a three-way switch so a miss falls through to Fuzzy and a total miss is still graded (not left pending):
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
  - [x] `server/internal/game/answers_test.go`: update `TestRecordAnswerFreeTextNoMatchSetsIsCorrectFalse` — it currently asserts `Stage == grading.StageExact` for response `"תל אביב"` against accepted `["ירושלים"]`; that response also misses Fuzzy (unrelated word, AC-2), so the assertion becomes `Stage == grading.StageFuzzy` (the pipeline now runs one stage further before giving up). Update the doc comment to explain it's graded across Exact **and** Fuzzy now, still not left ungraded. `TestRecordAnswerFreeTextExactMatchSetsIsCorrectTrue` needs no change — an Exact hit still records `stage = exact` (proves the short-circuit).
    Add:
    - `TestRecordAnswerFreeTextFuzzyMatchSetsIsCorrectTrue` — a one-letter-typo response against a single accepted answer; assert `IsCorrect == true` and `Stage == grading.StageFuzzy`.

- [x] **Task 5: Quality gates + local E2E** (all ACs)
  - [x] Local gates: `gofmt -l .` · `go1.26.5 vet ./...` · `go1.26.5 test ./...` (zero regressions) · `sqlc generate` diff — expected **empty** (Task 1 note; unlike story 3.4). No `web/` changes in this story — skip `npm run lint`/`tsc -b` only if genuinely nothing under `web/` changed.
  - [x] Local Go E2E (`cmd/e2escratch`, deleted after use, same pattern as 2.1–3.4): create a scratch game with one Free-Text question (accepted answers `["ירושלים"]`). Open lobby, join 2 players (A, B). `POST /start`. Drive via signed inbound webhook `POST`s:
    - A replies `"ירוסלים"` (one-letter typo, ש→ס) — after the send, query the scratch DB directly and assert `is_correct = true, stage = 'fuzzy'`.
    - B replies `"תל אביב"` (unrelated word) — assert `is_correct = false, stage = 'fuzzy'` (graded, not left ungraded — proves the widened CHECK constraint from Task 1 actually applies and the migration ran cleanly).
    - `POST /close-question` → `POST /reveal`: must succeed (200) — both recorded answers are graded (this also confirms the migration's CHECK-constraint rename didn't break `RevealCurrentQuestion`'s `NOT EXISTS (... stage IS NULL)` guard from story 3.4).
    - Clean up the scratch game row afterward; delete the harness afterward — same convention as every prior story.

### Review Findings

Code review 2026-08-05 (Blind Hunter / Edge Case Hunter / Acceptance Auditor). Verified against the working tree at review time: `go build ./...`, `go vet ./...`, `go test ./internal/grading/... ./internal/game/...` all clean; CI's Hebrew copy-centralization gate clean.

- [x] [Review][Decision] **Levenshtein threshold is too lenient at short accepted-answer lengths — verified wrong grades against the shipped seed pack** — `levenshteinThreshold` returns 1 for any normalized accepted answer of ≤4 runes, which at those lengths is not "one typo" but "any nearby short word". Confirmed against `server/migrations/00005_question_packages.sql`, which ships in every DB: Q6 (Hanukkah candles, accepted `44`, 2 runes) grades `45`, `43`, `4`, `14` as **correct** — `45` being the single most likely wrong answer; Q9 (chapters in Psalms, accepted `ק"נ` → normalizes to `קנ`, 2 runes) grades the reply `כן` ("yes") as **correct**; Q9's `150` grades `100`, `250`, `50`, `15` as correct; Q3 (accepted `סיני`, 4 runes) grades `סין` (China) as correct. At the other end, threshold 3 for ≥11 runes lets Q10's `מנשה ואפרים` accept `מנשה ואשר` (distance exactly 3 — names a son of Jacob who is not Joseph's). Note also that the **shortest** accepted alternative sets the accept radius for the whole question, so an organizer adding a short convenience alias silently widens grading rather than narrowing it. Since no `UPDATE` against `answers` exists anywhere, a wrong verdict is permanent and feeds the leaderboard. The story flags this threshold as its own `[ASSUMPTION]` and asks review to flag it — this is that flag, with evidence. Decision needed on the calibration rule (e.g. distance 0 below some length floor, a ratio-based threshold, or distance < length). [server/internal/grading/fuzzy.go:55-64]
- [x] [Review][Decision] **Migration 00012's Down block is unrunnable the moment one fuzzy verdict exists** — `ADD CONSTRAINT ... CHECK` validates existing rows, and under Task 4's `default:` arm **every** free-text answer now writes `stage='fuzzy'`, misses included. From the first free-text answer graded under this code, `goose down` past 00012 fails with 23514 and rollback is blocked without hand-written SQL — exactly when an operator reaches for it (bad deploy mid-event). The Debug Log's "round-trips through goose correctly" only exercised the Up direction. Decision needed on the rollback data policy (delete fuzzy rows, relabel them, or document 00012 as one-way). [server/migrations/00012_answer_grading_fuzzy_stage.sql:11-12]
- [x] [Review][Decision] **`Normalize` deletes punctuation instead of substituting a separator** — `Normalize("תל-אביב")` → `תלאביב`, so a response typed `תל אביב` costs 1 of the 2 available edits before the typo the stage exists to absorb is even considered; two separators exhaust the budget entirely. `normalize_test.go`'s `{"comma stripped", "שלום,עולם", "שלומעולמ"}` asserts this as intended, while no case covers the spaced variant that reveals the inconsistency. Changing delete → replace-with-space changes matching semantics broadly and a delivered test pins the current behavior, so this is a deliberate call, not a mechanical fix. [server/internal/grading/normalize.go:56-69, server/internal/grading/normalize_test.go:13]
- [x] [Review][Patch] `GradeFuzzy` grades any ≤1-rune response correct when an accepted answer normalizes to empty or one rune [server/internal/grading/fuzzy.go:74-83] — `levenshteinThreshold(0)` and `(1)` both return 1 and there is no length guard. Reachable through the normal authoring UI: `validateQuestion` accepts a 1-rune accepted answer (`server/internal/httpapi/questions.go:141-145`), so `"?"` (→ normalizes to `""`) or `"7"` passes; `ImportPackageQuestions` (`server/internal/store/queries/packages.sql:24-33`) copies bank `accepted_answers` with no Go-side validation at all, and both `questions_type_shape` and `package_questions_type_shape` check only `cardinality >= 1`, never element emptiness. With accepted `["7"]`, `GradeFuzzy` returns true for `""`, `"8"`, `"0"`, `"א"`, `"?"`, `"😀"`. The empty-response half is production-reachable: `wa.classify` filters blank bodies with `strings.TrimSpace`, which does not remove category-Cf marks, so a body of a lone U+200F classifies as text and `stripFormatMarks` then empties it. Guard: skip an accepted answer that normalizes to empty, and never fuzzy-match an empty normalized response.
- [x] [Review][Patch] MCQ letter-parsing rewrite is out of scope, untested, and silently repairs a CI gate that has been red since story 3.4 [server/internal/game/answers.go:51-83] — Task 4 and the Project Structure Notes both declare the `mcq` case untouched, and the File List describes the answers.go delta as only the three-way switch; in fact ~26 of its 50 changed lines replace `mcqOptionLetters` with a `hebrewOptionLetterBase = 0x05D0` offset scheme. The rewrite is behavior-preserving (U+05D0-U+05D3 are consecutive; the `RuneCountInString == 1` guard preserves the old map's rejection of multi-character input) and it is **necessary** — verified that `git grep` for the Hebrew block at HEAD hits `server/internal/game/answers.go`, i.e. `.github/workflows/ci.yml`'s copy-centralization gate has been failing since that literal landed, and the working tree is now clean. But no test pins א→1 / ד→4 after the rewrite, and no story artifact (Debug Log, Completion Notes, Change Log, File List, `deferred-work.md`) records that the gate was red or that this fixed it. Add the MCQ parse test and document the fix.
- [x] [Review][Patch] Delivered `normalize.go` is not the source Task 2 prescribes, and the substitution is undocumented [server/internal/grading/normalize.go:8-55] — Task 2 quotes the full file including `var finalLetterForms = map[rune]rune{'ך': 'כ', ...}` and is checked `[x]`; what landed is five `finalXxx = 0x05Dx` constants plus a `+1`-offset `toMedialForm`. The deviation is forced (the spec's literal code contains Hebrew glyphs and would fail the same CI gate) — a spec defect, not a code defect, and the `+1` offset is correct for all five letters and is genuinely covered by `normalize_test.go`. Same class: Task 4's quoted baseline block opens `trimmed := strings.TrimSpace(rawText)` where the real baseline reads `strings.TrimSpace(body)` (story 3.4's `stripFormatMarks` output); the delivered code correctly kept `body`. Record both deviations in the story so the spec and the code stop disagreeing.
- [x] [Review][Patch] Two of the three threshold tiers are pinned by no test [server/internal/grading/fuzzy_test.go:29-50] — replacing `levenshteinThreshold`'s entire body with `return 1` leaves every delivered test green, including `TestRecordAnswerFreeTextFuzzyMatchSetsIsCorrectTrue`. No case exists at accepted-answer length ≥5 (tier 2) or ≥11 (tier 3), and none where distance exactly equals the threshold and must match. The one knob that decides whether real people score is untested across two thirds of its domain.
- [x] [Review][Patch] `TestLevenshtein` is 100% ASCII, so the "runes, never bytes" property it exists to protect is untested [server/internal/grading/fuzzy_test.go:5-27] — all eight cases are `"abc"`-shaped. Swapping `[]rune(a)` for `[]byte(a)` keeps them green while `Levenshtein("שלום","שלוב")` silently returns 2 instead of 1 (Hebrew letters are 2 bytes each), doubling every real distance. Add at least one Hebrew case.
- [x] [Review][Patch] `Normalize` does not strip category Cf, and `stripFormatMarks` is applied to the response only — never to accepted answers [server/internal/grading/normalize.go:56-69, server/internal/game/answers.go:157] — `unicode.IsSpace` (via `strings.Fields`) covers NBSP and the Z categories but not U+200F RTL MARK, U+200E LRM, U+200B, or U+FEFF, which RTL keyboards and copy-paste routinely inject. An organizer pasting an accepted answer from a Hebrew document carries U+200F; verified that accepted `"‏44‎"` against a cleanly typed `"44"` fails **both** Exact and Fuzzy (normalized accepted is 4 runes → threshold 1, distance 2). Every correct participant is graded wrong on a question that looks fine to the organizer, with no re-grade path. Story 3.4 already established Cf-stripping as the right call on the response side; `Normalize` should do the same for both sides.
- [x] [Review][Patch] `unicode.IsPunct` covers category P only — emoji and math/currency symbols survive and consume the edit budget [server/internal/grading/normalize.go:60] — `❤️` (U+2764 U+FE0F), `+ < = > | ~ ^ $` are category S, not P. A participant answering `"ירושלים ❤️"` (ordinary WhatsApp behavior) normalizes to 10 runes against a 7-rune accepted answer → distance 3 > threshold 2 → **incorrect**. Whether a right answer scores depends on which emoji the participant picked: a single `😀` squeaks in at exactly the threshold, a ZWJ family emoji does not. Strip `unicode.IsSymbol` alongside `unicode.IsPunct`.
- [x] [Review][Patch] No case folding, so Latin/transliterated answers grade by word length rather than correctness [server/internal/grading/normalize.go:56-69] — accepted `"NASA"` (4 runes, threshold 1) vs response `"nasa"` is distance 4 → **incorrect**, while accepted `"Jerusalem"` (9, threshold 2) vs `"jerusalem"` is distance 1 → **correct**. Identical error class, opposite outcome, decided purely by length. Add lowercasing in `Normalize`.
- [x] [Review][Patch] `normalize_test.go` covers no degenerate or realistic-noise input [server/internal/grading/normalize_test.go:5-34] — fourteen cases, none for whitespace-only input, tabs/newlines (WhatsApp multi-line answers), digits, Latin, mixed script, a medial letter that must be left alone, nikud-only input, NBSP, emoji, or bidi marks. This function is the sole gatekeeper for what the distance metric ever sees, and the untested classes are precisely what a WhatsApp participant produces.
- [x] [Review][Patch] `min3` is hand-rolled despite Go's builtin `min` accepting three arguments [server/internal/grading/fuzzy.go:35-44] — the module targets go1.26.5; delete the helper and call `min(a, b, c)`.
- [x] [Review][Patch] Close out story 3.4's deferral that was explicitly routed to this story [_bmad-output/implementation-artifacts/deferred-work.md] — 3.4 deferred "Bank-imported `accepted_answers` bypass the only trimming in the system" with revisit trigger **Story 3.5**, citing seed row `ק"נ` (ASCII quote) vs. the U+05F4 gershayim a Hebrew keyboard emits. Verified this story does close the gershayim class (both U+0022 and U+05F4 are category Po, so both normalize away and the two spellings match at distance 0) and mitigates the untrimmed-import class (`strings.Fields` trims inside `Normalize`, so Fuzzy now rescues what Exact still misses) — but the import path itself is unchanged and nothing records that this deferral was revisited. Update the entry rather than leaving it silently open.
- [x] [Review][Defer] `stage` cannot distinguish "fuzzy accepted this" from "fuzzy was consulted and declined", and pre-3.5 rows are not backfilled [server/internal/game/answers.go:171-174] — deferred, pre-existing design consequence
- [x] [Review][Defer] Nothing persists which accepted answer matched, at what distance, against what threshold [server/internal/grading/fuzzy.go:74-83] — deferred, pre-existing
- [x] [Review][Defer] Hebrew Presentation Forms (U+FB1D-FB4F) and Yiddish digraphs (U+05F0-U+05F2) survive `Normalize` [server/internal/grading/normalize.go:31-46] — deferred, pre-existing class
- [x] [Review][Defer] `parseMCQOption`'s comment overstates the strictness the code has ever had (`strconv.Atoi` accepts `"+2"`, `"02"`) [server/internal/game/answers.go:61-70] — deferred, pre-existing

#### Review resolutions applied (2026-08-05)

All 3 `decision-needed` and 11 `patch` findings above were applied in the same session. Gates after the fixes: `go build ./...`, `go vet ./...`, `go test ./... -count=1` (every package ok), CI's Hebrew copy-centralization gate clean, and `gofmt -l` clean on LF-normalized copies of every touched file (the repo-wide CRLF artifact noted in the Debug Log is unchanged).

**Decisions taken by the user:**

1. **Threshold recalibrated** to `0 / <=2 runes, 1 / 3-6, 2 / 7-12, 3 / 13+`, plus a `distance < length` requirement in `GradeFuzzy`. Fixes the 2-rune seed-pack cases (`44` no longer accepts `45`; the letter-numeral form of `150` no longer accepts the word for "yes") and, because `מנשה ואפרים` is 11 runes and drops from threshold 3 to 2, the wrong-son case too. **Residual, deliberately accepted:** at 3-6 runes one edit is still tolerated, so `150` still accepts `100` and `סיני` still accepts `סין`. Both are pinned as `RESIDUAL:` cases in `TestGradeFuzzy` so they are visible rather than silent — tightening further would cost genuine typo tolerance on ordinary short Hebrew words, which is a call for NFR-9 pilot evidence, not for review.
2. **Migration 00012's Down relabels before re-narrowing** (`UPDATE answers SET stage = 'exact' WHERE stage = 'fuzzy'`), so rollback is possible without deleting a participant's answer.
3. **Punctuation becomes a separator** rather than vanishing, with intra-word marks (geresh, gershayim, apostrophes, quotes) still deleted so an acronym stays one token. The `comma stripped` test case changed accordingly.

**Spec deviations recorded (finding: "Delivered `normalize.go` is not the source Task 2 prescribes"):**

- Task 2 quotes `normalize.go` with a `finalLetterForms` map spelled with Hebrew glyphs. That source **cannot** land: `.github/workflows/ci.yml`'s copy-centralization gate rejects any Hebrew literal in a non-test `.go` file, and `grading` has no exemption. The delivered file uses code-point constants (`finalKaf` … `finalTsadi`) plus a `+1`-offset `toMedialForm`; the offset is correct for all five letters and is covered by `normalize_test.go`. **The spec block is wrong, the code is right.**
- Task 4's quoted baseline opens `trimmed := strings.TrimSpace(rawText)`; the real baseline reads `strings.TrimSpace(body)` (story 3.4's `stripFormatMarks` output). The delivered code correctly kept `body`. Same conclusion.
- The same CI gate is why `answers.go`'s `mcqOptionLetters` map became `hebrewOptionLetterBase` arithmetic — an out-of-scope but **necessary** change: that gate had been red since the literal landed, and this story is what makes it green. Note the accepted-letter mapping was already pinned by the pre-existing `TestRecordAnswerMCQLetterAccepted` (all four letters), so the gap was only on the rejection side; `TestRecordAnswerMCQUnparseableReturnsFormatHint` gained `ה` (the code point immediately after DALET, which must not become option 5) and multi-rune cases.

**Process note:** the working tree was being edited by a concurrent session during this review — `normalize.go` was rewritten mid-run, and the initial diff snapshot caught it half-edited. Two reviewers consequently reported a non-existent compile failure; those findings were dismissed after verifying the tree builds and tests clean. The files were also `git add`-ed by something other than this review.


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

Claude Sonnet 5 (claude-sonnet-5), via the bmad-dev-story workflow.

### Debug Log References

- Re-verified this story's "current state" assumptions against the actual on-disk files before writing any code (per the story's own prerequisite note): `grading/pipeline.go`, `game/answers.go`, and `migrations/00011_answer_grading.sql` all matched the story's quoted snippets exactly — story 3.4 had since reached `done` (committed at `e5e3b16`) with no drift from what this story assumed.
- Confirmed the actual DB constraint name on the dev Postgres (Task 1) via `SELECT conname FROM pg_constraint WHERE conrelid = 'answers'::regclass` — `answers_stage_check`, matching the story's prediction exactly (Postgres's default naming for an inline `ALTER TABLE ... ADD COLUMN ... CHECK`).
- `gofmt -l .` flags ~20 files repo-wide (including several this story never touched — `engine.go`, `httpapi/*`, `wa/*`) — confirmed this is a pre-existing Windows `core.autocrlf=true` checkout artifact (`git config core.autocrlf` = `true`; `file` shows CRLF line terminators on the working-tree copies), not a regression. Verified every file this story actually created or edited is genuinely gofmt-clean by stripping `\r` into a scratch copy and running `gofmt -d` against that — zero real diffs.
- `sqlc generate` diff after Task 1: empty, as predicted (the migration only widens a CHECK constraint's allowed values; `stage` stays `pgtype.Text` in generated Go, sqlc never inspects CHECK bodies).
- Local Go E2E (`cmd/e2escratch`, deleted after use, same pattern as 2.1–3.4): booted the real `httpapi.NewRouter` + `wa` inbound webhook path in-process (`httptest.NewServer`) against the local dev Postgres (Docker), `wa.Client` pointed at a local fake Meta endpoint (`httptest.NewServer` echoing a fake message-ID response). Organizer provisioned in-process via `store.UpsertOrganizer` + `authSvc.Login` (no HTTP login round-trip, no `cmd/provision` subprocess). One early snag: the harness's first two runs reused static webhook message IDs (`wamid.e2e-1` etc.) across separate process invocations, so the second run's inbound requests were silently absorbed by the dedupe ledger (`wa_inbound_messages`) as "duplicate delivery, skipped" — the join/answer webhooks all returned 200 (by design, dedup is not an error) but nothing was actually processed, which read at first like a grading bug. Fixed by keying message IDs off `time.Now().UnixNano()` per run; the underlying grading code was never at fault. Full sequence once fixed: create scratch game + 1 Free-Text question (`accepted_answers: ["ירושלים"]`) → open-lobby → JOIN 2 players (A, B) via signed inbound webhook → `POST /start` → A replies `"ירוסלים"` (one-letter typo, ש→ס) → direct DB query confirmed `is_correct=true, stage='fuzzy'` → B replies `"תל אביב"` (unrelated word) → confirmed `is_correct=false, stage='fuzzy'` (graded, not left ungraded — proves the widened CHECK constraint from Task 1 round-trips through goose correctly) → `close-question` → `reveal` returned 200 (confirms the migration's constraint rename didn't break `RevealCurrentQuestion`'s `NOT EXISTS (... stage IS NULL)` guard from story 3.4). Scratch organizer deleted afterward (cascades to sessions/games/questions/participants/answers via `ON DELETE CASCADE`, confirmed by a follow-up query showing zero leftover rows); harness source and its build artifact deleted afterward (never committed).
- `go1.26.5 vet ./...` and `go1.26.5 test ./... -count=1` clean on every run (zero regressions across auth/config/game/grading/httpapi/store/wa/ws).

### Completion Notes List

- All 5 tasks and their subtasks complete; all 4 ACs satisfied and verified (unit tests + local E2E against real Postgres).
- AC-1/AC-2 (Fuzzy stage runs on an Exact miss, normalizes both sides, Levenshtein-threshold match, Exact short-circuits Fuzzy): `game.RecordAnswer`'s `free_text` case is now a three-way `switch true` (Exact → Fuzzy → miss); an Exact hit never calls `GradeFuzzy` (Go's `switch true` stops at the first matching case). `grading.Normalize`/`grading.GradeFuzzy`/`grading.Levenshtein` are pure functions with full unit coverage of the UJ-3 cases (trailing space, one-letter typo, unrelated word) plus the normalization cases (final letters, nikud, punctuation, whitespace collapse).
- AC-3 (a total miss is graded `false` at `stage='fuzzy'`, never left ungraded): `TestRecordAnswerFreeTextNoMatchSetsIsCorrectFalse` updated to assert `Stage == grading.StageFuzzy` (the pipeline now runs one stage further before giving up); confirmed end-to-end in the E2E (`תל אביב` vs. `ירושלים`).
- AC-4 (pure Go, `normalize.go` + `fuzzy.go`, unit tests): both files added under `server/internal/grading/`, zero store/DB/context dependency, matching the package's existing zero-I/O-in-tests posture. No new dependency added to `go.mod`.
- Migration 00012 widens `answers_stage_check` to admit `'fuzzy'` — confirmed the real constraint name on the dev DB before writing the migration (matched the story's prediction), and confirmed via the E2E that a `fuzzy`-staged row round-trips cleanly through the widened constraint and that `RevealCurrentQuestion`'s grading gate still functions correctly afterward.
- Levenshtein threshold (`levenshteinThreshold` in `fuzzy.go`) is the story's own documented `[ASSUMPTION]` — length-scaled (1/≤4, 2/≤10, 3/beyond) — flagging per the story's own note for code review; no PRD/architecture value exists to check it against.
- No `web/`/`strings.he.ts`/`messages_he.go` changes — every AC is server-internal grading computation only, same reasoning as story 3.4.
- No real-phone WhatsApp verification performed or needed — matches 3.2–3.4's precedent; the local E2E exercises the real webhook → inbound-routing → engine → grading → store path end-to-end against a fake Meta endpoint.

### File List

**New:**
- `server/migrations/00012_answer_grading_fuzzy_stage.sql`
- `server/internal/grading/normalize.go`
- `server/internal/grading/normalize_test.go`
- `server/internal/grading/fuzzy.go`
- `server/internal/grading/fuzzy_test.go`

**Modified:**
- `server/internal/grading/pipeline.go` (+`StageFuzzy` const, package doc comment updated)
- `server/internal/game/answers.go` (`RecordAnswer`'s `free_text` case gains the Fuzzy fallback via a three-way `switch true`)
- `server/internal/game/answers_test.go` (`TestRecordAnswerFreeTextNoMatchSetsIsCorrectFalse` updated to expect `stage=fuzzy`; new `TestRecordAnswerFreeTextFuzzyMatchSetsIsCorrectTrue`; section comment renamed to `"--- Grading (stories 3.4-3.5) ---"`)
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (story marked in-progress → review)

## Change Log

- 2026-08-05: Dev implementation complete (all 5 tasks, all 4 ACs) — `grading` package gains `normalize.go` (Hebrew normalization: final letters, nikud, punctuation, whitespace) and `fuzzy.go` (Levenshtein distance + length-scaled threshold matching), `game.Engine.RecordAnswer`'s Free-Text case extended into a three-way switch (Exact → Fuzzy → miss), migration 00012 widens `answers.stage`'s CHECK to admit `'fuzzy'`. All Go quality gates green (gofmt LF-normalized, vet, test, `sqlc generate` diff empty as predicted). Local Go E2E against a real webhook path + fake Meta endpoint passed clean (one-letter-typo match and unrelated-word miss both graded correctly at `stage='fuzzy'`; Reveal succeeded post-grading). Status: review.
- 2026-08-05: Code review (3 layers). 3 decision-needed + 11 patch findings applied in-session; 4 deferred to deferred-work.md; 7 dismissed. Levenshtein threshold recalibrated (exact-only below 3 runes, plus a distance < length guard) after verified mis-grades against the shipped seed pack; Normalize now strips category Cf and M on BOTH sides, treats punctuation and symbols as separators, keeps intra-word marks as deletions, and folds case; migration 00012 Down relabels fuzzy rows before re-narrowing so rollback is possible. Test coverage added for every threshold tier, Hebrew (rune-not-byte) Levenshtein distances, degenerate accepted answers, and the MCQ parser rejection boundary. Build/vet/test/CI-Hebrew-gate all green. NOT re-verified: the cmd/e2escratch local E2E was not re-run after these changes, and migration 00012 Down has never been executed against a real database. Status: done.
