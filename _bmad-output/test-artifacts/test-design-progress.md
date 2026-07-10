---
workflowStatus: 'completed'
totalSteps: 5
stepsCompleted: ['step-01-detect-mode', 'step-02-load-context', 'step-03-risk-and-testability', 'step-04-coverage-plan', 'step-05-generate-output']
lastStep: 'step-05-generate-output'
nextStep: ''
lastSaved: '2026-07-09'
inputDocuments:
  - _bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md
  - _bmad-output/planning-artifacts/architecture.md
  - _bmad-output/planning-artifacts/epics.md
  - _bmad/tea/config.yaml
  - knowledge/risk-governance.md
  - knowledge/probability-impact.md
  - knowledge/test-levels-framework.md
  - knowledge/test-priorities-matrix.md
  - knowledge/test-quality.md
  - knowledge/nfr-criteria.md
  - knowledge/adr-quality-readiness-checklist.md
  - knowledge/webhook-risk-guidance.md
---

# Test Design Progress

## Step 1: Mode Detection

- **Mode selected:** System-Level
- **Rationale:** PRD (`_bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md`), architecture (`_bmad-output/planning-artifacts/architecture.md`), and epics (`_bmad-output/planning-artifacts/epics.md`) all exist; no `sprint-status.yaml` found (implementation not started), so file-based detection selects System-Level. Rule "Both PRD/ADR + Epic/Stories → Prefer System-Level first" also applies.
- **Prerequisites confirmed:** PRD ✓, Architecture (with decision records) ✓, Epics/stories ✓ (26 stories), UX design docs available as supporting context.

## Step 2: Context Loaded

- **Config:** `tea_use_playwright_utils: true`, `tea_use_pactjs_utils: false`, `tea_pact_mcp: none`, `tea_browser_automation: auto`, `test_stack_type: auto`, `test_artifacts: _bmad-output/test-artifacts`, `test_design_output: _bmad-output/test-artifacts/test-design`.
- **Detected stack:** `fullstack` — from architecture (greenfield, no code yet): Go 1.26 backend (chi v5, coder/websocket, pgx v5, sqlc, goose) + React 19/TypeScript SPA (Vite, shadcn/ui, TanStack Query v5), PostgreSQL on Railway, single-instance deployable. Go tests via stdlib `testing`; web tests via Vitest; E2E candidate: Playwright.
- **Artifacts loaded:** PRD (18 FRs, 6 feature groups, SM-1–SM-6 + counter-metrics), architecture (all decisions versioned, 18/18 FR coverage validated, 3 tracked gaps), epics (4 epics, 26 stories).
- **Key extracts:** NFR targets — 1s display transitions, 3s lobby count, 5s/95% WA delivery, zero lost acked answers (SM-4), 80 msg/s provider ceiling; external deps — Meta Cloud API (webhook in / REST out), Claude API (degradable, fail-closed); integration risk points — webhook dedupe/idempotency, persist-before-ack, single server-side cutoff, speed-bonus ordering, Hebrew normalization.
- **Open threshold questions:** none blocking — PRD assumption-tagged targets (3s/5s/1s) treated as binding; Levenshtein threshold value undefined (flagged for risk assessment); default question time limit TBD (story 1.3 assumes 30s).
- **Browser exploration:** skipped — greenfield, no running app to explore.
- **Knowledge fragments:** core five for system-level + probability-impact, test-priorities-matrix, webhook-risk-guidance (system is webhook-driven).

## Step 3: Testability Review & Risk Assessment

### 🚨 Testability Concerns (ACTIONABLE)

1. **T-1 — Meta Cloud API base URL must be injectable.** The architecture defines `wa/client.go` but does not state that the Cloud API base URL is configurable. Without it, no integration/E2E test can substitute a fake WhatsApp provider, and the entire messaging path (dispatch pacing, retries, acks) is untestable below manual level. **Requirement:** `WHATSAPP_API_BASE_URL` env var (defaulting to Meta's); a local fake-provider stub becomes the backbone of integration and perf testing.
2. **T-2 — Engine needs an injectable clock.** The single server-side cutoff (FR-7), Speed Bonus ordering (FR-17), and token-bucket pacing all depend on time. A clock interface injected into `game.Engine` and `wa/dispatch.go` is required for deterministic boundary tests (answer at cutoff-1ms vs cutoff+1ms) without real sleeps.
3. **T-3 — AI stage needs an interface seam.** `grading/ai.go` must sit behind a small interface so the pipeline can be tested with scripted verdicts, timeouts, and errors (fail-closed path, FR-16). Real-Claude calls belong in a tiny opt-in smoke test, not the suite.
4. **T-4 — Reveal-gate needs defined escape semantics.** "Reveal activates only once every received answer is graded" has no documented behavior for a grading task that never completes (crash, counter leak). Untestable as specified and a live-event deadlock hazard (see R-002). **Requirement:** hard per-answer grading deadline after which the answer falls back to two-stage grade and the gate opens.
5. **T-5 — No seeding path for bulk participants.** Joining 80 participants for perf/E2E tests means 80 simulated webhook POSTs — acceptable, but requires the fake-provider harness (T-1) plus a test helper that generates valid HMAC-signed webhook payloads. Should be built once, early (Epic 2).
6. **T-6 — Real-WhatsApp validation is manual by nature.** Automated coverage necessarily stops at the webhook/client seam; Meta's actual delivery behavior (throughput, template rules, service window) can only be validated with the real number. **Requirement:** a scripted dress-rehearsal protocol (small real-device game) before the first pilot event — a planned test artifact, not an afterthought.

### ✅ Testability Assessment Summary (strengths)

- **Headless by construction:** every behavior is reachable via REST (organizer), webhook POST (participant), or WS (observers) — no UI-only logic.
- **Server-authoritative + DB-rebuildable state:** assertions can target Postgres truth; restart-recovery is directly testable.
- **Full-snapshot WS protocol:** one payload to assert; reconnect semantics (snapshot-on-connect, stale-seq drop) are cheap to test.
- **Deterministic tie-breaking:** monotonic sequence removes timestamp-collision flakiness from scoring tests.
- **Observability as invariant:** "healthy run produces zero ERRORs" and "every degradation logs WARN" are themselves automatable assertions.
- **Pure-Go grading stages:** normalization + Levenshtein are pure functions — ideal unit-test targets with a Hebrew golden corpus.
- **Filter-safety is CI-greppable** by design.

### ASRs (Architecturally Significant Requirements)

| ID | ASR | Type |
|---|---|---|
| ASR-1 | SM-4: acknowledged answer never lost (persist-before-ack + dedupe + UNIQUE) | ACTIONABLE |
| ASR-2 | Single server-side answer cutoff (FR-7) | ACTIONABLE |
| ASR-3 | Universal Reply — every inbound class gets a response (FR-2) | ACTIONABLE |
| ASR-4 | Idempotency: JOIN re-send + Meta webhook retry dedupe | ACTIONABLE |
| ASR-5 | Speed Bonus ordering: receipt timestamp + monotonic seq, shared ranks (FR-17) | ACTIONABLE |
| ASR-6 | Fail-closed AI degradation; Reveal gated on grading completion (FR-16) | ACTIONABLE |
| ASR-7 | Delivery: 5s/95% at 80 participants under 80 msg/s ceiling (FR-4) | ACTIONABLE |
| ASR-8 | WS snapshot reconnect: snapshot-on-connect, stale-seq drop, ≤1s transitions (FR-9) | ACTIONABLE |
| ASR-9 | Server restart mid-game recovers full state from Postgres (NFR-2) | ACTIONABLE |
| ASR-10 | Hebrew normalization correctness (finals, nikud, punctuation) | ACTIONABLE |
| ASR-11 | Filter-safety: zero external asset references outside index.html/index.css | ACTIONABLE |
| ASR-12 | Single-instance deployment (replicas=1) — in-memory hub assumption | FYI (config assertion) |
| ASR-13 | Ownership scoping: every game/question query scoped by organizer_id | ACTIONABLE |
| ASR-14 | PII redaction: phone last-4 only in logs (NFR-4) | ACTIONABLE |

### Risk Register

Scoring per probability-impact scale (P1 unlikely / P2 possible / P3 likely × I1 minor / I2 degraded / I3 critical). Owner default: Avraham (solo pilot); "dev" = implementation stories.

| ID | Cat | Risk | P | I | Score | Action |
|---|---|---|---|---|---|---|
| R-001 | DATA | Lost/duplicated answer under webhook retry + concurrency (SM-4 breach: ack sent, answer missing from leaderboard) | 2 | 3 | **6** | MITIGATE |
| R-002 | TECH | Reveal-gate deadlock: outstanding-grades counter never reaches zero (hung AI call, task crash) → "גלה תשובה" never activates mid-event | 2 | 3 | **6** | MITIGATE |
| R-003 | PERF | WhatsApp delivery misses 5s/95% at 80 (provider variance); known ~7s at 500 | 2 | 2 | 4 | MONITOR |
| R-004 | TECH | Cutoff boundary wrong (deadline comparison, clock source, close-vs-expiry race) → answers wrongly rejected/counted → scoring dispute (SM-1 failure class) | 2 | 3 | **6** | MITIGATE |
| R-005 | BUS | Wrong free-text grades: undefined Levenshtein threshold, Hebrew normalization bugs, AI equivalence misjudgment (SM-C1 both directions) | 3 | 2 | **6** | MITIGATE |
| R-006 | SEC | Webhook forgery/replay if HMAC verification flawed → fake joins/answers (cheating) | 1 | 3 | 3 | DOCUMENT |
| R-007 | SEC | Ownership-scoping miss (IDOR) → another organizer reads games or participant phone numbers (PII leak) | 2 | 3 | **6** | MITIGATE |
| R-008 | OPS | Railway config drift → more than 1 replica → split in-memory hub, missed fan-out mid-event | 1 | 3 | 3 | DOCUMENT |
| R-009 | OPS | Restart/deploy/migration mid-event: recovery bug or boot-time migration failure during a live game | 2 | 2 | 4 | MONITOR |
| R-010 | TECH | WS reconnect edge cases: display blank at winner moment, stale seq applied, proxy idle timeout | 2 | 2 | 4 | MONITOR |
| R-011 | BUS | Meta 24h service-window/pricing assumption wrong → mid-game template requirement or cost surprise | 1 | 3 | 3 | DOCUMENT (re-verify at account creation — already tracked) |
| R-012 | TECH | RTL/bidi rendering defects (mixed Hebrew + digits/letters in MCQ options, WhatsApp message formatting) | 2 | 1 | 2 | DOCUMENT |
| R-013 | DATA | Participant identity edge cases: profile name empty/emoji-only, number format variants, one person two devices | 2 | 2 | 4 | MONITOR |
| R-014 | PERF | Last-second free-text burst → N parallel Claude calls (rate limit / 5s timeouts stack) → Reveal visibly delayed | 2 | 2 | 4 | MONITOR |
| R-015 | SEC | PII leak in logs via non-canonical paths (webhook payload dumps, error contexts) | 2 | 2 | 4 | MONITOR |
| R-016 | OPS | Backups never restore-tested before pilot | 1 | 2 | 2 | DOCUMENT (pre-pilot drill) |
| R-017 | SEC | CSRF on cookie-authenticated organizer control actions (SameSite not specified in architecture) | 1 | 2 | 2 | DOCUMENT (specify SameSite=Lax + verify) |

**High risks (score 6) and mitigation focus:**

- **R-001 (SM-4):** integration tests proving persist-before-ack ordering; dedupe-by-message-ID tests (duplicate webhook POST); UNIQUE-constraint race test (two answers, same participant, parallel); restart-recovery E2E. Owner: dev (Epics 2–3). Timeline: with stories 2.1/3.3.
- **R-002 (Reveal gate):** define escape semantics (T-4) before story 3.4; unit tests on outstanding-grades tracker including task-death; timeout-path integration test. Owner: dev. Timeline: stories 3.4/3.6.
- **R-004 (cutoff):** injectable clock (T-2); boundary unit tests at cutoff±ε; close-action-vs-timer-expiry race test. Owner: dev. Timeline: stories 3.1/3.3.
- **R-005 (grading quality):** Hebrew golden corpus (exact/fuzzy/AI-positive/AI-negative cases) as a versioned fixture; calibrate and pin the Levenshtein threshold with tests; stage-recording assertions for SM-C1 audit. Owner: dev + Avraham (corpus content). Timeline: stories 3.4–3.6.
- **R-007 (IDOR):** authz test per route (foreign game ID → GAME_NOT_FOUND), including `/ws` and results; PII exposure assertions. Owner: dev. Timeline: story 1.3 onward, every new route.

### NFR Planning Assessment

| Category | Thresholds (source) | Status | Planned evidence |
|---|---|---|---|
| Performance | 1s display transition; 3s lobby count; 5s/95% delivery at 80; 80 msg/s ceiling (PRD/NFR-1, arch) | Defined | Perf harness vs fake provider (T-1) at 80 + 500 simulated participants; WS-latency assertions in E2E; dispatch pacing unit tests with fake clock |
| Reliability | SM-4 zero lost answers; restart recovery; auto-reconnect; fail-closed AI; zero ERRORs on healthy run (NFR-2, NFR-8) | Defined | R-001/R-002 test sets; kill-and-recover E2E; log-level assertions |
| Security | HMAC on webhooks; argon2id + server sessions; ownership scoping; env-only secrets; PII redaction (NFR-4, NFR-7) | Defined, 2 gaps | Authz suite; signature tests; cookie-flag checks; log-redaction tests. **UNKNOWN:** session expiry duration; SameSite attribute (R-017) — clarify, do not guess |
| Scalability | 30–80 now; must-not-preclude 500 (NFR-3) | Defined | 500-participant simulation vs fake provider before commercial phase (deferred, documented) |
| Maintainability | CI gates: gofmt, vet, sqlc diff, eslint, tsc (arch) | Defined | CI pipeline itself + test-quality DoD (deterministic, isolated, <300 lines, <1.5 min) |
| Compliance/Privacy | Minimal PII; retention policy deferred to commercial (PRD §4.7) | Partially UNKNOWN | Pilot: redaction tests only; retention/deletion policy is a tracked pre-commercial item, not a pilot test target |
| Hebrew/filter-safety | Zero external assets; system fonts; RTL everywhere (NFR-5) | Defined | CI grep test (ASR-11); RTL visual review checklist at dress rehearsal |

**UNKNOWN thresholds converted to clarifications:** Levenshtein threshold value (→ R-005), session expiry (→ security plan), default per-question time limit (story 1.3 assumes 30s — confirm), AI-call concurrency cap (→ R-014).

**ADR Quality Readiness snapshot (8 categories):** Testability 2/4 ⚠️ (T-1..T-4 requirements), Test Data 2/3 ✅ (synthetic-only; no prod data exists), Scalability/Availability 2/4 ⚠️ (stateful-by-design accepted; no availability SLA — event-time availability is the real requirement), DR 0/3 ⚠️ (pilot-appropriate; pre-pilot restore drill — R-016), Security 3/4 ⚠️ (CSRF/SameSite gap — R-017), Monitorability 2/4 ⚠️ (logs-only, no metrics endpoint — acceptable for pilot, revisit at commercial), QoS/QoE 4/4 ✅, Deployability 1/3 ⚠️ (single-instance restart downtime accepted; **"no deploys during live events" runbook required**).

## Step 5: Outputs Generated & Validated

- **Execution mode:** sequential (config `tea_execution_mode: auto`; parallel agents not user-requested in this session).
- **Outputs written:**
  - `_bmad-output/test-artifacts/test-design-architecture.md` — concerns contract for Architecture/Dev (blockers T-1–T-4, risk register R-001–R-017, NFR testability, mitigation plans)
  - `_bmad-output/test-artifacts/test-design-qa.md` — QA execution recipe (~63 scenarios A1–L3, P0–P3, NFR evidence plan, PR/Nightly/Weekly strategy, ~75–120 h)
  - `_bmad-output/test-artifacts/test-design/whatsapp-clickers-handoff.md` — BMAD handoff (risk-to-story mapping onto the existing 26 stories, scenario→AC mapping, data-testid list, phase gates)
- **Checklist validation:** passed. One correction applied during validation: risk band counts fixed to 5 high / 9 medium / 3 low (score-3 risks moved to the medium table).
- **Key open assumptions:** PRD assumption-tagged latency targets treated as binding; 30s default time limit (story 1.3) unconfirmed; Levenshtein threshold, session expiry, SameSite, and AI concurrency cap are UNKNOWN thresholds requiring decisions — tracked as risks/clarifications, not guessed.
- **Immediate decisions needed:** T-4 reveal-gate escape semantics (before story 3.4); approval of ⚠️ items in the architecture doc Quick Guide.

## Step 4: Coverage Plan & Execution Strategy

**Test levels for this stack:** UNIT = Go stdlib `testing` / Vitest (pure logic, fake clock); INT = Go `httptest` + real local Postgres + fake WhatsApp provider (T-1) + scripted AI seam (T-3); COMP = Vitest + Testing Library (display stages, WS hook); E2E = Playwright driving Host Dashboard + Audience Display with participants simulated via signed webhook POSTs (T-5); PERF = load simulation vs fake provider; STATIC = CI greps/config assertions; MANUAL = scripted dress rehearsal (T-6). Duplicate-coverage guard applied: logic proven at UNIT is not re-proven at INT/E2E; E2E covers journeys only.

### Coverage Matrix (scenario groups → level, priority, risk/ASR trace)

**A. WhatsApp intake & Universal Reply (FR-1–3)**
| ID | Scenario | Level | Pri | Trace |
|---|---|---|---|---|
| A1 | Webhook GET handshake answers verify token | INT | P1 | Story 2.1 |
| A2 | Invalid HMAC rejected + logged; valid processed | INT | P0 | R-006, NFR-7 |
| A3 | Duplicate WhatsApp message ID processed exactly once | INT | P0 | ASR-4, R-001 |
| A4 | JOIN valid code in lobby → row + welcome reply; re-send idempotent (no dup, same reply) | INT | P0 | ASR-4, FR-1 |
| A5 | JOIN invalid/expired code → Hebrew not-found reply | INT | P1 | FR-1 |
| A6 | Late JOIN → spectator role, excluded from dispatch, included in final results | INT | P1 | FR-3 |
| A7 | Universal Reply matrix: unrecognized / media / sticker / empty → help reply; **every parse branch replies** (table-driven) | INT | P0 | ASR-3 |
| A8 | Inbound parser units: JOIN forms, MCQ letter/digit, free text, >200 chars, noise | UNIT | P0 | FR-5 |
| A9 | Display-name edge cases (empty/emoji profile), phone format variants | UNIT | P2 | R-013 |
| A10 | JOIN → lobby snapshot broadcast (count ≤3s path) | INT | P1 | NFR-1 |

**B. Answer intake, cutoff & SM-4 (FR-5, 7, 8)**
| ID | Scenario | Level | Pri | Trace |
|---|---|---|---|---|
| B1 | Persist-before-ack: DB insert failure ⇒ no ack sent (fault injection) | INT | P0 | ASR-1, R-001 |
| B2 | Second answer → "already counts" reply, no new row | INT | P0 | FR-8 |
| B3 | Parallel duplicate answers race → UNIQUE holds, exactly one row | INT | P0 | ASR-1, R-001 |
| B4 | Cutoff boundary with fake clock: receipt at deadline−ε accepted, deadline+ε rejected + "השאלה נסגרה" | UNIT+INT | P0 | ASR-2, R-004 |
| B5 | Cutoff = earlier of (open+limit, explicit close); close-vs-expiry race | UNIT | P0 | ASR-2, R-004 |
| B6 | Unparseable MCQ reply → format hint, participant can still answer | INT | P1 | FR-5 |
| B7 | Free text >200 chars rejected with hint | UNIT | P1 | FR-5 |
| B8 | Answered count/percentage updates in snapshot while open | INT | P1 | FR-13 |

**C. Grading pipeline (FR-15, 16)**
| ID | Scenario | Level | Pri | Trace |
|---|---|---|---|---|
| C1 | MCQ mechanical grading; grade stored, never disclosed pre-Reveal | UNIT+INT | P0 | FR-15, FR-6 |
| C2 | Exact match short-circuits (stage=exact; fuzzy/AI never invoked) | UNIT | P0 | FR-16 |
| C3 | Hebrew normalization golden corpus: finals ך/ם/ן/ף/ץ, nikud, punctuation, trim (UJ-3 cases) | UNIT | P0 | ASR-10, R-005 |
| C4 | Fuzzy threshold calibration: 1-letter typo matches, unrelated word doesn't; threshold pinned by tests | UNIT | P0 | R-005 |
| C5 | AI invoked only on exact+fuzzy miss; verdict recorded stage=ai | UNIT (scripted seam) | P0 | FR-16 |
| C6 | AI timeout/error → fail-closed two-stage grade + WARN + stage recorded | INT | P0 | ASR-6, R-005 |
| C7 | Reveal gate: activates only when all received answers graded (straggler case) | INT | P0 | ASR-6 |
| C8 | Reveal-gate escape: hung grading task → fallback per T-4 semantics, gate opens | INT | P0 | R-002 |
| C9 | Last-second burst: ~30 concurrent free-text answers → gate opens within bounded time | PERF | P1 | R-014 |
| C10 | Real-Claude Hebrew smoke (opt-in, ~10 golden cases) | INT-live | P2 | R-005 |

**D. Scoring & Leaderboard (FR-17, 18)**
| ID | Scenario | Level | Pri | Trace |
|---|---|---|---|---|
| D1 | Configured points per correct answer applied | UNIT | P0 | FR-17 |
| D2 | Speed Bonus to first three correct by receipt timestamp; monotonic-seq tie-break | UNIT | P0 | ASR-5 |
| D3 | Fewer than three correct → fewer bonuses; zero-value bonus disabled | UNIT | P0 | FR-17 |
| D4 | Equal scores share rank; winner tie names all tied winners | UNIT | P0/P1 | FR-17, 3.9 |
| D5 | Leaderboard data present in snapshot after Reveal | INT | P1 | FR-18 |

**E. Game engine & live control (FR-13, 14)**
| ID | Scenario | Level | Pri | Trace |
|---|---|---|---|---|
| E1 | State machine: only legal transitions; leaderboard skippable; no auto-advance | UNIT | P0 | FR-13 |
| E2 | Illegal transition rejected with stable error code | UNIT | P0 | FR-13 |
| E3 | REST control action → persisted transition → snapshot broadcast + engine event | INT | P0 | arch flow |
| E4 | Kill server mid-game → restart → full state recovered from Postgres, game continues | E2E | P0 | ASR-9, R-009 |
| E5 | Results summary (final leaderboard + per-question response rates) persists across sessions | INT | P1 | FR-14 |
| E6 | Space = primary CTA, Escape = confirm-stop | E2E | P2 | UX-DR13 |

**F. Realtime / WebSocket (FR-9)**
| ID | Scenario | Level | Pri | Trace |
|---|---|---|---|---|
| F1 | Snapshot-on-connect for role=host and role=display | INT | P0 | ASR-8 |
| F2 | Unauthenticated /ws rejected | INT | P0 | NFR-7 |
| F3 | Stale seq dropped by client hook | COMP | P1 | ASR-8 |
| F4 | Drop connection → auto-reconnect → re-render current state; "מתחבר..." only while down | E2E | P1 | R-010 |
| F5 | State transition renders ≤1s (measured in E2E) | E2E | P1 | NFR-1 |

**G. Outbound dispatch & throughput (FR-4, 6)**
| ID | Scenario | Level | Pri | Trace |
|---|---|---|---|---|
| G1 | Token bucket stays ≤80 msg/s (fake clock) | UNIT | P0 | ASR-7 |
| G2 | Send retry ×3 with backoff; final failure WARN, game loop unblocked | INT | P0 | NFR-2 |
| G3 | Question open → every player receives question (MCQ options included), spectators none | INT | P0 | FR-4 |
| G4 | Post-Reveal: answerers get grade/points/rank; non-answerers silent; nothing sent pre-Reveal | INT | P0 | FR-6 |
| G5 | Game end: final results to all participants + spectators | INT | P0 | FR-6/18 |
| G6 | 80 simulated participants: delivery completes within 5s/95% vs fake provider | PERF | P1 | ASR-7, R-003 |
| G7 | 500 simulated participants: measure pacing (documented ~7s constraint) | PERF | P3 | NFR-3 |

**H. Auth, authz, privacy (FR-11, NFR-4/7)**
| ID | Scenario | Level | Pri | Trace |
|---|---|---|---|---|
| H1 | Login valid/invalid; sessions in Postgres survive restart; logout deletes session | INT | P0 | Story 1.2 |
| H2 | No session → 401 envelope on every /api route | INT | P0 | NFR-7 |
| H3 | IDOR suite: foreign organizer's game/questions/control/results/ws → GAME_NOT_FOUND / denied | INT | P0 | ASR-13, R-007 |
| H4 | Cookie flags: HttpOnly, Secure, SameSite (once specified — R-017) | INT | P1 | NFR-7 |
| H5 | Log redaction: no full phone number in any log line (canonical + error paths) | INT | P1 | ASR-14, R-015 |
| H6 | argon2id hashing/verification | UNIT | P1 | NFR-7 |

**I. Builder & Question Bank (FR-11, 12)**
| ID | Scenario | Level | Pri | Trace |
|---|---|---|---|---|
| I1 | Game CRUD + JOIN code; question add/edit/reorder/delete persists | E2E happy path + INT validation | P1 | FR-11 |
| I2 | MCQ = exactly 4 options, one correct; accepted answers editable | INT | P1 | FR-11 |
| I3 | Package import copies (edit copy ≠ bank changed); mix/reorder with custom | INT | P1 | FR-12 |
| I4 | Scoring config validation (non-negative ints, zero disables) | UNIT | P2 | FR-17 |

**J. Audience Display (FR-9, 10) — component-first (isolates OQ-7 layout churn)**
| ID | Scenario | Level | Pri | Trace |
|---|---|---|---|---|
| J1 | Stage switcher renders correct stage component per snapshot state | COMP | P1 | UX-DR16 |
| J2 | Timer from absolute deadline; remaining-only; gold ≤5s; prefers-reduced-motion static ring | COMP | P1 | UX-DR5 |
| J3 | Reveal: correct answer marked with ✓ (not color-only) + distribution | COMP | P1 | UX-DR14 |
| J4 | Leaderboard: top-10, shared ranks, movement indicators | COMP | P1 | UX-DR7 |
| J5 | Winner takeover incl. tie; aria-live assertive on transitions | COMP | P1 | UX-DR8/14 |
| J6 | **Golden-path E2E: full 3-question game** — build → lobby → 5 simulated participants join → MCQ + free-text rounds → reveal → leaderboard → winner; dashboard + display + WhatsApp messages all asserted | E2E | P0 | SM-1 proxy |

**K. Static / CI**
| ID | Scenario | Level | Pri | Trace |
|---|---|---|---|---|
| K1 | Filter-safety grep: no external URL asset outside index.html/index.css | STATIC | P0 | ASR-11 |
| K2 | gofmt / vet / sqlc diff / eslint / tsc gates | STATIC | P0 | arch |
| K3 | railway.json replicas==1 assertion | STATIC | P2 | R-008 |
| K4 | Hebrew-copy centralization grep (no Hebrew literals outside messages_he.go / strings.he.ts) | STATIC | P2 | arch patterns |

**L. Manual (pre-pilot release gates)**
| ID | Scenario | Level | Pri | Trace |
|---|---|---|---|---|
| L1 | Scripted dress rehearsal: real number, real devices (incl. kosher phone if available), all message classes, full game, zero ERROR logs | MANUAL | P0 | T-6, SM-1/SM-5 |
| L2 | Backup restore drill on Railway Postgres | MANUAL | P2 | R-016 |
| L3 | RTL/bidi visual review at projection scale | MANUAL | P2 | R-012 |

**Scenario totals:** P0 ≈ 33, P1 ≈ 20, P2 ≈ 9, P3 ≈ 1 (~63 planned scenarios; unit-level table-driven cases expand inside them).

### NFR Coverage & Evidence Plan (for later `nfr-assess`)

| NFR | Validation scenarios | Evidence artifact |
|---|---|---|
| Performance (NFR-1/3) | G6, G7, F5, C9, A10 | Perf-run reports (fake provider timings), Playwright traces with timing assertions |
| Reliability (NFR-2) | B1–B5, C6–C8, E4, F4, G2 | CI results + chaos E2E video/trace + log captures showing WARN-only degradations |
| Security (NFR-7) | A2, F2, H1–H6 | Authz suite report; cookie/header assertions; **blocker: SameSite + session-expiry unspecified (R-017)** |
| Privacy (NFR-4) | H5, J3 (no PII on display) | Log-audit test output |
| Observability (NFR-8) | Log-level assertions inside B/C/G tests; "zero ERROR on healthy run" asserted in J6 | Structured-log captures from E2E runs |
| Hebrew/filter-safety (NFR-5) | K1, K4, L3 | CI grep output + rehearsal checklist |
| Cost (NFR-6) | Not automatable pre-account: L1 verifies service-window behavior; message-count assertion in J6 (~P×Q×2–3 envelope) | Rehearsal report + dispatch counters |
| AI audit (NFR-9/SM-C1) | C5, C6 stage-recording assertions | `stage` column audit query documented |

### Execution Strategy (PR / Nightly / Pre-pilot)

- **PR (<10–15 min):** all UNIT + COMP + INT (local Postgres via Docker, fake provider, scripted AI) + STATIC gates + golden-path E2E (J6) if it stays under budget; otherwise J6 moves to merge-to-main.
- **Nightly:** full Playwright E2E suite (E4 restart-recovery, F4/F5, keyboard), PERF G6 + C9, full log-audit (H5).
- **Weekly / pre-milestone:** G7 (500 sim), C10 (real Claude, opt-in with `ANTHROPIC_API_KEY`), L2 restore drill.
- **Pre-pilot event (release gate):** L1 dress rehearsal + L3 RTL review + Meta pricing re-verification (R-011).
- Test-quality DoD applies throughout: deterministic (no hard waits — fake clock server-side, network-first waits in Playwright), isolated (per-test game IDs; parallel-safe), <300 lines, <1.5 min per test.

### Resource Estimates (ranges; solo dev + AI agents)

- **Test infrastructure** (fake WhatsApp provider + HMAC-signing helper + Docker Postgres fixture + Playwright setup + fake clock plumbing): ~15–25 h — build in Epics 1–2, it pays for itself immediately.
- **P0 scenarios:** ~30–45 h (spread across epic stories, not a separate phase)
- **P1 scenarios:** ~18–30 h
- **P2 scenarios:** ~8–14 h
- **P3 + rehearsal scripting:** ~4–8 h
- **Total:** ~75–120 h, amortized story-by-story across Epics 1–4 (each story lands with its scenarios; no end-loaded test phase).

### Quality Gates

- **Per story/PR:** P0 scenarios for touched FRs = 100% pass; CI static gates green.
- **Per epic:** all planned P0+P1 for the epic's FRs implemented and passing (P1 ≥ 95%); high-risk mitigations for that epic's risks (R-001/2/4/5/7) landed with their stories.
- **Pre-pilot (release):** P0 = 100%, P1 ≥ 95%; all score-6 risks MITIGATED or explicitly waived by Avraham with expiry; coverage ≥ 80% on `game/`, `grading/`, `wa/` packages (report via `go test -cover`); L1 dress rehearsal executed with zero ERROR log lines; NFR evidence artifacts collected for `nfr-assess`.
- Final NFR PASS/CONCERNS/FAIL deferred to `nfr-assess` once implementation evidence exists.
