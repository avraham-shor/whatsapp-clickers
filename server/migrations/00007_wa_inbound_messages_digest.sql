-- +goose Up
-- Story 2.1 code review (2026-08-02): 00006's premise was wrong.
--
-- That migration stored Meta's wamid raw, on the recorded rationale that a
-- wamid is "Meta's opaque 'wamid....' string". It is not opaque: its leading
-- segment base64-encodes the sender's MSISDN (established by patch P7 on
-- 2026-07-30, which fixed the log representation but deliberately left the
-- ledger holding the raw value). Combined with 00006's "no retention job,
-- unbounded growth is the accepted posture", every inbound message was writing
-- a recoverable Participant phone number into permanent storage, present in
-- every database backup, decided on a premise that had already been disproved.
--
-- The column now stores wa.WaMessageIDDigest(wamid) -- a stable, unsalted
-- SHA-256 prefix. Dedupe semantics are untouched: nothing ever reads this
-- column back, it is only ever compared for equality via
-- INSERT ... ON CONFLICT DO NOTHING, so the digest is a drop-in key. The name
-- wa_message_id is kept so the sqlc query and its generated code stay
-- byte-identical (the CI diff gate covers gen/); COMMENT ON COLUMN records the
-- real semantics.
--
-- Existing rows are purged rather than converted: the digest cannot be
-- computed in SQL, and the only rows present are Task 7 E2E leftovers plus the
-- 2026-07-30 live delivery. Losing those dedupe keys risks re-processing a
-- message only if Meta is still retrying one from before this deploy -- which
-- for the pilot means, at worst, one duplicate stub log line.
DELETE FROM wa_inbound_messages;

COMMENT ON COLUMN wa_inbound_messages.wa_message_id IS
    'SHA-256 digest of Meta''s wamid (wa.WaMessageIDDigest), never the raw wamid: the raw value base64-encodes the sender MSISDN. Equality-only dedupe key.';

-- +goose Down
COMMENT ON COLUMN wa_inbound_messages.wa_message_id IS NULL;
