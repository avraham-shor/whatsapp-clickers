import { QueryClient } from '@tanstack/react-query'

// The server's error envelope is {"error":{"code","message"}} with
// developer-facing English; user-facing Hebrew lives in strings.he.ts.
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

const LOGIN_PATH = '/api/auth/login'

interface ApiOptions {
  /**
   * What to do when the server answers 401. The default hard-redirects to
   * /login (the session is gone); the RequireAuth session probe opts into
   * 'throw' and renders <Navigate> itself.
   */
  on401?: 'redirect' | 'throw'
}

/**
 * Typed JSON fetch: sends/receives JSON, converts the error envelope into
 * ApiError. A 401 on any call except login itself means the session is gone.
 */
export async function api<T>(path: string, init?: RequestInit, options?: ApiOptions): Promise<T> {
  const response = await fetch(path, {
    headers: init?.body ? { 'Content-Type': 'application/json' } : undefined,
    ...init,
  })

  if (response.status === 401 && path !== LOGIN_PATH && options?.on401 !== 'throw') {
    window.location.assign('/login')
    // Never settles — the browser is navigating away.
    return new Promise<never>(() => {})
  }

  if (!response.ok) {
    let code = 'UNKNOWN'
    let message = `request failed with status ${response.status}`
    try {
      const body = (await response.json()) as { error?: { code?: string; message?: string } }
      if (body.error?.code) {
        code = body.error.code
        message = body.error.message ?? message
      }
    } catch {
      // Non-JSON error body (e.g. a proxy page) — keep the generic message.
    }
    throw new ApiError(response.status, code, message)
  }

  if (response.status === 204) {
    return undefined as T
  }
  try {
    return (await response.json()) as T
  } catch {
    // A 2xx with an empty or non-JSON body (e.g. a proxy page) must surface
    // as the typed error callers already handle, not an uncaught SyntaxError.
    throw new ApiError(response.status, 'INVALID_RESPONSE', 'expected a JSON response body')
  }
}

export const queryClient = new QueryClient({
  defaultOptions: {
    // No automatic retries: a failed session probe must resolve to the login
    // redirect promptly, not after three backoffs.
    queries: { retry: false },
  },
})
