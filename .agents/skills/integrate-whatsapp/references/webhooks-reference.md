# Webhook Reference

## Scopes

- Config-level: attach to a specific WhatsApp phone number (use `phone_number_id`).
- Project-level: receive lifecycle/workflow events across all numbers.
- Use config-level for any `whatsapp.message.*` and `whatsapp.conversation.*` events.
- WhatsApp message/conversation events are **not** delivered via project webhooks.

## Signature verification

Kapso signs outbound webhook requests:

- Header: `X-Webhook-Signature`
- Value: `HMAC-SHA256(webhook_secret_key, raw_request_body)` as hex

Verify against the raw request body bytes before JSON parsing.

## Event catalog

Message events (config-level):

- `whatsapp.message.received`
- `whatsapp.message.sent`
- `whatsapp.message.delivered`
- `whatsapp.message.read`
- `whatsapp.message.failed`

Conversation events:

- `whatsapp.conversation.created`
- `whatsapp.conversation.ended`
- `whatsapp.conversation.inactive`

Lifecycle events (project-level only):

- `whatsapp.config.created`
- `whatsapp.phone_number.created`
- `whatsapp.phone_number.deleted`

Workflow events:

- `workflow.execution.handoff`
- `workflow.execution.failed`

## Payload versions

- `v1`: legacy payloads with nested `whatsapp_config`.
- `v2`: modern payloads with `phone_number_id` at root (recommended).

## Buffering (message.received)

Use buffering to batch rapid inbound messages:

- `buffer_enabled`: true
- `buffer_window_seconds`: 1-60
- `max_buffer_size`: 1-100
