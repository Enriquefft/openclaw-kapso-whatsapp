---
title: WhatsApp Cloud API via Kapso Proxy
---

# WhatsApp Cloud API (Kapso Meta Proxy)

REST API reference for sending messages, managing templates, and querying history via Kapso's Meta proxy.

## Base URL and auth

```
Base URL: ${KAPSO_API_BASE_URL}/meta/whatsapp/v24.0
Auth header: X-API-Key: <api_key>
```

All payloads mirror the Meta Cloud API. Kapso adds storage and query features.

## Send messages

`POST /{phone_number_id}/messages`

All payloads require `messaging_product: "whatsapp"`.

### Text

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "text",
  "text": { "body": "Hello!", "preview_url": true }
}
```

### Image

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "image",
  "image": { "link": "https://example.com/photo.jpg", "caption": "Photo" }
}
```

Use `id` instead of `link` for uploaded media.

### Video

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "video",
  "video": { "link": "https://example.com/clip.mp4", "caption": "Video" }
}
```

### Audio

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "audio",
  "audio": { "link": "https://example.com/audio.mp3" }
}
```

### Voice message

Voice messages require `.ogg` files with OPUS codec. Set `voice: true`:

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "audio",
  "audio": { "id": "<MEDIA_ID>", "voice": true }
}
```

### Document

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "document",
  "document": { "link": "https://example.com/file.pdf", "filename": "report.pdf", "caption": "Report" }
}
```

### Sticker

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "sticker",
  "sticker": { "id": "<MEDIA_ID>" }
}
```

### Location

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "location",
  "location": { "latitude": 37.7749, "longitude": -122.4194, "name": "SF Office", "address": "123 Main St" }
}
```

### Contacts

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "contacts",
  "contacts": [{
    "name": { "formatted_name": "John Doe", "first_name": "John", "last_name": "Doe" },
    "phones": [{ "phone": "+15551234567", "type": "MOBILE", "wa_id": "15551234567" }],
    "emails": [{ "email": "john@example.com", "type": "WORK" }]
  }]
}
```

### Reaction

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "reaction",
  "reaction": { "message_id": "wamid......", "emoji": "👍" }
}
```

Note: Reactions only trigger `sent` status webhook (not delivered/read).

### Reply to a message

Add `context` to reply to a specific message:

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "text",
  "context": { "message_id": "wamid......" },
  "text": { "body": "Thanks for your message!" }
}
```

## Interactive messages

Require an active 24-hour session window. Use templates for outbound notifications outside the window.

### Buttons

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "interactive",
  "interactive": {
    "type": "button",
    "body": { "text": "Choose an option" },
    "action": {
      "buttons": [
        { "type": "reply", "reply": { "id": "yes", "title": "Yes" } },
        { "type": "reply", "reply": { "id": "no", "title": "No" } }
      ]
    }
  }
}
```

Max 3 buttons. Button titles max 20 chars.

### List

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "interactive",
  "interactive": {
    "type": "list",
    "header": { "type": "text", "text": "Shipping Options" },
    "body": { "text": "Choose your preferred shipping" },
    "footer": { "text": "Estimates may vary" },
    "action": {
      "button": "View Options",
      "sections": [{
        "title": "Fast",
        "rows": [
          { "id": "express", "title": "Express", "description": "1-2 days" },
          { "id": "priority", "title": "Priority", "description": "2-3 days" }
        ]
      }]
    }
  }
}
```

Max 10 sections, 10 rows total. Button text max 20 chars.

### CTA URL

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "interactive",
  "interactive": {
    "type": "cta_url",
    "body": { "text": "Track your order" },
    "action": {
      "name": "cta_url",
      "parameters": { "display_text": "Track Order", "url": "https://example.com/track/123" }
    }
  }
}
```

### Location request

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "interactive",
  "interactive": {
    "type": "location_request_message",
    "body": { "text": "Please share your location for delivery." },
    "action": { "name": "send_location" }
  }
}
```

### Flow

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "interactive",
  "interactive": {
    "type": "flow",
    "header": { "type": "text", "text": "Book Appointment" },
    "body": { "text": "Schedule your visit" },
    "action": {
      "name": "flow",
      "parameters": {
        "flow_message_version": "3",
        "flow_id": "123456789",
        "flow_cta": "Book Now",
        "mode": "published",
        "flow_token": "session_abc123",
        "flow_action": "navigate",
        "flow_action_payload": {
          "screen": "WELCOME_SCREEN",
          "data": { "customer_id": "cust_123" }
        }
      }
    }
  }
}
```

### Product

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "interactive",
  "interactive": {
    "type": "product",
    "body": { "text": "Check out this item" },
    "action": {
      "catalog_id": "CATALOG_ID",
      "product_retailer_id": "SKU_1234"
    }
  }
}
```

### Product list

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "interactive",
  "interactive": {
    "type": "product_list",
    "header": { "type": "text", "text": "Our Bestsellers" },
    "body": { "text": "Choose a product" },
    "action": {
      "catalog_id": "CATALOG_ID",
      "sections": [{
        "title": "Popular",
        "product_items": [
          { "product_retailer_id": "SKU_1234" },
          { "product_retailer_id": "SKU_2345" }
        ]
      }]
    }
  }
}
```

Max 10 sections, 30 products total.

### Catalog message

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "interactive",
  "interactive": {
    "type": "catalog_message",
    "body": { "text": "Browse our catalog." },
    "action": {
      "name": "catalog_message",
      "parameters": { "thumbnail_product_retailer_id": "SKU_THUMBNAIL" }
    }
  }
}
```

## Template messages

### Send with named parameters

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "template",
  "template": {
    "name": "order_confirmation",
    "language": { "code": "en_US" },
    "components": [{
      "type": "body",
      "parameters": [
        { "type": "text", "parameter_name": "customer_name", "text": "Jessica" },
        { "type": "text", "parameter_name": "order_number", "text": "ORD-12345" }
      ]
    }]
  }
}
```

### Send with positional parameters

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "template",
  "template": {
    "name": "order_confirmation",
    "language": { "code": "en_US" },
    "components": [{
      "type": "body",
      "parameters": [
        { "type": "text", "text": "Jessica" },
        { "type": "text", "text": "ORD-12345" }
      ]
    }]
  }
}
```

### Send with media header

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "template",
  "template": {
    "name": "seasonal_promotion",
    "language": { "code": "en_US" },
    "components": [
      {
        "type": "header",
        "parameters": [{ "type": "image", "image": { "link": "https://example.com/promo.jpg" } }]
      },
      {
        "type": "body",
        "parameters": [
          { "type": "text", "parameter_name": "sale_name", "text": "Summer Sale" },
          { "type": "text", "parameter_name": "discount", "text": "25%" }
        ]
      }
    ]
  }
}
```

### Send with URL button variable

```json
{
  "messaging_product": "whatsapp",
  "to": "15551234567",
  "type": "template",
  "template": {
    "name": "order_tracking",
    "language": { "code": "en_US" },
    "components": [
      {
        "type": "body",
        "parameters": [{ "type": "text", "parameter_name": "order_id", "text": "ORD-123" }]
      },
      {
        "type": "button",
        "sub_type": "url",
        "index": "0",
        "parameters": [{ "type": "text", "text": "ORD-123" }]
      }
    ]
  }
}
```

See [templates-reference.md](templates-reference.md) for full component rules.

## Template CRUD

### List templates

`GET /{business_account_id}/message_templates`

| Param | Description |
|-------|-------------|
| `name` | Filter by template name |
| `status` | `APPROVED`, `PENDING`, `REJECTED` |
| `category` | `AUTHENTICATION`, `MARKETING`, `UTILITY` |
| `language` | Language code (e.g., `en_US`) |
| `limit` | Max 100 |

### Create template

`POST /{business_account_id}/message_templates`

```json
{
  "name": "order_confirmation",
  "language": "en_US",
  "category": "UTILITY",
  "parameter_format": "NAMED",
  "components": [
    {
      "type": "BODY",
      "text": "Thank you, {{customer_name}}! Your order {{order_number}} is confirmed.",
      "example": {
        "body_text_named_params": [
          { "param_name": "customer_name", "example": "Pablo" },
          { "param_name": "order_number", "example": "ORD-12345" }
        ]
      }
    }
  ]
}
```

Response includes `id` and `status` (usually `PENDING`).

### Update template

`PUT /{business_account_id}/message_templates?hsm_id=<template_id>`

Same body structure as create.

### Delete template

`DELETE /{business_account_id}/message_templates?name=<name>` or `?hsm_id=<template_id>`

## Mark as read

`POST /{phone_number_id}/messages`

```json
{
  "messaging_product": "whatsapp",
  "status": "read",
  "message_id": "wamid......"
}
```

### With typing indicator

```json
{
  "messaging_product": "whatsapp",
  "status": "read",
  "message_id": "wamid......",
  "typing_indicator": { "type": "text" }
}
```

Typing indicator dismisses on send or after ~25s.

## Media

### Upload

`POST /{phone_number_id}/media` (multipart/form-data)

| Field | Value |
|-------|-------|
| `file` | Binary file |
| `messaging_product` | `whatsapp` |

Returns `{ "id": "<MEDIA_ID>" }` for use in send payloads.

### Get URL

`GET /{media_id}?phone_number_id=<phone_number_id>`

Returns temporary download URL (valid 5 minutes).

### Delete

`DELETE /{media_id}?phone_number_id=<phone_number_id>`

### Format limits

| Type | Formats | Max Size |
|------|---------|----------|
| Image | JPEG, PNG | 5 MB |
| Video | MP4, 3GP (H.264 + AAC) | 16 MB |
| Audio | AAC, AMR, MP3, M4A, OGG | 16 MB |
| Voice | OGG (OPUS codec only) | 16 MB |
| Document | PDF, DOC, DOCX, PPT, PPTX, XLS, XLSX, TXT | 100 MB |
| Sticker (static) | WEBP | 100 KB |
| Sticker (animated) | WEBP | 500 KB |

## Query history (Kapso)

These endpoints are Kapso-specific for stored conversation data.

### List messages

`GET /{phone_number_id}/messages`

| Param | Description |
|-------|-------------|
| `conversation_id` | Filter by conversation UUID |
| `direction` | `inbound` or `outbound` |
| `status` | `pending`, `sent`, `delivered`, `read`, `failed` |
| `since` / `until` | ISO 8601 timestamps |
| `limit` | Max 100 |
| `before` / `after` | Cursor pagination |
| `fields` | Use `kapso(...)` for extra fields |

### List conversations

`GET /{phone_number_id}/conversations`

| Param | Description |
|-------|-------------|
| `status` | `active` or `ended` |
| `last_active_since` / `last_active_until` | ISO 8601 timestamps |
| `phone_number` | Filter by customer phone (E.164) |
| `limit` | Max 100 |
| `before` / `after` | Cursor pagination |
| `fields` | Use `kapso(...)` for extra fields |

### Get conversation

`GET /{phone_number_id}/conversations/{conversation_id}`

### List contacts

`GET /{phone_number_id}/contacts`

| Param | Description |
|-------|-------------|
| `wa_id` | Filter by WhatsApp ID |
| `customer_id` | Filter by associated customer |
| `has_customer` | `true` or `false` |
| `limit` | Max 100 |
| `before` / `after` | Cursor pagination |

### Get contact

`GET /{phone_number_id}/contacts/{wa_id}`

## Response format

Successful send returns:

```json
{
  "messaging_product": "whatsapp",
  "contacts": [{ "input": "15551234567", "wa_id": "15551234567" }],
  "messages": [{ "id": "wamid.HBgN..." }]
}
```

## Errors

| Code | Description |
|------|-------------|
| 131047 | 24-hour window expired. Use template instead. |
| 1026 | Receiver incapable (e.g., address_message not supported) |
| 409 | Another message in-flight for this conversation. Retry shortly. |

## Kapso extensions

Add `fields=kapso(...)` to list endpoints:

- `kapso(default)` or `kapso(*)` - all default fields
- `kapso(direction,media_url,contact_name)` - specific fields
- `kapso()` - omit Kapso fields

Common fields: `direction`, `status`, `media_url`, `contact_name`, `flow_response`, `flow_token`, `content`, `message_type_data`.

## Notes

- Discover `phone_number_id` + `business_account_id` via `node scripts/list-platform-phone-numbers.mjs`
- All send payloads require `messaging_product: "whatsapp"`
- Graph version controlled by `META_GRAPH_VERSION` (default `v24.0`)
