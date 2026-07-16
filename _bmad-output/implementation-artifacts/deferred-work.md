# Deferred Work

Items deferred from reviews and workflows. Revisit when the referenced trigger arrives.

## Deferred from: code review of 1-1-project-scaffold-ci-and-deployed-walking-skeleton (2026-07-13)

- ~~Spacing-scale enforcement ("no values between stops", DESIGN.md)~~ **closed in 1.3 (2026-07-15): disposition "convention + review".** Mechanical enforcement via `--spacing: initial` stays rejected — it breaks shadcn component defaults (e.g. button `h-9`→36px), and the shadcn inheritance contract (DESIGN.md) says component internals keep their defaults. The binding convention, applied throughout 1.3's builder UI: hand-written classes use only on-scale spacing stops (4/8/12/16/24/32/48/64px ↔ Tailwind 1/2/3/4/6/8/12/16) for padding/margin/gap; buttons are always `h-10` (40px), never shadcn's default `h-9`. Enforcement is code-review attention, not tooling.
- Fail-fast config accepts placeholder secrets — `require()` in `server/internal/config/config.go` validates presence only, so dummy WhatsApp/Anthropic values pass in production (the runbook currently instructs exactly that for unconsumed vars). Add strength/placeholder validation when the vars become live: ~~`SESSION_SECRET` at Story 1.2 (session signing)~~ **done in 1.2** (≥32 chars + `change-me*` rejected, boot fails fast); WhatsApp secrets remain for Story 2.1 (webhook signature verification).

## Deferred from: code review of 1-3-create-a-game-and-author-questions (2026-07-15)

- Concurrent `CreateQuestion` can assign duplicate `position` values — `CreateQuestion` computes `position = COALESCE(MAX(position),0)+1` inside a single `INSERT ... SELECT`, but under READ COMMITTED two concurrent creates for the same game can both read the same MAX before either commits, and the migration deliberately omits `UNIQUE (game_id, position)` (the story chose transactional reorder over fighting a deferred constraint). Result: two questions briefly share a position, order falling to the `created_at` tiebreaker. Self-healing — the next `ReorderQuestions` rewrites positions to a clean 1..N. Low severity for a single-organizer builder. Revisit trigger: if multi-tab / concurrent authoring or an ordering-sensitive consumer of `position` appears, add a serialization guard (advisory lock or `SELECT ... FOR UPDATE` on the game) or a `UNIQUE (game_id, position) DEFERRABLE` constraint with retry.
