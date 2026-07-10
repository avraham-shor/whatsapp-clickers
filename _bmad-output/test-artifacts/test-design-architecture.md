---
workflowStatus: 'complete'
totalSteps: 5
stepsCompleted: [1, 2, 3, 4, 5]
lastStep: 'step-05-generate-output'
nextStep: ''
lastSaved: '2026-07-09'
workflowType: 'testarch-test-design'
inputDocuments:
  - _bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md
  - _bmad-output/planning-artifacts/architecture.md
  - _bmad-output/planning-artifacts/epics.md
---

# Test Design for Architecture: WhatsApp Clickers (Pilot)

**Purpose:** Architectural concerns, testability gaps, and NFR requirements for review by Architecture/Dev. Serves as the contract on what must be addressed before test development begins.

**Date:** 2026-07-09
**Author:** Murat (TEA) for Avraham Shor
**Status:** Architecture Review Pending
**Project:** whatsapp-clickers
**PRD Reference:** `_bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md`
**ADR Reference:** `_bmad-output/planning-artifacts/architecture.md`

---

## Executive Summary

**Scope:** Full pilot system — WhatsApp-only participant gameplay (webhook in / Cloud API out), server-authoritative game engine, three-stage free-text grading (Exact → Fuzzy → AI), Host Dashboard, Audience Display over WS snapshots.

**Business Context** (from PRD): pilot success = 3–5 real family events (30–80 participants) with no significant technical failure (SM-1) and zero lost acknowledged answers (SM-4). A live event cannot pause for debugging. Chanukah 2026 commercial launch is the horizon, not the first milestone.

**Architecture** (from architecture.md):

- Go 1.26 single binary (chi, coder/websocket, pgx/sqlc) + React 19 SPA; single Railway instance + managed Postgres
- Postgres-authoritative live state; persist-before-ack; in-memory hub for WS full-snapshot fan-out
- Meta WhatsApp Cloud API direct (80 msg/s ceiling); Claude Opus 4.8 for AI grading, fail-closed

**Expected Scale:** 30–80 participants/game, one game at a time; must not preclude 500. ~2,000 WhatsApp messages per 60-person game.

**Risk Summary:** 17 risks total — **5 high-priority (score 6)**, 9 medium (3–5), 3 low (1–2). Test effort ~75–120 hours amortized across epics (see QA doc).

---

## Quick Guide

### 🚨 BLOCKERS — Team Must Decide (Can't Proceed Without)

Pre-implementation critical path — required before integration tests can be written:

1. **T-1: Injectable WhatsApp API base URL** — add `WHATSAPP_API_BASE_URL` env var so a local fake provider can stand in for Meta; the entire messaging path is otherwise untestable below manual level (owner: dev, Epic 1–2 scaffold).
2. **T-2: Injectable clock in `game.Engine` and `wa/dispatch`** — deterministic cutoff-boundary and rate-limiter tests require a controllable time source (owner: dev, Epic 1 scaffold).
3. **T-3: AI-stage interface seam** — `grading/ai.go` behind a small interface for scripted verdicts/timeouts/errors (owner: dev, story 3.6 design).
4. **T-4: Reveal-gate escape semantics** — define what happens when a grading task never completes (recommended: hard per-answer grading deadline → fall back to two-stage grade, log WARN, gate opens). Undefined behavior here is a live-event deadlock (R-002) and untestable as specified (owner: Avraham decision + dev, before story 3.4).

### ⚠️ HIGH PRIORITY — Team Should Validate

1. **R-004 cutoff boundary:** confirm the cutoff rule compares webhook server-receipt time against the engine deadline with one clock source; approve boundary semantics (receipt == deadline → accepted or rejected — pick one and document).
2. **R-005 grading quality:** approve building a versioned Hebrew golden corpus (Avraham supplies real family-quiz answer examples) and pinning the Levenshtein threshold with tests before story 3.5.
3. **R-007 IDOR/PII:** approve authz-test-per-route as a story-level Definition of Done from story 1.3 onward (participant phone numbers make any scoping miss a PII leak).
4. **R-017 CSRF:** architecture specifies HTTP-only Secure session cookies but not `SameSite`; recommend `SameSite=Lax` minimum — approve and add to architecture patterns.
5. **Ops runbook:** approve a "no deploys during live events" rule plus a pre-pilot backup-restore drill (R-009, R-016).

### 📋 INFO ONLY — Solutions Provided

1. **Test strategy:** unit-heavy for engine/grading/scoring, integration (real Postgres + fake provider) for webhook/dispatch/authz, component tests for display stages, one golden-path E2E, perf sim at 80/500. Details in QA doc.
2. **Coverage:** ~63 prioritized scenarios (33 P0 / 20 P1 / 9 P2 / 1 P3), risk-linked.
3. **Real-provider validation:** scripted dress rehearsal with the real number before the first pilot event — planned as a release gate, not ad-hoc.

---

## Risk Assessment

**Total risks identified:** 17 (5 high score 6, 9 medium 3–5, 3 low 1–2)

### High-Priority Risks (Score ≥6) — IMMEDIATE ATTENTION

| Risk ID | Category | Description | P | I | Score | Mitigation | Owner | Timeline |
|---|---|---|---|---|---|---|---|---|
| **R-001** | **DATA** | Lost/duplicated answer under webhook retry + concurrency — SM-4 breach (ack sent, answer missing) | 2 | 3 | **6** | Persist-before-ack in one transaction; dedupe by WhatsApp message ID; `UNIQUE (question_id, participant_id)`; restart recovery | dev | Stories 2.1 / 3.3 |
| **R-002** | **TECH** | Reveal-gate deadlock: outstanding-grades counter never reaches zero → "גלה תשובה" never activates mid-event | 2 | 3 | **6** | T-4 escape semantics: per-answer grading deadline, fallback to two-stage grade, gate opens | Avraham + dev | Before story 3.4 |
| **R-004** | **TECH** | Cutoff boundary wrong (deadline comparison, clock source, close-vs-expiry race) → scoring disputes (SM-1 failure class) | 2 | 3 | **6** | Single clock source via T-2; documented boundary rule; engine owns one deadline | dev | Stories 3.1 / 3.3 |
| **R-005** | **BUS** | Wrong free-text grades: unpinned Levenshtein threshold, Hebrew normalization bugs, AI equivalence misjudgment (SM-C1) | 3 | 2 | **6** | Versioned Hebrew golden corpus; threshold pinned by tests; `stage` recorded per grade for audit | dev + Avraham | Stories 3.4–3.6 |
| **R-007** | **SEC** | Ownership-scoping miss (IDOR) → foreign organizer reads games or participant phone numbers (PII) | 2 | 3 | **6** | organizer_id scoping in every query (already designed); authz check per new route as DoD | dev | Story 1.3 onward |

### Medium-Priority Risks (Score 3–5)

| Risk ID | Category | Description | P | I | Score | Mitigation | Owner |
|---|---|---|---|---|---|---|---|
| R-003 | PERF | Delivery misses 5s/95% at 80 (provider variance); ~7s at 500 known | 2 | 2 | 4 | Perf sim vs fake provider; dress-rehearsal measurement | dev |
| R-009 | OPS | Restart/deploy/migration mid-event; boot migration failure | 2 | 2 | 4 | Restart-recovery test; no-deploy-during-events runbook | Avraham |
| R-010 | TECH | WS reconnect edge cases — display blank at key moment | 2 | 2 | 4 | Reconnect/backoff tests; snapshot-on-connect | dev |
| R-013 | DATA | Identity edge cases: empty/emoji profile name, number variants | 2 | 2 | 4 | Fallback display name; parser edge tests | dev |
| R-014 | PERF | Last-second free-text burst → stacked AI calls delay Reveal | 2 | 2 | 4 | Concurrency cap on AI calls; burst test | dev |
| R-015 | SEC | PII leak in logs via non-canonical paths | 2 | 2 | 4 | Central redaction helper; log-audit assertions | dev |
| R-006 | SEC | Webhook forgery/replay if HMAC verification flawed | 1 | 3 | 3 | HMAC verify (designed); negative tests | dev |
| R-008 | OPS | Config drift → >1 replica splits in-memory hub | 1 | 3 | 3 | CI assertion on railway.json | dev |
| R-011 | BUS | Meta service-window/pricing assumption wrong | 1 | 3 | 3 | Re-verify at account creation (tracked) | Avraham |

### Low-Priority Risks (Score 1–2)

| Risk ID | Category | Description | P | I | Score | Action |
|---|---|---|---|---|---|---|
| R-012 | TECH | RTL/bidi rendering defects at projection scale | 2 | 1 | 2 | Rehearsal visual review |
| R-016 | OPS | Backups never restore-tested | 1 | 2 | 2 | Pre-pilot restore drill |
| R-017 | SEC | CSRF on cookie-authenticated control actions (SameSite unspecified) | 1 | 2 | 2 | Specify SameSite=Lax; header check |

**Risk category legend:** TECH architecture/integration · SEC security · PERF performance · DATA integrity · BUS business/UX harm · OPS deployment/operations.

---

## NFR Testability Requirements

| NFR Category | Threshold / Requirement | Current Design Support | Gap / Decision Needed | Planned Evidence |
|---|---|---|---|---|
| Performance | 1s display transition; 3s lobby count; 5s/95% delivery at 80; ≤80 msg/s | Supported (in-process fan-out has margin; burst math documented) | Fake provider (T-1) + fake clock (T-2) needed to measure | Perf-sim reports at 80/500; E2E timing assertions |
| Reliability | SM-4 zero lost acked answers; restart recovery; fail-closed AI; silent self-healing | Supported by construction (persist-before-ack, dedupe, DB-rebuildable hub) | T-4 reveal-gate escape semantics undefined | Integration/chaos test results; WARN-only log captures |
| Security | HMAC webhooks; argon2id + server sessions; organizer scoping; PII redaction | Supported | **UNKNOWN:** session expiry; SameSite attribute (R-017) | Authz suite results; header/cookie assertions; log audit |
| Scalability | 30–80 now; must-not-preclude 500 | Supported (documented 7s constraint at 500) | None for pilot | 500-participant simulation (pre-commercial) |
| Maintainability | CI: gofmt, vet, sqlc diff, eslint, tsc | Supported (CI defined) | None | CI status + coverage reports |
| Hebrew/filter-safety | Zero external assets; RTL everywhere; system fonts | Supported (build-time enforcement) | None | CI grep output; rehearsal RTL checklist |

**Unknown thresholds (do not guess):** Levenshtein threshold value (→ R-005), session expiry duration, `SameSite` attribute (→ R-017), default per-question time limit (story 1.3 assumes 30s — confirm), AI-call concurrency cap (→ R-014).

**Assessment boundary:** final PASS/CONCERNS/FAIL belongs to `nfr-assess` after implementation evidence exists.

---

## Testability Concerns and Architectural Gaps

### 🚨 ACTIONABLE CONCERNS — Architecture/Dev Must Address

#### 1. Blockers to Fast Feedback

| Concern | Impact | What Architecture Must Provide | Owner | Timeline |
|---|---|---|---|---|
| **Hardcoded Meta endpoint** | No integration/perf test can exercise the messaging path | `WHATSAPP_API_BASE_URL` env var (T-1) | dev | Epic 1–2 scaffold |
| **Ambient time in engine/dispatcher** | Cutoff and rate-limit tests need real sleeps → slow, flaky | Injectable clock (T-2) | dev | Epic 1 scaffold |
| **Direct SDK call in grading** | Fail-closed path untestable without live API | AI interface seam (T-3) | dev | Story 3.6 design |
| **Undefined reveal-gate escape** | Scenario unspecifiable; deadlock hazard (R-002) | Grading-deadline fallback semantics (T-4) | Avraham + dev | Before story 3.4 |

#### 2. Architectural Improvements Needed

1. **Signed-webhook test helper + fake provider harness (T-5)**
   - **Current problem:** simulating participants requires hand-rolling HMAC-signed Meta payloads per test.
   - **Required change:** build once in Epic 2: a fake Cloud API server (records outbound sends, controllable latency/failures) + payload factory that signs with the test app secret.
   - **Impact if not fixed:** every WhatsApp-path test reinvents the harness; perf tests impossible.
2. **Dress-rehearsal protocol (T-6)**
   - **Current problem:** Meta's real delivery behavior (throughput, service window, media classes) is unreachable by automation.
   - **Required change:** scripted small real-device game (checklist: all message classes, kosher-phone participant if available, zero ERROR logs) as a pre-pilot release gate.
   - **Impact if not fixed:** first "test" of the real provider is the first family event.

### Testability Assessment Summary

**What works well:** headless-by-construction (REST/webhook/WS covers all behavior); Postgres-authoritative state gives one assertable truth and direct restart-recovery testing; full-snapshot WS protocol is trivially assertable; monotonic-sequence tie-breaking removes timing flakiness; "zero ERRORs on a healthy run" (NFR-8) is itself an automatable invariant; grading normalization/fuzzy stages are pure functions.

**Accepted trade-offs (no action required):** stateful single instance (rebuildable, pinned to 1 — pilot-appropriate); logs-only observability, no metrics endpoint (revisit at commercial phase); restart-with-downtime deploys (covered by runbook); no seeding API (product API + webhook simulation suffice).

---

## Risk Mitigation Plans (High-Priority Risks ≥6)

#### R-001: Lost/duplicated answer — SM-4 breach (Score 6)

**Mitigation:** 1) answer INSERT and dedupe check in one transaction, ack sent only after commit; 2) dedupe keyed on WhatsApp message ID at webhook intake; 3) `UNIQUE (question_id, participant_id)` as last line; 4) hub state always rebuilt from DB on boot.
**Owner:** dev · **Timeline:** stories 2.1/3.3 · **Status:** Planned · **Verification:** QA doc scenarios A3, B1–B3, E4.

#### R-002: Reveal-gate deadlock (Score 6)

**Mitigation:** 1) decide T-4 semantics (recommended: per-answer grading deadline ≈ AI timeout + margin; on expiry, grade by two stages, log WARN, decrement counter); 2) outstanding-grades tracker owned by the engine, decremented in exactly one code path; 3) no unbounded waits anywhere in the reveal path.
**Owner:** Avraham (decision) + dev · **Timeline:** before story 3.4 · **Status:** Decision pending · **Verification:** QA doc scenarios C7–C8.

#### R-004: Cutoff boundary wrong (Score 6)

**Mitigation:** 1) engine computes and stores one absolute deadline at question open; 2) intake compares webhook server-receipt timestamp against it (documented ≤ vs < rule); 3) explicit close overwrites deadline to now — one rule, one field; 4) all time reads through the injected clock (T-2).
**Owner:** dev · **Timeline:** stories 3.1/3.3 · **Status:** Planned · **Verification:** QA doc scenarios B4–B5.

#### R-005: Wrong free-text grades (Score 6)

**Mitigation:** 1) versioned Hebrew golden corpus (Avraham curates realistic answers: typos, final-letter forms, nikud, synonyms, wrong-but-close); 2) Levenshtein threshold chosen against the corpus and pinned by tests; 3) `stage` recorded on every grade; 4) AI prompt judges equivalence only, strict `{"correct": bool}` output.
**Owner:** dev + Avraham (corpus) · **Timeline:** stories 3.4–3.6 · **Status:** Planned · **Verification:** QA doc scenarios C2–C6, C10.

#### R-007: IDOR / PII exposure (Score 6)

**Mitigation:** 1) every store query takes organizer_id (already the design — enforce in code review); 2) `/ws` validates session + game ownership before subscribing; 3) authz negative test added with every new route (DoD); 4) error is `GAME_NOT_FOUND`, never a distinguishable 403 that confirms existence.
**Owner:** dev · **Timeline:** story 1.3 onward · **Status:** Planned · **Verification:** QA doc scenarios H2–H3.

---

## Assumptions and Dependencies

### Assumptions

1. Meta 24-hour service window covers a full game with free-form messages at ≈₪0 (re-verify at account creation — R-011).
2. Single instance + in-memory hub holds for the pilot; replicas pinned to 1.
3. PRD assumption-tagged latency targets (3s lobby, 5s/95% delivery, 1s display) are treated as binding thresholds for test design.
4. OQ-7 UX revision changes copy/layout only — isolated to `messages_he.go`, `strings.he.ts`, and display stage components; test structure is unaffected, string assertions are updated when OQ-7 lands.

### Dependencies

1. Meta WhatsApp Business verification + dedicated number — gates Epic 2 integration against the real provider; test number suffices for development (start immediately, per architecture).
2. OQ-7 UX revision — blocks final copy assertions for §4.2/§4.3 stories, not test scaffolding.
3. T-1–T-4 blockers — gate integration test development (see Quick Guide).

### Risks to Plan

- **Risk:** OQ-7 lands late → copy assertions churn. **Impact:** string-level test updates. **Contingency:** assert on structure/identifiers, keep copy assertions centralized against the two copy files.
- **Risk:** Meta account delays → no real-provider rehearsal window. **Impact:** L1 gate slips. **Contingency:** fake-provider coverage holds; rehearsal is the only deferred item — do not run a pilot event without it.

---

**End of Architecture Document**

**Next steps — Architecture/Dev:** resolve the four 🚨 blockers (T-1–T-4), approve the five ⚠️ items, assign the R-002 decision.
**Next steps — QA:** see companion `test-design-qa.md`; build the Epic-2 test harness (fake provider + webhook signer) first.
