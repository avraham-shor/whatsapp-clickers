---
name: WhatsApp Clickers
status: final
created: 2026-07-06
updated: 2026-07-09
sources:
  - _bmad-output/planning-artifacts/briefs/brief-whatsapp-clickers-2026-06-18/brief.md
  - _bmad-output/brainstorming/brainstorming-session-2026-06-24-1000.md
  - _bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md
  - _bmad-output/planning-artifacts/architecture.md
  - _bmad-output/planning-artifacts/epics.md
---

> **Revision 2026-07-09 (OQ-7).** The PRD's WhatsApp-only pivot (logged 2026-07-08) supersedes this spine's previous participant mobile-web surfaces. Participants play entirely in WhatsApp; the Audience Display — previously a v2 assumption — is now the game's stage. PRD user journeys UJ-1–UJ-5 are authoritative for Key Flows. Validated at the Finalize reviewer gate (rubric + accessibility); findings dispositioned same day.

## Foundation

Three surfaces, one product:

- **WhatsApp conversation** — the Participant's only surface. Questions arrive as messages; answers are replies; results come back in the same chat. Works identically on any device that runs WhatsApp, including kosher smartphones with no browser. No UI system — this surface is conversation design, governed by Voice and Tone below. All outbound copy is centralized in `messages_he.go` (architecture).
- **Audience Display** — the projector-facing web view of the live Game. Output-only: no interactive controls. Opened from the Host Dashboard as a separate browser window. 16:9 landscape. RTL Hebrew. Carries the authoritative countdown timer and the room's drama.
- **Host Dashboard** — the Organizer's authenticated desktop web surface: builder, lobby, live control, results. Multi-panel, keyboard and mouse. RTL Hebrew. Optimized for 1280px+; the single-column collapse is a supported mode (see Responsive & Platform).

UI system for the web surfaces: shadcn/ui. `DESIGN.md` is the visual identity reference; this spine owns experience and behavior. shadcn default tokens are overridden per `DESIGN.md` where specified; DESIGN.md Components states the inheritance contract.

The marketing site is out of pilot scope (PRD §6.2 — commercial phase); it is no longer part of this spine.

## Information Architecture

### WhatsApp conversation — outbound message types

This inventory and the grammar matrix below mirror the **message templates table** (Voice and Tone), which is the canonical row list — a new message type must be added there first.

| Message | Trigger | Content |
|---|---|---|
| Welcome | Valid `JOIN <code>` during lobby | Name confirmation, keep-this-chat-open instruction, name-correction hint |
| Pre-lobby reply | Valid `JOIN <code>` before the lobby opens | Code recognized; registration hasn't opened; try again when the Organizer announces `[ASSUMPTION A17]` |
| Question — MCQ | Organizer opens an MCQ Question | Question number/total, question text, options א–ד, how to answer (letter or digit) + time limit |
| Question — Free-Text | Organizer opens a Free-Text Question | Question number/total, question text, how to answer + time limit |
| Acknowledgment | Valid answer to an open Question | "התקבל ✓" — receipt without grade (FR-5) |
| Already answered | Second answer to the same Question | First answer counts; selection is final |
| Format hint | Unparseable reply during an open MCQ | How to answer (letter or digit); Participant may still answer |
| Too long | Free-Text reply over 200 characters | Limit hint; Participant may resend shorter |
| Question closed | Answer after the server-side cutoff | Polite rejection, anticipation for the next question (FR-7) |
| Personal result | Organizer reveals; answerers only | Grade, points, Speed Bonus if any, current rank (FR-6). Non-answerers get **silence** — the room's screen shows the reveal; no scolding |
| Final results | Game ends; Participants with a score | Winner name(s) + score, personal score + rank |
| Final results — spectator | Game ends; Spectators | Winner name(s) + score only — a Spectator never answers, so has no score or rank to report |
| Final results — no winner | Game ends with no positive score (e.g. stopped before the first Reveal) | No winner named, no personal placing, no trophy; sent to Participants and Spectators alike, and no winner message follows |
| Winner's final message | Game ends; winner(s) only | Personal מזל טוב variant; ties addressed jointly `[ASSUMPTION A16]` |
| Help (Universal Reply) | Any unrecognized message, any time | How to join, where the code comes from (FR-2) |
| Invalid code | `JOIN` with unknown/expired code | Code not found; check with the Organizer |
| Spectator notice | `JOIN` after game start | Game already started; results will arrive at game end (FR-3) |
| Name updated | `שם: <השם>` reply from a registered Participant | Confirmation of the new display name `[ASSUMPTION A3 — resolves OQ-3: profile name by default, correctable by reply]` |

Navigation: none. The conversation is server-driven push; a Participant can only reply. The chat never goes silent on anyone — every inbound message class, including media and stickers, gets a response (Universal Reply).

### Audience Display — stages

One component per stage (architecture). Output-only; transitions follow Organizer actions.

| Stage | Content (FR-10) |
|---|---|
| Lobby | JOIN instructions ({components.stage-lobby}: code + phone number both at Display scale) + live participant counter climbing |
| Question — MCQ | Split-hero: progress + {components.timer-hero} + live answer count ("63 ענו") above; question + {components.stage-option} rows below |
| Question — Free-Text | Split-hero as above; below: question + instruction line "כתבו את התשובה בוואטסאפ" (Heading 800) in place of option rows `[ASSUMPTION A15]` |
| Reveal — MCQ | Correct option marked ({components.stage-option-correct}); wrong options demoted ({components.stage-option-dimmed}); answer distribution ({components.distribution-bar}) |
| Reveal — Free-Text | {components.stage-answer-card}: the Accepted Answer's primary form, success-filled with ✓; counts line "X ענו · Y צדקו". No distribution bars `[ASSUMPTION A15]` |
| Leaderboard | Top 10 ranks with movement indicators ({components.leaderboard-row-mover}) `[ASSUMPTION A11: top-10 depth]` |
| Winner takeover | {components.winner-card}: winner name(s), score, geometric celebration; ties stack up to three names `[ASSUMPTION A16]` |

### Host Dashboard — surfaces

| Surface | Purpose |
|---|---|
| Sign-in | Pilot: manually provisioned credentials or magic link `[ASSUMPTION per PRD §4.4]` |
| Game builder | Create game; add/edit MCQ + Free-Text Questions (Accepted Answers, per-question time limit); configure scoring; receive JOIN Code |
| Question Bank | Browse Packages, preview, import into a Game (FR-12) |
| Lobby | Registered participants live (count + list), launch Audience Display, start Game |
| Live control panel | Drive the state machine: open/close Question, Reveal, Leaderboard (skippable), next/end |
| Post-game results | Final Leaderboard + per-question response rates on screen. No export — out of pilot scope |

Superseded and removed from IA: all participant mobile-web screens; account/programs surface (manual provisioning in pilot); marketing surfaces; the "הורד תוצאות" export CTA.

→ Visual references (spine wins on conflict, stated once for all of them):
- `mockups/key-stage-lobby.html` — lobby stage · `mockups/key-stage-question.html` — MCQ question stage + ≤5s gold state · `mockups/key-stage-reveal.html` — MCQ reveal with distribution · `mockups/key-stage-winner.html` — winner takeover.
- `mockups/key-screens-1.html` predates the pivot: its participant MCQ screen is superseded; its host control panel remains indicative; its winner screen's identity system (gold-on-green, confetti) is indicative while its phone-scale layout is superseded by the stage {components.winner-card}.

## Voice and Tone

**The WhatsApp bot is warm and playful (חם ומשחקי)** — decision 2026-07-09. It sounds like an enthusiastic game host, not a system. Warmth comes from personal address, celebration at peak moments, and encouragement after misses — never from verbosity. Messages stay short and front-loaded: participants read under time pressure, so the first three words carry the message.

Rules:

- **Playfulness lives in low-pressure moments** — welcome, results, final results, spectator and help replies. While a Question is open, messages are minimal and instant ("התקבל ✓ — בהצלחה!").
- **Emoji:** symbolic set only — 🎉 🏆 ⚡ ✓ — at most one per message, except the **winner, final-results, and speed-bonus result** messages, which may carry two. `[ASSUMPTION A1 + A20 — filter-safe conservatism]`
- **Gender-neutral Hebrew:** the platform cannot know a Participant's gender. Use past-2nd-singular forms whose spelling reads for both (נרשמת, ענית, זכית, סיימת), impersonal constructions ("התוצאות יגיעו לכאן"), and plural imperatives for instructions (השיבו, שלחו, כתבו). Never "ברוך הבא", never "חכה". `[ASSUMPTION A2]`
- **The 🏆 discipline:** the trophy and "מזל טוב" appear only at the winner moment — the verbal equivalent of DESIGN.md's gold rule.
- Host Dashboard copy stays direct and terse (unchanged); the warm-playful register belongs to the participant conversation.

### WhatsApp message templates

**This table is the canonical message list** — `messages_he.go` implements exactly these rows; the IA inventory and grammar matrix mirror it.

Placeholders: `[N]` question number · `[Q]` total questions · `[T]` time limit (s) · `[שם]` display name · `[קוד]` JOIN Code · `[ניקוד]` points · `[בונוס]` Speed Bonus points · `[דירוג]` current rank · `[תשובה נכונה]` correct answer.

| Message | Copy | When |
|---|---|---|
| Welcome | "היי [שם], נרשמת! 🎉 השאירו את הצ'אט הזה פתוח — השאלות יגיעו לכאן. (לא [שם]? שלחו לדוגמה — שם: רחל לוי)" | Valid JOIN during lobby |
| Pre-lobby reply | "הקוד נכון! ההרשמה עוד לא נפתחה — שלחו שוב את ההודעה כשהמארגן מכריז שמתחילים." | Valid JOIN before lobby `[A17]` |
| Question — MCQ | "שאלה [N] מתוך [Q]:\n[שאלה]\nא. [...]\nב. [...]\nג. [...]\nד. [...]\nהשיבו באות (א–ד) או בספרה (1–4) — יש לכם [T] שניות!" | Question opens |
| Question — Free-Text | "שאלה [N] מתוך [Q]:\n[שאלה]\nכתבו את התשובה בהודעה — יש לכם [T] שניות!" | Question opens |
| Acknowledgment | "התקבל ✓ — בהצלחה!" | Valid answer `[A6: contains the PRD-mandated "התקבל ✓"]` |
| Already answered | "כבר ענית ✓ התשובה הראשונה היא שקובעת." | Second answer |
| Format hint (MCQ) | "כדי לענות שלחו אות (א–ד) או ספרה (1–4) — עוד יש זמן!" | Unparseable reply, question open |
| Too long (Free-Text) | "התשובה ארוכה מדי — עד 200 תווים. שלחו שוב, בקצרה!" | >200 chars |
| Question closed | "השאלה נסגרה — מתכוננים לשאלה הבאה!" | Late answer |
| Result — correct | "נכון! 🎉 +[ניקוד] נקודות\nמקום [דירוג] בטבלה" | Reveal; answered correctly, no bonus |
| Result — correct + bonus | "נכון! 🎉 +[ניקוד] נקודות\n⚡ בונוס מהירות +[בונוס]\nמקום [דירוג] בטבלה" | Reveal; won a Speed Bonus (two emoji allowed, A20) |
| Result — wrong | "לא נכון הפעם. התשובה: [תשובה נכונה]\nמקום [דירוג] בטבלה — עוד הכול פתוח!" | Reveal; mid-game `[A5]` |
| Result — wrong, last question | "לא נכון הפעם. התשובה: [תשובה נכונה]\nמקום [דירוג] בטבלה" | Reveal; final question (no "עוד הכול פתוח") `[A5]` |
| Final results | "המשחק נגמר! 🏆 הזוכה: [שם] עם [ניקוד] נקודות.\nסיימת במקום [דירוג] עם [ניקוד] נקודות — כל הכבוד!" | Game ends; Participants with a score. Ties: "הזוכים: [שם] ו-[שם] עם [ניקוד] נקודות" `[A16]` — **all** tied names are joined, with no cap: A16's "up to three names" is a Winner-takeover *layout* constraint (finite projector space), not a copy rule, and WhatsApp has no such limit |
| Final results — spectator | "המשחק נגמר! 🏆 הזוכה: [שם] עם [ניקוד] נקודות." | Game ends; Spectators (no personal score/rank line — Spectators never answer); ties: "הזוכים: [שם] ו-[שם] עם [ניקוד] נקודות" `[A16]` |
| Final results — no winner | "המשחק נגמר! הפעם לא נצברו נקודות — נתראה במשחק הבא!" | Game ends with no positive score (e.g. stopped before the first Reveal); sent to Participants and Spectators alike; no winner message is sent |
| Winner's final message | "מזל טוב, [שם]! 🏆 ניצחת עם [ניקוד] נקודות!" | Game ends, winner(s); ties: "מזל טוב, [שם] ו-[שם]! 🏆 ניצחתם עם [ניקוד] נקודות!" |
| Help (Universal Reply) | "כאן משחק החידון! כדי להצטרף שלחו: JOIN ואחריו הקוד (לדוגמה: JOIN COHEN24). את הקוד מקבלים מהמארגן." | Any unrecognized message |
| Invalid code | "הקוד [קוד] לא נמצא. בדקו את הקוד עם המארגן ושלחו שוב: JOIN ואחריו הקוד." | Unknown/expired code |
| Spectator notice | "המשחק כבר התחיל! נרשמת כצופה — התוצאות יגיעו לכאן בסוף המשחק 🏆" | JOIN after start |
| Name updated | "עודכן ✓ מעכשיו: [שם]" | Registered `שם:` reply |

Free-Text results never mention which Validation Stage matched (exact/fuzzy/AI) — that is organizer-facing detail. `[ASSUMPTION A7]`
All templates compose LTR tokens (JOIN, codes, digits) bidi-safe per DESIGN.md Typography.

### Host microcopy

| Context | Write | Don't write |
|---|---|---|
| Open question CTA | "פתח שאלה" | "הצג שאלה למשתתפים" |
| Close question CTA | "סגור שאלה" | "עצור קבלת תשובות" |
| Reveal answer CTA | "גלה תשובה" | "הצג את התשובה הנכונה" |
| Next question CTA | "שאלה הבאה ←" | "עבור לשאלה הבאה" |
| Launch display CTA | "פתח מסך קהל" `[ASSUMPTION]` | "הצג מצב מקרן" |
| Stop game control | "עצור" (opens confirm dialog) | Instant-kill buttons without confirmation |
| Live stat | "87 ענו" ({components.host-stat-pill}) | "87 משתתפים השיבו" |
| Error — connection | "החיבור נפסק — מתחבר מחדש..." | "שגיאת רשת" |

Errors describe what happened and what the system is doing. Never apologies. Never vague ("שגיאה"). Never blame the user. UI strings centralized in `strings.he.ts`.

## Component Patterns

Behavioral. Visual specs live in `DESIGN.md.Components`.

### WhatsApp conversation grammar

The reply for every inbound message is a function of (Game state × message class). No inbound message is ever ignored.

| Inbound | Pre-lobby (draft) | Lobby | Question open | Between questions² | Game over |
|---|---|---|---|---|---|
| `JOIN <valid>` | Pre-lobby reply `[A17]` | Welcome (idempotent — same reply on repeat) | Spectator notice | Spectator notice | Help |
| `JOIN <invalid>` | Invalid code | Invalid code | Invalid code | Invalid code | Invalid code |
| Letter א–ד / digit 1–4 (registered) | Help | Help | Ack (first) / Already answered | Question closed | Help |
| Free text (registered, FT Question open) | Help | Help | Ack / Too long (>200) | Question closed | Help |
| Unparseable during open MCQ | — | — | Format hint | Question closed | — |
| `שם: <השם>`¹ | Name updated | Name updated | Name updated | Name updated | Name updated |
| Anything else (incl. media, stickers, empty) | Help | Help | Help | Help | Help |

¹ Registered Participants only; an unregistered `שם:` gets Help.
² Covers the closed, revealed, and leaderboard states — everything between one question's cutoff and the next question's open.

Answer intake: MCQ replies accepted as letter (א–ד) or digit (1–4) — and both are advertised in the question copy; digits are the easier motor path for Participants with tremors or low literacy, for children, and for screen-reader users. First valid answer is final (FR-8). Cutoff is a single server-side moment — the earlier of timer expiry or Organizer close (FR-7); the receipt timestamp decides.

### Audience Display stages

- **Transitions:** stage follows Organizer actions within 1s (FR-9). Brief cross-fade `[ASSUMPTION: ≤300ms; instant under reduced-motion]`.
- **Timer:** authoritative countdown for the room. Ring depletes via CSS animation; at ≤5s the ring turns {colors.gold} **and thickens 8px → 14px** — never a hue-only cue. Shows remaining seconds only — never the total. Under `prefers-reduced-motion`: static ring, numeral still counts. The room's audible last-five-seconds count (Flow 5) is a designed behavior — it is the non-visual urgency channel for blind Participants.
- **Live answer count:** "63 ענו" updates in the hero band as answers arrive.
- **Reveal — MCQ:** correct option fills {colors.success} with ✓; wrong options demote to {components.stage-option-dimmed} (light fill, dark text — still legible while the room re-reads them); {components.distribution-bar} draws the room's answer shape, labels outside the fills, ✓ on the correct bar's label. No individual data ever appears on the shared screen (PRD FR-10 out-of-scope rule).
- **Reveal — Free-Text:** {components.stage-answer-card} presents the Accepted Answer's primary form; counts line "X ענו · Y צדקו". `[A15]`
- **Leaderboard:** top 10; rows that climbed carry ▲ + places moved `[A11]`, indicator sized to the stage floor (≥40px). The reshuffle animation is explicitly in the reduced-motion list — static reordering when reduced. No current-user highlighting — it's a shared screen.
- **Winner takeover:** full-screen {components.winner-card}; geometric confetti; static celebratory frame under reduced-motion. Ties: up to three names stacked, score once `[A16]`.
- **Resilience:** on connection drop, auto-reconnect and re-render current state without Organizer intervention. While reconnecting: "מתחבר מחדש..." over the last rendered state.
- **Display controls (on the dashboard):** a "הפחת אנימציות" toggle forces the static equivalents for the whole room — the audience cannot set `prefers-reduced-motion` on a projector.

### Host control panel

State machine: exactly one primary CTA active at any time. No auto-advance; every advance is an explicit Organizer action; no state can be skipped accidentally. The Leaderboard is skippable (UJ-4).

| Game state | Active CTA ({components.host-primary-action}) | Secondary CTAs |
|---|---|---|
| Lobby | "התחל משחק" | "פתח מסך קהל" |
| Between questions | "פתח שאלה [N]" | "עצור" |
| Question open | "סגור שאלה" | — |
| Question closed | "גלה תשובה" (activates once all received answers are graded — FR-16) | — |
| Answer revealed | "שאלה הבאה" | "עצור" |
| Game over | — (results summary on screen) | "חזור לראשית" |

The primary CTA is **one persistent button whose label and action swap in place** — focus is retained across state changes, and a polite live region announces the new state. "עצור" (replacing the previous inline "סיים משחק") always opens the confirm-stop dialog; ending a game is never a single action, keyboard included. While a Question is open the dashboard shows live response count and percentage, plus a {components.timer-inline} mirroring the authoritative stage countdown. "פתח מסך קהל" is available from Lobby through Game over.

### Game builder & Question Bank

- **Per-question time limit:** configurable per Question; default 20s `[ASSUMPTION A8 — matches the PRD's FR-4 example]`. The field carries guidance: 30–45s accommodates screen-reader users and slower-motor Participants; the Organizer-paced close (never auto-advance) is the built-in accommodation for a slow room.
- **Free-Text Accepted Answers:** multi-value editor; the Organizer defines every accepted string; the AI never invents correctness. The first Accepted Answer is the primary form shown at Reveal on the stage `[A15]`.
- **Question Bank browse:** Package cards with title, question count, and a question preview `[ASSUMPTION A13]`; shadcn defaults per DESIGN.md's inheritance contract. Import copies the Package's Questions into the Game; copies are fully editable and carry a "מהמאגר" badge; editing never modifies the Bank (FR-12).
- **Mixing:** imported and custom Questions reorder freely in one list.
- **Empty states:** a new Game with no Questions prompts "הוסיפו שאלה ראשונה — או ייבאו חבילה מהמאגר" with both CTAs; an empty Question Bank (pre-content pilot moment) states that packages are on the way rather than showing a bare list.

## State Patterns

| State | Participant's WhatsApp | Audience Display | Host Dashboard |
|---|---|---|---|
| Pre-lobby (draft) | Pre-lobby reply to early JOINs `[A17]` | — (not yet launched) | Builder |
| Lobby | Welcome after JOIN; then quiet until start | stage-lobby: JOIN instructions + climbing counter | Count + list rising; Start; launch display |
| Question open | Question message; ack on answer; hints on bad input | Split-hero: timer + live answer count + question + options / FT instruction | Live count/% + timer-inline; Close |
| Question closed | Late answers politely rejected | Timer at 0; options hold | Reveal (gated on grading) |
| Reveal | Personal result to answerers; **silence to non-answerers** | MCQ: correct marked + dimmed + distribution · FT: answer card + counts | Distribution; Next / Leaderboard |
| Leaderboard | — (quiet) | Top 10 + movers | Next / עצור |
| Game over | Final results to everyone; winner variant to the winner(s) | Winner takeover (tie-aware) | Results summary on screen |
| Participant's device offline | WhatsApp queues delivery on reconnect — platform sends normally; no special handling | — | — |
| Display disconnects | — | Auto-reconnect; "מתחבר מחדש..." over last state | No action needed |
| Organizer disconnects | **No message** `[ASSUMPTION A4 — resolves OQ-5: room-level problems are solved in the room]` | Holds last state; reconnect indicator | On return: dashboard re-renders current state; server-authoritative, nothing lost |
| Late joiner | Spectator notice; final results at game end | — | No notification — late join is passive |

## Interaction Primitives

- **Reply to answer** — the only participant interaction. Letter (א–ד) or digit (1–4) for MCQ; typed text (≤200 chars) for Free-Text. First valid reply is final; no confirmation step, no undo.
- **`שם:` command** — the one participant "setting": rename by reply, any time (registered Participants).
- **Audience Display: zero interaction.** Output-only. Fullscreen via the browser (F11) `[ASSUMPTION A12: no kiosk mode, no pairing flow]`.
- **Host keyboard shortcuts** (confirmed at the 2026-07-09 accessibility review, closing the earlier open note):
  - **Space** fires the primary CTA **only when focus is on `body` or the main region** — never inside inputs or dialogs. When any button has focus, native activation wins; there is no global handler racing it.
  - **Escape** has exactly one meaning: close the open dialog. It never opens one.
  - Stopping the game is the visible "עצור" control; destructive advances are always behind the confirm dialog, keyboard included.
- **No pull-to-refresh / refresh dependency** on live web surfaces — reconnection is automatic, never manual.

## Accessibility Floor

Behavioral. Visual contrast ratios live in `DESIGN.md.Colors` (contrast table — headline: white on {colors.green-800} = 7.1:1, white on {colors.success} = 5.0:1).

- **WhatsApp:** plain text only — no formatting-dependent meaning, no ASCII art. Emoji sparing (A1) so screen readers don't spam. Numbers as digits. Screen-reader users play with WhatsApp's own accessibility (TalkBack/VoiceOver reading incoming messages) — our copy is linear and front-loaded for exactly that. Digits 1–4 advertised as an equal answer path.
- **Audience Display — distance legibility:** stage floor 40px, display-only content 48px at 1080p, viewing assumption stated in DESIGN.md `[A19]`; AA contrast on every element per the DESIGN.md contrast table; color never the sole signal (✓ marks the correct option *and* the correct distribution bar).
- **Motion:** all stage animation (timer ring, distribution bars, leaderboard reshuffle, confetti, transitions) respects `prefers-reduced-motion: reduce` — static equivalents, numeral still counts. The dashboard's "הפחת אנימציות" toggle forces the same statics for the room.
- **Live regions:** counters (`aria-live="polite"`) are throttled — announce at most every 5s or at milestones ("50 ענו"); the timer numeral is **excluded** from live regions (announced only at question open and the ≤5s threshold); `aria-live="assertive"` is reserved for stage transitions. The display is projected, but it is still a web page a screen-reader user may open.
- **Host Dashboard:** visible focus ring on all interactive elements (shadcn default preserved); the persistent primary button retains focus across state swaps; all text in `rem`, functional and non-overlapping at 200% zoom (WCAG 1.4.4) — served by the supported single-column collapse; errors programmatically associated via `aria-describedby` — never floating orphan toasts.
- **Semantics:** `dir="rtl"` + `lang="he"` on every web document; bidi isolation on LTR tokens per DESIGN.md Typography.

## Inspiration & Anti-patterns

**Lifted from WhatsApp's own simplicity:**
The entire participant UX is now the pattern the user already knows: send a message, get a reply. Registration, answering, results — one chat. We invent nothing on the participant's side; that is the product's identity (PRD §5).

**Lifted from live game show formats:**
The split-hero, now literally on a stage. The drama (timer, progress, the room's pulse) lives in the green band; the content (question, options) below. The projector is the show; the phone is the private ballot.

**Lifted from Kahoot:**
The discipline of large, unambiguous option display. We keep the size and simplicity at projection scale; we discard the color-coded grid (blue/red/yellow/green per option) which fails in RTL context and adds cognitive load — our options are uniformly green, differentiated by letter (א–ד) and position.

**Superseded — "Rejected: WhatsApp for question delivery":**
This spine previously rejected WhatsApp as the content channel (latency, threading). The PRD pivot (2026-07-08) inverted that: WhatsApp-only is the defining product decision, and the original concerns are now architecture constraints (delivery targets FR-4, message-volume envelope) rather than UX rejections.

**Rejected — per-second countdown in WhatsApp:**
No ticking messages. The time limit is stated once in the question message; the authoritative countdown lives on the Audience Display. Message-per-second would be noise, cost, and throttling risk. The room's audible count-down in the last five seconds is the designed non-visual channel.

**Rejected — scolding non-answerers:**
Participants who didn't answer get no per-question message (FR-6). The room's screen already shows the reveal; a "you missed it" message punishes graciously-losing grandparents.

**Rejected — auto-advance timers on host side:**
The Organizer controls pacing. A family of 80 may need extra time to argue about an answer. Auto-advance removes that agency. Timing is always an Organizer decision — and it is also the product's strongest accessibility accommodation: a slow room gets the time it needs.

**Rejected — sound effects (v1):**
Unchanged. Deferred to v2 as an optional host-controlled setting.

## Responsive & Platform

**WhatsApp conversation:** any device that runs WhatsApp — including kosher smartphones with no browser. No device assumptions, no viewport, no orientation. This is the pivot's core win.

**Audience Display:** 16:9 landscape, designed at 1920×1080, minimum 1280×720 `[ASSUMPTION A9/A19]`. A separate browser window on the Organizer's machine, dragged/mirrored to the projector (FR-9). Scales via rem/viewport units; no portrait layout.

**Host Dashboard:** desktop web. Optimized 1280px+. The single-column collapse (top navigation, stacked panels) is a **supported, tested mode** for the builder and live-control surfaces — it is the same code path that 200% browser zoom produces (WCAG 1.4.4), and the live control panel is a natural single column (one primary CTA + stats).

## Key Flows

PRD journeys UJ-1–UJ-5 are authoritative. UJ numbers are the binding identifiers; titles are rendered in Hebrew (the PRD's English names are the fallback for non-Hebrew readers).

### Flow 1 — UJ-1: שלמה פותח את האירוע (Organizer)

שלמה מארגן ערב משפחתי ל-60 משתתפים; המשחק הוכן מראש (10 שאלות, חבילת חנוכה מהמאגר + 3 שאלות משלו). יום קודם הוא שיתף בקבוצת המשפחה את המספר של המשחק כאיש קשר — מי ששמר אותו לא יצטרך להעתיק ספרות מהמסך.

1. שלמה נכנס לדשבורד בלפטופ ופותח את הלובי.
2. לוחץ "פתח מסך קהל" — חלון חדש נגרר למקרן, מוצג במסך מלא. הוא ניגש לשורה האחרונה בחדר ובודק שהטקסט הקטן ביותר קריא.
3. המקרן מציג: הקוד COHEN24 והמספר 050-1234567 — שניהם ענקיים — והמונה מטפס: 4... 23... 57.
4. כל מצטרף מקבל בוואטסאפ: "היי [שם], נרשמת! 🎉 השאירו את הצ'אט הזה פתוח."
5. **שיא:** כשהחדר מלא, שלמה לוחץ "התחל משחק" — המקרן עובר למצב משחק והחדר משתתק.
6. "פתח שאלה 1" — 60 טלפונים מזמזמים יחד; הטיימר הגדול מתחיל לרדת.

כישלון בשוליים: בן דוד ששולח JOIN אחרי ההתחלה מקבל: "המשחק כבר התחיל! נרשמת כצופה..." — ודודה ששלחה JOIN יום קודם קיבלה: "הקוד נכון! ההרשמה עוד לא נפתחה."

### Flow 2 — UJ-2: סבא משה מנצח (Participant, emotional climax)

משה כהן, 72, סמארטפון כשר, 60 קרובי משפחה בחדר. (ענפי הכישלון — תשובה שגויה ותשובה מאוחרת — מסופרים ב-Flows 3 ו-5.)

1. הנכד מכתיב לו: שלח JOIN COHEN24. משה שולח.
2. תשובה מיידית: "היי משה כהן, נרשמת! 🎉 השאירו את הצ'אט הזה פתוח."
3. שאלה 3 מגיעה בוואטסאפ — שאלת הלכה. משה יודע.
4. הוא משיב "א" — ראשון מכולם. "התקבל ✓ — בהצלחה!"
5. על המקרן הטבעת מתעבה והופכת זהב בחמש השניות האחרונות; החדר סופר בקול; שלמה חושף.
6. הטלפון של משה מזמזם: "נכון! 🎉 +100 נקודות / ⚡ בונוס מהירות +50 / מקום 1 בטבלה"
7. **שיא:** אחרי 10 שאלות המקרן מתפוצץ בקונפטי גיאומטרי: "מזל טוב, משה כהן! 1,240 נקודות" — והטלפון שלו מקבל: "מזל טוב, משה! 🏆 ניצחת עם 1,240 נקודות!"
8. המשפחה צווחת. מדברים על זה שבוע.

### Flow 3 — UJ-3: רבקה עונה על שאלת טקסט חופשי (Participant)

רבקה, מורה לתנ"ך, 45.

1. בוואטסאפ: "שאלה 5 מתוך 10: כתבו את שמה של אם שמשון — יש לכם 20 שניות!" על המקרן: השאלה + "כתבו את התשובה בוואטסאפ".
2. רבקה מקלידה "צרורה" ושולחת. "התקבל ✓ — בהצלחה!"
3. [בשרת: exact match — נכון. שלב ההתאמה לא מוצג לה.]
4. **שיא:** אחרי החשיפה הטלפון מזמזם: "נכון! 🎉 +100 נקודות / מקום 4 בטבלה" — והמקרן מציג את כרטיס התשובה: "צרורה ✓ — 41 ענו · 28 צדקו".

מקרי קצה: "צרורה " (רווח) → fuzzy → נכון; "אמא של שמשון" → AI semantic → נכון; "דבורה" → "לא נכון הפעם. התשובה: צרורה / מקום 9 בטבלה — עוד הכול פתוח!"

### Flow 4 — UJ-4: שלמה מציל שאלה שנכשלה (Organizer, live recovery)

שאלה 6 נסגרה עם 12% מענה — החדר היה עסוק בקינוחים.

1. שלמה לוחץ "גלה תשובה" — המקרן מסמן את התשובה; מי שענה מקבל תוצאה אישית; מי שלא — כלום. אף אחד לא ננזף.
2. שלמה מדלג על הלוח: לוחץ ישר "שאלה הבאה". הכפתור הראשי הוא אותו כפתור — רק התווית מתחלפת; הפוקוס נשאר עליו.
3. **שיא:** שאלה 7 נפתחת, הטלפונים מזמזמים, החדר חוזר. אין panic mode — הכלי נותן לו לנהל.

### Flow 5 — UJ-5: החדר צופה בדרמה (the room, shared-screen flow)

בין תשובה לתשובה, כל העיניים על המקרן.

1. שאלה פתוחה: הטיימר הגדול יורד, המונה "63 ענו" מטפס בזמן אמת.
2. חמש שניות אחרונות — הטבעת מתעבה והופכת זהב. החדר סופר בקול: "חמש... ארבע..." — גם מי שלא רואה את המסך שומע את הרגע.
3. חשיפה: התשובה הנכונה נצבעת ירוק־הצלחה עם ✓; האופציות השגויות מתבהרות אך נשארות קריאות; פסי ההתפלגות מציירים את החדר ("רוב החדר אמר ג!") — וה-✓ מסמן גם את הפס הנכון.
4. **שיא:** לוח התוצאות מתערבב — שורות שטיפסו נושאות ▲ והחדר מגיב בקריאות.
5. מקרה קצה: בן דוד עונה "ב" אחרי הסגירה — הטלפון שלו עונה בשקט: "השאלה נסגרה — מתכוננים לשאלה הבאה!" והתשובה לא נספרת. המסך המשותף לא יודע מזה דבר.
