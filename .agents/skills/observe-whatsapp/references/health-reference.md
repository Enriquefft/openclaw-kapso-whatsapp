# Health Check Interpretation

## 1) 60-second triage order

1. Confirm the health check ran (status + timestamp).
2. Identify the blocking check in this order:
   - `checks.phone_number_access.passed`
   - `checks.token_validity.passed` (only present on some configs)
   - `checks.messaging_health.overall_status` (BLOCKED = critical, LIMITED = degraded)
   - `checks.webhook_verified.passed`
   - `checks.webhook_subscription.passed`
   - `checks.test_message.passed` (only if a test was requested)
3. Lead with overall status (healthy/degraded/unhealthy), then name failing checks.

## 2) If status is "error"

Explain that the health check itself failed to run. Surface the error message and timestamp, then ask them to retry. If it repeats, collect JSON + config id for escalation.

## 3) Overall status (status)

This is the UI headline only. Always explain the underlying checks too.

- `healthy`: all required checks passed.
- `degraded`: core access works, but something needs attention (webhook, messaging limited, test failed).
- `unhealthy`: a critical requirement failed or messaging is blocked.
- `error`: health check failed to run.

## 4) Access token check (checks.token_validity)

Only present on some configs.

- passed: true -> token is valid.
- passed: false -> critical blocker.

Tell the user: token is invalid/expired or missing permissions. They cannot fix this directly; escalate to support.

## 5) Phone number access (checks.phone_number_access)

- passed: false -> critical blocker (Meta access is failing).
- passed: true -> show details if present.

Details:
- `details.verified_name`, `details.display_phone_number`, `details.quality_rating` (GREEN/YELLOW/RED).
- If `details.status` is "PENDING" (case-insensitive), treat as account under review.

Account under review guidance:
- "Meta is reviewing your account; this is normal and can take a few hours."
- Tell them to check Meta account status in WhatsApp Manager.

## 6) Messaging health (checks.messaging_health)

Use `checks.messaging_health.overall_status`:
- `AVAILABLE`: can send normally.
- `LIMITED`: partially restricted.
- `BLOCKED`: sending blocked.

If `checks.messaging_health.error` exists, say Meta status couldn't be retrieved.

### Entity-level diagnosis
If overall status is LIMITED or BLOCKED, inspect `checks.messaging_health.details.entities[]`.
Pick the most actionable entity:
1. Any BLOCKED entity with errors
2. Else any LIMITED entity with additional_info
3. Else any LIMITED/BLOCKED entity not BUSINESS
4. Else BUSINESS (only if it's the only limited one)

Entity name mapping:
- PHONE_NUMBER -> Phone Number
- WABA -> WhatsApp Business Account
- BUSINESS -> Business Portfolio
- APP -> Application
- MESSAGE_TEMPLATE -> Message Template

Description sources:
- additional_info joined if present
- else first error's error_description plus possible_solution
- else "Check Meta Business Suite for details"

Special case (payment method issue):
- If LIMITED with payment issue, explain templates are blocked but 24-hour window messages still work.

## 7) Webhook verification (checks.webhook_verified)

- passed: false -> inbound events won't work until verified.
- Message to user: "Meta hasn't verified the webhook yet; it will verify after a message is sent."

Steps:
1. Send a WhatsApp message to the business number.
2. Re-run health check.
3. Confirm `webhook_verified.passed` is true.

## 8) Webhook subscription (checks.webhook_subscription)

- passed: false -> app not subscribed to webhook events.
- If they used their own Meta app, this may still work; confirm actual inbound events.

## 9) Test message (checks.test_message)

- passed: false -> use error as primary explanation.
- Confirm the number used from `details.phone_number`.

Reminder: recipient must message first to open the 24-hour window.

## 10) What to collect when escalating

- timestamp, status
- any `checks.*.error` strings
- for limitations: entity_type, id, can_send_message, errors, additional_info
- phone number details: id, display_phone_number, status

## 11) Display name note

If display name is not approved, it affects cold message limits but is not usually the error source.

## 12) Special error

If you see error `141000`, tell the user to contact support.
