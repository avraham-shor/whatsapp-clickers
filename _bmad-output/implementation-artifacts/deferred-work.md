# Deferred Work

Items deferred from reviews and workflows. Revisit when the referenced trigger arrives.

## Deferred from: code review of 1-1-project-scaffold-ci-and-deployed-walking-skeleton (2026-07-13)

- Spacing-scale enforcement ("no values between stops", DESIGN.md) — `web/src/index.css` declares the 4/8/12/16/24/32/48/64px stops but Tailwind v4's default `--spacing: 0.25rem` multiplier still generates off-scale utilities. Enforcing via `--spacing: initial` breaks shadcn component defaults (e.g. button `h-9`→36px). Owner decision 2026-07-13: defer the enforcement call to the first real UI story (1.3), where there is actual UI to validate against.
- Fail-fast config accepts placeholder secrets — `require()` in `server/internal/config/config.go` validates presence only, so `SESSION_SECRET=change-me-long-random-string` and dummy WhatsApp/Anthropic values pass in production (the runbook currently instructs exactly that for unconsumed vars). Add strength/placeholder validation when the vars become live: `SESSION_SECRET` at Story 1.2 (session signing), WhatsApp secrets at Story 2.1 (webhook signature verification).
