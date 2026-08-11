---
baseline_commit: a4523ea6c65d0e598efcf459a18187d769e79415
---

# Story 4.2: Lobby Stage — the Room Fills the Screen

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As the room,
we want the projector to show how to join and the counter climbing,
so that joining becomes part of the show (FR-10 lobby, UJ-1).

## ⚠️ Prerequisite: branch from 4.1's work, not from `main`

> **RESOLVED — this whole section was obsolete before development started (recorded at code review, 2026-08-11).** By the time 4.2 ran, both 4.1 (`4b55278`, PR #9) and 3.11 (`06b16ab`, PR #10) had merged; `main` was at `8f04cc5`, which is the real base this story branched from. The `baseline_commit` in the frontmatter (`a4523ea`, story 3.10) is stale and was deliberately left untouched per `bmad-dev-story`'s preserve rule. Read the rest of this section as history, not as instruction — but the list of shapes to verify on disk below was still accurate and was confirmed before any code was written.

At the time this story was created, **story 4.1 (Audience Display Shell) is implemented, reviewed, and marked `done` — but NOT committed.** Its work sits as uncommitted changes on branch `story/4-1-audience-display-shell`, and `main`/`HEAD` is at `a4523ea` (story 3.10). The `baseline_commit` above is that tip because 4.1 has no commit yet; it is **not** the tree you should branch from.

**Branch from wherever 4.1's work actually sits** when you start — its commit on `story/4-1-audience-display-shell`, or `main` once 4.1 has merged. This story is meaningless without 4.1: every file it touches is either created or modified by 4.1.

Verify these shapes on disk before writing any code (they are 4.1's landed state):

- `web/src/features/display/display-page.tsx` — exports `interface StageProps { snapshot: LobbySnapshot; reducedMotion: boolean }`; `stageByState` is a `Record<GameState, ComponentType<StageProps>>` with all seven entries pointing at `StagePlaceholder`; the stage wrapper is `<div key={rendered.state} className="stage-fade …">`.
- `web/src/features/display/stage-placeholder.tsx` — `export const StagePlaceholder: ComponentType<StageProps>`.
- `web/src/index.css` — a `.stage-root` block (unlayered) defining `--stage-display: 5vw`, `--stage-heading: 3.3334vw`, `--stage-ui: 2.5vw`, `--stage-body: 2.0834vw`, `--stage-margin: 2.5vw`, followed by an `@layer components` block holding `.stage-fade`.
- `web/src/lib/strings.he.ts` — a `display` block with `connecting`, `reconnecting`, `notFound`, `waiting`, `stateAnnouncement`.
- `web/src/lib/types.ts` — `LobbySnapshot` carries `joinCode`, `platformNumber`, `participantCount`, `displaySettings`.
- `server/internal/game/snapshot.go` — `Snapshot` already carries `JoinCode`, `PlatformNumber`, `ParticipantCount`. **This story needs no new snapshot field.**

If 4.1 has since merged or changed further, re-confirm the quoted shapes before relying on them.

## Acceptance Criteria

1. **Given** the Game is in `lobby`, **then** the display shows the JOIN instructions ("שלחו JOIN <code> למספר <number>") and a live participant counter, per DESIGN.md's `stage-lobby` spec. *(epic AC-1)*
2. **Given** a Participant joins, **then** the counter increments within 3 seconds (FR-1 consequence). *(epic AC-2)*
3. **Given** the rendered lobby stage, **then** typography lands at projection scale per A19: JOIN Code + phone number at Display 900 (**120px+ at 1080p**, digit-grouped, bidi-isolated), instruction at Heading 800 (64px), counter in the oversized pill. *(epic AC-2, second half)*

### Derived requirements — binding, and each has a source

These are not extra scope; they are what "per the stage-lobby spec" resolves to. Each is cited so you can check it yourself.

4. **The lobby stage paints its own dark ground.** DESIGN.md `components.stage-lobby` specifies the counter as an "oversized pill: green-50 background, green-800 tabular numeral" — a `green-50` pill is invisible on the shell's `surface-base` ground (they are the same colour, `#F0FDF4`). `mockups/key-stage-lobby.html` puts the whole stage on `green-800`. The stage must therefore paint `bg-green-800` **full-bleed, behind the shell's `--stage-margin` padding**, with `ink-on-dark` text (7.1:1, DESIGN.md contrast table).
5. **The counter announces politely and throttled.** EXPERIENCE.md Accessibility Floor: "counters (`aria-live="polite"`) are throttled — announce at most every 5s or at milestones". This is the project's first counter. The shell's `aria-live="assertive"` region stays reserved for state transitions and is not touched.
6. **Bidi isolation on every LTR run.** DESIGN.md Typography: "Bidi isolation is mandatory wherever LTR tokens … sit inside RTL Hebrew: `<bdi>`/`dir` spans on web surfaces". That is ~~three runs~~ **four `<bdi>` elements** *(corrected at code review, 2026-08-11 — the original prose undercounted; Task 3's own snippet, `mockups/key-stage-lobby.html` and the shipped code all carry four)*: `JOIN <code>` inside the instruction sentence, **the phone number inside the instruction sentence**, the standalone code, and the standalone phone number.
7. **Reduced motion is a no-op here, deliberately.** The lobby stage has no animation of its own — the counter changes value, it does not move. `StageProps.reducedMotion` is therefore unread by this stage, and that is correct, not an omission. **Do not invent an animation in order to have something to disable.**

### Scope boundaries for this story

- **Frontend only. Zero `.go` files change.** The snapshot already carries `joinCode`, `platformNumber` and `participantCount`; the hub already fans broadcasts to `role=display`; a JOIN already triggers a broadcast (`wa/inbound.go:236`). A `.go` diff in this story means you went off-spec. The one exception is a throwaway `cmd/e2escratch` harness, deleted after use.
- **No migration, no `sqlc generate`, no `queries/*.sql` change.**
- **One entry in `stageByState` changes: `lobby`.** `draft`, `question_open`, `question_closed`, `revealed`, `leaderboard`, `finished` all keep `StagePlaceholder` — they belong to 4.3–4.6. Do not "while we're here" any of them.
- **No gold anywhere.** Gold's two permitted moments are the ≤5s timer (4.3) and the winner (4.6). `mockups/key-stage-lobby.html` says it outright: "אין זהב — זה לא רגע שיא". A `text-gold`/`bg-gold`/`border-gold` in this diff is wrong (UX-DR2).
- **No change to `web/src/lib/use-game-socket.ts`, `server/internal/ws/**`, `server/internal/wa/**`.**
- **No change to the dashboard's lobby page or live control panel.** `lobby-page.tsx` keeps its own text counter; this story does not unify them.
- **No new npm package, no `shadcn add`, no `components/ui/*` edit.**
- **Do not add a frontend test framework** without an explicit decision — see Task 5.

## Tasks / Subtasks

- [x] **Task 1: `web/src/index.css` — one ramp step** (AC: 3)

  DESIGN.md `components.stage-lobby` puts the code at "120px+" while A19's Display-900 figure (96px) is the *timer numeral*. They are different steps and the difference is deliberate — DESIGN.md Typography: the phone number "is the hardest visual task in the product and gets maximum scale". `mockups/key-stage-lobby.html` resolves it at `9cqw` (172.8px at 1920).

  - [x] Append **inside the existing `.stage-root` block**, after `--stage-margin`:
    ```css
      /* DESIGN.md components.stage-lobby: the JOIN Code and the phone
         number sit ABOVE the display step - the spec says "120px+" while
         A19's 96px Display figure is the timer numeral. Transcribing the
         number is the product's hardest visual task (DESIGN.md
         Typography), so it gets maximum scale. 9vw = 172.8px at 1920,
         matching mockups/key-stage-lobby.html's 9cqw. */
      --stage-lobby-code: 9vw;
    ```
  - [x] **Nothing else in `index.css` changes.** Not the `@theme` blocks, not `@layer base`, not the `@layer components` `.stage-fade` rules, not the `@keyframes`, not the existing five ramp values, and not the long deviation comment above `.stage-root` (it records Avraham's 4.1 decision and is still accurate — `vw`, not `rem`).
  - [x] **No new `@theme` colour token.** `green-800`, `green-50`, `ink-on-dark`, `text-primary` all already exist.
  - [x] The ramp block stays unlayered (4.1's review decided this: they are declarations on a class nothing else defines). Do not move it into a layer.

- [x] **Task 2: `web/src/lib/strings.he.ts` — the lobby copy** (AC: 1, 5)

  All Hebrew for this story lives here. Append a `lobby` sub-block **inside the existing `display` block**, after `stateAnnouncement`:

  ```ts
    // Lobby stage (story 4.2). The instruction is split into two fragments
    // rather than one interpolated sentence because the two LTR runs must
    // be wrapped in <bdi> (DESIGN.md Typography: bidi isolation is
    // mandatory on web surfaces) and a plain string cannot carry markup.
    // The stage composes them as:
    //   {instructionPrefix} <bdi>JOIN {code}</bdi> {instructionTo} <bdi>{number}</bdi>
    // reproducing EXPERIENCE.md Flow 1 step 3 and mockups/key-stage-lobby.html.
    lobby: {
      instructionPrefix: 'שלחו',
      instructionTo: 'למספר',
      // [ASSUMPTION]: mockups/key-stage-lobby.html renders "57 הצטרפו".
      // Plural at every count, including 1: Hebrew singular past tense is
      // gendered ("הצטרף") and A2 forbids gendered address, so the mildly
      // imperfect "1 הצטרפו" is the gender-safe choice. Same class as
      // deferred-work.md's 1.5 "Hebrew dual-form count" entry.
      joinedLabel: 'הצטרפו',
      // Polite, throttled announcement (EXPERIENCE.md Accessibility
      // Floor). A full sentence, unlike the visual pill, because a screen
      // reader gets no layout to carry the meaning.
      countAnnouncement: (count: number) => `${count} הצטרפו למשחק`,
    },
  ```

  - [x] `JOIN` is **not** a string in this file. It is the one product-defined Latin exception (DESIGN.md Brand & Style) and is composed in the stage as part of the `<bdi>` run, exactly as `mockups/key-stage-lobby.html` does.
  - [x] **Nothing else in `strings.he.ts` changes.** In particular do not touch the `live` block's display-control keys or the `display` block's `connecting`/`reconnecting`/`notFound`/`waiting`/`stateAnnouncement` — all four were confirmed by Avraham at 4.1's review.
  - [x] Flag the two `[ASSUMPTION]` items (`joinedLabel`, `countAnnouncement`) for Avraham in the Dev Agent Record so they get the same confirm-at-review treatment 4.1's copy got.

- [x] **Task 3: `web/src/features/display/lobby-stage.tsx` (new) — the stage** (AC: 1, 2, 3, 4, 5, 6, 7)

  The architecture names this exact file: `features/display/lobby-stage.tsx — JOIN instructions + climbing counter`.

  - [x] **The full-bleed dark ground** (AC-4). The shell root is `position: relative` with `padding: var(--stage-margin)`, so an absolutely-positioned child resolves against the **padding box** — i.e. it covers the safe margin too. That is exactly what the mockup wants (green is full-bleed; the 48px margin lives *inside* it). Two siblings, no `z-index`:
    ```tsx
    <div aria-hidden className="absolute inset-0 bg-green-800" />
    <div className="relative flex flex-col items-center gap-12 text-center text-ink-on-dark">…</div>
    ```
    **Both must be positioned and in this DOM order.** Painting order puts positioned elements above non-positioned in-flow content, so a positioned backdrop with a *static* content sibling would cover the text. Do **not** reach for `-z-10` instead: the shell root establishes no stacking context (`overflow-hidden` does not), so a negative-z child would slide behind the root's own background and vanish.
  - [x] **The instruction line** (AC-1, 3, 6) — Heading 800 at `--stage-heading` (64px at 1920), `leading-heading`:
    ```tsx
    <p className="text-[length:var(--stage-heading)] font-heading leading-heading">
      {strings.display.lobby.instructionPrefix}{' '}
      <bdi dir="ltr">JOIN {snapshot.joinCode}</bdi>{' '}
      {strings.display.lobby.instructionTo}{' '}
      <bdi dir="ltr" className="tabular-nums">{snapshot.platformNumber}</bdi>
    </p>
    ```
    `JOIN` and the code are **one** `<bdi>` run, as in the mockup — splitting them lets the bidi algorithm reorder the pair.
  - [x] **The two giant tokens** (AC-3, 6) — Display 900 at `--stage-lobby-code`, grouped in their own tighter sub-stack so the outer 48px rhythm does not separate the pair:
    ```tsx
    <div className="flex flex-col items-center gap-2">
      <p className="text-[length:var(--stage-lobby-code)] font-display leading-[1.1] tracking-[0.08em]">
        <bdi dir="ltr">{snapshot.joinCode}</bdi>
      </p>
      <p className="text-[length:var(--stage-lobby-code)] font-display leading-[1.1] tabular-nums">
        <bdi dir="ltr">{snapshot.platformNumber}</bdi>
      </p>
    </div>
    ```
    - [x] **`tracking-[0.08em]` on the code, and nothing on the number.** DESIGN.md Typography: "Uppercase labels (for any English-language interface elements) receive +0.08em tracking" — the mockup applies it to `.lobby-code` and explicitly zeroes it on `.lobby-number` ("digits carry no uppercase tracking"). Getting this backwards is a visible defect.
    - [x] **`tabular-nums` on the phone number, both places it appears.** DESIGN.md `stage-lobby.phone`: "digit-grouped, tabular-nums".
    - [x] **`leading-[1.1]`, not `leading-heading`.** At 172.8px, a 1.38 line-height burns ~66px of vertical space per line for no legibility gain and risks clipping at the 1280×720 minimum under the shell's `overflow-hidden`. The mockup uses 1.1. This is the one place the story leaves the two documented line-heights.
    - [x] **Do not reformat the phone number.** `platformNumber` comes from `WHATSAPP_DISPLAY_NUMBER`, which is already the human-readable, digit-grouped Graph API `display_phone_number` (e.g. `+972 50-000-0000`). Client-side regrouping would be a guess about an international format the config already settled. "Digit-grouped" in the AC is satisfied by the configured value plus `tabular-nums`.
  - [x] **The counter pill** (AC-1, 3):
    ```tsx
    <p className="rounded-full bg-green-50 px-12 py-4 text-[length:var(--stage-ui)] font-ui text-green-800 tabular-nums">
      <span className="text-[length:var(--stage-heading)] font-heading">{count}</span>{' '}
      {strings.display.lobby.joinedLabel}
    </p>
    ```
    - [x] `bg-green-50` + `text-green-800` is DESIGN.md `components.stage-lobby.counter` verbatim. `green-800` on `green-50` is ~6.9:1.
    - [x] **`rounded-full`, not `rounded-pill`.** `--radius-pill: 9999px` is defined in `@theme` but no component in the project has ever used the generated utility; every existing pill (`features/live/response-stats.tsx:16`, `features/builder/games-list-page.tsx:83` — both `rounded-full bg-green-50 … text-green-800`, the same token trio) uses `rounded-full`. Match the precedent rather than being the first caller of an untested utility name.
    - [x] The numeral steps up to `--stage-heading` (64px) rather than the mockup's off-ramp 60px — an existing step, still comfortably over A19's 48px floor for display-only content, and it avoids inventing a sixth size.
    - [x] **Padding stays on the px spacing scale** (`px-12` = 48px, `py-4` = 16px), matching the mockup's proportions at 1920. Only the *type* ramp is viewport-relative — that was 4.1's decision and it is not extended to spacing here. Use only the on-scale stops (1/2/3/4/6/8/12/16 ↔ 4/8/12/16/24/32/48/64px). Off-scale stops like `gap-5` still *compile* — `--spacing: initial` was considered and rejected in story 1.1 because it breaks shadcn component defaults — so this is enforced by convention and review, not by the build (`deferred-work.md`, 1.1 entry).
  - [x] **The monotonic count** (AC-2, and the deferred-entry resolution — see Task 5):
    ```tsx
    // participantCount can arrive out of order: two concurrent joins each
    // do INSERT -> buildSnapshot -> Broadcast independently, and the hub
    // stamps `seq` at Broadcast()-call time, so the older (lower) count
    // can win the higher seq and the client's `seq < lastSeq` guard will
    // accept it (deferred-work.md, 2.4 entry — this story is its named
    // trigger). Holding the maximum is sound HERE and only here: in lobby
    // the count is monotonically non-decreasing by construction - nothing
    // deletes a participant row, and a spectator can only be created past
    // lobby. It is a symptom-level guard on the one surface a whole room
    // watches; the general seq-ordering fix stays deferred (see Task 5).
    const [highWater, setHighWater] = useState(snapshot.participantCount)
    if (snapshot.participantCount > highWater) setHighWater(snapshot.participantCount)
    const count = Math.max(highWater, snapshot.participantCount)
    ```
    - [x] State adjusted **during render** (React's documented "storing information from previous renders" pattern), not a ref and not an effect — `display-page.tsx` uses the identical shape and documents why: this project's lint rules reject reading a ref during render (`react-hooks/refs`) and calling `setState` from an effect body (`react-hooks/set-state-in-effect`).
    - [x] `Math.max` on the rendered value as well as the setState, so the render that triggers the update is already correct.
    - [x] This state resets when the game changes **only because of Task 4's key**. Do not add a second `gameId` guard here.
  - [x] **The throttled announcer** (AC-5):
    ```tsx
    const announceIntervalMs = 5000
    …
    // Starts null, i.e. the region mounts EMPTY. Assistive tech does not
    // announce content that is already present when a live region first
    // appears - 4.1's code review found exactly this bug on the shell's
    // state announcer. Seeding this with `count` would silently swallow
    // the first announcement; starting empty means the first interval
    // tick is a real change and is read out.
    const [announced, setAnnounced] = useState<number | null>(null)
    // Latest-ref via effect (the same pattern display-controls.tsx uses):
    // writing a ref in an effect body is allowed, reading one during
    // render is not.
    const countRef = useRef(count)
    useEffect(() => {
      countRef.current = count
    })
    // setState inside an interval callback is async, so it does not trip
    // react-hooks/set-state-in-effect. Empty deps deliberately: a dep on
    // `count` would restart the timer on every join and, in a fast-filling
    // lobby, it would never fire at all.
    useEffect(() => {
      const id = setInterval(() => setAnnounced(countRef.current), announceIntervalMs)
      return () => clearInterval(id)
    }, [])
    ```
    and render it as a third sibling of the fragment, outside the visual stack (`sr-only` is `position: absolute`, so it takes no layout space in the shell's flex wrapper):
    ```tsx
    <p aria-live="polite" className="sr-only">
      {announced === null ? '' : strings.display.lobby.countAnnouncement(announced)}
    </p>
    ```
    - [x] `polite`, never `assertive` — UX-DR14 reserves assertive for game-state transitions, which the **shell** owns. Two assertive regions on one page would fight.
    - [x] Re-setting the same value is a React bail-out, so a quiet lobby announces nothing. That is the desired behaviour, not a bug to work around.
    - [x] The visible pill updates immediately; only the announcement is throttled. Do not throttle the visual number.
  - [x] **Signature and typing**: `export function LobbyStage({ snapshot }: StageProps)`. A plain function component, unlike `StagePlaceholder`'s `ComponentType<StageProps>` const — that form exists only because the placeholder reads *no* props and an unused parameter is a lint error here. `reducedMotion` stays destructured-out (AC-7).
  - [x] **`import type { StageProps } from './display-page'`** — type-only, exactly as `stage-placeholder.tsx` does. `display-page.tsx` imports this module, so a value import of `StageProps` would be a runtime cycle.
  - [x] **Zero Hebrew literals in this file.** Every string comes from `strings.display.lobby`. Comments in English.
  - [x] **Nothing focusable** (the display is output-only): no `<button>`, `<a>`, `<input>`, `tabIndex`, or `onClick`.
  - [x] **No `shadow-*` class** — DESIGN.md: "Audience Display: flat."

- [x] **Task 4: `web/src/features/display/display-page.tsx` — two lines** (AC: 1)

  - [x] Import `LobbyStage` and point the one map entry at it:
    ```tsx
    lobby: LobbyStage, // story 4.2 — lobby stage
    ```
    The six other entries keep `StagePlaceholder` and their `// story 4.x` comments unchanged.
  - [x] Widen the stage wrapper's key to include the game:
    ```tsx
    key={`${rendered.gameId}:${rendered.state}`}
    ```
    This story is the first to give a stage local state (the high-water count, the announced value), and `DisplayPage` does **not** remount when the URL's `gameId` changes — the same trap 4.1's code review found and fixed for `retained`/`lastSnapshot`. Without this, navigating `/display/A` → `/display/B` carries game A's counter into game B. Fixing it in the shell fixes it once for 4.3–4.6 too.
  - [x] **Nothing else in this file changes.** The `Record<GameState, …>` annotation and its `?? StagePlaceholder` runtime fallback, the render-phase `retained` pattern, the `usePrefersReducedMotion` hook, the OR that produces `reducedMotion`, the optional-chained `displaySettings?.reducedMotion`, the always-mounted `aria-live="assertive"` announcer, the three overlay branches, and the reconnect band's `top-[var(--stage-margin)]` offset are all 4.1 review decisions with recorded reasoning.

- [x] **Task 5: re-triage the two `deferred-work.md` entries this story triggers** (no product code)

  Both name story 4.2 explicitly. Each needs a written outcome appended under the entry, in the same style the file already uses. **Neither is to be implemented as a general fix in this story.**

  - [x] **"Concurrent joins can broadcast lobby snapshots out of order"** (2.4 entry, `deferred-work.md:52-53`; its 4.1 note reads *"NOT triggered — but 4.2 will trigger it … must resolve this entry before it ships"*). **Outcome: mitigated on this surface, general fix re-deferred.** Record:
    - What this story does: the lobby stage holds a high-water participant count (Task 3), so the projector can never *regress* to a stale lower number. Record why that is sound rather than a lie — in `lobby` the count is monotonically non-decreasing by construction (no delete path; spectators are created only past lobby) — and record its two limits explicitly: it does not help the dashboard's own lobby counter, and it would be **wrong** to copy into any later stage where a snapshot field can legitimately decrease.
    - What it does not do: the underlying `seq`-at-`Broadcast()`-time ordering defect is untouched, and it is the same defect as the 3.1 entry (`deferred-work.md:68-70`), whose fix is a `ws`/`control.go` redesign.
    - **New trigger**, replacing "story 4.2": *the first snapshot field rendered on the display that can legitimately decrease* (4.3's answered count cannot; 4.5's leaderboard positions can), *or the general `seq` fix landing*. Cross-reference the 3.1 entry so the two cannot drift.
    - Report whatever the Task 6 E2E actually observed about interleaving — measured evidence, not a guess.
  - [x] **"Nothing exercises the `StageProps` wiring that five later stories depend on"** (4.1 entry, `deferred-work.md:145`; trigger: *"story 4.2 … the natural place to decide whether this contract warrants the project's first frontend test"*). **Outcome: partially closed by observation; the framework decision is Avraham's, and the recommendation is to defer to 4.3.** Record:
    - Partially closed: `LobbyStage` is the first stage that actually *reads* `snapshot`, so passing the nullable snapshot or the wrong object is now immediately visible in a browser rather than invisible. The `reducedMotion` half of the wiring is **still** unexercised — this stage has no motion (AC-7) — and stays so until 4.3's timer ring.
    - Recommendation: **do not** add Vitest here. `LobbyStage` is pure presentation with no logic a unit test could assert that the browser pass does not; 4.3's countdown (deriving remaining seconds from an absolute deadline, plus the ≤5s threshold) is the first genuinely unit-testable frontend logic in the project and is the honest place to spend a framework's setup cost. The architecture does anticipate Vitest (`Structure Patterns`: "TS tests co-located … (Vitest)"), so this is a *when*, not a *whether*.
    - **New trigger: story 4.3.** Surface the recommendation to Avraham at code review rather than deciding silently — adding the project's first test framework is not a stage story's call.
  - [x] Confirm in writing that the **2.5 spectator-roster entry** (`deferred-work.md:61-62`) is still **not** triggered — 4.1 already reasoned that 4.2's counter is lobby-only, where a spectator row cannot exist by definition. State that it was checked, so 4.3 does not have to re-derive it.
  - [x] ~~Do not re-triage the seven other Epic-4-adjacent entries; 4.1's review already dispositioned them and re-pointed their triggers off epic numbers.~~ **False premise, corrected at code review (2026-08-11).** At least one of them — the reconnect-band overlay entry — still carried `Revisit trigger: **story 4.2**` in as many words. The developer re-triaged it anyway (measured 66px of clearance, re-pointed the trigger off a story number) and said why. That was the right call: leaving a fired trigger bearing this story's name in the file would have forced a later reader to re-derive it.

  > **Task 5's stated outcome for the `StageProps` entry was inverted, deliberately and in the open (recorded at code review, 2026-08-11).** The boxes below are checked because the *work* was done, not because the recommendation was followed: this task recommended **"do not add Vitest here"** and "defer to 4.3", and the story did the opposite. Both halves of the recommendation's premise had expired — 3.11 had already landed Vitest (so no framework decision remained to make, and the scope boundary, which forbids adding a *framework*, was never engaged), and "pure presentation with no logic a unit test could assert" was simply wrong, since the high-water guard is logic and is the mitigation for a measured defect. The reversal is disclosed in the Dev Agent Record, the Change Log and `deferred-work.md`.

- [x] **Task 6: quality gates, E2E, and the manual browser pass** (all ACs)

  - [x] **Frontend Hebrew centralization**: `grep -rlP '[\x{0590}-\x{05FF}]' --include='*.tsx' web/src` must list only the two known pre-existing violations (`features/builder/scoring-editor.tsx`, `features/live/control-page.tsx` — both comments, both on `origin/main`, both recorded in `deferred-work.md`). **`lobby-stage.tsx` must not appear.**
  - [x] **Frontend gates**: `npm run lint` · `npx tsc -b --noEmit` · `npm run build` · both filter-safety scans (source and built bundle — no external URL, no `@import` of a font, no `url(//…)`).
  - [x] **Backend, unchanged**: run `go build ./... && go test ./...` once to prove zero regression, then confirm `git status` shows **no `.go`, no `.sql`, no `migrations/` diff**. A diff there means Task boundaries were crossed.
  - [x] **Local Go E2E** (`cmd/e2escratch`, deleted after use — the 2.1–4.1 convention, including the fake-provider `WHATSAPP_API_BASE_URL` override so no real WhatsApp traffic leaves the machine). Two established patterns to reuse verbatim rather than reinvent:
    - **WS client** — 4.1's: `websocket.Dial` from `github.com/coder/websocket` (already a direct dependency), session cookie on the dial request's header, since the handshake authenticates *before* upgrading.
    - **Inbound JOIN** — 2.4's: a `POST` to the webhook endpoint with an `X-Hub-Signature-256: sha256=<hmac>` header over the **raw body**, computed with `WHATSAPP_APP_SECRET`. Copy `webhook_test.go`'s `sign()` helper; an unsigned or mis-signed body is rejected before any game code runs, and the resulting silence looks exactly like "the counter doesn't work". Each message needs a **unique `wamid`** or the dedupe ledger drops it as a Meta retry.

    Two things only a real server proves:
    - **Scenario A — the counter really climbs on a `role=display` socket.** Log in, create a scratch game, `open-lobby`. Open a `role=display` socket and read the initial frame: `participantCount` is 0 and `joinCode`/`platformNumber` are non-empty and match the game/config. POST one signed `JOIN <code>` webhook from a fresh phone → the **already-open display socket** receives a frame with `participantCount: 1` **within 3 seconds** (assert the elapsed time, not just the value — AC-2 is a latency claim). Repeat for a second phone → 2. Re-send the *first* phone's JOIN → idempotent, and assert **no** new frame arrives for it (`participants.go` skips the broadcast when `created` is false).
    - **Scenario B — measure the interleaving this story's Task 5 must report on.** Fire ~20 concurrent signed JOIN webhooks from 20 distinct phones, record every `participantCount` the display socket receives in arrival order, and report in the Debug Log: the final value (must be 20), and **whether any frame carried a count lower than one already delivered**. Either answer is useful — an observed regression is direct evidence for the deferred entry; none observed across a few runs is evidence the window is narrow. Do not report a conclusion the run did not produce.
    - **A green E2E is not evidence unless you confirm which process answered** — 3.10's Debug Log records a run silently served by a leftover server. Build the binary once, exec it directly, grep the log for exactly one `server listening` and no `bind:` error.
    - Clean up the scratch game rows afterward (cascades); delete the harness.
  - [x] **Manual browser pass** (`make dev`, real browser). ~~No frontend test framework exists, so this is the *only* verification for the rendering ACs.~~ *(Premise corrected at code review, 2026-08-11: Vitest landed in 3.11 and this story added `lobby-stage.test.tsx`. The browser pass is still the only verification for the **geometry** ACs — computed pixel sizes, full-bleed coverage, bidi layout order — which jsdom cannot measure.)*

    **How to make a Participant join locally** — two options, and you do not need Meta for the first:
    - **Signed webhook POST against the `make dev` server** (preferred, and repeatable): the same signed body the E2E builds, aimed at `localhost`. This is what lets you drive checks 3 and 4 as many times as you like.
    - **A real phone**, if Avraham is available: `make dev` + a cloudflared quick tunnel (**`--protocol http2`** — the default protocol does not work in this environment) + one Graph API `POST /subscriptions` call to repoint Meta's webhook at the tunnel. This is exactly how 2.4's real-phone pass was performed; see its Task 6 notes.

    The checks:
    1. **AC-1** — open the lobby, click "פתח מסך קהל". The display window shows the instruction line, the giant JOIN code, the giant phone number, and the counter pill, on a **green-800 ground that reaches all four edges** (no light strip in the 48px margin), all RTL, nothing focusable.
    2. **AC-6** — the code reads left-to-right correctly inside the Hebrew sentence, and so does the number. Deliberately test a code with digits in it and confirm neither run reorders.
    3. **AC-2** — send a real `JOIN <code>` (or POST the signed webhook) with both windows visible: the pill increments **within a second or so**, and the dashboard's own count moves too.
    4. **AC-5** — with a screen reader (or by watching the DOM), confirm the `sr-only` region's text changes at most once per ~5s while joins arrive quickly, and that the pill's visible number does not lag.
    5. **AC-3 / projection scale** — resize between 1280×720 and 1920×1080: at 1920 the code and number measure ~173px, the instruction ~64px, the pill numeral ~64px; at 1280 everything scales proportionally, **nothing clips**, and no scrollbar ever appears (the shell is `overflow-hidden` — a clipped bottom is silent).
    6. **Transitions** — press "התחל משחק": the display leaves the lobby stage for the placeholder within a second, and the green ground goes with it (the backdrop must not leak into the next stage). Confirm the state announcement still fires.
    7. **Reconnect** — kill the server with the lobby stage up: the stage stays on screen with "מתחבר מחדש…" over it and the counter holds its last value. Restart: it re-renders with no interaction, and the counter does not drop. **Record how the band reads against the green ground** (see Dev Notes — the band's box will be near-invisible; only its white text needs to be legible).
    8. **Two games** — with the display open on game A, edit the URL to game B's id (both in lobby, different counts): B's counter must show B's count, not A's. This is what Task 4's key buys.
    9. **Palette / filter-safety** — Festival Green only, **no gold anywhere**, Network tab shows zero requests to any external host and no `Fetch/XHR` on the display route.
    10. Zero console errors, both windows.

### Review Findings

Code review 2026-08-11 (`bmad-code-review`, three parallel layers: Blind Hunter, Edge Case Hunter, Acceptance Auditor). 12 findings survived triage; 11 were dismissed. Every contested claim below was re-verified against the working tree before classification — the three layers contradicted each other twice, and both contradictions are resolved in the dismissal notes at the end.

**Decisions needed (Avraham)**

- [x] [Review][Decision] **The two `[ASSUMPTION]` Hebrew strings are still unconfirmed** — `joinedLabel: 'הצטרפו'` (plural at every count including 1, because Hebrew singular past tense is gendered and A2 forbids gendered address) and `countAnnouncement: (n) => '${n} הצטרפו למשחק'` (a full sentence because a screen reader gets no layout to carry the meaning). Both were authored to EXPERIENCE.md's rules and the mockup, not transcribed from a spec row. Task 2 explicitly required this confirm-at-review. [web/src/lib/strings.he.ts:238-256]
- [x] [Review][Decision] **`deferred-work.md` records an unverifiable human decision as established fact** — the new sub-bullet under the 4.1 `StageProps` entry states "Avraham's call at the start of 4.2's dev pass was therefore to write the test rather than defer it." `deferred-work.md` is the project's long-lived shared record and a later reader will treat that as settled history; nothing in the repo corroborates it. The substance is defensible either way (no framework was added — Vitest predates this branch — so the scope boundary was never in play). Confirm the attribution, or soften it to "the developer's call, on the grounds that the story's premise had expired."
- [x] [Review][Decision] **The announcer has no leading edge and no flush, so a short lobby announces nothing at all** — the 5s interval is anchored at mount with no leading tick and no announcement on unmount. A room that fills in 4 seconds and starts, or any join landing in the final <5s before the state transition, is never announced: the stage unmounts and `clearInterval` runs. The quoted requirement is "announce at most every 5s **or at milestones**" and only the interval half exists — no milestone path was implemented. Options: (a) accept as-is, (b) add a leading edge (announce immediately, then suppress for 5s), (c) add milestone announcements, (d) flush on unmount. Found independently by the Blind and Edge layers. [web/src/features/display/lobby-stage.tsx:66-69]
- [x] [Review][Decision] **The 9vw ramp has no fit guard, and both failure modes are silent** — (1) *Vertical:* the stack measures ~29.4vw + 136px, and `.stage-root` is `min-h-svh` (a **min**imum), so when it exceeds the viewport the root grows rather than clipping — `overflow-hidden` never engages and the counter pill sits below the fold on a surface nobody scrolls. Breaks whenever `viewportHeight < 0.344 × viewportWidth + 136px`: a non-maximized window, or an ultrawide panel. Both verified targets (1920×1080, 1280×720) fit, and the dev's measured 512px stack at 1280 matches this model exactly — the fit is a property of two tested aspect ratios, not of a guard. (2) *Horizontal:* the giant phone number has no `whitespace-nowrap` and no `min()`/`clamp()`; because both the font size and the container scale with `vw`, the headroom is **glyph-count-bound and viewport-independent** at roughly 19 glyphs. `+972 50-000-0000` is 17. A longer `WHATSAPP_DISPLAY_NUMBER` wraps at its space (feeding failure mode 1) or, if space-free, clips mid-digit with no ellipsis — on the one token the room exists to transcribe. Note `whitespace-nowrap` alone converts a wrap into a clip; the real remedy is a `min()`/`clamp()` on the ramp. Both AC-3 and the browser pass scope to 1080p, so this is beyond-spec robustness, not an AC violation. [web/src/index.css:219, web/src/features/display/lobby-stage.tsx:112-121, web/src/features/display/display-page.tsx:55]
- [x] [Review][Decision] **The join code and phone number are announced twice, and the visual duplicates are not hidden from assistive tech** — the instruction line carries `JOIN <code>` and the number, then the giant sub-stack repeats both with no `aria-hidden`. A screen reader linearises code and number twice each. The giant tokens exist for one stated reason — to be transcribed from the back row — which is a purely visual affordance and the textbook case for `aria-hidden="true"`. Counter-argument: a low-vision user on the projector may *want* the code repeated. Not covered by DESIGN.md or EXPERIENCE.md, so it is a call, not a defect. [web/src/features/display/lobby-stage.tsx:112-122]

**Patches**

- [x] [Review][Patch] An empty lobby announces "0 הצטרפו למשחק" five seconds after it opens — `announced` starts `null` specifically so the first tick is always a real state change and gets read out; that same choice means the zero case cannot be silent. `OpenLobby` always enters `lobby` with zero participants, so this fires on **every** game. A non-event announced as an event, which inverts the stated reason for the null seed. Found independently by all three layers. Minimal fix: skip the announcement while the count is 0. [web/src/features/display/lobby-stage.tsx:53,66-69,145-147]
- [x] [Review][Patch] The throttle test does not pin the throttle — it only ever advances time by exactly 5000ms, so it passes for `announceIntervalMs` set to 5000, 100, or 1; the "burst" assertion passes because *no timer ran*, not because a throttle held. Missing the one negative boundary (advance ~4999ms after a count change, assert the region has **not** updated). Two adjacent gaps in the same file: deleting `return () => clearInterval(id)` leaves the whole suite green (a per-transition timer leak on a page designed to run for hours), and `participantCount: 0` — the value every lobby starts at, and the trigger for the patch above — is never rendered in any test. [web/src/features/display/lobby-stage.test.tsx:84-107]
- [x] [Review][Patch] The `Math.max` comment states a false fact about React, and contradicts `display-page.tsx`'s own correct statement of the same mechanism — "so the render that triggers the update is already correct rather than one frame stale" is wrong: when a component calls its own setter during its own render, React discards that render's output and re-runs immediately, so there is no stale frame to fix and `Math.max` can never observe `highWater < participantCount` in a committed render. `display-page.tsx:94-96` describes the identical pattern correctly ("React re-runs this component immediately without committing the discarded render"). The code is harmless; the comment will mislead whoever copies this pattern into 4.3–4.6. Fix the comment (keeping `Math.max` as belt-and-braces is fine). [web/src/features/display/lobby-stage.tsx:43-45]
- [x] [Review][Patch] This story file asserts five things that are now false — the ⚠️ prerequisite section (L19) still says 4.1 is "implemented, reviewed … but NOT committed"; Testing standards (L348) says "No frontend test framework exists, and this story does not add one"; Task 6 (L263) repeats that premise; Task 5 (L247) says the other Epic-4-adjacent entries need no re-triage when the reconnect-band entry still carried `Revisit trigger: **story 4.2**`; and Task 5's `- [x]` sits above a stated outcome ("**do not** add Vitest here") that the story did the opposite of. Every deviation is disclosed loudly in the Dev Agent Record, so this is a stale-record problem, not a transparency one — but the story is the permanent artifact. [this file]
- [x] [Review][Patch] AC-6's prose undercounts the bidi runs — it says "three runs" while its own Task 3 snippet, the mockup, and the shipped code all carry **four** `<bdi dir="ltr">` elements; the phone number inside the instruction sentence is an LTR run and must be isolated too. The code is right and the AC text is wrong. [this file, AC-6]

**Deferred**

- [x] [Review][Defer] `StageProps` lives in `display-page.tsx`, so the module graph has a real cycle held apart by a single `type` keyword [web/src/features/display/lobby-stage.tsx:7] — deferred, structural and beyond this story's scope (Task 3 mandated the type-only import).
- [x] [Review][Defer] Nothing on the Audience Display has heading semantics — every text element is a `<p>`, including the join code [web/src/features/display/lobby-stage.tsx:95,114,119] — deferred, pre-existing shell-level decision affecting all of 4.2–4.6.

**Dismissed (11) — the two cross-layer contradictions, resolved**

- **"The widened `key` is behaviourally inert" (Edge Case Hunter) — WRONG, do not remove the key.** The layer argued that `/display/A → /display/B` always renders `snapshot === null` first, so `retained.gameId !== gameId` unmounts the stage regardless of the key. It missed the module-level store cache in `use-game-socket.ts:23-34`, which is **never evicted**: navigating to a game whose store already holds a snapshot returns non-null on the very first render, `rendered.gameId` is `B`, and `key={rendered.state}` alone would be `'lobby'` for both games — no remount, and game A's high-water count carries into game B. Task 4's key is load-bearing.
- **"The full-bleed backdrop's containing block is unverifiable / `.stage-fade` may break it mid-animation" (Blind Hunter) — verified handled.** `.stage-fade` animates `opacity` only (`@keyframes stage-fade-in`), and opacity creates a stacking context but **not** a containing block for absolutely positioned descendants. `.stage-root` is `relative` and is the nearest positioned ancestor, so `inset-0` resolves against its padding box as documented. The reconnect band is a later sibling and still paints above.
- **"`9vw` ≠ the mockup's `9cqw`" (Blind Hunter)** — the mockup's query container is `.frame-shell` (`container-type:inline-size`) at the full 1920 design width, with the 2.5cqw safe margin applied *inside* it. The two are exactly equal at 1920; the comment is accurate.
- **`Math.max(undefined, undefined)` → `NaN` on deploy skew** — `participantCount` has been on the snapshot since story 2.3; the shell optional-chains `displaySettings` precisely because that field was new in 4.1. Not the same risk class.
- **The high-water guard is permanent and enforced by nothing** — the premise was independently verified: no `DELETE FROM participants` exists anywhere in `server/`, and `spectatorAllowedStates` excludes `lobby`. The limits and the "revisit both entries together" coupling are recorded in `deferred-work.md`.
- **The fragmented instruction sentence is an i18n anti-pattern** — documented decision with a sound rationale (`<bdi>` cannot ride a plain string; Unicode isolates are assigned to `messages_he.go`), single-locale product.
- **The giant code lacks the mockup's `tabular-nums`; the pill lacks the mockup's `line-height:1.38`** — DESIGN.md is the spine and asks for tabular figures on `phone` only; both match the story's verbatim snippets and were browser-verified.
- **The pill label renders at 32px at 1280×720** — inherited from 4.1's accepted `vw`-ramp deviation; A19's floors and every AC here are stated at 1080p.
- **Scenario B reports a final count of 22 where Task 6 said "must be 20"** — Scenario A's two earlier phones joined the same game first; 20 + 2 = 22. Internally consistent.
- **`expect(screen.getByText(...)).toBeTruthy()` asserts nothing** — true (the query throws on miss), but it is a standard Testing Library idiom; folded into the test-quality patch above.
- **Meta-finding: the diff's comments cite artifacts a reviewer cannot check** — not actionable as a code finding; the citation-heavy style is this project's established convention.

### Review resolution (2026-08-11) — all 10 items applied

Avraham approved the full recommended package. What actually changed:

| Item | Resolution |
|---|---|
| D1 — the two `[ASSUMPTION]` Hebrew strings | **Confirmed as-is.** `joinedLabel: 'הצטרפו'` matches the mockup verbatim, and every non-gendered alternative (`מצטרפים`, `משתתפים`) breaks at the same point at count 1 while reading worse at the counts the screen actually shows. No code change. |
| D2 — the uncorroborated attribution in `deferred-work.md` | **Softened**, with the original wording and the reason for the change recorded in place. The technical reasoning stands without it, and the scope boundary was never engaged (no *framework* was added). |
| D3 + P1 — the announcer | **Rewritten together**, since they were the same bug from two ends. Was: a bare `setInterval` anchored at mount. Now: announce only a **change** from the count the stage arrived to, the first one immediately (leading edge), later ones throttled to at most one per 5s and carrying whatever the count has reached by then. Closes both the `countAnnouncement(0)` on every empty lobby and the total silence of a room that fills and starts inside 5s. Milestones deliberately still unimplemented — nothing defines one. |
| D4 — the ramp fit guard | **Vertical fixed, horizontal deferred.** `--stage-lobby-code: min(9vw, 16vh)` — the two terms are *equal* at 16:9, so every measured value on both verified resolutions is unchanged and the guard engages only on a viewport shorter than 16:9. The horizontal (glyph-count) half went to `deferred-work.md`: its threshold is a config value visible the moment it is set, and `whitespace-nowrap` alone would swap a wrap for a clip. |
| D5 — duplicate tokens announced twice | **`aria-hidden` added** to the giant code/number sub-stack. Both strings are already spoken by the instruction line inside a sentence that gives them meaning; the giant pair is a purely visual affordance for transcription from the back row. |
| P2 — the throttle test did not test the throttle | **Fixed and demonstrated red.** The suite now pins the constant with a negative boundary (a change at 4998ms must still be unannounced) — verified by setting `announceIntervalMs` to 100 and watching exactly that assertion fail (`expected '3 הצטרפו למשחק' to be '2 הצטרפו למשחק'`) before restoring it. Also added: the empty-lobby silence case, the leading edge, and an unmount test asserting `vi.getTimerCount()` drops to 0, which fails if the effect's `clearTimeout` is deleted. 3 cases → 6. |
| P3 — the false React claim beside `Math.max` | **Comment corrected.** `Math.max` kept as belt-and-braces, now labelled as redundant-but-explicit rather than as rescuing a stale frame React never commits. |
| P4 — five false statements in this file | **All five corrected in place**, by strike-through plus a dated note rather than silent edit (the project's convention): the ⚠️ prerequisite, Testing standards, Task 6's browser-pass premise, Task 5's "do not re-triage the others", and the inverted Task 5 outcome. |
| P5 — AC-6 undercounted the bidi runs | **Corrected** to four `<bdi>` elements, naming the one the prose omitted. |

**Gates re-run after the patches:** `npm run lint` clean · `npx tsc -b --noEmit` clean · `npm test` **8 passed** (2 files) · `npm run build` clean · `--stage-lobby-code:min(9vw, 16vh)` confirmed in the built CSS · Hebrew-literal grep over `*.tsx` back to **only** the two known pre-existing violations (a first pass at these patches put Hebrew into two English comments and tripped the gate; both were reworded to `countAnnouncement(0)`). Zero `.go`, `.sql` or `migrations/` diff. No dependency change.

**Not re-run:** the Go E2E and the 63-check browser pass — their harnesses were deleted at the end of the dev session by design. The announcer rewrite invalidates the old **check 4** measurement specifically (visible pill un-throttled while the polite region stayed empty for 5s); that behaviour is now covered by Vitest instead, which is a stronger guarantee for this particular logic. The geometry checks are unaffected: `min(9vw, 16vh)` is arithmetically identical to `9vw` at both verified resolutions, and `aria-hidden` changes no layout.

## Dev Notes

### What 4.1 built that this story consumes — read before writing anything

The shell is done and this story is a *tenant* of it. Everything below already works; re-implementing any of it is the most likely way to go wrong:

- **The socket.** `useGameSocket(gameId, 'display')` handles connect, exponential backoff, indefinite retry, the `seq < lastSeq` stale guard and the StrictMode double-mount. **Do not touch `use-game-socket.ts`.**
- **Last-snapshot retention.** The shell keeps the last non-null snapshot (keyed by `gameId`) and renders the stage *under* the reconnect band. A stage never sees `null` — `StageProps.snapshot` is non-nullable, and that is the contract.
- **The reconnect / connecting / notFound overlays.** The stage never draws a connection state and never announces its own arrival.
- **The cross-fade.** `stage-fade` + the wrapper key. The stage adds no transition of its own.
- **The assertive state announcer.** Owned by the shell, mounted in every branch. The stage's counter region is `polite` and separate.
- **`reducedMotion`.** Already the OR of the viewer's `prefers-reduced-motion` and the Organizer's room-level snapshot setting, and also mirrored as `data-reduced-motion` on `.stage-root` for pure-CSS cases. **No stage ever calls `matchMedia`.**
- **The projection ramp.** `--stage-display` / `--stage-heading` / `--stage-ui` / `--stage-body` / `--stage-margin` in `index.css`. Use the variables; do not hardcode a px.

### Where the lobby stage's data comes from

Every field is already on the snapshot; nothing new crosses the wire.

| Rendered | Snapshot field | Origin |
|---|---|---|
| JOIN code | `joinCode` | `games.join_code`, set at game creation (story 1.3) |
| Phone number | `platformNumber` | `WHATSAPP_DISPLAY_NUMBER` env var → `game.NewEngine`'s `platformNumber` → every `buildSnapshot`/`emptySnapshot` |
| Counter | `participantCount` | `len(ListParticipants(gameID))` in `buildSnapshot` |

`platformNumber` is config, not per-game data, and it is already human-formatted (`+972 50-000-0000` shape, Meta's `display_phone_number`). `joinCode` and `platformNumber` have existed on the snapshot since story 2.3, so unlike `displaySettings` they need **no** optional-chaining against deploy skew.

### The 3-second increment is already delivered — this story only renders it

The chain exists end to end: signed webhook → `wa/inbound.go` `handleJoin` → `game.Engine.Join` → `joinLobby` → guarded `CreateParticipant` → `buildSnapshot` → `wa/inbound.go:236` `broadcaster.Broadcast(gameID, snapshot)` → `ws.Hub.Broadcast` fans out to **every** connection for that game regardless of role → the display's `useGameSocket` sets state → React re-renders. There is nothing to add on the server; AC-2 is a verification obligation, not an implementation one.

Two documented behaviours of that path the stage must not be surprised by:

1. **An idempotent repeat JOIN broadcasts nothing.** `joinLobby` returns early when `created` is false — the count did not change, so there is no frame. Correct, and the E2E asserts it.
2. **A failed post-join `buildSnapshot` skips the broadcast entirely** rather than degrading to `emptySnapshot` (there is no safe placeholder count — it would undercount). The counter simply does not move until the next join or a reconnect. That is the documented behaviour, not a bug for this story to compensate for.

### The out-of-order broadcast, and why the guard is where it is

`deferred-work.md`'s 2.4 entry names this story as its trigger, and 4.1's review sharpened the reason: two concurrent joins each run INSERT → `buildSnapshot` → `Broadcast` independently, and `Hub.Broadcast` stamps `seq` **at call time**, not in commit order (`ws/hub.go:153-161`). Interleave them and the older, lower count can carry the higher `seq`; the client's `seq < lastSeq` guard then accepts it as newer and the room's counter sticks one low until the next join.

Three candidate homes for a fix, and why only one fits a stage story:

- **Fix `seq` properly** (derive it from something commit-ordered, or serialize broadcast-after-write per game). This is the right fix and it is the 3.1 entry's fix. It spans `ws`, `httpapi/control.go`, and every broadcasting writer, and 4.1's review already re-deferred it with Avraham's agreement. Note `games.updated_at` is **no longer** a valid commit-ordered source (4.1's `UpdateGameDisplaySettings` bumps it without a state change) — `deferred-work.md:144`.
- **Serialize in the engine.** The broadcast happens in `wa/inbound.go`, *after* the engine returns, so a per-game mutex inside `participants.go` would not span it. Moving the broadcast into the engine means giving the engine a broadcaster — an inversion of the architecture's one-way dependency (`wa`/`ws`/`httpapi` → `game`).
- **A high-water guard on the one surface a whole room watches.** ~4 lines, provably not a lie in `lobby` (the count only ever grows there), and it removes the only user-visible consequence on a projector. This is what the story does — as a *symptom* guard, recorded as such, with the general defect left open.

**Do not generalise the high-water pattern.** It is correct only because `lobby` has no decrement path. 4.5's leaderboard positions genuinely move both ways; freezing a maximum there would be a real bug.

### Design decisions worth flagging explicitly

- **The lobby stage is the first stage with its own background, and it will not be the last.** The shell paints `surface-base` (light) because DESIGN.md's split-hero game stages are light below the band; the lobby and the winner takeover (4.6) are full-ground green. Rather than teaching the shell about per-stage grounds, each such stage paints its own full-bleed backdrop. That keeps the shell's contract at exactly `StageProps` and means 4.6 has a pattern to copy rather than a shell change to negotiate.
- **The instruction sentence is fragmented in `strings.he.ts`, and that is the lesser evil.** Two competing absolutes: all Hebrew copy lives in `strings.he.ts`, and LTR runs inside RTL text must be `<bdi>`-isolated on web surfaces. A single interpolated string cannot carry markup. Unicode isolate marks (U+2066/U+2069) were the alternative — rejected because DESIGN.md assigns those to `messages_he.go`'s templates and `<bdi>` to web surfaces, and every existing web surface (`lobby-page.tsx`'s join code and platform number) already uses `<bdi>`.
- **The counter announces politely and throttled; the shell announces state assertively.** Two live regions, two politeness levels, two owners — exactly as EXPERIENCE.md's Accessibility Floor splits them. A single merged region would either spam the reader with counts or bury the state change.
- **The pill numeral steps to 64px instead of the mockup's 60px.** 60px is not a ramp step, and A19's floor for display-only content is 48px. Reusing `--stage-heading` keeps the ramp at five (now six) values rather than one-per-component.
- **The instruction line repeats the code and number that appear giant below it.** That is the mockup's design, not redundancy to optimise away: the sentence tells the room *what to do*, and the giant tokens exist to be *transcribed* from the back row.
- **`reducedMotion` is unused here.** Deliberately (AC-7). A lobby whose counter changes value is not a lobby that moves.
- **The reconnect band sits on green-900 over a green-800 ground, and that is a near-invisible container.** The shell's band is `bg-green-900` — chosen against `surface-base`, where it reads as a distinct band. On this stage the two greens are ~1.4:1 apart, so the band's *box* effectively disappears even though its white text stays ~10:1 and perfectly legible. **Do not "fix" this by changing the shell** — the band belongs to 4.1 and 4.6's winner takeover will land on green-800 too, so a real fix (a border, or a token that adapts) is a shell decision for whichever story first proves it matters. Check it in the browser pass and record what you saw; if the text is legible, that is a pass with a note, not a defect.

### Existing code this story modifies — current state, and what must survive

- **[web/src/features/display/display-page.tsx](web/src/features/display/display-page.tsx)** — one map entry and one key expression. Everything else is a 4.1 review decision with recorded reasoning: the render-phase `retained` pattern (lint-forced), the `?? StagePlaceholder` runtime fallback (deploy skew), the always-mounted assertive announcer (a live region populated at mount is not announced), the reconnect band's `--stage-margin` offset (overscan), and the optional-chained `displaySettings?.reducedMotion`.
- **[web/src/index.css](web/src/index.css)** — one custom property appended inside `.stage-root`. The `@theme` tokens, the shadcn `:root`/`.dark` variables, `@layer base`, the `@layer components` `.stage-fade` block, the `@keyframes`, and the `vw`-vs-`rem` deviation comment (Avraham's 4.1 decision, and the record of the forfeited 200%-zoom clause) are all untouched.
- **[web/src/lib/strings.he.ts](web/src/lib/strings.he.ts)** — one `lobby` sub-block inside `display`. The `live` block's five display-control keys and the `display` block's existing keys were confirmed by Avraham at 4.1's review; leave their comments intact.
- **[web/src/features/display/stage-placeholder.tsx](web/src/features/display/stage-placeholder.tsx)** — **unchanged.** It remains the permanent `draft` stage and the temporary occupant of five entries.
- **[_bmad-output/implementation-artifacts/deferred-work.md](_bmad-output/implementation-artifacts/deferred-work.md)** — Task 5's two written outcomes, appended under the existing entries in the file's established style (sub-bullet, dated, trigger re-pointed). Do not rewrite the original text; the file's convention is to strike through wrong premises rather than quietly edit them.

### Testing standards

~~No frontend test framework exists, and this story does not add one (every frontend story's precedent, 1.2 through 4.1 — and see Task 5 for the decision Avraham owes at review). **The browser pass in Task 6 is the frontend verification, and it carries all three ACs.**~~

**Corrected at code review (2026-08-11).** Both clauses were false by the time this story ran: Vitest, `@testing-library/react` and `jsdom` landed with story 3.11 and are in `web/package.json`, and this story added `web/src/features/display/lobby-stage.test.tsx` (6 cases after review). No framework was *added* here, so the scope boundary — which forbids adding a test **framework** without an explicit decision — was never engaged. The division of labour that actually applies: **Vitest carries the logic** (the high-water guard, the announcer's silence-until-change, its leading edge, its 5s throttle, and its timer cleanup), and **the browser pass carries the geometry** (computed pixel sizes, full-bleed coverage, bidi layout order, no-clipping at the 1280×720 floor) — none of which jsdom can measure.

Go stdlib `testing`, co-located `_test.go`, stubs grown in place — no mock framework. This story adds no Go code and therefore no Go tests; run the suite only to prove zero regression. **No real-DB unit tests**: the documented standard since 2.1, whose four-entry deferral family was re-deferred at 4.1's review under a trigger that no longer names an epic.

`go test -race` may be unavailable (`CGO_ENABLED=0` in this environment, per 3.8–4.1's Change Logs). If so, say so in the Dev Agent Record rather than implying race coverage.

Story 3.10's and 4.1's manual browser passes may still be outstanding. If so, do them in the same session — this story's pass already puts you in front of both surfaces.

### Project Structure Notes

**New:**
- `web/src/features/display/lobby-stage.tsx` — named by the architecture (`display/ … lobby-stage.tsx  # JOIN instructions + climbing counter`)

**Modified:**
- `web/src/features/display/display-page.tsx` (one map entry, one key)
- `web/src/index.css` (+1 custom property inside `.stage-root`)
- `web/src/lib/strings.he.ts` (+`display.lobby` block)
- `_bmad-output/implementation-artifacts/deferred-work.md` (Task 5 re-triage)

**Untouched (a diff here means you went off-spec):** all of `server/**` · `server/migrations/**` · `web/src/lib/use-game-socket.ts`, `api.ts`, `types.ts`, `text.ts`, `use-space-action.ts` · `web/src/app.tsx` · `web/src/components/**` · `web/src/features/display/stage-placeholder.tsx` · `web/src/features/lobby/**`, `features/live/**`, `features/builder/**`, `features/results/**`, `features/auth/**` · `web/index.html` · `web/package.json`.

### References

- Epic + ACs: [_bmad-output/planning-artifacts/epics.md](_bmad-output/planning-artifacts/epics.md#L682-L695) (Story 4.2), [#L191-L194](_bmad-output/planning-artifacts/epics.md#L191-L194) (Epic 4 framing)
- Lobby stage visual spec: DESIGN.md frontmatter `components.stage-lobby`; contrast table (`Colors`); `Typography` → projection scale `[A19]`, bidi isolation, uppercase tracking; `Elevation & Depth` → "Audience Display: flat"
- Behaviour: EXPERIENCE.md → IA `Audience Display — stages` (Lobby row); `State Patterns` (Lobby row); `Accessibility Floor` → Live regions (polite + throttled counters); `Key Flows` Flow 1 (UJ-1) steps 2–4
- Mockup: `_bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/mockups/key-stage-lobby.html` — the authoritative pixel resolution of the stage-lobby spec (spine wins on conflict)
- FR-1's 3-second consequence: [prd.md](_bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md#L75-L82)
- Architecture: `Frontend Architecture` (display renders exclusively from the WS snapshot); `Structure Patterns`; the `features/display/` tree naming `lobby-stage.tsx`; `Component Boundaries (Web)`
- Deferred entries this story must resolve: [deferred-work.md](_bmad-output/implementation-artifacts/deferred-work.md) lines 52–53 (2.4 out-of-order lobby broadcasts) and 145 (4.1 `StageProps` wiring); context in lines 68–70 (3.1 `seq`) and 144 (`updated_at` no longer commit-ordered)
- Prior story: [4-1-audience-display-shell-the-screen-that-follows-the-game.md](_bmad-output/implementation-artifacts/4-1-audience-display-shell-the-screen-that-follows-the-game.md) — the shell contract, its Review Findings, and the four decisions Avraham resolved on 2026-08-09

### Latest technical information

No dependency changes and no new libraries. The versions this story writes against, as pinned in `web/package.json`: React 19.2, React Router 8.2, Tailwind CSS 4.3, TypeScript 6.0, Vite 8.1, `eslint-plugin-react-hooks` 7.1.

Two version-specific details that will bite if missed:

- **Tailwind v4 arbitrary values need the `length:` hint for CSS variables in `text-*`**: `text-[length:var(--stage-heading)]`, not `text-[var(--stage-heading)]` (ambiguous between font-size and colour). This is the form 4.1 established. If it fails to resolve, fall back to `style={{ fontSize: 'var(--stage-lobby-code)' }}` — never to a hardcoded px.
- **Three utilities this story is the project's first caller of**: `leading-heading` (from `--leading-heading: 1.38`), `tabular-nums`, and `leading-[1.1]`. `tabular-nums` is stock Tailwind and safe; `leading-heading` comes from the `--leading-*` theme namespace and *should* generate, but **verify it in the built output** rather than assuming — if it does not, use `leading-[1.38]` and say so in the Debug Log rather than dropping the line-height. (`rounded-pill` is the fourth candidate and Task 3 already routes around it.)
- **`eslint-plugin-react-hooks` v7** enforces `react-hooks/refs` (no ref reads during render) and `react-hooks/set-state-in-effect` (no `setState` in an effect body). Both shape Task 3's code: state adjusted during render for the high-water count, latest-ref written *in* an effect, and `setState` called only from the interval callback (async, therefore allowed). Do not "clean up" any of the three into the shape the rules reject.

## Dev Agent Record

### Agent Model Used

claude-sonnet-5 (Claude Code, `bmad-dev-story`), 2026-08-11.

### Debug Log References

**Branch point — the prerequisite resolved itself.** The story's ⚠️ section says 4.1's work is uncommitted on `story/4-1-audience-display-shell` and that `baseline_commit` (`a4523ea`) is *not* the tree to branch from. By the time this story was developed, **both 4.1 and 3.11 had merged to `main`**: `origin/main` is at `8f04cc5` (PR #10, story 3.11) which contains `4b55278` (4.1) and `06b16ab` (3.11). This story branches from `8f04cc5` as `story/4-2-lobby-stage`. `baseline_commit` in the frontmatter is left untouched per the workflow's preserve rule, but **`8f04cc5` is the real base**. Every shape the story asked to verify on disk was confirmed present and unchanged before any code was written.

**Tailwind utilities — all four "first caller" candidates verified in the BUILT css, not assumed.** The story flagged `leading-heading`, `tabular-nums` and `leading-[1.1]` as unverified, with a documented fallback. No fallback was needed:

| utility | generated declaration |
|---|---|
| `leading-heading` | `line-height:var(--leading-heading)` (1.38) |
| `leading-[1.1]` | `line-height:1.1` |
| `tracking-[0.08em]` | `letter-spacing:.08em` |
| `text-[length:var(--stage-lobby-code)]` | `font-size:var(--stage-lobby-code)` |
| `.font-display` | `font-weight:var(--font-weight-display)` (900) |

`--stage-lobby-code: 9vw` is present in the built CSS. `rounded-pill` was never called, per Task 3.

**`go test -race` is unavailable here** — `CGO_ENABLED=0`, and `go test -race` answers *"-race requires cgo"*. Carried from 3.8–4.1. **No race coverage is claimed.** This story adds zero Go code, so the Go suite was run only to prove zero regression: all 8 packages `ok`, `go vet ./...` clean. The bare `gofmt -l .` CRLF caveat carried since 3.4 was not exercised — **zero `.go` files are in this diff**, verified with `git status --porcelain | grep -E '\.go$|\.sql$|migrations/'` → empty.

**Local Go E2E — `server/cmd/e2escratch`, created, run 4×, deleted.** Run on **port 8099** rather than 8080, deliberately: a `make dev` from the previous session was still holding 8080, and running beside it defeats 3.10's "which process answered?" trap by construction rather than by grepping. `WHATSAPP_API_BASE_URL` and `ANTHROPIC_API_BASE_URL` were pointed at dead local addresses — the log carries the two override confirmations and **zero `graph.facebook.com`**, so no WhatsApp traffic left the machine. Exactly one `server listening`, zero `bind:` errors.

*Scenario A — the counter really climbs on a `role=display` socket.* Initial frame: `participantCount=0`, `state=lobby`, `joinCode`/`platformNumber` non-empty and matching the game/config. First signed JOIN → count 1 **in 17ms**; second → count 2 **in 10ms** (subsequent runs: 9/8, 8/9, 9/7ms). AC-2's budget is 3 seconds; the measurement is ~200× inside it. A repeat JOIN from an already-joined phone, with a **fresh wamid**, broadcast **nothing** — `joinLobby`'s `created == false` early return, as documented.

*The harness's own first bug is worth recording, because it is the trap the story warned about.* Runs 2 and 3 initially scored **0 joins with a silent green webhook 200** — reused wamids, dropped by the dedupe ledger as Meta retries. The silence is indistinguishable from "the counter doesn't work". Fixed with a per-invocation `runID` in every wamid.

*Scenario B — the interleaving is REAL, and this is the evidence Task 5 reports.* 20 concurrent signed JOINs from 20 distinct phones, every `participantCount` recorded in arrival order:

| run | arrival order | frames lower than one already delivered |
|---|---|---|
| 1 | `[4 5 14 14 14 16 15 14 16 16 17 17 21 17 21 16 21 21 21 22]` | **4 / 20** |
| 2 | `[20 20 20 20 20 21 20 22 21 21 21 21 21 21 20 21 22 22 21 22]` | **10 / 20** |
| 3 | `[14 19 19 19 19 19 19 19 19 19 19 19 20 19 19 22 22]` | **2 / 17** |
| 4 | `[20 ×18 22 22]` | 0 / 20 |

**Three of four runs delivered counts that go backwards** — run 1 shows `17 → 21 → 17` and `21 → 16`; run 2 shows `22 → 21` *after* the true total had already been displayed. The final value was correct in all four runs, and the DB held 22 participants every time. Two secondary observations: runs deliver **fewer frames than joins** (17 for 22 in run 3) — the hub's 8-slot per-connection drop, which weakens the "the next join self-heals it" argument because the healing frame can itself be dropped; and the regression window is clearly wide, not the "few milliseconds" the 3.1 entry estimates for the two-writer case. Without the high-water guard this stage would visibly count backwards in front of a room.

**Manual browser pass — 63 checks, all green, in real Edge.** `playwright-core` + `channel: 'msedge'` (installed into the scratchpad, never into `web/package.json`); driver deleted afterwards. Seeded through the real API and real HMAC-signed webhooks. Two phases because check 7 needs the Go server killed mid-run.

*Phase 1 (45 checks): AC-1/2/3/5/6, checks 1–6, 8, 9, 10.* Green ground measured **full-bleed 1920×1080 at (0,0)** with all four screen corners painting `rgb(22, 101, 52)` — the backdrop does cover the 48px safe margin, as Task 3 predicted from the padding-box rule. Zero focusables, zero `box-shadow`, `dir="rtl"`. Exactly **4 `<bdi>` runs**, all `dir="ltr"`, and each measured character-by-character to prove it really lays out left-to-right inside the RTL page (`JOIN VZ52JH`: first char x=1038 < last x=1408). Type at 1920: instruction **64.0px**, code **172.8px**, number **172.8px**, pill numeral **64.0px**; code tracking **13.824px** (=0.08em), number tracking **normal**; code line-height **1.1**, instruction **1.38**. No scrollbar. Zero gold elements, zero requests to any external host, zero console errors.

*Check 4 / AC-5 is the one worth reading the numbers for.* Six joins fired ~420ms apart: the **visible pill** read 1,2,3,4,5,6 at 425/857/1274/1690/2113/2527ms while the **polite region stayed empty the entire time**, then caught up to `6 הצטרפו למשחק` after the 5s tick. That is both halves of the requirement in one measurement — throttled announcement, un-throttled display — and it confirms the region mounts empty rather than pre-filled.

*Check 8* — with the display on game A showing **7**, rewriting the URL to game B in place showed **3** and B's own join code. Task 4's `${gameId}:${state}` key is what buys that; without it A's high-water count would have carried over.

*Phase 2 (18 checks): the 1280×720 floor and a REAL reconnect.* At 1280 every ramp value scales to exactly 0.667× (instruction 42.67px, code/number 115.2px, pill 42.67px), content sits at 104..616 of 720 vertically and 162..1118 of 1280 horizontally — **nothing clips, no scrollbar in either axis**. For check 7 the Go server was **killed** (`ctx.setOffline` does not close an established socket and would have passed vacuously) and `ws.on('close')` was asserted to have fired **before** anything else was trusted: the lobby stage stayed on screen, the band appeared over it, the counter held 4, the green ground survived; after restart the band vanished with no interaction, the count did not drop, and a fresh JOIN incremented it to 5 — proving the socket was genuinely live again, not merely painted.

**Two things the browser pass measured that feed `deferred-work.md` directly.** (1) *The band-overlay entry, whose trigger literally names story 4.2:* band box **48–124px**, this stage's content box starts at **190px** — 66px of clearance, nothing covered, because the shell's wrapper is `justify-center` and this stack is ~700px in a 984px box. Re-deferred with the trigger re-pointed off a story number. (2) *Band legibility on green:* `rgb(20, 83, 45)` on `rgb(22, 101, 52)` (~1.4:1 — the box is effectively invisible) with white text at ~10:1 (perfectly legible). Exactly what the Dev Notes predicted: **a pass with a note, not a defect**, and not this story's to fix.

**Console errors: 0 in phase 1; 4 in phase 2, all during the deliberate outage** — two WS handshake rejections and two Vite-proxy 502s while the Go server was dead. Those errors *are* the reconnect machinery working; same class as the finding 4.1's review re-scoped.

**Cleanup.** Both scratch organizers (`e2escratch-4-2`, `browserpass42`) deleted with their games (cascade); 110 scratch `wa_inbound_messages` ledger rows deleted; `cmd/e2escratch`, the Playwright driver and every scratch binary removed. Verified afterwards: only the 6 pre-existing games remain, and `git status` shows no `.go`, `.sql` or `migrations/` diff.

**One environment note for Avraham.** A `make dev` pair from the previous session (started ~10 hours earlier) was holding :8080 and :5173. It was stopped so the reconnect check could kill and restart a server it owned, and the servers this pass started were stopped afterwards — **nothing is running now**; `make dev` when you want it back.

### Completion Notes List

**All three ACs are satisfied, plus the four derived requirements.**

- **AC-1** — the display in `lobby` renders the JOIN instruction (`שלחו JOIN <code> למספר <number>`) and a live participant counter, per DESIGN.md's `stage-lobby` spec. Verified in a real browser.
- **AC-2** — a Participant joining increments the counter well inside 3 seconds: **17ms / 10ms** measured on the wire, and the pill visibly climbed 1→6 across 2.5s in the browser.
- **AC-3** — projection scale verified by computed style at 1920: code and number **172.8px** (spec: "120px+"), instruction **64px**, pill numeral **64px**, digit-grouped and `tabular-nums`, all bidi-isolated.
- **AC-4** (derived) — the stage paints its own **full-bleed** `green-800` ground behind the shell's `--stage-margin`, measured 1920×1080 at (0,0) with all four corners green. `ink-on-dark` text.
- **AC-5** (derived) — the counter announces **politely and throttled**: region mounts empty, visible pill un-throttled, announcement catches up on the 5s tick. The shell's assertive region is untouched.
- **AC-6** (derived) — three LTR runs isolated in four `<bdi dir="ltr">` elements, each proven to lay out left-to-right inside the RTL page by per-character geometry, on a code containing digits.
- **AC-7** (derived) — `reducedMotion` is unread by this stage, deliberately. No animation was invented in order to disable one.

**Two decisions Avraham owes a look at, and one he already made.**

1. **[ASSUMPTION] — the two Hebrew strings, unconfirmed, same treatment 4.1's copy got.** `joinedLabel: 'הצטרפו'` (plural at every count including 1, because Hebrew singular past tense is gendered and A2 forbids gendered address — "1 הצטרפו" is the gender-safe imperfection) and `countAnnouncement: (n) => '${n} הצטרפו למשחק'` (a full sentence, because a screen reader gets no layout to carry the meaning). Both were authored to EXPERIENCE.md's rules and the mockup, not transcribed from a spec row. **Please confirm or replace at review.**
2. **The high-water guard is a symptom-level fix and is recorded as one.** It is provably not a lie in `lobby` (the count only grows there) and it removes the only user-visible consequence on a projector. The general `seq`-ordering defect is untouched and stays deferred. **Do not copy this pattern into 4.5** — leaderboard positions legitimately move both ways.
3. **Vitest: the story's prohibition rested on a premise that had already expired, and you overruled it.** Story 4.2 was written to defer the project's first *display* test to 4.3 because "no frontend test framework exists" — but 3.11 landed Vitest + `@testing-library/react` before this story ran, and the second half of the premise ("`LobbyStage` is pure presentation with no logic a unit test could assert") was wrong: the high-water guard **is** logic, and it is the mitigation for a measured defect. You chose to write the test. `web/src/features/display/lobby-stage.test.tsx` has 3 cases and was **demonstrated red** against a build with the guard removed and the announcer pre-seeded (`expected '3 הצטרפו למשחק' to be ''`) before being made green. It also closes the `snapshot` half of 4.1's `StageProps`-wiring entry; the `reducedMotion` half stays open for 4.3.

**Task 5 — four `deferred-work.md` entries written, one more than the story listed.** The 2.4 out-of-order entry (mitigated here, general fix re-deferred, trigger re-pointed to *the first display field that can legitimately decrease*, with the four-run measurement recorded as reusable evidence); the 4.1 `StageProps` entry (`snapshot` half closed, `reducedMotion` half re-triggered on 4.3, and the story's stale premise corrected in writing); the 2.5 spectator-roster entry (confirmed still **not** triggered, and now noted as load-bearing for the high-water guard's soundness, so the two must be revisited together). **The fourth was not in the story's list**: the 4.1 reconnect-band-overlay entry names *"story 4.2"* as its trigger in as many words. It fired, was measured, and was re-deferred with its trigger re-pointed off a story number — recorded rather than skipped, because a later reader finding an unfired trigger with 4.2's name on it would have to re-derive it.

**No Go code, no migration, no new dependency, no `shadcn add`, no gold.** One entry changed in `stageByState`. Zero Hebrew literals in either new `.tsx` file — the test reads its expected copy out of `strings.he.ts` rather than retyping it, which keeps it out of the copy-centralization grep *and* makes it assert the wiring instead of asserting that two files agree on a string.

### File List

**New**

- `web/src/features/display/lobby-stage.tsx` — the lobby stage
- `web/src/features/display/lobby-stage.test.tsx` — Vitest cases: StageProps wiring, the high-water guard, and the polite announcer (silence until the count changes, the leading edge, the 5s throttle with its negative boundary, and timer cleanup on unmount). 3 at dev time, **6 after code review**.

**Modified**

- `web/src/features/display/display-page.tsx` — `stageByState.lobby` → `LobbyStage`; stage wrapper key widened to `${gameId}:${state}`
- `web/src/index.css` — `--stage-lobby-code: 9vw` appended inside `.stage-root`
- `web/src/lib/strings.he.ts` — `display.lobby` block appended
- `_bmad-output/implementation-artifacts/deferred-work.md` — Task 5's four re-triage outcomes
- `_bmad-output/implementation-artifacts/4-2-lobby-stage-the-room-fills-the-screen.md` — this file (tasks, Dev Agent Record, Change Log, Status)
- `_bmad-output/implementation-artifacts/sprint-status.yaml` — `4-2-…` → `review`

**Created and deleted within the session (never committed)**

- `server/cmd/e2escratch/` — the Go E2E harness
- scratchpad Playwright driver (`pass.mjs`, `pass2.mjs`) and every scratch binary

## Change Log

| Date | Change |
|---|---|
| 2026-08-11 | Story 4.2 implemented: lobby stage with JOIN instructions, giant code + phone number, and a live participant counter with a high-water guard and a throttled polite announcer. Branched from `8f04cc5` (4.1 and 3.11 both merged), not from the stale `baseline_commit`. |
| 2026-08-11 | Added the project's first Audience Display test (`lobby-stage.test.tsx`, 3 cases, demonstrated red first) — Avraham's decision, overruling the story's now-expired "no frontend test framework exists" premise. |
| 2026-08-11 | Task 5: four `deferred-work.md` entries re-triaged, including one the story did not list (the reconnect-band overlay, whose trigger names story 4.2). The 2.4 out-of-order entry now carries measured evidence: 3 of 4 E2E runs delivered counts that go backwards. |
| 2026-08-11 | Verification: Go E2E ×4 (no WhatsApp traffic left the machine), 63-check browser pass in real Edge across two phases including a real kill-and-restart reconnect, full frontend + backend gate suite green. `-race` unavailable (`CGO_ENABLED=0`); no race coverage claimed. |
| 2026-08-11 | Code review (`bmad-code-review`, three parallel layers). 12 findings after triage, 11 dismissed. All 5 decisions and all 5 patches resolved; 2 items deferred to `deferred-work.md`. |
| 2026-08-11 | Review fix: the counter announcer rewritten from a mount-anchored `setInterval` to change-driven with a leading edge — it no longer announces `countAnnouncement(0)` on every empty lobby, and no longer stays silent through a room that fills and starts inside 5s. Test cases 3 → 6, with the throttle constant now pinned by a negative boundary (demonstrated red at 100ms). |
| 2026-08-11 | Review fix: `--stage-lobby-code` → `min(9vw, 16vh)`. `.stage-root` is `min-h-svh`, so an overlong stack grows the root instead of clipping and drops the counter below the fold silently. The two terms are equal at 16:9, so every verified measurement is unchanged. |
| 2026-08-11 | Review fix: `aria-hidden` on the giant code/number pair (already spoken by the instruction line); `Math.max` comment corrected (it claimed to rescue a render React never commits); five false statements in this story file and one uncorroborated attribution in `deferred-work.md` corrected in place. |
