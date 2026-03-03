# WhatsApp Templates via Meta Proxy

## Environment

Required env vars:

- `KAPSO_API_BASE_URL` (host only, no `/platform/v1`, e.g. `https://api.kapso.ai`)
- `KAPSO_API_KEY`
- `META_GRAPH_VERSION` (optional, default: `v24.0`)
- `KAPSO_META_BASE_URL` (optional, defaults to `${KAPSO_API_BASE_URL}/meta/whatsapp`)

## Discover IDs (recommended)

Template CRUD requires `business_account_id` (WABA ID). Sending messages and uploading media require `phone_number_id` (Meta phone number id).

Use the Platform API to discover both:

- Script: `node scripts/list-platform-phone-numbers.mjs`
- Raw: `GET /platform/v1/whatsapp/phone_numbers` (header: `X-API-Key: $KAPSO_API_KEY`)

## Meta proxy endpoints used

- List WABA phone numbers:
  - `GET /{business_account_id}/phone_numbers`
- List templates:
  - `GET /{business_account_id}/message_templates`
- Create template:
  - `POST /{business_account_id}/message_templates`
- Update template:
  - `POST /{business_account_id}/message_templates?hsm_id=<template_id>`
- Delete template (not scripted):
  - `DELETE /{business_account_id}/message_templates?name=<template_name>`
- Send template message:
  - `POST /{phone_number_id}/messages`
- Upload media for send-time headers:
  - `POST /{phone_number_id}/media`

## Template concepts

Categories:
- MARKETING: promotional content.
- UTILITY: transactional updates.
- AUTHENTICATION: OTP/verification (special rules below).

AUTHENTICATION templates:
- Require Meta business verification.
- Body text is fixed by Meta (not customizable).
- Must include an OTP button (COPY_CODE or ONE_TAP).
- Send-time still requires the OTP value in body param {{1}} and URL button param.
- If user wants custom OTP text, use UTILITY instead.

Status flow:
- Kapso does not maintain a separate draft state; create/update calls go to Meta immediately.
- Use `status` from Meta (`APPROVED`, `PENDING`, `REJECTED`, etc) via list/status scripts.

Parameter types:
- POSITIONAL: `{{1}}`, `{{2}}` (sequential).
- NAMED: `{{customer_name}}` (lowercase + underscores). Prefer NAMED.

Component types:
- HEADER (optional)
- BODY (required)
- FOOTER (optional)
- BUTTONS (optional)

## Parameter format (creation time)

Set `parameter_format`:
- `POSITIONAL` (default): `{{1}}`, `{{2}}` with no gaps.
- `NAMED` (recommended): `{{order_id}}`.

## Example requirements (creation time)

If any variables appear in HEADER or BODY, you must include examples:
- POSITIONAL: `example.header_text` and 2D `example.body_text`.
- NAMED: `example.header_text_named_params` and `example.body_text_named_params`.

## Components cheat sheet (creation time)

### Header (TEXT, named)

```json
{
  "type": "HEADER",
  "format": "TEXT",
  "text": "Sale starts {{sale_date}}",
  "example": {
    "header_text_named_params": [
      { "param_name": "sale_date", "example": "December 1" }
    ]
  }
}
```

### Header (TEXT, positional)

```json
{
  "type": "HEADER",
  "format": "TEXT",
  "text": "Sale starts {{1}}",
  "example": {
    "header_text": ["December 1"]
  }
}
```

### Header (IMAGE/VIDEO/DOCUMENT)

```json
{
  "type": "HEADER",
  "format": "IMAGE",
  "example": {
    "header_handle": ["<header_handle>"]
  }
}
```

### Body (named)

```json
{
  "type": "BODY",
  "text": "Hi {{customer_name}}, order {{order_id}} is ready.",
  "example": {
    "body_text_named_params": [
      { "param_name": "customer_name", "example": "Alex" },
      { "param_name": "order_id", "example": "ORDER-123" }
    ]
  }
}
```

### Body (positional)

```json
{
  "type": "BODY",
  "text": "Order {{1}} is ready for {{2}}.",
  "example": {
    "body_text": [["ORDER-123", "Alex"]]
  }
}
```

### Footer (no variables)

```json
{
  "type": "FOOTER",
  "text": "Reply STOP to opt out"
}
```

### Buttons

```json
{
  "type": "BUTTONS",
  "buttons": [
    { "type": "QUICK_REPLY", "text": "Need help" },
    { "type": "URL", "text": "Track", "url": "https://example.com/track?id={{1}}", "example": ["https://example.com/track?id=ORDER-123"] }
  ]
}
```

Button ordering rules:
- Do not interleave QUICK_REPLY with URL/PHONE_NUMBER.
- Valid: QUICK_REPLY, QUICK_REPLY, URL, PHONE_NUMBER
- Invalid: QUICK_REPLY, URL, QUICK_REPLY
- Dynamic URL variables must be at the end of the URL.

URL button variables use positional placeholders in the URL (for example `{{1}}`). At send-time, include a `button` component with `sub_type: "url"` and the correct `index`.

Example (send-time URL button param):

```json
{
  "type": "button",
  "sub_type": "url",
  "index": "0",
  "parameters": [{ "type": "text", "text": "ORDER-123" }]
}
```

## AUTHENTICATION components

```json
{
  "type": "BODY",
  "add_security_recommendation": true,
  "code_expiration_minutes": 10
}
```

```json
{
  "type": "BUTTONS",
  "buttons": [
    { "type": "OTP", "otp_type": "COPY_CODE", "text": "Copy code" }
  ]
}
```

## Send-time components

Named parameters:

```json
{
  "type": "body",
  "parameters": [
    { "type": "text", "parameter_name": "order_id", "text": "ORDER-123" }
  ]
}
```

Positional parameters:

```json
{
  "type": "body",
  "parameters": [
    { "type": "text", "text": "ORDER-123" }
  ]
}
```

AUTHENTICATION send-time:

```json
[
  {
    "type": "body",
    "parameters": [{ "type": "text", "text": "123456" }]
  },
  {
    "type": "button",
    "sub_type": "url",
    "index": "0",
    "parameters": [{ "type": "text", "text": "123456" }]
  }
]
```

Media header send-time (use id or link, not both):

```json
{
  "type": "header",
  "parameters": [
    { "type": "image", "image": { "id": "4490709327384033" } }
  ]
}
```

## Header handle limitation

The Meta proxy does not expose resumable upload endpoints for `header_handle`. Use Platform media ingest (`/platform/v1/whatsapp/media` with `delivery: meta_resumable_asset`) if a header_handle is required.

```json
{
  "type": "header",
  "parameters": [
    { "type": "image", "image": { "link": "https://example.com/header.jpg" } }
  ]
}
```

Rules:

- Use either `id` or `link` (never both).
- Always include the header component when the template has a media header.
