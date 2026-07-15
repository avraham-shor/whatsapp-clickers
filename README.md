# WhatsApp Clickers

Live trivia game platform played over WhatsApp (Hebrew, RTL): organizers author
games on a web dashboard, participants play from their phones via WhatsApp
messages, and an audience display shows the game on a big screen.

## Repository layout

```
server/   Go 1.26 backend — chi router, pgx (Postgres), goose migrations, sqlc
web/      React 19 + TypeScript SPA — Vite 8, Tailwind v4, shadcn/ui (RTL, Hebrew)
_bmad-output/, docs/   Planning artifacts (PRD, architecture, UX) — kept in repo
```

The production artifact is a **single Go binary** that embeds the built SPA
(`server/internal/webdist`) and runs migrations at boot.

## Local development

### Prerequisites

- **Go 1.26.x** — `go.mod` pins `go 1.26.5`; any Go ≥ 1.21 auto-switches
  toolchains if it can. See Windows notes below if toolchain download fails.
- **Node 22+** and npm
- **PostgreSQL** — easiest via Docker:
  `docker run -d -e POSTGRES_PASSWORD=dev -e POSTGRES_DB=whatsapp_clickers -p 5432:5432 postgres:17`
  (or use a Railway dev database and copy its `DATABASE_URL`)
- **make** (optional) — on Windows: Git Bash + `choco install make`
- **sqlc** (only when changing SQL): `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`

### Setup

1. `cp server/.env.example server/.env` and fill values (dummy values are fine
   for everything except `DATABASE_URL` in this phase). Load it into your shell
   before running the server (e.g. `set -a; . server/.env; set +a` in Git Bash).
2. `cd web && npm install`

### Run

```
make dev        # Go API on :8080 + Vite dev server on :5173 (proxies /api, /ws, /webhooks)
```

No-make fallback (two terminals):

```
cd server && go run ./cmd/server
cd web && npm run dev
```

Open http://localhost:5173 (dev) — the Vite proxy forwards API calls to :8080.

### Other targets

```
make build      # web build → embed into server → server/bin/server
make test       # go test ./...
make lint       # go vet + gofmt + eslint + tsc
make generate   # sqlc generate (commit the gen/ output)
```

After `make build`, `server/internal/webdist/dist/` holds real build output —
do **not** commit it; only the placeholder `index.html` is tracked.

### Windows notes (Hebrew user path, filtered network)

- The checkout path contains Hebrew characters — Go, Node and Vite handle this;
  if a tool fails mysteriously on paths, report it rather than moving the repo.
- If `go` cannot auto-download the 1.26.5 toolchain (this happens on some
  filtered networks: `zip: not a valid zip file`), install the SDK manually:
  1. Download `go1.26.5.windows-amd64.zip` from https://go.dev/dl/ and verify
     its SHA-256 against the value published on that page.
  2. Extract it to `%USERPROFILE%\sdk\go1.26.5` and create an empty marker file
     `%USERPROFILE%\sdk\go1.26.5\.unpacked-success`.
  3. `go install golang.org/dl/go1.26.5@latest` (puts a `go1.26.5` wrapper in
     `%USERPROFILE%\go\bin`, which is on PATH). The system `go` then switches
     to 1.26.5 automatically inside this repo. (Already done on the dev machine.)

## Wire format & conventions (binding)

- Domain names: `Organizer`, `Participant`, `Game`, `Question` — never
  host/player/quiz/session as identifiers.
- JSON: camelCase; success = direct payload; errors =
  `{"error":{"code":"SCREAMING_SNAKE","message":"..."}}`.
- All user-facing Hebrew strings live in `web/src/lib/strings.he.ts` — no
  Hebrew literals in components.
- **Filter-safety:** no file may reference an external URL asset (webfonts,
  CDN scripts, remote images). CI enforces this with a grep gate. System font
  stack only.

## CI

`.github/workflows/ci.yml` gates every PR and push to main:
`gofmt`, `go vet`, `go test`, sqlc generate diff-check (pinned v1.31.1),
eslint, `tsc -b --noEmit`, frontend build, and the filter-safety grep (source
tree + built bundle).

## Deployment (Railway)

Merges to `main` auto-deploy via Railway's GitHub integration ("wait for CI"
enabled) — the workflow itself contains no deploy step and no Railway token.
The build is defined by the multi-stage `Dockerfile` (node:22 SPA build →
golang:1.26.5 binary with embedded SPA → minimal alpine runtime); Railway is
pointed at it via `railway.json`. (Nixpacks was tried first but its toolchains
are stale — Go 1.22 / Node 18.) `numReplicas: 1` is **mandatory** (the future
in-memory WebSocket hub assumes a single instance — do not enable autoscaling).

### Operator runbook — one-time Railway setup (human steps)

1. Create a Railway project in the **EU region**.
2. Add a service linked to the GitHub repo `avraham-shor/whatsapp-clickers`,
   deploy on push to `main`, and enable **"wait for CI"**.
3. Add a managed **PostgreSQL** database to the project.
4. On the service, set environment variables (see `server/.env.example`):
   `DATABASE_URL` (reference the Railway Postgres variable), the five WhatsApp/
   Anthropic secrets (dummy values acceptable until Epics 2–3), and
   `SESSION_SECRET` (long random string). Railway injects `PORT` itself.

   ⚠️ **`SESSION_SECRET` is enforced since Story 1.2**: the server refuses to
   boot if it is shorter than 32 characters or still starts with `change-me`.
   The value set during the 1.1 setup was the placeholder — **replace it in
   Railway Variables with a real random value (`openssl rand -base64 48`)
   BEFORE merging Story 1.2 to `main`**, or the next deploy will fail fast at
   boot (by design).
5. Confirm replicas = 1 (Settings → Scaling).
6. Verify the deployed URL: `/` serves the SPA (Hebrew, RTL), `GET /api/health`
   returns 200, and boot logs show `migrations applied`.

### Provisioning an Organizer

There is no self-serve signup in the pilot; accounts are created with the
provisioning CLI. It is safe to re-run — an existing username gets its
password replaced. The `organizers` table must exist first: the server
applies migrations at boot, so boot the Story 1.2 (or later) server once
against that database before provisioning.

Local (Git Bash; password via stdin — never as an argument):

```
cd server
echo -n 'the-password' | DATABASE_URL='postgres://postgres:dev@localhost:5432/whatsapp_clickers' go run ./cmd/provision -username avraham
```

Production: copy the **public** `DATABASE_URL` from the Railway Postgres
service's Connect tab and run the same command with it. Running the command
without piping input prompts for the password interactively (input is echoed
— prefer the piped form on a shared screen).

### Meta WhatsApp Business setup — deferred to Epic 2 start (owner decision 2026-07-13)

Setup plan: a personal Facebook account was created now so it ages; at Epic 2
start, create a free Business Portfolio + test number (~30 min, no registered
business entity needed). Full business verification is deferred until the
system proves itself — an unverified WABA suffices for a modest pilot on the
user-initiated flow. Pricing was re-verified against official Meta docs
(2026-07-13): service messages are free AND service-window replies are exempt
from tier messaging limits, so the ≈₪0 assumption holds. Revisit at Story 2.1.

## Custom domain (open decision — not implemented)

Railway's default domain suffices for the pilot. If a custom domain is wanted
later, use a `clickers.anash-list.com` subdomain (CNAME → Railway); avoid path
prefixes.
