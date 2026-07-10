# Reconciliation Review — Brief → PRD + Addendum

**Source input:** `briefs/brief-whatsapp-clickers-2026-06-18/brief.md`
**Compared against:** `prds/prd-whatsapp-clickers-2026-07-08/prd.md` + `addendum.md`
**Date:** 2026-07-08
**Reviewer context honored (deliberate decisions, not gaps):** (1) WhatsApp upgraded to full Play Channel, superseding the brief's exclusion; (2) self-serve purchase, payments, and result export deferred to the commercial phase; (3) pilot calibrated to family events (30–80), 500-participant scale kept as commercial design target.

**Classification legend:**
- **PRD** — covered in the PRD (FR/SM/section cited)
- **ADD** — covered in the addendum
- **DEF/OVR** — deliberately deferred or overridden per the context above (decision-logged)
- **GAP** — silently dropped (no coverage, no parked-backlog entry, no logged decision)

## Reconciliation Table

| # | Brief item | Kind | Classification | Where / Notes |
|---|-----------|------|----------------|---------------|
| 1 | Real-time interactive quiz platform for the Israeli market | Positioning | PRD | §1 Vision |
| 2 | Organizers = school principals, HR managers, event coordinators (institutional primary buyer) | Requirement (audience) | DEF/OVR | PRD §2.2 lists institutions as pilot Non-Users; §1 "scales toward institutions" in commercial phase. Deliberate per context (3) |
| 3 | Groups of up to 500 participants per session | Constraint (scale) | DEF/OVR + ADD | Pilot 30–80 (PRD §0, SM-1); 500 kept as commercial design target (addendum "Throughput" + "Overrides") |
| 4 | Registration via single WhatsApp message — no app, no account, no URL | Requirement | PRD | FR-1; §1 Vision; JTBD |
| 5 | IVR competitive failure (uncomfortable, numeric-only, no visual feedback / leaderboard energy) | Positioning | PRD | §1 "IVR systems are numeric-only and joyless" |
| 6 | Kahoot competitive failure — URL + game-code coordination friction at scale | Positioning | PRD | §1 (reframed: "LTR-first and unreachable behind Haredi content filters") |
| 7 | `[ASSUMPTION]` Kahoot's Hebrew experience is poor enough to be a real barrier — **to be validated with target users** | Assumption to validate | **GAP** | The PRD replaces this rationale with the filter-safety / kosher-smartphone hypothesis (validated via SM-5), but the original competitive assumption and its validation task vanish — not in PRD §9 Assumptions Index, not in §8 Open Questions, not in addendum. See Note G-3 |
| 8 | Hebrew-native / RTL built from the ground up ("Hebrew-native, not Hebrew-compatible") | Qualitative differentiator | PRD | §1 "filter-safe and RTL-native from the ground up"; §6.1 |
| 9 | Hebrew-language AI validation as differentiator | Capability | PRD | §4.5, FR-16; UJ-3 uses Hebrew examples. (Minor: no explicit NFR that fuzzy/AI stages handle Hebrew-specific variance — ktiv male/haser, final letters; see Note M-2) |
| 10 | Flat fee per program, purchase directly on website, immediate access | Requirement (business) | DEF/OVR + ADD | PRD §6.2 (decision logged); addendum backlog: flat-fee / two-tier pricing, marketing site |
| 11 | JOIN `<code>` to a designated WhatsApp number; identified by phone number; confirmation reply | Requirement | PRD | FR-1; Glossary (Participant, JOIN Code) |
| 12 | Phone number as identity — no logins, no passwords, no registration forms | Qualitative differentiator | PRD | Glossary "Participant"; §1 Vision; JTBD |
| 13 | Participants join a dedicated web interface | Requirement | PRD | §4.3 Web Channel (FR-9, FR-10); UX docs authoritative |
| 14 | View question + countdown timer on web | Requirement | PRD | §4.3 description (split-hero screen with countdown timer) |
| 15 | Submit answers on web | Requirement | PRD | FR-10 |
| 16 | See personal score and live leaderboard | Requirement | PRD | §4.3 (Leaderboard with current Participant pinned); FR-18 |
| 17 | Question type: multiple choice, single correct answer | Requirement | PRD | Glossary MCQ; FR-15. (Narrowed to 2–4 options — brief set no count; harmless narrowing, Note M-1) |
| 18 | Question type: free text, answer in own words | Requirement | PRD | Glossary Free-Text; FR-16; UJ-3 |
| 19 | Validation stage 1: exact match against host-defined accepted answers | Requirement | PRD | FR-16; Glossary Validation Stage |
| 20 | Validation stage 2: fuzzy match for spelling variations and typos | Requirement | PRD | FR-16; UJ-3 |
| 21 | Validation stage 3: AI semantic — only when first two stages fail | Requirement | PRD | FR-16 consequence ("AI stage invoked only when Exact and Fuzzy both fail") |
| 22 | Host defines all accepted answers; AI never generates correct answers | Constraint | PRD | §4.5; Glossary Accepted Answer ("The AI never invents these") |
| 23 | Scoring: configurable points per correct answer | Requirement | PRD | FR-17 |
| 24 | Speed bonus for first, second, third correct responses | Requirement | PRD | FR-17; Glossary Speed Bonus (+ OQ-2 / addendum on cross-channel fairness) |
| 25 | Free text with AI validation "at scale" — unlocks trivia, knowledge competitions, educational assessments | Qualitative (use-case ambit) | PRD (partial) | Trivia/competition covered; "educational assessments" follows institutions into the commercial phase (implicitly deferred with #2). Live-only Non-Goal (§5) excludes async assessment. Note M-3 |
| 26 | Built for live events — real-time leaderboards, countdown timers, instant answer feedback, "energy of a room" | Qualitative differentiator | PRD | §2.1 emotional JTBD; UJ-2 "Saba wins" climax; UJ-4 "keeping the room's energy"; FR-9, FR-18 |
| 27 | Participants non-technical; should not need to learn anything new | Qualitative constraint | PRD | JTBD "join in seconds with a tool they already use daily"; Universal Reply (FR-2) |
| 28 | Only required pre-event action: one WhatsApp message | Requirement | PRD | FR-1; §1 Vision |
| 29 | Success: participants connect in under 2 minutes at event start | Success signal | PRD | SM-3 (≥90% within 2 min; explicitly cites the brief's goal) |
| 30 | Success: **zero** participants fail to register due to technical friction | Success signal | PRD (relaxed) | SM-3 relaxes "zero fail" to "≥90% succeed" — but does so citing the brief, so not silent. Note M-4 |
| 31 | Success: host can create and run a full quiz **without external support** | Success signal | **GAP** | No SM measures organizer self-sufficiency. SM-1 measures technical failure, SM-2 measures satisfaction — neither tests unaided operation. Pilot design actively works against it (Avraham manually provisions Games, §6.2). See Note G-1 |
| 32 | Success: organizer feels it went smoothly and would use the platform again | Success signal | PRD | SM-2 |
| 33 | Business success: paying customers within first quarter of launch | Success signal | DEF/OVR (partial) → **GAP** | Follows from the payments deferral, but the addendum's commercial backlog parks *features* (pricing, export, site) while the brief's *business success criteria* are not parked anywhere. See Note G-2 |
| 34 | Business success: NPS from organizers sufficient to drive word-of-mouth | Success signal | PRD (adapted) / partial GAP | SM-2 is stated as "proxy for the viral loop"; §1 names the built-in viral loop. NPS-as-metric itself is dropped without a parked pointer — folded into Note G-2 |
| 35 | Business success: repeat purchase rate | Success signal | DEF/OVR (partial) → **GAP** | No purchases in pilot (deliberate); but the metric is not carried into the deferred commercial backlog. Folded into Note G-2 |
| 36 | v1: host dashboard — create games, create/manage MCQ + free-text questions | Requirement | PRD | FR-11 |
| 37 | v1: open/close questions | Requirement | PRD | FR-13 |
| 38 | v1: view live results | Requirement | PRD | FR-13 (live response count/%) + FR-14 (post-game summary) |
| 39 | v1: real-time updates — participant count, answers received, live rankings | Requirement | PRD | FR-1 (lobby count), FR-13, FR-9, FR-18 |
| 40 | v1: result export | Requirement | DEF/OVR + ADD | PRD FR-14 Out of Scope (decision logged); addendum backlog (+ "memory PDF" upsell) |
| 41 | v1: Hebrew-first UI throughout | Requirement | PRD | §6.1 "Hebrew-first, RTL-native, filter-safe UI throughout" |
| 42 | v1: self-serve purchase and access on the website | Requirement | DEF/OVR + ADD | PRD §6.2 with `[NOTE FOR PM]` load-bearing flag; addendum backlog |
| 43 | Out of v1: mobile app | Exclusion | PRD | §5 Non-Goals ("web or WhatsApp only") |
| 44 | Out of v1: WhatsApp-delivered question display | Exclusion | DEF/OVR | **Deliberately superseded** — WhatsApp is a full Play Channel (§4.2; decision logged 2026-07-08; addendum "Overrides") |
| 45 | Out of v1: multi-language beyond Hebrew | Exclusion | PRD | §5 Non-Goals; §2.2 Non-Users |
| 46 | Out of v1: advanced analytics/reporting beyond basic export | Exclusion | PRD | Preserved implicitly — FR-14 gives on-screen summary only; nothing adds analytics |
| 47 | Out of v1: team/group scoring modes | Exclusion | PRD + ADD | §5 Non-Goals; addendum backlog (team scoring parked for commercial) |
| 48 | Vision: become the standard quiz platform for Hebrew-speaking institutions ("the Kahoot of Hebrew") | Vision | PRD (partial) | §1 "scales toward institutions"; the 2–3-year category-default ambition is not restated, acceptable since the PRD explicitly builds on (not duplicates) the brief |
| 49 | Vision: richer question types | Vision | **GAP** (minor) | Not in PRD non-goals, not in addendum backlog. Note G-4 |
| 50 | Vision: integrations with school management systems | Vision | **GAP** (minor) | Not parked anywhere. Note G-4 |
| 51 | Vision: white-label options for event companies | Vision | **GAP** (minor) | Not parked anywhere (addendum GTM ladder covers institutions and sponsorships, not white-label). Note G-4 |
| 52 | Vision: Arabic-language support / MENA expansion | Vision | ADD | Addendum backlog: "Arabic-language expansion (long-term)" |
| 53 | Vision: WhatsApp-identity + local-language-AI model repeatable across underserved language markets | Vision | Not carried (acceptable) | Strategic thesis beyond PRD horizon; brief remains the authoritative home for it |

## Notes

### Genuine gaps (silently dropped)

**G-1 — Organizer self-sufficiency success signal dropped (most important).**
The brief's organizer success criterion "The host can create and run a full quiz without external support" has no counterpart in PRD §7. SM-1 tests technical reliability, SM-2 tests satisfaction/intent — neither tests whether an organizer operated the builder, lobby, and live state machine *unaided*. This matters doubly because the pilot's manual provisioning (§6.2, Avraham sets up Games) structurally hides self-serve usability problems: the pilot could pass SM-1–SM-5 with Avraham quietly doing setup for every organizer, and the commercial phase (which is entirely premised on self-serve) would inherit an untested builder UX. Recommended fix: add an SM such as "at least N pilot Organizers build and run their Game end-to-end with no intervention beyond initial provisioning," or explicitly log the decision to defer this signal.

**G-2 — Brief's business success signals not parked in the commercial backlog.**
The addendum's "Deferred commercial-phase backlog" carefully parks commercial *features* (pricing tiers, export, marketing site, GTM ladder) but none of the brief's business *success criteria*: paying customers within the first quarter of launch, NPS strong enough to drive word-of-mouth, and repeat purchase rate. SM-2 covers the word-of-mouth idea as a qualitative proxy, but the three measurable commercial signals disappear rather than being deferred-with-a-pointer. Cheap fix: one line in the addendum backlog ("commercial success metrics from brief §Success Criteria: Q1 paying customers, NPS, repeat purchase — define in commercial PRD update").

**G-3 — Kahoot-Hebrew-barrier validation assumption dropped.**
Brief: `[ASSUMPTION: Hebrew experience in Kahoot is poor enough that it is a real barrier — to be validated with target users]`. The PRD swaps the competitive rationale to filter-blocking/kosher-smartphone (which SM-5 does validate) but the original assumption is neither carried into the §9 Assumptions Index nor closed with a logged decision. If the commercial phase targets institutions whose participants *do* have browsers and no filters, the "Kahoot Hebrew is bad enough" claim becomes load-bearing again — and it will still be unvalidated. Low urgency for the pilot, but it should be parked, not lost.

**G-4 — Three vision-expansion items missing from the parked backlog.**
The addendum backlog captures most of the brief's expansion vision (Arabic, seasonal packages, team scoring, presentation mode) but silently omits: richer question types, school-management-system integrations, and white-label for event companies. Zero pilot impact; worth three bullet lines in the addendum so the commercial PRD update starts from a complete parked list.

### Minor observations (not gaps)

- **M-1** — PRD narrows MCQ to 2–4 options; the brief set no count. Deliberate-looking design constraint (matches WhatsApp lettered replies א–ד), but it is not flagged as a delta from the brief.
- **M-2** — "Hebrew language AI validation" (brief differentiator) is realized functionally (FR-16, Hebrew examples in UJ-3) but there is no explicit requirement that the Fuzzy/AI stages handle Hebrew-specific variance (ktiv male/haser, final-letter forms, transliteration). Consider a one-line consequence under FR-16 or an architecture note.
- **M-3** — "Educational assessments" as a free-text use case travels implicitly with the institutional deferral; the §5 Non-Goal "not an async learning/LMS tool" is compatible (live in-class assessment stays possible) but nobody states this.
- **M-4** — Brief's "zero participants fail to register" was relaxed to SM-3's "≥90% succeed within 2 minutes." Not silent (SM-3 cites the brief's goal), but the relaxation itself is unexplained; a five-word rationale ("zero is unmeasurable/unrealistic at pilot") would close the audit trail.

### Deliberate decisions verified as properly logged (not gaps)

- WhatsApp full Play Channel supersedes the brief's exclusion — logged in PRD §0/§4.2 and addendum "Overrides."
- Self-serve purchase, payments, result export deferred — logged in PRD §6.2/FR-14 and addendum "Overrides" + backlog.
- 500-participant scale kept as commercial design target; pilot at 30–80 — logged in addendum "Overrides" and reflected in throughput analysis.
- Institutional buyers deferred to commercial phase — PRD §2.2 Non-Users, consistent with the family-pilot calibration.

### Coverage summary

Of 53 extracted brief items: 36 covered in PRD, 3 covered via addendum (several PRD items also reinforced there), 8 deliberately deferred/overridden per logged decisions, 6 gaps (1 moderate-high: G-1; 2 moderate: G-2, G-3; 3 minor vision items: G-4).
