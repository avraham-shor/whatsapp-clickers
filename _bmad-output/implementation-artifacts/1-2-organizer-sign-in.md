---
baseline_commit: 3cebfb4
---

# Story 1.2: Organizer Sign-In

Status: done

## Story

As an Organizer,
I want to sign in to my dashboard with provisioned credentials,
so that my Games are accessible only to me.

## Acceptance Criteria

1. **Given** a manually provisioned account (documented provisioning path storing an argon2id hash; `organizers` + `sessions` tables created in this story), **when** I submit valid credentials on the Hebrew RTL login page, **then** a server-side session is created in Postgres, an HTTP-only Secure cookie is set, and I land on my games list.
2. **Given** invalid credentials, **then** the error copy (from `strings.he.ts`) describes what happened in Hebrew — never vague, never blaming (UX-DR12).
3. **Given** no valid session, **when** any `/api/*` route is called, **then** the response is 401 with the `{"error":{"code","message"}}` envelope, and the SPA redirects to login.
4. **When** I log out, **then** the session row is deleted server-side and the cookie cleared.
5. **Given** a server restart, **then** existing sessions survive (Postgres-backed).

## Tasks / Subtasks

- [x] Task 1: SESSION_SECRET goes live — config hardening (AC: 1; closes deferred-work item #2 for this var)
  - [x] `internal/config/config.go`: add validation for `SESSION_SECRET` now that it is consumed — minimum 32 chars AND reject the documented placeholder (any value starting with `change-me`). Error message must name the var and say how to generate a real one
  - [x] `internal/config/config_test.go`: tests — too-short secret rejected, placeholder rejected, real value passes (existing `fullEnv()` helper needs a ≥32-char `SESSION_SECRET`)
  - [x] `server/.env.example`: replace the SESSION_SECRET placeholder comment with generation instructions (`openssl rand -base64 48`; PowerShell: `[Convert]::ToBase64String((1..48 | %{Get-Random -Max 256}))` or use Git Bash)
  - [x] ⚠️ README operator note + **HUMAN step for Avraham**: the Railway `SESSION_SECRET` is currently the placeholder per the 1.1 runbook — a **real random value must be set in Railway Variables BEFORE this story merges to main**, or the deploy will fail fast at boot (by design). Add this to the runbook section
- [x] Task 2: Schema + store layer — `organizers` and `sessions` (AC: 1, 4, 5)
  - [x] `server/migrations/00002_organizers_sessions.sql` (goose Up/Down): `organizers` (id UUID PK DEFAULT gen_random_uuid(), username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, created_at timestamptz NOT NULL DEFAULT now()); `sessions` (id UUID PK DEFAULT gen_random_uuid(), organizer_id UUID NOT NULL REFERENCES organizers(id) ON DELETE CASCADE, token_hash TEXT NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL); index `idx_sessions_organizer_id`
  - [x] `internal/store/queries/organizers.sql`: `GetOrganizerByUsername :one`; `UpsertOrganizer :one` (INSERT … ON CONFLICT (username) DO UPDATE SET password_hash — used by the provisioning CLI)
  - [x] `internal/store/queries/sessions.sql`: `CreateSession :one`; `GetSessionOrganizer :one` (JOIN organizers, WHERE token_hash = $1 AND expires_at > now()); `DeleteSessionByTokenHash :exec`; `DeleteExpiredSessions :exec` (housekeeping — call opportunistically on login)
  - [x] Run `sqlc generate`, commit `gen/` output; add thin wrapper methods on `store.Store` following the existing `Ping` pattern
- [x] Task 3: Auth domain package — `internal/auth` (AC: 1, 2, 4)
  - [x] `go get golang.org/x/crypto` (argon2)
  - [x] `auth/password.go`: `HashPassword` / `VerifyPassword` using argon2id, PHC string format (`$argon2id$v=19$m=19456,t=2,p=1$<b64salt>$<b64key>`); params per OWASP baseline: m=19456 KiB, t=2, p=1, 16-byte salt (crypto/rand), 32-byte key; Verify parses params **from the stored hash** (future-proof) and compares with `subtle.ConstantTimeCompare`
  - [x] `auth/session.go`: `Service` struct holding the store interface + `[]byte(SESSION_SECRET)`. Token = 32 bytes crypto/rand, cookie value = base64url; DB stores hex `HMAC-SHA256(secret, token)` — a read-only DB leak cannot forge or replay sessions. Methods: `Login(ctx, username, password) (token, Organizer, error)` — on unknown username verify against a package-level dummy argon2id hash anyway (uniform timing, no user enumeration); `Authenticate(ctx, token) (Organizer, error)`; `Logout(ctx, token) error`. Session TTL: 30 days fixed `[ASSUMPTION — architecture doesn't specify; pilot-appropriate]`
  - [x] `auth/auth_test.go`: hash→verify roundtrip; wrong password fails; tampered PHC string fails cleanly; token HMAC is deterministic per secret and differs across secrets; Login timing path executes dummy verify for unknown user (behavioral: unknown-user and wrong-password both return the same sentinel error `ErrInvalidCredentials`)
- [x] Task 4: Provisioning CLI — the documented path (AC: 1)
  - [x] `server/cmd/provision/main.go`: flags `-username`; password read from stdin (piped or interactive prompt — never argv, never logged); requires only `DATABASE_URL` env (do NOT use `config.Load()` — it demands all 7 vars); hashes via `auth.HashPassword`, upserts via store; prints organizer id + username on success
  - [x] README: new "Provisioning an Organizer" subsection under the operator runbook — local (`go run ./cmd/provision -username avraham` with local DATABASE_URL) and production (same command with the Railway public `DATABASE_URL` from the Postgres service's Connect tab)
- [x] Task 5: HTTP surface — login/logout/me + session middleware (AC: 1, 2, 3, 4)
  - [x] `internal/httpapi/middleware.go`: `RequireOrganizer(svc)` — reads the `wc_session` cookie, calls `svc.Authenticate`, injects organizer into request context (unexported context key + typed getter); missing/invalid/expired → 401 `{"error":{"code":"UNAUTHORIZED","message":"valid session required"}}`
  - [x] `internal/httpapi/auth_handlers.go`: `POST /api/auth/login` `{username, password}` → 200 `{"id":…,"username":…}` + `Set-Cookie: wc_session=<token>; Path=/; HttpOnly; Secure; SameSite=Lax; Max-Age=2592000`; bad credentials → 401 `INVALID_CREDENTIALS` (same response for unknown user and wrong password); malformed body → 400 `INVALID_REQUEST`. `POST /api/auth/logout` → deletes session row, clears cookie (same attributes, Max-Age=0) → 204. `GET /api/auth/me` (protected) → 200 `{"id":…,"username":…}`
  - [x] `internal/httpapi/router.go`: restructure into `r.Route("/api", …)` — public: `/api/health`, `/api/auth/login`; everything else under the API branch wrapped in `RequireOrganizer` (logout + me now; games etc. in 1.3). NewRouter signature grows (store + auth service + static) — update `cmd/server/main.go` wiring. **Regression guard: every existing `router_test.go` case must pass unchanged** — SPA fallback, asset 404, 405/404 JSON envelopes, and public `/api/health` are 1.1 review-hardening behavior (watch chi subrouter NotFound/MethodNotAllowed semantics when nesting)
  - [x] slog: INFO on login success/logout (`organizer_id`, `username`), INFO on failed login (`username` only — never the password, never the cookie token); no new ERROR paths (NFR-8)
  - [x] `router_test.go` additions following the existing stub pattern (`stubPinger`): login OK sets HttpOnly+Secure+SameSite=Lax cookie and returns organizer JSON; login bad creds → 401 envelope `INVALID_CREDENTIALS`; `GET /api/auth/me` without cookie → 401 envelope; with valid cookie → 200; logout → 204, cookie cleared (Max-Age=0), session deleted (stub asserts); expired session → 401; `/api/health` still public
- [x] Task 6: Frontend — login page, route tree, API client (AC: 1, 2, 3, 4)
  - [x] `npm install react-router @tanstack/react-query` (React Router v8 — single package name `react-router`, ESM-only; TanStack Query v5)
  - [x] `npx shadcn@latest add input label card` (Button already present from 1.1)
  - [x] `src/lib/api.ts`: typed fetch wrapper — JSON in/out, parses the error envelope into a typed `ApiError {code, message}`; exports `QueryClient` setup. On 401 from any call **except** `/api/auth/login`: hard-redirect to `/login` (AC-3 second half)
  - [x] `src/app.tsx`: React Router v8 route tree — `/login` → `LoginPage`; `/` → `RequireAuth` layout (queries `/api/auth/me`; pending → minimal loader; 401 → `<Navigate to="/login">`) → `GamesListPage`. `main.tsx`: wrap in `QueryClientProvider` + router provider; remove the placeholder `App.tsx` composition (keep or delete `App.tsx` — route tree replaces it)
  - [x] `src/features/auth/login-page.tsx`: RTL Hebrew login form — username + password fields (shadcn Input/Label), submit via TanStack `useMutation` → on success navigate to `/`; while pending, disable the submit button (architecture: mutations disable their trigger). Host-slate palette (`bg-host-surface`, `text-host-text`), centered card (`rounded-md` = 12px, border `host-border`, no shadow), buttons 40px height, visible focus rings (shadcn defaults preserved), all sizes rem
  - [x] Error display per UX-DR12: error text from `strings.he.ts`, associated to the form via `aria-describedby`, never vague/apologizing/blaming — suggested copy in Dev Notes; **no Hebrew literals in the component**
  - [x] `src/features/builder/games-list-page.tsx`: minimal authenticated landing — page title from `strings.he.ts`, empty-state placeholder ("games CRUD arrives in Story 1.3"), and a logout button (calls `POST /api/auth/logout`, then navigate to `/login`)
  - [x] `src/lib/strings.he.ts`: add all new UI strings (login title, field labels, submit, error messages, games-list title, logout)
- [x] Task 7: Quality gates + end-to-end verification (all ACs)
  - [x] All five local gates: `gofmt` clean, `go vet ./...`, `go test ./...`, `sqlc generate` (empty diff), `eslint`, `tsc -b --noEmit`, `npm run build`
  - [x] Manual E2E against local Postgres (Docker, per README): provision a user via CLI → `make dev` → login on :5173 → lands on games list → refresh keeps you in (cookie) → direct `curl` to `/api/auth/me` without cookie → 401 envelope → logout clears → **restart the Go server → the still-held cookie works again (AC-5)** → login with wrong password → Hebrew UX-DR12 error
  - [x] Verify `make build` binary serves `/login` via SPA fallback (client route, no file extension)

### Review Findings

- [x] [Review][Patch] Password reset doesn't invalidate existing sessions [server/internal/store/auth.go:UpsertOrganizer] — Decision: add a `DeleteSessionsByOrganizerID` query and call it from the provisioning upsert path, so resetting a password invalidates any leaked/stale session for that organizer.
- [x] [Review][Patch] No username case/whitespace normalization [server/migrations/00002_organizers_sessions.sql; server/internal/store/queries/organizers.sql; server/cmd/provision/main.go] — Decision: trim whitespace on the `-username` flag and enforce case-insensitive uniqueness (citext or a `lower(username)` unique index) so re-provisioning with different casing updates the existing account instead of creating a duplicate.
- [x] [Review][Patch] Logout doesn't clear cookie when Logout fails server-side [server/internal/httpapi/auth_handlers.go:handleLogout; web/src/features/builder/games-list-page.tsx] — On a 503 from `svc.Logout`, the handler never calls `sessionCookie("", -1)`, leaving the browser holding a cookie it believes was cleared; the frontend mutation also has no `onError`, so the user sees no feedback at all.
- [x] [Review][Patch] Provisioning CLI accepts a 1-character password [server/cmd/provision/main.go:readPassword] — only an empty password is rejected; there is no minimum length/complexity floor.
- [x] [Review][Patch] `sessions` table has no index on `expires_at` [server/migrations/00002_organizers_sessions.sql] — `DeleteExpiredSessions` runs a `WHERE expires_at <= now()` delete on every successful login, a full table scan without this index.
- [x] [Review][Patch] `DeleteExpiredSessions` error is silently discarded [server/internal/auth/session.go:Login] — `_ = s.store.DeleteExpiredSessions(ctx)` drops the error with no logging; if cleanup starts failing there is no signal anywhere.
- [x] [Review][Patch] `handleLogin` collapses unrelated failures into one 503 `DB_UNAVAILABLE` [server/internal/httpapi/auth_handlers.go:handleLogin] — DB errors, token-generation failures, and context-timeout expiry from `Service.Login` all map to the same generic error, which could mislead on-call triage.
- [x] [Review][Patch] Missing exact-boundary test for `SESSION_SECRET` length [server/internal/config/config_test.go] — tests cover 31 (rejected) and 44 (accepted) characters but not exactly 32, so an off-by-one in `len(secret) < minSessionSecretLen` wouldn't be caught either direction.
- [x] [Review][Patch] `VerifyPassword` shadows the `time` package with a local variable named `time` [server/internal/auth/password.go] — harmless today only because this file doesn't import `time`; a latent trap for future edits.
- [x] [Review][Patch] `VerifyPassword` doesn't validate parsed argon2 params before calling `argon2.IDKey` [server/internal/auth/password.go] — a corrupted/hand-edited PHC hash with `t=0`/`p=0` could panic the request goroutine, and an unbounded `m=` value could trigger an oversized allocation.
- [x] [Review][Patch] `POST /api/auth/login` has no request-body size limit [server/internal/httpapi/auth_handlers.go:handleLogin] — no `http.MaxBytesReader` guard before JSON decode on a public, unauthenticated endpoint.
- [x] [Review][Patch] No catch-all route in the SPA router [web/src/app.tsx] — an unmatched client path falls through to React Router's default English/unstyled error boundary, breaking the app's Hebrew-only UI contract.
- [x] [Review][Patch] `api.ts` doesn't guard malformed JSON on success responses [web/src/lib/api.ts] — a non-204 2xx response with an empty/malformed body throws an uncaught `SyntaxError` instead of the typed `ApiError` callers expect.

## Dev Notes

### What this story is — and is not

Auth vertical slice only: two tables, argon2id + Postgres sessions, login/logout/me endpoints, session middleware, RTL login page, minimal games-list landing, provisioning CLI. It does **not** build: games CRUD or the real games list (1.3), any WhatsApp/WS code, password reset/change, self-serve signup (commercial phase), login rate-limiting (architecture: explicitly "Nice-to-Have (future)" — do not add it), magic links (PRD mentions them as an alternative; architecture decided credentials — follow architecture). The `/api/health` endpoint stays public (Railway monitoring).

### Previous story intelligence (1.1 — read this, it will save you rework)

- **Module path**: `github.com/avraham-shor/whatsapp-clickers`. Go 1.26.5 via the `go1.26.5` wrapper (system go is 1.22 — the repo's `go.mod` toolchain directive handles it; if a `go get` misbehaves on this filtered network, see README Windows notes — SDK was installed manually).
- **Established code patterns to follow** (all landed in 1.1 review hardening): every handler responds via `writeJSON`/`writeError` (in `router.go`) — 405s and 404s already emit the envelope; server has read/write timeouts; DB calls inside handlers get a `context.WithTimeout` bound (health uses 5s — do the same for auth queries); graceful shutdown via `signal.NotifyContext` exists in `main.go` — don't disturb it.
- **Test conventions**: table-free, direct `httptest` cases with tiny stub interfaces defined in the test file (`stubPinger`); `fstest.MapFS` for static. Config tests use a `lookup` function injection — extend `fullEnv()`, don't invent a new harness. Co-located `_test.go` per Go standard.
- **`store` is the only pgx importer** — handlers depend on small interfaces. `NewRouter` currently takes a `Pinger` interface; grow this pattern (e.g., accept an `AuthService` interface defined in httpapi), don't pass `*store.Store` concretely into handlers.
- **sqlc**: v1.31.x pinned in CI; `sqlc generate` then commit `gen/`; CI diff-gate includes untracked files (`git add -N`). Schema dir is `migrations/` — sqlc reads the migration SQL to derive models, so the CREATE TABLE statements must be clean standard SQL (goose `-- +goose Up/Down` annotations are fine, sqlc ignores them).
- **Migrations**: goose v3 **Provider API** with `WithSessionLocker` (see `store/migrate.go`) — embedded FS via `migrations/embed.go`; just drop `00002_organizers_sessions.sql` next to `00001_init.sql` and the glob picks it up. Numbering starts at 00002 per the 1.1 variance note (architecture's illustrative 00001_organizers_sessions lands here).
- **Deferred-work item lands here** (deferred-work.md): placeholder-secret validation for `SESSION_SECRET` was explicitly deferred TO THIS STORY. Task 1 closes it. The WhatsApp/Anthropic placeholder checks stay deferred to 2.1 — touch only SESSION_SECRET.
- **Deploy reality**: Railway builds via multi-stage Dockerfile (NOT Nixpacks), `numReplicas: 1` is law. The deployed instance's `SESSION_SECRET` is a placeholder today → Task 1's Railway human step is release-blocking.
- **Windows dev**: Hebrew path in checkout works; `make` needs Git Bash; README documents the no-make fallback.

### Architecture guardrails (violations = rework)

- **Glossary nuance**: `sessions` as an auth table name is architecture-sanctioned (it's in the canonical table list). The glossary ban on "session" applies to *Game* synonyms — never call a Game/quiz run a "session" in identifiers or copy. Similarly `username` is fine; but the person is an `Organizer` — `organizers` table, `organizer_id` FK, `RequireOrganizer` middleware. Never `user`/`admin`/`host` identifiers.
- **Wire format**: camelCase JSON, success = direct payload (`{"id":…,"username":…}` — no wrapper), errors = `{"error":{"code","message"}}`, codes SCREAMING_SNAKE (`UNAUTHORIZED`, `INVALID_CREDENTIALS`, `INVALID_REQUEST`), `message` developer-facing English — user-facing Hebrew lives ONLY in `strings.he.ts` (AC-2).
- **Dependency direction**: `httpapi` → `auth` → `store`. `auth` may import `store` (or better: define a narrow store interface in `auth` and let `*store.Store` satisfy it). `store` remains the only pgx importer. Handlers thin — credential/session logic lives in `auth`, not in handlers.
- **Routes**: REST under `/api`, kebab-case plural nouns; auth is the exception domain: `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me` (architecture names `auth_handlers.go # login/logout`).
- **DB naming**: tables plural snake_case, `id UUID DEFAULT gen_random_uuid()` PKs, `<singular>_id` FKs, `timestamptz` UTC, indexes `idx_<table>_<cols>`. `gen_random_uuid()` is built into Postgres 13+ — no extension needed.
- **Frontend boundaries**: `features/auth/` and `features/builder/` may import `lib/` and `components/ui/` — never each other. TanStack Query for CRUD (this story's login mutation + me query); `isPending`/`isError` only — no hand-rolled loading flags. All Hebrew via `strings.he.ts`.
- **Logging**: slog JSON, canonical keys; INFO lifecycle, zero ERRORs on a healthy run. Never log passwords, tokens, or hashes.

### Security implementation specifics

- **argon2id** (`golang.org/x/crypto/argon2`, latest — published 2026-07): OWASP baseline params m=19456 KiB (19 MiB), t=2, p=1; 16-byte random salt; 32-byte key. Store as PHC string so params are self-describing; `VerifyPassword` parses them from the hash, enabling future param bumps without migration. Compare with `crypto/subtle.ConstantTimeCompare`.
- **Session tokens**: 32 bytes from `crypto/rand`, base64url in the cookie. DB stores `hex(HMAC-SHA256(SESSION_SECRET, rawToken))` in `sessions.token_hash` — this is how SESSION_SECRET "signs" sessions (deferred-work's term): a DB dump alone can't produce a valid cookie, and rotating the secret invalidates all sessions at once.
- **Cookie**: `wc_session`; `HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=2592000` (30d, matching the DB `expires_at`). `Secure` on localhost works — modern browsers treat `http://localhost` as a trustworthy context. SameSite=Lax is the pilot CSRF posture: cross-site POSTs are blocked; state-changing routes are POST-only `[ASSUMPTION — no CSRF token in pilot; revisit with commercial phase]`.
- **User enumeration**: unknown username still runs `VerifyPassword` against a fixed dummy hash; both failure modes return one sentinel (`ErrInvalidCredentials`) → one wire response (`INVALID_CREDENTIALS`) → one Hebrew string.
- **Session expiry**: `expires_at > now()` enforced in the lookup query (not in Go) — restart-safe and clock-consistent (AC-5 falls out of Postgres persistence). Lazy cleanup: `DeleteExpiredSessions` fired on login (no cron in pilot).

### UX guardrails for the login page (Host Dashboard surface)

- **Palette**: host-slate, NOT Festival Green — `bg-host-surface` (#F8FAFC), text `host-text`, borders `host-border`; green only as the accent on the primary button (`bg-green-800`, per DESIGN.md host-primary-action). Gold: never (UX-DR2 — gold has exactly two moments, neither is here).
- **Structure**: this story does NOT build the 240px sidebar shell (UX-DR9 — that arrives with the real dashboard in 1.3); a centered card on the slate surface is correct for sign-in. Card: 12px radius, 1px `host-border` border, **no box-shadow** (shadows are modal-only on the dashboard), 24px internal padding.
- **Controls**: buttons 40px height; inputs with visible labels (shadcn Label — placeholder is not a label); shadcn focus rings preserved; everything in rem; functional at 200% zoom (single-column already is).
- **Error copy (AC-2, UX-DR12)**: describes what happened + what to do; never "שגיאה" alone, never apology, never blame. Suggested strings (final copy is dev's craft, principles are binding — host register is direct/terse, not the WhatsApp playful voice):
  - invalid credentials: `שם המשתמש והסיסמה לא תואמים לחשבון קיים. בדקו את הפרטים ונסו שוב.`
  - server/network failure: `החיבור לשרת נכשל — מנסים שוב עוד רגע.` (align with EXPERIENCE.md's connection-error pattern)
  - Wire the message to the form with `aria-describedby` on the inputs/form region (UX-DR14 — programmatic association, no floating toast).
- **RTL/lang**: `index.html` already carries `lang="he" dir="rtl"` — verify the form renders RTL correctly with shadcn inputs (logical properties should handle it; the shadcn init was run with `rtl: true`).
- **Sign-in surface reference**: EXPERIENCE.md Host Dashboard surfaces table row 1 — "Pilot: manually provisioned credentials"; no signup link, no password-reset link (nothing to link to).

### Latest tech intelligence (web research 2026-07-13)

- **React Router v8** (released 2026-06): install `npm install react-router` — single package, ESM-only, ES2022, requires React ≥19.2.7 / Vite ≥7 / Node ≥22.22 — repo has React 19.2.7, Vite 8.1.1, CI Node 22.x ✓. Import everything from `react-router` (`createBrowserRouter`, `RouterProvider`, `Navigate`, `useNavigate`). Data-mode `createBrowserRouter` recommended; loaders optional — TanStack Query owns data fetching per architecture, so use plain route elements + the `RequireAuth` wrapper, not route loaders.
- **TanStack Query v5** (5.101.x current): `useMutation` for login/logout, `useQuery({queryKey:['auth','me']})` for the session probe; v5 object-signature only; `isPending` replaced v4's `isLoading` on mutations. On logout success call `queryClient.clear()` to drop cached auth state.
- **golang.org/x/crypto**: fetch latest (July 2026 release); only the `argon2` package is needed — it pulls `golang.org/x/sys` transitively.
- **shadcn CLI v4.13** (from 1.1): `-b radix` primitives, preset `nova`, `rtl: true` already in `components.json`; `add input label card` is non-interactive-safe. Re-check that `add` doesn't reintroduce a fontsource import (1.1 had to strip `@fontsource-variable/geist` — filter-safety grep in CI will catch it if it sneaks in).

### Project Structure Notes

New files: `server/migrations/00002_organizers_sessions.sql` · `server/internal/store/queries/{organizers,sessions}.sql` (+ regenerated `gen/`) · `server/internal/auth/{password,session}.go` (+ tests) · `server/internal/httpapi/{middleware,auth_handlers}.go` · `server/cmd/provision/main.go` · `web/src/app.tsx` · `web/src/lib/api.ts` · `web/src/features/auth/login-page.tsx` · `web/src/features/builder/games-list-page.tsx` · shadcn `input/label/card` under `components/ui/`.

Updated files: `server/internal/config/config.go` (+ test) · `server/internal/httpapi/router.go` (+ test) · `server/cmd/server/main.go` (wire auth service into router) · `web/src/main.tsx` · `web/src/lib/strings.he.ts` · `server/.env.example` · `README.md` · `web/package.json` · `server/go.mod`.

**Variances from the canonical architecture tree (documented):** (1) `cmd/provision/` is added — the architecture requires a "documented provisioning path" outcome but names no mechanism; a CLI reusing `auth.HashPassword` + the store is the smallest auditable path (mirrors 1.1's `webdist/` precedent for structural additions). (2) `httpapi/errors.go` (central domain-error mapper) is NOT created yet — two auth error codes don't justify it; the existing `writeError` suffices. It arrives with 1.3's richer error surface. (3) `App.tsx` is superseded by `app.tsx` route tree per the canonical tree (delete `App.tsx`).

### Testing standards

Go stdlib `testing`, co-located, stub interfaces in-test (existing style). This story ships: config validation tests (Task 1), auth package tests (Task 3), httpapi auth flow tests (Task 5). No real-Postgres integration tests in CI (matches 1.1) — the manual E2E list in Task 7 covers the DB path, incl. the AC-5 restart check. Frontend: still no Vitest (architecture: "added when first frontend tests land" — the login form doesn't force it; eslint + tsc strict + build gate it). CI must stay green on the PR — all five gates are enforced there.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-1.2] — story + ACs (verbatim)
- [Source: _bmad-output/planning-artifacts/architecture.md#Authentication-&-Security] — argon2id, server-side sessions, cookie posture, ownership scoping
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation-Patterns-&-Consistency-Rules] — naming, wire format, error envelope, logging, glossary
- [Source: _bmad-output/planning-artifacts/architecture.md#Complete-Project-Directory-Structure] — auth/, httpapi/, features/auth/ placement
- [Source: _bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md#4.4] — FR-11 sign-in half; pilot manual provisioning; NFR-7
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#Host-Dashboard] — sign-in surface, error-copy principles (UX-DR12), accessibility floor
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/DESIGN.md] — host-slate palette, host-primary-action, elevation rules, 40px buttons
- [Source: _bmad-output/implementation-artifacts/1-1-project-scaffold-ci-and-deployed-walking-skeleton.md] — module path, code/test patterns, review hardening, Windows notes
- [Source: _bmad-output/implementation-artifacts/deferred-work.md] — SESSION_SECRET placeholder validation lands in this story
- Web research 2026-07-13: React Router v8 (remix.run/blog/react-router-v8 — ESM-only, React 19.2.7+/Vite 7+/Node 22.22+); TanStack Query 5.101.x; OWASP argon2id baseline m=19456/t=2/p=1 (Password Storage Cheat Sheet); golang.org/x/crypto July 2026 release

## Dev Agent Record

### Agent Model Used

claude-fable-5 (Claude Code)

### Debug Log References

- Transient toolchain crashes during `go build`/`go test` on the dev machine (Windows, `VirtualAlloc … errno=1455` — pagefile pressure); every gate passed deterministically on re-run. Local builds used `-buildvcs=false` because git exits 128 inside the sandboxed build env — flag affects local verification only, not CI/Dockerfile.
- `make` unavailable in the session shell; `make build` was executed as its exact constituent commands (npm build → copy into webdist → go build). `webdist/dist` restored to the committed placeholder afterwards.
- Windows case-insensitive FS: the story-mandated `App.tsx` → `app.tsx` rename is invisible to git as a working-tree change, which would break module resolution on Linux CI. Fixed by staging the rename explicitly (`git rm --cached web/src/App.tsx` + `git add web/src/app.tsx`) — this is why those two paths appear staged.
- Review fixes (2026-07-15): migration 00002 was edited **in place** (allowed — never merged/deployed). Verified end-to-end against a fresh scratch DB in the local Postgres container: migration applies cleanly, `ON CONFLICT (lower(username))` inference works (same UUID across `'  Avraham  '`/`avraham`/`AVRAHAM` provisions, single row, trimmed), password reset deletes existing session rows, 5-char password rejected exit 1. The already-migrated local dev DB `whatsapp_clickers` was aligned manually (dropped `organizers_username_key`, created `idx_organizers_username_lower` + `idx_sessions_expires_at`) — anyone else with a pre-review local DB must do the same or reset the Docker volume. Railway is unaffected (00002 never ran there).

### Completion Notes List

- **Task 1**: `SESSION_SECRET` validation live — <32 chars or any `change-me*` value refuses boot with an actionable message (verified against the built binary). Closes the deferred-work item for this var; WhatsApp/Anthropic placeholder checks remain deferred to 2.1 as specified.
- **Task 2**: `organizers` + `sessions` schema in goose migration 00002; sqlc queries + committed `gen/`. Added sqlc overrides (uuid→`string`, timestamptz→`time.Time`) so generated models carry no pgx types — keeps `store` the only pgx importer (all columns NOT NULL, safe). Store wrappers translate `pgx.ErrNoRows` → `store.ErrNotFound` so callers never touch pgx sentinels.
- **Task 3**: `internal/auth` — argon2id PHC (OWASP m=19456/t=2/p=1, params parsed from stored hash on verify, constant-time compare); session tokens 32B crypto/rand, DB stores hex HMAC-SHA256(secret, token); unknown-user login verifies a package-level dummy hash (uniform timing) and both failure modes share `ErrInvalidCredentials`. TTL 30d fixed. 9 unit tests.
- **Task 4**: `cmd/provision` — `-username` flag, password via stdin only (piped or echoed interactive prompt on stderr; no x/term to avoid an unsanctioned dependency), requires only `DATABASE_URL`, upserts via store. README runbook subsection added.
- **Task 5**: Router restructured to `r.Route("/api", …)` with explicit JSON NotFound/MethodNotAllowed on the API branch (chi subrouters don't inherit handlers set after mounting); public = health + login, everything else behind `RequireOrganizer`. Login/logout/me handlers per spec (cookie `wc_session; Path=/; HttpOnly; Secure; SameSite=Lax; Max-Age=2592000`; clearing = same attributes, Max-Age=0). Middleware distinguishes `ErrNoSession` (401) from DB failure (503 DB_UNAVAILABLE — a DB outage must not bounce users to login). slog INFO only; failed login logs username only. All 9 pre-existing router tests pass unchanged; 9 auth-flow tests added (18 total).
- **Task 6**: React Router v8 (`react-router@8.2.0`) + TanStack Query v5 (`5.101.2`); `app.tsx` route tree replaces `App.tsx` (deleted per canonical tree). `api.ts` hard-redirects to `/login` on 401 for every call except login itself; the `RequireAuth` session probe opts into `on401:'throw'` and renders `<Navigate>` (reconciles the two spec bullets — soft navigation for the probe, hard redirect for everything else). Login page: host-slate centered card (12px radius, host-border, no shadow, 24px padding), 40px controls, visible labels + focus rings, error copy from `strings.he.ts` wired via `aria-describedby` + `role="alert"`, submit disabled while pending. Games-list landing with logout (clears query cache, navigates to /login). shadcn `input/label/card` added — filter-safety greps (source + bundle, CI's exact patterns) clean.
- **Task 7**: All gates green — gofmt, go vet, go test (3 pkgs, fresh), sqlc generate idempotent, eslint, tsc -b, vite build. Manual E2E against local Docker Postgres 17 (via curl + the built binary, headless session): provision CLI ✓ → login 200 + full-attribute cookie ✓ → `/api/auth/me` with cookie 200 / without cookie 401 envelope ✓ → wrong password and unknown user return identical `INVALID_CREDENTIALS` ✓ → logout 204, Max-Age=0, session row deleted (DB count 0) ✓ → **server killed and restarted → pre-restart cookie authenticates (AC-5)** ✓ → placeholder/short SESSION_SECRET refuse boot ✓ → `/login` served via SPA fallback from the binary, `lang="he" dir="rtl"`, Hebrew UX-DR12 error copy present in the served bundle ✓ → `/api/health` still public ✓. Zero ERROR log lines across the healthy E2E run. Note: browser-visual pass (form rendering at :5173) not possible in this headless session — recommended as part of review.
- **Human step CLOSED 2026-07-15**: Avraham set a real `SESSION_SECRET` in Railway Variables before the push; deploy booted clean (validation passed). Production organizer account provisioned via the CLI against the Railway public `DATABASE_URL`; production login verified in the browser.
- **Review fixes (all 13 findings resolved, 2026-07-15)**: ① provisioning now deletes all sessions for the organizer after upsert (`DeleteSessionsByOrganizerID` query + store wrapper) — password reset invalidates leaked/stale sessions; ② username trimmed in the CLI and uniqueness enforced case-insensitively via `idx_organizers_username_lower` on `lower(username)` (lookup + ON CONFLICT both use `lower()`); ③ logout clears the cookie even when the server-side delete fails (503 + Max-Age=0), and the games-list page shows a Hebrew alert on logout failure; ④ provisioning enforces an 8-char (NIST floor) minimum password, counted in runes; ⑤ `idx_sessions_expires_at` added for the per-login cleanup delete; ⑥ cleanup failure now logs `slog.Warn` instead of being discarded; ⑦ login failures map to distinct codes — `ErrTokenGeneration` sentinel → 500 `INTERNAL`, other infrastructure errors → 503 `DB_UNAVAILABLE`, both logged with the underlying cause; ⑧ exact-32-char `SESSION_SECRET` boundary test pins the off-by-one; ⑨ `time` shadow renamed `timeCost`; ⑩ parsed argon2 params validated before `IDKey` (t/p ≥ 1, 8·p ≤ m ≤ 2 GiB, non-empty salt) so a tampered PHC hash can't panic or over-allocate; ⑪ login body capped at 4 KiB via `http.MaxBytesReader` → 413 `REQUEST_TOO_LARGE`; ⑫ `path: '*'` catch-all route renders a Hebrew not-found page with a home link; ⑬ non-JSON 2xx bodies in `api.ts` throw typed `ApiError(INVALID_RESPONSE)` instead of an uncaught `SyntaxError`. 6 new tests (2 auth, 3 httpapi, 1 config); all gates re-run green (gofmt, vet, go test, sqlc idempotent, eslint, tsc, vite build); DB-level behavior verified against a fresh Postgres (see Debug Log).

### File List

New:
- server/migrations/00002_organizers_sessions.sql
- server/internal/store/queries/organizers.sql
- server/internal/store/queries/sessions.sql
- server/internal/store/gen/organizers.sql.go
- server/internal/store/gen/sessions.sql.go
- server/internal/store/auth.go
- server/internal/auth/password.go
- server/internal/auth/session.go
- server/internal/auth/auth_test.go
- server/internal/httpapi/middleware.go
- server/internal/httpapi/auth_handlers.go
- server/cmd/provision/main.go
- web/src/app.tsx
- web/src/lib/api.ts
- web/src/features/auth/login-page.tsx
- web/src/features/builder/games-list-page.tsx
- web/src/components/ui/input.tsx
- web/src/components/ui/label.tsx
- web/src/components/ui/card.tsx

Modified:
- server/internal/config/config.go
- server/internal/config/config_test.go
- server/internal/httpapi/router.go
- server/internal/httpapi/router_test.go
- server/internal/store/gen/models.go
- server/cmd/server/main.go
- server/sqlc.yaml
- server/go.mod
- server/go.sum
- server/.env.example
- web/src/main.tsx
- web/src/lib/strings.he.ts
- web/package.json
- web/package-lock.json
- README.md

Deleted:
- web/src/App.tsx (superseded by web/src/app.tsx; case-rename staged explicitly for Linux CI)

## Change Log

- 2026-07-14: Story 1.2 implemented end-to-end — SESSION_SECRET hardening, organizers/sessions schema + store, argon2id + HMAC-signed Postgres sessions (`internal/auth`), provisioning CLI, login/logout/me + `RequireOrganizer` middleware with `/api` router restructure, RTL Hebrew login page + games-list landing (React Router v8 + TanStack Query v5). 21 new tests (3 config, 9 auth, 9 httpapi); all quality gates green; manual E2E incl. AC-5 restart-survival verified. Status → review.
- 2026-07-15 (later): Committed as af6c000 and pushed; Railway deploy green with the real SESSION_SECRET; production organizer provisioned (CLI over public DATABASE_URL) and browser login verified by Avraham. Status → done.
- 2026-07-15: Addressed code review findings — 13 items resolved (sessions invalidated on password reset, case-insensitive usernames, logout cookie-clear on failure + UI feedback, password floor, expires_at index, cleanup-error logging, differentiated login error codes, boundary test, `timeCost` rename, argon2 param validation, 4 KiB login body cap, SPA catch-all, typed non-JSON response error). 6 tests added (27 total for the story); all gates green; migration re-verified on a fresh DB. Status → review.
