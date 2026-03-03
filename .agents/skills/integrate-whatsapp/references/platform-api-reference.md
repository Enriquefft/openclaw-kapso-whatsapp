# Kapso Platform API Overview

## Authentication

All Platform API requests require:

```
X-API-Key: <api_key>
```

Base host: `https://api.kapso.ai`
Platform API base path: `/platform/v1`

## Meta proxy (WhatsApp Cloud API)

Base URL: `https://api.kapso.ai/meta/whatsapp/v24.0`

Use Meta proxy for WhatsApp Cloud API calls (messages, templates, media, flows). Auth still uses `X-API-Key`.

## Multi-tenant WhatsApp (Customers)

Use Customers when your end-users connect their own WhatsApp numbers.

Flow:
1. Create customer
2. Create setup link
3. Customer completes embedded signup
4. Use their phone_number_id for sending

Endpoints:
- `POST /customers`
- `GET /customers`
- `GET /customers/:id`
- `POST /customers/:customer_id/setup_links`
- `POST /customers/:customer_id/whatsapp/phone_numbers`
- `GET /whatsapp/phone_numbers?customer_id=<uuid>` (filter phone numbers by customer)

If you are only sending from your own WhatsApp number, skip Customers.

## Core Platform API endpoints

Webhooks:
- Project-level: `GET/POST /whatsapp/webhooks`
- Config-level: `GET/POST /whatsapp/phone_numbers/:id/webhooks`
- Test delivery: `POST /whatsapp/webhooks/:id/test`

Messages and conversations:
- `GET /whatsapp/messages`
- `GET /whatsapp/messages/:id` (WAMID)
- `GET /whatsapp/conversations`
- `GET /whatsapp/conversations/:id`

Message list query params (use `GET /whatsapp/messages`):
- `phone_number_id`, `conversation_id`, `phone_number`
- `direction` (inbound|outbound), `status` (pending|sent|delivered|read|failed)
- `message_type` (text|image|audio|video|document), `has_media` (true|false)
- `page`, `per_page`

Example:
`GET /whatsapp/messages?conversation_id=<uuid>&phone_number_id=<id>&direction=inbound&per_page=50`

Conversation list query params (use `GET /whatsapp/conversations`):
- `phone_number_id`, `phone_number`
- `status` (active|ended)
- `page`, `per_page`

Example:
`GET /whatsapp/conversations?phone_number_id=<id>&status=active&per_page=50`

Workflows:
- `GET /workflows`
- `POST /workflows`
- `GET /workflows/:id`
- `GET /workflows/:id/definition`
- `PATCH /workflows/:id`
- `GET /workflows/:id/variables`
- `GET /workflows/:workflow_id/executions`
- `POST /workflows/:workflow_id/executions`
- `GET /workflow_executions/:id`
- `PATCH /workflow_executions/:id`
- `POST /workflow_executions/:id/resume`
- `GET /workflows/:workflow_id/triggers`
- `POST /workflows/:workflow_id/triggers`
- `PATCH /triggers/:id`
- `DELETE /triggers/:id`

Functions:
- `GET /functions`
- `POST /functions`
- `GET /functions/:id`
- `PATCH /functions/:id`
- `POST /functions/:id/deploy`
- `POST /functions/:id/invoke`
- `GET /functions/:id/invocations`

Integrations:
- `GET /integrations`
- `POST /integrations`
- `PATCH /integrations/:id`
- `DELETE /integrations/:id`
- `GET /integrations/apps`
- `GET /integrations/actions`
- `GET /integrations/accounts`
- `POST /integrations/connect_token`
- `GET /integrations/actions/:action_id/schema`
- `POST /integrations/actions/:action_id/configure_prop`
- `POST /integrations/actions/:action_id/reload_props`

Logs:
- `GET /api_logs`
- `GET /webhook_deliveries`

Provider models:
- `GET /provider_models`

## Guidance

- For template creation/sending, use the Meta proxy endpoints (see `integrate-whatsapp` skill).
- For WhatsApp Flows, use Platform API flow endpoints (see `integrate-whatsapp` skill).
- For workflow graph edits, use workflow definition endpoints (see `automate-whatsapp` skill).
