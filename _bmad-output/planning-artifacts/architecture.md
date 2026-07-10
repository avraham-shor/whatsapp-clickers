---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
inputDocuments:
  - _bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md
  - _bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/addendum.md
  - _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/DESIGN.md
  - _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md
  - _bmad-output/planning-artifacts/briefs/brief-whatsapp-clickers-2026-06-18/brief.md
workflowType: 'architecture'
lastStep: 8
status: 'complete'
completedAt: '2026-07-09'
project_name: 'whatsapp-clickers'
user_name: 'Avraham Shor'
date: '2026-07-08'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**

18 FRs across 6 feature groups, mapping to these architectural capabilities:

- **WhatsApp Inbound (FR-1, FR-2, FR-3, FR-5, FR-7, FR-8):** webhook-driven message
  intake — parse JOIN codes, answers (א–ד / 1–4 / free text ≤200 chars), and noise;
  every inbound message gets a reply (Universal Reply); idempotent registration;
  first-valid-answer-wins with server receipt timestamps; single server-side cutoff
  per question.
- **WhatsApp Outbound (FR-4, FR-6):** burst dispatch on question open (one message ×
  all participants, 5s/95% target), individual acks, post-Reveal result messages,
  final results. ~P×Q×2–3 messages per game (~2,000 for a 60-person, 10-question game).
- **Live Game Engine (FR-13, FR-17, FR-18):** server-authoritative state machine
  (lobby → question open → closed → reveal → leaderboard → … → game over), driven
  exclusively by explicit Organizer actions; scoring with Speed Bonuses ordered by
  server receipt timestamp; live leaderboard computation.
- **Real-time Fan-out (FR-9, FR-10, FR-13):** Audience Display renders game state with
  ≤1s transitions and auto-reconnect; dashboard shows live answer counts; lobby
  counter increments ≤3s after JOIN.
- **Grading Pipeline (FR-15, FR-16):** MCQ mechanical grading; free-text three-stage
  validation (Exact → Fuzzy → AI Semantic), graded as answers arrive, matching stage
  recorded, fail-closed degradation when AI unavailable, Reveal gated on completion.
- **Authoring (FR-11, FR-12, FR-14):** authenticated CRUD — games, questions, accepted
  answers, scoring config; Question Bank package import (copy semantics); post-game
  results view. Pilot auth is lightweight (manually provisioned).

**Non-Functional Requirements:**

- **Latency targets:** 1s Audience Display state transitions; 3s lobby-count update;
  5s/95% WhatsApp question delivery (provider-dependent — validate at 80 and 500).
- **Reliability:** server-authoritative state throughout; zero lost acknowledged
  answers (SM-4); all clients (dashboard, display) can disconnect/reconnect without
  state corruption; degraded modes silent and self-healing — a live event cannot pause.
- **Scale:** pilot 30–80 participants/game, single game at a time; architecture must
  not preclude 500 (WhatsApp provider throughput is the binding constraint).
- **Cost envelope:** WhatsApp messages and AI validation calls are the two variable
  costs, both ∝ participants × questions; per-message thrift must not cut reply
  guarantees (SM-C2). Budget ceiling open (OQ-6).
- **Privacy:** phone numbers are PII and the identity key; store minimum (number,
  display name, per-game scores); no third-party sharing.
- **Hebrew/RTL/filter-safety:** all surfaces Hebrew-native RTL; zero external asset
  dependencies (fonts/images/CDNs); no imagery of people. Fuzzy matching and AI
  validation must handle Hebrew text (final-letter forms, nikud, spelling variants).

**Scale & Complexity:**

- Primary domain: full-stack web + messaging integration (webhook backend, real-time
  push, two organizer web surfaces, WhatsApp as sole participant surface)
- Complexity level: medium — low data volume and user counts, elevated by real-time
  coordination, external messaging dependency, and correctness guarantees
- Estimated architectural components: ~8 (WhatsApp gateway in/out, game engine/state
  machine, grading pipeline, real-time push channel, persistence, host dashboard app,
  audience display app, auth)

### Technical Constraints & Dependencies

- **WhatsApp provider (OQ-1 — owned by this workflow):** official WhatsApp Business
  Cloud API vs. BSP. Unofficial gateways are ruled out (ToS violation; mid-event ban
  risk). The 24-hour service window opened by each participant's inbound JOIN likely
  covers the whole game with free-form messages — must verify against current Meta
  pricing. Per-number messages/second rate limit constrains question-open bursts and
  therefore game pacing at 500 participants.
- **AI provider for semantic validation:** Hebrew-capable LLM; invoked only on
  Exact/Fuzzy miss; timeout/error ⇒ fail-closed two-stage grading, logged.
- **Greenfield:** no existing codebase; no team-size constraint documented —
  solo-developer pilot assumed.
- **Deferred but must-not-preclude:** self-serve purchase/payments, result export,
  marketing site, 500-participant scale.

### Cross-Cutting Concerns Identified

1. **Real-time state propagation** — one organizer action fans out to WhatsApp
   (per-participant messages), Audience Display, and dashboard; each channel has its
   own latency target and failure mode.
2. **Idempotency & ordering** — idempotent JOIN, one-answer-per-participant, server
   receipt timestamps, deterministic tie-breaking; correctness of the Speed Bonus and
   the answer cutoff depends on these.
3. **Rate limits & cost** — every feature that sends messages must respect provider
   throughput and the message-volume envelope; queueing/backpressure is a system-wide
   concern, not a feature-local one.
4. **Hebrew text processing** — normalization for fuzzy matching, RTL rendering on web
   surfaces, Hebrew microcopy in WhatsApp templates.
5. **PII discipline** — phone numbers flow through webhooks, logs, and storage;
   minimal retention posture applies everywhere.
6. **Silent resilience** — auto-reconnect, degraded grading, no user-visible error
   states during a live event; observability must be built in (the degradation is
   logged even when invisible).

## Starter Template Evaluation

### Primary Technology Domain

Full-stack web + messaging integration: Go backend (WhatsApp webhooks, game engine,
WebSocket push) + TypeScript React frontend (Host Dashboard + Audience Display),
PostgreSQL, deployed as a single process on a simple PaaS (Railway/Render/Fly.io).
Driven by user preference: Go backend, TS frontend, "keep it simple", PostgreSQL.

### Starter Options Considered

1. **go-blueprint CLI** (`go-blueprint create --framework chi --driver postgres`) —
   scaffolds a Go project with framework/DB choices. Rejected: generates opinionated
   boilerplate we'd immediately rewrite (its layout, its config style); adds a third-
   party dependency to project genesis for ~10 files we can define precisely ourselves.
2. **Echo v5 framework starter** — batteries-included (middleware, built-in WebSocket).
   Rejected as base: more framework surface than needed; chi + stdlib is the
   minimalist fit for "keep it simple" (chi is a router over net/http, zero deps).
3. **Official minimal toolchain (SELECTED)** — `go mod init` + standard Go layout for
   the backend; `create-vite` (react-ts) + `shadcn init` for the frontend. In the Go
   ecosystem this IS the idiomatic starter; on the frontend these are the official
   generators the UX spec's shadcn/ui system expects.

### Selected Starter: Official minimal toolchain (Go standard layout + create-vite + shadcn)

**Rationale for Selection:**

- Go culture is stdlib-first; a hand-defined ~10-file layout beats fighting generated
  boilerplate, and gives AI agents an exact, documented structure with zero mystery files.
- The UX spec mandates shadcn/ui; `create-vite` + `npx shadcn init` is that system's
  official installation path (Vite template supported).
- One repo, two top-level apps (`/server` Go, `/web` React), one deployable: the Go
  binary serves the built SPA, terminates WebSockets, and receives WhatsApp webhooks —
  a single long-running process matches the PaaS preference and the real-time NFRs.

**Initialization Commands:**

```bash
# Backend (Go 1.26)
mkdir server && cd server
go mod init github.com/<org>/whatsapp-clickers
go get github.com/go-chi/chi/v5          # router (v5, zero-dep, net/http-native)
go get github.com/coder/websocket        # WebSocket (actively maintained)
go get github.com/jackc/pgx/v5           # PostgreSQL driver/toolkit

# Frontend (Vite ~9.x, Node 20.19+/22.12+)
npm create vite@latest web -- --template react-ts
cd web && npx shadcn@latest init         # shadcn/ui per UX spec (Vite template)
```

**Architectural Decisions Provided by Starter:**

**Language & Runtime:**
- Go 1.26 backend (single static binary; goroutines for burst dispatch & timers)
- TypeScript (strict) React frontend via Vite react-ts template

**Styling Solution:**
- Tailwind CSS + shadcn/ui (installed by `shadcn init`), design tokens overridden
  per DESIGN.md frontmatter (Festival Green palette, system-ui font stack — no
  external font/CDN dependencies, satisfying filter-safety)

**Build Tooling:**
- `go build` → single binary embedding/serving `web/dist` static assets
- Vite build for the SPA (both surfaces: Host Dashboard + Audience Display routes)

**Testing Framework:**
- Go: standard `testing` package (stdlib-first, no framework)
- Web: Vitest (Vite-native) — added when first frontend tests land

**Code Organization:**
- Monorepo: `/server` (Go: `cmd/`, `internal/`) + `/web` (Vite React)
- chi v5 for HTTP routing; coder/websocket for realtime; pgx v5 for Postgres

**Development Experience:**
- Vite dev server with HMR proxying API/WS to the Go server
- Single-process production model — dev/prod parity on a simple PaaS

**Note:** Project initialization using these commands should be the first
implementation story.

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (Block Implementation):**
- WhatsApp provider: Meta WhatsApp Business Cloud API, direct (resolves OQ-1)
- Live game state model: Postgres-authoritative with in-memory fan-out hub
- AI validation provider: Claude Opus 4.8 via official Go SDK
- Hosting: Railway (single service + managed Postgres)

**Important Decisions (Shape Architecture):**
- Auth: password + server-side session cookie (pilot-scoped)
- SQL access: sqlc-generated type-safe queries over pgx v5
- Realtime: WebSocket pushing full game-state snapshots
- Frontend: single SPA, React Router v8, TanStack Query v5

**Deferred Decisions (Post-MVP):**
- Multi-instance scale-out (Redis pub/sub) — single instance suffices for 30–80,
  and remains viable at 500; deferred until >1 concurrent large game
- Payments, result export, marketing site — commercial phase (per PRD §6.2)
- Higher WhatsApp throughput tier — revisit at the 500-participant target

### Data Architecture

- **PostgreSQL (Railway managed)** — source of truth for everything, including
  live game state. **An answer is persisted before its "התקבל ✓" ack is sent**
  (SM-4: an acknowledged answer is never lost). Server restart mid-game recovers
  full state from the DB.
- **In-memory game hub** — per-game live state (open question, counts, deadline)
  cached in the Go process for WebSocket fan-out and fast reads; always rebuildable
  from Postgres. No Redis — unnecessary at this scale, revisit only for multi-instance.
- **sqlc 1.31 + pgx v5** — SQL-first, compile-time-checked queries; no ORM.
- **Migrations: goose v3** — plain SQL files in `server/migrations`, run on deploy.
- **Integrity in the schema**: `UNIQUE (question_id, participant_id)` enforces
  one-answer-per-participant (FR-8) at the DB level; answer receipt timestamps
  (`timestamptz`) recorded server-side at webhook receipt, ordering ties broken by
  a monotonic sequence (FR-17 server-order tie-breaking).
- **Validation strategy**: validate at boundaries (webhook payloads, API handlers);
  trust internal code. Free-text answers capped at 200 chars at intake (FR-5).

### Authentication & Security

- **Organizer auth**: manually provisioned username + password (argon2id hash),
  HTTP-only Secure session cookie, server-side sessions in Postgres. No self-serve
  signup in the pilot (per PRD §4.4).
- **Audience Display**: same-origin route opened from an authenticated dashboard
  session — inherits the session cookie; no pairing flow (per FR-9 assumption).
- **Ownership**: every game/question query scoped by `organizer_id` (FR-11).
- **Webhook security**: verify Meta's `X-Hub-Signature-256` HMAC (app secret) on
  every inbound webhook; static verify-token for webhook registration.
- **PII discipline**: phone numbers stored once in `participants`; redacted in
  logs (last-4 only); TLS terminated by Railway; secrets via env vars only.

### API & Communication Patterns

- **WhatsApp integration (OQ-1 RESOLVED): Meta Cloud API, direct.**
  - *Cost*: every Participant initiates contact (JOIN), opening a 24-hour service
    window; all game messages are free-form replies inside it → **message cost ≈ 0**
    under current Meta pricing (service messages free since Nov 2024; per-template
    pricing since Jul 2025 doesn't apply — we send no templates mid-game).
    Resolves the OQ-6 cost concern for messaging.
  - *Throughput*: default 80 msg/sec per number (inbound+outbound). Question-open
    burst to 80 participants ≈ 1–2s — inside the FR-4 5s/95% target. At 500
    participants ≈ 7s: the known constraint for the commercial phase (auto-upgrade
    to 1,000 mps requires volumes we won't hit); pacing/design accommodates it.
  - *Mechanics*: inbound via webhook (`POST /webhooks/whatsapp`); **dedupe by
    WhatsApp message ID** (Meta retries webhooks — idempotent processing is
    mandatory and also satisfies FR-1 idempotent JOIN). Outbound via a worker pool
    with a token-bucket rate limiter under the 80 mps ceiling; per-message retry
    with backoff; failures logged, never block the game loop.
  - *Number*: dedicated WhatsApp Business number (brand front door — continuity
    matters per addendum). Unofficial gateways remain ruled out (ToS ban risk).
- **Dashboard API**: REST JSON under `/api/*` (chi). Uniform error envelope
  `{"error": {"code", "message"}}`; cookies for auth; no GraphQL.
- **Realtime**: WebSocket at `/ws` (coder/websocket) for Host Dashboard and
  Audience Display. Server pushes **full game-state snapshots** (not diffs): a
  reconnecting display re-renders current state from the first message —
  satisfying FR-9 auto-reconnect with zero client-side state reconciliation.
  Organizer actions go over REST (auditable, simple); WS is push-only.
- **AI Semantic validation (FR-16)**: **Claude Opus 4.8** (`claude-opus-4-8`)
  via the official Go SDK (`anthropic-sdk-go`). Strict structured output for the
  verdict (`{"correct": bool}`); the prompt provides question, Accepted Answers,
  and the participant's answer — the model judges equivalence only, never invents
  correctness. Per-call timeout (~5s) with **fail-closed degradation** to
  two-stage grading, logged with the matching stage recorded. Invoked only on
  Exact→Fuzzy miss; grading runs as answers arrive, so Reveal wait covers only
  last-second answers. Cost: ~200 tokens/call ⇒ negligible at P×Q scale.
- **Hebrew fuzzy stage (FR-16)**: normalization (trim, final-letter forms
  ך/ם/ן/ף/ץ → כ/מ/נ/פ/צ, nikud stripping, punctuation) + Levenshtein distance
  threshold — pure Go, no external service.

### Frontend Architecture

- **Single SPA** (Vite + React 19 + TypeScript strict) serving both surfaces:
  `/` Host Dashboard (builder, lobby, live control, results) and `/display/:gameId`
  Audience Display (output-only, full-screen, projection scale).
- **Routing**: React Router v8 (8.x, ESM-only, requires React 19).
- **Server state**: TanStack Query v5 for CRUD (games, questions, bank).
- **Live state**: one WebSocket hook per surface feeding React state via
  `useSyncExternalStore`; no Redux/Zustand — the server snapshot is the store.
  Auto-reconnect with exponential backoff built into the hook (FR-9).
- **Design system**: shadcn/ui + Tailwind, tokens overridden per DESIGN.md
  (Festival Green, gold rules, system-ui font stack, `dir="rtl"`, `lang="he"`).
  Zero external asset requests — filter-safety is enforced at build time.
- **Timer**: Audience Display countdown driven by server-sent deadline timestamp
  (client renders remaining time; server cutoff is authoritative per FR-7).

### Infrastructure & Deployment

- **Railway, EU region**: one service — the Go binary serving the SPA (embedded
  `web/dist`), `/api`, `/ws`, and `/webhooks/whatsapp` — plus managed PostgreSQL.
  Single instance by design (the in-memory hub assumes it).
- **CI/CD**: GitHub Actions — `go test` + `go vet` + frontend build on PR;
  deploy to Railway on merge to main. goose migrations run at service start.
- **Configuration**: env vars only — `DATABASE_URL`, `WHATSAPP_ACCESS_TOKEN`,
  `WHATSAPP_PHONE_NUMBER_ID`, `WHATSAPP_APP_SECRET`, `WHATSAPP_VERIFY_TOKEN`,
  `ANTHROPIC_API_KEY`, `SESSION_SECRET`.
- **Observability**: Go `slog` structured JSON logs → Railway logs. Every silent
  degradation is logged (AI-stage fallback, WS reconnects, webhook retries,
  send failures) — the PRD's "degraded modes must be silent and self-healing"
  requires that they be *visible in logs* even when invisible in the room.
- **Scaling path**: 30–80 now on one instance; 500 works on the same architecture
  (Go handles the connections trivially; WhatsApp 80 mps is the only bottleneck).
  Beyond one concurrent mega-game: add Redis pub/sub + multi-instance — a bounded,
  known refactor that this design does not preclude.

### Decision Impact Analysis

**Implementation Sequence:**
1. Repo scaffold (starter commands) + Railway service + Postgres + CI
2. Schema + migrations + sqlc queries (games, questions, participants, answers, sessions)
3. Auth + Host Dashboard CRUD (builder, question bank)
4. WhatsApp webhook in/out + JOIN flow + Universal Reply (needs Meta business setup — long lead item, start early)
5. Game engine state machine + answer intake + grading pipeline (exact → fuzzy → AI)
6. WebSocket hub + Audience Display + live control
7. Scoring, leaderboard, reveal/results messages, winner takeover

**Cross-Component Dependencies:**
- The **game engine** is the center: WhatsApp intake, grading, scoring, and both
  web surfaces all consume its state transitions; it owns the single server-side
  answer cutoff (FR-7) that scoring and grading depend on.
- **Webhook dedupe + persisted-before-ack** together guarantee SM-4 end-to-end.
- **Full-snapshot WS protocol** couples Audience Display and dashboard to one
  serialization of game state — one place to maintain, trivial reconnects.
- **Meta business verification + number acquisition** is the only external
  process on the critical path — it gates step 4 and should start immediately.

## Implementation Patterns & Consistency Rules

### Pattern Categories Defined

**Critical Conflict Points Identified:** 12 areas where AI agents could make
different choices — naming across two languages, wire formats, state enums,
Hebrew copy placement, error/loading handling, test location, ID strategy.

### Naming Patterns

**Database (PostgreSQL):**
- Tables: plural `snake_case` — `games`, `questions`, `participants`, `answers`,
  `organizers`, `sessions`, `question_packages`
- Columns: `snake_case`; PKs are `id UUID DEFAULT gen_random_uuid()`; FKs are
  `<singular>_id` (`game_id`, `question_id`); timestamps `created_at` /
  `updated_at` (`timestamptz`, UTC)
- Indexes: `idx_<table>_<cols>` (`idx_answers_question_participant`)
- Enums as `TEXT` + `CHECK` constraints, not Postgres enum types (simpler migrations)

**API:**
- REST: plural kebab-case nouns, actions as sub-resources —
  `GET /api/games/{gameID}`, `POST /api/games/{gameID}/questions/{questionID}/reveal`
- Path params in chi style `{gameID}`; query params camelCase
- WebSocket: single endpoint `/ws?gameId=...&role=display|host`
- Webhooks: `/webhooks/whatsapp` (outside `/api`, no session auth — HMAC instead)

**Code:**
- Go: standard conventions — `MixedCaps` exported / `mixedCaps` unexported,
  short lowercase package names (`game`, `wa`, `grading`), no `util` package
- TypeScript: `PascalCase` component names, kebab-case filenames
  (`answer-distribution.tsx` — shadcn/ui convention), hooks `use-game-socket.ts`
  exporting `useGameSocket`
- Shared vocabulary is the PRD Glossary: `Organizer`, `Participant`, `Game`,
  `Question`, `Answer`, `Reveal`, `SpeedBonus`, `Leaderboard` — never synonyms
  (`host`, `player`, `quiz`, `session` are forbidden as identifiers)

### Structure Patterns

- Go tests co-located: `engine_test.go` next to `engine.go` (Go standard)
- TS tests co-located: `answer-distribution.test.tsx` next to source (Vitest)
- Frontend organized by feature: `web/src/features/{builder,lobby,live,display,auth}/`
  plus `web/src/components/ui/` (shadcn primitives — never hand-edited) and
  `web/src/lib/` (api client, ws hook, types)
- Go organized by domain under `server/internal/` (structure detailed in the
  next section); HTTP handlers thin — business logic lives in domain packages

### Format Patterns

- **JSON wire format: camelCase** (`gameId`, `createdAt`, `speedBonus`) — the
  only consumer is our TS frontend; Go structs carry explicit `json:"..."` tags.
  DB stays snake_case; sqlc bridges the two.
- **Success responses**: direct payload, no wrapper — `GET /api/games/{id}` →
  the game object; lists return `{"items": [...]}`
- **Errors**: always `{"error": {"code": "GAME_NOT_FOUND", "message": "..."}}`
  with correct HTTP status; `code` is SCREAMING_SNAKE, stable, machine-readable;
  `message` is developer-facing English (user-facing Hebrew lives in the frontend)
- **Dates**: RFC 3339 UTC strings in JSON (`"2026-07-09T18:30:00Z"`); clients
  render local time. Countdown deadlines sent as absolute timestamps, never
  "seconds remaining"
- **Game state enum** (single source of truth, identical strings in Go, TS, DB):
  `draft → lobby → question_open → question_closed → revealed → leaderboard → finished`
  (`leaderboard` is skippable per FR-13; `revealed` may advance directly)

### Communication Patterns

- **WebSocket protocol**: server→client only, one message type —
  `{"type": "snapshot", "seq": 41, "state": <GameSnapshot>}` containing the full
  render-ready state for the current game phase. No diffs, no client→server
  messages (organizer actions are REST). Clients drop snapshots with stale `seq`.
- **Internal events (Go)**: the game engine exposes state transitions on a
  channel; WhatsApp dispatch and the WS hub subscribe. Event names past-tense:
  `QuestionOpened`, `QuestionClosed`, `AnswerRevealed`, `GameFinished`.
- **Hebrew copy centralized**:
  - WhatsApp message templates: one Go file `server/internal/wa/messages_he.go` —
    every outbound string in one place (OQ-7 conversation-design revisions touch
    one file)
  - Frontend strings: `web/src/lib/strings.he.ts` — no Hebrew literals inside
    components
- **Logging (slog, JSON)**: canonical keys `game_id`, `question_id`,
  `participant_id` (redacted phone: `phone_last4`), `wa_message_id`, `stage`
  (grading stage), `event`. Levels: `INFO` state transitions, `WARN` degradations
  (AI fallback, WS drop, send retry), `ERROR` only for invariant violations —
  a live event must produce zero ERRORs on a healthy run.

### Process Patterns

- **Error handling (Go)**: errors wrapped with `fmt.Errorf("context: %w", err)`;
  handlers map domain errors → error envelope via one central mapper; panics
  recovered by chi middleware → 500 + log, never process exit mid-game
- **Degradation rule**: any failure inside the game loop (AI timeout, WA send
  failure) degrades per its FR and logs WARN — it never blocks a state transition
  and never surfaces as an error to the room
- **Loading states (frontend)**: TanStack Query's built-in `isPending`/`isError`
  only — no hand-rolled `loading` flags; mutations disable their trigger button;
  live surfaces render "מתחבר..." only when the WS is down (per EXPERIENCE.md
  error copy: describe what's happening, never apologize)
- **Retry ownership**: WhatsApp sends retry server-side (3 attempts, backoff);
  WS reconnects client-side (exponential backoff, indefinite); REST mutations do
  NOT auto-retry (organizer actions must stay explicit — FR-13)
- **Validation timing**: reject at the boundary (HTTP 4xx / WhatsApp reply hint);
  DB constraints are the last line, not the first

### Enforcement Guidelines

**All AI Agents MUST:**
- Use PRD Glossary terms verbatim in all identifiers, tables, and routes
- Persist an answer before acknowledging it (SM-4 — no exceptions)
- Route every outbound WhatsApp string through `messages_he.go`, every UI string
  through `strings.he.ts`
- Emit state changes only through the game engine — no component or handler
  mutates game state directly
- Run `gofmt`, `go vet`, `sqlc generate` (diff must be empty), `eslint`,
  `tsc --noEmit` before completing any story — CI enforces all five

**Pattern updates**: this section is the single source of truth; changing a
pattern means updating it here first, then code.

### Pattern Examples

**Good:**
- `POST /api/games/{gameID}/questions/{questionID}/close` → engine transition →
  snapshot broadcast → WA result dispatch subscribes to `QuestionClosed`
- `{"error": {"code": "ANSWER_ALREADY_RECORDED", "message": "participant already answered question"}}`

**Anti-Patterns:**
- ❌ `players` table, `quizId` field, `HostDashboard` component named `AdminPanel`
  (glossary violations)
- ❌ Sending "התקבל ✓" before the answer row is committed
- ❌ WS messages like `{"type": "participant_joined", "delta": ...}` (diff protocol)
- ❌ Hebrew string literals inside a React component or a Go handler
- ❌ A `utils` package / `helpers.ts` junk drawer

## Project Structure & Boundaries

### Complete Project Directory Structure

```
whatsapp-clickers/
├── README.md
├── .gitignore
├── Makefile                        # dev/build/test/generate targets
├── railway.json                    # Railway service config (build + start commands)
├── .github/
│   └── workflows/
│       └── ci.yml                  # go test/vet, sqlc diff, eslint, tsc, build
├── server/
│   ├── go.mod
│   ├── go.sum
│   ├── sqlc.yaml                   # sqlc config → internal/store/gen
│   ├── .env.example                # documented env vars, no secrets
│   ├── cmd/
│   │   └── server/
│   │       └── main.go             # wire-up: config → store → engine → hubs → router; runs goose migrations
│   ├── migrations/
│   │   ├── 00001_organizers_sessions.sql
│   │   ├── 00002_games_questions.sql
│   │   ├── 00003_participants_answers.sql
│   │   └── 00004_question_packages.sql
│   └── internal/
│       ├── config/
│       │   └── config.go           # env parsing, fail-fast on missing vars
│       ├── store/                  # ── data boundary: ONLY package touching Postgres
│       │   ├── db.go               # pgx pool setup
│       │   ├── queries/            # hand-written SQL (sqlc input)
│       │   │   ├── games.sql
│       │   │   ├── questions.sql
│       │   │   ├── participants.sql
│       │   │   ├── answers.sql
│       │   │   ├── organizers.sql
│       │   │   └── packages.sql
│       │   └── gen/                # sqlc output — never hand-edited
│       ├── game/                   # ── the center: server-authoritative engine
│       │   ├── engine.go           # state machine, transitions, answer cutoff (FR-7, FR-13)
│       │   ├── engine_test.go
│       │   ├── events.go           # QuestionOpened, QuestionClosed, AnswerRevealed, GameFinished
│       │   ├── answers.go          # intake: first-valid-wins, persist-before-ack (FR-5, FR-8)
│       │   ├── scoring.go          # points, speed bonus, ties, leaderboard (FR-17, FR-18)
│       │   ├── scoring_test.go
│       │   └── snapshot.go         # GameSnapshot builder (the one WS payload)
│       ├── grading/                # ── validation pipeline (FR-15, FR-16)
│       │   ├── pipeline.go         # exact → fuzzy → AI orchestration; stage recording
│       │   ├── normalize.go        # Hebrew normalization (finals, nikud, punctuation)
│       │   ├── normalize_test.go
│       │   ├── fuzzy.go            # Levenshtein w/ threshold
│       │   ├── fuzzy_test.go
│       │   └── ai.go               # Claude Opus 4.8 call, timeout, fail-closed
│       ├── wa/                     # ── WhatsApp boundary (FR-1–FR-8 messaging side)
│       │   ├── client.go           # Cloud API outbound HTTP client
│       │   ├── webhook.go          # inbound handler: HMAC verify, dedupe by message ID
│       │   ├── inbound.go          # parse → JOIN / answer / unrecognized (Universal Reply)
│       │   ├── inbound_test.go
│       │   ├── dispatch.go         # worker pool, token-bucket ≤80mps, retry w/ backoff
│       │   └── messages_he.go      # ALL outbound Hebrew copy (single file, per patterns)
│       ├── auth/
│       │   ├── auth.go             # argon2id verify, session create/lookup
│       │   └── auth_test.go
│       ├── httpapi/                # ── HTTP boundary: thin handlers only
│       │   ├── router.go           # chi mux: /api, /ws, /webhooks, static SPA
│       │   ├── middleware.go       # session auth, request logging, panic recovery
│       │   ├── errors.go           # domain error → {"error":{code,message}} mapper
│       │   ├── auth_handlers.go    # login/logout
│       │   ├── games.go            # builder CRUD (FR-11)
│       │   ├── questions.go        # question CRUD incl. accepted answers
│       │   ├── packages.go         # Question Bank browse/import (FR-12)
│       │   ├── control.go          # start/open/close/reveal/next/end (FR-13)
│       │   └── results.go          # post-game summary (FR-14)
│       └── ws/                     # ── realtime boundary
│           ├── hub.go              # per-game connection registry, broadcast
│           └── handler.go          # /ws upgrade, role=host|display, snapshot-on-connect
├── web/
│   ├── package.json
│   ├── vite.config.ts              # dev proxy: /api,/ws → localhost Go server
│   ├── tsconfig.json
│   ├── components.json             # shadcn/ui config
│   ├── index.html                  # lang="he" dir="rtl"
│   └── src/
│       ├── main.tsx                # router mount
│       ├── app.tsx                 # React Router v8 route tree
│       ├── index.css               # Tailwind + DESIGN.md tokens (Festival Green, gold)
│       ├── components/
│       │   └── ui/                 # shadcn primitives — generated, never hand-edited
│       ├── lib/
│       │   ├── api.ts              # typed fetch client + TanStack Query setup
│       │   ├── use-game-socket.ts  # WS hook: connect, reconnect w/ backoff, snapshot state
│       │   ├── types.ts            # GameSnapshot, GameState enum, API types (mirror Go)
│       │   └── strings.he.ts       # ALL UI Hebrew copy
│       └── features/
│           ├── auth/
│           │   └── login-page.tsx
│           ├── builder/            # FR-11, FR-12
│           │   ├── games-list-page.tsx
│           │   ├── game-editor-page.tsx
│           │   ├── question-editor.tsx
│           │   └── package-import-dialog.tsx
│           ├── lobby/              # FR-13 (lobby half)
│           │   └── lobby-page.tsx  # live count, participant list, "התחל משחק"
│           ├── live/               # FR-13 (control half)
│           │   ├── control-page.tsx    # one-primary-action state machine (EXPERIENCE.md)
│           │   └── response-stats.tsx  # live answered count/%
│           ├── results/            # FR-14
│           │   └── results-page.tsx
│           └── display/            # FR-9, FR-10 — Audience Display
│               ├── display-page.tsx    # /display/:gameId — full-screen, stage switcher
│               ├── lobby-stage.tsx     # JOIN instructions + climbing counter
│               ├── question-stage.tsx  # question, options, timer, answer count
│               ├── timer-ring.tsx      # authoritative countdown, gold ≤5s
│               ├── reveal-stage.tsx    # correct answer + distribution
│               ├── leaderboard-stage.tsx
│               └── winner-stage.tsx    # takeover + CSS geometric celebration
```

### Architectural Boundaries

**API Boundaries:**
- `/api/*` — session-authenticated REST for the Organizer (dashboard only)
- `/ws` — session-authenticated WebSocket, push-only snapshots
- `/webhooks/whatsapp` — HMAC-authenticated, Meta-facing only
- `/*` — static SPA (embedded `web/dist`)

**Component Boundaries (Go):**
- `store` is the only package importing pgx; everything else uses its typed API
- `game` owns all state transitions and the snapshot; it imports `store` and
  `grading`, and emits events — it never imports `wa`, `ws`, or `httpapi`
- `wa` and `ws` are transport adapters: they subscribe to `game` events and call
  engine methods; neither holds game state
- `httpapi` handlers translate HTTP ↔ engine/store calls; no business logic
- Dependency direction: `httpapi`/`wa`/`ws` → `game` → `grading`/`store` (one way)

**Component Boundaries (Web):**
- `features/*` may import `lib` and `components/ui`; features never import each
  other — shared logic is promoted to `lib`
- `display/*` renders exclusively from the WS snapshot (no TanStack Query) —
  it must work with nothing but a `gameId` and a socket
- All Hebrew copy flows from `strings.he.ts`; all server data types from `types.ts`

**Data Boundaries:**
- One schema, one database; `answers` is append-once (UNIQUE constraint)
- In-memory hub state is a cache of DB truth — rebuilt on boot, never diverges
  as source

### Requirements to Structure Mapping

| PRD Feature Group | Server | Web |
|---|---|---|
| §4.1 Registration & Inbox (FR-1–3) | `wa/webhook.go`, `wa/inbound.go`, `wa/messages_he.go`, `game` (participant registry) | lobby counter via snapshot |
| §4.2 WhatsApp Gameplay (FR-4–8) | `wa/dispatch.go`, `game/answers.go`, `game/engine.go` | — |
| §4.3 Audience Display (FR-9–10) | `ws/`, `game/snapshot.go` | `features/display/*` |
| §4.4 Host Dashboard (FR-11–14) | `httpapi/*`, `auth/` | `features/{auth,builder,lobby,live,results}` |
| §4.5 Answer Validation (FR-15–16) | `grading/*` | reveal-stage shows distribution |
| §4.6 Scoring & Leaderboard (FR-17–18) | `game/scoring.go` | `leaderboard-stage`, `winner-stage` |

**Cross-Cutting Concerns:**
- Universal Reply (FR-2): `wa/inbound.go` default branch — every parse path ends
  in a reply
- SM-4 (zero lost answers): `game/answers.go` (persist-before-ack) +
  `wa/webhook.go` (dedupe) + `answers` UNIQUE constraint
- Filter-safety: `index.html` + `index.css` only — no other file may reference
  an external URL asset (CI-greppable)
- Observability: slog configured in `cmd/server/main.go`; conventions per
  Implementation Patterns

### Integration Points

**Internal Communication:**
Organizer action (REST) → `httpapi/control.go` → `game.Engine` transition →
persists via `store` → emits event → `ws.Hub` broadcasts new snapshot AND
`wa.Dispatcher` sends the corresponding messages. One action, one transition,
two transports.

**External Integrations:**
- Meta Cloud API (in: webhook; out: `wa/client.go`) — the only inbound dependency
- Claude API (`grading/ai.go`) — outbound only, degradable
- Railway Postgres (`store/db.go`)

**Data Flow (answer path):**
WhatsApp reply → webhook (HMAC, dedupe) → `wa/inbound.go` parse →
`game/answers.go` (open? first? cutoff?) → INSERT answer → ack via dispatcher →
`grading` pipeline (async) → grade stored → live count snapshot → on Reveal:
scores computed, results dispatched, snapshot broadcast.

### Development Workflow Integration

- **Dev**: `make dev` runs the Go server (port 8080) + `vite dev` (port 5173,
  proxying `/api`, `/ws`, `/webhooks`); local Postgres via Docker or Railway dev DB;
  WhatsApp webhooks tunneled (e.g. `cloudflared`) during integration work
- **Build**: CI builds `web/` → `go build` with `web/dist` embedded (`embed.FS`)
  → single binary; `sqlc generate` diff-checked
- **Deploy**: Railway builds from `railway.json`; goose migrations run at boot
  before the server accepts traffic

## Architecture Validation Results

### Coherence Validation ✅

**Decision Compatibility:**
- Go 1.26 + chi v5 + coder/websocket + pgx v5 + sqlc 1.31: all current, all
  interoperate (sqlc targets pgx/v5 natively)
- React Router v8 requires React 19 + Vite 7+ — satisfied by create-vite 9.x
  template; TanStack Query v5 requires React 18+ ✅
- Single-instance in-memory hub ⇄ Railway: `railway.json` must pin replicas to 1
  (documented; autoscaling would break WS fan-out)
- Claude Go SDK (`anthropic-sdk-go`) supports structured outputs used by
  `grading/ai.go` ✅

**Pattern Consistency:**
camelCase wire / snake_case DB bridged by sqlc + struct tags; glossary-driven
naming applies identically in Go, TS, SQL, and routes; full-snapshot WS protocol
matches the "server-authoritative, reconnect-friendly" decision.

**Structure Alignment:**
Dependency rule (`httpapi`/`wa`/`ws` → `game` → `grading`/`store`) is enforceable
by import direction alone; every pattern has a physical home (copy files, error
mapper, engine events).

### Requirements Coverage Validation ✅

**Functional Requirements Coverage (18/18):**

| FR | Architectural home |
|---|---|
| FR-1 Join | `wa/inbound.go` → `game` registry; snapshot updates lobby count (≤3s target: in-process, ~ms) |
| FR-2 Universal Reply | `wa/inbound.go` — every parse branch terminates in a reply, incl. media/sticker/empty → help |
| FR-3 Spectator | `game` registry with participant role; final-results dispatch includes spectators |
| FR-4 Delivery | `wa/dispatch.go` worker pool; 80 participants ≈ 1–2s at 80 mps (inside 5s/95%) |
| FR-5 Intake+ack | `game/answers.go` persist-before-ack; 200-char cap; format hints |
| FR-6 Results | `AnswerRevealed` event → per-participant grade/points/rank messages |
| FR-7 Cutoff | single server-side deadline in `game/engine.go` (earlier of timer/close) |
| FR-8 One answer | first-valid-wins + `UNIQUE (question_id, participant_id)` |
| FR-9 Projection | `/display/:gameId`, snapshot-on-connect, client auto-reconnect; 1s target trivially met in-process |
| FR-10 Stage content | one component per state in `features/display/` |
| FR-11 Builder | `httpapi/games.go`+`questions.go`, organizer-scoped queries |
| FR-12 Bank import | `httpapi/packages.go` — import copies rows, never references |
| FR-13 Live control | `httpapi/control.go` → explicit engine transitions; no auto-advance anywhere |
| FR-14 Results screen | `httpapi/results.go` + `features/results/` |
| FR-15 MCQ grading | `grading/pipeline.go` mechanical branch |
| FR-16 3-stage validation | exact→fuzzy→AI, stage recorded, fail-closed; Reveal control gated on grading completion (engine tracks outstanding grades) |
| FR-17 Scoring | `game/scoring.go`; receipt-timestamp order, monotonic-seq ties, shared ranks |
| FR-18 Leaderboard/winner | scoring → snapshot → leaderboard/winner stages + final WA messages |

**Non-Functional Requirements Coverage:**
- Latency: all fan-out in-process (1s/3s targets have order-of-magnitude margin);
  WhatsApp burst math documented against the 5s/95% target
- Reliability (SM-4): persist-before-ack + webhook dedupe + DB-rebuildable hub +
  panic recovery + client auto-reconnect
- Scale: 30–80 trivial; 500 works with ~7s delivery (documented constraint);
  scale-out path (Redis) explicitly deferred, not precluded
- Cost: messaging ≈ ₪0 (service window); AI ≈ negligible (invoked only on
  exact/fuzzy miss, ~200 tokens) — OQ-6 substantially de-risked
- Privacy: minimal PII schema, log redaction, env-only secrets
- Hebrew/RTL/filter-safety: system fonts, zero external assets (CI-greppable),
  centralized Hebrew copy, `dir="rtl"` at the root

### Implementation Readiness Validation ✅

- All critical decisions carry named versions (Go 1.26, chi v5, pgx v5,
  sqlc 1.31, goose v3, React 19, React Router v8, TanStack Query v5, Vite 9.x,
  claude-opus-4-8)
- Patterns cover the 12 identified conflict points with good/anti examples
- Structure is file-level specific; every FR maps to named files
- Enforcement is mechanical: gofmt, go vet, sqlc diff, eslint, tsc in CI

### Gap Analysis Results

**Critical Gaps:** none.

**Important Gaps (tracked, non-blocking):**
1. **OQ-7 UX designs pending** — WhatsApp conversation copy and Audience Display
   per-state layouts await the UX revision. Architecture isolates the blast
   radius (`messages_he.go`, `strings.he.ts`, one component per stage); blocks
   story creation for §4.2/§4.3 content, not architecture (per PRD).
2. **Spectator schema detail** — FR-3 needs an explicit `participants.role`
   (`player`/`spectator`) column; called out here so the epics phase includes it.
3. **Meta pricing re-verification** — the ≈₪0 cost conclusion rests on current
   service-window pricing; re-verify against Meta's published rates when the
   Business account is created (before first pilot event).

**Nice-to-Have (future):**
- Login rate-limiting (pilot: handful of manually provisioned accounts)
- Postgres backup/restore drill (Railway automated backups; verify before pilot)
- Load test at 500 simulated participants before the commercial phase

### Architecture Completeness Checklist

**Requirements Analysis**
- [x] Project context thoroughly analyzed
- [x] Scale and complexity assessed
- [x] Technical constraints identified
- [x] Cross-cutting concerns mapped

**Architectural Decisions**
- [x] Critical decisions documented with versions
- [x] Technology stack fully specified
- [x] Integration patterns defined
- [x] Performance considerations addressed

**Implementation Patterns**
- [x] Naming conventions established
- [x] Structure patterns defined
- [x] Communication patterns specified
- [x] Process patterns documented

**Project Structure**
- [x] Complete directory structure defined
- [x] Component boundaries established
- [x] Integration points mapped
- [x] Requirements to structure mapping complete

### Architecture Readiness Assessment

**Overall Status:** READY FOR IMPLEMENTATION

**Confidence Level:** high — the three tracked gaps are external (UX revision,
Meta account process) or one-column schema details; none changes a decision.

**Key Strengths:**
- One deployable, one state authority, one snapshot protocol — minimal moving
  parts for a solo-run live-event system
- OQ-1 resolved with verified cost (~₪0) and throughput (80 mps) numbers
- SM-4 (zero lost answers) is guaranteed by construction, not by care
- Glossary-enforced vocabulary + centralized Hebrew copy keep AI agents and the
  OQ-7 UX revision from colliding

**Areas for Future Enhancement:**
- Redis pub/sub + multi-instance when concurrent large games arrive
- Result export, payments, marketing site (commercial phase)
- Question-open pacing strategy for 500 participants (~7s delivery)

### Implementation Handoff

**AI Agent Guidelines:**
- Follow all architectural decisions exactly as documented
- Use implementation patterns consistently across all components
- Respect project structure and boundaries (import direction is law)
- Refer to this document for all architectural questions

**First Implementation Priority:**
1. Run the starter initialization commands (Starter Template Evaluation section)
2. In parallel — start the Meta WhatsApp Business setup (business verification +
   dedicated number): the only external lead time on the critical path
