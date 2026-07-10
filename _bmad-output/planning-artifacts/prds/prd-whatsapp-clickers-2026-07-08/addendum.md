# PRD Addendum — WhatsApp Clickers

Depth that belongs downstream (architecture, UX, commercial planning) but was raised during PRD discovery. Not requirements — context and options.

## WhatsApp integration considerations (for architecture — OQ-1)

- **Conversation window economics.** Under the official WhatsApp Business Cloud API, a user-initiated inbound message (JOIN) opens a 24-hour service window during which free-form business replies are permitted without pre-approved templates. Since every Participant initiates contact and Games last hours, the entire game flow likely fits inside service windows — a significant cost and flexibility advantage. Must be verified against current Meta pricing (per-conversation vs. per-message models have changed over time).
- **Throughput.** Question open triggers a burst: one message per Participant, near-simultaneously — and since the pivot, that means *every* Participant, every Question. At 80 participants this is likely trivial; at the 500-participant commercial target, provider rate limits (messages/second per number) become the binding constraint on the FR-4 delivery target and therefore on game pacing itself. Architecture should measure the real limit and consider it when sizing the commercial phase.
- **Message volume envelope.** P participants × Q questions × ~3 messages (question, ack, result) + registration + final results. A 60-person, 10-question game ≈ 2,000 outbound messages.
- **Number acquisition and identity.** A dedicated WhatsApp Business number is needed; its continuity matters (the number becomes the brand's front door in a market where trust is transferred person-to-person).
- **Unofficial gateways** (e.g., libraries that automate WhatsApp Web) violate WhatsApp ToS and risk number bans mid-event — a catastrophic live-event failure mode. Flagged as an explicit anti-recommendation for the pilot despite lower cost.

## Speed Bonus channel fairness (OQ-2 — RESOLVED)

Resolved 2026-07-08 by the WhatsApp-only pivot: with a single play channel there is no cross-channel skew. The three options considered while two channels existed (accept the skew / per-channel clocks / separate bonus pools) are preserved in git history and `.decision-log.md` only for the record. Residual per-participant delivery variance within WhatsApp (network conditions, device speed) is accepted for the pilot; answers are ordered by server receipt timestamp.

## Kosher-smartphone rationale (now core design, not hypothesis)

The WhatsApp-only decision (2026-07-08) makes the kosher-smartphone scenario the default posture rather than an edge case: a device with WhatsApp but no browser plays identically to every other device, because no device uses a browser. This is the moat — no web-based competitor can serve these participants at all — and it removed an entire product surface (the participant web interface) along with its cost. What the pilot still validates (SM-5): that message-based play works comfortably across all generations at a live event.

## Deferred commercial-phase backlog (from brief + brainstorming 2026-06-24)

Not pilot requirements; parked for the commercial PRD update:

- Self-serve purchase, flat-fee pricing; two-tier structure idea: ₪99 ready-made package / ₪180–250 build-your-own. Volume assumption behind the pricing: ~200 sales × 4 holiday peaks ≈ ₪80–100k/year from packages alone (brainstorming idea #20).
- Commercial-phase success metrics carried from the brief (not pilot metrics): paying customers within the first quarter of launch, organizer NPS strong enough to drive word-of-mouth, repeat-purchase rate.
- Referral program (brainstorming idea #7): per-organizer referral code, 3 referrals = free event — the designed mechanism for the family viral loop.
- Positioning tagline (brainstorming idea #18): "תביאו את המשפחה — אנחנו מביאים את המשחק".
- Result export; "memory PDF" upsell (₪20) — designed results keepsake.
- Marketing site (landing + pricing + post-purchase redirect), filter-safe.
- Filter certification (Netfree, Etrog/Rimon) — business process, prerequisite for Haredi GTM.
- GTM ladder: families → community WhatsApp groups → ambassador figure → institutions (₪1,000–2,000) + local-business sponsorships (₪300).
- Family-league tournament format (10 families × ₪50).
- Viral winner card shared to WhatsApp after each game.
- Per-question category label in the builder (e.g., הלכה / כללי / ילדים) — deferred; the pilot achieves multigenerational design through a content requirement on the pilot Question Package (PRD FR-12 Notes) instead of new builder machinery.
- Seasonal packages per holiday calendar (Chanukah, Purim, Pesach…), presentation/projector mode, sound effects (host-controlled), team scoring, Arabic-language expansion (long-term).
- Longer-horizon vision items from the brief: richer question types, school-management-system integrations, white-label for event companies.

## Market assumption carried from the brief (validate before institutional GTM)

The brief flagged: "Hebrew experience in Kahoot is poor enough that it is a real barrier — to be validated with target users." The pilot's Haredi wedge does not test this (Kahoot is unreachable behind filters regardless); the assumption becomes load-bearing again when the commercial phase targets unfiltered institutional audiences (schools, HR) where Kahoot is an actual alternative.

## Overrides of prior documents (audit pointers)

Recorded in `.decision-log.md`; summarized for downstream readers:

- **Participant web interface — removed entirely** (2026-07-08 pivot): participants play exclusively via WhatsApp. Supersedes the brief's web-play model, the brief's "Explicitly out of v1: WhatsApp-delivered question display", EXPERIENCE.md's "Rejected — WhatsApp for question delivery", and EXPERIENCE.md's participant mobile screens (join, waiting room, split-hero question, reveal, personal leaderboard, winner screen). DESIGN.md's visual identity (palette, typography, timer/leaderboard/winner-card language) remains binding, now applied to the Audience Display at projection scale; EXPERIENCE.md remains authoritative for the Host Dashboard.
- **EXPERIENCE.md "presentation mode is v2" assumption — inverted** (2026-07-08): the Audience Display is pilot-essential; without personal screens it is the room's only shared view.
- Brief's v1 in-scope "Result export" and "Self-serve purchase" — **deferred** out of the pilot (2026-07-08).
- Brief's 500-participant scale — remains the commercial design target; pilot operates at 30–80.
- Interim "both channels" decision (earlier on 2026-07-08) — **superseded the same day** by the WhatsApp-only pivot; recorded here because reconcile-*.md reports and review-rubric.md were written against the two-channel draft and partially describe superseded behavior.
