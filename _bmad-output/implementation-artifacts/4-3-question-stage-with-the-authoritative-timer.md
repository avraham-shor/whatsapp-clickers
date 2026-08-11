---
baseline_commit: 4bd687c0f93e1ac5cafca72cfc9abe1bd9a317c1
---

# Story 4.3: Question Stage with the Authoritative Timer

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As the room,
we want the question, its options, a big countdown, and the live answer count on screen,
so that all eyes share the same drama (FR-9 timer, FR-10 question, UJ-5).

## ⚠️ Prerequisite: branch from 4.2's commit, not from `main`

**Story 4.2 (Lobby Stage) is committed and `done`** — `4bd687c` on branch `story/4-2-lobby-stage`, with all twelve of its code-review findings resolved. That commit is the `baseline_commit` above and is the tree to branch from. As of story creation it had **not** yet merged to `main` (`origin/main` is still `8f04cc5`, the 3.11 merge), so branching from `main` would give you a tree with no `lobby-stage.tsx`, no `--stage-lobby-code`, and a `stageByState` whose `lobby` entry is still a placeholder — every task below would then be editing files that do not exist.

If 4.2 has merged by the time you start, branch from `main` instead and confirm `4bd687c` is in its history.

This story consumes 4.2's shapes directly: it refactors `lobby-stage.tsx`, appends to the same `.stage-root` block, and appends to the same `display` copy block.

Verify these shapes on disk before writing any code (they are 4.2's landed state, already patched from its code review):

- `web/src/features/display/display-page.tsx` — exports `interface StageProps { snapshot: LobbySnapshot; reducedMotion: boolean }`; `stageByState.lobby` is `LobbyStage`; the other six entries are `StagePlaceholder`; the stage wrapper key is `` `${rendered.gameId}:${rendered.state}` ``.
- `web/src/features/display/lobby-stage.tsx` — holds the high-water count **and** a leading-edge throttled `aria-live="polite"` announcer built from `announceIntervalMs`, `mountedAt`, `lastAnnouncedAt`, `countRef` and a `setTimeout` effect. **Task 2 extracts exactly that announcer.**
- `web/src/features/display/lobby-stage.test.tsx` — **5 passing Vitest cases.** They are this story's regression gate for Task 2 and must stay green **without being edited**.
- `web/src/index.css` — `.stage-root` (unlayered) defines `--stage-display`, `--stage-heading`, `--stage-ui`, `--stage-body`, `--stage-margin`, `--stage-lobby-code: min(9vw, 16vh)`; an `@layer components` block holds `.stage-fade` and its two reduced-motion kills; `@keyframes stage-fade-in` sits outside the layer.
- `web/src/lib/strings.he.ts` — `display` block with `connecting`, `reconnecting`, `notFound`, `waiting`, `stateAnnouncement`, `lobby`.
- `web/src/lib/types.ts` — `CurrentQuestion` already carries `id`, `position`, `type`, `text`, `options?`, `timeLimitSeconds`, `answerCutoffAt`, `answeredCount`. **This story needs no new snapshot field.**

If 4.2 has since merged or changed further, re-confirm the quoted shapes before relying on them.

## Acceptance Criteria

1. **Given** a Question opens, **then** the display shows the question text, MCQ options with letters א–ד (or a free-text answer hint), and the live answered count ("63 ענו") from snapshots, in the split-hero layout language adapted to projection scale — question 64px Heading 800, options 40px Body 500, progress + count labels 40px UI 600 (A19); a Free-Text Question shows "כתבו את התשובה בוואטסאפ" at Heading 800 in place of option rows (A15). *(epic AC-1)*
2. **Given** the server-sent absolute deadline (`currentQuestion.answerCutoffAt`), **when** the timer renders, **then** the countdown is computed client-side from that deadline (the server cutoff remains authoritative, FR-7), showing **remaining seconds only — never the total**. *(epic AC-2)*
3. **Given** the countdown reaches ≤5 seconds, **then** the ring transitions to gold **and thickens 8px→14px** (UX-DR2, UX-DR5 — never a hue-only cue), the numeral treatment unchanged, **and** with reduced motion the ring is static while the numeral still counts (UX-DR14). *(epic AC-3)*

### Derived requirements — binding, and each has a source

These are not extra scope; they are what "the split-hero layout language" and the surrounding specs resolve to. Each is cited so you can check it yourself.

4. **The stage paints the whole split-hero, full-bleed.** DESIGN.md `components.split-hero`: green-800 band on top at **30–35% of stage height** carrying progress label → timer-hero → live answer count; `surface-raised` (**white**, not the shell's `surface-base`) below carrying the question and options; "never inverted; game stages only". `mockups/key-stage-question.html` resolves the band at **35%** with a `rounded.md` (12px) bottom edge. Like 4.2's lobby ground, the stage paints this itself behind the shell's `--stage-margin` — the shell's contract stays exactly `StageProps`.
5. **This stage owns BOTH `question_open` and `question_closed`.** EXPERIENCE.md State Patterns: *"Question closed | … | Timer at 0; options hold"*. `display-page.tsx` already comments both entries `// story 4.3 — question stage`. At `question_closed` the numeral reads **0**, the ring is **not** urgent (gold's moment has passed — UX-DR2 rations it to two moments), and the question, options and answered count hold unchanged.
6. **The countdown is clamped against clock skew in both directions.** The deadline is absolute and the projector laptop's clock is not trusted: `remainingMs = clamp(Date.parse(answerCutoffAt) - Date.now(), 0, timeLimitSeconds * 1000)`. The upper clamp is not decoration — a client clock running **behind** the server would otherwise display a number **larger than the total**, which AC-2 forbids in as many words. A non-finite parse renders 0.
7. **The answered count holds a high-water mark, exactly as 4.2's lobby counter does.** `deferred-work.md`'s 2.4 entry (measured evidence: *3 of 4 E2E runs delivered counts that go backwards*) records that `seq` is stamped at `Broadcast()`-call time, so concurrent writers can deliver a lower count under a higher `seq` and the client's `seq < lastSeq` guard accepts it. Answers arrive in exactly that burst shape. The same guard is sound here for the same reason — within one open Question the answered count is monotonically non-decreasing by construction (`answers` is append-once under `UNIQUE (question_id, participant_id)`; nothing deletes an answer row) — and it must **reset when `currentQuestion.id` changes** so question 2 does not inherit question 1's count.
8. **The timer numeral is excluded from live regions; the answered count is polite and throttled; the ≤5s threshold announces once.** EXPERIENCE.md Accessibility Floor, verbatim: *"counters (`aria-live="polite"`) are throttled — announce at most every 5s or at milestones; the timer numeral is **excluded** from live regions (announced only at question open and the ≤5s threshold); `aria-live="assertive"` is reserved for stage transitions."* Question-open is already announced by the shell's assertive region. So: no live region on the numeral, one polite throttled region for the count, one polite region that fills once at the ≤5s threshold.
9. **`reducedMotion` is read as a JS prop here, not via the `data-reduced-motion` CSS channel.** Both channels exist (4.1's Dev Notes). `.stage-fade` uses the CSS one because the **shell** owns that animation and no component reads it. The ring is owned by a component, and `deferred-work.md`'s 4.1 `StageProps` entry names this story as the trigger for closing its `reducedMotion` half: *"its timer ring is the first stage motion, and the first thing that can observe `reducedMotion` being wrong."* A CSS-only kill is untestable in jsdom and would leave that entry open. **The prop drives it, and a test asserts it.**
10. **A degraded snapshot must not crash the projector.** `game/emptySnapshot` sets `CurrentQuestion: nil` while carrying the real `State`, so a post-commit `buildSnapshot` failure at open-question delivers `state: "question_open"` with `currentQuestion: null`. There is no error boundary anywhere in the app (4.1's Dev Notes), so an unguarded deref puts React Router's English crash page on the projector. Render `strings.display.waiting` and return.

### Scope boundaries for this story

- **Frontend only. Zero `.go` files change.** `CurrentQuestion` already carries `id`/`position`/`type`/`text`/`options`/`timeLimitSeconds`/`answerCutoffAt`/`answeredCount`, and `Snapshot` carries `questionCount`; every accepted answer already broadcasts a fresh snapshot (`wa/inbound.go:236`) and every control transition broadcasts one (`httpapi/control.go`). A `.go` diff means you went off-spec. The one exception is a throwaway `cmd/e2escratch` harness, deleted after use.
- **No migration, no `sqlc generate`, no `queries/*.sql` change.**
- **Two entries in `stageByState` change: `question_open` and `question_closed`.** `draft`, `revealed`, `leaderboard`, `finished` keep `StagePlaceholder` — they belong to 4.4–4.6. Do not "while we're here" any of them.
- **Gold appears here and only on the ring at ≤5s.** This is the first of gold's two permitted moments (UX-DR2); the second is 4.6's winner. No gold on the progress label, the count, the options, the question, or the band. And **no gold at `question_closed`** (derived req. 5).
- **Single-column options only.** DESIGN.md Layout offers "or 2×2 grid when all four options are short" as an `[ASSUMPTION]`; `mockups/key-stage-question.html` — the authoritative pixel resolution — renders a single column. Do not build the grid.
- **No distribution bars, no correct-answer marking, no dimmed options.** Those are 4.4's reveal stage, and the snapshot deliberately never carries the correct answer (`game/snapshot.go`'s comment on `CurrentQuestion`) — there is nothing to mark even if you wanted to.
- **No change to `web/src/lib/use-game-socket.ts`, `server/internal/ws/**`, `server/internal/wa/**`.**
- **No change to the dashboard's live control panel.** `control-page.tsx` keeps its own question block and `ResponseStats` pill; this story does not unify them.
- **No new npm package, no `shadcn add`, no `components/ui/*` edit.**
- **`lobby-stage.test.tsx` is not edited.** Task 2 must leave its 5 cases passing verbatim; that is how the refactor proves it changed no behaviour.

## Tasks / Subtasks

- [x] **Task 1: `web/src/features/display/stage-props.ts` (new) — close the import cycle before four more files copy it** (no AC; `deferred-work.md` 4.2 entry, trigger: *"story 4.3 — the second stage file is the point where the pattern stops being a one-off and starts being a convention four more files will copy"*)

  `StageProps` lives in `display-page.tsx`, which value-imports every stage, so the display module graph contains a real cycle held apart by one `type` keyword that no type checker enforces. Writing `import { StageProps }` instead of `import type { StageProps }` in any of the four remaining stage files reintroduces a genuine circular dependency whose symptom is an undefined component at module-init time — a blank projector.

  - [x] Create `web/src/features/display/stage-props.ts` containing **only** the interface and its existing doc comment, moved verbatim from `display-page.tsx`:
    ```ts
    import type { LobbySnapshot } from '@/lib/types'

    /** What every stage component receives. …(the existing comment, unchanged)… */
    export interface StageProps {
      snapshot: LobbySnapshot
      reducedMotion: boolean
    }
    ```
  - [x] Update the three existing importers to `import type { StageProps } from './stage-props'` and **delete the "runtime cycle" comment** above each — it documents a hazard that no longer exists, and leaving it is worse than never writing it: `stage-placeholder.tsx`, `lobby-stage.tsx`, and `display-page.tsx` (which now imports the type rather than declaring it).
  - [x] **Not `@/lib/types`.** That file's header is "Wire types mirroring the Go payloads"; `StageProps` is a component contract, not a wire type. Keeping it in `features/display/` keeps the display's contract co-located.
  - [x] `display-page.tsx` must **re-export nothing**. A `export type { StageProps } from './stage-props'` left behind would preserve the very import path that recreates the trap.

- [x] **Task 2: `web/src/lib/use-throttled-announcement.ts` (new) — one announcer, two counters** (AC: derived req. 8)

  This stage needs 4.2's throttled polite announcer for the answered count. Copying it would be the third instance of this project's most expensive mistake: `deferred-work.md`'s 3.11 entries record that the *same* single-flight pattern was duplicated into three files and shipped the same bug twice, and that the fix was one `lib` hook rather than three patches. The architecture's Web boundary rule (*"shared logic is promoted to `lib`"*) makes this the same call, and `use-single-flight.ts` / `use-space-action.ts` set the location precedent.

  - [x] Extract, **verbatim in behaviour**, from `lobby-stage.tsx`: the module-level `announceIntervalMs`, the `announced` state, the `mountedAt` / `lastAnnouncedAt` / `countRef` refs, the latest-ref effect and the leading-edge `setTimeout` effect. Signature:
    ```ts
    /** The value most recently announced, or null while nothing has been
     *  announced yet. Render it inside an aria-live="polite" region; render
     *  '' when it is null. */
    export function useThrottledAnnouncement(value: number): number | null
    ```
  - [x] Carry the four load-bearing comments across unchanged — each records a bug that was actually shipped and found: (a) the region mounts **empty**, because assistive tech does not announce content already present when a live region first appears; (b) `mountedAt` suppresses the value the component **arrived** to, or every game opens by announcing "0"; (c) the **leading edge**, or a room that fills and starts inside 5s announces nothing before the stage unmounts; (d) `setState` inside the timeout callback is async and therefore legal under `react-hooks/set-state-in-effect`, while writing the latest-ref *in* an effect is legal where reading a ref during render is not.
  - [x] Rewire `lobby-stage.tsx` to `const announced = useThrottledAnnouncement(count)` and delete the now-dead local state, refs, effects and constant. Its `<p aria-live="polite" className="sr-only">` render expression is unchanged.
  - [x] **Gate: `npm test` passes with `lobby-stage.test.tsx` untouched.** All 5 cases — including the 4998ms negative boundary that pins the 5s constant, and the unmount case that pins the cleanup — must stay green. If any of them needs editing, the extraction changed behaviour and is wrong. Do not edit the test to make it pass. (`git diff 4bd687c -- web/src/features/display/lobby-stage.test.tsx` must be empty at the end of the story.)
  - [x] `announceIntervalMs = 5000` stays a module constant inside the hook, not a parameter. No caller has a reason to differ, and a parameter would let one drift off the Accessibility Floor's number silently.

- [x] **Task 3: `web/src/index.css` — two ramp steps, one animation** (AC: 1, 3)

  - [x] Append **inside the existing `.stage-root` block**, after `--stage-lobby-code`:
    ```css
      /* DESIGN.md components.timer-hero: 220px ring. 11.4583vw = 220px at
         1920; the 20.3704vh term is the same fit guard 4.2's review added to
         --stage-lobby-code - at 16:9 the two are EQUAL (20.3704vh = 220px at
         1080), so every measured value on both verified resolutions is
         unchanged and min() only engages on a viewport shorter than 16:9,
         where the 35% band would otherwise not contain the ring. */
      --stage-timer-ring: min(11.4583vw, 20.3704vh);
      /* DESIGN.md components.stage-option minHeight: 96px. 5vw = 96px at
         1920, 8.8889vh = 96px at 1080 - equal at 16:9, same guard shape. */
      --stage-option-min: min(5vw, 8.8889vh);
    ```
  - [x] Append **inside the existing `@layer components` block**, after the `.stage-fade` rules:
    ```css
      /* The timer ring's depletion (EXPERIENCE.md Component Patterns: "Ring
         depletes via CSS animation"). Duration and a NEGATIVE delay are set
         inline from the server's absolute deadline, so a display that
         connects mid-question - or reconnects - resumes at the right point
         instead of restarting the sweep. Layered for the same reason
         .stage-fade is (code review, 2026-08-09): an unlayered rule would
         outrank any animation-* utility a later stage puts on the element.
         There is deliberately NO prefers-reduced-motion rule here: this
         stage kills the sweep from the reducedMotion PROP (see the story's
         derived requirement 9), because the CSS channel is invisible to a
         unit test and would leave 4.1's StageProps entry open. */
      .stage-timer-sweep {
        animation-name: stage-timer-deplete;
        animation-timing-function: linear;
        animation-fill-mode: forwards;
        animation-duration: 0s; /* replaced inline, per question */
      }
    ```
  - [x] Append after `@keyframes stage-fade-in`, outside the layer (matching its placement):
    ```css
    @keyframes stage-timer-deplete {
      from { stroke-dashoffset: 0; }
      to   { stroke-dashoffset: var(--stage-timer-circumference); }
    }
    ```
    `--stage-timer-circumference` is set inline on the `<circle>` from the same constant the `strokeDasharray` uses, so the two can never disagree.
  - [x] **Nothing else in `index.css` changes.** Not the `@theme` blocks, not the shadcn `:root`/`.dark` variables, not `@layer base`, not the existing five ramp values or `--stage-lobby-code`, not `.stage-fade` or its two reduced-motion kills, and not the long `vw`-vs-`rem` deviation comment (Avraham's 4.1 decision, still accurate).
  - [x] **No new `@theme` colour token.** `gold`, `green-800`, `surface-raised`, `ink-on-dark`, `ink-on-dark-muted`, `text-primary` all already exist.

- [x] **Task 4: `web/src/lib/strings.he.ts` — the question copy** (AC: 1, 8)

  Append a `question` sub-block **inside the existing `display` block**, after `lobby`:

  ```ts
    // Question stage (story 4.3).
    question: {
      // EXPERIENCE.md IA > Audience Display - stages, Free-Text row, and the
      // epic AC, both verbatim. Not an assumption.
      freeTextHint: 'כתבו את התשובה בוואטסאפ',
      // The option letter as the mockup renders it ("א."). The LETTERS come
      // from questionEditor.optionLetters - see Task 5; only the trailing
      // period lives here, so the two surfaces cannot drift on the alphabet.
      optionLetter: (letter: string) => `${letter}.`,
      // [ASSUMPTION]: EXPERIENCE.md's Accessibility Floor requires the count
      // to be polite and throttled but gives no sentence. A full sentence,
      // unlike the visual "63 ענו", because a screen reader gets no layout to
      // carry the meaning - the same reasoning as display.lobby.countAnnouncement.
      countAnnouncement: (count: number) => `${count} ענו על השאלה`,
      // [ASSUMPTION]: EXPERIENCE.md's Accessibility Floor says the timer
      // numeral is announced "only at question open and the ≤5s threshold"
      // but gives no wording. Gender-neutral (A2), no digit read out - the
      // threshold is the event, not the number.
      urgentAnnouncement: 'חמש שניות אחרונות',
    },
  ```

  - [x] **`questionProgress` and `answeredStat` are NOT redefined here.** `strings.live.questionProgress` already renders `שאלה ${n} מתוך ${total}` and `strings.live.answeredStat` already renders `${count} ענו` — the exact sentences `mockups/key-stage-question.html` puts in the hero band and EXPERIENCE.md quotes for both surfaces. Import them from `live` and comment why: two blocks holding one sentence is a drift risk, and `strings.he.ts` is one module whose block names are organisational, not access-controlled. Flag the reuse in the Dev Agent Record so Avraham can say if he'd rather they were duplicated or promoted to a shared block.
  - [x] **Nothing else in `strings.he.ts` changes.** In particular leave `display.lobby`'s two strings and their `[ASSUMPTION]` comments alone. 4.2's review ticked that decision, but the markers are still in the file at `4bd687c`, so whether they were confirmed-as-is or simply not cleaned up is not knowable from the repo — either way it is 4.2's record to correct, not this story's, and editing them here would silently overwrite a decision you did not witness.
  - [x] Flag the two new `[ASSUMPTION]` items (`countAnnouncement`, `urgentAnnouncement`) in the Dev Agent Record for the same confirm-at-review treatment 4.1's and 4.2's copy got.

- [x] **Task 5: `web/src/features/display/timer-ring.tsx` (new) — the authoritative countdown** (AC: 2, 3, and derived reqs. 6, 8, 9)

  The architecture names this exact file: `timer-ring.tsx — authoritative countdown, gold ≤5s`. It owns the numeral, the sweep and the urgency state; it does **not** know about the snapshot, the band, or the game.

  - [x] **Props** — a narrow, snapshot-free contract, so the ring is unit-testable without building a whole snapshot:
    ```tsx
    interface TimerRingProps {
      /** RFC 3339 UTC, from the snapshot's currentQuestion.answerCutoffAt. */
      deadlineIso: string
      /** The question's configured limit — the upper clamp, never displayed. */
      totalSeconds: number
      /** True once the Organizer has closed the question: the numeral pins to
       *  0, the sweep does not run, and urgency is over. */
      closed: boolean
      /** Already the OR of the viewer's OS setting and the room-level toggle. */
      reducedMotion: boolean
    }
    ```
  - [x] **The remaining-time derivation** (AC-2, derived req. 6):
    ```tsx
    // Absolute deadline in, remaining seconds out. The server cutoff stays
    // authoritative (FR-7) - this only renders the room's view of it.
    //
    // Clamped BOTH ways against a projector laptop's untrusted clock. The
    // upper clamp is the load-bearing one: a client clock running behind the
    // server would otherwise display a number LARGER than the configured
    // limit, and AC-2 forbids ever showing the total. Math.ceil so the last
    // whole second reads "1" and 0 appears exactly at the deadline.
    const deadlineMs = Date.parse(deadlineIso)
    const rawMs = Number.isFinite(deadlineMs) ? deadlineMs - now : 0
    const remainingMs = closed ? 0 : Math.min(Math.max(rawMs, 0), totalSeconds * 1000)
    const remainingSeconds = Math.ceil(remainingMs / 1000)
    ```
  - [x] **The tick** — self-scheduling and aligned to the second boundary, not a naive 1000ms interval:
    ```tsx
    const [now, setNow] = useState(() => Date.now())
    // Scheduled to the NEXT integer-second boundary rather than a fixed
    // 1000ms interval: a fixed interval drifts against the deadline and makes
    // the room see a number repeat or skip. setState inside a timeout
    // callback is async, so it does not trip react-hooks/set-state-in-effect.
    // Depending on remainingMs is deliberate - each tick reschedules the next
    // one - and the effect stops scheduling once the countdown hits 0.
    useEffect(() => {
      if (remainingMs <= 0) return
      const id = setTimeout(() => setNow(Date.now()), remainingMs % 1000 || 1000)
      return () => clearTimeout(id)
    }, [remainingMs])
    ```
  - [x] **The urgency state** (AC-3): `const urgent = !closed && remainingSeconds <= 5`. Not `<= 5` alone — at `question_closed` the numeral is 0 and gold's moment has passed (derived req. 5). Urgency is **independent of `reducedMotion`**: it is the accessibility cue, not the animation.
  - [x] **The ring is an SVG circle, not a CSS border**, and the reason is worth reading before you change it. DESIGN.md's `timer-hero` describes a `border` (8px, thickening to 14px), and the mockup renders exactly that — but a border cannot deplete, and EXPERIENCE.md requires the ring to deplete (*"Ring depletes via CSS animation"*), which AC-3's own reduced-motion clause presupposes: *"the ring is **static** while the numeral still counts"* is vacuous unless the ring is otherwise moving. An SVG circle is visually identical to the border at both widths, animates via `stroke-dashoffset`, and — because viewBox units are relative — makes the stroke scale with the ring automatically, so the 8px/14px pair needs no `vw` maths.
    ```tsx
    // viewBox 226 with r=106: outer edge is 106+8/2 = 110 at rest (220px
    // diameter, DESIGN.md's figure exactly) and 106+14/2 = 113 when urgent -
    // precisely the viewBox edge, so the thickening never clips. r is
    // CONSTANT across the two widths on purpose: changing it would change the
    // circumference and make the sweep jump at the 5s mark.
    const ringRadius = 106
    const ringCircumference = 2 * Math.PI * ringRadius
    ```
    ```tsx
    <svg viewBox="0 0 226 226" aria-hidden className="block h-full w-full">
      <circle
        cx="113" cy="113" r={ringRadius} fill="none"
        // rotate so depletion starts at 12 o'clock; SVG's 0° is 3 o'clock.
        // Clockwise, the universal countdown convention - RTL mirrors text
        // and reading order, not a clock-derived graphic.
        transform="rotate(-90 113 113)"
        stroke={urgent ? 'var(--color-gold)' : 'rgba(255,255,255,0.9)'}
        strokeWidth={urgent ? 14 : 8}
        strokeDasharray={ringCircumference}
        className={sweeping ? 'stage-timer-sweep' : undefined}
        style={sweep}
      />
    </svg>
    ```
    - [x] `rgba(255,255,255,0.9)` is DESIGN.md `timer-hero.border` verbatim — **not** `ink-on-dark` at full opacity, and never a green stroke ("white ring on dark band — never green-on-green", Do's and Don'ts).
    - [x] `var(--color-gold)` rather than a `stroke-gold` utility: Tailwind's `stroke-*` on an SVG presentation attribute is a class this project has never used, and the token is right there. Gold on green-800 is 4.3:1 — DESIGN.md's contrast table clears it for **non-text** only, which a ring is.
  - [x] **The sweep, captured once per mount** (AC-2, derived req. 9):
    ```tsx
    const sweeping = !reducedMotion && !closed && remainingMs > 0
    // Captured ONCE. animationDelay/-Duration recomputed on a later render
    // would restart the sweep from full - and this component re-renders on
    // every inbound answer frame, so a per-render style object would reset
    // the ring several times a second in a busy room. The negative delay is
    // what makes a display that connects (or reconnects) mid-question resume
    // at the right point instead of starting over.
    const [sweep] = useState(() => {
      const elapsed = Math.max(0, totalSeconds - remainingMs / 1000)
      return {
        animationDuration: `${totalSeconds}s`,
        animationDelay: `${-elapsed}s`,
        // consumed by @keyframes stage-timer-deplete, so the dasharray and
        // the final offset can never disagree
        '--stage-timer-circumference': String(ringCircumference),
      } as CSSProperties
    })
    ```
    - [x] **The caller must mount this with `key={`${question.id}:${question.answerCutoffAt}`}`** (Task 6) so a new question gets a fresh capture. Do not add a `useEffect` here to re-capture; the key is the mechanism and it is one line at the call site.
    - [x] Under `reducedMotion` the class is absent, so `stroke-dashoffset` stays at its base `0` and the ring renders **full and static** — the documented static equivalent (UX-DR14). The numeral keeps ticking because the tick effect is independent of the sweep.
  - [x] **The numeral** — Display 900 at `--stage-display` (96px), `ink-on-dark`, `tabular-nums`, centred over the SVG, and **explicitly outside any live region** (derived req. 8):
    ```tsx
    <div className="relative grid size-[var(--stage-timer-ring)] place-items-center">
      {/* svg above, absolutely filling */}
      {/* No aria-live, and no role: EXPERIENCE.md excludes the timer numeral
          from live regions - a per-second announcement would bury every other
          announcement on the page. */}
      <span className="relative text-[length:var(--stage-display)] font-display leading-none tabular-nums text-ink-on-dark">
        {remainingSeconds}
      </span>
    </div>
    ```
    - [x] `leading-none`, not `leading-heading`: at 96px a 1.38 line-height adds ~36px that the 220px ring cannot absorb, and the mockup sets `line-height:1`.
    - [x] **Never render the total.** No "12 / 20", no denominator, no `totalSeconds` in any visible string (AC-2). `totalSeconds` exists solely as the clamp and the animation duration.
  - [x] **The ≤5s announcement** (derived req. 8) — a second polite region, separate from the count's, filled once when the threshold is crossed and never cleared while the question is open:
    ```tsx
    <p aria-live="polite" className="sr-only">
      {urgent ? strings.display.question.urgentAnnouncement : ''}
    </p>
    ```
    Because `urgent` flips once and stays true until the stage transitions, this announces exactly once — no throttle needed and none wanted.
  - [x] **Nothing focusable, no `shadow-*`** — output-only surface, and DESIGN.md: "Audience Display: flat."

- [x] **Task 6: `web/src/features/display/question-stage.tsx` (new) — the split-hero** (AC: 1, and derived reqs. 4, 5, 7, 8, 10)

  The architecture names this exact file: `question-stage.tsx — question, options, timer, answer count`.

  - [x] **Signature**: `export function QuestionStage({ snapshot, reducedMotion }: StageProps)`, `import type { StageProps } from './stage-props'` (Task 1). Both props are read — this is the first stage that reads `reducedMotion`.
  - [x] **The degraded-snapshot guard, first thing in the body** (derived req. 10):
    ```tsx
    const question = snapshot.currentQuestion
    // emptySnapshot carries the real State with a nil CurrentQuestion, so a
    // post-commit buildSnapshot failure delivers question_open with no
    // question. There is no error boundary in this app; an unguarded deref
    // here puts React Router's English crash page on the projector.
    if (!question) {
      return <p className="text-[length:var(--stage-heading)] font-heading text-text-secondary">{strings.display.waiting}</p>
    }
    ```
    This early return sits **above** every hook, so no hook may be called before it. Put the high-water state and the announcement hook **inside a child component** (or below the guard with no hooks above it) — an early return between hooks is a rules-of-hooks violation the linter will catch. The simplest shape that satisfies both: keep the guard first, and let `TimerRing` and a small `AnsweredCount` child own their own hooks.
  - [x] **The full-bleed split-hero** (derived req. 4). Same containing-block rule 4.2 relies on: the shell root is `position: relative` with `padding: var(--stage-margin)`, so `inset-0` resolves against its **padding box** and covers the safe margin — which is what "full bleed with the 48px margin inside it" means.
    ```tsx
    <div className="absolute inset-0 flex flex-col bg-surface-raised">
      {/* band: the game's control domain */}
      <div className="flex flex-[0_0_35%] flex-col items-center justify-center gap-2 rounded-b-md bg-green-800 px-[var(--stage-margin)] py-4">
        …progress · ring · count…
      </div>
      {/* body: the thinking domain. min-h-0 so its children may shrink
          instead of overflowing the shell's overflow-hidden. */}
      <div className="flex min-h-0 flex-1 flex-col justify-center gap-4 px-[var(--stage-margin)] pt-4 pb-[var(--stage-margin)]">
        …question · options / hint…
      </div>
    </div>
    ```
    - [x] `bg-surface-raised` (**white**) on the body, not `bg-surface-base` — DESIGN.md `split-hero.bottom` is `surface-raised`; the shell's own ground is `surface-base` and they are different colours. Getting this wrong is a visible, easy-to-miss defect.
    - [x] `rounded-b-md` = 12px, DESIGN.md Shapes: "Split-hero top band: no border-radius at the top (full bleed). Bottom edge: rounded.md."
    - [x] `flex-[0_0_35%]` — the mockup's value, and its own comment explains why: *"35% fits 40px labels"*. Do not "tidy" it to 30%.
    - [x] **Only positioned/flow siblings, in this DOM order.** Unlike 4.2 there is no separate backdrop element: this one wrapper *is* the layout, so the painting-order trap does not arise. Do not reach for `-z-10` — the shell root establishes no stacking context and a negative-z child would slide behind its background.
  - [x] **The hero band's three rows** (AC-1):
    ```tsx
    <p className="text-[length:var(--stage-body)] font-ui leading-[1.2] text-ink-on-dark-muted tabular-nums">
      {strings.live.questionProgress(question.position, snapshot.questionCount)}
    </p>
    <TimerRing
      key={`${question.id}:${question.answerCutoffAt}`}
      deadlineIso={question.answerCutoffAt}
      totalSeconds={question.timeLimitSeconds}
      closed={snapshot.state === 'question_closed'}
      reducedMotion={reducedMotion}
    />
    <p className="text-[length:var(--stage-body)] font-ui leading-[1.2] text-ink-on-dark-muted tabular-nums">
      {strings.live.answeredStat(answered)}
    </p>
    ```
    - [x] **`leading-[1.2]`, not `leading-heading`, and the 0.4px matters.** The band is 378px at 1080p. With 1.2: `py-4` 32 + two `gap-2` 16 + labels 2×48 + ring 220 = **364px**, 14px of slack. With `leading-heading` (1.38) the labels become 55.2px each and the stack is **378.4px** — 0.4px over a 378px box, inside `overflow-hidden`, i.e. a silent clip. The mockup specifies 1.2 for both labels; this is the same class of arbitrary-value deviation 4.2 documented for `leading-[1.1]`.
    - [x] `ink-on-dark-muted` (4.4:1 on green-800, cleared for large text only — these are 40px) is DESIGN.md `split-hero.top` verbatim. Not `ink-on-dark`.
    - [x] `tabular-nums` on both labels — they change while the room watches, and a proportional digit set makes the line jitter.
    - [x] The `key` on `TimerRing` is load-bearing: it is what re-captures the sweep for a new question (Task 5).
  - [x] **The answered count's high-water guard and its announcer** (derived req. 7, 8). Put both in a small `AnsweredCount` child in the same file so the degraded-snapshot early return above stays legal:
    ```tsx
    // Same guard, same reason, same evidence as 4.2's lobby counter
    // (deferred-work.md 2.4: seq is stamped at Broadcast()-call time, and 3
    // of 4 measured E2E runs delivered counts that go backwards). Answers
    // arrive in exactly that concurrent-writer shape. Sound HERE because
    // within one open Question the count only grows - `answers` is
    // append-once under UNIQUE (question_id, participant_id) and nothing
    // deletes an answer row.
    //
    // Keyed on questionId rather than trusting the stage remount: today the
    // shell's key changes on every state transition so two question_open
    // stages are never adjacent, but that is the SHELL's invariant, and
    // question 2 inheriting question 1's count would be a silent lie in
    // front of a room. State adjusted during render (React's documented
    // "storing information from previous renders"), as display-page.tsx and
    // lobby-stage.tsx both do and both document.
    const [held, setHeld] = useState({ questionId, count })
    if (held.questionId !== questionId) setHeld({ questionId, count })
    else if (count > held.count) setHeld({ questionId, count })
    const shown = held.questionId === questionId ? Math.max(held.count, count) : count

    const announced = useThrottledAnnouncement(shown)
    ```
    - [x] The visible number is **never** throttled — only the announcement is (the same split 4.2 measured: pill at 425/857/1274ms, region silent until the 5s tick).
    - [x] The polite region ships **inside `AnsweredCount`**, right beside the visible label — `sr-only` is `position: absolute`, so it consumes no layout and no `gap` slot in the band's flex column:
      ```tsx
      <p aria-live="polite" className="sr-only">
        {announced === null ? '' : strings.display.question.countAnnouncement(announced)}
      </p>
      ```
    - [x] `polite`, never `assertive` — the shell owns the one assertive region (UX-DR14); two would fight.
    - [x] `AnsweredCount` is **module-local, not exported**. `noUnusedLocals` keeps it honest, and an exported non-page component in a `.tsx` file is the shape `react-refresh/only-export-components` exists to police (it is only relaxed under `components/ui/**`).
  - [x] **The question text** (AC-1) — and the one accessibility affordance this stage adds:
    ```tsx
    <h1 className="text-center text-[length:var(--stage-heading)] font-heading leading-heading text-text-primary">
      {question.text}
    </h1>
    ```
    - [x] **`<h1>`, not `<p>`.** `deferred-work.md`'s 4.2 entry records that nothing on the Audience Display has heading semantics and re-triggers on *"whenever the shell's `StageProps` contract is next revised"* — which Task 1 does. The question is unambiguously this stage's heading, Tailwind's preflight resets `h1` font-size and font-weight to `inherit` so there is **zero visual change**, and only one stage is mounted at a time so there is never a second `<h1>`. Task 8 records the partial resolution: the shell-wide decision (does the display get an `<h1>` in every stage, and who owns it) stays deferred.
    - [x] `text-primary` on white — 10.6:1, DESIGN.md's contrast table. Not `ink-on-dark`.
    - [x] `leading-heading` (1.38) here, per DESIGN.md Typography. The band's 1.2 deviation does not extend to the body.
  - [x] **MCQ options** (AC-1) — single column, `stage-option` verbatim:
    ```tsx
    <div className="flex min-h-0 flex-col gap-4">
      {options.map((text, index) => (
        <div
          key={index}
          className="flex min-h-0 shrink basis-[var(--stage-option-min)] items-center gap-4 overflow-hidden rounded-sm bg-green-800 px-6 text-[length:var(--stage-body)] leading-body text-ink-on-dark"
        >
          <span className="font-heading">{strings.display.question.optionLetter(letters[index] ?? '')}</span>
          <span className="font-body">{text}</span>
        </div>
      ))}
    </div>
    ```
    - [x] **Letter bold (700-ish), text regular (500).** DESIGN.md `stage-option`: `letterWeight: 700`, `textWeight: 500`, and the Don'ts row calls same-weight "the letter becomes invisible". This project has no 700 token; `font-heading` (800) is the nearest existing step and preserves the *contrast* the spec is about — record the substitution rather than inventing a `--font-weight-option: 700`.
    - [x] **Letter on the inline-start side** — the RTL Do's row. `flex` + `gap-4` in a `dir="rtl"` document puts it there for free; do **not** add `flex-row-reverse` or `ml-*`/`mr-*`. Use logical properties only.
    - [x] `rounded-sm` (8px) — DESIGN.md Shapes: "Stage options: rounded.sm. Not pill-shaped — options are choices, not tags." Never `rounded-full`.
    - [x] `basis-[var(--stage-option-min)]` with `shrink` and no `grow`: the rows are 96px at 1080p (the spec figure) and shrink only when a long question leaves no room, instead of clipping. `overflow-hidden` on the row keeps a shrunk row from spilling its text.
    - [x] **`letters` comes from `strings.questionEditor.optionLetters`** (`['א','ב','ג','ד']`), not a new array — one alphabet, one source. `?? ''` guards an options array longer than four (`CurrentQuestion.Options` is a raw `[]string` on the wire; the four-option rule is enforced at authoring time, not on the display).
    - [x] `key={index}` deliberately: options have no id, and position **is** their identity (DESIGN.md: "position-as-identity").
  - [x] **Free-Text branch** (AC-1, A15) — in place of the option rows, not in addition to them:
    ```tsx
    <p className="text-center text-[length:var(--stage-heading)] font-heading text-text-primary">
      {strings.display.question.freeTextHint}
    </p>
    ```
    Heading 800 per EXPERIENCE.md's Free-Text row. Branch on `question.type === 'mcq'` (render options) vs anything else (render the hint) — not on `options` being present, so a malformed MCQ with no options renders the hint rather than an empty column.
  - [x] **Zero Hebrew literals in this file.** Every string comes from `strings`. Comments in English.
  - [x] **Nothing focusable** — no `<button>`, `<a>`, `<input>`, `tabIndex`, `onClick`. **No `shadow-*`.**

- [x] **Task 7: `web/src/features/display/display-page.tsx` — two map entries** (AC: 1, 5)

  - [x] Import `QuestionStage` and point both entries at it:
    ```tsx
    question_open: QuestionStage, // story 4.3 — question stage
    question_closed: QuestionStage, // story 4.3 — question stage (timer at 0)
    ```
    The four other entries keep `StagePlaceholder` and their `// story 4.x` comments unchanged.
  - [x] Task 1's `StageProps` move also lands here (import instead of declare).
  - [x] **Nothing else in this file changes.** The `Record<GameState, …>` annotation and its `?? StagePlaceholder` runtime fallback, the render-phase `retained` pattern, `usePrefersReducedMotion`, the OR producing `reducedMotion`, the optional-chained `displaySettings?.reducedMotion`, the always-mounted assertive announcer, the three overlay branches, the `${gameId}:${state}` key and the reconnect band's `top-[var(--stage-margin)]` offset are all 4.1/4.2 review decisions with recorded reasoning.

- [x] **Task 8: re-triage the `deferred-work.md` entries this story triggers** (no product code)

  Four entries name this story or fire on it. Each needs a written outcome appended under the entry, in the file's established style (sub-bullet, dated, trigger re-pointed off story numbers).

  - [x] **"Nothing exercises the `StageProps` wiring"** (4.1 entry; `snapshot` half closed by 4.2, *"`reducedMotion` half … Trigger for the remainder: story 4.3"*). **Outcome: CLOSED.** Record that `TimerRing` reads `reducedMotion` as a prop (derived req. 9), that `question-stage.test.tsx` / `timer-ring.test.tsx` assert the sweep class is present when false and absent when true, and therefore that silently dropping the room-level half of 4.1's OR now fails a test. Note explicitly that the CSS `data-reduced-motion` channel was **available and deliberately not used here**, with the reason, so a later reader does not "simplify" it back and reopen the entry.
  - [x] **"`StageProps` lives in `display-page.tsx` … a real import cycle"** (4.2 entry, trigger: *story 4.3*). **Outcome: CLOSED by Task 1** — record the new module, the three rewired importers, and that the "runtime cycle" comments were deleted rather than left to describe a hazard that no longer exists.
  - [x] **"Nothing on the Audience Display has heading semantics"** (4.2 entry, trigger: *"whenever the shell's `StageProps` contract is next revised … the two are the same file and should be taken together"*). **Outcome: partially closed, re-deferred.** Record that this stage's question text is now an `<h1>`; that the shell-level question (does every stage get one, and does it belong to the shell or the stage?) is untouched; and re-point the trigger to **the next stage story that renders a dominant text element** (4.4's answer card, 4.6's winner name), or the first real screen-reader evaluation.
  - [x] **"The reconnect band overlays the top of the stage content box"** (4.1 entry, re-deferred by 4.2 with the trigger re-pointed to *"the first Audience Display stage whose content actually reaches the top of the content box"*). **This story is that stage** — the split-hero is full-bleed and its band starts at y=0. Record the **measured** geometry from Task 9's browser pass and apply this decision rule:
    - If the reconnect band covers the **timer numeral or the question text** → it must be fixed, and the fix is the shell's (reserve space, or move the band). Escalate to Avraham rather than patching the stage's layout to dodge it.
    - If it covers only the **progress label and/or the ring's top arc** → record it as a pass with a note and re-defer, exactly as 4.2 did. Predicted geometry to check against: band 48–124px; hero content ~364px centred in a 378px band, so the progress label sits ~7–55px and the ring ~63–283px, i.e. the band's lower half overlaps the ring's top arc while the numeral (centred, ~173px) stays clear.
    - Also re-check the **related legibility note** (green-900 band on a green-800 ground, ~1.4:1 box with ~10:1 white text). Here the band lands on green-800 again, so this is the second data point 4.2 asked for.
  - [x] Confirm in writing that the **2.4 out-of-order broadcast entry is NOT re-triggered** — its trigger is *"the first snapshot field rendered on the display that can legitimately decrease"* and it states outright that 4.3's answered count cannot. Record that this story nevertheless applies the same high-water guard for the same measured reason (derived req. 7), so a later reader does not think the guard is a new, unjustified pattern. Also confirm the **2.5 spectator-roster entry** is still not triggered (this stage renders no roster).
  - [x] Do not re-triage the other Epic-4-adjacent entries; 4.1's and 4.2's reviews already dispositioned them.

- [x] **Task 9: quality gates, E2E, and the manual browser pass** (all ACs)

  - [x] **Frontend Hebrew centralization**: `grep -rlP '[\x{0590}-\x{05FF}]' --include='*.tsx' web/src` must list only the two known pre-existing violations (`features/builder/scoring-editor.tsx`, `features/live/control-page.tsx` — both comments, both on `origin/main`, both recorded in `deferred-work.md`). **Neither new `.tsx` file may appear.** (CI still greps only `*.go` — `deferred-work.md`'s 3.10 entry — so this is on you.)
  - [x] **Frontend gates**: `npm run lint` · `npx tsc -b --noEmit` · `npm test` · `npm run build` · both filter-safety scans (source and built bundle — no external URL, no `@import` of a font, no `url(//…)`).
  - [x] **The Task 2 regression gate, called out separately because it is the point of the refactor**: `lobby-stage.test.tsx` passes **unedited**.
  - [x] **Vitest coverage this story owes** (the first genuinely unit-testable frontend logic on the display — `deferred-work.md`'s 4.1 entry called this out a story in advance). Co-located, `describe`/`it` imported explicitly (`globals` is off), `afterEach(cleanup)` because RTL's auto-cleanup only self-registers with a global `afterEach`. Expected copy is **read from `strings.he.ts`, never retyped** — 4.2's rule, for both its reasons.
    - `timer-ring.test.tsx`: remaining seconds derived from an absolute deadline with fake timers; the numeral **never exceeds `totalSeconds`** when the client clock is behind (set `Date.now` behind the deadline by more than the limit); it floors at 0 and stops ticking; `closed` pins it to 0 regardless of the deadline; the ≤5s boundary flips the stroke to gold **and** to width 14 (assert **both** — a hue-only cue is the specific thing UX-DR5 forbids, so a test that checks only the colour would pass the exact bug); `closed` is **not** urgent even at 0; `reducedMotion` removes the `stage-timer-sweep` class while the numeral still counts (this is the assertion that closes 4.1's deferred entry); a non-finite `answerCutoffAt` renders 0 rather than `NaN`.
    - `question-stage.test.tsx`: a null `currentQuestion` at `question_open` renders the waiting copy and does not throw; MCQ renders four rows with letters א–ד from `strings.questionEditor.optionLetters`; free-text renders the hint and **no** option rows; the answered count never goes backwards and **resets on a new question id**; the progress line reads off `position`/`questionCount`; `reducedMotion` reaches the ring through `StageProps`.
    - **Demonstrate at least the two guard tests red first** (remove the clamp; remove the high-water hold) and record the failure messages, as 3.11 and 4.2 both did. A test that has never been red has proved nothing.
  - [x] **Backend, unchanged**: run `go build ./... && go test ./...` once to prove zero regression, then confirm `git status` shows **no `.go`, no `.sql`, no `migrations/` diff**.
  - [x] **Local Go E2E** (`server/cmd/e2escratch`, deleted after use — the 2.1–4.2 convention, including the fake-provider `WHATSAPP_API_BASE_URL` override so no real WhatsApp traffic leaves the machine). Reuse 4.2's two patterns verbatim rather than reinventing: the `websocket.Dial` client from `github.com/coder/websocket` with the session cookie on the dial request's header, and the signed-webhook POST (`X-Hub-Signature-256: sha256=<hmac over the RAW body>`, `webhook_test.go`'s `sign()` helper, a **unique `wamid` per message** or the dedupe ledger silently drops it — 4.2's harness lost two runs to exactly that).

    Three things only a real server proves:
    - **Scenario A — the deadline on the wire is usable.** Log in, create a scratch game with a known `timeLimitSeconds` (use two questions: one MCQ, one Free-Text), open the lobby, join two phones, start the game. On an already-open `role=display` socket, read the `question_open` frame and assert: `currentQuestion` is non-null; `answerCutoffAt` parses as RFC 3339; `answerCutoffAt - serverNow` is within a second of `timeLimitSeconds`; `options` has four entries for the MCQ and is **absent** for the Free-Text; `answeredCount` is 0; `position`/`questionCount` are right.
    - **Scenario B — the answered count really climbs, and how it interleaves.** Send signed answers from ~20 distinct phones concurrently, record every `answeredCount` the display socket receives in arrival order, and report in the Debug Log: the final value, and **whether any frame carried a count lower than one already delivered**. 4.2 measured backwards counts in 3 of 4 runs on the lobby counter; this is the same mechanism on a different field, and it is the evidence derived req. 7 rests on. Report what the run produced, not what you expect.
    - **Scenario C — `question_closed` and the cutoff.** Close the question via the control endpoint and assert the display frame carries `state: "question_closed"` with `answerCutoffAt` now **at or before** the server's clock (`CloseCurrentQuestion` does `answer_cutoff_at = LEAST(answer_cutoff_at, now())`) — the fact derived req. 5 and the `closed` prop rest on. Then send one more signed answer and confirm it is rejected and `answeredCount` does not move.
    - **A green E2E is not evidence unless you confirm which process answered** — 3.10's Debug Log records a run silently served by a leftover server, and 4.2 sidestepped it by running on port 8099 beside a live `make dev`. Build the binary once, exec it directly, grep the log for exactly one `server listening` and no `bind:` error.
    - Clean up the scratch rows afterward (cascades) and delete the harness.
  - [x] **Manual browser pass** (real browser — `playwright-core` + `channel: 'msedge'`, installed into the scratchpad and never into `web/package.json`; see 4.2's Debug Log for the working shape). The unit tests cover the timer's arithmetic; **only the browser covers the layout**, and this stage's failure modes are silent clips.

    The checks:
    1. **AC-1 (MCQ)** — open a question with four options. Green band on top with progress label, ring, count; **white** body below with the question and four filled green-800 rows, letter bold on the inline-start side, 8px corners. Band bottom edge rounded 12px. `dir="rtl"`, nothing focusable, no `box-shadow`.
    2. **AC-1 (Free-Text)** — advance to the free-text question: the hint "כתבו את התשובה בוואטסאפ" at Heading 800 replaces the rows entirely, and **no** empty option column remains.
    3. **AC-2** — the numeral counts down once per second, never repeats or skips a number across a full 20s question, and **never shows a value above the configured limit**. Then set the OS clock back 60s (or stub `Date.now`) and confirm the clamp holds.
    4. **AC-3 (the one that must be measured, not eyeballed)** — at the moment the numeral reads 5, the stroke colour becomes `rgb(251, 191, 36)` **and** the stroke width becomes 14. Record both computed values at 6s and at 5s. Note the theme-token false-fail: Tailwind v4 tree-shakes an unreferenced `@theme` token, so read the literal computed colour, not `--color-gold`.
    5. **Reduced motion** — with the dashboard's "הפחת אנימציות" toggle on (and separately with the OS setting on): the ring is a full static circle, the numeral still counts, and the gold+thicken still fires at 5s. Confirm `stage-timer-sweep` is absent from the circle's class list.
    6. **Reconnect mid-question** — kill the Go server with a question open (`ctx.setOffline` does **not** close an established socket and passes vacuously; assert `ws.on('close')` fired). The stage stays on screen, the band appears over it, **and the countdown keeps counting correctly** — it is derived from an absolute deadline, so a dropped socket does not freeze it. **Measure what the band covers** and feed the numbers to Task 8. Restart: it re-renders with no interaction and the numeral has not jumped.
    7. **Mid-question connect** — open a *second* display window ~10s into a 20s question. Its ring must be ~half depleted, not full: this is what the negative `animation-delay` buys, and a per-render style object would look identical on the first window and wrong on this one.
    8. **`question_closed`** — press "סגור שאלה": the numeral is 0, the ring is **white, not gold**, the question and options hold, and the answered count holds. Confirm the state announcement fires.
    9. **Projection scale / fit — the arithmetic below is tight, so measure it.** At 1920×1080 expect: question 64px, options 40px, progress + count 40px at line-height 1.2, numeral 96px, ring 220px, option rows 96px, band 378px. Then test the two worst realistic cases at **both** 1920×1080 and 1280×720: (a) a one-line question with four short options, (b) a **two-line** question with four long, wrapping options. Report measured content heights and whether anything clips or a scrollbar appears. The shell is `overflow-hidden`, so a clipped bottom row is **silent**. The predicted budget at 1080p is body 702px − 16 (pt-4) − 48 (pb) = 638px of content against 88px (one-line question) + 16 (gap) + 432 (four 96px rows + three 16px gaps) = 536px — comfortable; a two-line question is 625px, which fits by 13px and is the case to actually measure. At 1280×720 the fixed 16px gaps and paddings are proportionally larger and the margin is thinner. **If it clips, the fix is a ramp value in `index.css`, not a layout rewrite — record the change and the numbers.**
    10. **Palette / filter-safety** — Festival Green plus gold **only** on the ring at ≤5s; zero requests to any external host; no `Fetch/XHR` on the display route.
    11. Zero console errors outside a deliberate outage.
  - [x] Story 3.10's, 4.1's and 4.2's manual passes may still be outstanding. If so, do them in the same session — this pass already puts you in front of those surfaces.

### Review Findings

Code review 2026-08-11. Layers run: Blind Hunter (diff-only), Edge Case Hunter (diff + project). **The Acceptance Auditor layer did not run** — it terminated on a session limit — so spec-conformance coverage here is partial: the reviewer verified the scope boundaries, the quality gates and the Task 2 regression gate directly (all hold), but no independent line-by-line audit of the ten derived requirements was performed. Findings 1–4 below were reproduced by the reviewer with a throwaway Vitest probe, not merely reported.

**Root theme.** Three of the four confirmed defects share one cause: `TimerRing` derives everything from `Date.now()` against an absolute deadline, and every path where that subtraction stops changing — the upper clamp, the lower floor, expiry — also stops the component, because `remainingMs` is the tick effect's only dependency. The clamps mask the *number* without correcting the *skew*, and the browser pass's 60s-skew check could not see it (with 60s of skew on a 20s question, a frozen "20" and a correctly clamped "20" are the same picture).

**Decisions taken by Avraham at review (2026-08-11):** the clock-skew *model* gap is deferred to a follow-up story rather than widening 4.3's frontend-only boundary; the answered-count high-water is to be **lifted above the stage** so it survives the shell's remount; and the reduced-motion ring is to show a **true static fraction** rather than a full circle. The three resolved items appear below in their new buckets.

- [x] [Review][Patch] **Lift the answered-count high-water above the stage so it survives the shell's remount** (resolved decision) — Verified: 8 → 7 across `question_open` → `question_closed`. `display-page.tsx` keys the stage `${gameId}:${state}`, so the state change unmounts `QuestionStage` and `AnsweredCount`'s `useState` re-initialises from whatever count the closing snapshot carries. That is the precise deferred-work 2.4 symptom the guard exists to prevent, on the one transition where the two snapshots can disagree. Hold the high-water keyed by `questionId` in the shell or the socket layer. The code comment at `question-stage.tsx:178-181` claims the `questionId` keying frees the guard from depending on the shell's remount; the reverse is true, and the comment must be corrected along with the fix. [web/src/features/display/question-stage.tsx:187, web/src/features/display/display-page.tsx:161]
- [x] [Review][Patch] The countdown freezes permanently when the projector clock is ≥1s behind the server — the clamped `remainingMs` is the tick effect's only dependency, so once the clamp engages the value stops changing, the effect never re-runs, and no successor timeout is ever scheduled (verified: numeral stuck at 20 for 12 simulated seconds, `getTimerCount()` = 0, while the CSS sweep empties the ring on schedule) [web/src/features/display/timer-ring.tsx:58-62]
- [x] [Review][Patch] **Give the ring an explicit `stroke-dashoffset` whenever the sweep animation is not running** — one fix, three symptoms. (a) The ring snaps back to a FULL circle the instant the countdown reaches 0 with the question still open, and stays full at `question_closed`: removing `stage-timer-sweep` removes `animation-fill-mode: forwards` with it, and no `stroke-dashoffset` fallback exists (verified: at 0, `class=null`, `stroke=var(--color-gold)`, no dashoffset attribute — a full gold ring reading "0"). (b) Under reduced motion the ring is a permanently full circle for the whole question, identical at 20s and at 1s — Avraham's decision is that it must show the true remaining fraction statically, which satisfies AC-3's "the ring is static while the numeral still counts" equally without misinforming. (c) It also removes the mid-question reduced-motion-toggle artefact below. Compute the offset from `remainingMs / (totalSeconds * 1000)` and guard `totalSeconds > 0` [web/src/features/display/timer-ring.tsx:70,115]
- [x] [Review][Patch] The ≤5s urgent announcement is never spoken when the ring mounts already urgent — the region is inserted into the DOM with its text already present, which assistive tech does not announce; this is the exact bug 4.1's review found on the shell announcer and that `use-throttled-announcement.ts` defends against, with no equivalent guard here. Reachable at `timeLimitSeconds = 5` (the DB minimum), on a display refresh inside the last 5s, and on a second display opened late (verified: region text present at first paint) [web/src/features/display/timer-ring.tsx:138-140]
- [x] [Review][Patch] The depletion sweep appears to run counterclockwise, contradicting the stated "Clockwise, the universal countdown convention" — with `stroke-dasharray: C` and dashoffset animating `0 → C`, the arc stays anchored at 12 o'clock and its far end retracts counterclockwise, so the gap grows 12 → 9 → 6 → 3. Clockwise depletion needs `0 → -C` or a `scaleX(-1)` mirror. **Confirm visually before patching** — this was reasoned from the dash mechanics, not measured, and no test or browser check covered sweep direction [web/src/index.css:288-291, web/src/features/display/timer-ring.tsx:99]
- [x] [Review][Patch] `useThrottledAnnouncement` has no re-baseline when the counter's subject changes, so a reconnect that swaps the question inside one mount (shell key `gameId:state` is unchanged if both snapshots are `question_open`) announces the new, lower count as an event — the "non-event announced as an event" the `mountedAt` ref exists to prevent. The mirror case also holds: the new question's count passing the old mount value exactly is permanently unannounceable [web/src/lib/use-throttled-announcement.ts:42,70]
- [x] [Review][Patch] Toggling reduced motion mid-question restarts the sweep from the elapsed value captured at mount — `sweep` is captured once and never recomputed, so the `false → true` transition (Organizer flips the room toggle, or the OS setting changes) returns the class with a stale `animationDelay` and the ring needs another full `totalSeconds` to deplete, finishing long after the numeral reaches 0 [web/src/features/display/timer-ring.tsx:77-86]
- [x] [Review][Patch] No test pins the negative-delay sweep arithmetic, the most intricate calculation in the story — every fixture uses `deadlineIn(20)` with `totalSeconds: 20`, so `elapsed` is always 0 and a sign error, a missing `Math.max`, or the wrong unit would all pass the suite. Browser check 7 measured it once by hand (−0.011s vs −12.008s); nothing prevents a regression [web/src/features/display/timer-ring.test.tsx]
#### Post-patch browser pass (2026-08-11)

The patches changed how the ring paints, so the story's original measurements for AC-3, reduced motion, mid-question connect and `question_closed` were re-taken in real Edge (`playwright-core` + `channel: 'msedge'`, installed into the scratchpad, never into `web/package.json`). Scratch organizer `browserpass-4-3`, single embedded binary on port 8099, `WHATSAPP_API_BASE_URL` and `ANTHROPIC_API_BASE_URL` pointed at a dead local address — exactly one `server listening`, zero `bind:` errors, zero external requests.

**The pass caught a regression the patch itself introduced, which no unit test could have seen.** The first fix expressed the sweep's direction as `to { stroke-dashoffset: calc(var(--stage-timer-circumference) * -1) }`. A `calc()` target against a plain `from: 0` **is not interpolable**, and the browser silently degrades the whole animation to discrete: measured on a 30s question, the computed offset read `0` for the first 15 seconds and then `-666.018` for the rest — the ring held completely full for half the question and vanished in one step. jsdom runs no animations, so all 37 unit tests passed against it. Fixed by moving the sign into the inline custom property (`--stage-timer-sweep-end: -666.018`) so both keyframe ends are plain numbers, and re-measured: the offset now walks `-23 → -59 → -95 → … → -631` in step with the numeral `29 → 28 → 26 → … → 2`, linear and monotonic.

| check | measured |
|---|---|
| Sweep interpolation, 30s question | 18 samples, offset linear `-23.1 → -631.3`, depletion 3.5% → 94.8%, tracking the numeral throughout |
| Sweep **direction** | Confirmed in an isolated reproduction with `isPointInStroke`: at `+50%` the gap sat at 9 o'clock (counterclockwise, the old behaviour); at `-50%` it sits at 3 o'clock, so the ring empties 12 → 3 → 6 → 9. In the live page the confirmation is the negative, monotonically decreasing computed offset — `isPointInStroke` proved **not** dash-aware there and returned "painted" at every probe, so it was not used as in-app evidence |
| AC-3 at 6s → 5s | `rgba(255, 255, 255, 0.9)` / `8px` → `rgb(251, 191, 36)` / `14px`, both channels, as before |
| **Expiry, question still open** | numeral `0`, offset `-666` (**100% depleted — empty**), sweep class absent, stroke still gold. Previously a full gold circle |
| **`question_closed`** | numeral `0`, offset `-666` (empty), `rgba(255,255,255,0.9)` at `8px`, question text and all 4 option rows held, assertive region read `השאלה נסגרה` |
| **Reduced motion, applied mid-question** | Static ring at the **true remaining fraction**, and it tracks: offset `-144.5 / -177.6 / -211.0 / -244.3` (21.7% → 36.7% depleted) while the numeral ran `47 → 44 → 41 → 38`. `data-reduced-motion="true"`, sweep class absent. Previously a full circle for the whole question |
| **Reduced motion turned back off** | Sweep resumed with `animationDelay: -25.015s` at numeral 33 of 60 — the elapsed time, not the mount-time `-0.341s`. This is the recapture patch |
| Second display opened mid-question | window 1 `-25.015s` vs window 2 `-27.543s`, offsets `-311.1` vs `-321.6` — 10.5px apart on a 666px circumference, i.e. the same arc, read one second apart |
| **Mounting already urgent** (5s limit) | numeral `4`, gold, `14px`, and the polite region reads `חמש שניות אחרונות`. Previously silent |
| Free-text question | Hint at `64.0013px / w800`, **0 option rows** — replaced, not appended |
| Layout | Band 35% of the wrapper, `rgb(22, 101, 52)`, bottom radius `12px`; option rows `96px`, `rgb(22, 101, 52)`, radius `8px`; 0 focusables, 0 box-shadows, no scrollbar |
| Hygiene | 0 external requests, 0 `fetch`/`XHR` on the display route, 0 console errors |

Not re-measured, and unchanged by the patches: the projection-scale fit table at 1280×720, the reconnect-band overlay geometry, and the Go E2E scenarios. The answered-count floor is covered by three Vitest cases rather than a browser check — reproducing a backwards frame in a browser needs the concurrent-writer burst the E2E harness built, and the harness was deleted with the story.

- [x] [Review][Defer] A client clock AHEAD of the server pins the display to 0 / gold / "last five seconds" for the whole question, while the server still accepts answers [web/src/features/display/timer-ring.tsx:49] — deferred, **reason: fixing it needs a server-clock offset (a new snapshot field), which 4.3's frontend-only scope boundary forbids; the code conforms to derived req. 6 as written and the gap is in the model, so it belongs to a follow-up story rather than to this review**
- [x] [Review][Defer] Extreme question and option text has no clamp, ellipsis or type scaling and is clipped silently [web/src/features/display/question-stage.tsx:105-107,125-127] — deferred, pre-existing pattern

## Dev Notes

### What 4.1 and 4.2 built that this story consumes — read before writing anything

- **The socket.** `useGameSocket(gameId, 'display')` handles connect, exponential backoff, indefinite retry, the `seq < lastSeq` stale guard and the StrictMode double-mount. **Do not touch `use-game-socket.ts`.**
- **Last-snapshot retention + the reconnect band.** A stage never sees `null` for `snapshot` — `StageProps.snapshot` is non-nullable, and that is the contract. (`snapshot.currentQuestion` **is** nullable; derived req. 10.)
- **The cross-fade and the stage key.** `stage-fade` + `` key={`${gameId}:${state}`} ``. This story adds no transition of its own, and it relies on that key for the `question_open → question_closed` remount.
- **The assertive state announcer.** Shell-owned, mounted in every branch. This stage's two regions are `polite` and separate.
- **`reducedMotion`.** Already the OR of the viewer's `prefers-reduced-motion` and the Organizer's room-level snapshot setting, and also mirrored as `data-reduced-motion` on `.stage-root`. **No stage ever calls `matchMedia`.**
- **The projection ramp.** `--stage-display` / `--stage-heading` / `--stage-ui` / `--stage-body` / `--stage-margin` / `--stage-lobby-code`. Use the variables; do not hardcode a px.
- **The full-bleed pattern.** 4.2 established that a stage needing its own ground paints it with `absolute inset-0` against the shell root's padding box, rather than teaching the shell about per-stage grounds. This story is the second user and the first to make it a whole layout.
- **The high-water pattern and the throttled announcer.** Both from 4.2, both re-used here — the announcer via extraction (Task 2), the guard by repetition with the reason restated (derived req. 7).

### Where the question stage's data comes from

Every field is already on the snapshot; nothing new crosses the wire.

| Rendered | Snapshot field | Origin |
|---|---|---|
| "שאלה 3 מתוך 10" | `currentQuestion.position`, `questionCount` | `games.current_question_position`; `len(questions)` in `buildSnapshot` |
| Question text | `currentQuestion.text` | `questions.text` (story 1.3) |
| Options א–ד | `currentQuestion.options` | `questions.options`, `omitempty` — **absent on free-text** |
| Timer numeral + sweep | `currentQuestion.answerCutoffAt`, `timeLimitSeconds` | `games.answer_cutoff_at` = `now() + time_limit_seconds` at open (`queries/games.sql:78,122`); `LEAST(answer_cutoff_at, now())` at close (`:91`) |
| "63 ענו" | `currentQuestion.answeredCount` | `CountAnswersByQuestion` in `buildSnapshot` |

`answerCutoffAt` is serialized as `g.AnswerCutoffAt.UTC().Format(time.RFC3339)` — **second precision, no sub-second component**. `Date.parse` handles it; do not add a parser.

**The correct answer is deliberately absent from the wire at every state, including `revealed`** (`game/snapshot.go`'s comment on `CurrentQuestion`: one payload serves both `role=host` and `role=display`). There is nothing on this stage to mark correct, and 4.4 will need a decision about how the reveal gets it — not this story's problem, but do not go looking for a field that was removed on purpose.

### The two documented behaviours of the answer path this stage must not be surprised by

1. **An accepted answer broadcasts; a rejected one does not.** `wa/inbound.go:228-236` broadcasts only on `AnswerAccepted`, and only when `result.Snapshot.GameID != ""`. A late answer, a duplicate answer, or an unparseable reply moves nothing on screen — correct, and Scenario C asserts it.
2. **A failed post-answer `buildSnapshot` skips the broadcast entirely** rather than degrading. The count simply does not move until the next answer or a reconnect. That is documented behaviour, not something for this stage to compensate for.

### The ring: why an SVG, and the conflict it resolves

Three sources describe the same object and they do not obviously agree:

- **DESIGN.md `components.timer-hero`** — 220px diameter, `border: 8px rgba(255,255,255,0.9)`, numeral Display 96px `ink-on-dark`, urgent: "at ≤5s: border {gold} and thickens to 14px". A **border**.
- **`mockups/key-stage-question.html`** — `.hero-ring` is exactly that border, rendered as a static full circle at 12s and again at 4s. A static HTML mockup cannot show an animation, so its silence on depletion is not evidence.
- **EXPERIENCE.md Component Patterns** — "Ring depletes via CSS animation", and the Accessibility Floor lists "timer ring" first among the stage animations that must have a static equivalent under reduced motion.

The tie-breaker is the epic's own AC-3: *"with `prefers-reduced-motion: reduce`, the ring is **static** while the numeral still counts"*. That clause is meaningless unless the ring moves by default. So the ring depletes; DESIGN.md's `border` is describing the ring's *appearance* (which the SVG stroke reproduces at both widths), and EXPERIENCE.md — the home of behaviour, per DESIGN.md's own Components section — supplies the motion.

Three implementation routes were considered:

- **CSS `border` + a second element for the arc.** Two rings that must stay aligned across two widths. Rejected.
- **`conic-gradient` with `@property --angle`.** Animatable, but `@property` registration is a new mechanism in this codebase for one element, and the gradient must be masked into a ring.
- **SVG `<circle>` + `stroke-dashoffset`.** Chosen. Visually identical to the border, animatable in pure CSS, and — because viewBox units are relative — the 8px/14px stroke pair scales with the ring for free, with no second `vw` calculation. The constant radius across both widths is what keeps the sweep from jumping at the 5s mark.

**The negative `animation-delay` is the whole trick.** `animation-delay: -8s` on a 20s animation starts it as if it had been running for 8 seconds. That is what makes a display opened (or reconnected) mid-question show the right arc with no JS frame loop and no per-frame state. It also means the sweep must be captured **once** — a style object rebuilt on every render restarts it, and this component re-renders on every inbound answer frame.

### Design decisions worth flagging explicitly

- **The band's `leading-[1.2]` is a fit requirement, not a style preference.** 1.38 overflows the 378px band by 0.4px under `overflow-hidden`. The mockup specifies 1.2. Both numbers are in Task 6 so a reviewer can check the arithmetic rather than trust it.
- **`ink-on-dark-muted` on the band labels is 4.4:1** — cleared by DESIGN.md's contrast table for *large text only*, which 40px is. Do not reuse this token below 40px anywhere.
- **The option letter uses `font-heading` (800) where the spec says 700.** This project's `@theme` has no 700 step. The spec's point is the *contrast* between letter and text ("same weight … the letter becomes invisible"), which 800-vs-500 preserves more strongly than 700-vs-500. Recorded rather than solved by adding a sixth weight token.
- **`strings.live.questionProgress` and `strings.live.answeredStat` are reused across surfaces.** EXPERIENCE.md gives one sentence for each and both surfaces show it. Duplicating them into `display` would create two sources for one string and is exactly the drift `strings.he.ts` exists to prevent. If Avraham prefers a shared block, it is a rename.
- **Gold's first appearance is here.** `--color-gold` has been defined and unused since story 1.1; `index.css`'s own comment says "defined here, never applied in this story". This story applies it, on a dark green ground, on a non-text element, for at most five seconds per question. Nowhere else.
- **The reconnect band will land on green-800 again.** 4.2 measured the box at ~1.4:1 against the lobby's green ground with white text at ~10:1 — a pass with a note, and the note said 4.6 would be the second data point. This stage is the second data point instead. Record what you see; do not "fix" the shell.
- **The timer keeps counting through a dropped socket, and that is correct.** The deadline is absolute and already in hand. A room watching a reconnect band still sees a truthful countdown. Worth verifying (check 6) precisely because it looks like it ought to be broken.

### Existing code this story modifies — current state, and what must survive

- **[web/src/features/display/display-page.tsx](web/src/features/display/display-page.tsx)** — two map entries, plus Task 1's type move. Everything else is a 4.1/4.2 review decision with recorded reasoning: the render-phase `retained` pattern (lint-forced), the `?? StagePlaceholder` runtime fallback (deploy skew), the always-mounted assertive announcer (a live region populated at mount is not announced), the reconnect band's `--stage-margin` offset (overscan), the optional-chained `displaySettings?.reducedMotion`, and the `${gameId}:${state}` key (4.2 proved it load-bearing against the never-evicted socket-store cache).
- **[web/src/features/display/lobby-stage.tsx](web/src/features/display/lobby-stage.tsx)** — the announcer block is replaced by the Task 2 hook, and the `StageProps` import path changes. **Nothing else**: the high-water guard, its corrected `Math.max` comment, the `aria-hidden` on the giant tokens, the four `<bdi dir="ltr">` runs, `tracking-[0.08em]` on the code and none on the number, and the full-bleed backdrop's DOM order are all 4.2 review decisions.
- **[web/src/features/display/stage-placeholder.tsx](web/src/features/display/stage-placeholder.tsx)** — import path only. It remains the permanent `draft` stage and the temporary occupant of four entries.
- **[web/src/index.css](web/src/index.css)** — two custom properties inside `.stage-root`, one rule inside `@layer components`, one `@keyframes` beside the existing one. The `@theme` tokens, the shadcn variables, `@layer base`, `.stage-fade`, and the `vw`-vs-`rem` deviation comment are untouched.
- **[web/src/lib/strings.he.ts](web/src/lib/strings.he.ts)** — one `question` sub-block inside `display`. Leave `display.lobby`'s pending `[ASSUMPTION]` strings and the `live` block alone.
- **[_bmad-output/implementation-artifacts/deferred-work.md](_bmad-output/implementation-artifacts/deferred-work.md)** — Task 8's written outcomes, appended under the existing entries. Do not rewrite the original text; the file's convention is to append rather than quietly edit.

### Testing standards

**Vitest exists** (added in 3.11, first used on the display in 4.2) — co-located `*.test.tsx`, one `test` block in `web/vite.config.ts` (`environment: 'jsdom'`, `include: ['src/**/*.test.{ts,tsx}']`), `globals` **off** so every test imports `describe`/`it`/`expect`/`vi` explicitly, and a CI step in the existing `frontend` job. `afterEach(cleanup)` must be declared per file. Expected Hebrew is read from `strings.he.ts`, never retyped.

This story is the first with frontend logic worth testing on its own terms (`deferred-work.md` predicted exactly that a story in advance): a deadline-to-seconds derivation with two clamps and a threshold. **Test the derivation and the guards, not the layout** — the layout is what the browser pass is for, and asserting Tailwind class strings in jsdom proves nothing about a projector.

Go stdlib `testing`, co-located `_test.go`, stubs grown in place — no mock framework. This story adds no Go code and therefore no Go tests; run the suite only to prove zero regression. **No real-DB unit tests**: the documented standard since 2.1.

`go test -race` may be unavailable (`CGO_ENABLED=0` in this environment, per 3.8–4.2's Change Logs). If so, say so in the Dev Agent Record rather than implying race coverage.

### Project Structure Notes

**New:**
- `web/src/features/display/question-stage.tsx` — named by the architecture (`question-stage.tsx  # question, options, timer, answer count`)
- `web/src/features/display/timer-ring.tsx` — named by the architecture (`timer-ring.tsx  # authoritative countdown, gold ≤5s`)
- `web/src/features/display/question-stage.test.tsx`
- `web/src/features/display/timer-ring.test.tsx`
- `web/src/features/display/stage-props.ts` — not in the architecture's tree; it exists to close a recorded defect, and Task 8 records why
- `web/src/lib/use-throttled-announcement.ts` — `lib`, per the Web boundary rule and the `use-single-flight.ts` / `use-space-action.ts` precedent

**Modified:**
- `web/src/features/display/display-page.tsx` (two map entries, one type move)
- `web/src/features/display/stage-placeholder.tsx` (one import line)
- `web/src/features/display/lobby-stage.tsx` (one import line, announcer → hook)
- `web/src/index.css` (+2 custom properties, +1 component rule, +1 keyframes)
- `web/src/lib/strings.he.ts` (+`display.question` block)
- `_bmad-output/implementation-artifacts/deferred-work.md` (Task 8 re-triage)

**Untouched (a diff here means you went off-spec):** all of `server/**` · `server/migrations/**` · `web/src/lib/use-game-socket.ts`, `api.ts`, `types.ts`, `text.ts`, `use-space-action.ts`, `use-single-flight.ts` · `web/src/app.tsx` · `web/src/components/**` · `web/src/features/lobby/**`, `features/live/**`, `features/builder/**`, `features/results/**`, `features/auth/**` · `web/index.html` · `web/package.json` · `web/vite.config.ts` · `.github/workflows/ci.yml` · `web/src/features/display/lobby-stage.test.tsx`.

### References

- Epic + ACs: [epics.md](_bmad-output/planning-artifacts/epics.md#L697-L714) (Story 4.3), [#L652-L654](_bmad-output/planning-artifacts/epics.md#L652-L654) (Epic 4 framing)
- Visual spec: DESIGN.md frontmatter `components.split-hero`, `components.timer-hero`, `components.stage-option`; `Colors` contrast table; `Typography` → projection scale `[A19]`, line-heights, tracking; `Layout & Spacing` → 48px safe margin, single-column options, 16px between; `Shapes`; `Elevation & Depth` → "Audience Display: flat"; `Do's and Don'ts` → white ring / gold only at ≤5s / letter-bold-text-regular / RTL letter on inline-start
- Behaviour: EXPERIENCE.md → IA `Audience Display — stages` (Question — MCQ and Free-Text rows); `Component Patterns` → Timer, Live answer count; `State Patterns` → Question open **and Question closed** rows; `Accessibility Floor` → live regions (polite + throttled counters, timer numeral excluded, assertive reserved), Motion
- Mockup: `_bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/mockups/key-stage-question.html` — the authoritative pixel resolution, including the ≤5s alternate state (spine wins on conflict)
- FR-7 (single server-side cutoff) and FR-9/FR-10: [prd.md](_bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md)
- Architecture: `Frontend Architecture` → **Timer** ("countdown driven by server-sent deadline timestamp; client renders remaining time; server cutoff is authoritative per FR-7") and "display renders exclusively from the WS snapshot"; the `features/display/` tree naming `question-stage.tsx` and `timer-ring.tsx`; `Component Boundaries (Web)`; `Structure Patterns` (co-located Vitest)
- Deferred entries this story must resolve: [deferred-work.md](_bmad-output/implementation-artifacts/deferred-work.md) — the 4.1 `StageProps` entry and its two follow-ups (`reducedMotion` half, trigger *story 4.3*), the 4.2 import-cycle entry (trigger *story 4.3*), the 4.2 heading-semantics entry, and the 4.1 reconnect-band-overlay entry (trigger: *the first stage whose content reaches the top of the content box*). Not triggered, but confirm in writing: the 2.4 out-of-order entry and the 2.5 spectator-roster entry.
- Prior stories: [4-2-lobby-stage-the-room-fills-the-screen.md](_bmad-output/implementation-artifacts/4-2-lobby-stage-the-room-fills-the-screen.md) (the full-bleed pattern, the high-water guard, the announcer this story extracts, and its measured out-of-order evidence) · [4-1-audience-display-shell-the-screen-that-follows-the-game.md](_bmad-output/implementation-artifacts/4-1-audience-display-shell-the-screen-that-follows-the-game.md) (the shell contract and its four resolved decisions)

### Latest technical information

No dependency changes and no new libraries. The versions this story writes against, as pinned in `web/package.json`: React 19.2, React Router 8.2, Tailwind CSS 4.3, TypeScript 6.0, Vite 8.1, Vitest 4.1, `@testing-library/react` 16.3, `eslint-plugin-react-hooks` 7.1.

Five version-specific details that will bite if missed:

- **Tailwind v4 arbitrary values need the `length:` hint for CSS variables in `text-*`**: `text-[length:var(--stage-display)]`, not `text-[var(--stage-display)]` (ambiguous between font-size and colour). This is the form 4.1 established and 4.2 verified in the built CSS. If one fails to resolve, fall back to `style={{ fontSize: 'var(--…)' }}` — never to a hardcoded px.
- **Utilities this story is the project's first caller of** — `size-*`, `place-items-center`, `basis-[var(--…)]`, `shrink`, `font-body`, `leading-body`, `leading-none`, `leading-[1.2]`, `rounded-b-md`, `flex-[0_0_35%]`, `bg-surface-raised`, `text-ink-on-dark-muted`, `overflow-hidden` on a flex child. **Verify each in the BUILT css rather than assuming**, exactly as 4.2 did for its four (all four generated; no fallback was needed). `leading-body` comes from `--leading-body: 1.55` and `font-body` from `--font-weight-body: 500` — the same two namespaces `leading-heading` and `font-display` already resolved from, so both are expected to generate.
- **`verbatimModuleSyntax: true`** (`tsconfig.app.json`). A type imported without the `type` keyword is emitted as a real runtime import, which is precisely the cycle Task 1 removes — and it means every `StageProps` import must stay `import type`. `noUncheckedIndexedAccess` is **off**, so `letters[index]` types as the union rather than `… | undefined`: the `?? ''` in Task 6 is a genuine runtime guard against an over-long `options` array, not a type-level formality, and no lint rule will call it redundant (the config is `tseslint.configs.recommended`, not the type-checked variant).
- **Tailwind v4 tree-shakes an unreferenced `@theme` token.** `--color-gold` has been unused since 1.1, so today it is absent from the built CSS entirely. Once the ring references it the variable appears — and in the browser pass, assert the literal `rgb(251, 191, 36)`, not the custom property (reading `--color-gold` and finding `""` is a known false-fail in this environment).
- **`eslint-plugin-react-hooks` v7** enforces `react-hooks/refs` (no ref reads during render), `react-hooks/set-state-in-effect` (no `setState` in an effect body) and the rules-of-hooks ordering that makes Task 6's early return require the hooks to live in children. All three shape the code above. Do not "clean up" any of them into the shape the rules reject.
- **CSS custom properties in an inline `style` object need a `CSSProperties` cast in TS.** `{ '--stage-timer-circumference': '666.02' }` is not assignable to `CSSProperties` without `as CSSProperties`; that is the established workaround and there is no better one in React 19's typings.

## Dev Agent Record

### Agent Model Used

claude-sonnet-5 (Claude Sonnet 5)

### Debug Log References

**Branch note — the prerequisite resolved itself.** The story's prerequisite says to branch from 4.2's commit unless 4.2 has merged, in which case branch from `main` and confirm `4bd687c` is in its history. **It had merged**: `origin/main` is exactly `4bd687c`. Work branched from there as `story/4-3-question-stage`; the tree is identical either way, so nothing in the Prerequisite section's shape verification changed. (Local `main` was stale at `527f1ed` and was left alone.)

**Quality gates.** `npm run lint` clean · `npx tsc -b --noEmit` clean · `npm test` **27 passed / 4 files** · `npm run build` clean. Backend: `go build ./...`, `go vet ./...`, `go test ./...` — all 8 packages `ok`, run only to prove zero regression. `go test -race` **remains unavailable** (`CGO_ENABLED=0`; carried since 3.8) — **no race coverage is claimed.**

**Frontend Hebrew centralization.** `grep -rlP '[\x{0590}-\x{05FF}]' --include='*.tsx' web/src` lists **exactly the two known pre-existing violations** (`features/live/control-page.tsx`, `features/builder/scoring-editor.tsx`). Neither new `.tsx` appears. *Caught during the gate, worth recording:* `question-stage.test.tsx` initially failed this — its fixture used a Hebrew question and Hebrew options. They are Organizer-authored **data**, not copy, and the file never asserts their content, so they became Latin placeholders (`QUESTION-TEXT`, `OPTION-A`…). Every expected **string** is still read from `strings.he.ts`. A comment in the file records why. (Note the environment's `grep` rejects `-P` under this locale — ripgrep was used with the same class.)

**Filter-safety.** Source and built bundle scanned: zero external hosts, zero font `@import`, zero `url(//…)`. The only absolute URLs in the bundle are XML namespace URIs (`w3.org/2000/svg` etc., never fetched) and React/React-Router error-message strings — all pre-existing.

**Built-CSS verification** (Task 9's "verify each in the BUILT css rather than assuming"). All new utilities and tokens generate: `--stage-timer-ring`, `--stage-option-min`, `.stage-timer-sweep` (`animation-name:stage-timer-deplete;animation-duration:0s;animation-timing-function:linear;animation-fill-mode:forwards`), `@keyframes stage-timer-deplete`, `place-items-center`, `leading-none`, `line-height:1.2`, `rounded-b-md`, `bg-surface-raised`, `text-ink-on-dark-muted`, `font-body`, `leading-body`, `shrink`, `flex:0 0 35%`, and `var(--stage-display)` / `var(--stage-timer-ring)` / `var(--stage-option-min)` in `text-[length:…]` / `size-[…]` / `basis-[…]`. **No fallback to inline `style` was needed.** `--color-gold:#fbbf24` **now appears in the built CSS** — Tailwind v4's scanner picks up the literal `var(--color-gold)` in `timer-ring.tsx`, so the token that had been tree-shaken since 1.1 is emitted. This matters functionally, not cosmetically: had it stayed tree-shaken, `stroke="var(--color-gold)"` would resolve to nothing and the ring would have **vanished** at ≤5s.

**Red-first demonstrations** (both guards, as 3.11 and 4.2 did):
- *Upper clamp removed* (`Math.min(…, totalSeconds*1000)` deleted) → `TimerRing > never shows a number above totalSeconds when the client clock is behind` FAILED: `TestingLibraryElementError: Unable to find an element with the text: 20`. The ring rendered **80**. Clamp restored, green.
- *High-water hold removed* (`shown = count`) → `QuestionStage > never counts the answered total backwards` FAILED: `Unable to find an element with the text: 7 ענו`. The count fell back to **4**. Guard restored, green.

**One test-harness fact worth carrying forward** (it looked like a component bug and is not). A single `act(() => vi.advanceTimersByTime(5000))` advances the ring by only **one** second. The tick is self-scheduling — the next `setTimeout` is not created until React commits — so a multi-second jump fires only the timer already pending. `tickSeconds()` in `timer-ring.test.tsx` advances one second per `act()` and documents this. Real wall-clock time does not jump between commits, and browser check 3 covers the real thing.

**Local Go E2E — `server/cmd/e2escratch`, created, run 15×, deleted.** Port **8099**, `WHATSAPP_API_BASE_URL` and `ANTHROPIC_API_BASE_URL` pointed at dead local addresses (both overrides confirmed in the log; **zero `graph.facebook.com`**, so no WhatsApp traffic left the machine). Exactly **1 `server listening`, 0 `bind:` errors** — the 3.10 "which process answered?" trap closed by construction. Unique `wamid` per message per run (4.2 lost two runs to the dedupe ledger).

*Scenario A — the deadline on the wire is usable.* `question_open` frame: `currentQuestion` non-null; `answerCutoffAt` = `2026-08-11T12:10:56Z`, parses as RFC 3339; **`cutoff − serverNow` = 19.74s against a 20s limit**; `options` = 4 for the MCQ and **absent** on the Free-Text; `answeredCount` = 0; `position`/`questionCount` = 1/2. All four assertions PASS.

*Scenario B — the answered count climbs; the interleaving needs the right window to appear.* **Report of what the runs produced, not what was expected.** 20 phones, every `answeredCount` recorded in arrival order, 20 frames delivered for 20 answers in every run (**no hub 8-slot drops observed here**, unlike 4.2):

| burst shape | runs | arrival order | frames lower than one already delivered |
|---|---|---|---|
| simultaneous (no stagger) | 4 | `[20 ×20]` every run | **0 / 20** each |
| 1500ms stagger | 3 | `[1 2 3 … 20]` every run | **0 / 20** each |
| 300ms / 150ms stagger | 1 each | `[1 2 3 … 20]` | **0 / 20** |
| **60ms stagger** | 6 | e.g. `[2 2 4 5 14 17 20 20 20 19 20 …]`, `[1 3 3 6 6 6 9 9 9 11 …]` | **1 / 20 on one run; 0 on five** |

The two extremes are clean for opposite reasons: fired simultaneously, every INSERT commits before any snapshot's count query runs, so **every frame carries the final total**; spread over 1.5s, each answer is fully processed before the next starts. The interleaving lives between them — at a 60ms stagger the repeated values (`6 6 6`, `9 9 9`) are concurrent writers stamping the same count, and **one run delivered `20 → 19`, a genuine backwards frame.** So the mechanism is confirmed present on this field, but **markedly rarer than 4.2 measured on the lobby counter** (3 of 4 runs there): **1 backwards frame across ~15 runs / ~300 frames here.** The high-water guard is retained for the reason derived req. 7 gives, and the honest summary is that it is cheap insurance against a rare-but-real delivery-order defect rather than a frequent one.

*Scenario C — `question_closed` and the cutoff.* After `close-question`: `state: "question_closed"`, `answerCutoffAt` = `12:10:39Z` against a server clock of `12:10:39Z` — **cutoff pulled back to at-or-before now (Δ −0.4s)**, confirming `LEAST(answer_cutoff_at, now())` and the fact derived req. 5 and the `closed` prop rest on. Question text and 4 options hold; `answeredCount` = 20. A further signed answer from a freshly-joined phone produced **zero frames** and the count did not move. Both assertions PASS.

**Manual browser pass — 65 checks, all green, in real Edge** (`playwright-core` + `channel: 'msedge'`, installed into the scratchpad, never into `web/package.json`; driver deleted afterwards). Seeded through the real API and real HMAC-signed webhooks. Three phases; phase 2 owns the Go server process so it can genuinely kill it.

*One setup fact that cost a cycle and is worth recording for 4.4–4.6:* **the display's WS handshake is authenticated**, so a Playwright context without the session cookie gets `401` on `/ws`, never receives a snapshot, and the shell renders its connecting/notFound branch — which looks exactly like a broken stage. `ctx.addCookies()` with the login cookie is required.

*Phase 1 — 47/47. Checks 1, 2, 3, 4, 5, 7, 8, 9(part), 10, 11.* Full-bleed wrapper measured **1920×1080 at (0,0)**. Band `rgb(22, 101, 52)` (green-800), **378px = exactly 35%**, top radius `0px` / bottom radius `12px`. Ground `rgb(255, 255, 255)`. Question is an **`<h1>`**, 64.0013px / weight 800 / `rgb(20, 83, 45)`. Four option rows, `rgb(22, 101, 52)`, radius `8px`, 40.0013px text, **96px tall**; letter weight **800** vs text **500**; in RTL the letter measured at **x=1814** against the text at **x=1679** — inline-start, with no `flex-row-reverse` and no physical margins. Band labels **40.0013px at line-height 48.0015px (=1.2)** in `rgba(255, 255, 255, 0.7)`. Ring **219.98px**, numeral **96px / 900 / line-height 96px**. Zero focusables, zero `box-shadow`, `dir="rtl"`, no scrollbar. *(The sub-pixel values are `vw` arithmetic landing on spec: 3.3334vw × 1920 = 64.00128.)*

*Check 4 / AC-3 is the one that had to be measured, and both channels were.* At **6s**: `stroke: rgba(255, 255, 255, 0.9)`, `stroke-width: 8px`. At **5s**: `stroke: rgb(251, 191, 36)`, `stroke-width: 14px`. Read as literal computed values, never via `--color-gold` (the known false-fail). A hue-only regression would fail the width assertion.

*Check 7 / the negative delay is what it claims to be.* Window 1 (mounted at question open) vs window 2 (opened ~11s in): `animationDelay` **−0.011s vs −12.008s**, same `20s` duration, and both rings sitting at the **same** `stroke-dashoffset` (417.966px vs 417.635px of a 666.018px circumference, ~63% depleted). A per-render style object would have looked correct on window 1 and wrong on window 2.

*Check 5 / reduced motion, both channels separately.* Room-level toggle: `stage-timer-sweep` **absent**, `stroke-dashoffset: 0px` (full static ring), `data-reduced-motion="true"`, and the numeral still ran **8 → 5**. **Gold and the 14px thickening still fired at ≤5s** — the accessibility cue is independent of the animation, as AC-3 requires. OS-level `prefers-reduced-motion: reduce` alone produced the identical result.

*Check 8 / `question_closed`.* Numeral **0**, ring `rgba(255,255,255,0.9)` at **8px — white, not gold**, sweep class absent, question and all 4 options held, count held, and the shell's assertive region read `השאלה נסגרה`.

*Check 2 / Free-Text.* Hint `כתבו את התשובה בוואטסאפ` at 64.0013px / 800, and **0 option rows** — replaced, not appended.

*Check 10/11.* The only gold element on the page at ≤5s was `circle.stroke`. **Zero requests to any external host, zero `fetch`/`XHR` on the display route, zero console errors.**

*Phase 2 — 17/17. Check 6 and the rest of check 9.* Fit measured at both resolutions for both worst cases:

| case | 1920×1080 | 1280×720 |
|---|---|---|
| (a) one-line question, 4 short options | h1 88px (1 line), rows 96×4, content ends 981 of 1080 | h1 59px, rows 64×4, ends 671 of 720 |
| (b) **two-line** question, 4 long wrapping options | h1 **177px (2 lines)**, rows 96×4, content ends **1025 of 1080** | h1 118px (2 lines), rows **58×4**, ends 688 of 720 |

**Nothing clipped, no scrollbar, no row-text overflow in any of the four combinations.** The 1280×720 two-line case is the interesting one: the option rows **shrank from 64px to 58px** rather than clipping — `basis-[var(--stage-option-min)]` with `shrink` and no `grow` behaving exactly as Task 6 specifies. **No ramp value needed changing.**

*Check 6 — a real outage.* The Go server was **SIGKILLed** (not `ctx.setOffline`, which does not close an established socket and would pass vacuously); `ws.on('close')` was asserted to have fired before anything else was trusted. The stage stayed on screen with all 4 rows, the band appeared over it reading `מתחבר מחדש…`, and — the part that looks like it ought to be broken — **the countdown kept counting through the outage, 295 → 292**, because the deadline is absolute and already in hand. After restart the band cleared **with no interaction**, the same question re-rendered, and the numeral did not jump (292 → 288). 6 console errors, **all during the deliberate outage** (3 WS handshake failures + 3 Vite-proxy 502s) — that is the reconnect machinery working.

*Check 3's second half — the clock-skew clamp in a real browser.* `Date.now` stubbed **60s behind** the server via `addInitScript` before any app code ran, on a 20s question: the naive subtraction yields **80**, and the display rendered **20** and held there. AC-2's "never show the total" survives an untrusted projector clock.

**Band-overlay geometry — the measurement Task 8 needed** (1920×1080, during the real outage): band **48–124px**; progress label 23–71px **covered**; ring 79–299px **top arc covered**; timer numeral 141–237px **clear by 17px**; question `<h1>` 445–533px **clear by 321px**. That is the "progress label and/or ring's top arc only" branch of the story's decision rule → recorded as a pass with a note and re-deferred, **not** escalated. Band legibility on green measured again: `rgb(20, 83, 45)` on `rgb(22, 101, 52)` (~1.4:1 box) with white text at ~10:1 — the same result 4.2 got, now reproduced on a structurally different stage.

**Cleanup.** Scratch organizer `e2escratch-4-3` deleted with all 29 of its games (cascade); **702** scratch `wa_inbound_messages` ledger rows deleted (a clean time boundary — the previous activity was the day before); `cmd/e2escratch`, the Playwright drivers and every scratch binary removed. Verified afterwards: 7 games remain (5 `avraham` + 2 from stories 3.8), 148 ledger rows, and `git status` shows **no `.go`, `.sql` or `migrations/` diff**. Pre-existing scratch organizers from stories 3.3/3.8 were left alone — not this story's to remove.

**Nothing is left running.** The scratch Go servers and the Vite dev server started for this pass were all stopped; `make dev` when you want it back.

### Completion Notes List

**All three ACs and all ten derived requirements are implemented, and every one is covered by a test, a measurement, or both.**

- **AC-1** — question text, MCQ options with א–ד, the free-text hint, and the live answered count, in the split-hero at projection scale. Verified in Vitest (content off the snapshot) and measured in a real browser (every figure above).
- **AC-2** — countdown derived client-side from `answerCutoffAt`, remaining seconds only, never the total. Clamped both ways; the upper clamp demonstrated red first and re-verified in a browser with the clock 60s behind.
- **AC-3** — gold **and** 8→14px thickening at ≤5s, numeral unchanged, static ring under reduced motion while the numeral counts. Both channels asserted in Vitest and measured in the browser; reduced motion verified from the room toggle *and* the OS setting, independently.

**Four `deferred-work.md` entries re-triaged, two confirmed not-triggered** (Task 8): the 4.1 `StageProps` entry **CLOSED in full** (its `reducedMotion` half now has two tests); the 4.2 import-cycle entry **CLOSED** by `stage-props.ts`; the 4.2 heading-semantics entry **partially closed and re-deferred** with its trigger re-pointed off the now-fired `StageProps` hook; the 4.1 reconnect-band entry **re-deferred as a pass with a note**, now with measured geometry and a note that the numeral's 17px clearance makes it tight rather than lucky. The 2.4 out-of-order and 2.5 spectator-roster entries were confirmed **not** re-triggered, in writing, with reasons.

**Four things for Avraham to confirm at review:**

1. **Two new `[ASSUMPTION]` strings** (`display.question.countAnnouncement` → `${count} ענו על השאלה`, and `urgentAnnouncement` → `חמש שניות אחרונות`). EXPERIENCE.md mandates the behaviour but gives no wording. Same confirm-at-review treatment 4.1's and 4.2's copy got.
2. **`strings.live.questionProgress` and `strings.live.answeredStat` are imported by the display**, not duplicated into `display.question` — EXPERIENCE.md gives one sentence for each and both surfaces show it, so two copies would be a drift risk. Flagged as Task 4 asks: say if you would rather they were duplicated, or promoted to a shared block (it is a rename either way).
3. **The option letter uses `font-heading` (800) where DESIGN.md says 700.** This project's `@theme` has no 700 step. The spec's point is the letter-vs-text *contrast* ("same weight … the letter becomes invisible"), which 800-vs-500 preserves more strongly than 700-vs-500. Recorded rather than solved by inventing a sixth weight token.
4. **The white ground sits on the full-bleed wrapper, with the green band painted over its top 35%**, rather than on the body div (which measures transparent). Same rendered result with one element fewer, and `rounded-b-md` on the band reveals the wrapper's white behind the band's bottom corners — which is the point of the radius. Measured `rgb(255, 255, 255)`.

**Scope held.** Frontend only — **zero `.go`, `.sql` or `migrations/` diff**, verified after cleanup. No migration, no `sqlc generate`, no new npm package, no `shadcn add`, no `components/ui/*` edit. Exactly two `stageByState` entries changed; `draft`/`revealed`/`leaderboard`/`finished` keep `StagePlaceholder`. Gold appears only on the ring at ≤5s and nowhere else. Single-column options; no distribution bars, no correct-answer marking. `use-game-socket.ts`, `ws/**`, `wa/**` and the dashboard control panel untouched.

**`lobby-stage.test.tsx` was not edited** — `git diff 4bd687c -- web/src/features/display/lobby-stage.test.tsx` is **empty**, and all 5 of its cases (including the 4998ms negative boundary and the unmount cleanup case) pass against the extracted hook. That is the whole proof that Task 2's refactor changed no behaviour.

### File List

**New**
- `web/src/features/display/stage-props.ts`
- `web/src/features/display/timer-ring.tsx`
- `web/src/features/display/timer-ring.test.tsx`
- `web/src/features/display/question-stage.tsx`
- `web/src/features/display/question-stage.test.tsx`
- `web/src/lib/use-throttled-announcement.ts`
- `web/src/features/display/display-page.test.tsx` — added at code review, to cover the shell-level answered-count floor

**Modified**
- `web/src/features/display/display-page.tsx`
- `web/src/features/display/lobby-stage.tsx`
- `web/src/features/display/stage-placeholder.tsx`
- `web/src/index.css`
- `web/src/lib/strings.he.ts`
- `_bmad-output/implementation-artifacts/deferred-work.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `_bmad-output/implementation-artifacts/4-3-question-stage-with-the-authoritative-timer.md`

**Created and deleted within the story** (no diff remains)
- `server/cmd/e2escratch/main.go` — the Go E2E harness

## Change Log

| Date | Change |
|---|---|
| 2026-08-11 | Story 4.3 created — question stage, split-hero, and the authoritative deadline-driven timer ring. Baseline set to `4bd687c` (story 4.2, committed and `done`, not yet merged to `main`). |
| 2026-08-11 | Implemented on branch `story/4-3-question-stage`. 4.2 had merged by start time, so the branch is off `origin/main` (= `4bd687c`), per the story's own prerequisite rule. |
| 2026-08-11 | Tasks 1–2: `StageProps` moved to `stage-props.ts` (import cycle closed, three importers rewired, stale "runtime cycle" comments deleted); 4.2's throttled announcer extracted to `lib/use-throttled-announcement.ts` with `lobby-stage.test.tsx` green and **unedited**. |
| 2026-08-11 | Tasks 3–7: two ramp values + the sweep rule and keyframes in `index.css`; `display.question` copy block; `timer-ring.tsx` (absolute-deadline countdown, both clamps, second-boundary tick, gold+thicken at ≤5s, once-per-mount sweep capture); `question-stage.tsx` (full-bleed split-hero, degraded-snapshot guard, high-water answered count, `<h1>` question, free-text branch); both `stageByState` entries pointed at it. |
| 2026-08-11 | 17 new Vitest cases (10 ring, 9 stage — 27 total in the suite). Both guards demonstrated **red first**: the upper clamp rendered 80 instead of 20, and the count fell back to 4 instead of holding 7. |
| 2026-08-11 | Task 8: four `deferred-work.md` entries re-triaged (two closed, two re-deferred with re-pointed triggers) and two confirmed not-triggered in writing. |
| 2026-08-11 | Task 9: all gates green (lint, tsc, 27 tests, build, Hebrew-centralization grep, filter-safety, Go build/vet/test). Go E2E scenarios A/B/C across 15 runs; 65-check browser pass in real Edge across three phases, including a real SIGKILL outage and a browser-side clock-skew clamp check. Scratch data and harness removed; no `.go`/`.sql`/`migrations/` diff. |
| 2026-08-11 | **Post-patch browser pass.** Re-measured every ring behaviour the patches touched, in real Edge against the embedded production bundle. Caught and fixed a regression the direction patch had introduced: a `calc()` keyframe target is not interpolable against `from: 0`, so the sweep silently degraded to a single discrete jump at the halfway mark — invisible to all 37 unit tests, because jsdom runs no animations. The sign moved into the inline custom property so both keyframe ends are plain numbers. Full measurement table in the Review Findings section. |
| 2026-08-11 | **Code review.** Blind Hunter + Edge Case Hunter ran; the Acceptance Auditor layer failed on a session limit, so spec conformance was spot-checked by the reviewer instead of independently audited. Eight patches applied, three decisions taken, two items deferred, five dismissed as unreachable. The four confirmed defects were all reproduced with a throwaway Vitest probe before being reported: the countdown froze permanently on the total whenever the projector clock ran ≥1s behind the server (the clamped remainder was the tick effect's only dependency); the ring snapped back to a full gold circle at 0 and held full through `question_closed` and under reduced motion (removing the sweep class removes `animation-fill-mode` with it); the ≤5s announcement was never spoken when the ring mounted already urgent; and the answered count fell 8 → 7 across the `question_open` → `question_closed` remount, because the shell's key discards the stage's own high-water at exactly that moment. The sweep direction was additionally measured in Edge with `isPointInStroke` and found to deplete **counterclockwise**, contradicting the code's own stated convention — fixed by negating the keyframes' target offset. Gates re-run: lint, tsc, build clean; **37 tests pass, up from 27**; `lobby-stage.test.tsx` still unedited; still zero `.go`/`.sql`/`migrations/` diff. |
