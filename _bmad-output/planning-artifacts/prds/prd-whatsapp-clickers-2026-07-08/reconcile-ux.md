# UX ↔ PRD Reconciliation — WhatsApp Clickers

- **Date:** 2026-07-08
- **Sources compared:**
  - `ux-designs/ux-whatsapp-clickers-2026-07-05/DESIGN.md` (status: final)
  - `ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md` (status: final)
  - `prds/prd-whatsapp-clickers-2026-07-08/prd.md` (status: draft)
  - `prds/prd-whatsapp-clickers-2026-07-08/addendum.md`

## Deliberate decisions respected (NOT reported as gaps)

1. **WhatsApp as full Play Channel** supersedes EXPERIENCE.md "Rejected — WhatsApp for question delivery" (PRD §0, §4.2; addendum "Overrides"). The *decision* is not questioned below — only its missing UX coverage is flagged (F-1), which the override note itself does not resolve.
2. **Result export deferred** out of the pilot (PRD FR-14 Out of Scope; addendum), despite the "הורד תוצאות" CTA in EXPERIENCE.md's host state machine. Only the unaddressed knock-on to the Game-over state is flagged (F-8).
3. **UJ-1–UJ-4 mirror EXPERIENCE.md's Key Flows** — verified: the four UJ summaries are faithful to Flows 1–4; the only divergence (200-person corporate event vs. 30–80 pilot scale) is flagged inline by the PRD itself. No silent drift found.
4. The PRD intentionally **references rather than duplicates** the UX docs for the Web Channel (§4.3 "this PRD adds no deltas"). Web-Channel behaviors covered by that delegation (states, microcopy, reconnection, accessibility floor, portrait lock, platform minimums for the participant surface) are treated as covered, not as gaps.

---

## Findings

Severity: **HIGH** = blocks or destabilizes downstream architecture/epics work; **MEDIUM** = must be resolved before dev of the affected feature; **LOW** = should be logged, not blocking.

### Category (c) — New PRD concepts with no UX coverage (UX docs are final ⇒ implies new UX work)

#### F-1 (HIGH) — The WhatsApp Play Channel has zero UX design, and the PRD is authoring UX inline
The override legitimizes the *channel*, but the final UX docs contain nothing for it: EXPERIENCE.md's IA, Voice & Tone tables, Component Patterns, State Patterns, and Accessibility Floor are all web-only. Meanwhile the PRD embeds conversation microcopy directly in requirements — "התקבל ✓" (FR-5), "השאלה נסגרה — חכה לשאלה הבאה" (UJ-5), "נכון! +100 — מקום 12 מתוך 63" (UJ-5), "השיבו באות א-ד" / "השיבו בהודעת טקסט" (FR-4) — copy that under this workflow's division of labor belongs to a UX artifact, subject to the same voice rules ("written for the first three words", no vague errors) that govern web microcopy.

Undesigned surface area includes at minimum:
- Message templates for: welcome, help/Universal Reply, question (MCQ vs Free-Text), ack, duplicate-answer, format-hint, closed-question, per-question result, final result, spectator reply.
- **Time-limit communication:** web players see a countdown ring; the FR-4 question message says how to answer but not how long they have. EXPERIENCE.md's "participant cannot see the total duration" principle can't even apply — WhatsApp players see nothing.
- **Correct-answer disclosure parity:** EXPERIENCE.md Flow 3 shows wrong web answerers the correct answer ("לא נכון. התשובה: צרורה") and Free-Text reveal shows the matched stage (exact/fuzzy/AI). FR-6's result message contains grade, points, rank — no correct answer, no stage. Deliberate or dropped? Unstated.
- **Non-answerers:** FR-6 sends results only to Participants "who answered." Web non-answerers see a locked/reveal state; WhatsApp non-answerers get silence between question and next question — in tension with the Universal-Reply trust posture.
- **Winner-moment parity:** the "Saba wins" climax (PRD §2.1, a *designed* emotional beat) is a gold full-screen takeover on web and an ordinary text message on WhatsApp. The addendum flags speed-bonus skew undermining Saba-wins but not the experience gap itself.
- **Voice/format rules for the channel** (message length, emoji policy, RTL/letter conventions in plain text where DESIGN.md's visual identity cannot apply).

**Action:** commission a WhatsApp-Channel UX addendum (or EXPERIENCE.md v2 section) before architecture freezes message flows; move PRD-embedded strings there and have FRs reference it, as §4.3 does for web.

#### F-2 (HIGH) — Question Bank browse/import is a new host surface absent from the UX docs
PRD FR-12 requires browsing the Question Bank, importing a Question Package, then mixing/reordering/editing/deleting imported questions alongside custom ones — and §6.1 makes it pilot scope. EXPERIENCE.md's host IA has no such surface; its Game builder row covers only "Create game, add/edit questions (MCQ + free text), set scoring rules." Related: FR-11 adds **per-question time limits** to the builder, also not present in the builder's UX description. The builder overall has the thinnest UX coverage of any pilot-critical surface (one IA row; no flow, no component patterns, no mockup) while carrying two FRs.

**Action:** flag Game-builder + Question-Bank-import UX design as required pre-dev work.

#### F-3 (MEDIUM) — Spectator is a first-class PRD concept with a one-line UX footprint
The PRD Glossary defines Spectator, and FR-3 registers late joiners as Spectators with a Web Channel link "to watch the Leaderboard." In EXPERIENCE.md this exists only as a state-pattern row and a Flow-1 failure message ("ניתן לצפות בלוח הניקוד") — there is **no spectator surface** in the participant IA (the eight surfaces all assume an answering player). Undesigned: what a spectator sees during an open question, during reveal, at game end (winner takeover?); whether their screen auto-advances; whether they appear in the participant count the host sees.

**Action:** add a spectator screen/state to the UX spec; PRD should state whether Spectators are excluded from the lobby/participant counts (affects FR-1's count consequence and FR-13).

### Category (a) — UX behaviors/constraints creating requirements the PRD should own but doesn't

#### F-4 (HIGH) — Play-Channel assignment is undefined, yet five requirements depend on it
Every Participant registers via WhatsApp (FR-1) and every welcome reply includes the Web Channel link — so *every* Participant is potentially dual-channel. The Glossary says a Participant "plays through exactly one Play Channel at a time but may use either," but nothing defines how a Participant *becomes* a WhatsApp-Channel Participant: opt-in keyword? default-until-link-clicked? sticky per question? This is load-bearing for: FR-4 (who receives question messages — if it's everyone, the P×Q×3 message-volume NFR and the addendum's ~2,000-message envelope roughly double and web players get duplicate stimuli), FR-8 (cross-channel dedup), OQ-2 (fairness), OQ-6 (budget), and SM-5 (identifying "active WhatsApp-Channel players"). It is also a UX question (does a web-connected player still get WhatsApp pings?).

**Action:** add an FR (or extend FR-4) defining channel assignment and switching semantics; mirror into the WhatsApp UX work (F-1).

#### F-5 (HIGH) — "Question closed" cutoff semantics are undefined across channels
EXPERIENCE.md defines two distinct closing mechanisms on web: the timer locks answer options **immediately at 0s** ("regardless of selection state"), while the host's "סגור שאלה" is a separate explicit state-machine action (no auto-advance). The PRD never reconciles these: FR-7 rejects answers "arriving after the Question closed" and FR-13 has the Organizer close questions, but doesn't say whether the server-side answer cutoff is timer expiry or the Organizer's close action. On the Web Channel the client lock makes it moot; on the WhatsApp Channel there is no client lock — if cutoff = host action, a WhatsApp reply arriving at timer+5s (before the host clicks close) counts while every web player was already locked at 0s. That is a scoring-integrity issue beyond OQ-2 (which covers only speed-bonus latency, not answer-window length).

**Action:** PRD should own a single cutoff definition (recommend: timer expiry closes answer intake on all channels; host "close" only advances the state machine) and add it as an FR-7/FR-13 consequence.

#### F-6 (MEDIUM) — Organizer access/authentication is unspecified after dropping the Account surface
EXPERIENCE.md's host IA includes "Account / programs — Purchase access, manage owned programs." The PRD deliberately defers self-serve purchase (§6.2, manual provisioning by Avraham) — but silently drops the rest: no FR states how a pilot Organizer reaches their provisioned Game (login? magic link? shared credentials?), whether one Organizer can hold multiple Games, or how a Game moves from "built in advance" (UJ-1) to "run on event night." Even a manual-provisioning pilot needs an access mechanism, and it's a product decision (security/simplicity trade-off), not a pure architecture detail.

**Action:** add a minimal FR for Organizer access in the pilot (even if it's "single-use dashboard link issued at provisioning time").

#### F-7 (LOW) — Host Dashboard UX authority is never delegated the way the Web Channel's is
§4.3 explicitly assigns behavior/states/microcopy/accessibility of the *Participant Web Channel* to the UX docs. §4.4 references EXPERIENCE.md only for the state-machine principle. Host-side constraints therefore sit in the UX docs without a PRD anchor: minimum supported width 1024px / optimized 1280px+ (EXPERIENCE.md Responsive & Platform, DESIGN.md Layout), the accessibility floor as applied to the dashboard (focus rings, 48px effective hit areas), and 768px-tablet degradation tolerance. Since the Glossary's "Web Channel" is participant-only, a literal downstream reader could conclude the host dashboard has no binding UX spec.

**Action:** add one sentence to §4.4 mirroring §4.3's delegation ("layout, platform minimums, accessibility per DESIGN.md/EXPERIENCE.md; no deltas").

### Category (b) — Contradictions beyond the deliberate overrides

#### F-9 (MEDIUM) — MCQ "2–4 options" (PRD) contradicts the UX's fixed-four design
PRD Glossary: "MCQ (2–4 options, one correct)." The final UX docs design for exactly four everywhere: EXPERIENCE.md IA ("Timer + question + 4 answer options"), DESIGN.md's option-letter spec (א–ד), and the explicit rationale in Inspiration & Anti-patterns ("our four options are uniformly green, differentiated by letter (א/ב/ג/ד) and position" — *position* is part of the identity system, which changes with 2–3 options). If 2–3-option questions are pilot scope, the layout/spacing for them is undesigned; FR-4's WhatsApp message ("lettered options (א–ד)") and FR-5's digit mapping also silently assume the count. Either the PRD should say "4 options, fixed, for the pilot" or the UX docs need the variable-count variant.

#### F-10 (LOW) — FR-15 vs FR-17/FR-16 grading-timing tension (internal to PRD, surfaced by the UX speed-bonus flow)
FR-15 grades MCQ "at Reveal time," but FR-17 awards Speed Bonuses to the fastest *correct* answers by receipt timestamp, and EXPERIENCE.md Flow 2 shows the bonus computed by answer order — which requires correctness to be known per-answer before/at reveal, consistent with FR-16's "grading runs as answers arrive" assumption but not with FR-15's wording. Harmonize the wording so downstream doesn't implement two grading moments.

### Knock-ons and residue in the "final" UX docs (log for the UX owner)

#### F-8 (LOW) — Export deferral breaks the UX Game-over state
With "הורד תוצאות" deferred, EXPERIENCE.md's host state machine has a Game-over state whose **only primary CTA no longer exists in the pilot** (leaving secondary "חזור לראשית"). The PRD defers the feature but doesn't state the pilot-scope Game-over primary action. One-line fix in either doc; without it, the state-machine invariant "exactly one primary CTA active at any time" is violated at Game over.

#### F-11 (LOW) — Unresolved UX notes not carried into PRD Open Questions
- EXPERIENCE.md host keyboard shortcuts carry "[NOTE FOR UX: confirm full list before dev; not blocking]" — still unresolved in a final doc, absent from PRD §8.
- The same line says "Escape = pause / confirm-stop dialog," but no pause state exists in the host state machine (EXPERIENCE.md's own table) or the PRD. Clarify whether "pause" is a real state (if so, it's a missing FR) or stale wording.

### Checked and found consistent (no action)
- Join flow, welcome-with-link, idempotent JOIN — FR-1 matches EXPERIENCE.md Foundation/Flow 1.
- Free-text behavior: explicit submit, 200-char limit, three-stage validation with matched stage shown on web Reveal — FR-10/FR-16 match EXPERIENCE.md Flow 3 and component table.
- Host pacing: no auto-advance, explicit actions, skippable leaderboard — FR-13 matches the state machine and Flow 4.
- Non-goals: sound effects, presentation mode — PRD §5 matches EXPERIENCE.md's rejections/assumptions.
- Filter-safe, Hebrew-native, RTL — PRD §6.1 anchors to DESIGN.md.
- Late-join message and web spectator *entry* behavior — FR-3 matches EXPERIENCE.md state patterns (design depth gap covered by F-3).
- Host-disconnect for WhatsApp players — already tracked as OQ-5; participant display name — already tracked as OQ-3.

## Priority summary

| # | Sev | Type | Finding |
|---|-----|------|---------|
| F-1 | HIGH | (c) | WhatsApp Play Channel has no UX design; PRD is authoring conversation microcopy inline |
| F-4 | HIGH | (a) | Channel assignment/switching undefined — load-bearing for FR-4/FR-8, message volume, OQ-2/OQ-6, SM-5 |
| F-5 | HIGH | (a) | Answer-cutoff semantics undefined: web locks at timer 0s, WhatsApp intake window ambiguous → scoring-integrity risk |
| F-2 | HIGH | (c) | Question Bank import + enriched Game builder (per-question time limits) have no UX coverage |
| F-9 | MED | (b) | PRD's "MCQ 2–4 options" contradicts UX's fixed-four (א–ד, position-based identity) design |
| F-3 | MED | (c) | Spectator is a first-class PRD concept with no designed surface |
| F-6 | MED | (a) | Organizer access/auth unspecified after Account/programs surface dropped |
| F-7 | LOW | (a) | Host Dashboard lacks the explicit UX-authority delegation the Web Channel has |
| F-8 | LOW | knock-on | Game-over state's primary CTA (export) deferred with no pilot replacement named |
| F-10 | LOW | (b) | FR-15 "grade at Reveal" vs speed-bonus/as-answers-arrive grading wording |
| F-11 | LOW | residue | Keyboard-shortcut confirmation note and phantom "pause" state unresolved in final UX doc |
