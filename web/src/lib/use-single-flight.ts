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
      try {
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
      } catch {
        // The release above is only armed once mutateAsync has returned a
        // thenable. If it ever threw synchronously instead, the lock would
        // stay set for the component's lifetime — this hook's own failure
        // mode, reintroduced through a different door. Unreachable against
        // query-core 5.101.2 (Mutation.execute() is an `async function`,
        // so it can only ever reject, and MutationObserver.mutate()'s
        // synchronous statements have no realistic throw path), so this is
        // a guard on the invariant rather than a live path: releasing the
        // lock must not depend on anything outside this function behaving.
        // (Story 3.11 code review, 2026-08-11.)
        inFlightRef.current = false
      }
    },
    [mutation],
  )
}
