# WhatsApp Business Platform: Flow JSON Reference

## IMPORTANT: Version Requirements

**Always use Flow JSON version 7.3** (the current recommended version). Earlier versions like 5.0 are no longer supported for publishing flows.

```json
{
  "version": "7.3",
  "data_api_version": "3.0",
  ...
}
```

---

Flow JSON enables businesses to create workflows in WhatsApp by accessing the features of WhatsApp Flows using a custom JSON object developed by Meta. These workflows are initiated, run, and managed entirely inside WhatsApp. They can include multiple screens, data flows, and response messages.

To visualize the complete user experience, use the **Builder** in the WhatsApp Manager (Account tools > Flows), which emulates the entire Flow experience.

## Structure of Flow JSON

Flow JSON consists of the following sections:

| Section               | Description                                                                                                                              |
| :-------------------- | :--------------------------------------------------------------------------------------------------------------------------------------- |
| **Screen Data Model** | Commands to define static types that power the screen.                                                                                   |
| **Screens**           | Used to compose layouts using standard UI library components.                                                                            |
| **Components**        | Individual building blocks that make up a screen (text fields, buttons, etc.).                                                           |
| **Routing Model**     | Defines the rules for screen transitions (e.g., Screen 1 can only go to Screen 2). Used for validation.                                  |
| **Actions**           | Syntax to invoke client-side logic. Allowed actions: `navigate`, `data_exchange`, `complete`, `open_url` (v6.0+), `update_data` (v6.0+). |

## Data Exchange quick spec (Kapso Functions)
- WhatsApp sends (POST): `action` (`INIT`/`data_exchange`/`BACK`), `screen`, `data` (fields), `flow_token`, optional `flow_token_signature` (JWT with Meta app secret). Expect response in ~10–15s.
- Kapso forwards to your Function as JSON: `{ source: "whatsapp_flow", flow: { id, meta_flow_id }, data_exchange: <original payload>, signature_valid: true|false, received_at: "<iso8601>" }`.
- Your Function must return JSON:
  - Next screen: `{ "screen": "NEXT_SCREEN", "data": { ... } }`
  - Validation on same screen: `{ "screen": "CURRENT", "data": { "error_message": "Try again" } }`
  - Completion: `{ "screen": "SUCCESS", "data": { "extension_message_response": { "params": { ... } } } }`
  - Optional: include `vars` to persist variables; `next_edge` is not required for WhatsApp flows.
- Do NOT include `endpoint_uri` / `data_channel_uri` (Kapso injects). Do NOT wrap JSON in markdown fences.
- Runtime tips: keep total work under ~12s, minimal external calls, respond with `Content-Type: application/json`.

### Common gotchas (read me first)
- **Dynamic property rule:** `${form.*}` and `${data.*}` must be the entire property value. `"Hello ${data.name}"` is invalid unless you use a backticked nested expression (v6.0+): ``"text": "`'Hello ' ${data.name}`"``.
- **Schema as whitelist:** On dynamic screens, declare **every** field your endpoint may return under `screen.data`. Undeclared fields may be dropped by Meta/Kapso even if your endpoint returns them.
- **Custom payload keys:** Extra keys in action payloads are forwarded as-is to `data_exchange.data`. Avoid reserved names like `action`, `screen`, `data`; prefer explicit names such as `user_action`, `step`, `intent`.
- **Version is mandatory:** Every endpoint response must include `"version": "3.0"` or the UI can appear to return empty data (silent failure).
- **Error responses need full data:** When you stay on the same screen with `error_message`, still include all data the screen expects (lists, init-values, etc.) or components can break.
- **Carry data forward:** Global references (`${screen.SCREEN.form.field}`) exist, but the safest pattern is to re-send needed values in each response: return them in `data` for the next screen and reference via `${data.field}`.
- **Routing pattern:** Route primarily on `data_exchange.action` then `data_exchange.screen`; see the example in “Action Routing Pattern” below for a multi-screen skeleton.

### Mini handler examples
Cloudflare Worker:
```javascript
async function handler(request) {
  const body = await request.json();
  const screen = body.data_exchange?.screen || "WELCOME";
  return new Response(JSON.stringify({
    screen: "SUCCESS",
    data: { message: `Thanks from ${screen}` }
  }), { headers: { "Content-Type": "application/json" } });
}
```

## Data Endpoint Quick Start (Kapso Functions)

This section is the **minimum you must do** to make a WhatsApp Flow endpoint work reliably with Kapso Functions.

### Enabling flows encryption
- Endpoint operations (create/update) require Flows encryption to be enabled.
- The Kapso Agent can enable this for you automatically when needed.
- If the agent can’t enable it (for example, no phone number selected) or asks you to do it manually:
  - In Kapso, click the **Settings** gear in the top-right toolbar.
  - Open the WhatsApp/Phone configuration section.
  - Toggle **Enable encryption** for the phone number / WhatsApp Business Account.

### 1. Response Contract (Critical)

Every response from your Function **must** include all three fields:

```json
{
  "version": "3.0",
  "screen": "NEXT_SCREEN_ID",
  "data": { }
}
```

- **CRITICAL:** if `version` is missing or incorrect, the flow can appear to
  succeed at the HTTP layer but behave as if no data was returned (silent
  failure / empty data in the UI).

- `version`
  - **Required** for all `data_exchange` responses.
  - Use `"3.0"` unless Meta changes the `data_api_version`.
  - If you omit or mis-set this, the flow may **fail silently** and appear to return empty data.
- `screen`
  - The screen to render next. Must exist in the Flow’s `routing_model`.
- `data`
  - Object with properties consumed by your Flow JSON (lists, error messages, etc.).

Examples:

```json
{
  "version": "3.0",
  "screen": "APPOINTMENT_SLOTS",
  "data": {
    "available_slots": [
      { "id": "slot-1", "title": "Today 10:00" },
      { "id": "slot-2", "title": "Today 11:00" }
    ]
  }
}
```

```json
{
  "version": "3.0",
  "screen": "APPOINTMENT_SLOTS",
  "data": {
    "error_message": "Please select a valid time",
    "available_slots": [
      { "id": "slot-1", "title": "Today 10:00" },
      { "id": "slot-2", "title": "Today 11:00" }
    ]
  }
}
```

```json
{
  "version": "3.0",
  "screen": "SUCCESS",
  "data": {
    "extension_message_response": {
      "params": {
        "flow_token": "<FLOW_TOKEN>",
        "summary": "Booked for 2025-01-10 at 10:00"
      }
    }
  }
}
```

### 2. Request Payload Map

Kapso decrypts Meta’s encrypted payload and forwards the following JSON to your Function:

```json
{
  "source": "whatsapp_flow",
  "flow": {
    "id": "<kapso_flow_id>",
    "meta_flow_id": "<meta_flow_id>"
  },
  "data_exchange": {
    "version": "3.0",
    "action": "INIT",
    "screen": "CURRENT_SCREEN_ID",
    "data": {},
    "flow_token": "<FLOW_TOKEN>",
    "flow_token_signature": "<JWT>"
  },
  "signature_valid": true,
  "received_at": "2025-01-01T12:34:56Z"
}
```

In your handler:

- User inputs: `body.data_exchange.data.<field_name>`
- Current screen: `body.data_exchange.screen`
- Flow action: `body.data_exchange.action` (`"INIT"`, `"data_exchange"`, `"BACK"`)
- Flow token: `body.data_exchange.flow_token`
- Signature: `body.data_exchange.flow_token_signature` + `body.signature_valid` if you want to enforce it.

You **do not** need to implement encryption/decryption yourself; Kapso already does that before calling your Function.

Note: in invocation logs we store a simplified shape with a top-level
`flow_id` for convenience. The JSON sent to your Function uses the `flow`
object as shown above.

### 3. Action Routing Pattern

A simple decision tree that works for most flows:

- **Best practice:** route primarily by `data_exchange.action` and
  `data_exchange.screen`. Use custom fields inside `data` only for secondary
  branching, not as your main router.

```ts
const de = body.data_exchange;
const action = de.action;
const screen = de.screen;
const data = de.data || {};

if (action === "INIT") {
  // First time the flow calls your endpoint
  return {
    version: "3.0",
    screen: "FIRST_SCREEN",
    data: {}
  };
}

if (action === "BACK") {
  // User pressed back; decide whether to re-load or reuse previous data
  return {
    version: "3.0",
    screen: screen || "FIRST_SCREEN",
    data: {}
  };
}

if (action === "data_exchange") {
  if (screen === "SCREEN_A") {
    // Process Screen A and send Screen B
    return {
      version: "3.0",
      screen: "SCREEN_B",
      data
    };
  }

  if (screen === "SCREEN_B") {
    // Maybe complete the flow
    return {
      version: "3.0",
      screen: "SUCCESS",
      data: {
        extension_message_response: {
          params: {
            flow_token: de.flow_token
          }
        }
      }
    };
  }
}
```

End-to-end multi-screen skeleton (INIT → SCREEN_A → SCREEN_B → SUCCESS):

```ts
if (action === "INIT") {
  return { version: "3.0", screen: "SCREEN_A", data: {} };
}

if (action === "data_exchange") {
  if (screen === "SCREEN_A") {
    // validate, fetch data
    return { version: "3.0", screen: "SCREEN_B", data: { slots, user_email: data.email } };
  }

  if (screen === "SCREEN_B") {
    // final submission
    return {
      version: "3.0",
      screen: "SUCCESS",
      data: {
        extension_message_response: {
          params: { flow_token: de.flow_token }
        }
      }
    };
  }
}
```

You can also use a custom field inside `data` (for example, `data.action`) to branch when multiple buttons on the same screen go to different places.

When in doubt, start with a minimal pattern:

- On `INIT`, return your first screen with any initial data.
- On `data_exchange` from that screen, validate inputs and either:
  - stay on the same screen with `error_message`, or
  - move to the next screen with derived data.
- On `data_exchange` from the final screen, return `SUCCESS` with an
  `extension_message_response` payload to complete the flow.

### 4. Error Handling Rules

Follow Meta’s rules, but in practice:

- Always return HTTP `200` from your Function (Kapso handles HTTP errors upstream).
- Never rely on 4xx/5xx to signal validation errors to the user.
- Use `data.error_message` with the same `screen` to show a snackbar and keep the user on the current screen.
- When returning an error, still include all data fields the screen expects (lists, init values, etc.) so components continue to render.
- Keep payloads small; avoid huge objects or unnecessary fields.

Example (validation failure):

```json
{
  "version": "3.0",
  "screen": "APPOINTMENT_SLOTS",
  "data": {
    "error_message": "That time is no longer available. Please pick another.",
    "available_slots": []
  }
}
```

### 5. Component Data Contract (Dynamic Options)

When a component uses a data source from the endpoint, the field name and shape must match exactly.

Example – dropdown with dynamic options:

```json
{
  "type": "Dropdown",
  "name": "slot_id",
  "data-source": "${data.available_slots}"
}
```

Your endpoint must return:

```json
{
  "version": "3.0",
  "screen": "APPOINTMENT_SLOTS",
  "data": {
    "available_slots": [
      { "id": "slot-1", "title": "Today 10:00" },
      { "id": "slot-2", "title": "Today 11:00" }
    ]
  }
}
```

- Field name in `data` → `available_slots`
- Each item → `{ "id": string, "title": string }`

### 6. Debugging & Logging Checklist

When debugging a Flow + Endpoint integration:

1. Log the raw forwarded body (from Kapso to your Function).
2. Log:
   - `data_exchange.action`
   - `data_exchange.screen`
   - `data_exchange.data`
3. Log the external API request/response (status, body shape).
4. Log the final response you return to Kapso (with `version`, `screen`, `data`).
5. If the UI shows an empty screen or no data:
   - Check `version` is `"3.0"` in responses.
   - Check `screen` exists in `routing_model`.
   - Check `data` field names match the Flow JSON.
   - Check array/object shape (especially for lists).

---

## Top-level Flow JSON Properties

### Required Properties
*   **`version`**: Represents the version of Flow JSON to use during compilation.
*   **`screens`**: An array representing the different pages (nodes) of the user experience.

### Optional Properties
*   **`routing_model`**: Represents the routing system. Generated automatically if the Flow does not use a Data Endpoint. Required if using an endpoint.
*   **`data_api_version`**: The version used for communication with the WhatsApp Flows Data Endpoint (currently `3.0`).
*   **`data_channel_uri`**: **Note:** Not supported as of Flow JSON 3.0. For v3.0+, configure the URL using the `endpoint_uri` field provided by the Flows API.

> **Version fields cheat sheet**
>
> - Flow JSON `version`: which Flow JSON schema version your flow uses (for example, `"3.1"`).
> - `data_api_version`: protocol version for the data endpoint (currently `"3.0"`).
> - Data-exchange payload `version`: the value Meta sends in `data_exchange.version` (should match `data_api_version`).
> - Endpoint response `version`: you must echo `"3.0"` in every response your Function returns. If you omit it, Meta may treat the payload as invalid and your flow can appear to return empty data.

**Example (Version 2.1 - Legacy)**
```json
{
 "version": "2.1",
 "data_api_version": "3.0",
 "routing_model": {"MY_FIRST_SCREEN": ["MY_SECOND_SCREEN"] },
 "screens": [...],
 "data_channel_uri": "https://example.com"
}
```

**Example (Version 3.1 - Current)**
```json
{
 "version": "3.1",
 "data_api_version": "3.0",
 "routing_model": {"MY_FIRST_SCREEN": ["MY_SECOND_SCREEN"] },
 "screens": [...]
}
```

---

## Screens

Screens are the main unit of a Flow. Each screen represents a single node in the state machine.

**Schema:**
```json
"screen" : {
  "id": "string",
  "terminal": boolean,
  "success": boolean,
  "title": "string",
  "refresh_on_back": boolean,
  "data": object,
  "layout": object,
  "sensitive": []
}
```

### Properties
*   **`id`** (Required): Unique identifier. `SUCCESS` is a reserved keyword and cannot be used.
*   **`layout`** (Required): The UI Layout. Can be predefined or a container with custom content using the WhatsApp Flows Library.
*   **`terminal`** (Optional): If `true`, this screen ends the flow. A Footer component is mandatory on terminal screens.
*   **`title`** (Optional): Attribute rendered in the top navigation bar.
*   **`success`** (Optional, terminal only): Defaults to `true`. Marks whether terminating on this screen is a successful business outcome.
*   **`data`** (Optional): Declaration of dynamic data that fills the components using JSON Schema.
    *   **Dynamic screens:** Declare **every field** your endpoint will return for this screen. Undeclared fields may be dropped and unavailable for binding even if the endpoint returns them.
    ```json
    {
      "data": {
        "first_name": {
          "type": "string",
          "__example__": "John"
        }
      }
    }
    ```
*   **`refresh_on_back`** (Optional, Data Endpoint only): Defaults to `false`. Defines whether to trigger a data exchange request when using the back button to return to this screen.
    *   **`false`**: Screen loads with previously provided data/input. Avoids roundtrip; snappier experience.
    *   **`true`**: Sends a request to the Endpoint (Action: `BACK`, Screen: Name of previous screen) to revalidate/update data.
*   **`sensitive`** (Optional, v5.1+): Defaults to `[]`. Array of field names containing sensitive data. When the Flow completes, the summary message will mask these fields based on the component type. Each field name must correspond to an input component (with matching `name` attribute) that exists on the same screen. Do not include fields from other screens. Review/summary screens that only display text (no input components) should not have a `sensitive` array.

### Sensitive Fields Masking Behavior
| Component           | Masking | Consumer Experience |
| :------------------ | :------ | :------------------ |
| Text Input          | ✅       | Masked (••••)       |
| Password / OTP      | ❌       | Hidden completely   |
| Text Area           | ✅       | Masked (••••)       |
| Date Picker         | ✅       | Masked (••••)       |
| Dropdown            | ✅       | Masked (••••)       |
| Checkbox Group      | ✅       | Masked (••••)       |
| Radio Buttons Group | ✅       | Masked (••••)       |
| Opt In              | ❌       | Display as-is       |
| Document Picker     | ✅       | Hidden completely   |
| Photo Picker        | ✅       | Hidden completely   |

---

## Layout

Layout represents screen UI Content.
*   **`type`**: Currently, only `"SingleColumnLayout"` (vertical flexbox container) is available.
*   **`children`**: An array of components from the WhatsApp Flows Library.

---

## Routing Model

Required only when using an Endpoint. It is a directed graph where screens are nodes and transitions are edges.

*   **Structure**: A map of screen IDs to an array of possible next screen IDs.
*   **Limits**: Maximum 10 "branches" or connections.
*   **Entry Screen**: A screen with no inbound edge.
*   **Terminal**: All routes must eventually end at a terminal screen.

**Example:**
```json
"routing_model": {
  "MY_FIRST_SCREEN": ["MY_SECOND_SCREEN"],
  "MY_SECOND_SCREEN": ["MY_THIRD_SCREEN"]
}
```

---

## Properties (Static vs. Dynamic)

### Static Properties
Set once and never change.
```json
"title": "Demo Screen"
```

### Dynamic Properties
Set content based on server or user data.
*   **Form properties**: `${form.field_name}` (Data entered by user).
*   **Screen properties**: `${data.field_name}` (Data provided by server or previous screen).

Supported types: `string`, `number`, `boolean`, `object`, `array`.

### Nested Expressions (v6.0+)
Allows conditionals and string concatenation. Must be wrapped in backticks (`` ` ``).
*   **Equality**: `==`, `!=`
*   **Math Comparisons**: `<`, `<=`, `>`, `>=`
*   **Logical**: `&&`, `||`
*   **Math Operations**: `+`, `-`, `/`, `%` (numbers only, not for strings)
*   **Escaping**: To use backticks inside a string, use double backslash: `\\` before it.

**Examples:**
*   **String Concatenation**: ``"text": "`'Hello ' ${form.first_name}`"`` (no `+` operator, just place strings next to each other)
*   **Multiple strings**: ``"text": "`${data.street} ', ' ${data.city} ', ' ${data.country}`"``
*   **Math**: ``"text": "`'Amount: ' ${data.total} / ${form.group_size}`"``
*   **Logic**: ``"visible": "`(${form.age} > 18) && ${form.accept}`"``
*   **Equality**: ``"visible": "`${form.first_name} == ${form.last_name}`"``

**Important:** Outside backticked nested expressions, a property cannot mix static text with `${...}`. Use either a pure dynamic value (e.g., `"text": "${data.name}"`) or the backtick form above for string concatenation.

---

## Forms and Data Handling

### Forms (Legacy vs v4.0+)
*   **Before v4.0**: Inputs had to be wrapped in a `"type": "Form"` component.
*   **v4.0+**: The `Form` component is optional. You can place interactive components directly in the layout.

### Form Configuration
*   **`init-values`**: Key-value object to pre-fill inputs. Types must match the component (e.g., Array of Strings for CheckboxGroup, String for TextInput).
*   **`error-messages`**: Key-value object to set custom errors.

**Example (v4.0+ No Form Component):**
```json
{
  "id": "DEMO_SCREEN",
  "data": {
     "init_values": { "first_name": "Jon" }
  },
  "layout": {
      "type": "SingleColumnLayout",
      "children": [
         {
             "type": "TextInput",
             "name": "first_name",
             "label": "First Name",
             "init-value": "${data.init_values.first_name}"
         }
      ]
  }
}
```

### Global Dynamic Referencing (v4.0+)
Allows accessing data from any screen globally.
**Syntax:** `${screen.<screen_name>.(form|data).<field-name>}`

*   **`screen`**: Global variable keyword.
*   **`screen_name`**: ID of the screen to reference.
*   **`(form|data)`**: Storage type.
*   **`field-name`**: Specific field.

**Use Case:** You no longer need to pass payloads explicitly in the `navigate` action to carry data forward.
**Practical tip:** For reliability, especially across different Flow JSON versions, also include needed values in the `data` you return for the next screen (e.g., echo `user_email` and `selected_date` forward) so components can bind via `${data.*}` even if global references are unavailable.

---

## Actions

Actions trigger asynchronous logic.

### 1. `navigate`
Transitions to the next screen.
*   **Payload**: Required, even if empty `{}`. Data passed here is available in the next screen via `${data.field}`.
*   **Note**: Do not use on the footer of a terminal screen.

```json
"on-click-action": {
  "name": "navigate",
  "next": { "type": "screen", "name": "NEXT_SCREEN" },
  "payload": {
    "name": "${form.first_name}"
  }
}
```

With no data to pass:
```json
"on-click-action": {
  "name": "navigate",
  "next": { "type": "screen", "name": "NEXT_SCREEN" },
  "payload": {}
}
```

### 2. `complete`
Terminates the flow and sends the payload to the chat thread via webhook.
*   **Recommendation**: Keep payload size minimum. Avoid base64 images.

```json
"on-click-action": {
  "name": "complete",
  "payload": {
    "selected_items": "${form.selected_items}"
  }
}
```

### 3. `data_exchange`
Sends data to the WhatsApp Flows Data Endpoint. The server response determines the next step.
*   Payload keys are forwarded as-is to your endpoint. Avoid generic names like `action` that may collide with Meta fields; prefer explicit keys such as `user_action`, `step`, `intent`.

```json
"on-click-action": {
  "name": "data_exchange",
  "payload": {
    "discount_code": "${data.discount_code}"
  }
}
```

### 4. `update_data` (v6.0+)
Triggers an immediate update to the screen's state based on user interaction (e.g., changing a dropdown list based on a previous selection) without a full screen refresh.

```json
"on-click-action": {
  "name": "update_data",
  "payload": {
    "state_list": "${data.countries[form.selected_country].states}"
  }
}
```

### 5. `open_url` (v6.0+)
Opens a URL in the device's default browser.
*   **Supported Components**: `EmbeddedLink` and `OptIn`.
*   **Payload**: Accepts only a `url` property.

```json
{
   "type": "EmbeddedLink",
   "text": "Terms and Conditions",
   "on-click-action": {
      "name": "open_url",
      "url": "https://www.whatsapp.com/"
   }
}
```

---

## Limitations

*   **File Size**: Flow JSON content string cannot exceed **10 MB**.
*   


# Flow JSON Components

Components are the building blocks of WhatsApp Flows. They allow you to build complex UIs and display business data using attribute models.

*   **Limit:** The maximum number of components per screen is **50**.

**Supported Components:**
*   **Text:** Heading, Subheading, Caption, Body, RichText
*   **Input:** TextEntry (TextInput, TextArea), DatePicker, CalendarPicker, Media upload
*   **Selection:** CheckboxGroup, RadioButtonsGroup, Dropdown, Chips Selector, Switch
*   **Display/Media:** Image, Image Carousel
*   **Navigation/Action:** Footer, OptIn, EmbeddedLink, NavigationList
*   **Logic:** If

---

## Text Components

### Heading
Top level title of a page.
| Parameter | Description                                 |
| :-------- | :------------------------------------------ |
| `type`    | (Required) `"TextHeading"`                  |
| `text`    | (Required) String or Dynamic `${data.text}` |
| `visible` | Boolean. Default: `true`.                   |

### Subheading
| Parameter | Description                                 |
| :-------- | :------------------------------------------ |
| `type`    | (Required) `"TextSubheading"`               |
| `text`    | (Required) String or Dynamic `${data.text}` |
| `visible` | Boolean. Default: `true`.                   |

### Body
| Parameter       | Description                                           |
| :-------------- | :---------------------------------------------------- |
| `type`          | (Required) `"TextBody"`                               |
| `text`          | (Required) String or Dynamic `${data.text}`           |
| `font-weight`   | Enum: `{'bold','italic','bold_italic','normal'}`      |
| `strikethrough` | Boolean                                               |
| `visible`       | Boolean. Default: `true`.                             |
| `markdown`      | Boolean. Default: `false`. (Requires Flow JSON V5.1+) |

### Caption
| Parameter       | Description                                           |
| :-------------- | :---------------------------------------------------- |
| `type`          | (Required) `"TextCaption"`                            |
| `text`          | (Required) String or Dynamic `${data.text}`           |
| `font-weight`   | Enum: `{'bold','italic','bold_italic','normal'}`      |
| `strikethrough` | Boolean                                               |
| `visible`       | Boolean. Default: `true`.                             |
| `markdown`      | Boolean. Default: `false`. (Requires Flow JSON V5.1+) |

### Markdown Support (v5.1+)
`TextBody` and `TextCaption` support limited markdown if `markdown: true` is set.
```json
{
   "type": "TextBody",
   "markdown": true,
   "text": [ "This text is ~~***really important***~~" ]
}
```

### Limits and Restrictions
| Component  | Property        | Limit / Restriction                  |
| :--------- | :-------------- | :----------------------------------- |
| Heading    | Character Limit | 80                                   |
| Subheading | Character Limit | 80                                   |
| Body       | Character Limit | 4096                                 |
| Caption    | Character Limit | 409                                  |
| All        | Text            | Empty or Blank value is not accepted |

---

## RichText (v5.1+)
Provides rich formatting capabilities and rendering for large texts (Terms, Policies, etc.).

| Parameter | Description                                               |
| :-------- | :-------------------------------------------------------- |
| `type`    | (Required) `"RichText"`                                   |
| `text`    | (Required) String or String Array. Dynamic `${data.text}` |
| `visible` | Boolean. Default: `true`.                                 |

**Note:**
*   **Until V6.2:** Must be a standalone component (cannot share screen with others).
*   **Starting V6.3:** Can be used with a `Footer` component.

### Supported Syntax
*   **Headings:** `# Heading 1`, `## Heading 2` (Others render as body text).
*   **Formatting:** `**bold**`, `*italic*`, `~~strikethrough~~`.
*   **Lists:** Ordered (`1. Item`) and Unordered (`- Item` or `+ Item`).
*   **Images:** Base64 inline only. `![Alt](data:image/png;base64,...)`.
    *   *Formats:* png, jpg/jpeg, webp (iOS 14.6+).
*   **Links:** `[Text](URL)`
*   **Tables:** Standard Markdown syntax. Columns widths based on header content size.

### Syntax Cheatsheet
| Syntax                               | RichText | TextBody | TextCaption |
| :----------------------------------- | :------- | :------- | :---------- |
| `# H1`, `## H2`                      | ✅        | ❌        | ❌           |
| `**bold**`, `*italic*`, `~~strike~~` | ✅        | ✅        | ✅           |
| Lists (`+`, `-`, `1.`)               | ✅        | ✅        | ✅           |
| `[Link](url)`                        | ✅        | ✅        | ✅           |
| `![Image](base64)`                   | ✅        | ❌        | ❌           |
| Tables                               | ✅        | ❌        | ❌           |

---

## Text Entry Components

### TextInput
| Parameter       | Description                                                        |
| :-------------- | :----------------------------------------------------------------- |
| `type`          | (Required) `"TextInput"`                                           |
| `name`          | (Required) String                                                  |
| `label`         | (Required) String.                                                 |
| `label-variant` | `large` (v7.0+) - Prominent style, multi-line support.             |
| `input-type`    | Enum: `{'text','number','email', 'password', 'passcode', 'phone'}` |
| `pattern`       | (v6.2+) Regex string. Requires `helper-text`.                      |
| `required`      | Boolean                                                            |
| `min-chars`     | Integer                                                            |
| `max-chars`     | Integer. Default: 80.                                              |
| `helper-text`   | String                                                             |
| `visible`       | Boolean. Default: `true`.                                          |
| `init-value`    | (v4.0+) String. Only outside `Form`.                               |
| `error-message` | (v4.0+) String. Only outside `Form`.                               |

### TextArea
| Parameter       | Description                          |
| :-------------- | :----------------------------------- |
| `type`          | (Required) `"TextArea"`              |
| `name`          | (Required) String                    |
| `label`         | (Required) String.                   |
| `label-variant` | `large` (v7.0+)                      |
| `max-length`    | Integer. Default: 600.               |
| `helper-text`   | String                               |
| `enabled`       | Boolean                              |
| `visible`       | Boolean. Default: `true`.            |
| `init-value`    | (v4.0+) String. Only outside `Form`. |
| `error-message` | (v4.0+) String. Only outside `Form`. |

### Limits
| Component | Property           | Limit              |
| :-------- | :----------------- | :----------------- |
| TextInput | Helper/Error/Label | 80 / 30 / 20 chars |
| TextArea  | Helper/Label       | 80 / 20 chars      |

---

## Selection Components

### CheckboxGroup & RadioButtonsGroup
**CheckboxGroup:** Pick multiple. **RadioButtonsGroup:** Pick one.

| Parameter            | Description                                         |
| :------------------- | :-------------------------------------------------- |
| `type`               | `"CheckboxGroup"` or `"RadioButtonsGroup"`          |
| `name`               | (Required) String                                   |
| `data-source`        | (Required) Array of objects (See structure below)   |
| `label`              | String (Required v4.0+)                             |
| `min-selected-items` | Integer (Checkbox only)                             |
| `max-selected-items` | Integer (Checkbox only)                             |
| `enabled`            | Boolean                                             |
| `required`           | Boolean                                             |
| `visible`            | Boolean                                             |
| `on-select-action`   | Action (`data_exchange`, `update_data` v6.0+)       |
| `on-unselect-action` | (v6.0+) Action (`update_data` only)                 |
| `init-value`         | Array<String> (Checkbox) or String (Radio). (v4.0+) |
| `media-size`         | (v5.0+) Enum: `{'regular', 'large'}`                |

**Data Source Structure:**
*   **< v5.0:** `id`, `title`, `description`, `metadata`, `enabled`.
*   **> v5.0:** Adds `image` (Base64), `alt-text`, `color` (hex).
*   **> v6.0:** Adds `on-select-action`, `on-unselect-action`.

**Limits:**
*   **Items:** Min 1, Max 20.
*   **Image Size:** 100KB (v6.0+), 300KB (<v6.0).
*   **Text:** Title (30), Desc (30), Meta (300).

### Dropdown
| Parameter            | Description                                         |
| :------------------- | :-------------------------------------------------- |
| `type`               | (Required) `"Dropdown"`                             |
| `label`              | (Required) String                                   |
| `data-source`        | (Required) Array (Same structure as Checkbox/Radio) |
| `required`           | Boolean                                             |
| `enabled`            | Boolean                                             |
| `visible`            | Boolean                                             |
| `on-select-action`   | Action (`data_exchange`, `update_data`)             |
| `on-unselect-action` | (v6.0+) Action (`update_data` only)                 |
| `init-value`         | String (v4.0+)                                      |

**Limits:**
*   **Items:** Max 200 (no images), Max 100 (with images).
*   **Text:** Label (20), Title (30), Desc (300).

### Chips Selector (v6.3+)
Allows picking multiple selections.

| Parameter                | Description                                                               |
| :----------------------- | :------------------------------------------------------------------------ |
| `type`                   | `"ChipsSelector"`                                                         |
| `name`                   | (Required) String                                                         |
| `data-source`            | Array: `id`, `title`, `enabled`, `on-select-action`, `on-unselect-action` |
| `label`                  | (Required) String                                                         |
| `min/max-selected-items` | Integer                                                                   |
| `on-select-action`       | Action (`data_exchange`, `update_data` v7.1+)                             |
| `on-unselect-action`     | (v7.1+) Action (`update_data` only)                                       |

**Limits:** Min 2 options, Max 20 options. Label 80 chars.

### Switch (v4.0+)
Renders components based on a value match.

| Parameter | Description                                                      |
| :-------- | :--------------------------------------------------------------- |
| `type`    | `"Switch"`                                                       |
| `value`   | (Required) String variable to evaluate (e.g., `${data.animal}`)  |
| `cases`   | (Required) Map. Key = string value. Value = Array of Components. |

---

## Action & Navigation Components

### Footer
| Parameter         | Description       |
| :---------------- | :---------------- |
| `type`            | `"Footer"`        |
| `label`           | (Required) String |
| `on-click-action` | (Required) Action |
| `left-caption`    | String            |
| `center-caption`  | String            |
| `right-caption`   | String            |

*   **Rule:** Can set Left+Right OR Center, but not all 3.
*   **Limits:** Label (35 chars), Captions (15 chars). 1 Footer per screen.

### OptIn
| Parameter                   | Description                                                 |
| :-------------------------- | :---------------------------------------------------------- |
| `type`                      | `"OptIn"`                                                   |
| `label`                     | (Required) String                                           |
| `name`                      | (Required) String                                           |
| `on-click-action`           | Action (Triggers "Read more"). Supports `open_url` (v6.0+). |
| `on-select/unselect-action` | (v6.0+) `update_data` only.                                 |

*   **Limits:** 120 chars, Max 5 per screen.

### EmbeddedLink
| Parameter         | Description                                                       |
| :---------------- | :---------------------------------------------------------------- |
| `type`            | `"EmbeddedLink"`                                                  |
| `text`            | (Required) String                                                 |
| `on-click-action` | (Required) Action (`navigate`, `data_exchange`, `open_url` v6.0+) |

*   **Limits:** 25 chars, Max 2 per screen.

### NavigationList (v6.2+)
List of options to navigate between screens.

| Parameter         | Description                                                       |
| :---------------- | :---------------------------------------------------------------- |
| `type`            | `"NavigationList"`                                                |
| `list-items`      | (Required) Array.                                                 |
| `on-click-action` | `data_exchange` or `navigate`. Can be on component or item level. |

**Item Structure:**
*   `main-content` (Req): `title` (30 chars), `description` (20), `metadata` (80).
*   `start`: `image` (Base64, 100KB), `alt-text`.
*   `end`: `title` (10 chars), `description` (10), `metadata` (10).
*   `badge`: String (15 chars). Max 1 item with badge per list.
*   `tags`: Array<string> (Max 3).

**Restrictions:** Max 2 per screen. Cannot be on terminal screen. Cannot be combined with other components.

---

## Date & Time Components

### DatePicker
**Important (v5.0+):** Uses formatted date string `"YYYY-MM-DD"`. This decouples values from time zones.

| Parameter               | Description                                       |
| :---------------------- | :------------------------------------------------ |
| `type`                  | `"DatePicker"`                                    |
| `name`                  | (Required) String                                 |
| `label`                 | (Required) String                                 |
| `min-date` / `max-date` | String (Timestamp in ms). See *Guidelines* below. |
| `unavailable-dates`     | Array <Timestamp in ms>.                          |
| `on-select-action`      | Only `data_exchange`.                             |

**Guidelines (< v5.0):**
If business and user are in different time zones, timezone offsets can cause incorrect dates.
*   *Recommendation:* Use v5.0+ logic ("YYYY-MM-DD") or calculate offsets carefully using the user's timezone (collected via Dropdown).

### CalendarPicker (v6.1+)
Full calendar interface.

| Parameter             | Description                                                  |
| :-------------------- | :----------------------------------------------------------- |
| `type`                | `"CalendarPicker"`                                           |
| `mode`                | `"single"` (Default) or `"range"`.                           |
| `label`               | String. If range: `{"start-date": "...", "end-date": "..."}` |
| `min-date`/`max-date` | String `"YYYY-MM-DD"`.                                       |
| `include-days`        | Array<Enum> (Mon, Tue, etc.). Default: All.                  |
| `min-days`/`max-days` | Integer (Range mode only).                                   |
| `on-select-action`    | `data_exchange`. Payload: String (Single) or Object (Range). |

---

## Media Components

### Image
| Parameter          | Description                     |
| :----------------- | :------------------------------ |
| `type`             | `"Image"`                       |
| `src`              | (Required) Base64 string.       |
| `scale-type`       | `cover` or `contain` (Default). |
| `aspect-ratio`     | Number. Default 1.              |
| `width` / `height` | Integer.                        |

*   **Limits:** Max 3 per screen, 300kb (Rec), 1MB Payload limit. Format: JPEG, PNG.

### Image Carousel (v7.1+)
Slide through multiple images.

| Parameter      | Description                           |
| :------------- | :------------------------------------ |
| `type`         | `"ImageCarousel"`                     |
| `images`       | (Required) Array `{ src, alt-text }`. |
| `aspect-ratio` | `"4:3"` (Default) or `"16:9"`.        |
| `scale-type`   | `cover` or `contain` (Default).       |

*   **Limits:** Min 1, Max 3 images. Max 2 carousels per screen.

### PhotoPicker (v4.0+)
Allows users to upload photos from camera or gallery.

| Parameter              | Description                                                                 |
| :--------------------- | :-------------------------------------------------------------------------- |
| `type`                 | (Required) `"PhotoPicker"`                                                  |
| `name`                 | (Required) String. Must be unique on the screen.                            |
| `label`                | (Required) String. Max 80 chars. Dynamic supported.                         |
| `description`          | String. Max 300 chars. Dynamic supported.                                   |
| `photo-source`         | Enum: `"camera_gallery"` (default), `"camera"`, `"gallery"`                 |
| `max-file-size-kb`     | Integer (kibibytes). Default: 25600 (25 MiB). Range: [1, 25600]             |
| `min-uploaded-photos`  | Integer. Default: 0. Range: [0, 30]. Use this instead of `required`.        |
| `max-uploaded-photos`  | Integer. Default: 30. Range: [1, 30]                                        |
| `enabled`              | Boolean. Default: true                                                      |
| `visible`              | Boolean. Default: true                                                      |
| `error-message`        | String or Object for image-specific errors                                  |

**Important:**
- `required` property is NOT supported. Use `min-uploaded-photos: 1` for required.
- Only 1 PhotoPicker allowed per screen.
- Cannot use PhotoPicker and DocumentPicker on the same screen.
- Cannot be used in `navigate` action payload. Use `data_exchange` or `complete`.
- Must be top-level in action payload: `"media": "${form.photo_picker}"` (not nested).

### DocumentPicker (v4.0+)
Allows users to upload documents.

| Parameter                 | Description                                                              |
| :------------------------ | :----------------------------------------------------------------------- |
| `type`                    | (Required) `"DocumentPicker"`                                            |
| `name`                    | (Required) String. Must be unique on the screen.                         |
| `label`                   | (Required) String. Max 80 chars. Dynamic supported.                      |
| `description`             | String. Max 300 chars. Dynamic supported.                                |
| `max-file-size-kb`        | Integer (kibibytes). Default: 25600 (25 MiB). Range: [1, 25600]          |
| `min-uploaded-documents`  | Integer. Default: 0. Range: [0, 30]. Use this instead of `required`.     |
| `max-uploaded-documents`  | Integer. Default: 30. Range: [1, 30]                                     |
| `allowed-mime-types`      | Array of strings. See supported list below.                              |
| `enabled`                 | Boolean. Default: true                                                   |
| `visible`                 | Boolean. Default: true                                                   |
| `error-message`           | String or Object for document-specific errors                            |

**Supported MIME types:** `application/pdf`, `application/msword`, `application/vnd.openxmlformats-officedocument.wordprocessingml.document`, `application/vnd.ms-excel`, `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`, `application/vnd.ms-powerpoint`, `application/vnd.openxmlformats-officedocument.presentationml.presentation`, `application/zip`, `application/gzip`, `application/x-7z-compressed`, `text/plain`, `image/jpeg`, `image/png`, `image/gif`, `image/webp`, `image/heic`, `image/heif`, `image/avif`, `image/tiff`, `video/mp4`, `video/mpeg`

**Important:**
- `required` property is NOT supported. Use `min-uploaded-documents: 1` for required.
- Only 1 DocumentPicker allowed per screen.
- Cannot use PhotoPicker and DocumentPicker on the same screen.
- Cannot be used in `navigate` action payload. Use `data_exchange` or `complete`.
- Must be top-level in action payload: `"media": "${form.document_picker}"` (not nested).

### Media Upload Limits
- Max 10 files can be sent in response message.
- Max aggregated size: 100 MiB for response message attachments.
- Files stored in WhatsApp CDN for up to 20 days (encrypted).

---

## Logic Components

### If (v4.0+)
Conditional rendering.

| Parameter   | Description                           |
| :---------- | :------------------------------------ |
| `type`      | `"If"`                                |
| `condition` | (Required) Boolean expression string. |
| `then`      | (Required) Array of Components.       |
| `else`      | Array of Components.                  |

**Operators:** `==`, `!=`, `&&`, `||`, `!`, `>`, `>=`, `<`, `<=`, `()`.
**Rules:**
*   Condition must resolve to boolean.
*   Max nesting: 3 levels.
*   **Footer Rule:** If used inside `If`, it must exist in *both* `then` and `else` branches (or neither), and no footer can exist outside the `If`.

---

## Dynamic Components (Tutorial)

This pattern allows updating the UI (e.g., refreshing time slots) when a user interacts with a component (e.g., selects a date).

**Prerequisites:** Requires a Data Endpoint and `data_api_version: "3.0"`.

### Step 1: Define the Screen
Create a screen with a `DatePicker` and a `Dropdown`.

```json
{
  "version": "7.2",
  "data_api_version": "3.0",
  "routing_model": { "BOOKING": [] },
  "screens": [
    {
      "id": "BOOKING",
      "terminal": true,
      "title": "Booking appointment",
      "data": {
        "is_dropdown_visible": { "type": "boolean", "__example__": false },
        "available_slots": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": { "id": {"type": "string"}, "title": {"type": "string"} }
            },
            "__example__": []
        }
      },
      "layout": {
        "type": "SingleColumnLayout",
        "children": [
          {
            "type": "DatePicker",
            "name": "date",
            "label": "Select date",
            "on-select-action": {
                "name": "data_exchange",
                "payload": {
                    "date": "${form.date}",
                    "component_action": "update_date"
                }
            }
          },
          {
             "type": "Dropdown",
             "label": "Pick a slot",
             "name": "slot",
             "required": "${data.is_dropdown_visible}",
             "visible": "${data.is_dropdown_visible}",
             "data-source": "${data.available_slots}"
          },
          {
             "type": "Footer",
             "label": "Book",
             "on-click-action": { "name": "complete", "payload": {} }
          }
        ]
      }
    }
  ]
}
```

### Step 2: Server Response
When the user selects a date, the `on-select-action` triggers `data_exchange`. Your server should respond with the new data to update the screen state.

**Expected Server Response Payload:**
```json
{
  "version": "3.0",
  "screen": "BOOKING",
  "data": {
    "is_dropdown_visible": true,
    "available_slots": [
      { "id": "1", "title": "08:00" },
      { "id": "2", "title": "09:00" }
    ]
  }
}
```

---

## Common Patterns & Gotchas

### Required + Visible for Inputs

Many input and selection components (`TextInput`, `Dropdown`, `CheckboxGroup`,
`RadioButtonsGroup`, etc.) support both `required` and `visible`.

- **Important:** WhatsApp Flows validation will fail if a component is
  effectively hidden but marked as required (for example,
  `required: true` and `visible: false`).
- When you hide a component until data is ready, either:
  - keep `required` omitted or `false` while it is hidden and only validate on a
    later screen, or
  - bind `required` to the same condition as visibility, for example:
    `required: "${data.is_dropdown_visible}"`.

The **Dynamic Components (Tutorial)** above uses this pattern for the `Dropdown`
that is only required once it is visible.

### Dynamic dropdowns (invisible → visible)

For patterns where:

- The user first picks a date or filter.
- The endpoint returns a set of options.
- A dropdown becomes visible once options are available.

Recommended pattern:

1. In `data`, declare:
   - a boolean flag like `is_dropdown_visible` (default `false`),
   - an array like `available_slots` with the `{ id, title }` structure.
2. In the layout:
   - bind `visible` to `${data.is_dropdown_visible}`,
   - bind `data-source` to `${data.available_slots}`,
   - optionally bind `required` to the same visibility flag.
3. In the endpoint response:
   - set `is_dropdown_visible: true` when you have options,
   - populate `available_slots` with the list of choices.

This keeps validation rules consistent and avoids hidden-but-required inputs.

### Routing by action + screen

For data endpoints, prefer routing using:

- `data_exchange.action`: `"INIT"`, `"data_exchange"`, `"BACK"`,
- `data_exchange.screen`: the current screen ID.

Custom fields inside `data` (for example, `data.action`) can still be used as
secondary hints, but they should not replace the canonical
`action + screen` decision tree described in the **Data Endpoint Quick Start**.

---

# Additional Technical Reference

## Logic Component Details (`If`)

### Supported Operators Reference
The `condition` property supports specific operators and data types.

| Operator          | Symbol | Types Allowed           | Rules & Return Type                                                                                      |
| :---------------- | :----- | :---------------------- | :------------------------------------------------------------------------------------------------------- |
| **Parentheses**   | `()`   | Boolean, Number, String | Used to define precedence. <br> *Example:* `${form.opt} \|\| (${data.num} > 5)`                          |
| **Equal**         | `==`   | Boolean, Number, String | Both sides must be the same type. <br> *Example:* `${form.city} == 'London'`                             |
| **Not Equal**     | `!=`   | Boolean, Number, String | Both sides must be the same type. <br> *Example:* `${data.val} != 5`                                     |
| **AND**           | `&&`   | Boolean                 | Evaluates true only if both sides are true. High priority. <br> *Example:* `${form.opt} && ${data.bool}` |
| **OR**            | `\|\|` | Boolean                 | Evaluates true if at least one side is true. <br> *Example:* `${form.opt} \|\| ${data.bool}`             |
| **NOT**           | `!`    | Boolean                 | Negates the statement. <br> *Example:* `!(${data.num} > 5)`                                              |
| **Greater Than**  | `>`    | Number                  | *Example:* `${data.num} > 5`                                                                             |
| **Greater/Equal** | `>=`   | Number                  | *Example:* `${data.num} >= 5`                                                                            |
| **Less Than**     | `<`    | Number                  | *Example:* `${data.num} < 5`                                                                             |
| **Less/Equal**    | `<=`   | Number                  | *Example:* `${data.num} <= 5`                                                                            |

### Validation Errors & Limitations (`If`)
These are the specific validation errors you will encounter during Flow compilation if rules are violated.

| Scenario                                        | Validation Error Message                                                                           |
| :---------------------------------------------- | :------------------------------------------------------------------------------------------------- |
| Footer exists in `then`, but `else` is missing. | *Missing Footer inside one of the if branches. Branch "else" should exist and contain one Footer.* |
| Footer exists in `then`, but not in `else`.     | *Missing Footer inside one of the if branches.*                                                    |
| Footer exists in `else`, but not in `then`.     | *Missing Footer inside one of the if branches.*                                                    |
| Footer exists inside `If` AND outside `If`.     | *You can only have 1 Footer component per screen.*                                                 |
| The `then` array is empty.                      | *Invalid value found at: ".../then" due to empty array. It should contain at least one component.* |

---

## Logic Component Details (`Switch`)

### Validation Errors (`Switch`)

| Scenario                       | Validation Error Message                        |
| :----------------------------- | :---------------------------------------------- |
| The `cases` property is empty. | *Invalid empty property found at: ".../cases".* |

---

## Rich Text Best Practices

### Working with Large Texts
While `RichText` allows static text arrays, it is recommended to use **dynamic data** for large documents (like Terms of Service).
*   **Why?** Improves JSON readability and allows you to update the text from your server without changing the Flow JSON.
*   **How?** Send the markdown string as a normal string property in your data payload (no need to convert it to an array of strings).

**Example:**
```json
{
   "type": "RichText",
   "text": "${data.terms_of_service_content}"
}
```

---

## Navigation List: Specific Restrictions

While the general limits were provided, note these specific behaviors for `NavigationList`:

1.  **Image Placeholder:** If an image in the `start` object exceeds **100KB**, it will not be displayed; it will be replaced by a generic placeholder.
2.  **Truncation:**
    *   `label` content (>80 chars) will truncate.
    *   `description` content (>300 chars) will truncate.
    *   `list-items` content (>20 items) will not render if the limit is reached.
3.  **Action definition:** You cannot define `on-click-action` on **both** the component level and the item level simultaneously. It must be one or the other.

---

# UX & Content Best Practices

These guidelines are distilled from Meta’s Flows docs and are intended to
produce flows that feel fast, clear, and trustworthy.

## Flow Length & Screen Design

- **Keep flows short.** Design flows so a typical user can complete the main
  task in **under ~5 minutes**.
- **One primary task per screen.** Avoid screens that try to collect many
  unrelated inputs at once. If you need multiple tasks, split them into
  separate screens.
- **Limit components per screen.**
  - Too many components make the layout noisy and slow to load.
  - Consider what happens if a user leaves mid-flow: fewer components per
    screen means less data lost when they re-enter.
- **Use the right screen for the right task.**
  - If a sub-flow is needed (for example, “Forgot password”), keep it
    **small (≤ 3 screens)** and return the user to the main task afterwards.

## Initiation & Navigation

- **Initiation message should match the first screen.**
  - The chat message + CTA that opens the flow must clearly describe the task.
  - The first screen should immediately reflect that task—no surprises.
- **Set expectations up front.**
  - In either the chat copy or the first screen, indicate roughly how long it
    will take (for example, “This will take about 2–3 minutes.”).
- **Use clear, action-oriented titles.**
  - Examples: “Book appointment”, “Confirm details”, “Update contact info”.
  - Use titles to show progress where it helps, for example “Step 1 of 3”.
- **Always end with a summary / confirmation screen.**
  - Show what the user is about to submit (or what was submitted).
  - Make the final CTA explicit, for example “Confirm booking”.

## CTAs, Copy & Style

- **CTAs should describe the outcome.**
  - Good: “Confirm booking”, “Submit application”.
  - Weak: “Continue”, “Next”.
- **Use sentence case consistently.**
  - Prefer “Book appointment” over “BOOK APPOINTMENT”.
- **Use emojis sparingly.**
  - Only when they add clarity or match brand tone.
  - Avoid them in critical or highly formal flows.
- **Avoid redundant copy.**
  - Don’t repeat the same phrase in title and body without adding information
    (for example, avoid “Complete registration” and then “Complete registration
    below”).

## Forms & Input Quality

- **Choose the appropriate component:**
  - Use `DatePicker` / `CalendarPicker` for dates (not free-text).
  - Use `TextArea` for long free-text answers.
  - Use `Dropdown` / `RadioButtonsGroup` / `CheckboxGroup` instead of free text
    when the set of options is known.
- **Order fields logically.**
  - For example: first name → last name → email → phone.
- **Make non-critical fields optional.**
  - Only mark fields as required if they are truly needed to complete the task.
- **Use helper text for tricky fields.**
  - Example: phone number formats, password rules, date formats.

## Options & Lists

- **Keep option lists small.**
  - Aim for **≤ 10 options per screen** when possible.
- **Pick the right selection component:**
  - Use **RadioButtons** when the user must pick exactly one option.
  - Use **CheckboxGroup** when multiple selections are allowed.
  - Use **Dropdown** when there are many options (Meta recommends using it
    when there are ~8 or more choices).
- **Use sensible defaults.**
  - Where appropriate, make the first option (or the most common one) the
    default selection.

## Error Handling & Validation UX

- **Errors should say what happened and how to fix it.**
  - For example: “This email looks invalid. Please check the format.” instead
    of just “Error”.
- **Show validation rules up front.**
  - Use helper text for passwords, date formats, numeric ranges, etc.
- **Prefer staying in-flow on errors.**
  - When endpoint data becomes invalid (for example, a slot becomes
    unavailable), send the user back to the previous relevant screen with a
    clear error message rather than ending the flow abruptly.

## Login & Trust

- **Use login screens only when necessary.**
  - Logging in can feel heavy inside a flow; only require it when the task
    truly needs authentication.
- **Place login later in the flow.**
  - Show value first (for example, show summary or available options), then
    prompt for login as one of the last steps before completion.
- **Maintain sense of place.**
  - Make it clear that login is still part of the same flow, not an external
    redirect.
- **Make support easy to reach.**
  - Provide a way (inside the flow or in follow-up messages) for users to
    contact support if something goes wrong.

## Termination & Follow-up

- **Clearly describe what happens after completion.**
  - The last screen should confirm the action and set expectations for what
    happens next (for example, “We’ll send a confirmation email shortly.”).
- **Keep completion payloads lean.**
  - Only send data that is needed downstream; avoid huge payloads or embedded
    images in completion responses.
- **Bookend with a chat message.**
  - After the Flow completes, send a human-readable message to the chat that
    summarizes the outcome and provides next steps or support information.
  - Combine this with the built-in summary behavior and `sensitive` fields when
    appropriate.

## Writing & Formatting

- **Use a clear content hierarchy.**
  - Use headings for the main point, body text for explanation, and captions
    for small hints or secondary information.
- **Format data appropriately.**
  - Use correct currency symbols, phone number formats, and localized date/time
    formats that match the user’s expectations.
- **Check grammar and spelling.**
  - Read flows end-to-end before publishing; keep terminology and
    capitalization consistent.
