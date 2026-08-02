---
baseline_commit: f23f54a
---

# Story 2.2: Universal Reply — the Chat Never Goes Silent

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a person messaging the platform number,
I want a helpful Hebrew reply to anything I send,
so that I always know what to do next (FR-2).

## Acceptance Criteria

1. **Given** an inbound message that is not a recognized command, **when** it is processed, **then** a short Hebrew help reply is queued to the sender — the EXPERIENCE.md templates table **Help** row, byte-exact. *(epic AC-1)*
2. **Given** an inbound message that is media, a sticker, or empty, **then** the same generic help reply is sent — **no inbound message class results in silence**. Concretely: `image`, `sticker`, `audio`, `video`, `document`, `location`, `contacts`, `reaction`, any unknown/future `type`, and a `text` message whose body is empty or whitespace-only. *(epic AC-2)*
3. **Given** the inbound parser, **then** every parse branch terminates in a reply, enforced *structurally* in `wa/inbound.go`: `classify` is total over `InboundMessage`, `replyFor` is total over the kind registry, and `Handle` enqueues exactly one reply on every path. A table-driven test walks the whole registry and fails if any kind yields empty copy. *(epic AC-3, first half)*
4. **Given** outbound copy, **then** all of it lives in `server/internal/wa/messages_he.go` per the EXPERIENCE.md message-templates table — warm-playful, short, gender-neutral (UX-DR15, revised 2026-07-09) — with LTR tokens bidi-isolated (DESIGN.md Typography), **and** no Hebrew literal exists in any other non-test Go file (CI-greppable). *(epic AC-3, second half)*
5. **Given** any log line produced by the reply path, **then** the phone number appears only as `phone_last4` and the Meta wamid only as `WaMessageIDDigest` output; a healthy run produces zero `ERROR` lines. *(NFR-4, NFR-8 — standing obligations, carried from 2.1's AC-5)*
6. **Given** the deployed/tunnelled server and a real WhatsApp message from a verified test recipient, **when** they send arbitrary Hebrew text and then a sticker, **then** both receive the Help reply on their phone. *(live proof of AC-1/AC-2 through the real Meta path)*

## Tasks / Subtasks

- [x] **Task 1: `messages_he.go` — the canonical copy file** (AC: 1, 4)
  - [x] New `server/internal/wa/messages_he.go`. File doc comment: this file is the **only** home for outbound WhatsApp Hebrew copy (architecture Communication Patterns + Enforcement); the canonical row list is EXPERIENCE.md → *Voice and Tone* → *WhatsApp message templates*; a new message type is added to that table **first**, then here.
  - [x] **Exactly one row this story: Help (Universal Reply).** Do **not** pre-create Welcome / Pre-lobby / Invalid code / Spectator notice / Name updated (2.4, 2.5) or any Question/Answer/Result row (Epic 3). Copy that cannot be exercised cannot be reviewed for fidelity, and each row lands with the story that sends it. Record the remaining rows and their owning stories in the file's doc comment so the map is visible.
  - [x] Canonical Help copy, byte-exact from the templates table:
        `כאן משחק החידון! כדי להצטרף שלחו: JOIN ואחריו הקוד (לדוגמה: JOIN COHEN24). את הקוד מקבלים מהמארגן.`
  - [x] **Bidi isolation is mandatory** (DESIGN.md Typography: "bidi-safe composition (isolate marks) in `messages_he.go` templates"). Both LTR runs — `JOIN` and `JOIN COHEN24` — are wrapped in U+2066 LEFT-TO-RIGHT ISOLATE (Go: `⁦`) … U+2069 POP DIRECTIONAL ISOLATE (Go: `⁩`). Without this the Unicode bidi algorithm visually reorders the code fragment in WhatsApp and the Participant reads a scrambled example code. The web side already does the equivalent with `<bdi dir="ltr">` ([game-editor-page.tsx:113](web/src/features/builder/game-editor-page.tsx#L113)) — this is the plain-text form of the same rule.
  - [x] Shape: keep the Hebrew literal as **one contiguous run** with `%s` placeholders and compose with `fmt.Sprintf` + a tiny `ltr(s string) string` helper returning `"⁦" + s + "⁩"`. **Write the isolates as Go `\u` escapes, never as pasted invisible characters** — they are unreviewable in a diff and one stray copy-paste silently drops them. Do **not** build the string by concatenating Hebrew fragments around bare Latin literals — mixed-direction concatenation is unreadable and mis-editable in every editor. Export nothing: `wa` owns all outbound copy and the dependency direction (`wa` → `game`) means no other package ever needs these strings.
  - [x] Identifiers (Task 6 refers to these by name): `msgHelpTemplate` — the unexported const holding the Hebrew run with its two `%s` placeholders; `helpMessage() string` — the unexported composer returning the finished, isolate-wrapped copy; `ltr(s string) string` — the isolate helper. Row constants follow `msg<Row>Template` as the table grows.
  - [x] Encoding discipline (Windows tax): write this file with the Write tool only — PowerShell redirection emits UTF-16 and a BOM. Hebrew in Go source is already proven in this repo ([webhook_test.go](server/internal/wa/webhook_test.go), [00005_question_packages.sql](server/migrations/00005_question_packages.sql)), so no new encoding risk — just don't reintroduce it.

- [x] **Task 2: `wa/inbound.go` — parse taxonomy + Universal Reply router** (AC: 1, 2, 3, 5)
  - [x] New `server/internal/wa/inbound.go` — the canonical filename from the architecture tree (`inbound.go # parse → JOIN / answer / unrecognized (Universal Reply)`).
  - [x] Consumer-defined interface in the same file (the `Deduper`/`SenderClient` precedent):
        `type Replier interface { Enqueue(to, body string) }` — `*Dispatcher` satisfies it; tests inject a stub.
  - [x] `type InboundRouter struct { replier Replier; logger *slog.Logger }` with `NewInboundRouter(replier Replier, logger *slog.Logger) *InboundRouter` (nil logger → `slog.Default()`, matching `NewWebhookHandler`/`NewDispatcher`) and `Handle(ctx context.Context, msg InboundMessage)`.
        ⚠️ **Do not name the type `InboundHandler`** — that identifier is already the consumer-defined interface in [webhook.go:46](server/internal/wa/webhook.go#L46) that this type implements. Add the compile-time assertion `var _ InboundHandler = (*InboundRouter)(nil)`.
  - [x] **The totality structure — this is AC-3, not decoration:**
    ```
    type inboundKind string
    const (
        kindText    inboundKind = "text"      // non-empty text, no recognized command
        kindEmpty   inboundKind = "empty"     // type "text" with blank/whitespace-only body
        kindNonText inboundKind = "non_text"  // image, sticker, audio, video, document, location, contacts, reaction, unknown
    )
    // allInboundKinds is the registry every kind must join. Stories that add a
    // kind (JOIN + שם: in 2.4, answers in 3.3) add it here and to replyFor's
    // switch; the registry test fails otherwise.
    var allInboundKinds = []inboundKind{kindText, kindEmpty, kindNonText}
    ```
  - [x] `classify(msg InboundMessage) inboundKind` — total: `msg.Type != "text"` → `kindNonText`; `strings.TrimSpace(msg.TextBody) == ""` → `kindEmpty`; else `kindText`.
  - [x] `replyFor(kind inboundKind) string` — exhaustive `switch` returning `helpMessage()` for all three kinds today, with a `default:` that also returns `helpMessage()` (belt and braces: a future kind added to the registry but forgotten in the switch still replies rather than going silent — the registry test catches the omission, the default keeps the chat alive if it doesn't).
  - [x] `Handle`: guard `msg.From == ""` → WARN `"inbound message has no sender, cannot reply"` + return (there is nobody to answer; this is the only silent path and it is a malformed-payload case, not an inbound class). Otherwise `classify` → `replyFor` → **exactly one** `r.replier.Enqueue(msg.From, reply)` → INFO `"universal reply queued"` with `kind`, `phone_last4` = `PhoneLast4(msg.From)`, `wa_message_id` = `WaMessageIDDigest(msg.WaMessageID)`.
  - [x] `ctx` is unused this story (`Enqueue` is fire-and-forget and never blocks). Keep the parameter — it satisfies `InboundHandler` and 2.4's registration lookup will use it. Name it `ctx`, add `_ = ctx` only if the linter demands it (it does not; unused function params are legal in Go).
  - [x] **Explicitly out of scope — do not implement:** `JOIN <code>` parsing, `שם:` parsing, MCQ letter/digit parsing, the 200-char Free-Text cap, the `participants` table, the `game` package (**must still not exist** after this story), any WebSocket work, any `web/` change. In 2.2 `JOIN ABC123` and `שם: רחל` are *unrecognized text* and correctly receive the Help reply — a test pins exactly that, so a later reader does not mistake it for a bug.

- [x] **Task 3: Dispatcher hardening at its first real caller** (AC: 2, 5)
  - [x] Empty recipient/body guard — the 2.1 deferred item whose stated revisit trigger is *"2.2's first `Enqueue` caller"* ([deferred-work.md:28](_bmad-output/implementation-artifacts/deferred-work.md#L28)). In `Dispatcher.Enqueue`, drop with WARN `"outbound message missing recipient or body, dropped"` when `to == ""` or `body == ""`, before touching the queue. Without it an empty `to` burns 3 attempts + 1s/2s backoff + 3 rate tokens per message.
  - [x] `Drain(ctx context.Context)` on `Dispatcher`: block until the queue is empty or `ctx` is done (poll `len(d.queue)` on a ~50ms ticker). Used by the shutdown sequence in Task 4. Document the known limit: an empty queue means nothing is *waiting*, not that in-flight sends have landed — those are bounded by `sendTimeout` and the shutdown grace.

- [x] **Task 4: Wiring — `main.go` composition root** (AC: 1, 2, 6)
  - [x] Delete `stubInboundHandler` from [main.go:25-34](server/cmd/server/main.go#L25-L34) entirely (type + method + its INFO line). Wire `inboundRouter := wa.NewInboundRouter(dispatcher, logger)` and pass it to `wa.NewWebhookHandler(...)`. Name the variable `inboundRouter`, not `router` — `router` is already the chi handler in the same function.
  - [x] **Fix the shutdown ordering — this is required for the story to actually hold, not a nice-to-have.** Today the dispatcher runs on the same signal-aware `ctx` as everything else ([main.go:83](server/cmd/server/main.go#L83)), so on SIGTERM its workers stop *immediately* while `srv.Shutdown` keeps draining in-flight webhook requests for up to 15s. Every reply those requests enqueue lands in a queue with no workers and is lost without a log line. That was harmless in 2.1 (the queue was wired but unfed — the review dismissed it on exactly that basis) and becomes "the chat goes silent on redeploy" the moment this story feeds it. Railway redeploys on every merge to main.
    - [x] Give the dispatcher its own `context.WithCancel(context.Background())` so it outlives the signal context, and run it as `go func() { defer close(dispatcherDone); dispatcher.Run(dispatchCtx) }()`.
    - [x] After `srv.Shutdown` returns (all handlers finished, so everything they queued is now *in* the queue): `dispatcher.Drain(drainCtx)` with a ~5s bounded `drainCtx`, then cancel `dispatchCtx`, then wait on `dispatcherDone` with a short grace before returning — so `Run`'s "shut down with unsent messages" summary WARN is actually emitted before the process exits.
    - [x] Keep total shutdown work inside Railway's SIGTERM window; the existing `srv.Shutdown` budget is 15s — size the drain grace so the sum stays comfortably under it (5s drain + short join is fine).

- [x] **Task 5: `WHATSAPP_API_BASE_URL` — injectable Cloud API base URL** (AC: 6 verification path; test-architecture item T-1)
  - [x] `config.go`: **optional** var (no `require()`, no placeholder validation), empty = use the client default. Add to `Config` as `WhatsAppAPIBaseURL`.
  - [x] `main.go`: when non-empty, pass `wa.WithBaseURL(cfg.WhatsAppAPIBaseURL)` to `wa.NewClient` — the option already exists ([client.go:36](server/internal/wa/client.go#L36)); this only reaches it from the environment.
  - [x] `.env.example`: document it as test-harness-only, unset in production.
  - [x] `config_test.go`: unset → empty (boot succeeds); set → carried through.
  - [x] Why here: this is test-architecture item **T-1** (owner: dev, scheduled "Epic 1–2 scaffold" — [test-design-architecture.md:52](_bmad-output/test-artifacts/test-design-architecture.md#L52)), and it is what makes Task 7's local E2E possible at all. Without it, verifying that a reply is actually *sent* means firing real messages at Meta on every local run — the `WithBaseURL` code option only helps in-process Go tests. Keep it to these four small edits; the fake provider itself is a scratchpad script, not a committed artifact.

- [x] **Task 6: Tests** (AC: 1, 2, 3, 4, 5)
  - [x] `server/internal/wa/inbound_test.go` — **scenario A7, the Universal Reply matrix** (test-design P0, `ASR-3`): one table-driven test over every inbound class, each asserting **exactly one** `Enqueue` call, to `msg.From`, with body == the canonical Help copy. Rows: plain Hebrew noise · plain Latin noise · a single emoji · `JOIN ABC123` (unrecognized in 2.2 — pin it) · `שם: רחל` (same) · empty `TextBody` · whitespace-only body · `image` · `sticker` · `audio` · `video` · `document` · `location` · `contacts` · `reaction` · `some_future_type`.
  - [x] Registry totality test: loop `allInboundKinds`, assert `replyFor(kind) != ""`. Add a `classify` test asserting its result is always a member of `allInboundKinds`.
  - [x] Copy-fidelity test: strip U+2066/U+2069 from `helpMessage()` and assert equality with the canonical table string held verbatim in the test. This is the guard that survives an editor mangling the mixed-direction literal.
  - [x] Bidi test: assert `helpMessage()` contains `"⁦JOIN⁩"` and `"⁦JOIN COHEN24⁩"` — proves the isolates are present *and* correctly placed, which the strip test alone cannot.
  - [x] Empty-sender test: `From == ""` → zero `Enqueue` calls + WARN.
  - [x] Log-discipline test (the 2.1 captured-slog pattern, `newTestLogger()` in [webhook_test.go:47](server/internal/wa/webhook_test.go#L47)): with a realistic full MSISDN and a realistic `wamid.` value, assert the captured output contains neither the full number nor the raw wamid, contains `phone_last4`, and contains no `ERROR`.
  - [x] `dispatch_test.go` additions: empty `to` → dropped + WARN, `SendText` never called; empty `body` → same; `Drain` returns once the queue empties, and returns on ctx expiry when it does not.
  - [x] Stub naming: `wa`'s test package already has `stubInboundHandler` — name the new one `stubReplier` and keep the grown-in-test, no-mock-framework convention.
  - [x] No test for `main.go` (composition root; covered by Task 7's E2E). No real-network test in `go test`.

- [x] **Task 7: CI copy-centralization gate** (AC: 4)
  - [x] Add a step to the `backend` job in [.github/workflows/ci.yml](.github/workflows/ci.yml), mirroring the shape of the existing filter-safety grep (explicit `status` handling so "no matches" is success, not a failed step): fail if any Hebrew codepoint appears in a `.go` file other than `internal/wa/messages_he.go` and `*_test.go`.
        `grep -rlP '[\x{0590}-\x{05FF}]' --include='*.go' . | grep -v '_test\.go$' | grep -v 'internal/wa/messages_he\.go$'`
  - [x] Test files are legitimately exempt — they assert on Hebrew fixtures today ([webhook_test.go](server/internal/wa/webhook_test.go), the httpapi suites) and will more so after Task 6.
  - [x] Run the same grep locally before committing; it must print nothing.

- [x] **Task 8: Quality gates + E2E verification** (all ACs)
  - [x] Local gates: `gofmt -l .` clean · `go vet ./...` · `go test ./...` (every existing suite stays green — regression guard) · `sqlc generate` empty diff (no schema change, but CI gates it) · `npm run lint` · `npx tsc -b --noEmit` · `npm run build` · both filter-safety greps · the new Task 7 grep. `make lint` + `make test` cover most of these.
  - [x] **Local E2E** (Python/stdlib per the 1.5 + 2.1 pattern — HMAC needs scripted requests, and Hebrew must never pass through Git Bash `curl -d`; write the script to the session scratchpad, not the repo, and encode payloads UTF-8 explicitly):
    - [x] Stand up a trivial fake provider (Python `http.server`, ~20 lines) that accepts `POST /{phoneNumberID}/messages`, records the JSON body, and returns `{"messages":[{"id":"wamid.fake"}]}` with 200. Boot the server with `WHATSAPP_API_BASE_URL` pointed at it.
    - [x] Drive one signed webhook POST per inbound class from the AC-2 list; for each, assert HTTP 200 **and** that the fake provider received exactly one send, addressed to the sender, whose `text.body` equals the canonical Help copy (compare as decoded UTF-8, including the isolate codepoints).
    - [x] Assert the log stream: one `"universal reply queued"` INFO per message with the right `kind`, zero `ERROR` lines, zero occurrences of the full test MSISDN, zero raw `wamid.` values.
    - [x] Shutdown check: enqueue-heavy burst → SIGTERM the server → confirm the fake provider still receives the queued replies (proves Task 4's drain ordering) and that any residual drop is logged, not silent.
    - [x] Clean up rows the run adds to `wa_inbound_messages`.
  - [x] **Live Meta E2E with Avraham** (AC: 6) — needs him and his phone; everything else in this story is verifiable without him, so do not block the other tasks on it. cloudflared quick tunnel (`--protocol http2` is required on this network — QUIC/UDP 7844 is blocked) → re-verify the callback URL in the Meta console (the quick-tunnel URL changes every run) → from the verified test recipient send (a) arbitrary Hebrew text and (b) a sticker → **both** must come back with the Help reply, rendered with `JOIN` and `JOIN COHEN24` reading left-to-right and unscrambled on the phone. Confirm zero ERROR lines and only `phone_last4` in the server log.
    - [x] Note for probes: `curl` on this box fails all TLS to Meta (`schannel: 0x80092012`); use Go or PowerShell.

### Review Findings

Code review 2026-08-02 — three parallel layers (Blind Hunter, Edge Case Hunter, Acceptance Auditor), all completed. The Acceptance Auditor found **no code-level AC violations** (Help copy verified byte-exact vs EXPERIENCE.md: 98 runes, 159 bytes, sha256 `ae51aedf…` matching all three sources; the CI gate verified live by planting a violation).

- [x] [Review][Decision] No per-sender damping on the universal reply — a mutual auto-responder loop is unbounded. An inbound from an auto-responder (WhatsApp Business away-message, another bot) triggers our Help reply → its auto-reply → our Help reply, with nothing rate-limiting, deduplicating per sender, or detecting repetition; reaction add+remove toggles on our own messages arrive as distinct wamids (dedupe never intervenes) and each earns a fresh Help reply. Burns the shared 70/s budget and Meta quality rating. [server/internal/wa/inbound.go:92-108] — **resolved 2026-08-02: deferred.** Meta test-mode prevents the loop today (only verified recipients receive our replies), and a damping policy (per-sender cooldown) is a design decision that deserves its own story. Trigger: before the number leaves test-mode / opens to a wide audience.
- [x] [Review][Decision] `WHATSAPP_API_BASE_URL` is accepted with zero URL validation — `dummy` boots green (pinned deliberately by `TestLoadAPIBaseURLNotPlaceholderValidated`), after which every send fails at request-build ×3 attempts + backoff, surfaced only as per-message WARNs. [server/internal/config/config.go:86-88] — **resolved 2026-08-02: dismissed, keep as-is.** The spec pinned "no placeholder validation"; the variable is test-harness-only, unset in production, and a bad value fails loudly and immediately for the developer running the harness.
- [x] [Review][Patch] Error-path exits bypass the entire dispatcher drain sequence — `return err` skips Drain/stop/join on both the ListenAndServe-failure and `srv.Shutdown`-error paths; concretely reachable: webhook `payloadBudget` (20s) outlives `httpShutdownTimeout` (15s), so one degraded-DB payload mid-flight times Shutdown out and every queued reply is abandoned with no summary WARN — on exactly the busy-shutdown path where the queue is fullest [server/cmd/server/main.go:130-142] — **fixed 2026-08-02**: drain/stop/join extracted into a shared `drainAndStop` closure called from all three exits (ListenAndServe error, Shutdown error, clean path); only the clean path's join gates the "server stopped cleanly" INFO.
- [x] [Review][Patch] Railway's SIGTERM→SIGKILL grace defaults to **0 seconds** — the entire shutdown machinery this story builds never runs on a real redeploy unless `RAILWAY_DEPLOYMENT_DRAINING_SECONDS` is set on the service (verified against Railway's deployment-teardown docs); main.go's comment asserts the timeouts "stay inside Railway's SIGTERM window" but the default window is zero. Set the service variable (≥ 25s) + fix the comment + document the requirement [server/cmd/server/main.go:74-86] — **fixed 2026-08-02**: comment corrected to state the 0s default and the 30s recommendation; ops note added to `.env.example` documenting the required `RAILWAY_DEPLOYMENT_DRAINING_SECONDS=30` service variable (not app-read, so it can't live in `Config`). **Avraham: set this on the Railway service dashboard before the next redeploy** — the code fix alone does not set it.
- [x] [Review][Patch] Totality-enforcement comments overpromise — `replyFor`'s `default:` arm returns `helpMessage()`, so `TestReplyForIsTotalOverRegistry` (non-empty check) can never fail for a kind added to the registry but forgotten in the switch; comments claim "the registry test fails otherwise" and "catches the omission at build time". Reword to what is actually guaranteed [server/internal/wa/inbound.go:29-31] — **fixed 2026-08-02**: both comments reworded to state the registry test only proves non-empty copy, not per-kind routing correctness; that burden shifts to the story adding the kind.
- [x] [Review][Patch] Whitespace-only recipient/body pass both new guards — `to == ""`/`body == ""`/`msg.From == ""` compare without TrimSpace, so `" "` sails through and burns 3 attempts + 3s backoff + 3 rate tokens, the exact waste the guard was added to eliminate [server/internal/wa/dispatch.go:116, server/internal/wa/inbound.go:93] — **fixed 2026-08-02**: both guards now use `strings.TrimSpace(...) == ""`.
- [x] [Review][Patch] CI copy-centralization exemption is an unanchored suffix match — `grep -v 'internal/wa/messages_he\.go$'` also exempts any future path merely ending that way; anchor it to the one real path [.github/workflows/ci.yml:54] — **fixed 2026-08-02**: anchored to `^\./internal/wa/messages_he\.go$`; re-ran the gate locally, still clean.
- [x] [Review][Patch] CI Hebrew detection misses Alphabetic Presentation Forms — U+FB1D–FB4F (precomposed ligatures, e.g. pasted from a PDF) evade `[\x{0590}-\x{05FF}]`; extend the class [.github/workflows/ci.yml:46] — **fixed 2026-08-02**: codepoint class extended to `[\x{0590}-\x{05FF}\x{FB1D}-\x{FB4F}]`.
- [x] [Review][Patch] Stale "**Not run: the Live Meta E2E subtask.**" line contradicts the recorded AC-6 completion — leftover from the pre-live-run revision; the Debug Log (15:14–15:24 run), Task 8 checkbox and Change Log all record the live run as done. Delete the line [story file, Completion Notes] — **fixed 2026-08-02**: line corrected to point at the recorded evidence.
- [x] [Review][Patch] File List annotation stale — `sprint-status.yaml (2.2 → in-progress)` describes an intermediate state; the submitted state is `review` [story file, File List] — **fixed 2026-08-02**: annotation updated to `→ review`.
~~[Review][Patch] Test fixture comment claims "non-breaking space" but the byte is an ASCII space~~ **false positive, verified 2026-08-02 and dismissed.** The fixture byte is genuinely U+00A0 (`\xa0`), confirmed by reading the raw file bytes; the reviewing agent misread it from the diff rendering. No change needed. [server/internal/wa/inbound_test.go:134]
- [x] [Review][Patch] "server stopped cleanly" is logged even when the dispatcher missed its stop grace — the grace-timeout branch WARNs and then falls through to the all-clear INFO; gate the INFO on a clean join [server/cmd/server/main.go:151-157] — **fixed 2026-08-02**: folded into the `drainAndStop` refactor above — the INFO is now inside `if drainAndStop() { ... }`.
- [x] [Review][Defer] Zero in-flight grace between `Drain` returning and context cancellation [server/cmd/server/main.go:148-150, server/internal/wa/dispatch.go:174-187] — deferred: WARN-logged and documented accepted pilot posture; trigger: Epic 3
- [x] [Review][Defer] Router INFO "universal reply queued" fires even when `Enqueue` dropped on a full queue [server/internal/wa/inbound.go:104-108] — deferred: fix needs the spec-pinned `Replier` signature to change; trigger: story 3.3
- [x] [Review][Defer] `dispatcherDrainTimeout` cannot cover the queue it guards (5s ≈ ≤420 of 1000 capacity) [server/cmd/server/main.go:31] — deferred: spec-endorsed sizing, pilot depth never approaches it; trigger: beyond-pilot scale

## Dev Notes

### What this story is — and is not

The inbound half of the WhatsApp conversation, with nothing game-shaped in it: a total parse taxonomy, one canonical Hebrew reply, and the wiring that turns 2.1's logging stub into a real responder. Story 2.1 built the transport in both directions; this story is the first code that *speaks*. It does **not** build: JOIN/registration (2.4), `שם:` handling (2.4), spectator logic (2.5), the `participants` table or the `game` package (2.3 — **must still not exist** after this story), any WebSocket or dashboard work (2.3), any answer parsing (Epic 3), any `web/` diff. No new Go or npm dependencies.

The visible outcome: send anything at all to the platform number and a Hebrew help message comes back. That is FR-2 in its entirety.

### Design decisions locked for this story (rationale recorded — do not relitigate)

- **One template row, not the whole table.** `messages_he.go` is created with the Help row only. The templates table is canonical and final, but copy that no code path can exercise cannot be reviewed for fidelity, and 17 unused constants invite drift when Epic 3 finally reaches them. Each row lands with the story that sends it; the file's doc comment carries the full map so nobody has to rediscover it. (2.1 refused to create the file early for the mirror-image reason: an empty file invites a literal to sneak in outside the canonical task.)
- **Bidi isolates, not raw LTR tokens.** DESIGN.md makes isolation mandatory in `messages_he.go`, and the Help copy is the worst case — a Latin keyword *and* an alphanumeric example code inside RTL Hebrew, ending in a period that the bidi algorithm will happily relocate. Isolates are invisible in WhatsApp and cost nothing; the alternative is a first impression that reads as broken.
- **Totality by construction, verified by registry.** "Every parse branch terminates in a reply" is an AC, so it gets a mechanism rather than a promise: a kind registry (`allInboundKinds`), a total `classify`, an exhaustive `replyFor` with a still-replying `default`, and a test that walks the registry. Later stories extend the registry; the test fails if they extend `classify` without extending `replyFor`.
- **`InboundRouter`, not `InboundHandler`.** The interface name is taken by webhook.go. The compile-time assertion documents the relationship without a comment that can rot.
- **Two INFO lines per inbound message is accepted.** webhook.go logs intake (`"inbound message accepted"`, the dedupe-correlation record) and the router logs the routing decision (`"universal reply queued"` with `kind`). The 2.1 review flagged the 2.1 double-INFO as cosmetic-and-resolved-in-2.2 — what resolves is the *stub's* content-free line, not the count. Inbound volume is a handful of messages per Participant; the thousands are outbound and unlogged per message. `kind` is the field that makes the reply path debuggable during a live event.
- **The only silent path is a message with no sender.** `From == ""` is a malformed payload, not an inbound class; there is literally no address to answer. WARN and drop. Every real inbound class replies.
- **Shutdown ordering is in scope.** See Task 4. A story whose title is "the chat never goes silent" cannot ship a redeploy path that silently swallows queued replies, and Railway redeploys on every merge. Bounded drain + explicit join, ~15 lines.
- **`WHATSAPP_API_BASE_URL` is in scope.** It is the difference between an E2E that proves replies are *sent* and one that proves the server returned 200. Test-architecture T-1, owner dev, scheduled for exactly this window. Optional var, no validation, default unchanged.
- **No client-side per-recipient throttle.** 2.1's research found Meta throttles sustained sends to the *same* recipient (~1 msg/6s). A hostile rapid-fire sender may therefore see some of our Help replies throttled Meta-side. Accepted for the pilot: they surface as the dispatcher's existing send-failure WARNs, and a per-recipient limiter would be new machinery guarding against a non-problem at pilot scale.
- **No reply-loop guard.** Meta does not deliver our own outbound messages back as inbound `messages[]` entries, so self-echo is not a path. Bot-to-bot ping-pong with a third-party auto-responder is theoretically possible and is bounded by Meta's per-pair throttle; not worth machinery in the pilot.

### Existing code this story modifies — current state, and what must survive

Read these before writing anything; three of the four are load-bearing for behavior this story must not break.

- **[server/internal/wa/webhook.go](server/internal/wa/webhook.go)** — *not modified*, but it defines the seam. `processMessage` (L207) normalizes into `InboundMessage`, drops empty-wamid messages, dedupes, then calls `h.inbound.Handle(ctx, msg)` **synchronously inside the HTTP request** and always answers 200. Consequences for the router: `Handle` must stay fast and non-blocking (`Enqueue` is, by design) and must never panic on hostile input — there is no panic-recovery middleware anywhere in the repo, and a panic here costs the connection and the reply. Note also that `TextBody` is populated *only* for `Type == "text"` (L215) — that is why `classify` keys on `Type` first.
- **[server/internal/wa/dispatch.go](server/internal/wa/dispatch.go)** — gains the empty-arg guard and `Drain`. Must survive untouched: `Enqueue` never blocks (queue-full → drop + WARN, NFR-2); the shared `rate.NewLimiter(70,70)`; 3 attempts with the 1s→2s table and the `init()` assertion tying `dispatchBackoff` to `dispatchMaxAttempts`; the `ctx.Err()` early-returns that keep shutdown from emitting misleading retry WARNs; WARN-never-ERROR on permanent failure.
- **[server/cmd/server/main.go](server/cmd/server/main.go)** — stub removed, router wired, dispatcher lifecycle separated. Must survive: `signal.NotifyContext` SIGTERM handling and the 15s `srv.Shutdown` drain; migrations running before serving; the `store` value satisfying `Deduper` directly (L85); server timeouts (the `WriteTimeout` comment about `/ws` is 2.3's, leave it).
- **[server/internal/config/config.go](server/internal/config/config.go)** — one optional var added. Must survive: the collect-all-offenders-in-one-error pattern in `load`, `validateSessionSecret`, and `validateWhatsAppValues` (the four `WHATSAPP_*` placeholder rejections that make local boots demand real Meta values). The new var goes through neither `require()` nor placeholder validation.

### Previous story intelligence (2.1 — established patterns, follow or rework)

- **The wamid is not opaque.** Its leading segment base64-encodes the sender's MSISDN; the first live delivery in 2.1 leaked a full phone number into an INFO line that every synthetic test fixture had hidden. `WaMessageIDDigest` exists for this. **Carry the generalized lesson forward:** treat any Meta-supplied identifier as embedding the MSISDN until decoded and checked, and never write a captured real wamid into the repo — 2.1's regression test builds one from the reserved test number `972500000000` instead. Do the same for any new fixture.
- **Test fixtures hide leaks.** No review layer caught the wamid leak because the test wamids were arbitrary strings. Task 6's log-discipline test therefore uses a *realistic* MSISDN and a *realistic* wamid shape, not `"wamid.test"`.
- **Captured-slog testing is the house pattern** — `newTestLogger()` returning `slog.New(slog.NewTextHandler(&buf, nil))` plus a buffer, injected through the constructor. Every constructor in `wa` takes an optional `*slog.Logger`; `NewInboundRouter` follows.
- **Concurrency + test buffers:** `dispatch_test.go` already needed a mutex-wrapped `syncBuffer` because worker goroutines and the polling test goroutine race on a plain `bytes.Buffer`. Reuse it for anything asserting on dispatcher logs. `go test -race` is **not available in this environment** (no cgo/gcc) — reason about races rather than expecting the detector to catch them.
- **Store discipline:** `store` is the only pgx importer; `wa` imports stdlib + `x/time/rate` only and receives its dependencies as interfaces. This story adds no store call at all.
- **Windows/toolchain tax:** Go 1.26.5 via the `go1.26.5` wrapper (system `go` is 1.22); `make` needs Git Bash; transient `VirtualAlloc errno=1455` clears on re-run; all new files lowercase; write files with the Write tool (PowerShell emits UTF-16); Hebrew must never pass through Git Bash `curl -d`.
- **E2E leftovers:** the dev DB carries `e2e-org-a`/`e2e-org-b` organizers from Epic 1 — irrelevant here (no session surface), but clean up any `wa_inbound_messages` rows this story's runs create.
- **Infrastructure stories carry heavy review load** (Epic 1 retro insight #1; 2.1 took 8 patches across two rounds). Likely review targets here: the shutdown reordering, the bidi composition, goroutine lifecycle around `Drain`, and whether every branch really does reply.

### Architecture guardrails (violations = rework)

- **Canonical filenames** (architecture directory tree): `wa/inbound.go` + `wa/inbound_test.go` and `wa/messages_he.go` — exactly these names, exactly this package. They are the two files the tree reserves for this story.
- **Copy centralization is an Enforcement rule, not a preference**: "Route every outbound WhatsApp string through `messages_he.go`" — a Hebrew literal in a handler is a named anti-pattern. Task 7 turns it into CI.
- **Dependency direction is law**: `wa` imports stdlib + `x/time/rate`; `httpapi` does not import `wa`; `main.go` composes. No `store` import from `wa`.
- **Logging (NFR-8)**: slog JSON, canonical keys — `wa_message_id` (digested), `phone_last4`, plus this story's `kind`. INFO for lifecycle, WARN for every degradation (missing sender, empty enqueue args, queue full, send retry/failure), ERROR never on a healthy run. The only phone-number log key that may exist is `phone_last4`.
- **PII (NFR-4)**: full MSISDNs live in memory and in the outbound HTTP request only — never in logs, never in error strings propagated upward.
- **Glossary**: `Participant`, `Game`, `Organizer` verbatim where they appear; `host`/`player`/`quiz`/`session` are forbidden as identifiers. Transport vocabulary (`InboundMessage`, `wa_message_id`, `inboundKind`) is consistent with 2.1's precedent.
- **UX-DR15 / EXPERIENCE.md Voice and Tone**: warm-playful, short, first-three-words-first, gender-neutral (past-2nd-singular and plural imperatives — never "ברוך הבא"), symbolic emoji only (🎉 🏆 ⚡ ✓), at most one per message. The Help row as written carries no emoji — do not add one.

### Latest tech intelligence

- **Graph API v25.0 stays pinned.** Verified live against the real API on 2026-07-30 (2.1's Meta E2E succeeded against `https://graph.facebook.com/v25.0`); v26.0 is expected ~Sep 2026. No version bump is in scope, and a fresh web check on 2026-08-02 turned up only stale third-party pages quoting older versions — the project's own live run is the stronger evidence. Leave `defaultBaseURL` alone; Task 5 only makes it *overridable from the environment*, it does not change the default.
- **Service-window economics unchanged**: replies inside the 24-hour service window are free and exempt from tier messaging limits. Every reply this story sends is a service reply to a user-initiated message, so it is inside the window by construction — the ≈₪0 assumption holds and no template-message approval is involved.
- **Per-recipient pair throttle** (~1 msg/6s sustained to the same recipient) — see the design decision above; accepted, no client-side limiter.
- **cloudflared on this network**: `--protocol http2` is required (UDP/7844 blocked); `winget` hangs, fetch the binary from GitHub releases; the quick-tunnel URL changes every run, so the Meta callback URL needs re-verifying each session (~30 seconds).

### Project Structure Notes

**New:** `server/internal/wa/inbound.go` · `server/internal/wa/inbound_test.go` · `server/internal/wa/messages_he.go`.

**Modified:** `server/internal/wa/dispatch.go` (+ `dispatch_test.go`) — empty-arg guard, `Drain` · `server/cmd/server/main.go` — stub removed, router wired, dispatcher lifecycle · `server/internal/config/config.go` (+ `config_test.go`) — `WHATSAPP_API_BASE_URL` · `server/.env.example` · `.github/workflows/ci.yml`.

**Untouched:** `server/internal/wa/webhook.go` and `client.go` (the seam and the transport are already correct) · all of `store/`, `httpapi/`, `migrations/` (no schema change) · all of `web/` (zero frontend diff) · `deps.go`.

**Variances from the canonical architecture tree (documented):** (1) `messages_he.go` ships with one row rather than the full templates table — rationale in Design Decisions. (2) `WHATSAPP_API_BASE_URL` is a new env var the architecture's variable list does not name; it is test-architecture item T-1 and is optional/unset in production.

### Testing standards

Go stdlib `testing`, co-located `_test.go`, stubs grown in-test (no mock framework), `*slog.Logger` injected for log assertions. Deterministic only: no `time.Sleep` assertions, no live-network calls in `go test` (`Drain`'s test polls a real short ticker — keep its bound in the tens of milliseconds). No real-DB tests in CI (1.1–1.5 posture); this story touches no DB path anyway. Frontend unchanged, gates still run. Test-design trace: **A7** (Universal Reply matrix, INT, P0, `ASR-3`) is this story's named obligation and is satisfied by Task 6's table-driven matrix plus Task 8's local E2E.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-2.2] — story + ACs verbatim, Epic 2 context
- [Source: _bmad-output/planning-artifacts/epics.md#Functional-Requirements] — FR-2 (Universal Reply, no class silent), NFR-4, NFR-8
- [Source: _bmad-output/planning-artifacts/epics.md#UX-Design-Requirements] — UX-DR15 (warm-playful, gender-neutral, emoji discipline, copy centralized in `messages_he.go`)
- [Source: ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#WhatsApp-message-templates] — the canonical Help row copy (verbatim source for Task 1)
- [Source: ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#WhatsApp-conversation-grammar] — "Anything else (incl. media, stickers, empty) → Help" across every game state; the grammar matrix this story's default branch implements
- [Source: ux-designs/ux-whatsapp-clickers-2026-07-05/DESIGN.md#Typography] — bidi isolation mandatory in `messages_he.go` templates
- [Source: _bmad-output/planning-artifacts/architecture.md#Communication-Patterns] — Hebrew copy centralized; slog canonical keys and level discipline
- [Source: _bmad-output/planning-artifacts/architecture.md#Enforcement-Guidelines] — route every outbound WhatsApp string through `messages_he.go`; the five gates before story completion
- [Source: _bmad-output/planning-artifacts/architecture.md#Complete-Project-Directory-Structure] — `wa/inbound.go`, `wa/inbound_test.go`, `wa/messages_he.go` placement
- [Source: _bmad-output/planning-artifacts/architecture.md#Requirements-to-Structure-Mapping] — "Universal Reply (FR-2): `wa/inbound.go` default branch — every parse path ends in a reply"
- [Source: _bmad-output/implementation-artifacts/2-1-whatsapp-webhook-intake-and-outbound-dispatch.md] — the seam (`InboundHandler`, `Deduper`, `SenderClient`), wamid/PII lessons, captured-slog test pattern, Windows tax, live Meta E2E procedure
- [Source: _bmad-output/implementation-artifacts/deferred-work.md#story-2.1] — empty recipient/body guard, revisit trigger "2.2's first `Enqueue` caller" (closed by Task 3)
- [Source: _bmad-output/test-artifacts/test-design-progress.md#Coverage-Matrix] — A7 Universal Reply matrix (INT, P0, ASR-3)
- [Source: _bmad-output/test-artifacts/test-design-architecture.md] — T-1 injectable `WHATSAPP_API_BASE_URL` (Task 5)
- [Source: server/internal/wa/webhook.go] — `InboundMessage` shape, synchronous handler call, `TextBody` only for `type == "text"`
- [Source: server/internal/wa/dispatch.go] — `Enqueue`/`Run` contract, retry table, `init()` assertion
- [Source: server/cmd/server/main.go] — current stub + dispatcher lifecycle (both replaced by Task 4)
- [Source: README.md#Meta-WhatsApp-Business-setup] — the runbook and cloudflared workflow used by Task 8's live E2E

## Dev Agent Record

### Agent Model Used

Claude Opus 5 (claude-opus-5), via the bmad-dev-story workflow, 2026-08-02.

### Debug Log References

- **Local E2E script** (Python/stdlib, 1.5 + 2.1 pattern): `e2e_2_2.py` in the session scratchpad, not committed. Boots the real server binary against the Docker dev Postgres with `WHATSAPP_API_BASE_URL` pointed at a ~40-line fake Cloud API provider, drives one signed webhook per inbound class, and asserts the reply was actually *sent*. **31/31 checks passed.**
- **Negative control for the shutdown fix** — the drain assertions would be worthless if they passed either way, so `main.go` was temporarily reverted to the 2.1 wiring (dispatcher on the signal ctx, no `Drain`) and the same E2E re-run: **29/31**, with a 40-message burst arriving during shutdown delivering only **14**, logging **10** as `dropped_count`, and **16 vanishing with no log line at all** — exactly the black-hole the deferred item described. Restored and re-verified green.
- **Copy-fidelity chain verified independently of the Go test.** `TestHelpMessageMatchesCanonicalCopy` compares `helpMessage()` against a constant in the same test file, so a correlated bidi mistake in both literals would pass. A scratchpad script (`verify_copy_fidelity.py`) closes the loop: it extracts the Help row straight from EXPERIENCE.md, the `canonicalHelpCopy` const from `inbound_test.go`, and `msgHelpTemplate` from `messages_he.go`, then compares all three byte-for-byte. All MATCH; canonical sha256 `ae51aedf05d916c88793cd3ce97a0a050989e8deb6997804d76ca9075ec1780b`, 98 runes, 159 UTF-8 bytes.
- **gofmt on Windows**: `core.autocrlf=true` gives the working tree CRLF, and `gofmt -l .` reports *every* file as unformatted — noise, not a finding. The real gate ran via a scratchpad script that LF-normalizes each file in memory before piping it through gofmt (48 files, clean), which is what Linux CI checks out.
- **E2E ledger rows** cleaned afterwards; `SELECT count(*) FROM wa_inbound_messages` → 0.
- **Live Meta E2E, 2026-08-02 15:14–15:24** (with Avraham). Tunnel `operations-appreciate-obituaries-pound.trycloudflare.com` (`--protocol http2`; the precheck again reported `UDP Connectivity … FAIL`, `TCP Connectivity … HTTP/2 … pass`). Server on :8080 with the real `.env` and **no** `WHATSAPP_API_BASE_URL` — the genuine Graph API client. Live results: Meta's verification handshake passed; two Hebrew text messages → `kind=text`; one sticker → `type=sticker`, `kind=non_text`; **all three replies delivered to Avraham's phone**, and he confirmed the example code renders left-to-right and unscrambled (the one thing no automated check can prove — the tests only prove U+2066/U+2069 sit in the right byte positions). Log audit: **0 ERROR, 0 send retries, 0 permanent failures, 0 queue drops**; `9221` appears only ever as `phone_last4`; zero raw `wamid.` values (only `wamid_…` digests); every 7+ digit run in the log traced to a timestamp fraction or the digest hex tail. The three live ledger rows were deleted afterwards.
- **Discovery that removes this story's only blocked step from future runs.** The README (and Task 8) route the per-session callback-URL update through the Meta console, which needs a browser on a mobile hotspot because the dashboard domains are filtered on this machine. That is unnecessary: the callback URL can be set over the Graph API with an **app access token** (`<APP_ID>|<APP_SECRET>`) — `POST /v25.0/<APP_ID>/subscriptions` with `object=whatsapp_business_account`, `callback_url`, `verify_token`, `fields=messages`. It fires the same GET handshake and returns `{"success":true}` only when the handshake passes. `graph.facebook.com` is reachable from the dev box via PowerShell/Go (only the *browser dashboard* is filtered), so a live E2E now needs no hotspot and no console session at all. Also re-confirmed read-only: the app `Clickers` is still subscribed to the WABA alongside `WA DevX Webhook Events 1P App`, so README step 7 does not need repeating.
- **Log-reading trap worth remembering**: a successful verify handshake writes **no** log line (only rejections are logged), so "no line" is the success signal. A `verify handshake rejected / verify token mismatch` WARN appeared mid-run and looked like Meta failing — it was this session's own deliberate wrong-token probe. Confirmed by re-POSTing the subscription and observing that no *new* WARN appeared while Meta returned `success:true`.

### Completion Notes List

**All 8 tasks complete and verified, including the live Meta E2E (AC-6), which ran with Avraham on 2026-08-02.** Every AC is satisfied; Status: review.

- **Task 1** — `messages_he.go` created with exactly one row (Help). The Hebrew is a single contiguous literal with two `%s` placeholders; `helpMessage()` composes it via `fmt.Sprintf` with `ltr()` wrapping each LTR run in U+2066/U+2069. Isolates are `\u` escapes, never pasted characters — enforced in both the source and the test by a scripted pass after authoring, since invisible characters cannot be typed reliably into an edit. The file doc comment carries the full 19-row template map with each row's owning story.
- **Task 2** — `inbound.go` with the totality structure: `inboundKind` + `allInboundKinds` registry, a total `classify` (Type first, because `webhook.go` only populates `TextBody` for `type == "text"` — a sticker always has an empty body and calling that "empty" would conflate two classes), an exhaustive `replyFor` with a still-replying `default`, and `Handle` enqueuing exactly one reply on every path. `var _ InboundHandler = (*InboundRouter)(nil)` pins the seam.
- **Task 3** — `Enqueue` drops empty `to`/`body` with a WARN before the queue; `Drain(ctx)` polls queue depth on a 50ms ticker until empty or ctx done. Both 2.1 deferred items whose revisit trigger was "2.2's first `Enqueue` caller" are now closed in `deferred-work.md`; the third (typed permanent-error classification) stays open with a **re-pointed trigger**, since the original one has now fired and would otherwise read as still-pending.
- **Task 4** — `stubInboundHandler` deleted; `inboundRouter` wired. Shutdown reordered: the dispatcher gets a `context.Background()`-derived context, and the sequence is `srv.Shutdown` → `Drain(5s)` → cancel → join `Run` with a 2s grace. Three named constants (`httpShutdownTimeout`/`dispatcherDrainTimeout`/`dispatcherStopGrace`) keep the total inside Railway's SIGTERM window. The negative control above is the evidence this was load-bearing rather than cosmetic.
- **Task 5** — `WHATSAPP_API_BASE_URL` added as the one optional WhatsApp var (no `require()`, no placeholder validation, TrimSpace'd like the rest); `main.go` reaches `wa.WithBaseURL` only when it is non-empty and logs that the override is active. 3 new config tests. Without this the E2E could only prove the webhook returned 200, not that a reply was sent.
- **Task 6** — 9 new tests in `inbound_test.go` (16-row Universal Reply matrix, registry totality, unregistered-kind default, classify membership, copy fidelity, bidi placement, empty sender, log discipline, both interface assertions) and 3 in `dispatch_test.go` (empty-arg drop across 3 sub-cases, `Drain` empties, `Drain` gives up on ctx). The log-discipline test reuses 2.1's `syntheticMSISDN`/`leakyWamid` fixtures on purpose — 2.1's wamid leak survived review precisely because synthetic fixtures hid it.
- **Task 7** — CI gains a `Hebrew copy centralization` step in the `backend` job. **Deviation from the task text, deliberate:** the spec'd one-liner pipes three greps, so the pipeline's exit status comes from the *last* grep and a genuine error in the first (exit 2) would be reported as "clean". The step splits the scan from the exclusions and checks the scan's status explicitly. It also exports `LC_ALL=C.UTF-8` — without it `grep -P` refuses the codepoint range ("supports only unibyte and UTF-8 locales"), which is exactly the failure the status guard caught when the grep was first run locally.
- **Task 8** — all local gates green: gofmt (LF-normalized, 48 files), `go vet ./...`, `go test ./...` (zero regressions across auth/config/httpapi/store/wa), `sqlc generate` (content-identical; the only diff was CRLF↔LF churn), eslint, `tsc -b --noEmit`, `npm run build`, both filter-safety greps, and the new Hebrew grep (only `*_test.go` and `messages_he.go` carry Hebrew). Local E2E 31/31. The Live Meta E2E subtask ran and passed — see the 2026-08-02 15:14–15:24 entry below and the final Change Log entry.
- **Scope held**: no `game` package, no `participants` table, no JOIN/`שם:` parsing (both are pinned as *unrecognized text* by name in the matrix test so a later reader does not mistake it for a bug), no WebSocket work, zero `web/` diff, no new Go or npm dependencies.

**Open item for whoever picks this up next (not blocking):** the Meta app's `callback_url` still points at this session's now-dead quick-tunnel URL — the same state it was in before this story started (it held 2.1's dead tunnel URL). Any future live run overwrites it in one API call; no cleanup is owed.

### File List

**New:**
- `server/internal/wa/messages_he.go`
- `server/internal/wa/inbound.go`
- `server/internal/wa/inbound_test.go`

**Modified:**
- `server/internal/wa/dispatch.go` (empty-arg guard, `Drain`, `drainPollInterval`)
- `server/internal/wa/dispatch_test.go` (3 new tests)
- `server/cmd/server/main.go` (stub removed, router wired, dispatcher lifecycle + shutdown ordering, base-URL override)
- `server/internal/config/config.go` (`WhatsAppAPIBaseURL`)
- `server/internal/config/config_test.go` (3 new tests)
- `server/.env.example` (documents the optional override)
- `.github/workflows/ci.yml` (Hebrew copy-centralization gate)
- `_bmad-output/implementation-artifacts/deferred-work.md` (2 items closed, 1 re-pointed)
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (2.2 → review)

## Change Log

- 2026-08-02: Story created by create-story workflow — full-context analysis: epics 2.2 + Epic 2 cross-story context (2.1 shipped, 2.3/2.4/2.5 scope boundaries), FR-2/NFR-4/NFR-8, UX-DR15, EXPERIENCE.md templates table + conversation-grammar matrix, DESIGN.md bidi rule, architecture patterns/enforcement/structure/FR-mapping, story 2.1 (all tasks, review findings, 8 patches, live-Meta learnings), deferred-work 2.1 items, test-design A7 + T-1, and a live read of `webhook.go`, `dispatch.go`, `client.go`, `redact.go`, `main.go`, `router.go`, `config.go`, `go.mod`, the `wa` test suite, `Makefile`, and `ci.yml`. Status: ready-for-dev.
- 2026-08-02: Tasks 1–7 implemented and verified, plus Task 8's local gates and local E2E (`messages_he.go` with the bidi-isolated Help row; `inbound.go` totality taxonomy + Universal Reply router; dispatcher empty-arg guard and `Drain`; `main.go` stub removal, router wiring and shutdown reordering; optional `WHATSAPP_API_BASE_URL`; 12 new Go tests; CI Hebrew copy-centralization gate). All local gates green, local E2E 31/31 including a negative control proving the shutdown fix is load-bearing (pre-fix: 14 of 40 replies delivered, 16 lost silently). Two 2.1 deferred items closed, one re-pointed. Task 8's "Live Meta E2E with Avraham" (AC-6) left unchecked pending his participation.
- 2026-08-02: **Live Meta E2E completed with Avraham — story complete.** Callback URL re-pointed at a fresh cloudflared tunnel **over the Graph API** rather than the Meta console (app access token → `POST /<APP_ID>/subscriptions`), which removes the mobile-hotspot/browser step the README documents; Meta's verification handshake passed against the running server (AC-1 live). Two Hebrew texts → `kind=text` and one sticker → `kind=non_text`, all three replies delivered to his phone, and he confirmed the bidi-isolated example code renders left-to-right and unscrambled (AC-6). Log audit clean: 0 ERROR, 0 send retries or failures, phone only ever as `phone_last4`, no raw wamid. Live ledger rows cleaned. **Status: review.**
- 2026-08-02: **Code review (3 parallel adversarial layers) — 12 findings, 10 patched, 2 deferred by user decision, 1 confirmed a false positive and dismissed.** No AC violations found. Patched: dispatcher drain now runs on every shutdown exit including error paths, not just the clean one; the "server stopped cleanly" INFO is gated on a clean dispatcher join; Railway's zero-second default SIGTERM grace documented with an ops note requiring `RAILWAY_DEPLOYMENT_DRAINING_SECONDS` to be set on the service; whitespace-only recipient/body/sender now rejected by the Enqueue and Handle guards (TrimSpace); totality-enforcement comments reworded to what the registry test actually proves; CI Hebrew gate anchored its exemption path and widened its codepoint range to catch Alphabetic Presentation Forms; two stale story-doc lines corrected. Deferred (user decision): auto-responder loop damping (Meta test-mode prevents it today; needs its own story before the number leaves test-mode) and the `WHATSAPP_API_BASE_URL` validation gap (spec-pinned as intentional, test-harness-only). Full diff re-verified: `go build`/`go vet`/`go test ./...` all green, CI Hebrew gate re-run locally clean. **Status: done.**
