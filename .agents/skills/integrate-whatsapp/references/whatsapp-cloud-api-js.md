---
title: whatsapp-cloud-api-js SDK
---

# whatsapp-cloud-api-js

Use the `@kapso/whatsapp-cloud-api` SDK for typed WhatsApp Cloud API calls.

## Install

```bash
npm install @kapso/whatsapp-cloud-api
```

## Create a client

Kapso proxy setup:

```ts
import { WhatsAppClient } from "@kapso/whatsapp-cloud-api";

const client = new WhatsAppClient({
  baseUrl: "https://api.kapso.ai/meta/whatsapp",
  kapsoApiKey: process.env.KAPSO_API_KEY!
});
```

Direct Meta setup:

```ts
const client = new WhatsAppClient({
  accessToken: process.env.WHATSAPP_TOKEN!
});
```

## Send a text message

```ts
await client.messages.sendText({
  phoneNumberId: "<PHONE_NUMBER_ID>",
  to: "+15551234567",
  body: "Hello from Kapso"
});
```

## Send a template message

```ts
await client.messages.sendTemplate({
  phoneNumberId: "<PHONE_NUMBER_ID>",
  to: "+15551234567",
  template: {
    name: "order_ready_named",
    language: { code: "en_US" },
    components: [
      {
        type: "body",
        parameters: [
          { type: "text", parameterName: "order_id", text: "ORDER-123" }
        ]
      }
    ]
  }
});
```

## Send an interactive button message

```ts
await client.messages.sendInteractiveButtons({
  phoneNumberId: "<PHONE_NUMBER_ID>",
  to: "+15551234567",
  bodyText: "Choose an option",
  buttons: [
    { id: "accept", title: "Accept" },
    { id: "decline", title: "Decline" }
  ]
});
```

## List conversations

```ts
const conversations = await client.conversations.list({
  phoneNumberId: "<PHONE_NUMBER_ID>",
  status: "active",
  limit: 20
});
```

## Get a conversation

```ts
const conversation = await client.conversations.get({
  conversationId: "123e4567-e89b-12d3-a456-426614174000"
});
```

## List messages

```ts
const messages = await client.messages.query({
  phoneNumberId: "<PHONE_NUMBER_ID>",
  conversationId: "123e4567-e89b-12d3-a456-426614174000",
  limit: 50
});
```

## List messages for a conversation (shortcut)

```ts
const messages = await client.messages.listByConversation({
  phoneNumberId: "<PHONE_NUMBER_ID>",
  conversationId: "123e4567-e89b-12d3-a456-426614174000",
  limit: 50
});
```

## Notes

- Use `phoneNumberId` from the connected WhatsApp number (discover via `node scripts/list-platform-phone-numbers.mjs`).
- With Kapso proxy, keep `baseUrl` and `kapsoApiKey` set.
- Template rules still apply (examples, button ordering, media headers).
- History endpoints (`messages.query`, `messages.listByConversation`, `conversations.list/get`) require Kapso proxy; they are not available with a direct Meta access token.
- Requests use camelCase keys and the SDK converts to snake_case for the API; responses come back camelCase.
