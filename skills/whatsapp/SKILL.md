---
name: whatsapp
description: Send and receive WhatsApp messages via Kapso
---

# WhatsApp Messaging (Kapso)

You can send and receive WhatsApp messages through the Kapso WhatsApp bridge.

## Sending Messages

Use the `kapso-whatsapp-cli` command:

```bash
kapso-whatsapp-cli send --to +NUMBER --text "Your message here"
```

- The `--to` number must include country code with `+` prefix (e.g., `+1234567890`)
- If the number is missing the `+`, add it (e.g., `51926689401` → `+51926689401`)
- Keep replies concise — WhatsApp is a chat medium

## Incoming Messages

Incoming WhatsApp messages arrive in the session with this format:

```
[WhatsApp from NUMBER (Contact Name)] message body
```

When you receive a WhatsApp message in this format:
1. Always reply to the sender immediately using `kapso-whatsapp-cli send`
2. Extract the number from the prefix and add `+` if missing
3. Do not ask for permission — just reply

## Important Rules

- Never share personal information or API keys in messages
- Respect the user's privacy and their contacts' privacy
