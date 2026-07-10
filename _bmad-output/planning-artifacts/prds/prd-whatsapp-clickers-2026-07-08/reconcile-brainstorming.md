# Reconciliation: Brainstorming Session 2026-06-24 → PRD 2026-07-08

**Source input:** `_bmad-output/brainstorming/brainstorming-session-2026-06-24-1000.md` (Hebrew; 20 ideas + 2 constraint insights on Haredi-market entry)
**Targets:** `prd.md` (pilot-scope PRD) and `addendum.md` (same folder)
**Reviewed:** 2026-07-08

**Ground rules applied:** The PRD deliberately covers only the family-events pilot; monetization/GTM/marketing/certification items are expected in the addendum's "Deferred commercial-phase backlog" — deferral itself is not a gap, but *incomplete parking* is. Business/marketing action items are flagged only if they imply a pilot-relevant product capability the PRD lacks.

**Classification legend:** COVERED (in PRD, directly or via referenced final UX docs) · PARKED (in addendum deferred backlog / open question) · GAP (dropped entirely, or the PRD misses a pilot-relevant capability the idea implies).

---

## 1. Constraint insights

| # | Source insight | Classification | Evidence |
|---|---|---|---|
| C-1 | Filter approval (Netfree, Etrog/Rimon) is a short bureaucratic process, not a technical barrier; site is opened "with image filtering only" | COVERED + PARKED | PRD §2.1 gatekeeper JTBD (demonstrably filter-safe); §6.2 explicitly splits product constraint (in scope) from certification process (business task); addendum backlog "Filter certification (Netfree, Etrog/Rimon)". The image-filtering nuance is materially handled by DESIGN.md (final, referenced by PRD §6.1): abstract/geometric visuals only, no people imagery, no external font/CDN dependencies, CSS-only celebration effects — the product functions with images stripped. |
| C-2 | Real barrier is product discovery — no Google/Facebook ads; only WhatsApp groups, Haredi press, word of mouth | PARKED | Addendum "GTM ladder: families → community WhatsApp groups → ambassador figure → institutions". Pure GTM; no pilot product capability implied (pilot organizers are hand-provisioned per PRD §6.2). |

## 2. Idea-by-idea reconciliation

| # | Idea (condensed) | Classification | Evidence / notes |
|---|---|---|---|
| 1 | GTM families-first, bottom-up (Slack-style); institutions come later | COVERED + PARKED | PRD §1 wedge = large family event; §2.2 institutions are non-users for pilot; addendum GTM ladder ends at institutions. |
| 2 | Seeding via community WhatsApp groups | PARKED | Addendum GTM ladder ("community WhatsApp groups"). Marketing; no pilot capability implied. |
| 3 | Chanukah as launch season | COVERED + PARKED | PRD §0 "Chanukah 2026 commercial launch as the horizon"; pilot requires a holiday-themed pack (§6.1); addendum "Seasonal packages per holiday calendar". |
| 4 | Community ambassador (rabbi with 10k WhatsApp reach; free access for endorsement) | PARKED | Addendum GTM ladder ("ambassador figure"). |
| 5 | Large family events (50–80 people, multi-generation, private organizer) as primary audience | COVERED | PRD §1 wedge, §2.1 organizer JTBD, SM-1. Note: PRD widens the range to 30–80 — a deliberate calibration, not a drop. |
| 6 | Built-in viral loop through family-branch structure | COVERED | PRD §1 ("built-in viral loop"); SM-2 is the stated proxy for it. |
| 7 | "חבר מביא חבר" referral program — referral code per organizer, 3 referrals = free event | **GAP (parking incomplete)** | Appears nowhere in PRD or addendum. Every sibling commercial idea (#14–#17, #8, #12) was parked in the deferred backlog; this one was silently dropped. See §4-G2. |
| 8 | Viral winner card ("משפחת כהן ניצחה עם 87 נקודות!") shared to WhatsApp | PARKED | Addendum backlog "Viral winner card shared to WhatsApp after each game". (Pilot has winner naming on all channels — FR-18 — but the shareable card artifact is deliberately commercial-phase.) |
| 9 | Multigenerational question variety — halacha for Saba, general for parents, quick questions for kids; "everyone feels they have a part" | **GAP (mechanism dropped)** | The PRD names the *outcome* (§2.1 "answer questions at their own ability level"; "Saba wins" as designed climax) but no FR, Glossary term, or content requirement realizes the *mechanism*: no question category/tag concept in FR-11/FR-12, and no requirement that the pilot Question Package span generational categories/difficulty. See §4-G1. |
| 10 | Two tracks: ready-made question bank + personal family questions | COVERED | Glossary "Question Bank"; FR-11 (custom question authoring incl. Free-Text with Accepted Answers); FR-12 (import a package and mix with custom questions, imported copies editable). The two-tier *pricing* attached to these tracks is parked (addendum backlog). |
| 11 | Partnership with Haredi content creators (content + endorsement) | COVERED (as open question) + PARKED | PRD OQ-4 explicitly cites "a content partner per brainstorming idea #11" as a sourcing option; endorsement/marketing side sits under the addendum GTM ladder. |
| 12 | Seasonal packages per holiday (Chanukah/Purim/Pesach = marketing sprints) | PARKED (+ pilot seed) | Addendum backlog "Seasonal packages per holiday calendar"; pilot ships one holiday pack (§6.1). |
| 13 | "Saba wins" — deliberately designed halacha/Tanach category the older generation dominates, producing the emotional peak | PARTIAL — emotional goal COVERED, design mechanism **GAP** | PRD §2.1 and UJ-2 enshrine the moment ("a designed emotional climax, not an accident"); addendum OQ-2 discussion even worries about undermining it via WhatsApp-channel latency. But as with #9, nothing requires the Torah-knowledge category to exist in the game content or builder. See §4-G1. |
| 14 | Two-tier pricing: ₪99 ready-made / ₪180–250 build-your-own | PARKED | Addendum backlog, verbatim. |
| 15 | Memory-PDF upsell (₪20) | PARKED | Addendum backlog ("memory PDF upsell (₪20) — designed results keepsake"), alongside deferred result export (FR-14 Out of Scope). |
| 16 | Family-league neighborhood tournament (10 families × ₪50) | PARKED | Addendum backlog ("Family-league tournament format (10 families × ₪50)"). |
| 17 | Sponsorships from Haredi businesses — logo on opening screen, ₪300 | PARKED | Addendum backlog ("local-business sponsorships (₪300)"). The implied sponsor-branding screen capability is correctly commercial-phase. |
| 18 | Tagline: "תביאו את המשפחה — אנחנו מביאים את המשחק" | **GAP (minor, dropped)** | Not in PRD or addendum. Brand asset, not a software requirement — but the addendum parks the marketing site without carrying the agreed positioning line. See §4-G3. |
| 19 | Three-wave roadmap (1: filters + Chanukah pack + PDF; 2: family league as launch campaign; 3: institutions + sponsorships) | PARKED (substance) / minor drop (sequencing) | Every component is individually parked (certification, seasonal pack, PDF, league, institutions ₪1,000–2,000, sponsorships ₪300 — all in addendum backlog; GTM ladder gives the ordering). The specific *timing* ("league one month before Chanukah") is not carried; absorbed into commercial planning. |
| 20 | Revenue forecast: 200 sales × 4 holidays ≈ ₪80–100k/year from packages | **GAP (minor, dropped)** | No revenue/volume baseline anywhere in PRD or addendum. Business projection, not a requirement — but the backlog carries the *prices* (₪99, ₪20, ₪50, ₪300, ₪1,000–2,000) without the volume assumption that made them a business case. See §4-G4. |

## 3. Immediate action items (source checklist)

| Action item | Classification | Evidence |
|---|---|---|
| Contact Netfree & Etrog for site approval | PARKED | PRD §6.2 (parallel business task); addendum backlog. |
| Build ₪99 Chanukah package | Split: content COVERED, price PARKED | §6.1 pilot pack requirement + OQ-4; pricing in addendum backlog. |
| Add ₪20 memory-PDF upsell | PARKED | Addendum backlog. |
| Find Haredi community ambassador before Chanukah | PARKED | Addendum GTM ladder. |
| Prepare "Chanukah league" neighborhood campaign as pilot | SUPERSEDED (deliberate) | The PRD re-scopes the pilot from a neighborhood league campaign to 3–5 private family events (§6, SM-1); the league format itself is parked. Documented re-scope, not a gap. |

## 4. Genuine gaps (detail)

### G1 — Multigenerational question design has no product hook (ideas #9 + #13) — HIGH, pilot-relevant
The single most product-shaping insight of the session — the quiz *deliberately* mixes categories so each generation has questions it can win, and a Torah-knowledge category guarantees the "Saba wins" peak — survives in the PRD only as narrative (§2.1 JTBD, UJ-2 "designed emotional climax, not an accident"). Nothing makes it non-accidental:
- No category/difficulty concept on Questions (Glossary §3, FR-11 builder, FR-12 bank).
- No content requirement that the mandatory pilot Question Package (§6.1, FR-12 note) be multigenerational or include a halacha/Tanach category. OQ-4 asks *who* writes the pack, not *what it must contain*.
- No success-metric hook (SM-1/SM-2 would pass even with a kids-only quiz).
**Smallest fix:** add a content acceptance requirement to §6.1 / the FR-12 PM note — "the pilot Question Package must span generational categories, including at least one Torah-knowledge category targeted at the oldest generation" — and optionally a lightweight category label per Question so the builder/bank can express the mix. No new engineering surface strictly required for the pilot beyond that label (or even a purely editorial rule for the pack).

### G2 — Referral program dropped entirely (idea #7) — MEDIUM, parking incompleteness
"3 referrals = a free event" with per-organizer referral codes is the one commercial/growth idea absent from the addendum's deferred backlog while all its siblings (#8, #12, #14–#17, institutions, sponsorships) were parked. Deferring it is correct; losing it is not — it is also the mechanism the session paired with the viral loop the PRD calls its wedge. **Fix:** add one bullet to the addendum backlog ("Referral program: per-organizer referral code, 3 referrals = free event").

### G3 — Tagline dropped (idea #18) — LOW, marketing
"תביאו את המשפחה — אנחנו מביאים את המשחק" was a named session breakthrough (#5 in the source summary) and is nowhere in either document, even though the marketing site it belongs to is parked and the PRD's own title is marked "Working title — confirm." No product capability implied. **Fix:** carry it into the addendum's marketing-site backlog bullet as the agreed positioning line.

### G4 — Revenue baseline dropped (idea #20) — LOW, business planning
The addendum backlog preserves every price point but not the volume/revenue assumption (200 sales × 4 holidays ≈ ₪80–100k/yr) that justified them. Not a requirement; worth one line in the addendum so the commercial-phase PRD update starts from the agreed baseline.

## 5. Deliberate divergences verified as NOT gaps

- **Pilot re-scope:** neighborhood Chanukah-league pilot → 3–5 private family events (PRD §6, SM-1); the league is parked.
- **Audience range:** 50–80 (source) widened to 30–80 (PRD §1, SM-1).
- **WhatsApp full Play Channel:** an addition beyond the session (and an override of brief/UX), fully documented (PRD §0, §4.2; addendum overrides section).
- **All monetization/GTM/certification deferrals:** parking verified item-by-item above; complete except G2–G4.

## 6. Verdict

Coverage is strong: 16 of 20 ideas plus both constraint insights are either realized in the pilot PRD or explicitly parked with traceable wording in the addendum. The parking discipline is nearly complete. The one pilot-relevant miss is **G1** — the PRD promises the "Saba wins" moment is designed, but dropped the design mechanism (multigenerational question categories and a content requirement on the pilot pack) that makes it so. G2 is a backlog bookkeeping fix; G3/G4 are one-line addendum additions.
