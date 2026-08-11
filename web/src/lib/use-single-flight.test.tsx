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

/** Drains every microtask currently queued. The setTimeout(0) is a
 *  macrotask boundary, not a sleep-based assertion: React Query reaches
 *  the mutationFn asynchronously (execute() awaits onMutate first), and
 *  releases the lock a further tick later, so a press and its aftermath
 *  are only observable once the queue has drained. After this call the
 *  state is final — it either happened or it never will. */
async function flush() {
  await act(async () => {
    await new Promise<void>((resolve) => { setTimeout(resolve, 0) })
  })
}

/** Settles the deferred standing in for the HTTP round trip, then drains. */
async function settle(target: { resolve: () => void }) {
  target.resolve()
  await flush()
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
    await flush()
    expect(mutationFn).toHaveBeenCalledTimes(1)

    // Exactly what control-page.tsx's [snapshot.state] effect does when
    // the WS frame beats the HTTP response.
    act(() => { result.current.mutation.reset() })

    await settle(first)

    act(() => { result.current.fire('reveal') })
    // The assertion that fails against the pre-fix code: the ref was
    // stranded true, so this second call issued nothing.
    await waitFor(() => expect(mutationFn).toHaveBeenCalledTimes(2))
  })

  // Story 3.1 review finding #143 — held Space / double click.
  it('drops a second call while the first is still in flight', async () => {
    const first = deferred()
    const mutationFn = vi.fn(() => first.promise)
    const { result } = renderHook(() => useHarness(mutationFn), { wrapper })

    // Two activations inside one render, before React Query can flip
    // isPending — the exact timing gap the ref exists to close.
    act(() => {
      result.current.fire('close-question')
      result.current.fire('close-question')
    })
    await flush()
    expect(mutationFn).toHaveBeenCalledTimes(1)

    await settle(first)
    expect(mutationFn).toHaveBeenCalledTimes(1)
  })

  // The uneventful path still has to work.
  it('allows the next call once the previous one settles on its own', async () => {
    const first = deferred()
    const second = deferred()
    const mutationFn = vi
      .fn<(path: string) => Promise<void>>()
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise)
    const { result } = renderHook(() => useHarness(mutationFn), { wrapper })

    act(() => { result.current.fire('close-question') })
    await flush()
    expect(mutationFn).toHaveBeenCalledTimes(1)

    // No reset() this time — the lock releases through the ordinary path.
    await settle(first)

    act(() => { result.current.fire('reveal') })
    await flush()
    expect(mutationFn).toHaveBeenCalledTimes(2)

    await settle(second)
  })
})
