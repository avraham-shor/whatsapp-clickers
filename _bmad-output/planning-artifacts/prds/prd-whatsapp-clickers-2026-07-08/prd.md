---
title: WhatsApp Clickers PRD
status: final
created: 2026-07-08
updated: 2026-07-08
---

# PRD: WhatsApp Clickers
*Working title — confirm.*

## 0. Document Purpose

This PRD defines the **pilot scope** of WhatsApp Clickers for the product owner (Avraham Shor) and the downstream architecture, epics, and development workflows. It is calibrated to a family-events pilot, with the Chanukah 2026 commercial launch as the horizon but not the first milestone. It builds on the Product Brief (`briefs/brief-whatsapp-clickers-2026-06-18/brief.md`), the UX specification (`ux-designs/ux-whatsapp-clickers-2026-07-05/DESIGN.md` and `EXPERIENCE.md`), and the market-entry brainstorming session (2026-06-24). Where this PRD contradicts those documents, **this PRD wins**. Two deliberate overrides matter most (both logged in `.decision-log.md`, 2026-07-08): (1) participants play **entirely inside WhatsApp** — there is no participant web interface, superseding both the brief's web-play model and EXPERIENCE.md's participant mobile screens; (2) a projector-facing **Audience Display** is promoted from a v2 assumption to pilot-essential. DESIGN.md's visual identity remains binding; EXPERIENCE.md remains authoritative for the Host Dashboard. Vocabulary is anchored in the Glossary (§3); features are grouped with globally numbered FRs; unconfirmed inferences are tagged inline `[ASSUMPTION]` and indexed in §9.

## 1. Vision

WhatsApp Clickers is a Hebrew-native, real-time quiz platform for live group events in Israel — family gatherings, school programs, corporate team days. Participants need exactly one tool: WhatsApp. They join by sending a single message, receive each question as a message, answer by replying, and get their results in the same chat. No app, no account, no browser — which means a participant on a kosher smartphone (WhatsApp, no browser) plays exactly like everyone else. Every message a participant sends gets a response; the chat never goes silent on them.

The room's drama lives on a shared screen: the Organizer projects the Audience Display — the question, a big countdown timer, the live leaderboard, and a festive winner takeover — while running the game from a desktop dashboard at their own pace. Personal phones are for answering; the projector is for the show. This split (answer privately, celebrate publicly) is what turns a quiz into an event.

Existing tools fail this market twice over: IVR systems are numeric-only and joyless; global platforms like Kahoot are LTR-first and unreachable behind Haredi content filters. WhatsApp Clickers is built filter-safe and RTL-native from the ground up, with free-text questions validated by a three-stage pipeline (exact → fuzzy → AI semantic) that no local competitor offers. The wedge is the large Haredi family event — 30–80 participants across three generations, an organizer who is a private individual, and a built-in viral loop (every successful event seeds invitations from other family branches). The pilot proves this wedge with a handful of real events; the commercial phase then adds self-serve purchase and scales toward institutions, following the Jewish holiday calendar as a recurring marketing engine.

## 2. Target User

### 2.1 Jobs To Be Done

- **Organizer (functional):** run a live quiz for a large family gathering without technical setup, support calls, or participant friction.
- **Organizer (emotional):** be the person who made the family evening memorable; feel in control of pacing throughout.
- **Participant (functional):** join and play in seconds with the one tool they already use daily — no new interface to learn at any point.
- **Participant (social/emotional):** compete across generations; the "Saba wins" moment — a grandparent topping the leaderboard on a Torah-knowledge question — is a designed emotional climax, not an accident.
- **Community gatekeeper (contextual):** the platform must be demonstrably filter-safe (Netfree / Etrog approved) so that recommending or allowing it carries no risk.

### 2.2 Non-Users (v1 pilot)

- Institutions buying self-serve (schools, HR departments) — commercial phase.
- Non-Hebrew-speaking audiences.
- Async/remote quiz takers — this is a live, same-room experience built around a shared screen.

### 2.3 Key User Journeys

The four Key Flows in EXPERIENCE.md predate the WhatsApp-only pivot and describe a web-play model; **the journeys below are authoritative**. UJ-1 and UJ-4 still track their EXPERIENCE.md counterparts (host-side); UJ-2, UJ-3, and UJ-5 are restated for WhatsApp-only play.

- **UJ-1. Shlomo opens the event.** Shlomo prepares a 10-question game in advance. At the event he opens the lobby, sends the Audience Display to the projector — it shows "שלחו JOIN COHEN24 למספר 050-XXXXXXX" and a live participant counter climbing as family members send the message. When the room is ready he presses "התחל משחק". *(Host side tracks EXPERIENCE.md Flow 1, scaled to family events of 30–80.)*
- **UJ-2. Saba Moshe wins.** Moshe, 72, sends `JOIN COHEN24` from his kosher smartphone and gets "ברוך הבא, משה כהן! ✓". When question 3 — a halacha question — arrives as a WhatsApp message, he replies "א" first of everyone and instantly sees "התקבל ✓". On the projector the timer hits gold, Shlomo reveals, and Moshe's phone buzzes: "נכון! +100 — בונוס מהירות +50". After ten questions the projector erupts in the winner takeover: "מזל טוב, משה כהן! 1,240 נקודות" — and his phone gets the same message. The family talks about it for a week.
- **UJ-3. Rivka answers a free-text question.** The question "כתבי את שמה של אם שמשון" arrives in her WhatsApp. She types "צרורה" and sends; "התקבל ✓" comes back. Exact match grades it correct; a typo would pass fuzzy matching; "אמא של שמשון" would pass AI semantic validation. After the reveal: "נכון! +100".
- **UJ-4. Shlomo recovers a failed question.** Only 12% answered question 6 — the room was mid-conversation; Shlomo reveals, skips the leaderboard, and opens question 7, keeping the room's energy without any panic mode. *(Tracks EXPERIENCE.md Flow 4.)*
- **UJ-5. The room watches the drama.** Between answers, all eyes are on the projector: the split layout with the big timer counting down, the live "63 ענו" counter, the leaderboard reshuffling after each reveal with the top movers highlighted. Nobody looks at a personal screen except to answer. **Edge case:** a cousin replies "ב" after Shlomo closed the question — his phone answers "השאלה נסגרה — חכה לשאלה הבאה" and his answer is not counted.

## 3. Glossary

Downstream workflows must use these terms exactly.

- **Organizer** — the person who creates and runs a Game from the Host Dashboard. Synonym "host" appears only in UI-surface names inherited from the UX spec (e.g., Host Dashboard).
- **Participant** — a person registered to a Game, identified by their WhatsApp phone number. Participants interact with the Game exclusively through WhatsApp messages.
- **Host Dashboard** — the Organizer's authenticated desktop web surface: builder, lobby, live control, results.
- **Audience Display** — the projector-facing web view of the live Game (join instructions, question, countdown timer, live answer count, Leaderboard, winner takeover). Output-only: no interactive controls. Opened from the Host Dashboard.
- **Game** — one live run of a quiz: a set of Questions, scoring rules, a JOIN Code, registered Participants, and a life-cycle (lobby → questions → game over).
- **JOIN Code** — the short code a Participant sends via WhatsApp (`JOIN <code>`) to register for a specific Game.
- **Question** — a single quiz item within a Game; either **MCQ** (four lettered options א–ד, one correct) or **Free-Text** (participant types an answer). `[ASSUMPTION: fixed four options in the pilot, matching the UX design's position-as-identity principle; true/false and 2–3-option variants are deferred.]`
- **Accepted Answer** — an Organizer-defined string that counts as correct for a Free-Text Question. The AI never invents these.
- **Validation Stage** — one of three graders applied to a Free-Text answer, in order: **Exact**, **Fuzzy**, **AI Semantic**. The first stage that matches decides the grade.
- **Reveal** — the Organizer action that publishes a Question's correct answer and per-Participant grades: to the Audience Display and, as individual result messages, to Participants.
- **Speed Bonus** — extra points awarded to the first, second, and third correct answers to a Question.
- **Leaderboard** — the ranked score list shown on the Audience Display between Questions and at game end.
- **Question Bank** — the platform's library of ready-made **Question Packages** (e.g., a Chanukah pack) that an Organizer can import into a Game alongside custom Questions.
- **Spectator** — a person who attempts to join after the Game started; receives the final results message at game end but no Questions. `[ASSUMPTION: results-only spectatorship — no mid-game content for late joiners.]`
- **Universal Reply** — the guarantee that every inbound WhatsApp message to the platform's number receives a response.

## 4. Features

### 4.1 Registration & WhatsApp Inbox

**Description:** Registration is one WhatsApp message. A Participant sends `JOIN <code>` to the platform's number; the system identifies them by phone number, registers them to the matching Game, and replies with a welcome message confirming their name and telling them the game will start soon — everything they will ever need happens in this same chat. Realizes UJ-1, UJ-2. The platform never ignores a message: unrecognized text gets a short help reply in Hebrew. `[ASSUMPTION: the Participant's display name is taken from their WhatsApp profile name, correctable by reply — exact mechanism is OQ-3.]`

#### FR-1: Join via WhatsApp message
A Participant can register for a Game by sending `JOIN <code>` to the platform's WhatsApp number. Realizes UJ-1, UJ-2.

**Consequences (testable):**
- A valid code during lobby returns a welcome reply with the Participant's name, and the Organizer's lobby count (dashboard and Audience Display) increments within 3 seconds. `[ASSUMPTION: 3s lobby-count latency target.]`
- An invalid or expired code returns a Hebrew reply explaining the code was not found.
- Re-sending JOIN with the same code is idempotent — the Participant is not duplicated and receives the same welcome reply.

#### FR-2: Universal Reply
Any person can send any message to the platform number and receive a response.

**Consequences (testable):**
- An unrecognized message returns a help reply (how to join, where to get a code).
- No inbound message class results in silence — including media, stickers, and empty messages. `[ASSUMPTION: media messages get the generic help reply.]`

#### FR-3: Late join becomes Spectator
A person sending JOIN after the Game started is registered as a Spectator: informed the Game already started, and sent the final results message at game end.

**Out of Scope:** mid-game content for Spectators (no question or leaderboard pushes while the Game runs).

### 4.2 WhatsApp Gameplay

**Description:** The Game is played entirely in WhatsApp — the defining decision of this PRD (logged 2026-07-08, superseding the brief's web-play model). When the Organizer opens a Question, every Participant receives it as a message; they answer by replying; they get an immediate acknowledgment; and after Reveal they get their personal result. The projector carries the countdown and the drama (§4.3); the phone carries the private act of answering. Realizes UJ-2, UJ-3, UJ-5.

#### FR-4: Question delivery
When the Organizer opens a Question, every registered Participant receives the question text — and for MCQ, the lettered options (א–ד) — as a WhatsApp message. Realizes UJ-2, UJ-3.

**Consequences (testable):**
- Message dispatch begins immediately on question open; target delivery within 5 seconds to 95% of Participants. `[ASSUMPTION: 5s/95% delivery target — depends on WhatsApp provider throughput; architecture must validate at 80 and at 500 Participants.]`
- The message states how to answer and the time limit (e.g., "יש לכם 20 שניות — השיבו באות א-ד"). The authoritative countdown runs on the Audience Display. `[ASSUMPTION: time limit stated in the message text; no per-second updates in WhatsApp.]`

#### FR-5: Answer intake with immediate acknowledgment
A Participant can answer an open Question by replying in WhatsApp; the system immediately acknowledges receipt without disclosing the grade.

**Consequences (testable):**
- MCQ replies are accepted as an option letter (א–ד) or digit (1–4). `[ASSUMPTION: both formats accepted.]`
- Free-Text replies are limited to 200 characters; longer messages are rejected with a hint.
- A valid answer to an open Question returns "התקבל ✓" and is recorded with its server receipt timestamp.
- A second answer to the same Question returns a reply that the first answer already counts — selection is final.
- An unparseable reply during an open MCQ returns a short format hint, and the Participant may still answer.

#### FR-6: Result after Reveal
After the Organizer reveals a Question, each Participant who answered receives their grade (נכון / לא נכון), points earned including any Speed Bonus, and their current rank. `[ASSUMPTION: rank included in the result message.]`

**Consequences (testable):**
- Grades are never sent before the Organizer's Reveal action (prevents answer leakage into the room while the Question is open).
- Participants who did not answer receive no per-question message. `[ASSUMPTION: silence for non-answerers — avoids scolding; the room's screen already shows the reveal.]`
- At game end, every Participant (and Spectator) receives a final results message (their score, rank, and the winner's name).

#### FR-7: Closed-question handling
An answer arriving after the Question closed is not counted and receives a polite Hebrew reply that the question closed. Realizes UJ-5 edge case.

**Consequences (testable):**
- The answer window ends at a single server-side cutoff: the earlier of the Question's timer expiry or the Organizer's explicit close action. An answer whose receipt timestamp is after the cutoff is rejected. `[ASSUMPTION: shared cutoff rule.]`

#### FR-8: One answer per Participant
A Participant has exactly one recorded answer per Question — the first valid reply received; later attempts get the "already answered" feedback (FR-5).

**Feature-specific NFRs:**
- Message-volume envelope: a Game of P Participants and Q Questions generates on the order of P×Q×2–3 outbound messages (question, ack, result for answerers) plus registration and final results — ~2,000 messages for a 60-person, 10-question Game. Cost and rate-limit implications are an architecture concern (OQ-1, OQ-6; addendum).

### 4.3 Audience Display

**Description:** The shared screen is the game's stage. From the Host Dashboard, the Organizer opens the Audience Display — a separate, output-only web view designed for projection — and it follows the Game state live: lobby with join instructions and a climbing participant counter; the open Question with its options, a big countdown timer, and a live answer count; the Reveal with the correct answer; the Leaderboard between Questions; and the full-screen winner takeover with celebration at game end. Visual identity per DESIGN.md (festival green, gold reserved for the final-five-seconds and the winner, filter-safe geometric celebration). This replaces EXPERIENCE.md's per-participant screens and promotes its v2 "presentation mode" assumption to pilot-essential (logged 2026-07-08). Realizes UJ-1, UJ-5.

#### FR-9: Live projection view
An Organizer can open the Audience Display from the Host Dashboard as a separate browser window suitable for a projector, and it renders the current Game state, transitioning automatically on Organizer actions with no interaction of its own. `[ASSUMPTION: separate browser window on the Organizer's machine, dragged/mirrored to the projector; no second device or pairing flow.]`

**Consequences (testable):**
- State transitions render within 1 second of the Organizer action. `[ASSUMPTION: 1s real-time target.]`
- The countdown timer on the Audience Display is the authoritative visible timer; at ≤5 seconds it shifts to the gold urgency treatment (DESIGN.md).
- If the display's connection drops, it auto-reconnects and re-renders the current state without Organizer intervention.

#### FR-10: Stage content per state
The Audience Display shows, per Game state: lobby (JOIN instructions + live participant count), open Question (question, MCQ options, timer, live answer count), Reveal (correct answer marked; answer distribution), Leaderboard (top ranks with movement indicators), winner takeover (winner name, score, celebration). `[ASSUMPTION: answer distribution shown on Reveal; top-10 leaderboard depth — final content per state is OQ-7 UX work.]`

**Out of Scope:** per-Participant private information on the shared screen (individual grades appear only in each Participant's WhatsApp).

### 4.4 Host Dashboard

**Description:** The Organizer's desktop surface: build the Game in advance, run the lobby, drive the live state machine, and see results. The control panel enforces exactly one primary action per state with no auto-advance — pacing is always the Organizer's decision (EXPERIENCE.md, which remains authoritative for this surface). In the pilot, Organizer accounts are provisioned manually (§6.2); the dashboard still requires authenticated access. `[ASSUMPTION: lightweight auth for the pilot — manually provisioned credentials or magic link; self-serve signup arrives with the commercial phase.]` Realizes UJ-1, UJ-4.

#### FR-11: Game builder
An Organizer can sign in to their dashboard and create a Game, add and edit MCQ and Free-Text Questions (including Accepted Answers and per-question time limits), configure scoring rules, and receive its JOIN Code. `[ASSUMPTION: per-question configurable time limit; default value TBD.]`

**Consequences (testable):**
- A Game and its Questions are accessible only to the Organizer who owns them.

#### FR-12: Question Bank import
An Organizer can browse the Question Bank and import a Question Package (e.g., Chanukah pack) into a Game, then mix, reorder, edit, or delete imported Questions alongside custom ones.

**Consequences (testable):**
- Imported Questions are copies — editing them does not modify the Question Bank.

**Notes:** `[NOTE FOR PM]` Pilot content: at least one complete Question Package must exist before the first pilot event; sourcing is Open Question OQ-4. The pilot Package must be **multigenerational by design** — mixed difficulty across generations, including a Torah-knowledge category the older generation dominates. This is what makes the "Saba wins" moment (UJ-2) a designed climax rather than an accident (brainstorming ideas #9, #13). A per-question category label is deferred (addendum).

#### FR-13: Lobby and live control
An Organizer can open the lobby (see the participant count rise and the participant list live), launch the Audience Display, start the Game, and drive it through the state machine: open Question → close Question → Reveal → (Leaderboard) → next Question or end Game — with the Leaderboard skippable (UJ-4).

**Consequences (testable):**
- Live response count and percentage update on the dashboard while a Question is open.
- No state can be skipped accidentally; every advance is an explicit Organizer action.

#### FR-14: Post-game results on screen
At game end the Organizer sees a results summary (final Leaderboard, per-question response rates) on the dashboard.

**Out of Scope:** Result export/download — deferred to the commercial phase (decision logged 2026-07-08).

### 4.5 Answer Validation

**Description:** MCQ grades mechanically. Free-Text answers pass through three Validation Stages in order — Exact match against Accepted Answers, Fuzzy match for typos and spelling variants, AI Semantic validation for equivalent wording — stopping at the first match. The Organizer defines all Accepted Answers; the AI only judges equivalence, never generates correctness. The matching stage is recorded. Realizes UJ-3.

#### FR-15: MCQ auto-grading
The system grades MCQ answers against the Organizer-defined correct option; grades are published to Participants only at Reveal.

#### FR-16: Three-stage Free-Text validation
The system grades a Free-Text answer via Exact → Fuzzy → AI Semantic stages against the Accepted Answers, recording which stage matched.

**Consequences (testable):**
- An exact string match never invokes the fuzzy or AI stages.
- The AI stage is invoked only when Exact and Fuzzy both fail.
- If the AI stage is unavailable (timeout/error), the answer is graded by the first two stages only and the degradation is logged. `[ASSUMPTION: fail-closed to non-AI grading rather than blocking the Reveal.]`
- All grading for a Question completes before the Organizer's Reveal is published; the Reveal control activates only once every received answer is graded. `[ASSUMPTION: grading runs as answers arrive, so any residual wait at Reveal covers only last-second answers still in the AI stage.]`

### 4.6 Scoring & Leaderboard

**Description:** Scoring rewards correctness and speed: a configurable point value per correct answer plus Speed Bonuses for the three fastest correct answers. With WhatsApp as the only participant surface, all Participants compete on a level field — question messages go out together and answers are timestamped on server receipt. The Leaderboard lives on the Audience Display between Questions and culminates in the winner takeover; each Participant's personal rank arrives in their WhatsApp result messages. Realizes UJ-2, UJ-5.

#### FR-17: Configurable scoring with Speed Bonus
An Organizer can set points per correct answer and Speed Bonus values; the system awards them automatically based on answer receipt timestamps.

**Consequences (testable):**
- Speed Bonuses go to at most the first, second, and third *correct* answers; if fewer correct answers exist, fewer bonuses are awarded.
- Ordering is by server receipt timestamp; identical timestamps break ties by server processing order. `[ASSUMPTION: server-order tie-breaking.]`
- Leaderboard ranks tie on equal scores; tied Participants share a rank. `[ASSUMPTION: shared-rank ties.]`

#### FR-18: Live Leaderboard and winner
The system maintains a live Leaderboard published after each Reveal on the Audience Display and names the winner at game end everywhere: the winner takeover on the Audience Display and the final results message in every Participant's WhatsApp.

**Out of Scope:** the full ranked Leaderboard is an Audience Display surface; Participants receive their own rank in result messages (FR-6), not the full list.

### 4.7 Cross-Cutting Constraints (all features)

- **Privacy.** Phone numbers are PII and the platform's identity key. Store the minimum (number, display name, per-Game scores); no sharing with third parties; no marketing use of Participant numbers without consent. Participant display names are visible to the whole room on the Audience Display Leaderboard — the welcome reply is the implicit notice. `[ASSUMPTION: pilot-level privacy posture — retention period and deletion policy to be set before the commercial phase; Israeli Privacy Protection Law applicability reviewed then.]`
- **Scale.** Pilot operates at 30–80 Participants per Game; the architecture must not preclude the 500-Participant commercial target (brief). WhatsApp throughput is the binding constraint (FR-4, OQ-1).
- **Reliability.** Server-authoritative Game state throughout: the dashboard or Audience Display can disconnect and reconnect without score loss or state corruption; an acknowledged answer is never lost (SM-4). A live event cannot be paused for debugging — degraded modes must be silent and self-healing.
- **Hebrew, RTL, filter-safety.** Owned by DESIGN.md and binding on every surface, including WhatsApp message copy and the Audience Display: Hebrew-native text, no external asset dependencies that content filters block, no imagery of people.
- **Cost.** WhatsApp message volume and AI-validation calls are the two variable costs; both scale with Participants × Questions. Pilot budget ceiling is OQ-6; SM-C2 forbids cutting the reply guarantees to save cost.

## 5. Non-Goals (Explicit)

- **No participant-facing web or app surface.** The participant experience is WhatsApp-only by design; the only web surfaces are the Organizer's (Host Dashboard, Audience Display). This is the product's identity, not a temporary cut.
- Not an async learning/LMS tool — live events only.
- Not a chat platform — WhatsApp interactions are structured game messages, not conversation; there is no human operator behind the number.
- Not multi-language in the pilot or v1 — Hebrew only.
- Not a content marketplace — the Question Bank is first-party in the pilot.
- No sound effects, no team/group scoring modes (v2 candidates per EXPERIENCE.md).

## 6. Pilot Scope (MVP)

### 6.1 In Scope

- WhatsApp registration with Universal Reply (§4.1).
- WhatsApp-only gameplay: question delivery, answer intake with ack, post-Reveal results, final results (§4.2).
- Audience Display for projection: lobby, question + timer, reveal, Leaderboard, winner takeover (§4.3) — promoted from EXPERIENCE.md's v2 assumption.
- Host Dashboard: builder, Question Bank import, lobby, live control, on-screen results (§4.4).
- Three-stage Free-Text validation including AI Semantic (§4.5).
- Configurable scoring, Speed Bonuses, live Leaderboard, winner moment (§4.6).
- At least one ready-made Question Package (holiday-themed) in the Question Bank, multigenerational by design with a Torah-knowledge category (see FR-12 Notes).
- Hebrew-first, RTL-native, filter-safe presentation throughout (per DESIGN.md).

### 6.2 Out of Scope for Pilot

- **Participant web interface** — removed by the WhatsApp-only pivot (2026-07-08); the participant mobile screens in the UX documents are superseded, not deferred.
- **Self-serve purchase and payments** — Avraham provisions Games for pilot organizers manually; commercial phase adds the website purchase flow. `[NOTE FOR PM]` Load-bearing for the business model — revisit immediately after a successful pilot.
- **Result export/download** — on-screen summary only; export lands with the commercial phase.
- **Marketing site** (landing, pricing) — commercial phase.
- **Filter-provider certification process** (Netfree/Etrog approval) — runs in parallel as a business task; the product constraint (filter-safe design) is in scope, the bureaucratic approval is not a software deliverable.
- Everything in §5 Non-Goals.

## 7. Success Metrics

Pilot success is defined qualitatively-first (decision logged 2026-07-08): several real family events that run smoothly.

**Primary**
- **SM-1**: 3–5 real family events (30–80 Participants each) completed end-to-end with no significant technical failure (no aborted Game, no mass message-delivery failure, no scoring dispute the Organizer couldn't resolve). Validates FR-1–FR-18 collectively.
- **SM-2**: Every pilot Organizer states they would run another event with the platform. Validates the overall experience; proxy for the viral loop.

**Secondary**
- **SM-3**: ≥90% of Participants who attempt to join succeed within 2 minutes of their first message (brief's registration-friction goal). Validates FR-1, FR-2.
- **SM-4**: Zero lost answers — every acknowledged answer is graded and reflected in the Leaderboard. Validates FR-5, FR-8, FR-15–FR-17.
- **SM-5**: At every pilot event, Participants from at least three generations answer Questions without assistance — the WhatsApp-only model works for grandparents and teenagers alike. Validates FR-4–FR-8 and the multigenerational content requirement (FR-12 Notes).
- **SM-6**: At least one pilot Game is built end-to-end by its Organizer without assistance. Early validation of the self-serve premise the commercial phase depends on — pilot-phase manual provisioning otherwise hides builder-usability problems (brief's "host runs a full quiz without external support"). Validates FR-11, FR-12.

**Counter-metrics (do not optimize)**
- **SM-C1**: AI validation leniency — do not tune the AI Semantic stage toward accepting more answers to make players happier; track Organizer-reported wrong grades (both directions). Counterbalances SM-2.
- **SM-C2**: Message thrift — do not reduce WhatsApp message volume (e.g., dropping acks or Universal Reply) to cut costs during the pilot; the reply guarantee is the product's trust signal. Counterbalances cost pressure on FR-2/FR-5.

## 8. Open Questions

1. **OQ-1 — WhatsApp provider:** Official WhatsApp Business (Cloud API via Meta or a BSP) vs. other integration paths — cost per message, throughput/rate limits at 80–500 participants, template-message rules, and number acquisition. With WhatsApp as the sole participant surface, delivery throughput now directly shapes game pacing. Owner: architecture workflow. Blocking for architecture, not for this PRD.
2. **OQ-2 — Speed-Bonus fairness across channels:** **Resolved 2026-07-08** — the WhatsApp-only pivot leaves a single channel and a level field. Kept for traceability.
3. **OQ-3 — Participant display name flow:** WhatsApp profile name vs. asking the Participant to reply with their name. Now the only naming mechanism (no web form exists). Owner: UX + Avraham.
4. **OQ-4 — Question Package sourcing:** who writes the first holiday pack (Avraham, a hired writer, or a content partner per brainstorming idea #11)? Owner: Avraham.
5. **OQ-5 — Organizer disconnect mid-Game:** the Audience Display holds last state and reconnects (FR-9); define what, if anything, Participants are told if the Organizer is gone for minutes. Owner: UX.
6. **OQ-6 — Pilot infrastructure/AI budget:** monthly cost ceiling for WhatsApp messages + AI validation during the pilot. Owner: Avraham.
7. **OQ-7 — UX design for the pivoted surfaces:** the participant mobile screens in the final UX docs are superseded; new UX work is needed for (a) the **WhatsApp conversation design** — message templates, tone, answer-format hints, non-answerer handling, winner-moment copy; and (b) the **Audience Display** — per-state content and layout (FR-10), adapting DESIGN.md's split-hero, timer, leaderboard, and winner-card language from phone screens to projection scale. Also: Question Bank browse/import and per-question time limit in the builder. Owner: UX workflow (`bmad-ux` revision). **Blocking** for story creation on §4.2 and §4.3; not blocking for architecture.

## 9. Assumptions Index

- §1/§2 — the WhatsApp-only model works across all generations at a live event (pilot validates; SM-5).
- §3 Glossary — MCQ is fixed at four options (א–ד) in the pilot; fewer-option variants deferred.
- §3 Glossary — Spectators get results-only (no mid-game content).
- §4.1 FR-1 — 3-second lobby-count latency target.
- §4.1 — display name sourced from WhatsApp profile, correctable by reply (OQ-3).
- §4.1 FR-2 — media/sticker messages receive the generic help reply.
- §4.2 FR-4 — 5s/95% WhatsApp question-delivery target, provider-dependent.
- §4.2 FR-4 — time limit stated in the question message text; authoritative countdown on the Audience Display only.
- §4.2 FR-5 — MCQ answers accepted as letter (א–ד) or digit (1–4).
- §4.2 FR-6 — per-question result message includes current rank; non-answerers get no per-question message.
- §4.2 FR-7 — single server-side answer cutoff (earlier of timer expiry / Organizer close).
- §4.3 FR-9 — Audience Display is a separate browser window on the Organizer's machine (no second-device pairing); 1s state-transition target.
- §4.3 FR-10 — Reveal shows answer distribution; top-10 Leaderboard depth (final per-state content is OQ-7).
- §4.4 — lightweight pilot auth for Organizers (manually provisioned credentials or magic link).
- §4.4 FR-11 — per-question configurable time limit, default TBD.
- §4.5 FR-16 — AI-stage failure degrades to two-stage grading (fail-closed); grading runs as answers arrive; Reveal control activates when grading completes.
- §4.6 FR-17 — server-order tie-breaking for simultaneous answers; tied scores share a Leaderboard rank.
- §4.7 — pilot-level privacy posture; retention/deletion policy and Israeli Privacy Protection Law review deferred to the commercial phase.
