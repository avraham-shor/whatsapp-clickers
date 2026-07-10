---
name: WhatsApp Clickers
description: "Hebrew-native, RTL, filter-safe live-quiz stage + host tool. Festival Green; gold at exactly two moments."
status: final
created: 2026-07-06
updated: 2026-07-09
sources:
  - _bmad-output/planning-artifacts/briefs/brief-whatsapp-clickers-2026-06-18/brief.md
  - _bmad-output/brainstorming/brainstorming-session-2026-06-24-1000.md
  - _bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md
colors:
  # Festival Green — Audience Display and brand
  surface-base: '#F0FDF4'   # alias of green-50: semantic name for the stage background
  surface-raised: '#FFFFFF'
  green-900: '#14532D'
  green-800: '#166534'
  green-600: '#16A34A'
  green-100: '#D1FAE5'
  green-50: '#F0FDF4'
  gold: '#FBBF24'
  text-primary: '#14532D'
  text-secondary: '#4B5563'
  ink-on-dark: '#FFFFFF'
  ink-on-dark-muted: '#FFFFFFB3'
  border-light: '#D1FAE5'
  success: '#15803D'
  error: '#DC2626'
  warning: '#F59E0B'
  # Host Dashboard — neutral slate with green accents
  host-surface: '#F8FAFC'
  host-border: '#E2E8F0'
  host-text: '#0F172A'
  host-text-secondary: '#64748B'
typography:
  display:
    fontFamily: "system-ui, -apple-system, 'Segoe UI', Arial, sans-serif"
    fontWeight: 900
    note: "Timer numeral, winner headline, lobby JOIN Code + phone number — used sparingly"
  heading:
    fontFamily: "system-ui, -apple-system, 'Segoe UI', Arial, sans-serif"
    fontWeight: 800
    note: "Question text, stage instruction lines, screen titles, nav brand"
  body:
    fontFamily: "system-ui, -apple-system, 'Segoe UI', Arial, sans-serif"
    fontWeight: 500
    note: "Option text, body copy, list rows"
  ui:
    fontFamily: "system-ui, -apple-system, 'Segoe UI', Arial, sans-serif"
    fontWeight: 600
    note: "Labels, progress indicators, status badges, live counters"
rounded:
  sm: 8px
  md: 12px
  lg: 16px
  xl: 22px
  pill: 9999px
spacing:
  '1': 4px
  '2': 8px
  '3': 12px
  '4': 16px
  '5': 24px
  '6': 32px
  '7': 48px
  '8': 64px
components:
  stage-option:
    background: '{colors.green-800}'
    text: '{colors.ink-on-dark}'
    letterWeight: 700
    textWeight: 500
    radius: '{rounded.sm}'
    minHeight: 96px
    fontSize: 40px
    states: none — output-only
  stage-option-correct:
    background: '{colors.success}'
    text: '{colors.ink-on-dark}'
    icon: "✓ inline-end"
    opacity: 1
  stage-option-dimmed:
    background: '{colors.green-100}'
    text: '{colors.text-primary}'
    letterWeight: 700
    note: "demoted, still legible — dim the fill, never the text"
  stage-answer-card:
    background: '{colors.success}'
    text: '{colors.ink-on-dark}'
    icon: "✓ inline-end"
    radius: '{rounded.md}'
    fontSize: 64px
    caption: "counts line beneath, {colors.text-secondary}, 48px"
  distribution-bar:
    fill: '{colors.green-600}'
    fillCorrect: '{colors.success}'
    label: "count + percent, {colors.text-secondary}, tabular-nums, 48px, always outside the fill on white; correct bar's label carries ✓"
    animation: "~400ms scaleX on entry; static under prefers-reduced-motion"
  timer-hero:
    diameter: 220px
    border: "8px rgba(255,255,255,0.9)"
    numeral: "{typography.display} 96px {colors.ink-on-dark}"
    urgent: "at ≤5s: border {colors.gold} and thickens to 14px"
  timer-inline:
    diameter: 60px
    border: "3px {colors.green-800}"
    numeral: "{colors.green-800} weight 800"
    urgent: "at ≤5s: border and numeral {colors.error}"
    note: "Host Dashboard only — never gold on light surfaces"
  split-hero:
    top: "{colors.green-800} band, 30–35% of stage height: progress label ({colors.ink-on-dark-muted}), timer-hero, live answer count"
    bottom: "{colors.surface-raised}: question + stage options / stage-answer-card"
    rule: "never inverted; game stages only"
  leaderboard-row:
    rank: '{colors.text-secondary} 600'
    name: '{colors.text-primary} 600'
    score: '{colors.green-800} 700 tabular-nums'
    minHeight: 72px
    fontSize: 48px
    border: 'bottom {colors.border-light}'
  leaderboard-row-mover:
    background: '{colors.green-50}'
    indicator: "▲ + places climbed, {colors.green-600}, beside the rank, ≥40px"
  stage-lobby:
    code: "{typography.display} 900, 120px+"
    phone: "{typography.display} 900, same scale as the code, digit-grouped, tabular-nums"
    instruction: "{typography.heading} 800, 64px"
    counter: "oversized pill: {colors.green-50} background, {colors.green-800} tabular numeral"
  winner-card:
    background: '{colors.green-800}'
    name: "{colors.gold}, {typography.display}, projection-large; ties: up to 3 names stacked, score shown once"
    score: '{colors.ink-on-dark-muted}'
    celebration: "CSS geometric confetti, {colors.gold} + {colors.green-600}, no images"
  host-stat-pill:
    background: '{colors.green-50}'
    text: '{colors.green-800}'
    border: '{colors.border-light}'
    shape: '{rounded.pill}'
  host-primary-action:
    base: "shadcn Button variant=default"
    background: '{colors.green-800}'
---

## Brand & Style

WhatsApp Clickers is built for the energy of a live room — a family holiday gathering, a corporate team day, a school year-end event. The visual language is not the polished sterility of a conference tool; it carries the heat of a game show combined with the warmth of a Jewish family celebration.

The brand speaks in two voices. The **Audience Display** is the visual voice — the projector is the game's stage, and everything below applies to it at projection scale. The **WhatsApp conversation** is the verbal voice: warm and playful (חם ומשחקי), personal, celebratory at peak moments — carried entirely by words, since WhatsApp offers no palette or typography. `[ASSUMPTION A1: emoji are the only visual vocabulary in chat — symbolic set only (🎉 🏆 ⚡ ✓); at most one per message, except the winner, final-results, and speed-bonus result messages (A20); no face or person emoji.]` Voice rules and message copy live in EXPERIENCE.md; the brand personality they express is defined here.

Festival green anchors the identity. It is the color of growth, of the natural world at full expression, and of the Jewish calendar's most joyful seasons — Chanukah, Pesach, Rosh Hashana. The platform launches with Chanukah packages and grows through the holiday cycle; the palette makes this relationship feel inevitable rather than branded.

Gold ({colors.gold}) appears in exactly two moments: when the countdown crosses into the last five seconds, and when the winner is revealed. Its rarity is its meaning. It signals "this is the moment" without needing to explain itself. Used anywhere else, it loses that power and must not be. **Gold lives only on dark green ({colors.green-800}/{colors.green-900}) surfaces — never on light ones**, where it fails non-text contrast (≈1.6:1 on white).

**Filter-safe is an identity constraint, not a limitation.** No photographs of people. No illustrations depicting mixed-gender groups. The decorative vocabulary is abstract: geometric shapes, iconography, Hebrew typography as visual texture. This serves the Haredi market while feeling intentional and modern to every other segment.

The platform speaks Hebrew as a native. No English fallbacks for Hebrew interface labels (the `JOIN` keyword is the single product-defined exception). System fonts that render Hebrew correctly on every device, with no external font CDN dependency that could be blocked by content filters.

Palette and layout provenance: selected from `.working/color-themes-1.html` (③ ירוק חגיגה) and `.working/directions-1.html` (כיוון ג — זירת הקרב), decisions #11–#14.

## Colors

Two palettes in one system, with a shared accent.

**The Audience Display** uses the full Festival Green vocabulary: deep forest tones ({colors.green-800} for filled elements and the hero band, {colors.green-900} for text), light green tints for backgrounds ({colors.surface-base}), and white ({colors.surface-raised}) for question content.

**Host Dashboard** uses neutral slate ({colors.host-surface}, {colors.host-text}) — calmer, more tool-like, appropriate for a laptop screen managing complex state. Green appears as accent color only: primary action buttons, live stat pills, active indicators.

Both palettes share {colors.gold} as the universal signal of peak moments — always on dark green, never on light surfaces.

Semantic colors — {colors.success}, {colors.error}, {colors.warning} — are separate from the brand green: {colors.success} (#15803D) is deliberately darker than {colors.green-600} so a correct answer reads as *different information*, not as more brand green. `[ASSUMPTION A18: success re-pointed from #16A34A after the accessibility review — the old value collided with green-600 and put white text at 3.3:1 at the Reveal.]`

The WhatsApp conversation carries no palette. There, the brand is voice, rhythm, and the two-moment discipline translated verbally: 🏆 and מזל טוב appear only at the winner moment, exactly as gold does on screen.

**Contrast targets (load-bearing combinations, WCAG AA):**

| Combination | Ratio | Requirement |
|---|---|---|
| {colors.ink-on-dark} on {colors.green-800} — options, hero band | 7.1:1 | ≥4.5:1 ✓ |
| {colors.ink-on-dark} on {colors.success} — correct option, answer card | 5.0:1 | ≥4.5:1 ✓ |
| {colors.text-primary} on {colors.green-100} — dimmed options | 9.7:1 | ≥4.5:1 ✓ |
| {colors.gold} on {colors.green-800} — winner name, urgent ring | 4.3:1 | ≥3:1 (large text / non-text only — never body-size) ✓ |
| {colors.ink-on-dark-muted} on {colors.green-800} — progress labels | 4.4:1 | ≥3:1 (large text only) ✓ |
| {colors.text-primary} on {colors.surface-base} | 10.6:1 | ≥4.5:1 ✓ |
| {colors.host-text} on {colors.host-surface} | 17.4:1 | ≥4.5:1 ✓ |
| {colors.green-800} ring on {colors.host-surface} — timer-inline | 6.9:1 | ≥3:1 non-text ✓ |

## Typography

Hebrew requires a specific approach. The `system-ui` stack resolves to San Francisco Hebrew on iOS/macOS, Segoe UI on Windows, and Noto Sans Hebrew on Android — each a high-quality Hebrew face, available without any network request. No webfont URLs are used: a blocked font CDN would silently fall back to an incorrect face.

Four semantic roles with no in-between weights:

- **Display (900):** The timer numeral. The winner's name. The lobby JOIN Code *and phone number* — transcribing that number is the hardest visual task in the product and gets maximum scale. Moments of maximum intensity — the weight reflects the stakes.
- **Heading (800):** The question. Stage instruction lines. The thing the room reads while the timer runs — heavy enough to land in one pass from the back row.
- **Body (500):** Option text. Prose copy. Present and readable, not competing with the heading.
- **UI (600):** Labels, progress text, badges, live counters. Between heading and body — informational, not decorative.

**Projection scale.** `[ASSUMPTION A19: the ramp assumes a projected image ≥2.5m wide (typical 100"+ event projection), legible to ~12m; at 1920×1080 — timer numeral 96px, question text 64px, option text 40px, leaderboard rows and distribution labels 48px, progress/counter labels 40px. Nothing on the stage below 40px.]` Question text holds at 64px — legibility pressure on it is softer because every Participant holds a private copy in WhatsApp; content that exists *only* on the display (leaderboard, distribution, counters) sits at 48px+. All sizes in rem so the whole ramp scales with viewport width. Organizer preflight guidance: stand at the back row and read the smallest text before starting (EXPERIENCE.md, Flow 1).

Letter-spacing is zero on all Hebrew text. Hebrew is dense at positive tracking and falls apart at negative. Uppercase labels (for any English-language interface elements) receive +0.08em tracking.

Line-height: heading at 1.38, body at 1.55. Generous — these are read across loud rooms by people of all ages.

**Bidi isolation is mandatory** wherever LTR tokens (the `JOIN` keyword, Latin codes, phone numbers, digits) sit inside RTL Hebrew: `<bdi>`/`dir` spans on web surfaces, and bidi-safe composition (isolate marks) in `messages_he.go` templates. Without it the Unicode bidi algorithm can visually reorder code and number fragments.

## Layout & Spacing

Base unit: 4px. Scale: 4 / 8 / 12 / 16 / 24 / 32 / 48 / 64px. Mixed values between scale stops are not used.

**Audience Display:** 16:9 landscape, designed at 1920×1080, minimum 1280×720. Single column, full-bleed. The split-hero band spans full stage width. `[ASSUMPTION: 48px safe margin on all edges — projector overscan tolerance.]` Content below the band: question centered, options in a single column (or 2×2 grid when all four options are short `[ASSUMPTION]`), 16px between options.

**Host Dashboard:** Left navigation sidebar (RTL: right sidebar) at 240px, main content area fills remainder. Optimized 1280px+; the single-column collapse (top navigation, stacked panels) is a **supported, tested mode** — it is the same code path 200% zoom hits (WCAG 1.4.4). Content panels use 24px internal padding.

**Targets:** the Audience Display has no interactive elements — output only. Host Dashboard buttons are sized to 40px height as desktop-standard, with 8px padding around them extending the hit area to a 48px effective target.

## Elevation & Depth

**Audience Display: flat.** No box-shadows on any stage element. The split between dark-green hero band and white body communicates depth through color contrast alone. Shadows on a projector add visual noise that competes with fast-changing content — and projectors wash them out anyway.

**Host Dashboard: two levels.** Level 0 — page background ({colors.host-surface}). Level 1 — panels and cards (white, bordered with {colors.host-border}). No box-shadow on cards. Modals only: `box-shadow: 0 8px 32px rgba(0,0,0,0.18)`.

## Shapes

- **Stage options:** {rounded.sm} (8px). Not pill-shaped — options are choices, not tags.
- **Split-hero top band:** no border-radius at the top (full bleed). Bottom edge: {rounded.md} (12px) on the band transition, or zero if the body flows without visual break.
- **Answer card (Free-Text reveal):** {rounded.md} (12px).
- **Host Dashboard cards:** {rounded.md} (12px).
- **Modal dialogs:** {rounded.xl} (22px).
- **Badges and pills:** {rounded.pill}.
- **Timer ring:** circular, always.

No fully-rounded options. No sharp square corners anywhere — the platform is warm, not clinical, but controlled.

## Components

See frontmatter `components:` for token specifications. Behavioral rules live in EXPERIENCE.md.

**Host Dashboard inherits shadcn/ui defaults.** This file specifies only the deltas: {components.host-primary-action}, {components.host-stat-pill}, and the palette overrides per Colors. Builder inputs, dialogs, tables, toasts, Question Bank package cards, and the "מהמאגר" badge are shadcn defaults with the host palette — deliberately unspecified further.

The **stage-option** is the projection descendant of the phone-era answer-option (superseded by the WhatsApp-only pivot, PRD 2026-07-08). It keeps the identity system — filled green, bold letter prefix (א–ד), position-as-identity — but sheds every interactive state: nobody taps the projector. The room reads it; the phones answer it. At Reveal, wrong options demote to **stage-option-dimmed** — light green fill with dark text — because the room re-reads them against the distribution bars ("רוב החדר אמר ג!"); the dim demotes, it never erases.

The **stage-answer-card** is the Free-Text counterpart of the marked correct option: at Reveal it presents the Accepted Answer's primary form as a single success-filled card with ✓, with an answered/correct counts line beneath. `[ASSUMPTION A15]`

The **split-hero** is the signature layout, now at stage scale. Green band on top = time and progress (the game's control domain). White below = the question and options (the thinking domain). This split is not a decoration — it is a structural signal. It is never inverted and never used on non-game stages.

The **distribution-bar** appears only at MCQ Reveal: it turns the room's collective answer into a visible shape without exposing any individual — personal grades arrive privately in each Participant's WhatsApp. The correct bar is distinguished by both its {colors.success} fill *and* the ✓ in its label — two greens alone are never reliably distinguishable for color-blind viewers at distance.

The **winner-card** uses CSS-animated geometric shapes for the celebration effect. No confetti images, no GIFs, no external assets. Geometric circles and rectangles falling in {colors.gold} and {colors.green-600} serve the purpose and are filter-safe. Ties: up to three names stacked at Display weight, the shared score shown once. `[ASSUMPTION A16]`

## Do's and Don'ts

| Do | Don't |
|---|---|
| Filled {colors.green-800} on stage options — unambiguous, high contrast at distance | Outlined options that blend into the white body |
| {colors.gold} only for timer urgency (≤5s) and winner takeover — and only on dark green | Gold on white/light surfaces (≈1.6:1); gold borders, labels, accents elsewhere |
| Option letter bold (700), option text regular (500) | Same weight for letter prefix and text — the letter becomes invisible |
| RTL: option letter on inline-start, text flows inline-end | LTR layout assumptions — the letter must be on the reading-start side |
| Bidi-isolate JOIN codes, phone numbers, digits inside Hebrew text | Raw LTR tokens in RTL strings — the bidi algorithm will reorder them |
| Stage text ≥40px at 1080p; display-only content ≥48px `[A19]` | Fine print, captions, or footnotes on a projected surface |
| Dim wrong options with a light fill, dark text ({components.stage-option-dimmed}) | Opacity-fading white-on-green text into illegibility |
| Abstract / geometric decoration only | Photos of people, illustrations depicting gender |
| System-ui font stack — renders correctly without network requests | External webfont URLs that content filters may block |
| White ring on dark-green hero band for timer; thicken + gold at ≤5s | Green ring on green background; hue-only urgency cues |
| {colors.success} fill **plus** ✓ icon for correct states | Color alone to communicate correct/wrong |
| Hebrew-native microcopy on every surface, including WhatsApp | English words embedded in Hebrew strings (except the `JOIN` keyword) |
| In chat: symbolic emoji, sparingly (🎉 🏆 ⚡ ✓) `[A1]` | Face/person emoji, emoji chains, decorative emoji noise |
| Gender-neutral Hebrew address (נרשמת, ענית, זכית) `[A2]` | Gendered forms ("ברוך הבא", "חכה") — half the room is not male |
