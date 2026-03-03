---
title: Connection detection
description: Know when customers complete WhatsApp onboarding
---

You have two ways to detect when customers connect their WhatsApp account through setup links.

## 1. Project webhooks

Configure a project webhook to receive the `whatsapp.phone_number.created` event. This is the recommended approach for server-to-server notifications.

### Setup

1. Open the sidebar and click **Webhooks**
2. Go to the **Project webhooks** tab
3. Click **Add Webhook**
4. Enter your HTTPS endpoint URL
5. Copy the auto-generated secret key
6. Subscribe to `whatsapp.phone_number.created` event

### Webhook payload

```json
{
  "phone_number_id": "123456789012345",
  "project": {
    "id": "990e8400-e29b-41d4-a716-446655440004"
  },
  "customer": {
    "id": "880e8400-e29b-41d4-a716-446655440003"
  }
}
```

### Handle the webhook

```javascript
app.post('/webhooks/project', async (req, res) => {
  const { event, data } = req.body;

  if (event === 'whatsapp.phone_number.created') {
    const { phone_number_id, customer } = data;

    // Update your database
    await db.customers.update(customer.id, {
      phone_number_id,
      whatsapp_connected: true,
      connected_at: new Date()
    });

    // Trigger welcome flow
    await sendWelcomeMessage(phone_number_id, customer.id);
  }

  res.status(200).send('OK');
});
```

See [webhooks documentation](/docs/platform/webhooks) for signature verification and best practices.

## 2. Success redirect URL

When customers complete WhatsApp setup, they're redirected to your `success_redirect_url` with query parameters.

### Setup

When creating a setup link, provide redirect URLs:

```javascript
const KAPSO_API_BASE_URL = 'https://api.kapso.ai';
const setupLink = await fetch(`${KAPSO_API_BASE_URL}/platform/v1/customers/customer-123/setup_links`, {
  method: 'POST',
  headers: {
    'X-API-Key': 'YOUR_API_KEY',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    setup_link: {
      success_redirect_url: 'https://your-app.com/whatsapp/success',
      failure_redirect_url: 'https://your-app.com/whatsapp/failed'
    }
  })
});
```

### Query parameters

After successful setup, customer is redirected to:

```
https://your-app.com/whatsapp/success?setup_link_id=...&status=completed&phone_number_id=123456789012345&business_account_id=...&provisioned_phone_number_id=...&display_phone_number=%2B15551234567
```

**Parameters**:
- `setup_link_id` - UUID of the setup link
- `status` - Always `completed` for success
- `phone_number_id` - WhatsApp phone number ID (primary identifier)
- `business_account_id` - Meta WABA ID (if available)
- `provisioned_phone_number_id` - Kapso phone number ID (if provisioning was used)
- `display_phone_number` - E.164 formatted phone number (URL encoded)

### Handle the redirect

```javascript
app.get('/whatsapp/success', async (req, res) => {
  const {
    setup_link_id,
    status,
    phone_number_id,
    business_account_id,
    provisioned_phone_number_id,
    display_phone_number
  } = req.query;

  // Update your database
  await db.customers.update({
    phone_number_id,
    business_account_id,
    display_phone_number: decodeURIComponent(display_phone_number),
    whatsapp_connected: true,
    connected_at: new Date()
  });

  // Show success page to customer
  res.render('whatsapp-connected', {
    phoneNumber: decodeURIComponent(display_phone_number)
  });
});
```

<Note>
These parameters are convenience identifiers to avoid extra API fetches. Use `phone_number_id` as the primary identifier.
</Note>

### Failure redirect

If setup fails, customer is redirected to your `failure_redirect_url`:

```
https://your-app.com/whatsapp/failed?setup_link_id=...&error_code=facebook_auth_failed
```

**Error codes**:
- `facebook_auth_failed` - Facebook login cancelled
- `phone_verification_failed` - Phone verification failed
- `waba_limit_reached` - Too many WhatsApp accounts
- `token_exchange_failed` - OAuth failed
- `link_expired` - Link expired (30 days)
- `already_used` - Link already used

```javascript
app.get('/whatsapp/failed', (req, res) => {
  const { setup_link_id, error_code } = req.query;

  // Log failure for monitoring
  await logSetupFailure(setup_link_id, error_code);

  // Show user-friendly error message
  res.render('whatsapp-setup-failed', {
    errorMessage: getErrorMessage(error_code)
  });
});
```

## Choosing the right method

**Use project webhooks when**:
- You need server-to-server notification
- Customer doesn't need immediate visual feedback
- You're building automated onboarding flows
- You need to process the connection before showing UI

**Use success redirect when**:
- Customer needs immediate confirmation in your app
- You want to show a custom success page
- You're building a wizard-style onboarding flow
- You need to collect additional information after connection

**Use both**:
- Webhook for backend processing (database updates, welcome messages)
- Redirect for frontend experience (success page, next steps)
