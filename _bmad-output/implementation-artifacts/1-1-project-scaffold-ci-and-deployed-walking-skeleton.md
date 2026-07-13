---
baseline_commit: NO_VCS
---

# Story 1.1: Project Scaffold, CI, and Deployed Walking Skeleton

Status: done

## Story

As the platform operator,
I want the monorepo initialized per the architecture's starter commands and deployed end-to-end,
so that every subsequent story builds and ships on working rails.

## Acceptance Criteria

1. **Given** a fresh clone, **when** the architecture's initialization commands are applied (Go 1.26 module + chi v5 + coder/websocket + pgx v5; `create-vite` react-ts + `shadcn init`), **then** the repo matches the architecture's directory structure (`/server` with `cmd/`+`internal/`, `/web`), and `make dev` runs the Go server (:8080) and Vite dev server (:5173) with `/api`/`/ws` proxied.
2. **Given** a production build, **when** `go build` runs, **then** the binary embeds `web/dist`, serves the SPA at `/`, and `GET /api/health` returns 200 JSON.
3. **Given** a pull request, **when** CI runs, **then** `go test`, `go vet`, `sqlc generate` diff-check, `eslint`, `tsc --noEmit`, and the frontend build all gate the merge, **and** merge to main deploys to Railway (EU, replicas pinned to 1) with managed Postgres, goose migrations running at boot.
4. **Given** the deployed SPA, **then** `index.html` carries `lang="he" dir="rtl"`, DESIGN.md tokens are configured (Festival Green + host-slate palettes, spacing/radius scale, system-ui stack — UX-DR1–UX-DR3), **and** a CI grep check verifies no file outside `index.html`/`index.css` references an external URL asset (UX-DR4, filter-safety).
5. **Given** a required env var is missing, **when** the server starts, **then** it fails fast with a clear message; `.env.example` documents all variables.

## Tasks / Subtasks

- [x] Task 1: Repository baseline (AC: 1, 3)
  - [x] `git init` at project root; author `.gitignore` (Go binary output, `node_modules/`, `web/dist/`, `.env`, editor junk; do NOT ignore `_bmad*/`, `docs/` — planning artifacts stay in the repo)
  - [x] Delete the leftover `test.js` at project root (unrelated debris, 2026-07-09)
  - [x] Create the GitHub repository (`gh repo create`) and push `main` — required for CI and Railway auto-deploy
- [x] Task 2: Backend scaffold — `/server` (AC: 1, 2, 5)
  - [x] `mkdir server && cd server && go mod init github.com/<github-user>/whatsapp-clickers` (use the real GitHub path from Task 1)
  - [x] `go get github.com/go-chi/chi/v5 github.com/coder/websocket github.com/jackc/pgx/v5 github.com/pressly/goose/v3`
  - [x] `internal/config/config.go` — parse ALL env vars (list in Dev Notes), fail fast listing every missing var by name; `PORT` optional, default `8080`
  - [x] `cmd/server/main.go` — wire-up: slog JSON logger → config → pgx pool → goose migrations → chi router → `http.Server`
  - [x] `internal/httpapi/router.go` — chi mux: `GET /api/health` → 200 `{"status":"ok","db":"ok"}` (direct payload, camelCase); SPA static serving with `index.html` fallback for non-`/api`,`/ws`,`/webhooks` paths
  - [x] `internal/store/db.go` — pgx pool setup from `DATABASE_URL`
  - [x] Go unit tests: config fail-fast behavior; health handler returns 200 JSON
- [x] Task 3: Migrations + sqlc plumbing (AC: 3)
  - [x] `server/migrations/00001_init.sql` — no-op goose migration (`SELECT 1;` up/down) proving the pipeline; `migrations/embed.go` exposes `//go:embed *.sql` FS; `main.go` runs `goose.Up` via `SetBaseFS` at boot before serving
  - [x] `server/sqlc.yaml` — `sql_package: "pgx/v5"`, queries `internal/store/queries/`, schema `migrations/`, output `internal/store/gen/`
  - [x] `internal/store/queries/health.sql` — `-- name: Ping :one` / `SELECT 1;` — proves `sqlc generate` end-to-end; health handler calls it
  - [x] Run `sqlc generate`, commit `gen/` output
- [x] Task 4: Frontend scaffold — `/web` (AC: 1, 4)
  - [x] `npm create vite@latest web -- --template react-ts` (Node 22 LTS)
  - [x] Tailwind v4: `npm install tailwindcss @tailwindcss/vite`; add plugin to `vite.config.ts`; single `@import "tailwindcss";` in `index.css`
  - [x] Path aliases `@/*` → `./src/*` in `tsconfig.json` AND `tsconfig.app.json`, plus `resolve.alias` in `vite.config.ts` (shadcn prerequisite)
  - [x] `npx shadcn@latest init` (creates `components.json`, `src/components/ui/`, `src/lib/utils.ts`)
  - [x] Strip ALL Vite demo boilerplate (logos, `App.css`, counter demo)
  - [x] `index.html`: `<html lang="he" dir="rtl">`, Hebrew `<title>`, local favicon only
  - [x] `index.css`: DESIGN.md tokens as Tailwind v4 `@theme` CSS variables — exact values in Dev Notes (Festival Green + host-slate + semantic colors, radius, spacing, system-ui font stack, weights 900/800/600/500)
  - [x] `src/lib/strings.he.ts` — created now (may hold just the app title); Hebrew literals in components are forbidden from day one
  - [x] `vite.config.ts` dev proxy: `/api`, `/webhooks` → `http://localhost:8080`; `/ws` → same with `ws: true`
- [x] Task 5: Embed + production build (AC: 2)
  - [x] `server/internal/webdist/` package: `dist.go` with `//go:embed all:dist` exposing `fs.FS`; commit a placeholder `dist/index.html` so `go build`/`go vet`/`go test` pass without a frontend build
  - [x] `Makefile` targets: `dev` (Go :8080 + Vite :5173 concurrently), `build` (web build → copy `web/dist/*` into `server/internal/webdist/dist/` → `go build -o bin/server ./cmd/server`), `test`, `generate` (sqlc), `lint`
  - [x] Verify: `make build` then run binary with env vars → `/` serves the SPA, `/api/health` returns 200
- [x] Task 6: CI pipeline (AC: 3, 4)
  - [x] `.github/workflows/ci.yml` on PR + push to main: `go vet ./...`, `go test ./...`, `sqlc generate` + `git diff --exit-code` (pin sqlc version), `npm ci && npm run lint`, `tsc -b --noEmit`, `npm run build`
  - [x] Filter-safety grep step: fail if any file under `web/` other than `index.html`/`index.css` matches external-asset patterns (`url(http`, `src="http`, `href="http`, `@import url(`, `fonts.googleapis`) — and even those two files must reference no external asset in this story
  - [x] Go setup pinned to 1.26.x, Node to 22.x
- [x] Task 7: Railway deployment (AC: 3) — code half
  - [x] `railway.json`: build command = web build + copy + `go build`; start command = the binary; `numReplicas: 1` (MANDATORY — in-memory hub assumes single instance)
  - [x] `.env.example` at `/server` documenting: `DATABASE_URL`, `WHATSAPP_ACCESS_TOKEN`, `WHATSAPP_PHONE_NUMBER_ID`, `WHATSAPP_APP_SECRET`, `WHATSAPP_VERIFY_TOKEN`, `ANTHROPIC_API_KEY`, `SESSION_SECRET`, `PORT` — placeholder values, no secrets
  - [x] README runbook for the operator steps (see Task 8) + local dev setup (local Postgres via Docker, `make dev`, Windows notes)
- [x] Task 8: Operator setup (HUMAN steps — document, do not fake) (AC: 3)
  - [x] Railway: create project in **EU region**, one service linked to the GitHub repo (deploy on push to main, "wait for CI" enabled), add managed PostgreSQL, set env vars, confirm replicas = 1
  - [x] Verify deployed URL: `/` serves SPA (RTL, Hebrew), `/api/health` → 200, boot logs show goose ran
  - [x] ⚠️ Kick off Meta WhatsApp Business setup NOW (business verification + dedicated number) — the only external lead-time item; it gates Epic 2. Re-verify Meta service-window pricing (≈₪0 conclusion) when the account exists

### Review Findings

Adversarial code review 2026-07-13 (Blind Hunter + Edge Case Hunter + Acceptance Auditor, diff 01552a0..HEAD). Verdict: no AC violations; all findings below are hardening/hygiene. 2 findings dismissed as noise.

- [x] [Review][Defer] Spacing scale "no values between stops" declared but not enforced — Tailwind v4's default `--spacing: 0.25rem` multiplier still generates off-scale utilities; enforcing via `--spacing: initial` would break shadcn defaults (e.g. button `h-9`). Deferred (owner decision 2026-07-13): enforcement call needs real UI to validate against — revisit at the first real UI story (1.3) [web/src/index.css]
- [x] [Review][Patch] SPA fallback returns 200+index.html for missing static assets — stale clients requesting old hashed bundles after a redeploy get HTML (white screen, MIME error) while logs show 200s; return 404 for file-like/`assets/` paths [server/internal/httpapi/router.go:73]
- [x] [Review][Patch] No graceful shutdown — Railway sends SIGTERM on every redeploy; in-flight requests are truncated and `defer pool.Close()` never runs; add `signal.NotifyContext` + `srv.Shutdown` [server/cmd/server/main.go:62]
- [x] [Review][Patch] `http.Server` missing `ReadTimeout`/`WriteTimeout`/`IdleTimeout` — slow clients hold connections indefinitely (slowloris surface) [server/cmd/server/main.go:62]
- [x] [Review][Patch] sqlc drift gate blind to untracked generated files — `git diff --exit-code` ignores new untracked `gen/*.go`; add `git add -N .` (or `git status --porcelain` check) before the diff [.github/workflows/ci.yml:50]
- [x] [Review][Patch] Filter-safety grep bypassable and swallows its own errors — quoted `url("http`, protocol-relative `//`, `srcset=` slip through; `|| true` masks grep exit 2; also blind to bundled node_modules CSS imports (scan built `web/dist` too) [.github/workflows/ci.yml:77]
- [x] [Review][Patch] TypeScript `strict` mode absent from both tsconfigs — undocumented drop from the Vite template default; no `strictNullChecks`/`noImplicitAny` behind the `tsc -b` CI gate [web/tsconfig.app.json, web/tsconfig.node.json]
- [x] [Review][Patch] Non-hermetic builds around `webdist/dist` — `.dockerignore` doesn't exclude `server/internal/webdist/dist` (stale local assets merge into the image); `make build` dirties the tracked placeholder; `make clean` can't restore it [.dockerignore, Makefile]
- [x] [Review][Patch] 405 responses violate the JSON error envelope — `r.MethodNotAllowed` never set; `POST /api/health` returns bare text 405 [server/internal/httpapi/router.go:24]
- [x] [Review][Patch] Health handler DB ping has no server-side timeout — black-holed DB hangs the request instead of prompt 503; wrap with `context.WithTimeout` [server/internal/httpapi/router.go:51]
- [x] [Review][Patch] `make dev` always exits 0 — backend fail-fast leaves Vite running against a dead proxy with no failure signal; propagate job exit status [Makefile:8]
- [x] [Review][Patch] CI has no gofmt step though the Makefile `lint` comment claims CI parity — unformatted Go merges cleanly [.github/workflows/ci.yml]
- [x] [Review][Patch] goose migrations run without a Postgres session lock — Railway zero-downtime redeploys can overlap boots; add `goose.WithSessionLocker` [server/internal/store/migrate.go]
- [x] [Review][Patch] Production container runs as root — no `USER` directive in the runtime stage; add non-root user [Dockerfile:21]
- [x] [Review][Patch] Dev Agent Record internal contradictions — variance (5) still claims `nixpacks.toml` exists (removed 2026-07-13); File List note says sprint-status → in-progress (actual: review); root `.gitignore` listed as New though it's in baseline 01552a0 [this file]
- [x] [Review][Patch] README Meta section says "start NOW" — contradicts the recorded owner decision to defer setup to Epic 2 start [README.md]
- [x] [Review][Defer] Fail-fast config accepts placeholder secrets (`SESSION_SECRET=change-me…` passes) — deferred: by design while vars are unconsumed per runbook; add strength/placeholder checks when Story 1.2 consumes SESSION_SECRET and Epic 2 consumes webhook secrets [server/internal/config/config.go:718]

## Dev Notes

### What this story is — and is not

This is the **walking skeleton**: repo, toolchain, CI, deploy, design tokens, one health endpoint. It creates **no domain tables, no auth, no WhatsApp code, no WebSocket endpoint** (`coder/websocket` is fetched as a dependency only; `/ws` is proxied in dev config but not implemented — Story 2.3 builds it). Story 1.2 creates `organizers`+`sessions`; don't anticipate schema here beyond the no-op init migration.

### Current state of the working directory

Near-greenfield, **not yet a git repo**: root contains an empty `README.md`, a leftover `test.js` (delete it), `docs/`, `.claude/`, `_bmad/`, `_bmad-output/` (planning artifacts — keep, commit). Everything in Tasks 2–5 is created from scratch; there is no existing code to preserve.

### Canonical initialization commands [Source: _bmad-output/planning-artifacts/architecture.md#Starter-Template-Evaluation]

```bash
# Backend (Go 1.26 — latest patch 1.26.5 as of 2026-07)
mkdir server && cd server
go mod init github.com/<github-user>/whatsapp-clickers
go get github.com/go-chi/chi/v5          # router (v5, zero-dep, net/http-native)
go get github.com/coder/websocket        # WebSocket (dependency only in this story)
go get github.com/jackc/pgx/v5           # PostgreSQL driver/toolkit
go get github.com/pressly/goose/v3       # migrations, embedded, run at boot

# Frontend (Node 22 LTS; Vite react-ts template)
npm create vite@latest web -- --template react-ts
# then Tailwind v4 + aliases (below) BEFORE:
npx shadcn@latest init
```

**shadcn on Vite prerequisites (2026 flow, Tailwind v4):** `npm install tailwindcss @tailwindcss/vite` → add `tailwindcss()` plugin to `vite.config.ts` → `@import "tailwindcss";` as the only Tailwind line in `index.css` → `@/*` path alias in **both** `tsconfig.json` and `tsconfig.app.json` **and** `vite.config.ts` `resolve.alias` → only then `npx shadcn@latest init`. There is **no `tailwind.config.js`** in Tailwind v4 — tokens live in `index.css` under `@theme`.

### Env vars (fail-fast list) [Source: architecture.md#Infrastructure-&-Deployment]

`DATABASE_URL`, `WHATSAPP_ACCESS_TOKEN`, `WHATSAPP_PHONE_NUMBER_ID`, `WHATSAPP_APP_SECRET`, `WHATSAPP_VERIFY_TOKEN`, `ANTHROPIC_API_KEY`, `SESSION_SECRET` — all required at boot even though only `DATABASE_URL` is consumed in this story (fail-fast is the AC; dummy values are fine locally and documented in `.env.example`). `PORT` is optional (default 8080; Railway injects it). Secrets never in code or logs.

### Design tokens — exact values (UX-DR1–UX-DR3) [Source: ux-designs/ux-whatsapp-clickers-2026-07-05/DESIGN.md frontmatter — authoritative]

Configure as CSS variables in `index.css` `@theme` (Tailwind v4) so both palettes are addressable as utilities:

- **Festival Green (Audience Display + brand):** `surface-base #F0FDF4`, `surface-raised #FFFFFF`, `green-900 #14532D`, `green-800 #166534`, `green-600 #16A34A`, `green-100 #D1FAE5`, `green-50 #F0FDF4`, `gold #FBBF24`, `text-primary #14532D`, `text-secondary #4B5563`, `ink-on-dark #FFFFFF`, `ink-on-dark-muted #FFFFFFB3`, `border-light #D1FAE5`
- **Semantic (separate from brand green):** `success #15803D` (⚠️ NOT #16A34A — re-pointed by accessibility review A18; the old value collided with green-600), `error #DC2626`, `warning #F59E0B`
- **Host Dashboard slate:** `host-surface #F8FAFC`, `host-border #E2E8F0`, `host-text #0F172A`, `host-text-secondary #64748B`
- **Radius:** sm 8px / md 12px / lg 16px / xl 22px / pill 9999px. **Spacing scale:** 4/8/12/16/24/32/48/64px — no values between stops
- **Typography:** `system-ui, -apple-system, 'Segoe UI', Arial, sans-serif` — NO webfont URLs ever (filter-safety). Weights: Display 900, Heading 800, UI 600, Body 500. Letter-spacing 0 on Hebrew. Line-height 1.38 heading / 1.55 body. All sizes in rem
- **Gold rule (UX-DR2):** gold appears in exactly two moments (timer ≤5s, winner) — this story only defines the token, never applies it

### Embed pattern (the one non-obvious build problem)

`go:embed` cannot reach outside the Go module (`/server`), and `web/dist` is outside it. Pattern: `server/internal/webdist/dist.go` with `//go:embed all:dist`; the `build` flow copies `web/dist/*` → `server/internal/webdist/dist/` before `go build`. Commit a **placeholder `dist/index.html`** so `go vet`/`go test`/`go build` succeed on a fresh clone without building the frontend; never commit real build output (CI/Railway always build fresh). SPA fallback: unknown non-`/api`/`/ws`/`/webhooks` paths serve `index.html` (client-side routes like `/display/:gameId` depend on this).

### Migrations + sqlc specifics

- goose v3, **embedded**: `migrations/embed.go` (`package migrations`, `//go:embed *.sql`) + `goose.SetBaseFS(...)` + `goose.Up` in `main.go` before the server listens — single-binary deploys need no migration files on disk. At least one `.sql` file must exist or the embed glob fails: hence `00001_init.sql` (no-op).
- sqlc 1.31.x, `sql_package: "pgx/v5"`, schema pointed at `migrations/`. `sqlc generate` fails with zero query files: hence `queries/health.sql` (`-- name: Ping :one` / `SELECT 1;`). CI diff-check: run `sqlc generate` then `git diff --exit-code` — pin the sqlc version in CI so local/CI output matches. `internal/store/gen/` is committed and never hand-edited.

### CI + deploy notes

- Vite react-ts template ships flat-config ESLint (`npm run lint`) and `tsc -b` in the build script — run `tsc -b --noEmit` as its own CI step anyway (the AC names it).
- Railway deploys via its GitHub integration on push to main with "wait for CI" enabled — no deploy step inside the workflow, no Railway token in GitHub secrets. EU region and Postgres are dashboard/operator steps (Task 8).
- `numReplicas: 1` in `railway.json` is architectural law: the future in-memory hub assumes a single instance; autoscaling would silently break WebSocket fan-out.
- Filter-safety grep (UX-DR4): the binding rule is "only `index.html` + `index.css` may reference assets; no file may reference an external URL asset." Start with the patterns in Task 6 over `web/` (excluding `node_modules`); refine as needed but the check must exist and must fail on a `fonts.googleapis.com` reference.

### Windows dev environment (this machine)

- The checkout path contains Hebrew (`c:\Users\שור\...`). Go, Node, Vite handle Unicode paths; if a tool fails inexplicably on paths, flag it in Dev Agent Record rather than silently relocating the project.
- `make` is not stock on Windows. Author the Makefile with POSIX-sh recipes (works under Git Bash / `choco install make`); document in README a no-make fallback: run `go run ./cmd/server` (in `/server`) and `npm run dev` (in `/web`) in two terminals.
- Local Postgres for dev: `docker run -e POSTGRES_PASSWORD=dev -p 5432:5432 postgres:17` or a Railway dev database — document both in README.

### Architecture compliance guardrails (violations = rework)

- **Glossary is law:** `Organizer`, `Participant`, `Game`, `Question` — never `host`/`player`/`quiz`/`session` as identifiers, tables, or routes. [Source: architecture.md#Naming-Patterns]
- **Wire format:** camelCase JSON; success = direct payload; errors = `{"error":{"code","message"}}` with SCREAMING_SNAKE code. The health endpoint follows this from day one.
- **Dependency direction:** `httpapi` → `store`; `store` is the only package importing pgx. Handlers thin.
- **Logging:** slog JSON from `cmd/server/main.go`; INFO for lifecycle events (boot, migrations ran, listening); a healthy run produces zero ERRORs.
- **Hebrew copy centralization:** `web/src/lib/strings.he.ts` exists from this story; no Hebrew literals in components. (`messages_he.go` arrives with Epic 2.)
- **Filter-safety:** zero external asset requests on any surface — enforced by the CI grep this story creates.
- **Before completion, all five must pass locally:** `gofmt`, `go vet`, `sqlc generate` (empty diff), `eslint`, `tsc --noEmit`. [Source: architecture.md#Enforcement-Guidelines]

### Custom domain (decision open — do not implement)

Operator owns `anash-list.com`; a `clickers.anash-list.com` subdomain via Railway custom domain + CNAME may come later. A path prefix is discouraged (SPA base-path, cookie, WS-URL complications). Railway's default domain suffices for the pilot. [Source: epics.md#Story-1.1 note]

### Testing standards

Go stdlib `testing`, co-located `_test.go` files. This story ships real tests: config fail-fast (missing var named in error) and health handler (200 + JSON shape). Frontend: no Vitest yet (per architecture, "added when first frontend tests land") — CI gates via eslint + tsc + build.

### Project Structure Notes

Target structure below is this story's slice of the architecture's canonical tree [Source: architecture.md#Complete-Project-Directory-Structure]:

```
whatsapp-clickers/
├── README.md  Makefile  railway.json  .gitignore
├── .github/workflows/ci.yml
├── server/
│   ├── go.mod  go.sum  sqlc.yaml  .env.example
│   ├── cmd/server/main.go
│   ├── migrations/00001_init.sql  migrations/embed.go
│   └── internal/
│       ├── config/config.go (+ config_test.go)
│       ├── httpapi/router.go (+ router_test.go)
│       ├── store/db.go  store/queries/health.sql  store/gen/ (generated)
│       └── webdist/dist.go  webdist/dist/index.html (placeholder)
└── web/
    ├── package.json  vite.config.ts  tsconfig.json  tsconfig.app.json  components.json
    ├── index.html (lang="he" dir="rtl")
    └── src/ main.tsx  app.tsx  index.css (tokens)  lib/strings.he.ts  lib/utils.ts  components/ui/
```

**Variances from the architecture tree (intentional, documented):** (1) `internal/webdist/` is added to solve the `go:embed` module-boundary constraint — the architecture names embedding but not its location. (2) Migration numbering starts at `00001_init.sql` (no-op pipeline proof); the architecture's illustrative `00001_organizers_sessions.sql` therefore lands as `00002_...` in Story 1.2. (3) `migrations/embed.go` added for embedded goose. Everything else matches the canonical tree exactly — later stories fill in the remaining files.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-1.1] — story + ACs (verbatim)
- [Source: _bmad-output/planning-artifacts/architecture.md#Starter-Template-Evaluation] — init commands, versions
- [Source: _bmad-output/planning-artifacts/architecture.md#Infrastructure-&-Deployment] — Railway, env vars, CI, replicas=1
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation-Patterns-&-Consistency-Rules] — naming, wire format, logging, enforcement
- [Source: _bmad-output/planning-artifacts/architecture.md#Complete-Project-Directory-Structure] — canonical tree
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/DESIGN.md] — token values (frontmatter authoritative), typography, filter-safety
- [Source: _bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md#4.7] — cross-cutting constraints
- Web research 2026-07-10: Go 1.26.5 current patch (go.dev/doc/go1.26); shadcn Vite install = Tailwind v4 `@tailwindcss/vite` + dual-tsconfig aliases (ui.shadcn.com/docs/installation/vite); sqlc stable 1.31.1 `pgx/v5` (docs.sqlc.dev); goose v3 embedded migrations via `SetBaseFS` (github.com/pressly/goose)

## Dev Agent Record

### Agent Model Used

claude-fable-5 (Claude Fable 5)

### Debug Log References

- **Go toolchain download blocked on this network** — `go get` failed with `zip: not a valid zip file` when auto-downloading go1.26.5 (filtered network intercepts Go's downloader; historical lock files show repeated past failures). Resolution: downloaded the official SDK zip via curl (`--ssl-no-revoke`), **verified SHA-256 against go.dev** (`97e6b2…fd38`, exact match), extracted to `%USERPROFILE%\sdk\go1.26.5`, installed the `golang.org/dl/go1.26.5` wrapper into `%USERPROFILE%\go\bin` and created the `.unpacked-success` marker. The system `go` (1.22.5, machine PATH) now transparently switches to 1.26.5 inside the repo — verified: `go version` in `/server` prints go1.26.5. Documented in README Windows notes.
- **`gh` CLI not installed** — GitHub repo created via REST API using the Git Credential Manager token (scopes: gist, repo, workflow). Repo: `avraham-shor/whatsapp-clickers`, **private** (safe default — flip to public at will).
- **Vite 8 template ships oxlint, not ESLint** — AC3 names eslint explicitly, so oxlint was replaced with ESLint flat config (typescript-eslint, react-hooks `configs.flat.recommended`, react-refresh vite); `react-refresh/only-export-components` disabled for `src/components/ui/**` (shadcn exports cva variants alongside components — standard practice).
- **TypeScript 6 deprecates `baseUrl`** (TS5101) — `paths` now resolve relative to the tsconfig, so `baseUrl` was dropped from both tsconfigs.
- **shadcn CLI (v4.13) changes** — `-b` now selects primitives library (`radix` chosen = classic shadcn); init offered native `--rtl` flag (enabled, `components.json` has `"rtl": true`); preset `nova` used non-interactively. Init injected `@fontsource-variable/geist` — removed (DESIGN.md mandates system-ui only) and `--font-sans` re-pointed to the system stack.
- **`go mod tidy` drops import-less deps** — `coder/websocket` (dependency-only this story) pinned via `server/deps.go` under a never-built `pin_deps` tag.
- **Railway Nixpacks build failed on stale toolchains (2026-07-13)** — operator's first deploy resolved Go 1.22.1 + Node 18 (nixpacks 1.41 snapshot), tripping EBADENGINE on Vite 8/ESLint 10 deps. Replaced with a multi-stage `Dockerfile` (node:22-alpine → golang:1.26.5-alpine → alpine:3.21, CGO disabled), `railway.json` switched to `"builder": "DOCKERFILE"`, `nixpacks.toml` removed. Image verified locally: build succeeds, container boots against Postgres, migrations idempotent (count:0 on already-migrated DB), health 200, SPA RTL served.

### Completion Notes List

- Tasks 1–7 complete; **Task 8 (operator/human steps) is pending Avraham** — full runbook in README §"Operator runbook" + §"Meta WhatsApp Business setup".
- Walking skeleton verified end-to-end locally against real Postgres 17 (Docker): boot logs (slog JSON) show config → db connected → `migrations applied count=1` → listening; `GET /api/health` → 200 `{"status":"ok","db":"ok"}`; `/` serves the SPA with `lang="he" dir="rtl"`; `/display/abc` falls back to index.html; `/api/nope` → 404 `{"error":{"code":"NOT_FOUND",…}}`; missing env vars → exit 1 naming every missing var (AC5).
- CI run on GitHub Actions (commit 68ab168): **all three jobs green** — backend (go vet, go test, sqlc v1.31.1 diff-check), frontend (npm ci, eslint, tsc -b --noEmit, build), filter-safety grep. Grep verified locally to fail on a `fonts.googleapis` reference and pass on the clean tree.
- Local quality gates all pass: `gofmt` (clean), `go vet`, `go test ./...` (config + httpapi suites), `sqlc generate` (empty diff), `eslint`, `tsc -b --noEmit`, `npm run build`.
- Wire-format/architecture guardrails honored: camelCase direct payload + SCREAMING_SNAKE error envelope from the health endpoint; `store` is the only package importing pgx (goose runs inside `store.Migrate`); Hebrew copy only in `strings.he.ts`; gold token defined but never applied.
- **Intentional variances (documented):** (1) local Node is 25.6 (CI pins 22.x — the gate is authoritative); (2) oxlint→ESLint swap per AC3; (3) `baseUrl` removed per TS6; (4) goose runs via the v3 Provider API (structured results, no stray plain-text logs) rather than global `SetBaseFS` — same embedded-FS boot behavior the task specifies; (5) Railway builds via a multi-stage `Dockerfile`, not Nixpacks — Nixpacks was tried first and replaced 2026-07-13 because its toolchain snapshot was stale (Go 1.22 / Node 18); see Debug Log.
- **Task 8 executed by operator 2026-07-13, deployment verified live:** https://whatsapp-clickers-production.up.railway.app — `/api/health` → 200 `{"db":"ok","status":"ok"}` (proves DB + goose ran), `/` serves the RTL Hebrew SPA, `/display/test` → 200 (SPA fallback), `/api/nope` → 404 error envelope. First Nixpacks deploy failed on stale toolchains → fixed with Dockerfile (see Debug Log); second failure was missing env vars (fail-fast worked as designed) → operator set Variables → healthy.
- **Meta subtask resolution (owner decision, 2026-07-13):** the pricing re-verification was completed early against official Meta docs — service messages free AND service-window replies exempt from tier messaging limits; the ≈₪0 conclusion holds and is stronger than assumed. The setup kickoff itself was **consciously deferred by the owner** (no Facebook account / no registered business yet): plan = personal FB account created now to age, free Business Portfolio + test number at Epic 2 start (~30 min, no business entity needed), business verification deferred until the system proves itself. An unverified WABA suffices for a modest pilot (user-initiated flow). Documented in README runbook and project memory; revisit at Story 2.1.

### File List

New:
- .github/workflows/ci.yml
- .gitignore (created in baseline commit 01552a0, Task 1)
- .dockerignore
- Dockerfile
- Makefile
- railway.json
- server/.env.example
- server/cmd/server/main.go
- server/deps.go
- server/go.mod
- server/go.sum
- server/internal/config/config.go
- server/internal/config/config_test.go
- server/internal/httpapi/router.go
- server/internal/httpapi/router_test.go
- server/internal/store/db.go
- server/internal/store/gen/db.go (generated, committed)
- server/internal/store/gen/health.sql.go (generated, committed)
- server/internal/store/gen/models.go (generated, committed)
- server/internal/store/migrate.go
- server/internal/store/queries/health.sql
- server/internal/webdist/dist.go
- server/internal/webdist/dist/index.html (placeholder)
- server/migrations/00001_init.sql
- server/migrations/embed.go
- server/sqlc.yaml
- web/.gitignore
- web/README.md
- web/components.json
- web/eslint.config.js
- web/index.html
- web/package.json
- web/package-lock.json
- web/public/favicon.svg
- web/src/App.tsx
- web/src/components/ui/button.tsx
- web/src/index.css
- web/src/lib/strings.he.ts
- web/src/lib/utils.ts
- web/src/main.tsx
- web/src/vite-env.d.ts
- web/tsconfig.json
- web/tsconfig.app.json
- web/tsconfig.node.json
- web/vite.config.ts

Modified:
- README.md (was empty — full project README + operator runbook)
- _bmad-output/implementation-artifacts/sprint-status.yaml (story status tracking: in-progress, then review)

Deleted:
- test.js (unrelated debris)

## Change Log

- 2026-07-10: Story 1.1 Tasks 1–7 implemented and verified (walking skeleton: repo, GitHub, Go server, React SPA, embed, CI green on Actions, Railway config). Task 8 operator steps handed off. Commits: 01552a0 (baseline), 68ab168 (skeleton).
- 2026-07-13: Railway build fix — Nixpacks (stale Go 1.22/Node 18) replaced with multi-stage Dockerfile, verified locally end-to-end; nixpacks.toml removed (transient, never a File List entry in final state).
- 2026-07-13: Task 8 completed by operator — Railway project live (EU, Postgres, replicas=1), deployment verified at whatsapp-clickers-production.up.railway.app (health 200, RTL SPA, migrations ran). Meta pricing re-verified from official docs (≈₪0 confirmed + tier-limit exemption for service-window replies); Meta account setup deferred to Epic 2 start by owner decision. All ACs satisfied → Status: review.
- 2026-07-13: Adversarial code review (Blind Hunter + Edge Case Hunter + Acceptance Auditor): no AC violations; 15 patch findings applied and verified (graceful shutdown + server timeouts, SPA asset-404, 405 JSON envelope, health ping timeout, goose session lock, sqlc gate `git add -N`, filter-safety grep hardened + built-bundle scan, gofmt in CI, TS `strict`, webdist build hygiene, `make dev` exit status, non-root container, doc fixes); 2 deferred to deferred-work.md (spacing-scale enforcement → 1.3, placeholder-secret validation → 1.2/2.1). All gates green (gofmt, vet, test incl. 2 new, eslint, tsc strict, build, filter-safety). Status: done.
