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

# Test Design for QA: WhatsApp Clickers (Pilot)

**Purpose:** Test execution recipe — what to test, at which level, and what QA needs from other roles.

**Date:** 2026-07-09
**Author:** Murat (TEA) for Avraham Shor
**Status:** Draft
**Project:** whatsapp-clickers

**Related:** See `test-design-architecture.md` for testability blockers (T-1–T-4), full risk register, and mitigation plans.

---

## Executive Summary

**Scope:** All 18 FRs across WhatsApp intake/gameplay, grading pipeline, scoring, game engine, Host Dashboard, and Audience Display — plus the NFR evidence plan for the pilot release gate.

**Risk Summary:**

- Total risks: 17 (5 high score 6, 9 medium, 3 low)
- Critical categories: DATA (SM-4 answer integrity), TECH (cutoff + reveal gate), BUS (Hebrew grading quality), SEC (IDOR/PII)

**Coverage Summary:**

- P0: ~33 scenarios (answer integrity, cutoff, grading, authz, golden-path E2E)
- P1: ~20 scenarios (delivery/results flows, reconnect, display components)
- P2: ~9 scenarios (edge cases, static checks, drills)
- P3: ~1 scenario (500-participant benchmark)
- **Total:** ~63 scenarios, ~75–120 hours (~2–3 weeks full-time equivalent), amortized story-by-story across Epics 1–4

---

## Not in Scope

| Item | Reasoning | Mitigation |
|---|---|---|
| **Real Meta delivery behavior** (throughput, service window, template rules) | Unreachable by automation; only the real number can prove it | Scripted dress rehearsal (L1) as pre-pilot release gate |
| **Filter-provider certification** (Netfree/Etrog) | Business process, not a software deliverable (PRD §6.2) | Filter-safe design enforced by CI grep (K1) |
| **Payments, result export, marketing site** | Deferred to commercial phase (PRD §6.2) | Re-enter test design at commercial phase |
| **Israeli Privacy Protection Law compliance review** | Deferred to commercial phase (PRD §4.7) | Pilot PII posture covered by redaction tests (H5) |
| **OQ-7 final copy/layout assertions** | UX revision pending | Structure/behavior tested now; string assertions pinned when OQ-7 lands (copy is centralized in 2 files) |

---

## Dependencies & Test Blockers

### Backend/Architecture Dependencies (Pre-Implementation)

**Source:** Architecture doc "Quick Guide" — blockers T-1–T-4.

1. **T-1 Injectable `WHATSAPP_API_BASE_URL`** — dev — Epic 1–2 scaffold. Without it the fake-provider harness cannot exist and all WhatsApp-path integration tests are blocked.
2. **T-2 Injectable clock** (engine + dispatcher) — dev — Epic 1 scaffold. Blocks deterministic cutoff (B4–B5) and rate-limiter (G1) tests.
3. **T-3 AI interface seam** — dev — story 3.6 design. Blocks fail-closed tests (C6–C8).
4. **T-4 Reveal-gate escape semantics** — Avraham decision — before story 3.4. Blocks C8 (scenario is unspecifiable until decided).

### QA Infrastructure Setup (Pre-Implementation, built during Epics 1–2)

1. **Fake WhatsApp provider** — local HTTP server standing in for Meta Cloud API: records outbound sends per recipient, injectable latency/failures, exposes counters for perf assertions.
2. **Webhook payload factory + HMAC signer** — generates valid Meta webhook JSON (text/media/sticker/empty classes) signed with the test app secret; the participant-simulation backbone.
3. **Docker Postgres fixture** — real Postgres per CI run; goose migrations applied; per-test isolation via unique game IDs (parallel-safe).
4. **Playwright setup** — dashboard + display projects; participants simulated via signed webhook POSTs (no browser needed for the WhatsApp side).
5. **Data factories** — synthetic Hebrew names, E.164 phone numbers (never real numbers), games/questions with per-test unique JOIN codes.

**Example (playwright-utils api-request fixture):**

```typescript
import { test } from '@seontechnologies/playwright-utils/api-request/fixtures';
import { expect } from '@playwright/test';
import { faker } from '@faker-js/faker';

test('@P0 @Security foreign game returns GAME_NOT_FOUND', async ({ apiRequest }) => {
  const foreignGameId = faker.string.uuid(); // not owned by this session's organizer

  const { status, body } = await apiRequest({
    method: 'GET',
    path: `/api/games/${foreignGameId}`,
  });

  expect(status).toBe(404);
  expect(body.error.code).toBe('GAME_NOT_FOUND');
});
```

---

## Risk Assessment

**Note:** Full details and mitigation plans in the Architecture doc. Summary with QA coverage mapping:

### High-Priority Risks (Score 6)

| Risk ID | Category | Description | Score | QA Test Coverage |
|---|---|---|---|---|
| **R-001** | DATA | Lost/duplicated answer (SM-4) | **6** | A3 dedupe, B1 persist-before-ack fault injection, B3 race, E4 restart recovery |
| **R-002** | TECH | Reveal-gate deadlock | **6** | C7 straggler completion, C8 hung-task escape |
| **R-004** | TECH | Cutoff boundary wrong | **6** | B4 boundary ±ε with fake clock, B5 close-vs-expiry race |
| **R-005** | BUS | Wrong free-text grades | **6** | C2–C4 golden corpus, C5–C6 AI stage/fail-closed, C10 live smoke |
| **R-007** | SEC | IDOR → PII leak | **6** | H2 401 sweep, H3 IDOR suite incl. /ws and results |

### Medium/Low-Priority Risks

| Risk ID | Category | Description | Score | QA Test Coverage |
|---|---|---|---|---|
| R-003 | PERF | Delivery 5s/95% at 80 | 4 | G6 perf sim; L1 rehearsal measurement |
| R-009 | OPS | Mid-event restart/migration | 4 | E4; L2 restore drill; runbook (ops) |
| R-010 | TECH | WS reconnect edges | 4 | F3 stale seq, F4 reconnect E2E |
| R-013 | DATA | Identity edge cases | 4 | A9 name/number edge units |
| R-014 | PERF | Last-second AI burst | 4 | C9 burst test |
| R-015 | SEC | PII in logs | 4 | H5 log audit |
| R-006 | SEC | Webhook forgery | 3 | A2 HMAC negative tests |
| R-008 | OPS | Replica drift | 3 | K3 config assertion |
| R-011 | BUS | Meta pricing/window | 3 | L1 rehearsal verification (business re-check) |
| R-012 | TECH | RTL/bidi defects | 2 | L3 visual review |
| R-016 | OPS | Untested backups | 2 | L2 restore drill |
| R-017 | SEC | CSRF/SameSite | 2 | H4 cookie-flag assertions (after spec decision) |

---

## NFR Test Coverage Plan

| NFR Category | Requirement / Threshold | Planned Validation | Tool / Level | Evidence Artifact | Priority |
|---|---|---|---|---|---|
| Performance | 5s/95% delivery at 80; ≤80 msg/s; 1s display; 3s lobby | G6/G7 dispatch sim; F5 measured transitions; A10 lobby path; C9 AI burst | Perf sim vs fake provider; Playwright timing | Perf-run reports; Playwright traces | P1 |
| Reliability | SM-4; restart recovery; fail-closed AI; auto-reconnect | B1–B5, C6–C8, E4, F4, G2 | Integration + chaos E2E | CI results; E4 trace; WARN-only log captures | P0 |
| Security | HMAC; sessions; scoping; cookie flags | A2, F2, H1–H6 | API/integration | Authz suite report; header assertions | P0 |
| Privacy | Phone last-4 only in logs; no PII on shared screen | H5 log audit; J3 display review | Integration + component | Log-audit output | P1 |
| Observability | Zero ERRORs on healthy run; WARN per degradation | Log assertions embedded in B/C/G tests and J6 | Integration/E2E | Structured-log captures | P1 |
| Hebrew/filter-safety | Zero external assets; copy centralization; RTL | K1, K4 static; L3 review | CI grep; manual | CI output; rehearsal checklist | P0 (K1) |
| Cost | ~P×Q×2–3 message envelope; service-window ≈₪0 | Message-count assertion in J6; L1 verifies real behavior | E2E counters; manual | Dispatch counters; rehearsal report | P2 |
| AI audit (SM-C1) | Matching stage recorded per grade | Stage assertions in C5–C6 | Unit/integration | `stage` column audit query (documented) | P1 |

**Missing thresholds or evidence sources (clarify before `nfr-assess`):** Levenshtein threshold value; session expiry duration; SameSite attribute; default per-question time limit (30s assumed); AI concurrency cap.

---

## Entry Criteria

- [ ] T-1–T-4 blockers resolved (Architecture doc Quick Guide)
- [ ] Fake provider + webhook signer harness merged (QA infra items 1–2)
- [ ] Docker Postgres fixture in CI; migrations run green
- [ ] Feature under test deployed/runnable locally via `make dev`
- [ ] Synthetic-only test data policy in place (no real phone numbers, ever)

## Exit Criteria (pilot release gate)

- [ ] P0 pass rate = 100%
- [ ] P1 pass rate ≥ 95% (failures triaged and accepted by Avraham)
- [ ] All score-6 risks MITIGATED, or explicitly waived by Avraham with reason and expiry
- [ ] Coverage ≥ 80% on `server/internal/game`, `grading`, `wa` packages (`go test -cover`)
- [ ] No open high-severity bugs
- [ ] L1 dress rehearsal executed: full real-device game, all message classes, zero ERROR log lines
- [ ] NFR evidence artifacts collected for `nfr-assess`

---

## Test Coverage Plan

**IMPORTANT:** P0/P1/P2/P3 = priority and risk level (what to focus on if time-constrained), NOT execution timing. See Execution Strategy for when tests run.

**Levels:** UNIT (Go `testing` / Vitest), INT (httptest + Docker Postgres + fake provider + scripted AI), COMP (Vitest + Testing Library), E2E (Playwright), PERF (load sim vs fake provider), STATIC (CI checks), MANUAL (scripted). No behavior is re-proven at a higher level once proven lower; E2E covers journeys only.

### P0 (Critical)

**Criteria:** blocks core functionality + high risk (≥6) + no workaround.

| Test ID | Requirement | Test Level | Risk Link | Notes |
|---|---|---|---|---|
| A2 | Invalid webhook HMAC rejected + logged; valid processed | INT | R-006 | Negative + positive |
| A3 | Duplicate WhatsApp message ID processed exactly once | INT | R-001 | Meta retry semantics |
| A4 | JOIN registers + welcome reply; re-send idempotent | INT | FR-1 | Includes DB unique (game, phone) |
| A7 | Universal Reply matrix — every inbound class gets a reply | INT | FR-2 | Table-driven: text/media/sticker/empty/noise |
| A8 | Inbound parser: JOIN forms, א–ד / 1–4, free text, >200, noise | UNIT | FR-5 | Table-driven |
| B1 | Persist-before-ack: DB failure ⇒ no ack (fault injection) | INT | R-001 | SM-4 core |
| B2 | Second answer rejected with "already counts" | INT | FR-8 | |
| B3 | Parallel duplicate answers → exactly one row | INT | R-001 | Race on UNIQUE |
| B4 | Cutoff boundary ±ε with fake clock; late reply gets "נסגרה" | UNIT+INT | R-004 | Needs T-2 |
| B5 | Cutoff = earlier of (expiry, close); race between them | UNIT | R-004 | |
| C1 | MCQ graded mechanically; never disclosed pre-Reveal | UNIT+INT | FR-15 | |
| C2 | Exact match short-circuits pipeline (stage=exact) | UNIT | R-005 | |
| C3 | Hebrew normalization golden corpus | UNIT | R-005 | finals/nikud/punctuation/trim; UJ-3 cases |
| C4 | Fuzzy threshold pinned: typo passes, unrelated fails | UNIT | R-005 | Threshold decision recorded |
| C5 | AI invoked only on exact+fuzzy miss; stage=ai recorded | UNIT | R-005 | Scripted seam (T-3) |
| C6 | AI timeout/error → fail-closed two-stage + WARN | INT | R-005 | |
| C7 | Reveal gate opens only when all received answers graded | INT | R-002 | Straggler case |
| C8 | Hung grading task → T-4 fallback, gate opens | INT | R-002 | Needs T-4 decision |
| D1 | Configured points applied per correct answer | UNIT | FR-17 | |
| D2 | Speed Bonus 1st/2nd/3rd by timestamp; seq tie-break | UNIT | R-004 | |
| D3 | Fewer than 3 correct → fewer bonuses; zero disables | UNIT | FR-17 | |
| D4 | Shared ranks on equal scores; winner tie names all | UNIT | FR-17 | Tie presentation per story 3.9 |
| E1 | State machine: legal transitions only; leaderboard skippable; no auto-advance | UNIT | FR-13 | |
| E2 | Illegal transition rejected with stable error code | UNIT | FR-13 | |
| E3 | REST control → persisted transition → snapshot + event | INT | FR-13 | One action, two transports |
| E4 | Kill server mid-game → restart → state recovered, game continues | E2E | R-001, R-009 | Chaos scenario |
| F1 | Snapshot-on-connect for host and display roles | INT | FR-9 | |
| F2 | Unauthenticated /ws rejected | INT | R-007 | |
| G1 | Token bucket ≤80 msg/s (fake clock) | UNIT | FR-4 | |
| G2 | Send retry ×3 + backoff; final failure WARN, loop unblocked | INT | NFR-2 | |
| G3 | Question open → all players receive it; spectators none | INT | FR-4 | |
| G4 | Post-Reveal results to answerers only; nothing pre-Reveal | INT | FR-6 | Non-answerer silence |
| G5 | Final results to all participants + spectators | INT | FR-6 | |
| H1 | Login/logout; sessions survive restart; invalid creds error copy | INT | NFR-7 | |
| H2 | 401 envelope on every unauthenticated /api route | INT | R-007 | Route sweep |
| H3 | IDOR suite: foreign game across all routes + /ws → GAME_NOT_FOUND | INT | R-007 | DoD per new route |
| J6 | Golden-path E2E: full 3-question game (build → lobby → 5 joins → MCQ + free-text → reveal → leaderboard → winner) across dashboard + display + messages | E2E | SM-1 | Also asserts message-count envelope + zero ERRORs |
| K1 | Filter-safety grep: no external URL assets | STATIC | NFR-5 | CI |
| K2 | gofmt / vet / sqlc diff / eslint / tsc | STATIC | arch | CI (already defined) |
| L1 | Dress rehearsal with real number and devices | MANUAL | R-003, R-011 | Pre-pilot release gate; scripted checklist |

**Total P0:** ~33 (table-driven units expand within scenarios)

### P1 (High)

**Criteria:** important features + medium risk + common workflows.

| Test ID | Requirement | Test Level | Risk Link | Notes |
|---|---|---|---|---|
| A1 | Webhook GET handshake (verify token) | INT | — | |
| A5 | Invalid/expired JOIN code → Hebrew not-found reply | INT | FR-1 | |
| A6 | Late JOIN → spectator: no dispatch, gets final results | INT | FR-3 | |
| A10 | JOIN → lobby snapshot broadcast (3s path) | INT | NFR-1 | |
| B6 | Unparseable MCQ reply → hint, can still answer | INT | FR-5 | |
| B7 | Free text >200 rejected with hint | UNIT | FR-5 | |
| B8 | Answered count/% in snapshot while open | INT | FR-13 | |
| C9 | Last-second burst (~30 free-text) → gate opens in bounded time | PERF | R-014 | Nightly |
| D5 | Leaderboard data in snapshot after Reveal | INT | FR-18 | |
| E5 | Results summary persists across sessions | INT | FR-14 | |
| F3 | Stale seq dropped by WS hook | COMP | R-010 | |
| F4 | Reconnect with backoff re-renders state; "מתחבר..." only while down | E2E | R-010 | |
| F5 | State transition renders ≤1s (measured) | E2E | NFR-1 | |
| G6 | 80 simulated participants: 5s/95% delivery vs fake provider | PERF | R-003 | Nightly |
| H4 | Cookie flags: HttpOnly, Secure, SameSite | INT | R-017 | After spec decision |
| H5 | Log audit: no full phone number anywhere | INT | R-015 | Canonical + error paths |
| H6 | argon2id hash/verify | UNIT | NFR-7 | |
| I1 | Game/question CRUD + reorder persists; JOIN code shown | E2E+INT | FR-11 | One happy-path E2E; validation at INT |
| I2 | MCQ exactly 4 options/1 correct; accepted answers editable | INT | FR-11 | |
| I3 | Package import copy semantics; mix/reorder | INT | FR-12 | |
| J1–J5 | Display stages: switcher, timer (gold ≤5s, reduced-motion), reveal ✓+distribution, leaderboard ranks/movers, winner+tie, aria-live | COMP | FR-10 | 5 component specs; OQ-7 copy pinned later |

**Total P1:** ~20

### P2 (Medium)

**Criteria:** secondary features + low risk + edge cases.

| Test ID | Requirement | Test Level | Risk Link | Notes |
|---|---|---|---|---|
| A9 | Display-name/number edge cases (empty/emoji profile) | UNIT | R-013 | |
| C10 | Real-Claude Hebrew smoke (~10 golden cases, opt-in) | INT-live | R-005 | Weekly, needs API key |
| E6 | Space/Escape keyboard shortcuts | E2E | UX-DR13 | |
| I4 | Scoring config validation (non-negative, zero disables) | UNIT | FR-17 | |
| K3 | railway.json replicas==1 assertion | STATIC | R-008 | |
| K4 | Hebrew-copy centralization grep | STATIC | arch | |
| L2 | Backup restore drill (Railway Postgres) | MANUAL | R-016 | Pre-pilot |
| L3 | RTL/bidi visual review at projection scale | MANUAL | R-012 | At rehearsal |

**Total P2:** ~9

### P3 (Low)

| Test ID | Requirement | Test Level | Notes |
|---|---|---|---|
| G7 | 500-participant dispatch simulation (pacing benchmark) | PERF | Pre-commercial; documents the ~7s constraint |

**Total P3:** 1

---

## Execution Strategy

**Philosophy:** run everything in PRs unless there's significant infrastructure overhead. Go tests + Vitest + parallelized Playwright fit comfortably in ~10–15 minutes at this project's size.

### Every PR (~10–15 min)

- All UNIT + COMP (Go `testing`, Vitest)
- All INT (Docker Postgres, fake provider, scripted AI — no external calls)
- Golden-path E2E (J6) + STATIC gates (K1–K4)

### Nightly (~30–60 min)

- Full Playwright suite (E4 chaos restart, F4/F5 reconnect/latency, E6)
- PERF: G6 (80-participant delivery), C9 (AI burst)
- H5 full log audit over the nightly run's captures

### Weekly / pre-milestone

- G7 (500-participant sim), C10 (real-Claude smoke, opt-in via `ANTHROPIC_API_KEY`)
- L2 restore drill (manual, pre-pilot at minimum)

### Manual (excluded from automation)

- L1 dress rehearsal (pre-pilot release gate), L3 RTL review, Meta pricing re-verification (R-011)

---

## QA Effort Estimate

QA/test development effort only (solo dev wearing the QA hat, AI-agent-assisted):

| Priority | Count | Effort Range | Notes |
|---|---|---|---|
| Infra | — | ~15–25 h | Fake provider, webhook signer, Postgres fixture, Playwright setup, fake clock plumbing |
| P0 | ~33 | ~30–45 h | Concurrency/fault-injection scenarios dominate |
| P1 | ~20 | ~18–30 h | Standard integration + component specs |
| P2 | ~9 | ~8–14 h | Edge cases, static checks, drill scripting |
| P3 | ~1 | ~2–4 h | Benchmark harness reuse |
| **Total** | ~63 | **~75–120 h (~2–3 weeks FTE)** | Amortized across Epics 1–4; each story lands with its scenarios — no end-loaded test phase |

**Assumptions:** infra built during Epics 1–2 and reused everywhere; estimates include design, implementation, debugging, CI integration; excludes ongoing maintenance (~10%).

---

## Implementation Planning Handoff

| Work Item | Owner | Target Milestone | Dependencies/Notes |
|---|---|---|---|
| Fake provider + webhook signer harness | dev | Epic 2 story 2.1 | Needs T-1 |
| Fake clock plumbing | dev | Epic 1 story 1.1 | T-2 |
| Hebrew golden corpus (content) | Avraham | Before story 3.5 | R-005; curate real family-quiz answers |
| T-4 reveal-gate decision | Avraham | Before story 3.4 | R-002 |
| Authz-test-per-route DoD | dev | Story 1.3 onward | R-007 |
| Dress-rehearsal script + checklist | Avraham + dev | Before first pilot event | L1 release gate |

---

## Tooling & Access

| Tool or Service | Purpose | Access Required | Status |
|---|---|---|---|
| Meta test number + app secret | Dev/integration webhook work | Meta developer account | Pending (business setup in flight) |
| Anthropic API key | C10 live smoke only (suite uses scripted seam) | `ANTHROPIC_API_KEY` | Ready (env) |
| Railway | Deploy target; L2 restore drill | Project access | Ready |
| cloudflared | Webhook tunnel for local integration | none | Ready |
| Docker | Local/CI Postgres | none | Ready |

---

## Interworking & Regression

| Service/Component | Impact | Regression Scope | Validation Steps |
|---|---|---|---|
| Greenfield — no existing consumers | — | Full PR suite is the regression suite from story 1.1 onward | Every PR runs the entire functional suite |
| Meta Cloud API (external) | Contract we consume | Webhook payload parsing (A7/A8) pinned against recorded real payloads | Refresh fixtures from rehearsal captures |
| Claude API (external) | Degradable dependency | C6 fail-closed guarantees suite never depends on it | C10 weekly smoke detects drift |

---

## Appendix A: Code Examples & Tagging

```typescript
import { test } from '@seontechnologies/playwright-utils/api-request/fixtures';
import { expect } from '@playwright/test';

// P0: control action → snapshot broadcast (E3)
test('@P0 @Integration reveal publishes snapshot with grades', async ({ apiRequest }) => {
  const { status } = await apiRequest({
    method: 'POST',
    path: `/api/games/${gameId}/questions/${questionId}/reveal`,
  });
  expect(status).toBe(200);

  const snapshot = await nextSnapshot(ws); // test helper: resolves on next WS message
  expect(snapshot.state).toBe('revealed');
  expect(snapshot.seq).toBeGreaterThan(prevSeq);
});
```

```bash
# Run only P0
npx playwright test --grep @P0
# P0 + P1
npx playwright test --grep "@P0|@P1"
# Go side
go test ./... -cover
```

Tags: `@P0–@P3`, `@Security`, `@Integration`, `@Perf` — priority tags mirror this document's IDs in test titles (e.g., `B4: cutoff boundary`).

## Appendix B: Knowledge Base References

- `risk-governance.md` — risk scoring methodology
- `probability-impact.md` — P×I scale definitions
- `test-priorities-matrix.md` — P0–P3 criteria
- `test-levels-framework.md` — level selection, duplicate-coverage guard
- `test-quality.md` — Definition of Done (deterministic, isolated, <300 lines, <1.5 min)
- `webhook-risk-guidance.md` — webhook boundary risk defaults

---

**Generated by:** BMad TEA Agent (Murat)
**Workflow:** `bmad-testarch-test-design`
**Version:** 4.0 (BMad v6)
