-- Dedupe ledger for inbound WhatsApp webhook deliveries. One atomic
-- statement -- no read-then-write race under concurrent deliveries of the
-- same message ID.

-- name: MarkWaMessageProcessed :execrows
INSERT INTO wa_inbound_messages (wa_message_id) VALUES ($1) ON CONFLICT DO NOTHING;
