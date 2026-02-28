# openclaw-kapso-whatsapp

An [OpenClaw](https://openclaw.dev) plugin that bridges [Kapso](https://kapso.ai) WhatsApp Cloud API to your OpenClaw gateway. Your agent can send and receive WhatsApp messages without persistent connections, phone emulation, or ban risk.

## How it works

The bridge supports three delivery modes: **polling** (default), **Tailscale Funnel** (auto-tunnel, no domain needed), and **your own domain** (reverse proxy).

### Option 1: Polling (default)

```
kapso-whatsapp-poller
    |  (polls every 30s)
Kapso REST API (GET /messages?direction=inbound)
    |  (new messages found)
OpenClaw Gateway (WebSocket :18789)
    |  (agent decides to reply)
kapso-whatsapp-cli send --to +NUMBER --text "reply"
    |
Kapso REST API (POST /messages) --> WhatsApp User
```

### Option 2: Tailscale Funnel (auto-tunnel)

```
Kapso webhook POST ──→ https://<machine>.<tailnet>.ts.net/webhook
                            |  (tailscale funnel auto-started)
                       Webhook server (:18790)
                            |  (instant delivery)
                       OpenClaw Gateway (WebSocket :18789)
                            |  (agent decides to reply)
                       kapso-whatsapp-cli send --to +NUMBER --text "reply"
                            |
                       Kapso REST API (POST /messages) --> WhatsApp User
```

### Option 3: Your own domain

```
Kapso webhook POST ──→ https://yourdomain.com/webhook
                            |  (reverse proxy → :18790)
                       Webhook server (:18790)
                            |  (instant delivery)
                       OpenClaw Gateway (WebSocket :18789)
                            |  (agent decides to reply)
                       kapso-whatsapp-cli send --to +NUMBER --text "reply"
                            |
                       Kapso REST API (POST /messages) --> WhatsApp User
```

The plugin ships two binaries:

- **`kapso-whatsapp-poller`** — Receives inbound messages via polling, webhooks, or both, and forwards them to the OpenClaw gateway over WebSocket. Tracks state to avoid duplicates.
- **`kapso-whatsapp-cli`** — CLI tool added to the agent's PATH so it can send messages on demand.

Both are statically compiled Go binaries with no runtime dependencies.

## Prerequisites

- [OpenClaw](https://openclaw.dev) gateway running
- A [Kapso](https://kapso.ai) account with WhatsApp Cloud API access
- Your Kapso API key and phone number ID

## Installation

### As an OpenClaw plugin (Nix flake)

Add to your flake inputs:

```nix
# flake.nix
inputs.kapso-whatsapp = {
  url = "github:Enriquefft/openclaw-kapso-whatsapp";
  inputs.nixpkgs.follows = "nixpkgs";
};
```

The flake exports an `openclawPlugin` contract. If your nix-openclaw module supports `customPlugins`:

```nix
# home-manager config
programs.openclaw.customPlugins = [
  {
    source = "github:Enriquefft/openclaw-kapso-whatsapp";
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

  # Poller as a systemd user service
  systemd.user.services.kapso-whatsapp-poller = {
    Unit = {
      Description = "Kapso WhatsApp Poller";
      After = [ "openclaw-gateway.service" ];
    };
    Service = {
      ExecStart = "${kapso.poller}/bin/kapso-whatsapp-poller";
      Restart = "on-failure";
      EnvironmentFile = [ "/path/to/your/env-file" ];
    };
    Install.WantedBy = [ "default.target" ];
  };
}
```

### Without Nix

```bash
go install github.com/Enriquefft/openclaw-kapso-whatsapp/cmd/kapso-whatsapp-cli@latest
go install github.com/Enriquefft/openclaw-kapso-whatsapp/cmd/kapso-whatsapp-poller@latest
```

Copy `skills/whatsapp/SKILL.md` into your OpenClaw workspace skills directory.

## Configuration

All configuration is through environment variables.

### Poller (`kapso-whatsapp-poller`)

| Variable | Required | Default | Description |
|---|---|---|---|
| `KAPSO_API_KEY` | Yes | — | Kapso API key |
| `KAPSO_PHONE_NUMBER_ID` | Yes | — | Kapso WhatsApp phone number ID |
| `OPENCLAW_GATEWAY_URL` | No | `ws://127.0.0.1:18789` | OpenClaw gateway WebSocket URL |
| `OPENCLAW_TOKEN` | No | — | Gateway auth token (if auth is enabled) |
| `KAPSO_POLL_INTERVAL` | No | `30` | Polling interval in seconds (minimum 5) |
| `KAPSO_STATE_DIR` | No | `~/.config/kapso-whatsapp` | Directory for poll state file |
| `KAPSO_MODE` | No | `polling` | `polling` = poll only, `tailscale` = auto-tunnel + webhook, `domain` = own domain + webhook |
| `KAPSO_POLL_FALLBACK` | No | `false` | When using `tailscale` or `domain`, also run polling as a safety net |
| `KAPSO_WEBHOOK_ADDR` | No | `:18790` | Webhook HTTP listen address |
| `KAPSO_WEBHOOK_VERIFY_TOKEN` | When `tailscale`/`domain` | — | Token for Meta webhook verification challenge |
| `KAPSO_WEBHOOK_SECRET` | No | — | HMAC-SHA256 secret for validating webhook signatures |

### CLI (`kapso-whatsapp-cli`)

| Variable | Required | Default | Description |
|---|---|---|---|
| `KAPSO_API_KEY` | Yes | — | Kapso API key |
| `KAPSO_PHONE_NUMBER_ID` | Yes | — | Kapso WhatsApp phone number ID |

## Usage

### Sending messages

```bash
kapso-whatsapp-cli send --to +1234567890 --text "Hello from OpenClaw"
```

### Running the poller

```bash
export KAPSO_API_KEY="your-key"
export KAPSO_PHONE_NUMBER_ID="your-phone-number-id"
export OPENCLAW_TOKEN="your-gateway-token"
kapso-whatsapp-poller
```

The poller stores its last-seen timestamp in `~/.config/kapso-whatsapp/last-poll` to avoid replaying old messages across restarts.

## Project structure

```
cmd/
  kapso-whatsapp-cli/       CLI for sending messages
  kapso-whatsapp-poller/    Receives inbound messages (polling, tailscale, or domain)
internal/
  kapso/                    Kapso API client, message types, list endpoint
  gateway/                  WebSocket client to OpenClaw gateway
  webhook/                  HTTP webhook server for real-time delivery
  tailscale/                Tailscale Funnel automation (auto-start, URL discovery)
skills/
  whatsapp/                 SKILL.md — agent instructions
```

## Delivery modes

### Option 1: Polling (default)

Works out of the box — no public endpoint, no domain, no tunnel. One HTTP request every 30 seconds with up to 30s latency on incoming messages. Fine for personal use.

```bash
# Default: polling only, no extra config needed
kapso-whatsapp-poller
```

### Option 2: Tailscale Funnel (zero-config tunnel)

For real-time delivery (< 1s latency) without owning a domain. The poller automatically starts [Tailscale Funnel](https://tailscale.com/kb/1223/funnel) and prints the webhook URL — just paste it into Kapso.

**Prerequisites:**
1. Install [Tailscale](https://tailscale.com/download) and run `tailscale up`
2. Enable HTTPS certificates: `tailscale cert`

**Start:**

```bash
export KAPSO_MODE="tailscale"
export KAPSO_WEBHOOK_VERIFY_TOKEN="your-secret-token"
kapso-whatsapp-poller
# Prints: register this webhook URL in Kapso: https://<machine>.<tailnet>.ts.net/webhook
```

The poller auto-runs `tailscale funnel 18790` — no manual tunnel setup needed. On shutdown (Ctrl+C / SIGTERM) the funnel process is cleaned up automatically.

**Why Tailscale Funnel?** No rate limits on incoming requests, open-source client, no browser interstitial, deterministic URLs, free for personal use.

### Option 3: Your own domain

If you already have a domain with HTTPS (e.g. behind nginx, Caddy, or Cloudflare), point your reverse proxy at the webhook server.

```bash
export KAPSO_MODE="domain"
export KAPSO_WEBHOOK_VERIFY_TOKEN="your-secret-token"
export KAPSO_WEBHOOK_SECRET="your-hmac-secret"  # optional but recommended
kapso-whatsapp-poller
```

Then configure your reverse proxy:

**nginx:**

```nginx
location /webhook {
    proxy_pass http://127.0.0.1:18790;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

**Caddy:**

```
yourdomain.com {
    reverse_proxy /webhook localhost:18790
}
```

Register `https://yourdomain.com/webhook` as the webhook URL in Kapso with your verify token.

### Polling fallback

Any webhook mode (`tailscale` or `domain`) can optionally run polling as a safety net. Messages are deduplicated by ID — if a webhook already delivered a message, the polling loop skips it.

```bash
export KAPSO_MODE="tailscale"   # or "domain"
export KAPSO_POLL_FALLBACK="true"
export KAPSO_WEBHOOK_VERIFY_TOKEN="your-secret-token"
kapso-whatsapp-poller
```

## Why Kapso instead of Baileys/direct WhatsApp?

- **No persistent connections** — stateless API calls, near-zero idle CPU
- **No phone emulation** — uses official Cloud API through Kapso, no ban risk
- **No session management** — nothing to keep alive
- **Lower power footprint** — ideal for home servers and laptops

## License

MIT
