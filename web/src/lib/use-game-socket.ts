import { useSyncExternalStore } from 'react'

import { api, ApiError } from '@/lib/api'
import type { LobbySnapshot, SnapshotEnvelope } from '@/lib/types'

const initialBackoffMs = 500
const maxBackoffMs = 10_000

/** snapshot is null before the first frame (or while reconnecting); notFound
 * means a one-shot REST probe confirmed the game doesn't exist/isn't owned
 * by this organizer — the caller should stop waiting and show a dead-end,
 * not keep rendering "connecting". */
export interface GameSocketState {
  snapshot: LobbySnapshot | null
  notFound: boolean
}

interface SocketStore {
  subscribe: (onChange: () => void) => () => void
  getSnapshot: () => GameSocketState
}

// Module-level cache keyed by "gameId:role" — the underlying WebSocket
// connection is a true external store, shared by every component watching
// the same game/role pair, and lives independently of any one component's
// render cycle (per architecture: state via useSyncExternalStore). Entries
// are never evicted: useGameSocket looks a store up fresh on every render
// (deliberately, so it never needs its own effect/memo to manage identity),
// so an entry removed here would reappear via createStore() on the very
// next render — with its own new socket — even while the original
// connection is still open and healthy. At pilot scale (a handful of games
// per organizer session) leaving idle, disconnected store objects cached is
// free; only their sockets are ever actually live.
const stores = new Map<string, SocketStore>()

function createStore(gameId: string, role: 'host' | 'display'): SocketStore {
  let state: GameSocketState = { snapshot: null, notFound: false }
  let lastSeq = -1
  let socket: WebSocket | null = null
  let backoff = initialBackoffMs
  let reconnectTimer: ReturnType<typeof setTimeout> | undefined
  let stopped = true
  // Whether the in-flight connection ever fired 'open'. A close before that
  // point means the handshake itself was rejected (or never reached the
  // server) — see the 'close' handler below for why that's classified
  // differently from a drop of a previously-live connection.
  let hasOpened = false
  const listeners = new Set<() => void>()

  const notify = () => {
    for (const listener of listeners) listener()
  }

  const setState = (next: Partial<GameSocketState>) => {
    state = { ...state, ...next }
    notify()
  }

  const scheduleReconnect = () => {
    reconnectTimer = setTimeout(() => {
      backoff = Math.min(backoff * 2, maxBackoffMs)
      connect()
    }, backoff)
  }

  // A handshake that never opened hides its HTTP rejection status from us
  // (the browser WebSocket API surfaces every pre-upgrade rejection as the
  // same opaque close event, whether it was 401/404/400 or the server being
  // down). A one-shot REST probe against the same game distinguishes them:
  // a 401 rides api()'s own default redirect-to-login; a 404 means this
  // organizer's session is fine but the game itself isn't — reconnecting
  // forever would just hammer the server with handshakes that can never
  // succeed, so this stops retrying and surfaces notFound instead; any
  // other outcome (5xx, network error) is treated as transient and falls
  // back to the normal reconnect loop.
  const probeThenDecide = () => {
    api<unknown>(`/api/games/${encodeURIComponent(gameId)}`).then(
      () => {
        scheduleReconnect()
      },
      (err: unknown) => {
        if (err instanceof ApiError && err.status === 404) {
          setState({ notFound: true })
          return
        }
        scheduleReconnect()
      },
    )
  }

  const connect = () => {
    if (stopped) return
    // A fresh connection starts a new snapshot sequence: the server's
    // in-memory seq counter resets to 0 on restart, so a reconnecting
    // client must accept whatever seq the new connection's first frame
    // carries rather than comparing it against the pre-disconnect value.
    lastSeq = -1
    hasOpened = false
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    // Captured locally so each handler can tell whether it belongs to the
    // still-current connection — under React StrictMode's dev-only
    // mount/unmount/remount cycle, a socket opened by a superseded connect()
    // call can still fire 'close' after a newer socket has already taken
    // over; without this guard that stale event would null out a snapshot
    // the new connection already delivered and open a redundant third socket.
    const thisSocket = new WebSocket(
      `${protocol}//${window.location.host}/ws?gameId=${encodeURIComponent(gameId)}&role=${role}`,
    )
    socket = thisSocket
    thisSocket.addEventListener('open', () => {
      if (socket !== thisSocket) return
      hasOpened = true
      backoff = initialBackoffMs
    })
    thisSocket.addEventListener('message', (event) => {
      if (socket !== thisSocket) return
      let envelope: SnapshotEnvelope
      try {
        envelope = JSON.parse(event.data as string) as SnapshotEnvelope
      } catch {
        return
      }
      // Reject anything that isn't a well-formed snapshot envelope — the
      // wire's only versioning affordance (type) is otherwise never
      // checked, and an unvalidated seq would silently disarm the
      // stale-drop guard below (`undefined < lastSeq` is false, so a
      // malformed frame would pass through and then reset lastSeq itself).
      if (envelope.type !== 'snapshot' || typeof envelope.seq !== 'number') return
      // seq === lastSeq is a harmless idempotent re-render, not a drop.
      if (envelope.seq < lastSeq) return
      lastSeq = envelope.seq
      setState({ snapshot: envelope.state })
    })
    thisSocket.addEventListener('close', () => {
      if (stopped || socket !== thisSocket) return
      // The WS is down — UX-DR12 wants the "connecting" copy shown, not
      // stale data, for as long as reconnecting is in progress.
      setState({ snapshot: null })
      if (!hasOpened) {
        probeThenDecide()
        return
      }
      scheduleReconnect()
    })
  }

  return {
    subscribe(onChange) {
      listeners.add(onChange)
      if (listeners.size === 1) {
        stopped = false
        // A fresh mount always re-classifies rather than trusting a
        // possibly-stale notFound from a cached store (see the
        // module-level comment: stores are never evicted).
        state = { ...state, notFound: false }
        connect()
      }
      return () => {
        listeners.delete(onChange)
        if (listeners.size === 0) {
          // The store itself stays cached in `stores` (see the module-level
          // comment) — only the socket stops. useGameSocket looks the store
          // up fresh on every render; evicting it here would let a later
          // render "find" nothing and create a second store with its own
          // socket, abandoning this one's still-live connection the moment
          // React's effect double-invoke (StrictMode) or any unrelated
          // re-render raced this cleanup.
          stopped = true
          clearTimeout(reconnectTimer)
          socket?.close()
          socket = null
          state = { ...state, snapshot: null }
        }
      }
    },
    getSnapshot() {
      return state
    },
  }
}

/**
 * Connects to /ws for (gameId, role) and returns the latest live state.
 * `snapshot` is null before the first frame arrives (or while
 * reconnecting); `notFound` means a REST probe confirmed this organizer
 * can't see this game. Reconnects indefinitely with exponential backoff for
 * transient drops; the caller never sees a stale-seq regression (AC-4).
 */
export function useGameSocket(gameId: string, role: 'host' | 'display'): GameSocketState {
  const key = `${gameId}:${role}`
  let store = stores.get(key)
  if (!store) {
    store = createStore(gameId, role)
    stores.set(key, store)
  }
  return useSyncExternalStore(store.subscribe, store.getSnapshot)
}
