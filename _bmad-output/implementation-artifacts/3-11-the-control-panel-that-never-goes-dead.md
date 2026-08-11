---
baseline_commit: 4b55278be8385bbd960e056f7f70c98c3f15e03f
---

# Story 3.11: The Control Panel That Never Goes Dead

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an Organizer,
I want the live control panel to keep responding to every action for the whole game,
so that one press of "סגור שאלה" cannot strand me — and the room — mid-game (FR-13, UX-DR10).

## ⚠️ This story is not in `epics.md`

It was injected from a review finding, not planned. It exists because 4.1's code-review browser pass found a **severe, pre-existing, `main`-branch defect** and `deferred-work.md` records it as *"Revisit trigger: **immediately** — this should be the next story, ahead of 4.2. It is the highest-severity item in this file."* ([deferred-work.md:143](_bmad-output/implementation-artifacts/deferred-work.md))

Numbered `3.11` because the defect lives in Epic 3's control panel (story 3.1's surface) and Epic 3 is still `in-progress` — this keeps 4.2–4.6 numbered exactly as `epics.md` has them. `epics.md` has **not** been amended; if you want the epic file to carry this story, that is a `correct-course` run, not a task here.

**Ship this before 4.2.**

## ⚠️ Prerequisite: branch from 4.1's work, not from `main`

The defect is on `main` (`origin/main` = `c148cc8`), but **one of the two broken files does not exist there.** `web/src/features/live/display-controls.tsx` was created by story 4.1 and is only on `story/4-1-audience-display-shell` (`4b55278`, the `baseline_commit` above), which is pushed but **not yet merged**. `git ls-tree origin/main -- web/src/features/live/` lists only `control-page.tsx` and `response-stats.tsx`.

So: **branch from `story/4-1-audience-display-shell`**, or from `main` once 4.1 has merged. Branching from `main` today makes Task 3a impossible. Follow the repo's branch convention: `story/3-11-the-control-panel-that-never-goes-dead`.

Verify these shapes on disk before writing any code:

- `web/src/features/live/control-page.tsx` — `submittingRef` + `fire()` at lines 80–88; the latest-ref pair and the `[snapshot.state]` reset effect at lines 105–114.
- `web/src/features/live/display-controls.tsx` — `submittingRef` + `submit()` at lines 54–63; the latest-ref + `[gameState]` reset effect at lines 72–78. **Same defect, second instance, not recorded anywhere yet.**
- `web/src/features/lobby/lobby-page.tsx` — `startSubmittingRef` + `fireStart()` at lines 42–47. **Not currently broken** (this file has no reset effect), but it is the third copy of the same pattern.
- `web/src/lib/use-space-action.ts` — exists, and its docstring is the precedent this story follows: *"Shared by every 'one persistent button' primary-action surface … so both get the same fix in one place."*
- `web/package.json` — no `test` script, no test framework. This story adds the project's first one (see Task 4).

## The defect, in one paragraph

`fire()`'s only early return is `if (submittingRef.current) return`, and the only thing that ever clears that ref is the `onSettled` callback passed to `action.mutate(...)`. Every control handler in `server/internal/httpapi/control.go` calls `hub.Broadcast(...)` **before** `writeJSON(...)` — deliberately, and in all six handlers (lines 212/235/259/282/306/336) — so the WebSocket frame normally beats the HTTP response. The new `snapshot.state` fires the reset effect, `action.reset()` runs, React Query detaches the observer from the in-flight mutation, and the per-call `onSettled` is **never invoked**. `submittingRef` stays `true` for the lifetime of the component. Every later press of the primary CTA is swallowed with no request, no banner, and no console message.

**Observed in 4.1's browser pass:** `start` (LobbyPage's own ref, unaffected) → `close-question` (works, poisons ControlPage's ref) → `reveal` **dead**. `location.reload()` and the identical click returns `reveal:200`.

**Verified in the installed source**, not inferred — `node_modules/@tanstack/query-core/build/modern/mutationObserver.js`:

```js
reset() {
  this.#currentMutation?.removeObserver(this);   // ← detaches
  this.#currentMutation = void 0;
  this.#updateResult();
  this.#notify();                                 // ← no `action`, so no onSettled
}
```

`#notify(action)` only invokes `#mutateOptions.onSettled` when it is called with an `action` of type `success`/`error`, and those calls arrive via `onMutationUpdate` — which the detached observer no longer receives.

**Both halves of this bug were introduced by the same code review.** 3.1's review (2026-08-04) added the ref guard in finding #143 and the reset effect in finding #146. Each is correct alone; together they deadlock. Neither review pass noticed, and 4.1's Dev Notes then marked all three sites off-limits, which is why it survived to now.

## Acceptance Criteria

1. **Given** a game past `lobby` and the normal broadcast-before-response ordering, **when** the Organizer drives the full state machine from the control panel (`close-question` → `reveal` → `next-question` → … → `finished`) pressing the primary CTA once per state, **then** every press issues its request and advances the game. **No press is swallowed and no page reload is needed at any point.**
2. **Given** any control mutation is in flight when a state change resets it, **then** the in-flight lock is released once the request settles — **regardless of whether React Query still has that mutation attached to its observer.**
3. **Given** the reduce-animations checkbox is toggled and a game-state change lands before the PUT settles, **then** the checkbox still accepts every later toggle. (`display-controls.tsx` carries the identical defect and is fixed by the same change.)
4. **Given** rapid repeated activation of one CTA — a held Space (OS auto-repeat) or a double click — inside a single request's window, **then** exactly **one** request is issued. Story 3.1's review finding #143 is a live guarantee and must survive this fix.
5. **Given** a stale error banner and a live state that advances, **then** the banner still clears. Story 3.1's review finding #146 is also a live guarantee; the fix must not trade one regression for the other.
6. **Given** the primary CTA, **then** it is still never `disabled` while a mutation is pending. Story 3.1's AC-4 focus retention depends on it (a disabled button drops DOM focus in every browser), and it is why the ref guard exists at all.
7. **Given** the fix, **then** an automated test in the repository asserts AC-2 and AC-4, runs in CI, and has been **demonstrated to fail against the pre-fix code**.

### Derived requirements — binding, and each has a source

8. **The lock's release must not depend on anything a third party can detach.** This is the root cause, stated as a requirement: the current design delegates the release to an observer callback, and `reset()` — called from a *different* effect, for an *unrelated* reason — silently cancels it. A fix that merely adds `submittingRef.current = false` to today's one known reset call site leaves the next `reset()` (a `gameId` guard, a new banner rule, a fourth surface) free to reintroduce the identical bug. See Dev Notes → *Three fixes were on the table*.
9. **One implementation, not three.** The pattern exists verbatim in three files with three copies of the same six-line comment, and the duplication is exactly why the bug shipped twice. `web/src/lib/use-space-action.ts` set the precedent for this codebase and states the reason in its own docstring; the architecture's Web boundary rule agrees: *"features never import each other — shared logic is promoted to `lib`"* (architecture § Component Boundaries (Web)).
10. **Vitest, co-located, is architecture-sanctioned — not a new decision to litigate.** Architecture § Structure Patterns already specifies *"TS tests co-located: `answer-distribution.test.tsx` next to source (Vitest)"*. **Avraham approved adding it in this story (2026-08-09)**, overriding 4.2's recommendation to wait for 4.3. That recommendation's premise — *"no genuinely unit-testable frontend logic exists yet"* — is falsified by this defect, which is pure frontend async logic that no type checker and no casual browser click-through catches.
11. **No Hebrew copy changes.** `strings.he.ts` is untouched. This story changes *when* requests fire, never what the panel says.

### Scope boundaries for this story

- **Frontend only. Zero `.go` files change.** The broadcast-before-response ordering is deliberate and documented, and it is not the bug — the client is. A `.go` diff means you went off-spec.
- **No migration, no `sqlc generate`, no `queries/*.sql` change.**
- **Do not "fix" the `seq` ordering defect.** `deferred-work.md`'s 3.1 and 2.4 entries describe a *different* bug (broadcast ordering across two writers) whose fix is a `ws`/`control.go` redesign. It stays deferred. This story does not touch `server/internal/ws/**`.
- **Do not touch `web/src/lib/use-game-socket.ts`.** The socket is not implicated.
- **Do not apply the REST response snapshot into the socket store.** That is the standing 3.1 entry ("Control mutations discard the REST response snapshot") and it is a separate architectural change. This story leaves all seven mutations as `api<void>`/`api<unknown>`.
- **Do not re-add `disabled={isPending}` anywhere** (AC-6).
- **No new UI, no new copy, no `components/ui/*` edit, no `shadcn add`.**
- **No `web/src/features/display/**` change.** The display is a reader; it has no mutations.
- New npm packages are limited to the four in Task 4. No others.

## Tasks / Subtasks

- [x] **Task 1: `web/src/lib/use-single-flight.ts` (new) — one lock, released by the request itself** (AC: 2, 4, 8, 9)

  ```ts
  import { useCallback, useRef } from 'react'
  import type { UseMutationResult } from '@tanstack/react-query'

  /**
   * Collapses duplicate activations of one mutation: while a call is in
   * flight, further calls are dropped. Shared by every surface with a
   * single primary action (the live control panel, the lobby's start
   * button, the display controls) so all three get the same fix in one
   * place — the same reason use-space-action.ts exists.
   *
   * The lock is released by the mutation's OWN promise, never by a
   * per-call onSettled. This is the whole point of the hook, and story
   * 3.11 exists because the previous shape got it wrong: useMutation's
   * `mutate(vars, { onSettled })` routes that callback through the
   * MutationObserver, and `mutation.reset()` calls
   * `currentMutation.removeObserver(this)` — after which the callback is
   * never invoked and the lock is held forever. Two of these three
   * surfaces run a reset() on every state change to clear stale error
   * banners, so that was the normal path, not an edge case.
   *
   * `mutateAsync` is `observer.mutate` unwrapped: it returns the
   * Mutation's own execute() promise, which settles whether or not any
   * observer is still attached. Nothing outside this hook can cancel the
   * release.
   */
  export function useSingleFlight<TData, TError, TVariables, TContext>(
    mutation: UseMutationResult<TData, TError, TVariables, TContext>,
  ): (variables: TVariables) => void {
    // A plain ref, not mutation.isPending: React Query does not flip
    // isPending on the closure's snapshot synchronously inside mutate(),
    // so a second call arriving before the next render (a rapid
    // double-press, or Space racing a click) would read a stale false and
    // fire twice. The ref is set synchronously, immune to that gap.
    // (Story 3.1 code review, finding #143 — still the reason.)
    const inFlightRef = useRef(false)
    return useCallback(
      (variables: TVariables) => {
        if (inFlightRef.current) return
        inFlightRef.current = true
        mutation
          .mutateAsync(variables)
          .catch(() => {
            // Swallowed deliberately. The error already lives on the
            // mutation's own state (isError/error), which is what every
            // caller renders; re-throwing here would only produce an
            // unhandled rejection. This is exactly what useMutation's own
            // `mutate` does — `observer.mutate(...).catch(noop)`.
          })
          .finally(() => {
            inFlightRef.current = false
          })
      },
      [mutation],
    )
  }
  ```

  - [x] **`import type`** for `UseMutationResult` — `verbatimModuleSyntax: true` in `tsconfig.app.json` makes a value import of a type an error.
  - [x] **Four generics, `TContext` included.** `UseMutationResult`'s fourth parameter defaults to `unknown`; declaring it lets inference succeed at every call site without anyone annotating anything.
  - [x] **The returned callback's identity changes every render** (`useMutation` returns a fresh object each render, so `[mutation]` never matches). That is unchanged from today's `useCallback(..., [action])` and is not this story's problem — do not "fix" it with a latest-ref, which would reintroduce a `react-hooks/refs` violation.
  - [x] **The `void`-variables case has one sharp edge. Verified with `tsc` against this repo's settings, so do not re-derive it:**
    - `fireStart()` with **zero arguments** — ✅ compiles. TypeScript lets you omit an argument whose parameter type is `void`.
    - `const f: () => void = fireStart` — ✅ compiles, so `useSpaceAction(fireStart, …)` is fine.
    - `onClick={fireStart}` — ❌ **does not compile.** React's `onClick` is `(e: MouseEvent…) => void`, and under `strictFunctionTypes` the parameter check is contravariant: `Type '{...}' is not assignable to type 'void'`. **Every DOM event handler must wrap it:** `onClick={() => fireStart()}`. Task 3b flags the one site this hits.
  - [x] Comments in English. Zero Hebrew literals (CI gate is Go-only, but `deferred-work.md` records the frontend convention and 4.2's Task 6 scans for it).

- [x] **Task 2: `web/src/features/live/control-page.tsx` — adopt the hook** (AC: 1, 2, 4, 5, 6)

  - [x] Replace the `submittingRef` + `fire` block (lines 80–88) with:
    ```tsx
    const fire = useSingleFlight(action)
    ```
    and import it from `@/lib/use-single-flight`.
  - [x] Drop the now-unused `useRef`/`useCallback` imports **only if** nothing else in the file still needs them — `actionRef`/`stopRef` still use `useRef`, so `useRef` stays and `useCallback` goes. `noUnusedLocals` is on; an unused import is a build failure, and so is removing one that is still used.
  - [x] **Move the six-line "A plain ref, not action.isPending" comment into the hook** (Task 1 already contains it). Do not leave a copy behind, and do not delete the reasoning — it is a live review decision from 3.1 #143.
  - [x] **Leave the reset effect exactly as it is** (lines 105–114, including the latest-ref pair and its comment). It is correct, it is finding #146, and AC-5 depends on it. The fix makes `reset()` harmless, it does not make it wrong.
  - [x] **Leave `stop.mutate()` alone** (line 210). It has no lock today and needs none: it sits behind an `AlertDialog` confirm, which is its own double-fire guard, and Radix unmounts the dialog on action. Adding a lock there is scope creep. *(Note for the browser pass: this is also why "עצור" keeps working even on a poisoned panel — the panel is not 100% dead today, only its primary CTA is.)*
  - [x] **Nothing else in this file changes.** Not `primaryActionByState`, not `stoppableStates`, not the `finished` branch, not the `aria-live="polite"` CTA announcer, not the error-banner precedence (`gradingIncomplete` before `anyConflict` — that ordering is load-bearing, `httpapi/errors.go` makes GRADING_INCOMPLETE a 409), not the persistent-button comment block, not the `DisplayControls` optional chaining.

- [x] **Task 3: the other two call sites — the second bug, and the one that is not broken yet** (AC: 3, 9)

  **3a — `web/src/features/live/display-controls.tsx`: the second, unrecorded instance.**

  This file has the identical defect and it is **not** in `deferred-work.md`. Tick "הפחת אנימציות", let a state change land before the PUT settles, and the checkbox is dead for the rest of the game. Because the checkbox is `checked={reducedMotion}` off the snapshot with no local echo, a dead control reads to the Organizer as "the toggle does nothing", which is the worst possible failure mode for a control whose whole job is to be believed.

  - [x] Replace the `submittingRef` + `submit` block (lines 54–63) with:
    ```tsx
    const submit = useSingleFlight(setReducedMotion)
    ```
  - [x] `onChange={(event) => submit(event.target.checked)}` is unchanged — the hook's parameter infers as `boolean`.
  - [x] Drop the now-unused `useRef` import **only if** unused — `mutationRef` still uses it, so it stays.
  - [x] **Leave the `[gameState]` reset effect as-is** (lines 72–78), same reasoning as Task 2.
  - [x] Everything else — `popupBlockedIn`, `openDisplay`'s named target and missing `noopener` (both explicit 4.1 review decisions with recorded reasoning), the local error banner, the native checkbox choice — is untouched.

  **3b — `web/src/features/lobby/lobby-page.tsx`: not broken, converted anyway.**

  This file has no reset effect, so its `onSettled` always fires and its lock never strands. It is converted so the third copy of the pattern cannot become the fourth bug, and so one test covers every call site (derived requirement #9). **Say this at review** — a reviewer seeing a working file changed deserves the reason.

  - [x] Replace the `startSubmittingRef` + `fireStart` block (lines 42–47) with:
    ```tsx
    const fireStart = useSingleFlight(startGame)
    ```
  - [x] **`onClick={fireStart}` on line 131 must become `onClick={() => fireStart()}`.** This is the one site hit by the `void`-parameter edge in Task 1 — leaving it as-is is a **compile error**, not a style question.
  - [x] `useSpaceAction(fireStart, …)` on line 48 is unchanged and compiles as-is (verified). It must stay **above** the early returns.
  - [x] Drop `useCallback` from the imports if nothing else uses it; `useRef` goes too if `startSubmittingRef` was its only user — check before deleting, `noUnusedLocals` punishes both directions.
  - [x] **`openLobby.mutate()` (line 81) keeps `disabled={openLobby.isPending}` and gets no lock.** That button is not the persistent primary CTA, carries no focus-retention requirement, and is on a branch (`draft`) the live state machine never returns to. Leave it.
  - [x] Nothing else changes — not the `notFound` branch, not the `!snapshot` branch, not the `bdi` join-code/platform-number markup, not `DisplayControls`' optional chaining.

- [x] **Task 4: the project's first frontend test framework** (AC: 7, 10)

  Approved by Avraham (2026-08-09) and pre-sanctioned by architecture § Structure Patterns. Keep it minimal: this is a regression guard, not a testing strategy.

  - [x] `npm i -D vitest jsdom @testing-library/react @testing-library/dom` in `web/`. Pinned-at-time-of-writing: `vitest@4.1.10` (peer `vite: ^6 || ^7 || ^8` — the project is on Vite 8.1, so v4 is the right major), `jsdom@30.0.1`, `@testing-library/react@16.3.2` (peer `react: ^18 || ^19`). **`@testing-library/dom` must be installed explicitly** — RTL v16 moved it from `dependencies` to `peerDependencies`.
  - [x] `web/vite.config.ts` — change the import to `vitest/config` and add a `test` block as a **sibling of `plugins`/`resolve`/`server`**. One config file, not two: the `@` alias and the react plugin are exactly what the test run needs, and duplicating them in a `vitest.config.ts` is how they drift. The result in full:
    ```ts
    import path from 'node:path'
    import { defineConfig } from 'vitest/config'
    import react from '@vitejs/plugin-react'
    import tailwindcss from '@tailwindcss/vite'

    // https://vite.dev/config/
    export default defineConfig({
      plugins: [react(), tailwindcss()],
      resolve: {
        alias: {
          '@': path.resolve(__dirname, './src'),
        },
      },
      server: {
        proxy: {
          '/api': 'http://localhost:8080',
          '/webhooks': 'http://localhost:8080',
          '/ws': {
            target: 'http://localhost:8080',
            ws: true,
          },
        },
      },
      test: {
        environment: 'jsdom',
        include: ['src/**/*.test.{ts,tsx}'],
      },
    })
    ```
    - [x] **`globals` stays off** (the Vitest default). Every test imports `describe`/`it`/`expect`/`vi` from `'vitest'` explicitly, matching this codebase's explicit-imports style and keeping `tsconfig.app.json`'s `"types": ["vite/client"]` untouched.
    - [x] If `import { defineConfig } from 'vitest/config'` fails to typecheck under `tsconfig.node.json`'s `module: nodenext`, the fallback is to keep importing from `'vite'` and add `/// <reference types="vitest/config" />` as the file's first line. Record which one you used.
  - [x] `web/package.json` — add `"test": "vitest run"` to `scripts`. **`vitest run`, not `vitest`** — the bare command is watch mode and would hang CI forever.
  - [x] `Makefile` — extend the `test` target so one command still runs everything:
    ```make
    ## test: backend + frontend test suites
    test:
    	cd server && go test ./...
    	cd web && npm test
    ```
  - [x] `.github/workflows/ci.yml` — one step in the existing `frontend` job, between `typecheck` and `build`:
    ```yaml
          - name: vitest
            run: npm test
    ```
    Do not touch the `backend` job, the `filter-safety` job, or either of the two bundle scans.

- [x] **Task 5: `web/src/lib/use-single-flight.test.tsx` (new) — the regression guard** (AC: 7)

  Co-located next to the source, per architecture § Structure Patterns. This is the project's first frontend test and has no local precedent to copy, so the harness is spelled out — adapt it, but do not go looking for a different shape.

  ```tsx
  import type { ReactNode } from 'react'
  import { afterEach, describe, expect, it, vi } from 'vitest'
  import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
  import { QueryClient, QueryClientProvider, useMutation } from '@tanstack/react-query'

  import { useSingleFlight } from './use-single-flight'

  // RTL's automatic cleanup only self-registers when a global afterEach
  // exists, and vitest `globals` is off here — without this line the second
  // test renders into the DOM the first one left behind.
  afterEach(cleanup)

  /** A promise whose settlement this test controls, standing in for the
   *  HTTP round trip the real mutations make. */
  function deferred() {
    let resolve!: () => void
    const promise = new Promise<void>((r) => { resolve = r })
    return { promise, resolve }
  }

  function wrapper({ children }: { children: ReactNode }) {
    // A FRESH client per test. Never import the app's shared queryClient
    // from @/lib/api — a module-level client leaks state between tests.
    // retry:false is already React Query's mutation default; stated so a
    // future default change cannot make this flaky.
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>
  }

  // Exposes both halves so a test can drive the mutation the way the real
  // reset effect does. Deliberately NOT exported: a .tsx file that exports
  // a non-component trips react-refresh/only-export-components.
  function useHarness(mutationFn: (path: string) => Promise<void>) {
    const mutation = useMutation({ mutationFn })
    return { mutation, fire: useSingleFlight(mutation) }
  }

  describe('useSingleFlight', () => {
    it('releases the lock when the mutation is reset mid-flight (story 3.11 regression)', async () => {
      const first = deferred()
      const mutationFn = vi.fn(() => first.promise)
      const { result } = renderHook(() => useHarness(mutationFn), { wrapper })

      act(() => { result.current.fire('close-question') })
      expect(mutationFn).toHaveBeenCalledTimes(1)

      // Exactly what control-page.tsx's [snapshot.state] effect does when
      // the WS frame beats the HTTP response.
      act(() => { result.current.mutation.reset() })

      await act(async () => { first.resolve() })

      act(() => { result.current.fire('reveal') })
      // The assertion that fails against the pre-fix code: the ref was
      // stranded true, so this second call issued nothing.
      await waitFor(() => expect(mutationFn).toHaveBeenCalledTimes(2))
    })

    // Story 3.1 review finding #143 — held Space / double click.
    it('drops a second call while the first is still in flight', async () => { /* … */ })

    // The uneventful path still has to work.
    it('allows the next call once the previous one settles on its own', async () => { /* … */ })
  })
  ```

  - [x] **Test 1 is the story.** Name it so a future reader knows what it guards.
  - [x] **Test 2 — AC-4 survives.** Two synchronous calls inside one in-flight window issue exactly **one** request.
  - [x] **Test 3 — the normal path.** After a call settles on its own (no `reset()`), the next call fires.
  - [x] **Test 2 — AC-4 survives.** Two synchronous calls inside one in-flight window issue exactly **one** request.
  - [x] **Test 3 — the normal path.** After a call settles on its own (no `reset()`), the next call fires.
  - [x] Count calls with `vi.fn()`, **not** by spying on `fetch` — the hook is under test, not the API client. Nothing in this file should import `@/lib/api`.
  - [x] `.tsx`, not `.ts` — the wrapper needs JSX, and architecture § Structure Patterns names `answer-distribution.test.tsx` as the shape.
  - [x] `tsc -b` **will** typecheck this file (`tsconfig.app.json` has `include: ["src"]`) and `eslint .` **will** lint it. Both are intended. `verbatimModuleSyntax` means `import type { ReactNode }`, and `noUnusedLocals` means no leftover imports.
  - [x] **Prove the test is a real guard, not decoration** (AC-7): stash the Task 1–3 changes (or revert the three call sites), run `npm test`, and **record the red output in the Debug Log**. A regression test that has never failed is an assumption. Then unstash and record the green.

- [x] **Task 6: `deferred-work.md` — close the entry, and add the one nobody recorded** (no product code)

  - [x] Append a **CLOSED** outcome under the 🔴 entry (`deferred-work.md:143`) in the file's established style (sub-bullet, dated, the original text left intact rather than edited). Record: which of the three candidate fixes was taken and why the other two were rejected (Dev Notes has the reasoning — do not re-derive it, cite it); that the fix is a shared `lib` hook rather than a patch at the call site, and why; and that a Vitest regression test now guards it.
  - [x] Add a **new, already-closed entry** for the second instance: `display-controls.tsx` carried the identical defect and was never recorded — found while writing this story, fixed here in the same change. Worth writing down even though it ships closed, because it is the evidence for derived requirement #9: the duplication is what let one review-created bug become two.
  - [x] Append to the 4.1 `StageProps` wiring entry (`deferred-work.md:145`): its trigger was *"story 4.2 … the natural place to decide whether this contract warrants the project's first frontend test"*, and that decision has now been **made here instead** — Vitest exists as of this story. Re-point the trigger: the `StageProps` contract itself is still unexercised (this story tests a `lib` hook, not the display), so what remains is *writing* a test, not *choosing a framework*. **New trigger: story 4.3**, which is still the first stage with unit-testable logic.
  - [x] Confirm in writing that the **3.1 `seq`-ordering entry** and the **3.1 "control mutations discard the REST response snapshot" entry** are **still open and untouched** by this story. They are adjacent enough that a later reader will assume this story closed them; say plainly that it did not, and that a stuck-panel report caused by *those* defects would look different from this one (stale state / wrong checkbox, versus a CTA that issues no request at all).
  - [x] Do not re-triage anything else in the file.

- [x] **Task 7: quality gates, and the browser pass that is the real proof** (all ACs)

  - [x] **Frontend gates**: `npm run lint` · `npx tsc -b --noEmit` · `npm test` · `npm run build` · both filter-safety scans (source and built bundle).
    - [x] `tsc -b` **will** typecheck the new `.test.tsx` — `tsconfig.app.json` has `include: ["src"]`. That is intended: a test that does not typecheck is not a guard. If a testing-library type does not resolve, fix the import, do not exclude the file.
    - [x] `npm run build` must stay clean of test code in the bundle. Vite only bundles the entry graph, so nothing imports the test — but confirm `dist/assets/*.js` contains no `vitest` string.
  - [x] **Backend, unchanged**: `go build ./... && go test ./...` once to prove zero regression, then confirm `git status` shows **no `.go`, no `.sql`, no `migrations/` diff**.
  - [x] **No Go E2E harness this time.** Unlike 2.1–4.1, this story's defect is entirely client-side and a `cmd/e2escratch` run would prove nothing the browser pass does not — the server already answers all six actions correctly today. Skip it deliberately, and say in the Debug Log that you skipped it and why, rather than leaving a reader to wonder.
  - [x] **Manual browser pass** (`make dev`, real browser) — this is the only test that exercises the actual WS-beats-HTTP race:
    1. **AC-1, the headline** — log in, create a game with **three** questions, open the lobby, start it. Then drive the entire machine with the mouse only, **never reloading**: `סגור שאלה` → `גלה תשובה` → `שאלה הבאה` → … through all three questions → `finished`. Every press must issue a request. Watch the Network tab: **one POST per press, zero silent presses.** Two questions is not enough — the original bug appears on the *second* press.
    2. **AC-1 via keyboard** — repeat the run using only Space from `document.body`. Same result.
    3. **AC-4** — hold Space down through a transition (OS auto-repeat), and separately double-click the CTA fast. Exactly **one** POST per intended action. If you see two, `useSpaceAction`'s `event.repeat` guard or the lock is broken.
    4. **AC-6** — confirm the primary button is never `disabled` mid-request and keeps DOM focus across a transition (`document.activeElement` stays the button). This is 3.1's AC-4 and it is easy to break by accident here.
    5. **AC-3** — tick "הפחת אנימציות", and within the same second press the primary CTA so the state change lands during the PUT. Then tick the checkbox again: **it must still respond.** Re-tick a few times and confirm the checkbox tracks the persisted value. *(If the checkbox shows the wrong value after a dropped frame, that is the separate, still-open 3.1 entry — note it, do not fix it.)*
    6. **AC-5** — force a stale banner: press the CTA twice fast enough that the second lands after the first succeeded (or drive a 409 by acting from a second tab). The banner appears, and **clears** on the next state advance.
    7. **"עצור" still works** — from `question_open`, open the confirm dialog, Escape closes it (and does not open one when none is open), and confirming actually ends the game.
    8. **Reconnect** — kill the server mid-game, restart it, then press the CTA. The panel must still respond; the lock must not have been left set by the disconnection.
    9. Zero console errors throughout, and zero requests to any external host.
  - [x] **If story 4.1's or 3.10's manual passes are still outstanding, this is not the session for them** — unlike 4.2, this story does not put you in front of those surfaces. Leave them.

### Review Findings

Code review 2026-08-11 (three parallel layers: Blind Hunter, Edge Case Hunter, Acceptance Auditor), plus independent re-verification of every gate and of the story's central technical claim against the installed `@tanstack/query-core` source.

- [x] **[Review][Patch] The Debug Log's jsdom explanation is factually wrong, and the real reason is worth recording** — *applied 2026-08-11: the Task 4 note now records the engine constraint and the dev/CI Node skew.* — Task 4's note claims *"`npm i -D jsdom` took the current latest; there is no jsdom 30 on the registry."* Both halves are false: jsdom `30.0.0` published 2026-07-27 and `30.0.1` on 2026-07-29, and `30.0.1` is the registry's `latest` tag. The actual cause is an **engine constraint**: jsdom 30 requires node `^22.22.2 || ^24.15.0 || >=26.0.0`, and this dev machine runs **Node 25.6.1**, which satisfies none of those ranges — so npm correctly fell back to 29.1.1. The outcome is benign and CI is deterministic (`npm ci` installs the lockfile's 29.1.1 on Node 22.x), but the permanent record currently states a checkable falsehood, and the real cause exposes an undocumented dev-vs-CI Node skew (dev on non-LTS 25.6.1, CI on 22.x — [ci.yml:83](.github/workflows/ci.yml#L83)) that a future dependency bump will hit again. [3-11-the-control-panel-that-never-goes-dead.md, Debug Log → Task 4]
- [x] **[Review][Patch] `useSingleFlight` has no `try/catch` around the initial call, so a synchronous throw would strand the lock permanently** — *applied 2026-08-11: the `mutateAsync` call is wrapped, with the catch releasing the lock and a comment recording why the path is unreachable today.* — the hook's whole purpose is that the release cannot be prevented, but the release is only armed *after* `mutateAsync(variables)` returns a thenable. If that call ever threw synchronously, `inFlightRef` would stay `true` for the component's lifetime — the exact failure this story exists to eliminate, reintroduced through a different door. Unreachable against TanStack Query 5.101.2 (`Mutation.execute()` is an `async function`, and `MutationObserver.mutate()`'s synchronous statements have no realistic throw path), so this is defensive, not a live defect. Cheap insurance given derived requirement #8's spirit. [web/src/lib/use-single-flight.ts:40-51]
- [x] **[Review][Defer] The returned callback's identity changes on every render, churning `useSpaceAction`'s global keydown listener** — deferred, pre-existing. `useCallback(..., [mutation])` never stabilizes because `useMutation` returns a fresh object each render, so `fireStart` is new every render and [use-space-action.ts:26](web/src/lib/use-space-action.ts#L26)'s `[onFire, enabled]` effect tears down and re-adds the `window` keydown listener on every render — including every WebSocket frame. **Not a correctness bug** (teardown and re-add are synchronous within one commit, so no keypress can be lost) and **not introduced here** (the old `useCallback(..., [action])` had identical churn). Task 1 explicitly declined to fix it and forbade the latest-ref approach. Recorded because a cleaner fix exists that the story did not consider and that does *not* violate that prohibition: depend on `[mutation.mutateAsync]` instead of `[mutation]`. `MutationObserver.bindMethods()` binds `mutate` once in the constructor and `#updateResult()` passes that same reference through, so `mutation.mutateAsync` is stable for the observer's lifetime — making the callback genuinely stable with a one-token change and no ref. [web/src/lib/use-single-flight.ts:36,53]
- [x] **[Review][Defer] `make test` and `make build` assume `web/node_modules` is already populated** — deferred, pre-existing. The new `cd web && npm test` line has no install prerequisite, so `make test` on a fresh clone fails at the vitest binary. CI is unaffected (it runs `npm ci` as its own step, [ci.yml:86-87](.github/workflows/ci.yml#L86-L87)). Not introduced by this story: `make build`'s `cd web && npm run build` already had the identical shape, so this is the Makefile's established convention rather than a new gap. [Makefile:23-25]

**Dismissed as noise (4):** an unused-`useRef` claim in `display-controls.tsx` (false positive — still imported and used at [display-controls.tsx:1](web/src/features/live/display-controls.tsx#L1), and `npm run lint` is clean); "test 1 asserts the end state, not the release mechanism" (fair in principle, but AC-7's actual requirement is that the test fail against the pre-fix shape, which was independently reproduced during this review); the hook's blanket `.catch()` hiding a throwing `onSuccess`/`onError` (unchanged from before — `useMutation`'s own `mutate` is `observer.mutate(...).catch(noop)`); and the docstring "one mutation" wording overclaiming versus a per-instance lock (no call site shares a mutation across two hook instances).

**Independently verified during review, not taken on the story's word:**

- **The central mechanism, read from the installed source.** `useMutation.js:40` returns `mutateAsync: result.mutate`; `mutationObserver.js:56-62` shows `mutate()` returns `this.#currentMutation.execute(variables)`; `mutationObserver.js:50-55` shows `reset()` calls `removeObserver` then `#notify()` with **no** action; and `#mutateOptions.onSettled` is reachable only from `#notify(action)` (lines 97/119). The diagnosis and the fix are both correct as described.
- **The fix's scope is complete and correctly bounded.** No `submittingRef` remains anywhere in `web/src`. The `onSettled` callbacks still present in the builder pages are *mutation-level* options, invoked from inside `execute()` (`mutation.js:123/137/159/181`) rather than through the observer — a different code path that never had this bug. Nothing else needed converting.
- **AC-7, the strongest claim, reproduced.** A throwaway negative-control test implementing the pre-fix `mutate(v, { onSettled })` shape inline (touching no existing file, deleted afterwards) failed with exactly the recorded error: `AssertionError: expected "vi.fn()" to be called 2 times, but got 1 times`. The regression guard is real.
- **Gates re-run from a clean tree:** `npm run lint` clean · `npx tsc -b --noEmit` exit 0 · `npm test` 3/3 · `npm run build` clean at 455.43 kB js / 41.13 kB css (matching the Debug Log exactly) · `grep -c vitest dist/assets/*.js` = 0.
- **Task 6 delivered in full.** The `deferred-work.md` diff is pure addition (no `-` lines), closes the 🔴 entry with the chosen approach and all three rejections, adds the `display-controls.tsx` instance, re-points the `StageProps` trigger to 4.3, and marks both adjacent 3.1 entries explicitly still-open.

**Claimed but not independently verifiable:** the manual browser pass (AC-1, AC-3, AC-5, AC-6, the "עצור" dialog, and the reconnect check) rests on a Playwright session narrated in the Dev Agent Record. The code is logically consistent with the reported results, but this review confirmed it by reading, not by re-running. Treat those line items as claimed-but-unconfirmed.

## Dev Notes

### Three fixes were on the table; here is why this one

`deferred-work.md` named three candidates. A fourth emerged from reading the React Query source. All four fix AC-1; they differ in what they cost.

| | Approach | Verdict |
|---|---|---|
| **(a)** | Also set `submittingRef.current = false` inside the reset effect | **Rejected.** Two lines, and it works *today*. But it couples banner hygiene to the submit lock, and it only covers the one `reset()` that currently exists. The next `reset()` — a `gameId` guard, a new banner rule, a fourth surface copying the pattern — silently reintroduces the bug. It also *changes semantics*: the lock would drop the instant the state advances, while a request is still in flight, which is not what the 3.1 comment says the guard does. |
| **(b)** | Key the reset off the mutation instead of `snapshot.state` | **Rejected.** It does not address the lock at all, and it breaks AC-5: the banner's whole purpose is to expire when the *live state* moves on, which is the only signal that says "the panel self-healed". |
| **(c)** | Skip `reset()` while the mutation is pending | **Rejected, but it is the honourable runner-up.** It is a genuine root-cause fix — never detach an observer whose callbacks you depend on — and it is one condition. Its cost: an error that lands *after* the state advanced now raises a banner that finding #146 deliberately suppressed (it would clear at the following transition, so the damage is bounded). Rejected because it is still a rule you have to remember at every future `reset()` call site, which is derived requirement #8. |
| **(d)** | Release the lock from the mutation's own promise (`mutateAsync().catch().finally()`) | **Chosen.** `mutateAsync` is `observer.mutate` unwrapped — it returns `Mutation.execute()`'s promise, and `removeObserver` does not cancel `execute()`. The release becomes unconditional: nothing outside the hook can prevent it. `reset()` keeps its exact current semantics, so AC-5 is untouched. And it makes the code finally do what 3.1's comment has claimed all along — hold the lock for the duration of the request, no more and no less. |

**The one honest cost of (d):** the lock is held for the whole round trip, so a press landing in the ~50–200 ms between the WS frame and the HTTP response is dropped. That window exists in today's *intended* design too (it is what the 3.1 guard was written to do), so this is a restoration, not a new cost — but it is a real one, and the browser pass's step 1 is where you would notice if it feels wrong in the hand. If it does, the follow-up is a per-action lock (`inFlightRef` holding the `path` rather than a boolean), which is deliberately **not** in this story: it changes the guarantee from "one request at a time" to "one request per action at a time", and that is a decision to make with evidence rather than pre-emptively.

### Why a `lib` hook and not three local patches

The pattern lives in three files with three near-identical comment blocks. It shipped broken in two of them. `web/src/lib/use-space-action.ts` — extracted at 3.1's review for precisely this reason — states the principle in its own docstring: *"Shared by every 'one persistent button' primary-action surface … so both get the same fix in one place."* Architecture § Component Boundaries (Web) says the same structurally: features never import each other; shared logic goes to `lib`.

`lobby-page.tsx` is converted too even though it is **not** currently broken (it has no reset effect, so its `onSettled` always fires). It is included so the third copy cannot drift into the fourth bug, and so the test covers all three call sites at once. Call this out at review — a reviewer seeing a working file changed deserves the reason.

### What must survive, file by file

- **[web/src/features/live/control-page.tsx](web/src/features/live/control-page.tsx)** — the reset effect and its latest-ref pair (3.1 #146; the refs are synced *in* an effect, not during render, because `react-hooks/refs` v7 forbids reading a ref during render); the persistent, never-`disabled` primary button (3.1 AC-4, found in 3.1's browser pass); the `gradingIncomplete`-before-`anyConflict` precedence (GRADING_INCOMPLETE is a 409, so the generic branch would shadow it); the `aria-live="polite"` CTA announcer; the `finished` early return placed *after* all hooks; the `DisplayControls` optional chaining (4.1 review — a redeploy briefly runs two instances, and there is no error boundary).
- **[web/src/features/live/display-controls.tsx](web/src/features/live/display-controls.tsx)** — the `[gameState]` reset effect; `popupBlockedIn` stored as a state value rather than a boolean + effect (avoids `react-hooks/set-state-in-effect`); `openDisplay`'s named target and deliberate absence of `noopener` (4.1 review: `noopener` forces `_blank` and makes the null return meaningless); the local error banner (kept out of ControlPage's already over-multiplexed one, per the 3.10 deferred entry).
- **[web/src/features/lobby/lobby-page.tsx](web/src/features/lobby/lobby-page.tsx)** — `useSpaceAction(fireStart, snapshot?.state === 'lobby' && snapshot.questionCount > 0)` sits **above** the early returns and must stay there; `disabled={snapshot.questionCount === 0}` is a real, non-transient reason and stays; `openLobby.mutate()` keeps its `disabled={openLobby.isPending}` — that button is not the persistent primary CTA and does not carry the focus-retention requirement, so leave it alone.
- **[web/src/lib/use-space-action.ts](web/src/lib/use-space-action.ts)** — **unchanged.** The `event.repeat` guard and the `document.activeElement !== document.body` check are both live 3.1 review fixes.

### The two adjacent bugs this story does *not* fix

Both are in `deferred-work.md`, both are about the control panel, and both will be mistaken for this one by a future reader:

1. **Control mutations discard the REST response snapshot** (3.1 entry, re-triaged at 4.1). A dropped WS frame leaves the panel on stale state. **Symptom:** the CTA fires, the request returns 200 or 409, but the panel does not advance. **This story's symptom is the opposite:** no request is issued at all. Fixing it means applying the returned snapshot into the socket store — an architecture change with its own story.
2. **Broadcast `seq` is assigned at `Broadcast()`-call time, not commit order** (3.1 entry, plus 2.4's sibling). **Symptom:** the room regresses to a stale state and sticks. Untouched here; note that `games.updated_at` is no longer a valid commit-ordered source for its fix (4.1 entry, `deferred-work.md:144`).

### Testing standards

Go stdlib `testing`, co-located `_test.go`, stubs grown in place — no mock framework. **This story adds no Go code and therefore no Go tests**; run the suite only to prove zero regression. `go test -race` is unavailable here (`CGO_ENABLED=0`, per 3.8–4.1's Change Logs) — say so rather than implying race coverage.

Frontend: this story **introduces** Vitest, co-located, per architecture § Structure Patterns. Keep the footprint honest — one config block, one script, one CI step, one test file. Do **not** take the opportunity to backfill tests for anything else; every other frontend surface stays verified by its browser pass, and a sprawling first test suite is how a framework addition becomes the story.

The browser pass in Task 7 remains the primary verification for AC-1, AC-3, AC-5 and AC-6 — the Vitest test covers AC-2 and AC-4 only, because the WS-beats-HTTP race that triggers the bug in production cannot be faithfully simulated in jsdom.

### Project Structure Notes

**New:**
- `web/src/lib/use-single-flight.ts` — shared logic in `lib`, per architecture § Component Boundaries (Web)
- `web/src/lib/use-single-flight.test.tsx` — co-located, per architecture § Structure Patterns

**Modified:**
- `web/src/features/live/control-page.tsx` (adopt the hook, delete the local ref)
- `web/src/features/live/display-controls.tsx` (same)
- `web/src/features/lobby/lobby-page.tsx` (same; not currently broken)
- `web/vite.config.ts` (+`test` block, import from `vitest/config`)
- `web/package.json` (+4 devDeps, +`test` script)
- `.github/workflows/ci.yml` (+1 step in the `frontend` job)
- `Makefile` (`test` target also runs the frontend suite)
- `_bmad-output/implementation-artifacts/deferred-work.md` (Task 6)

**Untouched (a diff here means you went off-spec):** all of `server/**` · `server/migrations/**` · `web/src/lib/use-game-socket.ts`, `api.ts`, `types.ts`, `strings.he.ts`, `text.ts`, `use-space-action.ts` · `web/src/app.tsx` · `web/src/index.css` · `web/src/components/**` · `web/src/features/display/**`, `features/builder/**`, `features/results/**`, `features/auth/**` · `web/index.html`.

### References

- The defect: [deferred-work.md:143](_bmad-output/implementation-artifacts/deferred-work.md) (the 🔴 entry, with the observed click sequence and the `main`-branch reproduction), and [4-1-audience-display-shell-the-screen-that-follows-the-game.md:766](_bmad-output/implementation-artifacts/4-1-audience-display-shell-the-screen-that-follows-the-game.md) + its Change Log entry for 2026-08-09
- The two review findings that created it: [3-1-run-the-game-live-control-state-machine.md:143](_bmad-output/implementation-artifacts/3-1-run-the-game-live-control-state-machine.md) (#143, the ref guard) and [:146](_bmad-output/implementation-artifacts/3-1-run-the-game-live-control-state-machine.md) (#146, the reset effect)
- Broadcast-before-response, all six handlers: [server/internal/httpapi/control.go:212](server/internal/httpapi/control.go#L212), 235, 259, 282, 306, 336
- Requirement: [epics.md § Story 3.1](_bmad-output/planning-artifacts/epics.md#L455-L480) — one primary CTA per state, every advance an explicit Organizer action (FR-13, UX-DR10); AC-4 there is the focus-retention/Space requirement this story must not break
- Behaviour: EXPERIENCE.md § *Host control panel* — *"one persistent button whose label and action swap in place — focus is retained across state changes"*
- Architecture: § *Structure Patterns* (co-located Vitest tests), § *Component Boundaries (Web)* (shared logic promoted to `lib`), § *Process Patterns* → Loading states (and note this story's documented deviation from *"mutations disable their trigger button"*, forced by 3.1 AC-4)
- Adjacent, still open: [deferred-work.md](_bmad-output/implementation-artifacts/deferred-work.md) — the 3.1 "discard the REST response snapshot" and "`seq` at Broadcast()-call time" entries, and 4.1's `updated_at` entry
- Prior stories: 3.1 (the control state machine and both review findings), 4.1 (found the bug; created `display-controls.tsx`)

### Latest technical information

Verified against the installed tree and npm on 2026-08-09.

- **`@tanstack/react-query` 5.101.2** — the mechanism is not a documented API contract, it is source behaviour, so it is worth pinning here. `useMutation.js` returns `{ ...result, mutate, mutateAsync: result.mutate }` where `mutate` is `(v, o) => observer.mutate(v, o).catch(noop)` and `result.mutate` is the bound `observer.mutate`. `observer.mutate` returns `Mutation.execute(variables)`. `observer.reset()` runs `currentMutation.removeObserver(this)`, and `execute()` is unaffected by observer removal. **This is why (d) works and (a)/(c) are workarounds.** A React Query major upgrade should re-check `mutationObserver.js`.
- **Vitest 4.1.10** — peer `vite: ^6.0.0 || ^7.0.0 || ^8.0.0` (the project is on Vite 8.1.1) and Node ≥ 20 (CI uses 22.x; this machine has 25.6.1). `environment: 'jsdom'` is still the config shape and `jsdom` is still a separate install. `globals` remains opt-in. `workspace` is gone in v4 in favour of `projects` — irrelevant here, but do not copy a v3 config off the internet.
- **`@testing-library/react` 16.3.2** — React 19 supported; `@testing-library/dom` is a **peer**, not a dependency, so install it explicitly. With `globals: false`, RTL's automatic cleanup does not self-register — call `afterEach(cleanup)` in the test file.
- **`jsdom` 30.0.1.**
- **TypeScript 6.0 / `tsconfig.app.json`** — `verbatimModuleSyntax` (use `import type`), `noUnusedLocals` + `noUnusedParameters` (an import left behind after removing the local ref is a build failure), `erasableSyntaxOnly`, `strict`. `include: ["src"]` means the new test file is typechecked by `tsc -b`, which is intended.
  - **`strictFunctionTypes` + a `void` parameter** is the one trap in this refactor, and it was confirmed by running `tsc` rather than reasoned about: a `(v: void) => void` is callable with no arguments and assignable to `() => void`, but **not** assignable to `(e: MouseEvent) => void`. So `useSpaceAction(fireStart, …)` compiles unchanged while `onClick={fireStart}` does not. Wrap DOM handlers: `onClick={() => fireStart()}` (Task 3b).
- **`eslint-plugin-react-hooks` 7.1** — `react-hooks/refs` (no ref reads during render) and `react-hooks/set-state-in-effect` are both enforced. `useSingleFlight` writes its ref inside a callback, never reads one during render, and sets no state, so it is clean by construction. Do not "simplify" the reset effects in Tasks 2 and 3 into shapes these rules reject.

### Questions for Avraham (raise at code review, not before)

1. **The ~50–200 ms swallowed-press window** (Dev Notes → the honest cost of (d)). Restoring the intended behaviour is the right default, but it is worth one sentence of confirmation after you have felt it in the browser pass. If it reads as sticky, the per-action lock is the follow-up.
2. **`epics.md` was not amended** for this injected story. If the epic file should carry 3.11, that is a `correct-course` run.
3. **The Vitest footprint stops here on purpose** — one hook, one test file. Confirm you do not want backfill tests for other frontend surfaces in this story.

## Dev Agent Record

### Agent Model Used

claude-sonnet-5 (Claude Code, `bmad-dev-story`), 2026-08-10.

### Debug Log References

**Branch base changed mid-story, deliberately.** The story's `baseline_commit` is `4b55278` (the tip of `story/4-1-audience-display-shell`) and the branch was cut from there, as the prerequisite section requires. Avraham then merged 4.1 into `main` (PR #9, `70aff1d`) while this story was in flight, so the branch was re-based onto `origin/main` — the story's own instruction ("branch from `story/4-1-audience-display-shell`, **or from `main` once 4.1 has merged**"). `git diff 4b55278 origin/main` is empty: identical trees, so the move changed history only, never content. `baseline_commit` is left at `4b55278` as recorded.

**Task 4 — two deviations from the story's pinned versions, both benign.**
- `jsdom` resolved to **29.1.1**, not the `30.0.1` the story names. ~~`npm i -D jsdom` took the current latest; there is no jsdom 30 on the registry.~~ **Corrected at code review (2026-08-11) — that explanation was wrong on both counts.** jsdom `30.0.0` published 2026-07-27 and `30.0.1` on 2026-07-29, and `30.0.1` is the registry's `latest` tag, so it existed and *was* the latest at implementation time. The actual cause is an **engine constraint**: jsdom 30 declares `engines.node = "^22.22.2 || ^24.15.0 || >=26.0.0"`, and this dev machine runs **Node 25.6.1**, which satisfies none of those ranges — npm correctly resolved down to 29.1.1, the newest release that accepts Node 25. Nothing depends on the major and the outcome is unchanged, but the reason matters for the next dependency bump: **the dev machine is on non-LTS Node 25.6.1 while CI pins Node 22.x** (`.github/workflows/ci.yml`), so local `npm i` and CI can legitimately resolve differently. CI itself is deterministic regardless — it runs `npm ci`, which installs the lockfile's 29.1.1 exactly.
- The `tsconfig.node.json` fallback was **not needed**. `import { defineConfig } from 'vitest/config'` typechecks clean under `module: nodenext` — `npx tsc -b --noEmit` exits 0 — so `vite.config.ts` imports from `vitest/config` directly with no `/// <reference types="vitest/config" />` line. Recording which one was used, as the subtask asked.

**Task 5 — the red run, and one correction to the story's test harness.** The story's snippet asserts `expect(mutationFn).toHaveBeenCalledTimes(1)` synchronously after `act(() => fire(...))`. That fails against the *fixed* code too: React Query reaches the `mutationFn` asynchronously (`execute()` awaits `onMutate` first), so nothing has been called yet at that point. The harness gained a `flush()` helper — one `setTimeout(0)` macrotask boundary inside `act`, which drains every queued microtask — used after each press and after each settle. It is a determinism device, not a `sleep`-based assertion: after the boundary the lock has either been released or never will be.

The regression guard was then demonstrated red against the pre-fix shape (the hook body temporarily reverted to `mutation.mutate(v, { onSettled: release })`, everything else identical):

```
FAIL  src/lib/use-single-flight.test.tsx > useSingleFlight > releases the lock when the mutation is reset mid-flight (story 3.11 regression)
AssertionError: expected "vi.fn()" to be called 2 times, but got 1 times
 ❯ src/lib/use-single-flight.test.tsx:75:44
 Test Files  1 failed (1)
      Tests  1 failed | 2 passed (3)
```

Exactly the right shape of red: **only** test 1 fails. Tests 2 and 3 pass against both implementations, which is the point — they are the guarantees (AC-4's double-fire collapse, and the ordinary settle path) that the pre-fix code already honoured and that this fix must not trade away. With the hook restored: `Test Files 1 passed (1) / Tests 3 passed (3)`.

**Task 7 — gates.** `npm run lint` clean · `npx tsc -b --noEmit` exit 0 · `npm test` 3/3 · `npm run build` clean (455.43 kB js / 41.13 kB css) · filter-safety clean on both source and built bundle · `grep -c vitest dist/assets/*.js` = **0** (no test code in the bundle) · `go build ./...` clean · `go test ./...` all packages ok. `npx tsc -p tsconfig.app.json --listFiles` confirms `use-single-flight.test.tsx` is genuinely in the typechecked program, not merely assumed to be.

`git status` shows **zero `.go`, zero `.sql`, zero `migrations/` diff** — the frontend-only boundary held.

**No Go E2E harness this time, deliberately.** Unlike 2.1–4.1 there is no `cmd/e2escratch` run for this story. The defect is entirely client-side: the server already answers all six control actions correctly today, and 4.1's own browser pass proved it (`reveal:200` after a reload, from the identical click). An E2E harness driving the REST API directly would exercise the one side of the race that was never broken, and would pass just as happily against the pre-fix code. The browser pass is the only test that puts a real WebSocket frame in front of a real HTTP response.

**Environment note for the browser pass.** The dev Postgres runs in Docker and the Docker engine was down (`com.docker.service` = Stopped; starting it needs elevation this session does not have). Avraham started it (2026-08-10). Driven headless via `playwright-core` + `channel: 'msedge'`, against `make dev`'s two processes, with a scratch organizer (`story-3-11-pass`) provisioned through `cmd/provision` so Avraham's own account and sessions were never touched.

**Browser pass — 17/17, plus a negative control and three follow-ups.** Every press was counted at the network layer, because this defect's only signature is a request that never happens.

| Step | AC | Result |
|---|---|---|
| 1 — mouse, 3 questions, no reload | AC-1 | 9 presses → 9 POSTs, all 200, reached `finished` |
| 2 — Space from `document.body` | AC-1 | 10 presses → 10 requests, reached `finished` |
| 3 — double click / held Space | AC-4 | exactly 1 request each |
| 4 — never `disabled`, focus retained | AC-6 | never disabled mid-request; `document.activeElement` still the button (`BUTTON\|סגור שאלה`) after the transition |
| 5 — reduce-animations raced by a state change | AC-3 | checkbox still accepted every later toggle (4 PUTs) |
| 6 — stale banner | AC-5 | see below — re-run properly |
| 7 — "עצור" | — | Escape closes the dialog (and is a no-op with none open); confirming ends the game |
| 8 — reconnect | — | see below |
| 9 — console / external hosts | — | zero console errors, zero external requests |

**The negative control is the part that makes the green meaningful.** A passing browser run proves nothing unless the harness can detect the bug, so step 1 was re-run with the hook body temporarily reverted to `mutation.mutate(v, { onSettled })` and nothing else changed:

```
=== PRE-FIX CODE ===
presses attempted: 9
requests issued: 6 -> [..., open-lobby, start, close-question]
SILENT/STUCK:
  reveal: press issued NO request
  next-question: CTA never appeared (panel stuck)
RESULT: HARNESS WENT RED
```

That is `deferred-work.md`'s reported sequence exactly — `close-question` works and poisons the ref, `reveal` is swallowed, the panel is stuck — reproduced in a real browser against a real server. With the hook restored the identical run issues all 9 requests.

**Step 6 (AC-5) was re-run because the first attempt was vacuous.** The main script tried to force a 409 by pressing twice quickly; with the fix in place the single-flight lock correctly prevents the second press, so no 409 ever occurred and "the banner cleared" was true only because no banner was ever raised. Re-driven deterministically instead: tab 1's WebSocket was intercepted with `page.routeWebSocket` and its inbound frames frozen, tab 2 advanced the game, and tab 1's now-genuinely-stale CTA was pressed — `close-question:409`, banner raised reading **"מצב המשחק השתנה בינתיים. הלוח יתעדכן אוטומטית."**, and the banner cleared once the frames were unfrozen and the live state advanced. A follow-up check confirms the panel is still usable after the 409 (`next-question:200`) — the lock releases on the error path too, which is `.catch().finally()` doing its job.

**Step 8 (reconnect)** was run on a plain page with no WebSocket interception, so a failure would have been the product's: mid-game, the server was killed and restarted, `/api/health` observed to flap, and the next press answered `reveal:200`. The lock was not left set by the disconnection. The 19 console errors during that run are outage noise (502s and failed WS handshakes while the port is dead), not application errors — the clean run reports zero.

**One caveat on the AC-3 evidence.** Step 5 proves the checkbox keeps *accepting* toggles after a state change lands mid-PUT, which is what this story fixes. It does not prove the checkbox always *displays* the persisted value — that is the separate, still-open 3.1/4.1 entry about mutations discarding the REST response snapshot, and Task 6 restates that it stays open.

### Completion Notes List

- **The root cause is gone, not patched around.** `useSingleFlight` releases its lock from `mutateAsync()`'s own promise — the `Mutation.execute()` promise, which `removeObserver` cannot cancel — so no future `reset()` from any effect, for any reason, at any call site can strand it. That was derived requirement #8, and it is why candidate (a) (clear the ref in the reset effect) was rejected despite being two lines: it would have worked today and broken again at the next `reset()`.
- **The reset effects were left exactly as they are** in both files, comments included. Finding #146 is still live and AC-5 still depends on it; the fix makes `reset()` harmless rather than making it wrong.
- **The defect existed twice and only one instance was recorded.** `display-controls.tsx` carried it identically and appears in no deferred-work entry. On a controlled checkbox it is worse than on a CTA: a dead toggle reads as "this control does nothing", which is the worst failure mode for a control whose only job is to be believed. Both are fixed by the one hook, which is derived requirement #9's whole argument.
- **`lobby-page.tsx` was converted although it was never broken** (no reset effect, so its `onSettled` always fired). Flagging it for review explicitly, since a reviewer will see a working file changed: it is the third copy of the pattern, and the reason this bug shipped twice is that the pattern was copied rather than shared. One test now covers all three call sites.
- **AC-4 and AC-6 were treated as things to preserve, not to re-litigate.** The ref-guard reasoning from 3.1 #143 moved into the hook verbatim rather than being deleted; the primary button is still never `disabled`; `use-space-action.ts` is untouched. Vitest test 2 and the browser pass's held-Space/double-click checks both pin the double-fire guarantee, and — deliberately — both pass against the pre-fix code too. Only test 1 goes red. That is the correct shape: the fix must close the deadlock without trading away either guarantee that created it.
- **The one honest cost, unchanged from the story's Dev Notes:** the lock is now held for the whole round trip, so a press landing in the ~50–200 ms between the WS frame and the HTTP response is dropped. In the browser pass this never registered as sticky at ordinary pressing speed, but it is a real property and it is Question 1 for review.
- **Scope held.** Zero `.go`, zero `.sql`, zero `migrations/` diff — verified with `git status`, not assumed. No Hebrew copy changed, no new UI, no `components/ui/*` edit, no `use-game-socket.ts` change, and all seven mutations remain `api<void>`/`api<unknown>`.
- **Two adjacent bugs stay open and are restated as open in `deferred-work.md`** (Task 6): control mutations discarding the REST response snapshot, and `seq` being assigned at `Broadcast()`-call time. A future reader will assume this story closed them. It did not, and their symptoms differ — those show a panel on stale state or a checkbox showing the wrong value; this one showed a CTA issuing no request at all.

### File List

**New**
- `web/src/lib/use-single-flight.ts` — the shared lock, released by the mutation's own promise
- `web/src/lib/use-single-flight.test.tsx` — the regression guard (the project's first frontend test)

**Modified**
- `web/src/features/live/control-page.tsx` — adopts the hook; local `submittingRef`/`fire` and the now-unused `useCallback` import removed
- `web/src/features/live/display-controls.tsx` — adopts the hook (the second, unrecorded instance of the defect)
- `web/src/features/lobby/lobby-page.tsx` — adopts the hook (not broken; converted anyway); `onClick={() => fireStart()}`; `useCallback`/`useRef` imports removed
- `web/vite.config.ts` — imports from `vitest/config`, adds the `test` block
- `web/package.json` — 4 devDeps, `test` script
- `web/package-lock.json` — lockfile for the above
- `Makefile` — the `test` target now runs the frontend suite too
- `.github/workflows/ci.yml` — one `vitest` step in the existing `frontend` job
- `_bmad-output/implementation-artifacts/deferred-work.md` — Task 6
- `_bmad-output/implementation-artifacts/sprint-status.yaml` — story status
- `_bmad-output/implementation-artifacts/3-11-the-control-panel-that-never-goes-dead.md` — this file

**Untouched, as the scope boundaries require:** all of `server/**` (zero `.go`, `.sql`, `migrations/` diff), `web/src/lib/use-game-socket.ts`, `api.ts`, `types.ts`, `strings.he.ts`, `text.ts`, `use-space-action.ts`, `web/src/components/**`, `web/src/features/display/**`.

## Change Log

- **2026-08-10 — Story 3.11 implemented.** The live control panel's in-flight lock moved out of three copied local refs and into one shared `lib` hook, `useSingleFlight`, which releases the lock from the mutation's own `mutateAsync()` promise instead of a per-call `onSettled`. This closes the deadlock between story 3.1's two review findings (#143's ref guard and #146's `reset()` effect), which were each correct alone and together left the primary CTA silently swallowing every press after the first. Adopted at all three call sites: `control-page.tsx` (broken), `display-controls.tsx` (broken, and previously unrecorded), and `lobby-page.tsx` (not broken; converted so the third copy cannot become the fourth bug). Both `reset()` effects, the never-`disabled` primary button and `use-space-action.ts` are unchanged.
- **2026-08-10 — Vitest added, minimally.** The project's first frontend test framework: four devDeps, a `test` block inside the existing `vite.config.ts` (not a second config file), a `test` script, one CI step in the `frontend` job, and the `Makefile` `test` target extended so one command still runs everything. One test file, `use-single-flight.test.tsx`, co-located per architecture § Structure Patterns. Approved by Avraham (2026-08-09), overriding 4.2's recommendation to wait for 4.3. No backfill tests for any other surface — deliberately.
- **2026-08-10 — Branch re-based onto `origin/main` mid-story.** The branch was cut from `story/4-1-audience-display-shell` (`4b55278`) as the story's prerequisite requires; Avraham then merged 4.1 (PR #9, `70aff1d`), so the branch moved to `origin/main` per the story's own alternative. Identical trees — `git diff 4b55278 origin/main` is empty. `baseline_commit` left as recorded.
- **2026-08-10 — `deferred-work.md` updated** (Task 6): the 🔴 top-severity entry closed with the chosen fix and the three rejections; a new already-closed entry added for the second, unrecorded instance in `display-controls.tsx`; 4.1's `StageProps` trigger re-pointed to story 4.3 now that the framework question is settled; and the two adjacent 3.1 entries (`seq` ordering, discarded REST snapshot) confirmed in writing as still open and untouched.
- **2026-08-10 — Verification.** Vitest 3/3, demonstrated red against the pre-fix hook (test 1 only, which is the correct shape). Browser pass 17/17 across all nine steps, plus a negative control that reproduced the original stuck-panel sequence in a real browser and went green again with the fix. `lint` · `tsc -b` · `build` · both filter-safety scans · `go build` · `go test` all clean; zero `.go`/`.sql`/`migrations/` diff. No Go E2E harness, deliberately — the server side was never broken.
