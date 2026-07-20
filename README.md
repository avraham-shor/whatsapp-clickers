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

1. `cp server/.env.example server/.env` and fill values. `DATABASE_URL` and
   the four `WHATSAPP_*` values must be real (see "Meta WhatsApp Business
   setup" below — the server refuses to boot on placeholder WhatsApp values
   since Story 2.1); `ANTHROPIC_API_KEY` can stay `dummy` until Epic 3. Load
   the file into your shell before running the server (e.g.
   `set -a; . server/.env; set +a` in Git Bash).
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
   `DATABASE_URL` (reference the Railway Postgres variable), the four
   `WHATSAPP_*` values (real test-number values — see "Meta WhatsApp
   Business setup" above), `ANTHROPIC_API_KEY` (dummy is still fine — Epic 3
   is the first consumer), and `SESSION_SECRET` (long random string).
   Railway injects `PORT` itself.

   ⚠️ **`SESSION_SECRET` is enforced since Story 1.2**: the server refuses to
   boot if it is shorter than 32 characters or still starts with `change-me`.
   The value set during the 1.1 setup was the placeholder — **replace it in
   Railway Variables with a real random value (`openssl rand -base64 48`)
   BEFORE merging Story 1.2 to `main`**, or the next deploy will fail fast at
   boot (by design).

   ⚠️ **`WHATSAPP_*` values are enforced since Story 2.1**: the server
   refuses to boot if any of the four is still `dummy` or starts with
   `change-me`. Set real values from the "Meta WhatsApp Business setup"
   runbook above in Railway Variables **before merging Story 2.1 to
   `main`**, or the next deploy will fail fast at boot (by design — same
   pattern as the `SESSION_SECRET` warning above). Production callback URL
   is the Railway domain + `/webhooks/whatsapp` — point Meta's webhook
   configuration at it when moving from local dev-tunnel testing to
   deployed testing (one app has one callback URL; dev tunnel and
   production alternate by re-running "Verify and save", which is
   acceptable for the pilot).
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

### Meta WhatsApp Business setup

Setup runbook for the Meta Cloud API (direct — no BSP middleman). A free
**test number** is sufficient for all of Epic 2 development (up to 5
registered recipients); a real dedicated number and business verification
are deliberately deferred until the system proves itself (pricing
re-verified 2026-07-13: service-window replies are free and exempt from
tier messaging limits, so the ≈₪0 assumption holds).

1. **Business Portfolio**: `business.facebook.com` → Create a business
   portfolio. No registered legal entity (עוסק) is required at this stage.
2. **Meta app**: `developers.facebook.com` → My Apps → Create App → type
   **Business**, linked to the portfolio above.
3. **WhatsApp product**: app dashboard → Add product → WhatsApp → Set up.
   This creates a WhatsApp Business Account, a free test phone number, a
   temporary access token (~24h), and the Phone Number ID.
4. **Register test recipients** (up to 5 — the hard cap for a test number):
   WhatsApp → API Setup → the "To" field → Manage phone number list → add
   and verify each recipient's real WhatsApp number.
5. **The four env values** (`server/.env` and Railway Variables — see
   `server/.env.example` for the full description of each):
   - `WHATSAPP_ACCESS_TOKEN` — a **System User** permanent token (Business
     settings → Users → System users → Add, role Admin → Assign assets →
     the app → Generate token → expiry never → permissions
     `whatsapp_business_messaging` + `whatsapp_business_management`). The
     API Setup screen's temporary token also works but expires in ~24h —
     regenerate it every dev session if you use that instead.
   - `WHATSAPP_PHONE_NUMBER_ID` — App dashboard → WhatsApp → API Setup
     (shown under the test number; the ID, never the display number).
   - `WHATSAPP_APP_SECRET` — App dashboard → App settings → Basic → App
     secret (click Show). HMAC-verifies every inbound webhook.
   - `WHATSAPP_VERIFY_TOKEN` — you invent this (e.g. `openssl rand -hex
     16`); paste the same value into the webhook subscription screen below.
6. **Webhook subscription**: app dashboard → WhatsApp → Configuration → set
   the **Callback URL** (see "Local webhook development" below for the
   local value) and **Verify Token** (your `WHATSAPP_VERIFY_TOKEN`) →
   **Verify and save** (fires our `GET /webhooks/whatsapp` handshake — a
   green check means it passed) → subscribe to the **`messages`** webhook
   field.

⚠️ From Story 2.1 onward the server refuses to boot with placeholder
WhatsApp values (`dummy` / `change-me...`) — real test-number values from
steps 1–5 above are required for `make dev`.

### Local webhook development

Meta must reach your local server over HTTPS, so a tunnel stands in for a
public URL during development:

1. Install cloudflared: Windows `winget install Cloudflare.cloudflared`.
2. `cloudflared tunnel --url http://localhost:8080` — no account needed
   (quick tunnel). It targets the **Go server on :8080 directly**, not the
   Vite dev server on :5173 — the Vite proxy is for browser dev, not Meta.
3. Use the printed `https://<random>.trycloudflare.com/webhooks/whatsapp`
   as the Callback URL in step 6 above.
4. The quick-tunnel URL changes every run — re-run "Verify and save" each
   dev session (~30 seconds).

**Network filter note** (confirmed 2026-07-19 against an Etrog-type
filter): if your network filter refuses `facebook.com` /
`business.facebook.com` / `developers.facebook.com` / `graph.facebook.com`
outright, the Meta *dashboard* screens above (portfolio, app, test number,
tokens, webhook config) need an unfiltered connection — e.g. a mobile
hotspot for that one session; it's dashboard-only traffic, so connection
quality doesn't matter. The tunnel and local server themselves are
unaffected: cloudflared talks to Cloudflare, not Meta, and Meta's inbound
webhook calls arrive through the tunnel regardless of the local network's
outbound filter.

## Custom domain (open decision — not implemented)

Railway's default domain suffices for the pilot. If a custom domain is wanted
later, use a `clickers.anash-list.com` subdomain (CNAME → Railway); avoid path
prefixes.
