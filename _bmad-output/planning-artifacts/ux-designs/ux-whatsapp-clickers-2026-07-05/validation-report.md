# Validation Report — whatsapp-clickers

- **DESIGN.md:** `_bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/DESIGN.md`
- **EXPERIENCE.md:** `_bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md`
- **Run at:** 2026-07-09 (OQ-7 revision, Finalize reviewer gate: rubric walker + accessibility lens)

## Overall verdict

A strong, source-extractable spine pair: all five PRD journeys are re-narrated as Key Flows with climaxes, every `{token}` reference in both files resolves, the conversation grammar (state × message-class) is genuinely contract-grade, and the two-palette / gold-discipline system is coherent end to end. One high-severity gap blocks clean downstream extraction — the Audience Display has no defined content for Free-Text Questions — plus a cluster of copy-contract self-inconsistencies and cross-document ripples that are cheap to fix but would confuse an AI story-dev if left standing.

The accessibility lens materially sharpens the picture: a token collision (`success` == `green-600`, both `#16A34A`) silently voids the distribution-bar's claimed correct-answer distinction and puts the Reveal — the game's most important moment — at the stage's weakest text contrast (3.3:1 before projector washout). The projection type ramp lacks a viewing-geometry basis, the lobby undersizes the phone-number transcription task, and the host keyboard shortcuts as specified can end a game by accident. None of this blocks Finalize; the six combined highs are all resolvable in-spine with token-level and copy-level fixes.

## Category verdicts

- Flow coverage — strong
- Token completeness — adequate
- Component coverage — adequate
- State coverage — adequate
- Visual reference coverage — adequate
- Bloat & overspecification — strong
- Inheritance discipline — adequate
- Shape fit — strong

## Findings by severity

### Critical (0)

None.

### High (6)

**[State coverage]** — Audience Display has no defined content for Free-Text Questions (EXPERIENCE.md IA + Component Patterns; DESIGN.md components)
Neither the FT question stage nor the FT reveal is specified; epics Stories 4.3/4.4 explicitly wait on this.
Fix: define FT question-stage body and FT reveal content (correct-answer card + answered/correct counts) + DESIGN.md component entry.

**[Accessibility]** — Token collision: `success` == `green-600` (#16A34A) (DESIGN.md frontmatter)
The correct distribution bar cannot render as distinct; falsifies "semantic colors are separate from brand green."
Fix: success → #15803D (white = 5.0:1) + ✓ in the correct bar's label.

**[Accessibility]** — Reveal is the stage's lowest-contrast moment: white on success = 3.30:1 (DESIGN.md stage-option-correct)
Projector washout pushes the game's key information below effective legibility; hierarchy inverted vs. 7.13:1 on normal options.
Fix: same success → #15803D token change resolves it.

**[Accessibility]** — Type ramp has no viewing-geometry basis (DESIGN.md Typography, A9)
Display-only surfaces (leaderboard, distribution labels, counters) go illegible past ~6–7m on a typical 100" projection.
Fix: state the viewing assumption; raise display-only surfaces one step (48px; stage floor 40px); back-row preflight guidance.

**[Accessibility]** — Lobby undersizes the phone number; bidi isolation unmandated (DESIGN.md stage-lobby; EXPERIENCE.md Flow 1)
Transcribing 10 digits from the projector is the product's hardest visual task (kosher phones: no links); mixed RTL/LTR can visually reorder.
Fix: number at Display scale, grouped, tabular-nums; bidi-isolation rule (web + messages_he.go); pre-share-as-contact-card Organizer guidance.

**[Accessibility]** — Host keyboard shortcuts will misfire at live-event stakes (EXPERIENCE.md Interaction Primitives)
Global Space races native focused-button activation (can end the game); Escape-opens-dialog inverts its universal meaning.
Fix: Space only when focus on body/main; native behavior wins on focused buttons; dedicated "עצור" control; Escape only closes; destructive advances behind confirm.

### Medium (10)

1. **[Token completeness]** Typography keys non-spec (`family`/`weight` → `fontFamily`/`fontWeight`) (DESIGN.md frontmatter).
2. **[Token completeness]** `components:` values are prose, not token objects — machine contract degraded (DESIGN.md frontmatter).
3. **[Token completeness]** Contrast targets absent from DESIGN.md; gold/green-800 ≈ 4.3:1 and muted/green-800 ≈ 4.4:1 unstated (DESIGN.md Colors).
4. **[Component coverage]** shadcn-inheritance contract absent from DESIGN.md (Components).
5. **[State coverage]** Pre-lobby JOIN (game in draft) unhandled in the conversation grammar (EXPERIENCE.md).
6. **[State coverage]** Winner-tie presentation undefined (winner-card + messages; epics 3.9/4.6 wait on it).
7. **[Inheritance]** A1 emoji rule contradicted by the Result-correct template (🎉+⚡) (EXPERIENCE.md Voice and Tone vs. templates).
8. **[Inheritance]** Invalid-code template has unimplementable placeholder `[קוד נכון]` (EXPERIENCE.md templates).
9. **[Inheritance]** Default time limit conflicts: spine A8 = 20s vs. epics Story 1.3 = 30s → ripple epics to 20s.
10. **[Inheritance]** epics UX-DR15 still binds the superseded "direct, short / ברוך הבא" tone → one-line ripple.

Plus accessibility mediums: digits 1–4 never advertised in copy; nothing tells Participants to keep the chat open; dimmed wrong options fall to ~2.1:1 (dim the fill, not the text); timer-inline gold ring ≈1.6:1 on white and breaks the gold rule (→ green-800 ring, error at ≤5s); live regions flood screen readers (throttle counters, exclude timer numeral); 200%-zoom claim contradicts the 1024px floor (promote single-column collapse); blind participants have no time channel (builder guidance 30–45s; name the room's audible count as designed behavior).

### Low (22)

Rubric: "verbatim" flow-name claim (soften); Flow 2 failure-path pointer; ink-on-dark-muted rgba → 8-digit hex; orphan/duplicate color tokens; rounded.pill naming; host-stat-pill/host-primary-action unreferenced in EXPERIENCE.md; package-card/badge in neither contract; grammar column naming (leaderboard state); unregistered `שם:` → Help footnote; host sign-in states; winner screen in old mockup unclassified; exploration-artifact provenance + stale .working duplicate; three-table sync note; meta-annotation in Result-correct copy cell; placeholder legend incomplete; last-question Result-wrong variant unspecified; casing normalization; missing `description` frontmatter; status still draft; keyboard-shortcut NOTE tracked to closure.

Accessibility: focus orphaned on CTA swaps (persistent button); distribution-bar labels outside the fill; "הפחת אנימציות" dashboard toggle; thicken timer ring at ≤5s; leaderboard reshuffle in reduced-motion list + ▲ size floor; Welcome rename hint colon stacking.

## Reviewer files

- `review-rubric.md`
- `review-accessibility.md`
