---
name: whatsapp
description: Send and receive WhatsApp messages via Kapso
---

# WhatsApp Messaging (Kapso)

## Sending Messages

```bash
kapso-whatsapp-cli send --to +NUMBER --text "Your message here"
```

- `--to` must include `+` and country code (e.g. `+51926689401`)
- Add `+` if the number is missing it (e.g. `51926689401` → `+51926689401`)

## Two Distinct Use Cases

### 1. Owner → Agent (incoming)

Incoming messages arrive as:
```
[WhatsApp from NUMBER (Name)] message body
```

These are always from the owner. **Always reply immediately** to the sender number. No permission needed.

### 2. Agent → Third Party (outbound)

Only send to contacts other than the incoming sender when the owner **explicitly instructs** it. Never proactively message third parties.

## Rules

- Never share personal information or API keys
- Replies should be concise — WhatsApp is a chat medium
- Respect contact privacy
