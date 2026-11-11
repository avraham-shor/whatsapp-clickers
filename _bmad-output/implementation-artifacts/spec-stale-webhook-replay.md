---
title: 'Staleness guard on inbound WhatsApp messages'
type: 'bugfix'
created: '2026-08-17'
status: 'in-progress'
baseline_commit: '01cd4f0d4f97f15f59452dda02bde332fdc078a8'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/deferred-work.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Meta retries undelivered webhooks for days, and intake has no staleness guard — it discards Meta's `messages[].timestamp` and stamps every message with the processing clock. A replayed message is recorded as an answer to whatever question is open *now*: graded against the wrong answer key, and burning the Participant's one allowed answer so their real answer is refused. Confirmed in production 2026-08-17; full evidence in `deferred-work.md`, field-report section — **read it, it carries the reasoning behind every decision below**.

**Approach:** Carry Meta's send time through intake, then reject on two layers. For answers the rule is *exact*: the current question's open time is derivable as `answer_cutoff_at - time_limit_seconds`, so a message sent before that provably predates the question. Everything else (JOIN, rename) gets one coarse age gate. A rejected message is answered, never silently dropped, and never writes a row.

## Boundaries & Constraints

**Always:**
- Fail **open** on an absent or unparseable Meta timestamp — treat as "unknown age" and process normally. A parsing bug must never silence the chat (story 2.2).
- A stale answer writes **no** `answers` row, so `already_answered` stays clean and the Participant's real answer is still accepted. This is the point of the fix.
- Every rejected message gets exactly one reply (story 2.2, FR-2).
- Hebrew copy lives only in `messages_he.go`, and the new row enters EXPERIENCE.md's canonical table **first**. CI fails on a Hebrew literal in any other non-test `.go` file.

**Ask First:**
- Any change to `answers.received_at`, `scoring.go`, or speed-bonus ordering.
- Making the exact answer rule configurable, or widening the coarse gate to the answer path.

**Never:**
- Do **not** adopt Meta's timestamp as the scoring clock (Avraham's decision, 2026-08-17). FR-7's "our clock, never Meta's" stands; the speed-bonus distortion under a delivery blip stays a known deferred defect.
- No tunable threshold on the answer path — the derived open time is exact.
- Nothing under `web/`; no change to `DispatchAnswerRevealed` (that is a separate spec).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Live answer | `SentAt` inside current question's window, question open | Unchanged: recorded, graded, `AnswerAccepted` | N/A |
| Replayed answer | `SentAt` precedes current question's open time | `AnswerStale`; **no row written**; stale reply | N/A |
| Delayed-but-valid | `SentAt` in window, delivered after close | Unchanged: `AnswerClosed` | N/A |
| Replayed JOIN | non-answer message, age > coarse gate | Stale reply; no `Join`/`Rename` call, no state change | N/A |
| No timestamp | Meta omits it, or it will not parse | Processed exactly as today | Fail open; WARN once |
| Second answer | `SentAt` in window, already answered | Unchanged: `AnswerAlreadyAnswered` | N/A |

</frozen-after-approval>

## Code Map

- `server/internal/wa/webhook.go` -- `metaMessage` l.226-233 (no `timestamp`); `processMessage` stamps `ReceivedAt` at l.297
- `server/internal/wa/inbound.go` -- `Handle` l.167 owns both age and replier (gate belongs here); `Registrar` l.25-29 holds `RecordAnswer`'s signature
- `server/internal/game/answers.go` -- `AnswerOutcome` l.64-75; `RecordAnswer` l.161, pre-checks l.182-187
- `server/internal/store/queries/answers.sql` -- `GetOpenQuestionForPlayer` l.48 needs the derived column; `RecordAnswer` l.81 guard unchanged
- `server/internal/wa/messages_he.go` -- sole home for outbound copy; header carries a "Current map" list to update
- `server/internal/store/queries/games.sql:78` -- establishes `answer_cutoff_at = now() + time_limit_seconds`, the identity the exact rule inverts

## Tasks & Acceptance

**Execution:**
- [ ] `EXPERIENCE.md` -- add a "Message arrived late" row to the templates table -- canonical list; `messages_he.go` implements it, not the reverse
- [ ] `server/internal/wa/messages_he.go` -- add constant + accessor, list it in the header map -- copy centralization is CI-enforced
- [ ] `server/internal/wa/webhook.go` -- add `Timestamp` to `metaMessage`; parse unix seconds into a new `InboundMessage.SentAt`, zero on absent/unparseable -- carries Meta's clock without displacing ours
- [ ] `server/internal/store/queries/answers.sql` -- add `(g.answer_cutoff_at - (q.time_limit_seconds || ' seconds')::interval)::timestamptz AS question_opened_at` to `GetOpenQuestionForPlayer`; run `make generate` -- the exact rule needs the open time
- [ ] `server/internal/game/answers.go` -- add `AnswerStale`; add a `sentAt` param to `RecordAnswer`; reject **before** the already-answered/closed pre-checks when `sentAt` is non-zero and precedes `question_opened_at` -- staleness is a fact about the message, so it outranks facts about the question
- [ ] `server/internal/wa/inbound.go` -- widen `Registrar.RecordAnswer`; add the coarse gate at the top of `Handle` (after the empty-sender guard, before `classify`); route `AnswerStale` to the new reply
- [ ] `webhook_test.go`, `inbound_test.go`, `answers_test.go` -- cover every matrix row -- fail-open and no-row-written matter most

**Acceptance Criteria:**
- Given a message whose Meta timestamp precedes the current question's open time, when processed while that question is open, then no `answers` row exists for that Participant and question, and a later in-window message from them is still accepted.
- Given Meta omits `timestamp`, when any message is processed, then behavior is identical to today's.
- Given the suite, when `make lint && make test` runs, then both pass, and `sqlc generate` leaves no diff.

## Spec Change Log

## Design Notes

Why exact beats a threshold: a fixed budget discards a legitimate answer to a question carrying a 3-minute timer, yet accepts a 90-second replay already two questions downstream. The derived open time self-scales to each question's timer. The identity holds for as long as the answer path can accept anything — `CloseCurrentQuestion`'s `LEAST(answer_cutoff_at, now())` perturbs it only once the question is closed, and a closed question never reaches the insert.

Why 5 minutes is safe for the coarse gate: WhatsApp's timestamp is set when Meta's server received the message, not when the phone sent it. The gap it measures is therefore always **our** delivery failure, never a Participant's connectivity — someone whose phone was off for 20 minutes is unaffected, because Meta never held the message during those 20 minutes.

```go
// wa/inbound.go — zero SentAt means unknown age, so fail open.
const staleMessageAge = 5 * time.Minute

if !msg.SentAt.IsZero() && msg.ReceivedAt.Sub(msg.SentAt) > staleMessageAge {
    r.replier.Enqueue(msg.From, staleMessageReply())
    return
}
```

## Verification

**Commands:**
- `make generate` -- expected: `QuestionOpenedAt` appears on the generated row struct; commit the result (CI runs a `sqlc generate` diff-check)
- `cd server && go test ./...` -- expected: all pass, including new matrix cases
- `make lint` -- expected: `go vet` clean, `gofmt -l` empty
- `grep -rlP '[\x{0590}-\x{05FF}\x{FB1D}-\x{FB4F}]' --include='*.go' server/ | grep -v _test.go | grep -v messages_he.go` -- expected: no output (mirrors CI)
