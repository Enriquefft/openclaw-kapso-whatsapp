---
title: Event types
description: Available webhook events and their payloads
---

All webhook payloads use v2 format with `phone_number_id` at the top level.

## Payload structure

Webhook payloads separate message data from conversation data:

- **message.kapso** - Message-scoped only: direction, status, processing_status, statuses (raw status history), origin, has_media, content (text representation), transcript (for audio), media helpers (media_data, media_url, message_type_data)
- **conversation** - Top-level identifiers (id, phone_number, phone_number_id). Optional conversation.kapso contains summary metrics (counts, last-message metadata, timestamps)
- **phone_number_id** - Included at top level for routing

## Project webhook events

Use project webhooks for connection lifecycle and workflow events only.

### whatsapp.phone_number.created

Fires when a customer successfully connects their WhatsApp through a setup link.

See [Connection detection](/docs/platform/detecting-whatsapp-connection) for implementation guide.

**Payload**:

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

### workflow.execution.handoff

Fires when a workflow execution is handed off to a human agent.

**Payload**:

```json
{
  "event": "workflow.execution.handoff",
  "occurred_at": "2025-12-08T12:00:00Z",
  "project_id": "990e8400-e29b-41d4-a716-446655440004",
  "workflow_id": "880e8400-e29b-41d4-a716-446655440001",
  "workflow_execution_id": "770e8400-e29b-41d4-a716-446655440002",
  "status": "handoff",
  "tracking_id": "track-abc123",
  "channel": "whatsapp",
  "whatsapp_conversation_id": "conv_789",
  "handoff": {
    "reason": "User requested human assistance",
    "source": "agent_tool"
  }
}
```

| Field | Description |
|-------|-------------|
| `handoff.reason` | Optional reason provided during handoff |
| `handoff.source` | `agent_tool` (from agent step) or `action_step` (from workflow action) |

### workflow.execution.failed

Fires when a workflow execution fails due to an error.

**Payload**:

```json
{
  "event": "workflow.execution.failed",
  "occurred_at": "2025-12-08T12:00:00Z",
  "project_id": "990e8400-e29b-41d4-a716-446655440004",
  "workflow_id": "880e8400-e29b-41d4-a716-446655440001",
  "workflow_execution_id": "770e8400-e29b-41d4-a716-446655440002",
  "status": "failed",
  "tracking_id": "track-abc123",
  "channel": "whatsapp",
  "whatsapp_conversation_id": "conv_789",
  "error": {
    "message": "Workflow execution timed out"
  }
}
```

## WhatsApp webhook events
Use phone-number webhooks for `whatsapp.message.*` and `whatsapp.conversation.*` events only.

<CardGroup cols={2}>
  <Card title="Message received" icon="message">
    `whatsapp.message.received`

    Fired when a new WhatsApp message is received from a customer. Supports message buffering for batch delivery.
  </Card>
  <Card title="Message sent" icon="paper-plane">
    `whatsapp.message.sent`

    Fired when a message is successfully sent to WhatsApp
  </Card>
  <Card title="Message delivered" icon="check">
    `whatsapp.message.delivered`

    Fired when a message is successfully delivered to the recipient's device
  </Card>
  <Card title="Message read" icon="eye">
    `whatsapp.message.read`

    Fired when the recipient reads your message
  </Card>
  <Card title="Message failed" icon="triangle-exclamation">
    `whatsapp.message.failed`

    Fired when a message fails to deliver
  </Card>
  <Card title="Conversation created" icon="comments">
    `whatsapp.conversation.created`

    Fired when a new WhatsApp conversation is initiated
  </Card>
  <Card title="Conversation ended" icon="clock">
    `whatsapp.conversation.ended`

    Fired when a WhatsApp conversation ends (agent action, manual closure, or 24-hour inactivity)
  </Card>
  <Card title="Conversation inactive" icon="timer">
    `whatsapp.conversation.inactive`

    Fired when no messages (inbound/outbound) for configured minutes (1-1440, default 60)
  </Card>
</CardGroup>

## Payload structures

### whatsapp.message.received

```json
{
  "message": {
    "id": "wamid.123",
    "timestamp": "1730092800",
    "type": "text",
    "text": { "body": "Hello" },
    "kapso": {
      "direction": "inbound",
      "status": "received",
      "processing_status": "pending",
      "origin": "cloud_api",
      "has_media": false,
      "content": "Hello"
    }
  },
  "conversation": {
    "id": "conv_123",
    "phone_number": "+15551234567",
    "status": "active",
    "last_active_at": "2025-10-28T14:25:01Z",
    "created_at": "2025-10-28T13:40:00Z",
    "updated_at": "2025-10-28T14:25:01Z",
    "metadata": {},
    "phone_number_id": "123456789012345",
    "kapso": {
      "contact_name": "John Doe",
      "messages_count": 1,
      "last_message_id": "wamid.123",
      "last_message_type": "text",
      "last_message_timestamp": "2025-10-28T14:25:01Z",
      "last_message_text": "Hello",
      "last_inbound_at": "2025-10-28T14:25:01Z",
      "last_outbound_at": null
    }
  },
  "is_new_conversation": true,
  "phone_number_id": "123456789012345"
}
```

### whatsapp.message.sent

```json
{
  "message": {
    "id": "wamid.456",
    "timestamp": "1730092860",
    "type": "text",
    "text": { "body": "On my way" },
    "kapso": {
      "direction": "outbound",
      "status": "sent",
      "processing_status": "completed",
      "origin": "cloud_api",
      "has_media": false,
      "statuses": [
        {
          "id": "wamid.456",
          "status": "sent",
          "timestamp": "1730092860",
          "recipient_id": "15551234567"
        }
      ]
    }
  },
  "conversation": {
    "id": "conv_123",
    "phone_number": "+15551234567",
    "status": "active",
    "last_active_at": "2025-10-28T14:31:00Z",
    "created_at": "2025-10-28T13:40:00Z",
    "updated_at": "2025-10-28T14:31:00Z",
    "metadata": {},
    "phone_number_id": "123456789012345",
    "kapso": {
      "contact_name": "John Doe",
      "messages_count": 2,
      "last_message_id": "wamid.456",
      "last_message_type": "text",
      "last_message_timestamp": "2025-10-28T14:31:00Z",
      "last_message_text": "On my way",
      "last_inbound_at": "2025-10-28T14:25:01Z",
      "last_outbound_at": "2025-10-28T14:31:00Z"
    }
  },
  "is_new_conversation": false,
  "phone_number_id": "123456789012345"
}
```

### whatsapp.message.delivered

```json
{
  "message": {
    "id": "wamid.456",
    "timestamp": "1730092888",
    "type": "text",
    "text": { "body": "On my way" },
    "kapso": {
      "direction": "outbound",
      "status": "delivered",
      "processing_status": "completed",
      "origin": "cloud_api",
      "has_media": false,
      "statuses": [
        {
          "id": "wamid.456",
          "status": "sent",
          "timestamp": "1730092860",
          "recipient_id": "15551234567"
        },
        {
          "id": "wamid.456",
          "status": "delivered",
          "timestamp": "1730092888",
          "recipient_id": "15551234567"
        }
      ]
    }
  },
  "conversation": {
    "id": "conv_123",
    "phone_number": "+15551234567",
    "status": "active",
    "last_active_at": "2025-10-28T14:31:28Z",
    "created_at": "2025-10-28T13:40:00Z",
    "updated_at": "2025-10-28T14:31:28Z",
    "metadata": {},
    "phone_number_id": "123456789012345"
  },
  "is_new_conversation": false,
  "phone_number_id": "123456789012345"
}
```

### whatsapp.message.failed

```json
{
  "message": {
    "id": "wamid.789",
    "timestamp": "1730093200",
    "type": "text",
    "text": { "body": "This message failed" },
    "kapso": {
      "direction": "outbound",
      "status": "failed",
      "processing_status": "completed",
      "origin": "cloud_api",
      "has_media": false,
      "statuses": [
        {
          "id": "wamid.789",
          "status": "sent",
          "timestamp": "1730093100",
          "recipient_id": "15551234567"
        },
        {
          "id": "wamid.789",
          "status": "failed",
          "timestamp": "1730093200",
          "recipient_id": "15551234567",
          "errors": [
            {
              "code": 131047,
              "title": "Re-engagement message",
              "message": "More than 24 hours have passed since the recipient last replied"
            }
          ]
        }
      ]
    }
  },
  "conversation": {
    "id": "conv_123",
    "phone_number": "+15551234567",
    "status": "active",
    "last_active_at": "2025-10-28T15:00:00Z",
    "created_at": "2025-10-28T13:40:00Z",
    "updated_at": "2025-10-28T15:00:00Z",
    "metadata": {},
    "phone_number_id": "123456789012345"
  },
  "is_new_conversation": false,
  "phone_number_id": "123456789012345"
}
```

### whatsapp.conversation.created

```json
{
  "conversation": {
    "id": "conv_789",
    "phone_number": "+15551234567",
    "status": "active",
    "last_active_at": "2025-10-28T14:00:00Z",
    "created_at": "2025-10-28T14:00:00Z",
    "updated_at": "2025-10-28T14:00:00Z",
    "metadata": {},
    "phone_number_id": "123456789012345",
    "kapso": {
      "contact_name": "John Doe",
      "messages_count": 0,
      "last_message_id": null,
      "last_message_type": null,
      "last_message_timestamp": null,
      "last_message_text": null,
      "last_inbound_at": null,
      "last_outbound_at": null
    }
  },
  "phone_number_id": "123456789012345"
}
```

### whatsapp.conversation.ended

```json
{
  "conversation": {
    "id": "conv_789",
    "phone_number": "+15551234567",
    "status": "ended",
    "last_active_at": "2025-10-28T15:10:45Z",
    "created_at": "2025-10-28T14:00:00Z",
    "updated_at": "2025-10-28T15:10:45Z",
    "metadata": {},
    "phone_number_id": "123456789012345",
    "kapso": {
      "contact_name": "John Doe",
      "messages_count": 15,
      "last_message_id": "wamid.999",
      "last_message_type": "text",
      "last_message_timestamp": "2025-10-28T15:10:45Z",
      "last_message_text": "Thanks!",
      "last_inbound_at": "2025-10-28T15:10:45Z",
      "last_outbound_at": "2025-10-28T15:10:30Z"
    }
  },
  "phone_number_id": "123456789012345"
}
```

### whatsapp.conversation.inactive

```json
{
  "conversation": {
    "id": "conv_789",
    "phone_number": "+15551234567",
    "status": "active",
    "last_active_at": "2025-10-28T13:00:00Z",
    "created_at": "2025-10-28T12:00:00Z",
    "updated_at": "2025-10-28T13:00:00Z",
    "metadata": {},
    "phone_number_id": "123456789012345"
  },
  "since_message": {
    "id": "msg_anchor",
    "whatsapp_message_id": "wamid.ANCHOR",
    "direction": "inbound",
    "created_at": "2025-10-28T13:00:00Z"
  },
  "inactivity": {
    "minutes": 60
  },
  "phone_number_id": "123456789012345"
}
```

### Multiple inactivity timeouts

Create separate webhooks for different timeout thresholds:

```json
// First webhook: 5 minute warning
{
  "events": ["whatsapp.conversation.inactive"],
  "inactive_after_minutes": 5
}

// Second webhook: 30 minute escalation
{
  "events": ["whatsapp.conversation.inactive"],
  "inactive_after_minutes": 30
}
```

Each webhook fires independently when its threshold is reached.

## Message origin

The `message.kapso.origin` field indicates how the message entered the system:

- **cloud_api** - Sent via Kapso API (outbound jobs, flow actions, API calls)
- **business_app** - Echoed from WhatsApp Business App (when using the Business App)
- **history_sync** - Backfilled during message history imports (only if project ran sync)

## Status history

The `message.kapso.statuses` array contains the complete history of raw Meta status events for a message, ordered chronologically. Each entry is the unmodified payload from Meta's webhook.

### Status object structure

Each status object in the array follows Meta's webhook format:

```json
{
  "id": "<WHATSAPP_MESSAGE_ID>",
  "status": "<STATUS>",
  "timestamp": "<UNIX_TIMESTAMP>",
  "recipient_id": "<PHONE_NUMBER>",
  "pricing": {
    "billable": true,
    "pricing_model": "<PRICING_MODEL>",
    "category": "<PRICING_CATEGORY>"
  },
  "errors": [
    {
      "code": 131031,
      "title": "<ERROR_TITLE>",
      "message": "<ERROR_MESSAGE>",
      "error_data": {
        "details": "<ERROR_DETAILS>"
      },
      "href": "<ERROR_CODES_URL>"
    }
  ],
  ...
}
```

| Field | Included when |
|-------|---------------|
| `pricing` | Sent status, plus delivered or read |
| `errors` | Failed to send or deliver |

See [Meta's status webhook reference](https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/components#statuses-object) for the complete schema.

Use this field to track the full lifecycle of outbound messages and understand failure causes. The array only appears when status events have been recorded.

## Message types

The `message.type` field can be one of:

- `text` - Plain text message
- `image` - Image attachment
- `video` - Video attachment
- `audio` - Audio/voice message
- `document` - Document attachment
- `location` - Location sharing
- `template` - WhatsApp template message
- `interactive` - Interactive message (buttons, lists)
- `reaction` - Message reaction
- `contacts` - Contact card sharing

## Message type-specific data

### Media messages (image/video/document)

```json
{
  "message": {
    "id": "wamid.789",
    "timestamp": "1730093000",
    "type": "image",
    "image": {
      "caption": "Photo description",
      "id": "media_id_123"
    },
    "kapso": {
      "direction": "inbound",
      "status": "received",
      "processing_status": "pending",
      "origin": "cloud_api",
      "has_media": true,
      "content": "Photo description Image attached (photo.jpg) [Size: 200 KB | Type: image/jpeg] URL: https://api.kapso.ai/media/...",
      "media_url": "https://api.kapso.ai/media/...",
      "media_data": {
        "url": "https://api.kapso.ai/media/...",
        "filename": "photo.jpg",
        "content_type": "image/jpeg",
        "byte_size": 204800
      },
      "message_type_data": {
        "caption": "Photo description"
      }
    }
  }
}
```

### Audio messages

```json
{
  "message": {
    "id": "wamid.790",
    "timestamp": "1730093100",
    "type": "audio",
    "audio": {
      "id": "media_id_456"
    },
    "kapso": {
      "direction": "inbound",
      "status": "received",
      "processing_status": "pending",
      "origin": "cloud_api",
      "has_media": true,
      "content": "[Audio attached] (voice.ogg) [Size: 50 KB | Type: audio/ogg] URL: https://api.kapso.ai/media/...\nTranscript: Hello, I need help with my order",
      "transcript": {
        "text": "Hello, I need help with my order"
      },
      "media_url": "https://api.kapso.ai/media/...",
      "media_data": {
        "url": "https://api.kapso.ai/media/...",
        "filename": "voice.ogg",
        "content_type": "audio/ogg",
        "byte_size": 51200
      }
    }
  }
}
```

### Location messages

```json
{
  "message": {
    "type": "location",
    "location": {
      "latitude": 37.7749,
      "longitude": -122.4194,
      "name": "San Francisco",
      "address": "San Francisco, CA, USA"
    }
  }
}
```

### Template messages

```json
{
  "message": {
    "type": "template",
    "template": {
      "name": "order_confirmation",
      "language": {
        "code": "en_US"
      },
      "components": [...]
    }
  }
}
```

### Interactive messages

```json
{
  "message": {
    "type": "interactive",
    "interactive": {
      "type": "button_reply",
      "button_reply": {
        "id": "btn_1",
        "title": "Confirm"
      }
    }
  }
}
```

### Reaction messages

```json
{
  "message": {
    "type": "reaction",
    "reaction": {
      "message_id": "wamid.HBgNNTU0MTIzNDU2Nzg5MA",
      "emoji": "👍"
    }
  }
}
```
