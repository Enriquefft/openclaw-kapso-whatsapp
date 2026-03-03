# Message Debugging Playbook

## Message delivery failed

1. Identify the message ID (`wamid.*`).
2. Review the status timeline in order: sent -> delivered -> read.
3. Surface error codes in status events and map to remediation.

## Common issues to confirm

- Recipient phone number formatting and registration.
- Template approval status (for business-initiated messages).
- Messaging health status (LIMITED/BLOCKED).
- Webhook subscription and inbound event receipt.
