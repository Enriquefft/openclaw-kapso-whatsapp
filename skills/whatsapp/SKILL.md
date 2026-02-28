---
name: whatsapp
description: Send WhatsApp messages via Kapso (outbound to third parties on owner instruction)
---

# WhatsApp Messaging (Kapso)

## Sending Messages (outbound to third parties)

```bash
kapso-whatsapp-cli send --to +NUMBER --text "Your message here"
```

- `--to` must include `+` and country code (e.g. `+51926689401`)
- Add `+` if the number is missing it (e.g. `51926689401` → `+51926689401`)

Use this tool only when the owner **explicitly instructs** you to contact a third party.
Confirm the number and message with the owner before sending unless they've been very explicit.

## Rules

- Never share personal information or API keys
- Never proactively message third parties
- Respect contact privacy
