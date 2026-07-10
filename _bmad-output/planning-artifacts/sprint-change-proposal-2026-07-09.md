---
date: 2026-07-09
project: whatsapp-clickers
workflow: correct-course
trigger: implementation-readiness-report-2026-07-09.md
mode: incremental
scope: minor
status: approved-and-applied
---

# Sprint Change Proposal — Epics ↔ Final UX Revision Re-sync

## 1. Issue Summary

**Problem statement:** `epics.md` (last saved 2026-07-09 22:09) predates the final OQ-7 UX revision (`DESIGN.md` + `EXPERIENCE.md`, saved 2026-07-09 22:17). The epics absorbed part of the revision (UX-DR13 keyboard model, UX-DR15 tone are marked resolved) but not all of it, leaving 3 major and 5 minor alignment defects — all confined to `epics.md`.

**Discovery:** The Implementation Readiness assessment (2026-07-09) rated the plan **NEEDS WORK — narrowly**, with FR coverage at 18/18 (100%) and all structural best-practice checks passing. The sole blocker to READY is this epics-vs-UX drift.

**Evidence (verified against the loaded UX sources):**
- Story 3.1's AC says Escape "opens a confirm-stop dialog" — EXPERIENCE.md (Interaction Primitives, confirmed at the 2026-07-09 accessibility review) says Escape has exactly one meaning: *close* the open dialog, never open one. The same AC lists the CTA "סיים משחק", renamed "עצור" by the revision.
- Three rows of the canonical WhatsApp message-templates table have no owning story: the `שם:` rename flow (A3/OQ-3 resolution), the pre-lobby JOIN reply (A17), and the winner's personal final message (A16, UJ-2's climax). Architecture specifies `messages_he.go` implements the table *exactly* — unowned rows mean orphan copy or silent scope loss.
- Epics UX-DR1 quotes `success #16A34A`; DESIGN.md re-pointed success to `#15803D` (A18) after the old value failed WCAG contrast at the Reveal (white text at 3.3:1).

## 2. Impact Analysis

**Epic impact:** None structural. All four epics remain as planned — no epic added, removed, resequenced, or reprioritized. Every fix is an AC-level or documentation-hygiene edit inside existing stories (2.4, 3.1, 3.9, 1.5, 4.1) plus tag cleanup across the document.

**Story impact:**
- Story 3.1 — keyboard AC rewritten; CTA list corrected.
- Story 2.4 — two ACs added (pre-lobby JOIN, `שם:` rename); OQ-3 assumption resolved.
- Story 3.9 — one AC added (winner's personal message); tie assumption resolved by A16.
- Story 1.5 — one AC added (bank card preview, "מהמאגר" badge, empty states).
- Story 4.1 — one AC added ("הפחת אנימציות" room-level toggle).
- Stories 2.2, 3.2, 3.8, 4.2–4.6 + Overview/FR-10/UX-DR5–8/UX-DR16/Epic 3–4 notes — stale `[OQ-7]`/`[ASSUMPTION]` tags re-pointed to canonical sources.

**Artifact conflicts:**
- **PRD:** No conflict; MVP untouched. The added behaviors realize existing PRD content (OQ-3 resolution, UJ-1 edge case, UJ-2 climax).
- **Architecture:** No structural change. Two note-level details land as story ACs rather than architecture edits: the `שם:` parse branch joins `wa/inbound.go`'s taxonomy (Story 2.4), and the reduce-motion setting rides the game snapshot as a display-settings field (Story 4.1).
- **UX:** The canonical source; unchanged.

**Technical impact:** None yet — no code exists. Preventive value: the fixes stop three classes of implementation defects (rejected keyboard model, orphaned message templates, WCAG-failing color token) before story creation.

## 3. Recommended Approach

**Selected path: Direct Adjustment (Option 1)** — modify existing stories within the current plan.

- **Rollback (Option 2): N/A** — no implementation exists to roll back.
- **MVP review (Option 3): Not needed** — scope and goals are unaffected.

**Rationale:** All defects are epic-document drift against an already-validated UX source of truth. Effort: **Low** (single focused edit pass on one file). Risk: **Low** (every new AC quotes its canonical UX row/spec verbatim). Timeline impact: none — this unblocks sprint planning rather than delaying it.

## 4. Detailed Change Proposals

All changes target `_bmad-output/planning-artifacts/epics.md`. All seven were reviewed and approved individually (incremental mode, 2026-07-09).

### CP-1 (Major) — Story 3.1: keyboard AC and CTA label

**Section: Acceptance Criteria — CTA list**

OLD:
> **Then** the control panel shows exactly one primary CTA per UX-DR10 ("פתח שאלה [N]" / "סגור שאלה" / "גלה תשובה" / "שאלה הבאה" / "סיים משחק"), so no state can be skipped accidentally,

NEW:
> **Then** the control panel shows exactly one primary CTA per UX-DR10 ("התחל משחק" / "פתח שאלה [N]" / "סגור שאלה" / "גלה תשובה" / "שאלה הבאה"), with "עצור" as the secondary stop control (renamed from "סיים משחק" per the UX revision 2026-07-09) always behind the confirm-stop dialog, so no state can be skipped accidentally,

**Section: Acceptance Criteria — keyboard**

OLD:
> **Given** the keyboard,
> **Then** Space triggers the primary CTA and Escape opens a confirm-stop dialog (UX-DR13), with all copy from `strings.he.ts` (UX-DR11).

NEW:
> **Given** the keyboard,
> **Then** Space fires the primary CTA only when focus is on `body` or the main region — never inside inputs or dialogs — focused buttons use native activation (no global handler racing them), and Escape has exactly one meaning: close the open dialog, never open one (UX-DR13); stopping the game is the visible "עצור" control, whose confirm dialog guards destructive advances on every path, keyboard included, with all copy from `strings.he.ts` (UX-DR11).

**Rationale:** The old AC implements the exact keyboard model the 2026-07-09 accessibility review rejected, and cites UX-DR13 while contradicting it.

### CP-2 (Major) — Story 2.4: pre-lobby JOIN reply + `שם:` rename flow

**Section: Acceptance Criteria — registration (modified)**

OLD:
> **Then** I am registered as a Participant (identified by phone number, display name from my WhatsApp profile name `[ASSUMPTION — OQ-3]`), and receive a Hebrew welcome reply with my name,

NEW:
> **Then** I am registered as a Participant (identified by phone number, display name from my WhatsApp profile name — per EXPERIENCE.md A3, resolving OQ-3: profile name by default, correctable via `שם:`), and receive the Hebrew welcome reply with my name including the name-correction hint, per the canonical templates table,

**Section: Acceptance Criteria — new AC (pre-lobby JOIN, A17)**

> **Given** a Game whose lobby has not yet opened (`draft`),
> **When** I send `JOIN <code>` with its valid code,
> **Then** I receive the pre-lobby reply per the EXPERIENCE.md templates table ("הקוד נכון! ההרשמה עוד לא נפתחה — שלחו שוב את ההודעה כשהמארגן מכריז שמתחילים.") and am not yet registered (A17; UJ-1's "הדודה ששלחה JOIN יום קודם" edge case).

**Section: Acceptance Criteria — new AC (`שם:` rename, A3)**

> **Given** I am a registered Participant (any role, any game state),
> **When** I send `שם: <השם>`,
> **Then** my display name is updated and I receive the confirmation "עודכן ✓ מעכשיו: [שם]" (templates table; the `שם:` parse branch joins `wa/inbound.go`'s parse taxonomy),
> **And** an unregistered sender's `שם:` message receives the Help reply (conversation grammar).

**Rationale:** Both behaviors are canonical rows in the templates table and rows in the conversation-grammar matrix; the welcome message itself advertises the rename. With no owning story, `messages_he.go` gets orphan rows or the behaviors are silently dropped.

### CP-3 (Major) — Story 3.9: winner's personal final message

**Section: Acceptance Criteria — final results (modified)**

OLD:
> **Then** every Participant **and** every Spectator receives their score, rank, and the winner's name `[OQ-7: final copy]`.

NEW:
> **Then** every Participant **and** every Spectator receives their score, rank, and the winner's name — copy per the EXPERIENCE.md templates table ("המשחק נגמר! 🏆 הזוכה: [שם] עם [ניקוד] נקודות...").

**Section: Acceptance Criteria — new AC (winner variant, A16)**

> **Given** the winner(s),
> **Then** they additionally receive the personal winner variant "מזל טוב, [שם]! 🏆 ניצחת עם [ניקוד] נקודות!" — ties addressed jointly ("מזל טוב, [שם] ו-[שם]! 🏆 ניצחתם עם [ניקוד] נקודות!") per A16 (UJ-2's emotional climax).

**Section: Acceptance Criteria — tie assumption (modified)**

OLD:
> **Then** all tied names are included as winners `[ASSUMPTION — PRD defines shared ranks but not winner-tie presentation]`.

NEW:
> **Then** all tied names are included as winners — "הזוכים: [שם] ו-[שם] עם [ניקוד] נקודות" (resolved by EXPERIENCE.md A16).

**Rationale:** The winner's personal message is a distinct canonical template row and the private half of UJ-2's climax; the tie-presentation assumption was resolved by A16.

### CP-4 (Major) — UX-DR1: stale success color token

**Section: UX Design Requirements — UX-DR1**

OLD:
> separate semantic colors (`success #16A34A`, `error #DC2626`, `warning #F59E0B`);

NEW:
> separate semantic colors (`success #15803D` — re-pointed from #16A34A by the 2026-07-09 accessibility review (A18): the old value collided with green-600 and put white text at 3.3:1 at the Reveal; `error #DC2626`, `warning #F59E0B`) — DESIGN.md frontmatter is authoritative for all token values;

**Rationale:** Story 1.1 configures tokens per UX-DR1; the stale value would reintroduce the exact WCAG failure the UX review fixed.

### CP-5 (Minor) — Stale `[OQ-7]`/`[ASSUMPTION]` tag sweep

Re-point every resolved tag to its canonical source so story authors do not treat resolved content as blocked (the PRD declared OQ-7 *blocking story creation* for §4.2/§4.3):

1. **Overview "OQ-7 dependency" paragraph** → rewritten: the UX revision landed 2026-07-09 and was validated at the reviewer gate; canonical sources are EXPERIENCE.md's WhatsApp message-templates table (all outbound copy), the Audience Display stage table (per-state content), and DESIGN.md A19 (projection sizing). Blast-radius note (messages_he.go / strings.he.ts / one component per stage) retained.
2. **Requirements Inventory FR-10** → `[OQ-7: final per-state content and layout]` becomes "(per-state content and layout per EXPERIENCE.md's stage table + A19 projection ramp)".
3. **UX Design Requirements preamble** → "Items marked [OQ-7] await the UX revision" replaced with a note that the revision is complete and tags below now cite their resolved sources.
4. **UX-DR5** → `[OQ-7: exact projection sizing]` becomes "(projection sizing per A19: timer numeral 96px at 1080p; ring thickens 8px→14px at ≤5s)".
5. **UX-DR6** → `[OQ-7]` becomes "(projection adaptation per the stage table + A19)".
6. **UX-DR7** → `[OQ-7: projection layout]` becomes "(rows 48px at 1080p per A19; top-10 depth per A11)".
7. **UX-DR8** → `[OQ-7: projection layout + final copy]` becomes "(layout per DESIGN.md winner-card spec; copy per the templates table; ties per A16)".
8. **UX-DR16** → `[OQ-7: final content and layout per state]` becomes "(content per EXPERIENCE.md's Audience Display stage table)".
9. **Epic 3 implementation notes** → "Message copy stories are `[OQ-7]`-tagged and isolated to `messages_he.go`" becomes "Message copy is canonical per EXPERIENCE.md's templates table, isolated to `messages_he.go`".
10. **Epic 4 implementation notes** → "(isolating `[OQ-7]` layout decisions)" becomes "(one component per display state, per the stage table)".
11. **Story 2.2** → `[OQ-7: final copy]` becomes "per the templates table Help row"; the warm-playful sentence already present stands.
12. **Story 3.2** → `[OQ-7: final copy]` becomes "copy per the templates table Question rows (MCQ / Free-Text)".
13. **Story 3.8** → `[OQ-7: final copy]` and the outdated example replaced with the canonical rows: "נכון! 🎉 +[ניקוד] נקודות / ⚡ בונוס מהירות +[בונוס] / מקום [דירוג] בטבלה" (two emoji allowed on the bonus message, A20) and the wrong-answer variants incl. the last-question variant without "עוד הכול פתוח" (A5).
14. **Story 4.2** → `[OQ-7: final layout]` becomes "per the stage-lobby spec"; `[ASSUMPTION — projection sizing pending OQ-7]` becomes "per A19: code + phone at Display 900 120px+, instruction at Heading 800 64px".
15. **Story 4.3** → `[OQ-7: layout]` becomes "per the split-hero spec at stage scale (A19)"; the ≤5s AC gains "and thickens 8px→14px — never a hue-only cue" per EXPERIENCE.md's timer rule.
16. **Story 4.4** → `[OQ-7: layout]` becomes "per the distribution-bar spec (labels outside the fills, ✓ on the correct bar's label)"; `[OQ-7: distribution presentation for free-text]` becomes "per A15: stage-answer-card with the Accepted Answer's primary form + counts line 'X ענו · Y צדקו' — no distribution bars".
17. **Story 4.5** → `[OQ-7: layout]` becomes "per A11: ▲ + places climbed beside the rank, indicator ≥40px; static reordering under reduced motion".
18. **Story 4.6** → `[OQ-7: final copy]` becomes "per the winner-card spec and templates table; ties stack up to three names, score shown once (A16)".

### CP-6 (Minor) — Story 1.5: Question Bank UX details

**Section: Acceptance Criteria — new AC**

> **Given** the bank browse view,
> **Then** each Package card shows title, question count, and a question preview (A13), imported copies carry a "מהמאגר" badge, a Game with no Questions prompts "הוסיפו שאלה ראשונה — או ייבאו חבילה מהמאגר" with both CTAs, and an empty Question Bank states that packages are on the way rather than showing a bare list (EXPERIENCE.md empty states).

### CP-7 (Minor) — Story 4.1: room-level reduce-motion toggle + Epic 2 note

**Section: Story 4.1 Acceptance Criteria — new AC**

> **Given** the dashboard's "הפחת אנימציות" toggle (EXPERIENCE.md Display controls),
> **When** the Organizer enables it,
> **Then** the Audience Display renders the static motion equivalents for the whole room — the setting rides the game snapshot (display-settings field), requiring no display interaction, because the audience cannot set `prefers-reduced-motion` on a projector.

**Section: Epic 2 implementation notes — appended**

> Note: FR-1's Audience-Display lobby-counter consequence completes at Story 4.2 — Epic 2 delivers the dashboard half (relevant when demoing Epic 2 as "done").

## 5. Implementation Handoff

**Scope classification: Minor** — direct implementation by the Developer agent; no backlog reorganization, no replan.

**Handoff plan:**
- **Developer agent (this session):** apply CP-1 through CP-7 to `epics.md` as a single edit pass.
- **sprint-status.yaml:** N/A — sprint planning has not run yet; no epics/stories added or removed by this proposal, so nothing to reconcile later either.
- **PM / Architect:** no action — PRD, UX, and architecture documents are untouched.

**Success criteria:**
1. No `[OQ-7]` tag remains in `epics.md` (the underlying open question is resolved; every former tag cites its canonical source).
2. Every row of EXPERIENCE.md's message-templates table has an owning story AC.
3. `epics.md` quotes `success #15803D` (or defers to DESIGN.md) — the failing value appears nowhere.
4. Story 3.1's keyboard AC matches EXPERIENCE.md's Interaction Primitives verbatim in meaning.
5. A re-run of the readiness check returns READY on the UX-alignment dimension.

**Next steps after application:** `bmad-sprint-planning` → `bmad-create-story` (nothing blocks story creation once this lands). In parallel, external lead items continue: Meta WhatsApp Business verification + number acquisition; owner decisions OQ-4 (pilot Question Package sourcing) and OQ-6 (budget ceiling).
