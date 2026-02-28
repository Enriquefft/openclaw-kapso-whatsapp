---
name: whatsapp
description: Send and receive WhatsApp messages via Kapso
---

# WhatsApp Messaging (Kapso)

## Sending Messages (outbound to third parties)

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

These are always from the owner. **Just respond naturally** — your reply is automatically
relayed back to WhatsApp by the bridge. No need to call `kapso-whatsapp-cli` for the reply.

Keep replies concise. Use plain text (no markdown) unless the owner asks for code or formatting.

### 2. Agent → Third Party (outbound)

Only send to contacts other than the incoming sender when the owner **explicitly instructs** it.
Confirm the number and message before sending. Never proactively message third parties.

## Rules

- Never share personal information or API keys
- Replies should be concise — WhatsApp is a chat medium
- Respect contact privacy
