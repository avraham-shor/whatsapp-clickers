---
title: 'Numeric-Token Exactness in Fuzzy Grading'
type: 'bugfix'
created: '2026-08-16'
status: 'done'
route: 'one-shot'
---

# Numeric-Token Exactness in Fuzzy Grading

## Intent

**Problem:** In a real production game, an accepted answer of "150" fuzzy-graded "100" and "050" as correct — every response within one Levenshtein edit of a short numeric answer was accepted, because the length-scaled fuzzy tier calibrated for word typos was also applied to digit strings, where a single changed digit is a different number, not a typo.

**Approach:** Added a token-level numeric rule to `GradeFuzzy`: after normalization, any whitespace-separated token consisting entirely of digits (with adjacent digit-groups split apart by a comma/period thousands-separator re-merged first) must match the response's same-position token exactly, with no edit tolerance; non-numeric tokens keep the existing length-scaled Levenshtein tolerance.

## Suggested Review Order

**The defect and its fix**

- Entry point — the type-rule rationale and what changed since story 3.5's length-based floor.
  [`fuzzy.go:39`](../../server/internal/grading/fuzzy.go#L39)

- The new exactness gate itself: any all-digits token must match the response's same-position token exactly.
  [`fuzzy.go:134`](../../server/internal/grading/fuzzy.go#L134)

- `GradeFuzzy` now runs the numeric gate before the existing whole-string Levenshtein/threshold check.
  [`fuzzy.go:194`](../../server/internal/grading/fuzzy.go#L194)

- Pinned regression tests reproducing the exact production report ("150" vs "100"/"050"/"50") plus the mixed numeric+word cases.
  [`fuzzy_test.go:103`](../../server/internal/grading/fuzzy_test.go#L103)

**Comma/period-grouped numbers (adversarial-review fix-forward)**

- `numericTokens` re-merges adjacent all-digits tokens so "1,000" and "1000" compare as one exact token — without this, Normalize's comma-to-space substitution would itself regress previously-correct matching.
  [`fuzzy.go:101`](../../server/internal/grading/fuzzy.go#L101)

- Pinned tests for both directions of the comma-grouping fix, plus a same-value-different-grouping negative case.
  [`fuzzy_test.go:118`](../../server/internal/grading/fuzzy_test.go#L118)

**Token alignment**

- `isNumericToken`: the digit-only predicate the whole rule is keyed on.
  [`fuzzy.go:84`](../../server/internal/grading/fuzzy.go#L84)

- Tests pinning that a numeric token not in first position, and a token-count mismatch, behave as intended.
  [`fuzzy_test.go:133`](../../server/internal/grading/fuzzy_test.go#L133)

**Known limitation, deferred rather than widened**

- A single Hebrew letter fused onto a number with no space (e.g. `כ150`) is not an all-digits token, so it falls back to the old word-tolerance path and can still fuzzy-match a wrong number. Not a regression — this shape had no protection before this fix either. See `deferred-work.md`, "Deferred from: adversarial review of fix/fuzzy-numeric-answers (2026-08-16)".
  [`fuzzy.go:170`](../../server/internal/grading/fuzzy.go#L170)
