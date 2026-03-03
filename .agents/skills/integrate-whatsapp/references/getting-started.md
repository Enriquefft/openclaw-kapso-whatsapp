---
title: Getting started
description: Enable WhatsApp for your customers
---

Kapso Platform lets your customers connect their own WhatsApp Business accounts without sharing credentials. Each customer uses their own number while you handle the automation.

## Quick example

```javascript
// 1. Create customer
const customer = await createCustomer({ name: 'Acme Corp' });

// 2. Setup link
const setupLink = await generateSetupLink(customer.id);
console.log(`Setup link: ${setupLink.url}`);

// 3. Send message
await sendWhatsAppMessage({
  customer_id: customer.id,
  phone_number: '+1234567890',
  content: 'Order confirmed!'
});
```

## Use cases

Perfect when you need your customers to use their own WhatsApp:

- **CRM platforms** - Let each client connect their WhatsApp for customer communication
- **Appointment booking** - Clinics and salons send reminders from their own number
- **E-commerce tools** - Stores send order updates using their WhatsApp Business
- **Marketing platforms** - Agencies manage multiple client WhatsApp accounts
- **Support software** - Each company provides support through their WhatsApp

## Step 1: Create a customer

```bash
curl -X POST https://api.kapso.ai/platform/v1/customers \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "customer": {
      "name": "Acme Corporation",
      "external_customer_id": "CUS-12345"
    }
  }'
```

Response:
```json
{
  "data": {
    "id": "customer-abc123",
    "name": "Acme Corporation",
    "external_customer_id": "CUS-12345"
  }
}
```

## Step 2: Generate setup link

```bash
curl -X POST https://api.kapso.ai/platform/v1/customers/customer-abc123/setup_links \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"setup_link":{}}'
```

Response:
```json
{
  "data": {
    "id": "link-xyz789",
    "url": "https://app.kapso.ai/whatsapp/setup/aBcD123...",
    "expires_at": "2024-03-15T10:00:00Z"
  }
}
```

Share the `url` with your customer. They'll click it, log in with Facebook, and connect their WhatsApp in ~5 minutes.

## Step 3: Send messages

After setup completes, use the customer's `phone_number_id` to send messages:

```bash
curl -X POST https://api.kapso.ai/meta/whatsapp/v24.0/110987654321/messages \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "messaging_product": "whatsapp",
    "to": "15551234567",
    "type": "text",
    "text": {
      "body": "Your order has been shipped!"
    }
  }'
```

## JavaScript example

```javascript
const KAPSO_API_KEY = 'YOUR_API_KEY';
const KAPSO_API_BASE_URL = 'https://api.kapso.ai';
const PLATFORM_API_URL = `${KAPSO_API_BASE_URL}/platform/v1`;
const WHATSAPP_API_URL = `${KAPSO_API_BASE_URL}/meta/whatsapp`;

async function onboardCustomer(customerData) {
  // 1. Create customer
  const customer = await fetch(`${PLATFORM_API_URL}/customers`, {
    method: 'POST',
    headers: {
      'X-API-Key': KAPSO_API_KEY,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ customer: customerData })
  }).then(r => r.json());

  // 2. Generate setup link
  const setupLink = await fetch(
    `${PLATFORM_API_URL}/customers/${customer.data.id}/setup_links`,
    {
      method: 'POST',
      headers: {
        'X-API-Key': KAPSO_API_KEY,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ setup_link: {} })
    }
  ).then(r => r.json());

  return setupLink.data.url;
}

async function sendMessage(phoneNumberId, recipientPhone, message) {
  return fetch(`${WHATSAPP_API_URL}/v24.0/${phoneNumberId}/messages`, {
    method: 'POST',
    headers: {
      'X-API-Key': KAPSO_API_KEY,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      messaging_product: 'whatsapp',
      to: recipientPhone,
      type: 'text',
      text: { body: message }
    })
  });
}
```

## What customers see

1. Click your setup link
2. Log in with Facebook
3. Connect their WhatsApp Business
4. Verify phone number
5. Done - you can now send messages

The entire process takes ~5 minutes.

## Next steps

- [Setup links](/docs/platform/setup-links) - Customize redirect URLs, branding, connection types
- [Connection detection](/docs/platform/detecting-whatsapp-connection) - Know when customers connect
- [Webhooks](/docs/platform/webhooks/overview) - Handle real-time message events
