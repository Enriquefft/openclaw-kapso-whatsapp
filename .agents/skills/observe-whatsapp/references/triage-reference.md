# Debugging Workflow

## Message delivery failed

1. Collect message ID (`wamid.*`).
2. Inspect message lifecycle timeline.
3. Translate error codes into user-facing guidance.

## WhatsApp config issues

1. Run a health check on the phone number config.
2. Review token validity, messaging health, and webhook subscription.
3. Explain whether the issue is critical or degraded.

## Webhook delivery failures

1. Review recent delivery attempts.
2. Check response status codes and error messages.
3. Verify webhook URL availability and signature verification logic.

## API errors

1. Review external API call logs.
2. Filter by status code or endpoint.
3. Identify auth errors, rate limits, or upstream failures.
