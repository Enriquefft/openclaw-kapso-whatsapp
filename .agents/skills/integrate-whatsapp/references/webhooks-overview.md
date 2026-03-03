---
title: Webhooks Overview (Kapso)
---

# Webhooks Overview

## Webhook types

### Project webhooks
Project-wide events (for example, `whatsapp.phone_number.created`).
Use **project webhooks** for connection lifecycle and workflow events only.

### WhatsApp webhooks
Message and conversation events for a specific `phone_number_id`.
Use **phone-number webhooks** for `whatsapp.message.*` and `whatsapp.conversation.*` events only.
WhatsApp message events cannot be delivered via project webhooks.

Kinds:

- **Kapso webhooks** (default): event-based payloads, filtering, buffering.
- **Meta webhooks**: raw Meta payloads, no filtering or buffering. One meta webhook per phone number.

Meta webhooks include `X-Idempotency-Key` (SHA256 hash of payload) for deduplication.

## Response requirements

- Your endpoint must return `200 OK` within 10 seconds.
- Non-200 responses trigger retries.

Retry schedule (Kapso webhooks):
- 10 seconds
- 40 seconds
- 90 seconds

## Signature verification

Kapso signs webhook requests:

- Header: `X-Webhook-Signature`
- Value: `HMAC-SHA256(webhook_secret_key, raw_request_body)` as hex

Verify against raw request bytes before parsing JSON.

## Headers (Kapso webhooks)

- `X-Webhook-Event`
- `X-Webhook-Signature`
- `X-Idempotency-Key`
- `X-Webhook-Payload-Version`
- `Content-Type: application/json`

Batched payloads may include:

- `X-Webhook-Batch: true`
- `X-Batch-Size: <n>`
