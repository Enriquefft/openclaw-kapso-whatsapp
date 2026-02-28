# openclaw-kapso-whatsapp

An [OpenClaw](https://openclaw.dev) plugin that bridges [Kapso](https://kapso.ai) WhatsApp Cloud API to your OpenClaw gateway. Your agent can send and receive WhatsApp messages without persistent connections, phone emulation, or ban risk.

## How it works

```
WhatsApp User
    |  (sends message)
Kapso Cloud API
    |  (webhook POST)
Your server (or Cloudflare Tunnel)
    |
kapso-whatsapp-webhook (:18790)
    |  (WebSocket)
OpenClaw Gateway (:18789)
    |  (agent decides to reply)
kapso-whatsapp-cli send --to +NUMBER --text "reply"
    |
Kapso REST API --> WhatsApp User
```

The plugin ships two binaries:

- **`kapso-whatsapp-webhook`** — HTTP server that receives Meta-format webhook events from Kapso, verifies HMAC-SHA256 signatures, and forwards incoming messages to the OpenClaw gateway over WebSocket.
- **`kapso-whatsapp-cli`** — CLI tool added to the agent's PATH so it can send messages on demand.

Both are statically compiled Go binaries with no runtime dependencies.

## Prerequisites

- [OpenClaw](https://openclaw.dev) gateway running
- A [Kapso](https://kapso.ai) account with WhatsApp Cloud API access
- Your Kapso API key, phone number ID, and webhook secret
- A way to receive webhooks (public IP, reverse proxy, or Cloudflare Tunnel)

## Installation

### As an OpenClaw plugin (Nix flake)

Add to your flake inputs:

```nix
# flake.nix
inputs.kapso-whatsapp = {
  url = "github:hybridz/openclaw-kapso-whatsapp";
  inputs.nixpkgs.follows = "nixpkgs";
};
```

The flake exports an `openclawPlugin` contract. If your nix-openclaw module supports `customPlugins`:

```nix
# home-manager config
programs.openclaw.customPlugins = [
  {
    source = "github:hybridz/openclaw-kapso-whatsapp";
    config.env = {
      KAPSO_API_KEY = "/run/secrets/kapso-api-key";
      KAPSO_PHONE_NUMBER_ID = "/run/secrets/kapso-phone-number-id";
    };
  }
];
```

Or wire manually:

```nix
{ inputs, pkgs, ... }:
let
  kapso = inputs.kapso-whatsapp.packages.${pkgs.system};
in {
  # CLI available to the agent
  home.packages = [ kapso.cli ];

  # Skill teaches the agent how to use the CLI
  home.file.".openclaw/workspace/skills/whatsapp".source =
    "${inputs.kapso-whatsapp}/skills/whatsapp";

  # Webhook server as a systemd user service
  systemd.user.services.kapso-whatsapp-webhook = {
    Unit = {
      Description = "Kapso WhatsApp Webhook Server";
      After = [ "openclaw-gateway.service" ];
    };
    Service = {
      ExecStart = "${kapso.webhook}/bin/kapso-whatsapp-webhook";
      Restart = "on-failure";
      EnvironmentFile = [ "/path/to/your/env-file" ];
    };
    Install.WantedBy = [ "default.target" ];
  };
}
```

### Without Nix

```bash
go install github.com/hybridz/openclaw-kapso-whatsapp/cmd/kapso-whatsapp-cli@latest
go install github.com/hybridz/openclaw-kapso-whatsapp/cmd/kapso-whatsapp-webhook@latest
```

Copy `skills/whatsapp/SKILL.md` into your OpenClaw workspace skills directory.

## Configuration

All configuration is through environment variables.

### Webhook server (`kapso-whatsapp-webhook`)

| Variable | Required | Default | Description |
|---|---|---|---|
| `KAPSO_WEBHOOK_SECRET` | Yes | — | Webhook signing secret from Kapso dashboard |
| `OPENCLAW_GATEWAY_URL` | No | `ws://127.0.0.1:18789` | OpenClaw gateway WebSocket URL |
| `OPENCLAW_TOKEN` | No | — | Gateway auth token (if gateway auth is enabled) |
| `KAPSO_WEBHOOK_ADDR` | No | `:18790` | Address the webhook server listens on |

### CLI (`kapso-whatsapp-cli`)

| Variable | Required | Default | Description |
|---|---|---|---|
| `KAPSO_API_KEY` | Yes | — | Kapso API key |
| `KAPSO_PHONE_NUMBER_ID` | Yes | — | Kapso WhatsApp phone number ID |
| `KAPSO_WEBHOOK_ADDR` | No | `http://localhost:18790` | Webhook server address (for `status` command) |

## Usage

### Sending messages

```bash
kapso-whatsapp-cli send --to +1234567890 --text "Hello from OpenClaw"
```

### Checking webhook health

```bash
kapso-whatsapp-cli status
```

### Running the webhook server

```bash
export KAPSO_WEBHOOK_SECRET="your-secret"
export OPENCLAW_TOKEN="your-gateway-token"
kapso-whatsapp-webhook
```

The server exposes:
- `POST /webhook` — receives Kapso webhook events
- `GET /webhook` — handles Meta webhook verification challenge
- `GET /health` — returns `200 ok`

## Receiving webhooks

The webhook server needs to be reachable from the internet. A few options:

**Cloudflare Tunnel (recommended for home servers):**

```bash
cloudflared tunnel login
cloudflared tunnel create kapso-webhook
cloudflared tunnel route dns kapso-webhook your-subdomain.example.com
cloudflared tunnel run --url http://localhost:18790 kapso-webhook
```

**Reverse proxy:** Point your existing nginx/caddy at `localhost:18790`.

**Direct:** If your server has a public IP, just expose port 18790.

Then in the Kapso dashboard, set the webhook URL to `https://your-host/webhook` and subscribe to message events.

## Webhook verification

The server verifies incoming webhooks using the `X-Hub-Signature-256` header (HMAC-SHA256 with your webhook secret). It also handles Meta's `GET` verification challenge for webhook registration.

## Project structure

```
cmd/
  kapso-whatsapp-cli/       CLI for sending messages
  kapso-whatsapp-webhook/   Webhook receiver server
internal/
  kapso/                    Kapso API client and Meta-format types
  webhook/                  HTTP server, signature verification, payload handling
  gateway/                  WebSocket client to OpenClaw gateway
skills/
  whatsapp/                 SKILL.md — agent instructions
```

## Why Kapso instead of Baileys/direct WhatsApp?

- **No persistent connections** — event-driven webhooks, near-zero idle CPU
- **No phone emulation** — uses official Cloud API through Kapso, no ban risk
- **No session management** — stateless HTTP, nothing to keep alive
- **Lower power footprint** — ideal for home servers and laptops

## License

MIT
