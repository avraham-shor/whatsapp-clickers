---
title: 'TEA Test Design → BMAD Handoff Document'
version: '1.0'
workflowType: 'testarch-test-design-handoff'
inputDocuments:
  - _bmad-output/test-artifacts/test-design-architecture.md
  - _bmad-output/test-artifacts/test-design-qa.md
sourceWorkflow: 'testarch-test-design'
generatedBy: 'TEA Master Test Architect'
generatedAt: '2026-07-09'
projectName: 'whatsapp-clickers'
---

# TEA → BMAD Integration Handoff

## Purpose

Bridges TEA's system-level test design with BMAD's epic/story workflows. Epics already exist (26 stories, `epics.md` 2026-07-09); this handoff feeds **story creation** (`create-story`) and any epics revision so quality requirements, risks, and test scenarios flow into each story's context.

## TEA Artifacts Inventory

| Artifact | Path | BMAD Integration Point |
|---|---|---|
| Architecture Test Design | `_bmad-output/test-artifacts/test-design-architecture.md` | Blockers T-1–T-4 → story 1.1/2.1/3.4/3.6 acceptance criteria; risk mitigations |
| QA Test Design | `_bmad-output/test-artifacts/test-design-qa.md` | Scenario IDs (A1–L3) → story acceptance criteria and test tasks |
| Risk Assessment | (embedded in both, IDs R-001–R-017) | Epic quality gates, story priority |
| Progress/working file | `_bmad-output/test-artifacts/test-design-progress.md` | Full analysis trail |

## Epic-Level Integration Guidance

### Risk References

- **Epic 1 (Foundation & Authoring):** R-007 IDOR (score 6) — authz scoping from story 1.3; T-1/T-2 testability blockers land in story 1.1 scaffold.
- **Epic 2 (WhatsApp Front Door):** R-001 answer/dedupe integrity (score 6) — webhook dedupe in story 2.1; R-006 HMAC; R-013 identity edges; fake-provider harness is an Epic-2 deliverable.
- **Epic 3 (Live Gameplay):** carries four of five high risks — R-001 (story 3.3), R-002 reveal gate (3.4/3.6, needs T-4 decision first), R-004 cutoff (3.1/3.3), R-005 grading quality (3.4–3.6, needs golden corpus).
- **Epic 4 (Audience Display):** R-010 reconnect, R-012 RTL — component-first testing isolates OQ-7 layout churn.

### Quality Gates

- **Per story:** P0 scenarios for the story's FRs pass 100%; new routes ship with an authz negative test (R-007 DoD).
- **Per epic:** epic's P0+P1 scenarios implemented and passing (P1 ≥ 95%); epic's high-risk mitigations landed with their stories.
- **Pre-pilot release:** all score-6 risks MITIGATED or waived with expiry; coverage ≥80% on `game`/`grading`/`wa`; L1 dress rehearsal with zero ERROR logs.

## Story-Level Integration Guidance

### P0/P1 Test Scenarios → Story Acceptance Criteria

Critical scenarios that MUST appear as acceptance criteria (QA doc IDs):

- **Story 1.1:** T-1 (`WHATSAPP_API_BASE_URL`), T-2 (injectable clock), K1 filter-safety grep in CI
- **Story 1.2:** H1 sessions, H2 401 sweep
- **Story 1.3:** H3 IDOR suite starts here (GAME_NOT_FOUND for foreign games)
- **Story 2.1:** A2 HMAC negative, A3 message-ID dedupe, G1 token bucket, G2 retry/WARN; fake-provider harness + webhook signer built here
- **Story 2.2:** A7 Universal-Reply matrix (every message class replies)
- **Story 2.4:** A4 idempotent JOIN + DB unique (game, phone)
- **Story 3.1:** E1/E2 state machine legality, B5 cutoff rule ownership
- **Story 3.3:** B1 persist-before-ack fault injection, B3 duplicate race, B4 cutoff boundary
- **Story 3.4:** C1/C2 + C7 reveal-gate tracker (T-4 semantics decided before this story)
- **Story 3.5:** C3 Hebrew golden corpus, C4 pinned threshold
- **Story 3.6:** C5/C6 scripted-AI fail-closed, C8 hung-task escape
- **Story 3.7:** D1–D4 scoring/bonus/tie units
- **Story 3.9:** G5 final results incl. spectators; D4 winner-tie presentation
- **Story 4.1:** F1 snapshot-on-connect, F2 /ws auth, F4 reconnect
- **Story 4.3:** J2 timer from absolute deadline, gold ≤5s, reduced-motion
- **End of Epic 4:** J6 golden-path full-game E2E

### Data-TestId Requirements

For stable Playwright selectors (per shadcn/kebab-case conventions): `data-testid` on — login form fields/submit; games-list rows; question editor (type toggle, options א–ד, correct marker, accepted-answers list, time-limit input); lobby counter + participant list; control-panel primary CTA (one per state — id can be static, e.g. `primary-action`); response stat pill; display stage roots (`stage-lobby`, `stage-question`, `stage-reveal`, `stage-leaderboard`, `stage-winner`); timer ring + numeral; leaderboard rows; winner card.

## Risk-to-Story Mapping

| Risk ID | Category | P×I | Recommended Story/Epic | Test Level |
|---|---|---|---|---|
| R-001 | DATA | 2×3=6 | Stories 2.1, 3.3 | INT + E2E (E4) |
| R-002 | TECH | 2×3=6 | Stories 3.4, 3.6 (T-4 decision first) | INT |
| R-004 | TECH | 2×3=6 | Stories 3.1, 3.3 | UNIT + INT |
| R-005 | BUS | 3×2=6 | Stories 3.4–3.6 (+ corpus task) | UNIT + INT-live |
| R-007 | SEC | 2×3=6 | Story 1.3 onward (DoD) | INT |
| R-003 | PERF | 2×2=4 | Story 3.2 (+ nightly G6) | PERF |
| R-009 | OPS | 2×2=4 | Story 3.1 (E4) + runbook | E2E + MANUAL |
| R-010 | TECH | 2×2=4 | Stories 2.3, 4.1 | COMP + E2E |
| R-013 | DATA | 2×2=4 | Story 2.4 | UNIT |
| R-014 | PERF | 2×2=4 | Story 3.6 (C9 nightly) | PERF |
| R-015 | SEC | 2×2=4 | Story 2.1 (+ nightly audit) | INT |
| R-006 | SEC | 1×3=3 | Story 2.1 | INT |
| R-008 | OPS | 1×3=3 | Story 1.1 (K3) | STATIC |
| R-011 | BUS | 1×3=3 | Business task (Meta account) | MANUAL |
| R-012 | TECH | 2×1=2 | Epic 4 + L3 review | MANUAL |
| R-016 | OPS | 1×2=2 | Pre-pilot L2 drill | MANUAL |
| R-017 | SEC | 1×2=2 | Story 1.2 (spec SameSite first) | INT |

## Recommended BMAD → TEA Workflow Sequence

1. **TEA Test Design** — done (this handoff)
2. **BMAD Create Story** (`create-story`) — consume this handoff per story; note OQ-7 blocks §4.2/§4.3 story *content*, not scaffolding
3. **TEA ATDD** (`atdd`) — generate failing acceptance tests for each story's P0 scenarios
4. **BMAD Implementation** (`dev-story`) — implement with test-first guidance
5. **TEA Automate** (`automate`) — expand to full coverage
6. **TEA Trace** (`trace`) — traceability matrix + gate decision before pilot

## Phase Transition Quality Gates

| From Phase | To Phase | Gate Criteria |
|---|---|---|
| Test Design | Story Creation | T-4 decision made; all score-6 risks have mitigation strategies (✔ done) |
| Story Creation | ATDD | Stories carry acceptance criteria from this handoff's scenario mapping |
| ATDD | Implementation | Failing acceptance tests exist for the story's P0 scenarios |
| Implementation | Test Automation | Story's acceptance tests pass; authz DoD met |
| Test Automation | Pilot Release | P0 100% / P1 ≥95%; coverage ≥80% on core packages; L1 rehearsal clean |
