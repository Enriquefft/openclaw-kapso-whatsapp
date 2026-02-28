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

- The `--to` number must include country code (e.g., `+1234567890`)
- The `--text` content should be quoted
- Only send messages when explicitly asked by the user

## Incoming Messages

Incoming WhatsApp messages are polled automatically and arrive as JSON:

```json
{"type": "message", "channel": "whatsapp", "from": "+NUMBER", "name": "Contact Name", "text": "message body"}
```

When you receive a WhatsApp message:
1. Acknowledge it naturally
2. Only reply if the user has instructed you to auto-reply, or if the context clearly calls for a response
3. Use `kapso-whatsapp-cli send` to reply

## Important Rules

- Never send messages without explicit user instruction
- Never share personal information or API keys in messages
- Keep replies concise — WhatsApp is a chat medium
- Respect the user's privacy and their contacts' privacy
