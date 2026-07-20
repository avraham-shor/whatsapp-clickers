-- +goose Up
-- Dedupe ledger for inbound WhatsApp webhook deliveries (Story 2.1, AC-3).
-- Meta retries non-200/timed-out deliveries for days, so an in-memory set
-- would lose the ledger on every redeploy mid-retry-window; one tiny table
-- with INSERT ... ON CONFLICT DO NOTHING is atomic and race-free under
-- concurrent webhook deliveries. wa_message_id is Meta's opaque "wamid...."
-- string, not a UUID, so it is the TEXT primary key and the natural dedupe
-- key -- no surrogate id needed. No retention job: pilot volume is
-- thousands of rows per game; unbounded growth is the accepted posture.
CREATE TABLE wa_inbound_messages (
    wa_message_id TEXT PRIMARY KEY,
    received_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE wa_inbound_messages;
