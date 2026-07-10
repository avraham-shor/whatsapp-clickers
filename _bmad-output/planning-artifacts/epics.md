---
stepsCompleted: [1, 2, 3]
inputDocuments:
  - _bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md
  - _bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/addendum.md
  - _bmad-output/planning-artifacts/architecture.md
  - _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/DESIGN.md
  - _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md
---

# whatsapp-clickers - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for whatsapp-clickers, decomposing the requirements from the PRD, UX Design if it exists, and Architecture requirements into implementable stories.

**Input-document precedence (per PRD §0 and addendum):** the PRD wins on conflict. EXPERIENCE.md's participant mobile screens, participant web flows, marketing-site surfaces, and "הורד תוצאות" (export) CTA are **superseded** by the 2026-07-08 WhatsApp-only pivot and are excluded from this breakdown. DESIGN.md's visual identity remains binding (now applied to the Audience Display at projection scale); EXPERIENCE.md (revised 2026-07-09) is authoritative for experience and behavior on all three surfaces.

**OQ-7 resolution:** the UX revision for (a) WhatsApp conversation design and (b) Audience Display per-state layouts landed 2026-07-09 and was validated at the reviewer gate. Canonical sources: EXPERIENCE.md's WhatsApp message-templates table (all outbound copy), its Audience Display stage table (per-state content), and DESIGN.md A19 (projection sizing). Implementation stays isolated to `server/internal/wa/messages_he.go`, `web/src/lib/strings.he.ts`, and one component per display stage.

## Requirements Inventory

### Functional Requirements

FR-1: A Participant can register for a Game by sending `JOIN <code>` to the platform's WhatsApp number. Valid code during lobby → welcome reply with the Participant's name; lobby count (dashboard + Audience Display) increments within 3 seconds. Invalid/expired code → Hebrew "code not found" reply. Re-sending JOIN is idempotent (no duplicate, same welcome reply).

FR-2: Universal Reply — any person sending any message to the platform number receives a response. Unrecognized text gets a short Hebrew help reply (how to join, where to get a code); no inbound message class results in silence, including media, stickers, and empty messages.

FR-3: A person sending JOIN after the Game started is registered as a Spectator: informed the Game already started, and sent the final results message at game end. No mid-game content for Spectators.

FR-4: When the Organizer opens a Question, every registered Participant receives the question text — and for MCQ, the lettered options (א–ד) — as a WhatsApp message. Dispatch begins immediately; target delivery within 5 seconds to 95% of Participants. The message states how to answer and the time limit; the authoritative countdown runs on the Audience Display.

FR-5: A Participant can answer an open Question by replying in WhatsApp; the system immediately acknowledges receipt ("התקבל ✓") without disclosing the grade. MCQ replies accepted as letter (א–ד) or digit (1–4); Free-Text replies limited to 200 characters (longer → rejection with hint); answers recorded with server receipt timestamp; second answer → "first answer already counts" reply; unparseable MCQ reply → format hint, Participant may still answer.

FR-6: After Reveal, each Participant who answered receives their grade (נכון / לא נכון), points earned including any Speed Bonus, and their current rank. Grades are never sent before Reveal. Non-answerers receive no per-question message. At game end, every Participant and Spectator receives a final results message (score, rank, winner's name).

FR-7: An answer arriving after the Question closed is not counted and receives a polite Hebrew "question closed" reply. The answer window ends at a single server-side cutoff: the earlier of timer expiry or the Organizer's explicit close action, judged by server receipt timestamp.

FR-8: A Participant has exactly one recorded answer per Question — the first valid reply received; later attempts get the "already answered" feedback.

FR-9: An Organizer can open the Audience Display from the Host Dashboard as a separate browser window suitable for a projector; it renders current Game state, transitioning automatically on Organizer actions with no interaction of its own. State transitions render within 1 second; the Display's countdown timer is the authoritative visible timer, shifting to gold urgency at ≤5 seconds; on connection drop it auto-reconnects and re-renders current state without Organizer intervention.

FR-10: The Audience Display shows, per Game state: lobby (JOIN instructions + live participant count), open Question (question, MCQ options, timer, live answer count), Reveal (correct answer marked; answer distribution), Leaderboard (top ranks with movement indicators), winner takeover (winner name, score, celebration). No per-Participant private information on the shared screen. (Per-state content and layout per EXPERIENCE.md's stage table + A19 projection ramp.)

FR-11: An Organizer can sign in to their dashboard and create a Game, add and edit MCQ and Free-Text Questions (including Accepted Answers and per-question time limits), configure scoring rules, and receive its JOIN Code. A Game and its Questions are accessible only to the Organizer who owns them.

FR-12: An Organizer can browse the Question Bank and import a Question Package into a Game, then mix, reorder, edit, or delete imported Questions alongside custom ones. Imported Questions are copies — editing them does not modify the Question Bank. (Pilot content: at least one complete multigenerational holiday Package must exist before the first pilot event; sourcing is OQ-4.)

FR-13: An Organizer can open the lobby (see participant count and list live), launch the Audience Display, start the Game, and drive it through the state machine: open Question → close Question → Reveal → (Leaderboard) → next Question or end Game — Leaderboard skippable. Live response count and percentage update on the dashboard while a Question is open; no state can be skipped accidentally; every advance is an explicit Organizer action.

FR-14: At game end the Organizer sees a results summary (final Leaderboard, per-question response rates) on the dashboard. Result export/download is out of scope (commercial phase).

FR-15: The system grades MCQ answers against the Organizer-defined correct option; grades are published to Participants only at Reveal.

FR-16: The system grades a Free-Text answer via Exact → Fuzzy → AI Semantic stages against the Accepted Answers, recording which stage matched. Exact match never invokes later stages; AI invoked only when Exact and Fuzzy both fail; AI unavailability (timeout/error) → graded by first two stages only, degradation logged (fail-closed); all grading for a Question completes before Reveal is published — the Reveal control activates only once every received answer is graded.

FR-17: An Organizer can set points per correct answer and Speed Bonus values; the system awards them automatically based on answer receipt timestamps. Speed Bonuses go to at most the first, second, and third correct answers; ordering by server receipt timestamp, ties broken by server processing order; equal scores share a Leaderboard rank.

FR-18: The system maintains a live Leaderboard published after each Reveal on the Audience Display and names the winner at game end everywhere: the winner takeover on the Audience Display and the final results message in every Participant's WhatsApp.

### NonFunctional Requirements

NFR-1: Latency — lobby count updates within 3s of JOIN; Audience Display state transitions within 1s of Organizer action; WhatsApp question delivery within 5s to 95% of Participants (provider-dependent; validate at 80 and at 500).

NFR-2: Reliability — server-authoritative Game state throughout; dashboard or Audience Display can disconnect and reconnect without score loss or state corruption; an acknowledged answer is never lost (SM-4); a live event cannot be paused for debugging — degraded modes must be silent and self-healing (and visible in logs).

NFR-3: Scale — pilot operates at 30–80 Participants per Game, single Game at a time; architecture must not preclude the 500-Participant commercial target. WhatsApp per-number throughput (80 msg/sec default) is the binding constraint.

NFR-4: Privacy — phone numbers are PII and the identity key; store the minimum (number, display name, per-Game scores); no third-party sharing; no marketing use without consent; phone numbers redacted in logs (last-4 only).

NFR-5: Hebrew / RTL / filter-safety — every surface (WhatsApp copy, Audience Display, Host Dashboard) is Hebrew-native RTL; zero external asset dependencies (fonts/images/CDNs); no imagery of people; system-ui font stack.

NFR-6: Cost — WhatsApp messages and AI validation calls are the two variable costs, both ∝ Participants × Questions (~2,000 messages for a 60-person, 10-question Game); per-message thrift must not cut reply guarantees (SM-C2). Messaging ≈ ₪0 under the 24-hour service window (re-verify against Meta pricing before first pilot event).

NFR-7: Security — organizer auth via manually provisioned credentials (argon2id hash, HTTP-only Secure session cookie, server-side sessions); every game/question query scoped by organizer; Meta webhook HMAC (`X-Hub-Signature-256`) verified on every inbound request; secrets via env vars only; TLS terminated by the platform.

NFR-8: Observability — structured JSON logs (slog) with canonical keys; every silent degradation logged (AI-stage fallback, WS reconnects, webhook retries, send failures); INFO for state transitions, WARN for degradations, ERROR only for invariant violations — a healthy live run produces zero ERRORs.

NFR-9: AI grading counter-metric (SM-C1) — do not tune the AI Semantic stage toward leniency; record the matching stage so Organizer-reported wrong grades (both directions) can be audited.

### Additional Requirements

**From Architecture — project genesis (Epic 1 Story 1 material):**

- Starter: official minimal toolchain — no third-party scaffold. Backend: `go mod init` + chi v5 + coder/websocket + pgx v5 (Go 1.26, standard layout `cmd/` + `internal/`). Frontend: `npm create vite@latest web -- --template react-ts` + `npx shadcn@latest init`. Exact init commands documented in architecture.md "Starter Template Evaluation". **Project initialization using these commands must be the first implementation story.**
- Monorepo: `/server` (Go) + `/web` (Vite React SPA); one deployable — the Go binary embeds and serves `web/dist`, terminates WebSockets, and receives WhatsApp webhooks.
- Hosting: Railway (EU region), single service + managed PostgreSQL; **replicas pinned to 1** (in-memory hub assumes single instance).
- CI/CD: GitHub Actions — `go test`, `go vet`, `sqlc generate` diff check, `eslint`, `tsc --noEmit`, frontend build on PR; deploy to Railway on merge to main.
- Configuration via env vars only: `DATABASE_URL`, `WHATSAPP_ACCESS_TOKEN`, `WHATSAPP_PHONE_NUMBER_ID`, `WHATSAPP_APP_SECRET`, `WHATSAPP_VERIFY_TOKEN`, `ANTHROPIC_API_KEY`, `SESSION_SECRET`.

**From Architecture — external lead items (start immediately, gates WhatsApp work):**

- Meta WhatsApp Business setup: business verification + dedicated WhatsApp Business number acquisition — the only external process on the critical path.
- Re-verify Meta pricing (service-window ≈ ₪0 conclusion) when the Business account is created, before the first pilot event.

**From Architecture — implementation-shaping requirements:**

- PostgreSQL is the source of truth for everything including live game state; **an answer is persisted before its "התקבל ✓" ack is sent**; server restart mid-game recovers full state from the DB.
- In-memory per-game hub caches live state for WebSocket fan-out; always rebuildable from Postgres; no Redis in the pilot.
- Data access: sqlc 1.31 + pgx v5 (SQL-first, no ORM); migrations via goose v3 (plain SQL in `server/migrations`, run at service boot).
- Schema integrity: `UNIQUE (question_id, participant_id)` on answers; server receipt `timestamptz` + monotonic sequence for tie-breaking; enums as TEXT + CHECK.
- **Spectator schema detail (architecture gap #2): `participants.role` column (`player`/`spectator`) must be included in the schema epics.**
- Webhook processing: verify HMAC, **dedupe by WhatsApp message ID** (Meta retries webhooks — idempotency mandatory).
- Outbound dispatch: worker pool with token-bucket rate limiter under 80 msg/sec; 3 retries with backoff; failures logged, never block the game loop.
- Realtime: single WebSocket endpoint `/ws?gameId=...&role=display|host`; server pushes **full game-state snapshots** (`{"type":"snapshot","seq":N,"state":...}`), push-only; organizer actions go over REST; clients drop stale `seq`.
- AI Semantic stage: Claude Opus 4.8 (`claude-opus-4-8`) via `anthropic-sdk-go`, strict structured output `{"correct": bool}`, ~5s timeout, fail-closed to two-stage grading.
- Hebrew fuzzy stage: normalization (final-letter forms, nikud stripping, punctuation, trim) + Levenshtein threshold — pure Go.
- Auth: argon2id password verify, server-side sessions in Postgres; Audience Display is a same-origin route inheriting the dashboard session cookie (no pairing flow).
- Centralized Hebrew copy: ALL outbound WhatsApp strings in `server/internal/wa/messages_he.go`; ALL UI strings in `web/src/lib/strings.he.ts` — no Hebrew literals in components or handlers.
- Game state enum (identical strings in Go/TS/DB): `draft → lobby → question_open → question_closed → revealed → leaderboard → finished` (leaderboard skippable).
- API: REST JSON under `/api/*`, camelCase wire format, error envelope `{"error":{"code","message"}}`; PRD Glossary terms are mandatory in all identifiers/tables/routes (never `host`/`player`/`quiz`/`session` as identifiers).
- Dependency direction (enforced): `httpapi`/`wa`/`ws` → `game` → `grading`/`store`; `store` is the only package touching Postgres; handlers thin.
- Frontend: single SPA, React 19 + React Router v8 + TanStack Query v5 (CRUD only); live surfaces driven by the WS snapshot via a `useSyncExternalStore` hook with exponential-backoff auto-reconnect; Display features render exclusively from the snapshot (no TanStack Query).
- Filter-safety enforcement: only `index.html` + `index.css` may reference assets; no file may reference an external URL asset (CI-greppable).
- Dev workflow: `make dev` (Go :8080 + Vite :5173 proxy); WhatsApp webhooks tunneled (e.g. cloudflared) during integration work.

### UX Design Requirements

*Sources: DESIGN.md (binding visual identity — now applied to Audience Display at projection scale) and EXPERIENCE.md (authoritative for Host Dashboard only). Participant mobile screens, portrait lock, keyboard handling, tap states, marketing/account surfaces, and the "הורד תוצאות" export CTA are superseded and excluded. The 2026-07-09 UX revision is complete — items below cite their resolved canonical sources (templates table, stage table, A11/A15/A16/A18/A19).*

UX-DR1: Implement DESIGN.md design tokens as Tailwind/shadcn theme overrides: dual palette — Festival Green (`green-900/800/600/100/50`, `surface-base #F0FDF4`, `surface-raised #FFFFFF`, `gold #FBBF24`) for the Audience Display and brand; neutral slate (`host-surface #F8FAFC`, `host-text #0F172A`, `host-border #E2E8F0`) for the Host Dashboard; separate semantic colors (`success #15803D` — re-pointed from #16A34A by the 2026-07-09 accessibility review (A18): the old value collided with green-600 and put white text at 3.3:1 at the Reveal; `error #DC2626`, `warning #F59E0B`) — DESIGN.md frontmatter is authoritative for all token values; spacing scale (4/8/12/16/24/32/48/64px); radius tokens (sm 8 / md 12 / lg 16 / xl 22 / pill).

UX-DR2: Gold appears in exactly two moments — countdown ≤5 seconds and the winner reveal — and nowhere else (enforced design rule).

UX-DR3: Typography: `system-ui` stack only (no webfont URLs); four semantic roles — Display 900 (timer numeral, winner headline), Heading 800 (question text, titles), Body 500, UI 600; letter-spacing 0 on Hebrew; line-height 1.38 heading / 1.55 body.

UX-DR4: Filter-safe decoration: abstract/geometric only; no photographs of people, no gender-depicting illustrations, no GIFs, no external assets; winner celebration is CSS-animated geometric shapes in gold + green-600.

UX-DR5: Timer components: `timer-hero` circular ring (white ring on dark green, numeral Display-weight, ring transitions to gold at ≤5s) for the Audience Display at projection scale (per A19: 220px ring, 96px numeral at 1080p; at ≤5s the ring turns gold **and thickens 8px→14px** — never a hue-only cue); `timer-inline` (60px, green-800 ring; error red at ≤5s — gold never on light surfaces, per UX revision 2026-07-09 A18) for the Host Dashboard. Countdown shows remaining seconds only (never total); ring animation respects `prefers-reduced-motion` (static ring, numeral still counts).

UX-DR6: Split-hero layout language — dark-green top panel (progress + timer) over white content panel (question + options), never inverted, never used on non-game screens — adapted from phone screens to Audience Display projection scale (per the stage table + A19; hero band 30–35% of stage height).

UX-DR7: Leaderboard components: `leaderboard-row` (rank / name / score with tabular-nums), top-10 depth (A11), movement indicators after each Reveal (rows ≥72px / 48px text at 1080p per A19; ▲ indicator ≥40px; static reordering under reduced motion). Flat elevation (no shadows) on game surfaces.

UX-DR8: Winner takeover: full-screen `winner-card` — green-800 background, winner name in gold Display weight, score muted, CSS geometric confetti (layout per DESIGN.md's winner-card spec; copy per the templates table; ties stack up to three names, score shown once — A16).

UX-DR9: Host Dashboard structure: RTL right-sidebar navigation (240px), content panels with 24px padding, two-level elevation (flat cards with borders; shadow on modals only), optimized 1280px+, minimum 1024px; buttons 40px height with extended hit area (48px effective touch targets).

UX-DR10: Host control panel is a one-primary-CTA state machine (per EXPERIENCE.md, minus export): Lobby → "התחל משחק"; Between questions → "פתח שאלה [N]" (+ "עצור"); Question open → "סגור שאלה"; Question closed → "גלה תשובה"; Revealed → "שאלה הבאה" / "עצור". The primary CTA is one persistent button whose label/action swap in place (focus retained); "עצור" always opens the confirm-stop dialog (renamed from "סיים משחק" per UX revision 2026-07-09). No auto-advance; a state can never be skipped accidentally; Game over shows on-screen results (no download CTA — export is out of scope).

UX-DR11: Host microcopy per EXPERIENCE.md table: "פתח שאלה", "סגור שאלה", "גלה תשובה", "שאלה הבאה ←", live stat "87 ענו" (`host-stat-pill`), connection error "החיבור נפסק — מתחבר מחדש...". All UI Hebrew copy centralized in `strings.he.ts`.

UX-DR12: Error-message principles (all surfaces): describe what happened and what the system is doing; never apologize; never vague ("שגיאה" alone forbidden); never blame the user; errors programmatically associated via `aria-describedby`. Live surfaces show "מתחבר..." only when the WS is down.

UX-DR13: Host keyboard shortcuts (confirmed by the UX revision 2026-07-09, closing the carried note): Space fires the primary CTA only when focus is on body/main — never inside inputs or dialogs; focused buttons use native activation (no global handler racing them); Escape only closes dialogs, never opens one; stopping the game is the visible "עצור" control, and destructive advances are always behind the confirm dialog, keyboard included.

UX-DR14: Accessibility floor: color is never the sole signal (✓/✗ icons alongside success/error fills, aria-labeled); visible focus rings on all dashboard interactive elements; `dir="rtl"` + `lang="he"` on the document; all text in rem, functional at 200% zoom (WCAG 1.4.4); `aria-live="polite"` on score/count updates, `aria-live="assertive"` on game-state transitions; WCAG AA contrast (white on green-800 = 7.1:1).

UX-DR15: WhatsApp conversation microcopy (resolved by the UX revision 2026-07-09, closing this `[OQ-7]` item): tone is **warm and playful (חם ומשחקי)** — short and front-loaded under time pressure, gender-neutral Hebrew (no "ברוך הבא"), symbolic-emoji discipline (🎉 🏆 ⚡ ✓ only). The canonical copy is EXPERIENCE.md's "WhatsApp message templates" table, which supersedes the examples previously quoted here. All outbound WhatsApp copy centralized in `messages_he.go`.

UX-DR16: Audience Display per-state content — lobby (JOIN instructions + climbing counter), question (question, options, timer, live answer count), reveal (correct answer + distribution), leaderboard (top ranks + movers), winner takeover — one component per state (content per EXPERIENCE.md's Audience Display stage table).

### FR Coverage Map

FR-1: Epic 2 - JOIN registration with welcome reply, idempotency, lobby-count update
FR-2: Epic 2 - Universal Reply for every inbound message class
FR-3: Epic 2 - Late join registers as Spectator (results-only)
FR-4: Epic 3 - Question delivery burst to all Participants on open
FR-5: Epic 3 - Answer intake with immediate "התקבל ✓" ack
FR-6: Epic 3 - Post-Reveal personal result messages + final results to all
FR-7: Epic 3 - Single server-side cutoff; late answers politely rejected
FR-8: Epic 3 - One recorded answer per Participant (first valid wins)
FR-9: Epic 4 - Projection view: 1s transitions, authoritative timer, auto-reconnect
FR-10: Epic 4 - Per-state stage content (lobby / question / reveal / leaderboard / winner)
FR-11: Epic 1 - Authenticated Game builder with Questions, Accepted Answers, scoring config, JOIN Code
FR-12: Epic 1 - Question Bank browse + Package import (copy semantics)
FR-13: Epic 3 - Lobby + live control state machine (explicit actions, Leaderboard skippable)
FR-14: Epic 3 - Post-game results summary on dashboard
FR-15: Epic 3 - MCQ auto-grading, published only at Reveal
FR-16: Epic 3 - Three-stage Free-Text validation (Exact → Fuzzy → AI), fail-closed, Reveal gated
FR-17: Epic 3 - Configurable scoring + Speed Bonuses by receipt timestamp
FR-18: Epic 4 - Live Leaderboard + winner takeover on the Audience Display (final WhatsApp winner message delivered by Epic 3's FR-6 story)

## Epic List

### Epic 1: Foundation & Game Authoring
An Organizer signs in to a deployed, working dashboard and builds a complete Game — MCQ and Free-Text Questions with Accepted Answers, per-question time limits, and scoring configuration — imports a Question Package from the Question Bank, and receives the Game's JOIN Code.
**FRs covered:** FR-11, FR-12
**Implementation notes:** Story 1.1 is project initialization using the architecture's starter commands (Go + chi + pgx / create-vite + shadcn init), Railway service + managed Postgres, CI pipeline. Database schema foundation, organizer auth (argon2id + server-side sessions), and design-token setup (UX-DR1–UX-DR3, UX-DR9) land in this epic. ⚠️ Meta WhatsApp Business verification + dedicated number acquisition (the only external lead-time item) must be kicked off in parallel with this epic — it gates Epic 2.

### Epic 2: WhatsApp Front Door — Registration & Inbox
Any person who messages the platform number gets a response (Universal Reply); a Participant sends `JOIN <code>` and is registered with a welcome reply; late joiners become Spectators; the Organizer watches the lobby fill live on the dashboard.
**FRs covered:** FR-1, FR-2, FR-3
**Implementation notes:** Includes the full WhatsApp infrastructure — webhook endpoint with HMAC verification and message-ID dedupe, outbound client with token-bucket rate limiting (≤80 msg/s) and retry, `messages_he.go` copy file, `participants.role` column (player/spectator), and the WebSocket foundation for the ≤3s lobby-count update. Note: FR-1's Audience-Display lobby-counter half lands at Story 4.2 — this epic delivers the dashboard half (relevant when demoing Epic 2 as "done").

### Epic 3: Live Gameplay — A Complete Game Runs in WhatsApp
The Organizer runs an entire Game from the live control panel (exactly one primary CTA per state, no accidental skips); Participants receive each Question as a WhatsApp message, answer by replying, get an immediate ack, and after Reveal receive their grade, points, and rank; Free-Text answers grade through Exact → Fuzzy → AI Semantic; Speed Bonuses reward the fastest correct answers; everyone (including Spectators) gets the final results message; the Organizer sees the post-game summary.
**FRs covered:** FR-4, FR-5, FR-6, FR-7, FR-8, FR-13, FR-14, FR-15, FR-16, FR-17
**Implementation notes:** The largest epic (~10 stories), cohesive around the game engine (`engine.go`, `answers.go`, `scoring.go`, `dispatch.go`). Grading stages build as ordered stories: exact → fuzzy (Hebrew normalization + Levenshtein) → AI (Claude Opus 4.8, fail-closed — the risk boundary). Message copy is canonical per EXPERIENCE.md's templates table and isolated to `messages_he.go`. Standalone: a full game is playable end-to-end via WhatsApp without the projected screen.

### Epic 4: The Room's Stage — Audience Display
The Organizer opens the Audience Display onto a projector; the room sees the lobby with a climbing counter, the Question with a big countdown timer (gold at ≤5s), the Reveal with answer distribution, the Leaderboard with movement indicators, and the festive winner takeover — all transitioning within 1 second with silent auto-reconnect.
**FRs covered:** FR-9, FR-10, FR-18
**Implementation notes:** Built entirely on Epic 3's WS snapshots; one component per display state (per EXPERIENCE.md's stage table); DESIGN.md tokens applied at projection scale (UX-DR5–UX-DR8, UX-DR16). The final WhatsApp winner message (FR-18's messaging half) is already delivered by Epic 3's FR-6 results story.

**Dependency flow:** Epic 1 → Epic 2 → Epic 3 → Epic 4; no epic requires a later epic to function.

## Epic 1: Foundation & Game Authoring

An Organizer signs in to a deployed, working dashboard and builds a complete Game — MCQ and Free-Text Questions with Accepted Answers, per-question time limits, and scoring configuration — imports a Question Package from the Question Bank, and receives the Game's JOIN Code.

### Story 1.1: Project Scaffold, CI, and Deployed Walking Skeleton

As the platform operator,
I want the monorepo initialized per the architecture's starter commands and deployed end-to-end,
So that every subsequent story builds and ships on working rails.

**Acceptance Criteria:**

**Given** a fresh clone,
**When** the architecture's initialization commands are applied (Go 1.26 module + chi v5 + coder/websocket + pgx v5; `create-vite` react-ts + `shadcn init`),
**Then** the repo matches the architecture's directory structure (`/server` with `cmd/`+`internal/`, `/web`), and `make dev` runs the Go server (:8080) and Vite dev server (:5173) with `/api`/`/ws` proxied.

**Given** a production build,
**When** `go build` runs,
**Then** the binary embeds `web/dist`, serves the SPA at `/`, and `GET /api/health` returns 200 JSON.

**Given** a pull request,
**When** CI runs,
**Then** `go test`, `go vet`, `sqlc generate` diff-check, `eslint`, `tsc --noEmit`, and the frontend build all gate the merge,
**And** merge to main deploys to Railway (EU, replicas pinned to 1) with managed Postgres, goose migrations running at boot.

**Given** the deployed SPA,
**Then** `index.html` carries `lang="he" dir="rtl"`, DESIGN.md tokens are configured (Festival Green + host-slate palettes, spacing/radius scale, system-ui stack — UX-DR1–UX-DR3),
**And** a CI grep check verifies no file outside `index.html`/`index.css` references an external URL asset (UX-DR4, filter-safety).

**Given** a required env var is missing,
**When** the server starts,
**Then** it fails fast with a clear message; `.env.example` documents all variables.

*Note — custom domain: the operator owns `anash-list.com` and may serve the Organizer surfaces from a subdomain (e.g., `clickers.anash-list.com`) via a Railway custom domain + CNAME. A path prefix (`anash-list.com/clickers`) is discouraged (SPA base-path, session-cookie, and WebSocket-URL complications). Railway's default domain suffices for the pilot; decision open.*

### Story 1.2: Organizer Sign-In

As an Organizer,
I want to sign in to my dashboard with provisioned credentials,
So that my Games are accessible only to me.

**Acceptance Criteria:**

**Given** a manually provisioned account (documented provisioning path storing an argon2id hash; `organizers` + `sessions` tables created in this story),
**When** I submit valid credentials on the Hebrew RTL login page,
**Then** a server-side session is created in Postgres, an HTTP-only Secure cookie is set, and I land on my games list.

**Given** invalid credentials,
**Then** the error copy (from `strings.he.ts`) describes what happened in Hebrew — never vague, never blaming (UX-DR12).

**Given** no valid session,
**When** any `/api/*` route is called,
**Then** the response is 401 with the `{"error":{"code","message"}}` envelope, and the SPA redirects to login.

**When** I log out,
**Then** the session row is deleted server-side and the cookie cleared.

**Given** a server restart,
**Then** existing sessions survive (Postgres-backed).

### Story 1.3: Create a Game and Author Questions

As an Organizer,
I want to create a Game and add, edit, reorder, and delete MCQ and Free-Text Questions,
So that I can prepare my quiz in advance.

**Acceptance Criteria:**

**Given** I am signed in,
**When** I create a Game,
**Then** it appears in my games list with a unique JOIN Code displayed (`games` + `questions` tables created in this story).

**Given** a Game in `draft`,
**When** I add an MCQ Question,
**Then** it has exactly four options (א–ד) with exactly one marked correct, and a per-question time limit pre-filled with the default (default 20s per UX revision 2026-07-09 A8; the field carries guidance that 30–45s accommodates screen-reader and slower-motor Participants).

**When** I add a Free-Text Question,
**Then** I can define one or more Accepted Answers and edit them later.

**When** I edit, reorder, or delete Questions,
**Then** the changes and order persist.

**Given** a Game owned by another Organizer,
**When** I request it by ID,
**Then** I receive `GAME_NOT_FOUND` (ownership scoping, FR-11),
**And** the dashboard follows UX-DR9 (right sidebar 240px, slate palette, 1024px minimum).

### Story 1.4: Configure Scoring Rules

As an Organizer,
I want to set points per correct answer and Speed Bonus values per Game,
So that scoring matches my event.

**Acceptance Criteria:**

**Given** a new Game,
**Then** scoring defaults are pre-filled: 100 points per correct answer; Speed Bonuses 50/30/20 for 1st/2nd/3rd fastest correct `[ASSUMPTION — defaults not fixed in PRD]`.

**When** I edit the values,
**Then** they persist and validate as non-negative integers (zero disables a bonus),
**And** the configuration is stored on the Game for the engine to consume (FR-17 configuration half).

### Story 1.5: Question Bank Package Import

As an Organizer,
I want to browse the Question Bank and import a Question Package into my Game,
So that I can run a ready-made quiz and mix it with my own questions.

**Acceptance Criteria:**

**Given** the Question Bank contains at least one Package (`question_packages` tables created and a sample pack seeded in this story; pilot content sourcing remains OQ-4),
**When** I browse the bank,
**Then** I see each Package's name and question count.

**When** I import a Package into my Game,
**Then** all its Questions are **copied** and appended in order to my Game (FR-12).

**Given** an imported Question,
**When** I edit or delete it in my Game,
**Then** the Question Bank Package is unchanged (copy semantics),
**And** imported and custom Questions can be mixed and reordered freely.

**Given** the bank browse view,
**Then** each Package card shows title, question count, and a question preview (A13), imported copies carry a "מהמאגר" badge, a Game with no Questions prompts "הוסיפו שאלה ראשונה — או ייבאו חבילה מהמאגר" with both CTAs, and an empty Question Bank states that packages are on the way rather than showing a bare list (EXPERIENCE.md empty states).

## Epic 2: WhatsApp Front Door — Registration & Inbox

Any person who messages the platform number gets a response (Universal Reply); a Participant sends `JOIN <code>` and is registered with a welcome reply; late joiners become Spectators; the Organizer watches the lobby fill live on the dashboard.

### Story 2.1: WhatsApp Webhook Intake and Outbound Dispatch

As the platform operator,
I want verified, idempotent WhatsApp message intake and rate-limited outbound sending,
So that the platform can converse reliably with Participants without losing or duplicating messages.

**Acceptance Criteria:**

**Given** Meta's webhook registration handshake,
**When** `GET /webhooks/whatsapp` arrives with the verify token,
**Then** the challenge is answered correctly (endpoint lives outside `/api`, no session auth).

**Given** an inbound `POST /webhooks/whatsapp`,
**When** the `X-Hub-Signature-256` HMAC is invalid,
**Then** the request is rejected and logged; valid signatures are processed.

**Given** Meta retries a webhook delivery,
**When** the same WhatsApp message ID arrives twice,
**Then** it is processed exactly once (dedupe by message ID).

**Given** outbound messages are queued,
**When** the dispatcher sends them,
**Then** a worker pool with a token-bucket limiter stays under 80 msg/sec, failed sends retry up to 3 times with backoff, and final failures are logged WARN without blocking anything.

**Given** any log line involving a Participant,
**Then** the phone number appears as last-4 only (`phone_last4`), per NFR-4,
**And** the local dev workflow (tunnel via cloudflared, Meta test number) is documented in the README.

*Note: requires the Meta WhatsApp Business setup kicked off in Epic 1 — a Meta test number suffices for development.*

### Story 2.2: Universal Reply — the Chat Never Goes Silent

As a person messaging the platform number,
I want a helpful Hebrew reply to anything I send,
So that I always know what to do next (FR-2).

**Acceptance Criteria:**

**Given** an inbound message that is not a recognized command,
**When** it is processed,
**Then** a short Hebrew help reply is sent (how to join, where to get a code) — per the templates table Help row.

**Given** an inbound message that is media, a sticker, or empty,
**Then** the same generic help reply is sent — no inbound message class results in silence.

**Given** the inbound parser,
**Then** every parse branch terminates in a reply (enforced in `wa/inbound.go` structure),
**And** all outbound copy lives in `messages_he.go` per the EXPERIENCE.md message-templates table: warm-playful, short, first-three-words-first, gender-neutral (UX-DR15, revised 2026-07-09).

### Story 2.3: Open the Lobby and Watch It Live

As an Organizer,
I want to open my Game's lobby and see the participant count and list live on the dashboard,
So that I know when the room is ready.

**Acceptance Criteria:**

**Given** my Game is in `draft`,
**When** I press "פתח לובי",
**Then** the Game transitions to `lobby` (the game-state enum and engine transition seed created here; the full control panel arrives in Epic 3),
**And** the `participants` table is created (including the `role` column, player/spectator).

**Given** the lobby page is open,
**When** it connects to `/ws?gameId=...&role=host` (session-authenticated),
**Then** it receives a full snapshot on connect (`{"type":"snapshot","seq":N,"state":...}`) and renders the current participant count and list (initially empty).

**Given** the connection drops,
**When** the socket hook reconnects (exponential backoff),
**Then** the first snapshot re-renders current state with no manual refresh, and stale `seq` messages are dropped.

**Given** the JOIN Code and platform number,
**Then** the lobby page displays them for the Organizer to relay to the room.

### Story 2.4: Join the Game with One WhatsApp Message

As a Participant,
I want to send `JOIN <code>` and get a welcome reply,
So that I am registered for the Game in seconds (FR-1).

**Acceptance Criteria:**

**Given** a Game in `lobby`,
**When** I send `JOIN <code>` with its valid code,
**Then** I am registered as a Participant (identified by phone number, display name from my WhatsApp profile name — per EXPERIENCE.md A3, resolving OQ-3: profile name by default, correctable via `שם:`), and receive the Hebrew welcome reply with my name including the name-correction hint, per the canonical templates table,
**And** the lobby count and list on the dashboard update within 3 seconds.

**Given** an invalid or expired code,
**When** I send `JOIN <wrongcode>`,
**Then** I receive a Hebrew reply explaining the code was not found.

**Given** I am already registered,
**When** I re-send `JOIN` with the same code,
**Then** I am not duplicated and receive the same welcome reply (idempotent — also guaranteed under webhook retry via Story 2.1 dedupe).

**Given** the persistence layer,
**Then** a unique constraint on (game, phone) enforces no-duplicates at the DB level.

**Given** a Game whose lobby has not yet opened (`draft`),
**When** I send `JOIN <code>` with its valid code,
**Then** I receive the pre-lobby reply per the EXPERIENCE.md templates table ("הקוד נכון! ההרשמה עוד לא נפתחה — שלחו שוב את ההודעה כשהמארגן מכריז שמתחילים.") and am not yet registered (A17; UJ-1's "הדודה ששלחה JOIN יום קודם" edge case).

**Given** I am a registered Participant (any role, any game state),
**When** I send `שם: <השם>`,
**Then** my display name is updated and I receive the confirmation "עודכן ✓ מעכשיו: [שם]" (templates table; the `שם:` parse branch joins `wa/inbound.go`'s parse taxonomy),
**And** an unregistered sender's `שם:` message receives the Help reply (conversation grammar).

### Story 2.5: Late Join Becomes Spectator

As a person joining after the Game started,
I want to be told the Game already began and still get the final results,
So that I am included without disrupting play (FR-3).

**Acceptance Criteria:**

**Given** a Game that has started (any state past `lobby`),
**When** I send `JOIN <code>` with its valid code,
**Then** I am registered with `role = spectator` and receive a Hebrew reply that the Game already started and results will arrive at game end.

**Given** a registered Spectator,
**Then** they are excluded from question dispatch lists (no mid-game content), and included in the final-results recipient list consumed by Epic 3's results story.

**Given** a Spectator re-sends JOIN,
**Then** registration is idempotent (same reply, no duplicate).

## Epic 3: Live Gameplay — A Complete Game Runs in WhatsApp

The Organizer runs an entire Game from the live control panel (exactly one primary CTA per state, no accidental skips); Participants receive each Question as a WhatsApp message, answer by replying, get an immediate ack, and after Reveal receive their grade, points, and rank; Free-Text answers grade through Exact → Fuzzy → AI Semantic; Speed Bonuses reward the fastest correct answers; everyone (including Spectators) gets the final results message; the Organizer sees the post-game summary.

### Story 3.1: Run the Game — Live Control State Machine

As an Organizer,
I want to start my Game and drive it state-by-state from the control panel,
So that pacing is always my explicit decision (FR-13).

**Acceptance Criteria:**

**Given** a Game in `lobby` with at least one Question,
**When** I press "התחל משחק",
**Then** the engine transitions through the canonical state machine (`lobby → question_open → question_closed → revealed → [leaderboard] → … → finished`), each advance an explicit REST action, persisted before the snapshot broadcast — no auto-advance anywhere.

**Given** any game state,
**Then** the control panel shows exactly one primary CTA per UX-DR10 ("התחל משחק" / "פתח שאלה [N]" / "סגור שאלה" / "גלה תשובה" / "שאלה הבאה"), with "עצור" as the secondary stop control (renamed from "סיים משחק" per the UX revision 2026-07-09) always behind the confirm-stop dialog, so no state can be skipped accidentally,
**And** from `revealed` I can skip the Leaderboard and open the next Question directly (UJ-4).

**Given** a Question opens,
**Then** the engine records the single server-side cutoff — the earlier of open-time + time limit, or my explicit "סגור שאלה" (FR-7 rule; enforced on intake in Story 3.3).

**Given** the keyboard,
**Then** Space fires the primary CTA only when focus is on `body` or the main region — never inside inputs or dialogs — focused buttons use native activation (no global handler racing them), and Escape has exactly one meaning: close the open dialog, never open one (UX-DR13); stopping the game is the visible "עצור" control, whose confirm dialog guards destructive advances on every path, keyboard included, with all copy from `strings.he.ts` (UX-DR11).

**Given** a server restart mid-Game,
**When** the control panel reconnects,
**Then** the full state is recovered from Postgres with no corruption (NFR-2).

### Story 3.2: Question Delivery to Every Phone

As a Participant,
I want each Question to arrive as a WhatsApp message the moment it opens,
So that I can play from any device with WhatsApp (FR-4).

**Acceptance Criteria:**

**Given** the `QuestionOpened` event,
**When** dispatch runs,
**Then** every `player`-role Participant receives the question text — MCQ with lettered options א–ד — including how to answer and the time limit — copy per the templates table Question rows (MCQ / Free-Text); Spectators receive nothing.

**Given** 80 registered Participants,
**When** a Question opens,
**Then** dispatch begins immediately and the burst completes within the 5s/95% target (token-bucket math per architecture: ~1–2s at 80 mps).

**Given** an individual send failure,
**Then** it retries up to 3 times with backoff, final failure logs WARN, and the game loop is never blocked (NFR-2).

### Story 3.3: Answer Intake with Immediate Acknowledgment

As a Participant,
I want my WhatsApp reply recorded instantly with a "התקבל ✓",
So that I know my answer counts (FR-5, FR-7, FR-8).

**Acceptance Criteria:**

**Given** an open Question (`answers` table created in this story: `UNIQUE (question_id, participant_id)`, server-receipt `timestamptz` + monotonic sequence),
**When** I reply with a valid answer (MCQ: letter א–ד or digit 1–4; Free-Text: ≤200 chars),
**Then** the answer row is **persisted before** the "התקבל ✓" ack is sent (SM-4), stamped with server receipt time.

**Given** a Free-Text reply over 200 characters,
**Then** it is rejected with a Hebrew hint; **and** an unparseable reply during an open MCQ returns a short format hint, leaving me able to answer.

**Given** I already answered this Question,
**When** I send a second answer,
**Then** it is not recorded and I am told my first answer counts — selection is final (FR-8).

**Given** a reply whose receipt timestamp is after the cutoff,
**Then** it is not counted and receives the polite "השאלה נסגרה" reply (FR-7, UJ-5).

**Given** the control panel during an open Question,
**Then** the live answered count and percentage update via snapshots (FR-13 consequence, `host-stat-pill` UX-DR11).

### Story 3.4: Grading Pipeline — MCQ and Exact Match

As an Organizer,
I want answers graded automatically as they arrive, without leaking grades early,
So that Reveal is instant and fair (FR-15, FR-16 exact stage).

**Acceptance Criteria:**

**Given** an MCQ answer,
**When** it is recorded,
**Then** it is graded mechanically against the correct option; the grade is stored and never disclosed before Reveal (FR-15).

**Given** a Free-Text answer,
**When** the pipeline runs,
**Then** the Exact stage compares against all Accepted Answers and a match records `stage = exact` (FR-16); an exact match never invokes later stages.

**Given** received answers with grading in progress,
**Then** the engine tracks outstanding grades per Question, and the "גלה תשובה" control activates only once every received answer is graded (structure that Story 3.6 stresses with async AI).

### Story 3.5: Hebrew Fuzzy Matching

As a Participant,
I want my answer accepted despite a typo or spelling variant,
So that spelling doesn't cost me points (FR-16 fuzzy stage).

**Acceptance Criteria:**

**Given** a Free-Text answer that misses Exact,
**When** the Fuzzy stage runs,
**Then** it normalizes both sides (trim, final-letter forms ך/ם/ן/ף/ץ → כ/מ/נ/פ/צ, nikud stripping, punctuation) and matches within a Levenshtein threshold, recording `stage = fuzzy`.

**Given** the UJ-3 cases,
**Then** "צרורה " (trailing space) matches Accepted Answer "צרורה"; a one-letter typo within threshold matches; an unrelated word does not.

**Given** the implementation,
**Then** it is pure Go (`normalize.go` + `fuzzy.go`) with unit tests covering the Hebrew normalization cases.

### Story 3.6: AI Semantic Validation, Fail-Closed

As a Participant,
I want an equivalent wording of the right answer to count,
So that I'm graded on knowledge, not phrasing (FR-16 AI stage).

**Acceptance Criteria:**

**Given** a Free-Text answer that misses both Exact and Fuzzy,
**When** the AI stage runs (Claude Opus 4.8 via `anthropic-sdk-go`),
**Then** the prompt carries the Question, the Accepted Answers, and the answer; the model returns strict structured output `{"correct": bool}` judging equivalence only — it never invents correctness; a match records `stage = ai`.

**Given** an AI timeout (~5s) or error,
**Then** the answer is graded by the first two stages only (fail-closed), and the degradation is logged WARN with the stage recorded (NFR-8, SM-C1 auditability).

**Given** last-second answers still in the AI stage at close,
**When** grading completes,
**Then** the Reveal control activates — grading runs as answers arrive, so the residual wait covers only those stragglers (FR-16).

### Story 3.7: Scoring with Speed Bonuses

As a Participant,
I want correct and fast answers rewarded automatically,
So that the competition is real (FR-17).

**Acceptance Criteria:**

**Given** a revealed Question,
**When** scoring runs,
**Then** each correct answer earns the Game's configured points, and Speed Bonuses go to at most the first, second, and third correct answers ordered by server receipt timestamp — ties broken by the monotonic sequence; fewer correct answers → fewer bonuses.

**Given** cumulative scores,
**Then** the Leaderboard ranks Participants with equal scores sharing a rank,
**And** the leaderboard data is included in the game snapshot (consumed by result messages now, the Audience Display in Epic 4).

**Given** `scoring.go`,
**Then** unit tests cover bonus ordering, timestamp ties, and fewer-than-three-correct cases.

### Story 3.8: Personal Results After Reveal

As a Participant,
I want my grade, points, and rank in my WhatsApp right after the Reveal,
So that I feel the game beat-by-beat (FR-6).

**Acceptance Criteria:**

**Given** the `AnswerRevealed` event,
**When** result messages dispatch,
**Then** each Participant who answered receives their grade, points earned including any Speed Bonus, and current rank — copy per the templates table result rows: "נכון! 🎉 +[ניקוד] נקודות / ⚡ בונוס מהירות +[בונוס] / מקום [דירוג] בטבלה" (two emoji allowed on the bonus message, A20), wrong-answer variants included — the last-question form drops "עוד הכול פתוח" (A5) (UJ-2).

**Given** a Question not yet revealed,
**Then** no grade is ever sent (FR-6: no leakage into the room).

**Given** a Participant who did not answer,
**Then** they receive no per-question message (silence by design).

### Story 3.9: Final Results for Everyone

As a Participant or Spectator,
I want the final results in my WhatsApp when the Game ends,
So that the event closes with my score and the winner's name (FR-6 game-end, FR-18 messaging half).

**Acceptance Criteria:**

**Given** the `GameFinished` event,
**When** final messages dispatch,
**Then** every Participant **and** every Spectator receives their score, rank, and the winner's name — copy per the EXPERIENCE.md templates table ("המשחק נגמר! 🏆 הזוכה: [שם] עם [ניקוד] נקודות...").

**Given** a tie for first place,
**Then** all tied names are included as winners — "הזוכים: [שם] ו-[שם] עם [ניקוד] נקודות" (resolved by EXPERIENCE.md A16).

**Given** the winner(s),
**Then** they additionally receive the personal winner variant "מזל טוב, [שם]! 🏆 ניצחת עם [ניקוד] נקודות!" — ties addressed jointly ("מזל טוב, [שם] ו-[שם]! 🏆 ניצחתם עם [ניקוד] נקודות!") per A16 (UJ-2's emotional climax).

### Story 3.10: Post-Game Summary on the Dashboard

As an Organizer,
I want a results summary when the Game ends,
So that I can review how the event went (FR-14).

**Acceptance Criteria:**

**Given** a finished Game,
**When** I view its results page,
**Then** I see the final Leaderboard and per-question response rates.

**Given** the pilot scope,
**Then** there is no export/download control (deferred to the commercial phase; UX-DR10 note),
**And** the results persist — accessible after navigating away or signing in again.

## Epic 4: The Room's Stage — Audience Display

The Organizer opens the Audience Display onto a projector; the room sees the lobby with a climbing counter, the Question with a big countdown timer (gold at ≤5s), the Reveal with answer distribution, the Leaderboard with movement indicators, and the festive winner takeover — all transitioning within 1 second with silent auto-reconnect.

### Story 4.1: Audience Display Shell — the Screen That Follows the Game

As an Organizer,
I want to open the Audience Display in a separate window that follows the Game live,
So that the projector always shows the current state with zero interaction (FR-9).

**Acceptance Criteria:**

**Given** an authenticated dashboard session,
**When** I launch the Audience Display,
**Then** a separate browser window opens at `/display/:gameId` — same-origin, inheriting my session cookie, no pairing flow — full-screen, output-only, `dir="rtl"`.

**Given** the display connects to `/ws?gameId=...&role=display`,
**Then** it renders the current state entirely from the first snapshot (no REST queries — snapshot is the only data source), and a stage-switcher renders one component per game state (UX-DR16).

**Given** an Organizer action,
**Then** the display transitions to the new state within 1 second (NFR-1).

**Given** a dropped connection,
**Then** it auto-reconnects with exponential backoff and re-renders current state with no Organizer intervention; "מתחבר..." appears only while the socket is down (UX-DR12),
**And** the surface uses the Festival Green palette and system-ui stack with zero external asset requests (UX-DR1, UX-DR3, UX-DR4).

**Given** the dashboard's "הפחת אנימציות" toggle (EXPERIENCE.md Display controls),
**When** the Organizer enables it,
**Then** the Audience Display renders the static motion equivalents for the whole room — the setting rides the game snapshot (display-settings field), requiring no display interaction, because the audience cannot set `prefers-reduced-motion` on a projector.

### Story 4.2: Lobby Stage — the Room Fills the Screen

As the room,
we want the projector to show how to join and the counter climbing,
So that joining becomes part of the show (FR-10 lobby, UJ-1).

**Acceptance Criteria:**

**Given** the Game is in `lobby`,
**Then** the display shows the JOIN instructions ("שלחו JOIN <code> למספר <number>") and a live participant counter, per the stage-lobby spec.

**Given** a Participant joins,
**Then** the counter increments within 3 seconds (FR-1 consequence),
**And** typography lands at projection scale per A19: JOIN Code + phone number at Display 900 (120px+, digit-grouped, bidi-isolated), instruction at Heading 800 (64px), counter in the oversized pill.

### Story 4.3: Question Stage with the Authoritative Timer

As the room,
we want the question, its options, a big countdown, and the live answer count on screen,
So that all eyes share the same drama (FR-9 timer, FR-10 question, UJ-5).

**Acceptance Criteria:**

**Given** a Question opens,
**Then** the display shows the question text, MCQ options with letters א–ד (or a free-text answer hint), and the live answered count ("63 ענו") from snapshots, in the split-hero layout language adapted to projection scale (UX-DR6; question 64px, options 40px per A19; Free-Text shows "כתבו את התשובה בוואטסאפ" in place of option rows, A15).

**Given** the server-sent absolute deadline,
**When** the timer renders,
**Then** the countdown is computed client-side from the deadline (server cutoff remains authoritative, FR-7), showing remaining seconds only — never the total.

**Given** the countdown reaches ≤5 seconds,
**Then** the ring transitions to gold **and thickens 8px→14px** (UX-DR2, UX-DR5 — never a hue-only cue), the numeral treatment unchanged,
**And** with `prefers-reduced-motion: reduce`, the ring is static while the numeral still counts (UX-DR14).

### Story 4.4: Reveal Stage — the Answer, Marked

As the room,
we want the correct answer marked and the answer distribution shown,
So that the reveal lands as one shared moment (FR-10 reveal).

**Acceptance Criteria:**

**Given** the Organizer reveals,
**Then** the display marks the correct option with the success treatment **and** a ✓ icon — color is never the sole signal (UX-DR14) — and shows the answer distribution across options per the distribution-bar spec: labels outside the fills, ✓ on the correct bar's label; wrong options demote to stage-option-dimmed.

**Given** a Free-Text Question,
**Then** the reveal shows the stage-answer-card with the Accepted Answer's primary form and the counts line "X ענו · Y צדקו" — no distribution bars (A15).

**Given** the shared screen,
**Then** no per-Participant private information appears (grades live only in each Participant's WhatsApp — FR-10 out-of-scope rule).

### Story 4.5: Leaderboard Stage — the Room Reshuffles

As the room,
we want the leaderboard between questions with the movers highlighted,
So that the competition stays alive all game (FR-18, FR-10).

**Acceptance Criteria:**

**Given** the Organizer advances to `leaderboard`,
**Then** the display shows the top 10 (or fewer) as rank / name / score rows (tabular-nums, `leaderboard-row` spec — UX-DR7) with shared ranks rendered correctly,
**And** movement indicators highlight climbers since the previous Leaderboard — ▲ + places climbed beside the rank, indicator ≥40px (A11), static reordering under reduced motion.

**Given** the Organizer skips the Leaderboard (UJ-4),
**Then** the display transitions directly from reveal to the next question — the stage renders only when entered.

### Story 4.6: Winner Takeover — מזל טוב!

As the room,
we want a full-screen festive winner moment,
So that the game ends on its emotional peak (FR-18, UJ-2).

**Acceptance Criteria:**

**Given** the Game finishes,
**Then** the display takes over full-screen with the `winner-card`: green-800 background, winner name in gold Display weight, score below ("מזל טוב, משה כהן! 1,240 נקודות" — per the winner-card spec and templates table) — gold's second and final permitted appearance (UX-DR2, UX-DR8).

**Given** the celebration,
**Then** it is CSS-animated geometric shapes in gold and green-600 only — no images, no GIFs, no external assets (UX-DR4), honoring `prefers-reduced-motion`.

**Given** a tie for first,
**Then** all tied winners are named — up to three names stacked, score shown once (A16, consistent with Story 3.9),
**And** game-state transitions announce via `aria-live="assertive"` (UX-DR14).
