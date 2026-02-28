# openclaw-kapso-whatsapp

Give your [OpenClaw](https://openclaw.dev) AI agent a WhatsApp number.
Official Meta Cloud API via [Kapso](https://kapso.ai) — a unified API for WhatsApp Cloud. No ban risk.
Stateless. Two Go binaries. Near-zero idle CPU.

[![CI](https://github.com/Enriquefft/openclaw-kapso-whatsapp/actions/workflows/ci.yml/badge.svg)](https://github.com/Enriquefft/openclaw-kapso-whatsapp/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/Enriquefft/openclaw-kapso-whatsapp)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/Enriquefft/openclaw-kapso-whatsapp)](https://github.com/Enriquefft/openclaw-kapso-whatsapp/releases)

## Architecture

```
WhatsApp --> Kapso API --> kapso-whatsapp-poller --> OpenClaw Gateway --> AI Agent
   ^                                |
   +--------------------------------+
          relay: reads session JSONL, sends reply back
```

Three delivery modes: **polling** (zero-config), **Tailscale Funnel** (auto-tunnel), or **your own domain**.

## No ban risk. No phone emulation.

Libraries like Baileys and whatsapp-web.js reverse-engineer WhatsApp Web — Meta actively detects and bans these connections. This bridge uses the **official Cloud API** through Kapso, so your number stays safe.

- **Stateless API calls** — no persistent connections, near-zero idle CPU
- **No session management** — nothing to keep alive or reconnect
- **Lower power footprint** — ideal for home servers and laptops
- **Works with any OpenClaw agent** — just drop in SKILL.md

## Demo

<!-- TODO: Record with asciinema. Show: export vars → start poller → message arrives → relay responds → CLI send. Target 45-60s. -->
_Coming soon — terminal recording of the full message flow._

## Quick start

No config file. No database. No reverse proxy.

```bash
export KAPSO_API_KEY="your-key"
export KAPSO_PHONE_NUMBER_ID="your-phone-number-id"
kapso-whatsapp-poller
```

That's it — polling mode works with zero configuration. To cut latency to under 1 second, see [Tailscale Funnel mode](#tailscale-funnel-zero-config-tunnel).

## Installation

### Go install

```bash
go install github.com/Enriquefft/openclaw-kapso-whatsapp/cmd/kapso-whatsapp-cli@latest
go install github.com/Enriquefft/openclaw-kapso-whatsapp/cmd/kapso-whatsapp-poller@latest
```

Copy the agent skill definition into your OpenClaw workspace:

```bash
cp skills/whatsapp/SKILL.md ~/.openclaw/skills/whatsapp/SKILL.md
```

### Prebuilt binaries

Download the latest release for your platform from [GitHub Releases](https://github.com/Enriquefft/openclaw-kapso-whatsapp/releases).

### NixOS / Home Manager

<details>
<summary>Nix flake + home-manager setup (includes sops-nix support)</summary>

```nix
# flake.nix
inputs.kapso-whatsapp = {
  url = "github:Enriquefft/openclaw-kapso-whatsapp";
  inputs.nixpkgs.follows = "nixpkgs";
};
```

Then in your home-manager config:

```nix
imports = [ inputs.kapso-whatsapp.homeManagerModules.default ];

services.kapso-whatsapp = {
  enable = true;
  package = inputs.kapso-whatsapp.packages.${pkgs.system}.poller;
  cliPackage = inputs.kapso-whatsapp.packages.${pkgs.system}.cli;
  secrets.apiKeyFile = config.sops.secrets.kapso-api-key.path;          # or any file path
  secrets.phoneNumberIdFile = config.sops.secrets.kapso-phone-number-id.path;
};
```

The module generates `~/.config/kapso-whatsapp/config.toml`, installs the CLI, and creates a systemd user service. Secrets are read from files at startup — works with sops-nix, agenix, or plain files.

</details>

## Configuration

The [Quick start](#quick-start) covers the minimum: just two env vars. Everything else has sensible defaults.

<details>
<summary>Full config reference</summary>

Create `~/.config/kapso-whatsapp/config.toml` (or set `KAPSO_CONFIG` to a custom path):

```toml
[kapso]
api_key = ""              # prefer KAPSO_API_KEY env var for secrets
phone_number_id = ""      # prefer KAPSO_PHONE_NUMBER_ID env var

[delivery]
mode = "polling"          # "polling" | "tailscale" | "domain"
poll_interval = 30        # seconds (minimum 5)
poll_fallback = false     # run polling alongside webhook as safety net

[webhook]
addr = ":18790"
verify_token = ""         # prefer KAPSO_WEBHOOK_VERIFY_TOKEN env var
secret = ""               # prefer KAPSO_WEBHOOK_SECRET env var

[gateway]
url = "ws://127.0.0.1:18789"
token = ""                # prefer OPENCLAW_TOKEN env var
session_key = "main"
sessions_json = "~/.openclaw/agents/main/sessions/sessions.json"

[state]
dir = "~/.config/kapso-whatsapp"
```

**Loading order:** built-in defaults → config file → env vars. Environment variables always win, so existing setups work unchanged.

### Secrets (env vars)

These are the only values you typically need to set as env vars:

| Variable | When needed |
|---|---|
| `KAPSO_API_KEY` | Always |
| `KAPSO_PHONE_NUMBER_ID` | Always |
| `KAPSO_WEBHOOK_VERIFY_TOKEN` | Tailscale or domain mode |
| `KAPSO_WEBHOOK_SECRET` | Domain mode (optional, HMAC validation) |
| `OPENCLAW_TOKEN` | If gateway auth is enabled |

All other settings have sensible defaults and belong in the config file if you need to change them.

</details>

### Security

The bridge enforces sender allowlisting, per-sender rate limiting, role tagging, and session isolation. By default, security mode is `allowlist` — only phone numbers listed in `[security.roles]` can interact with the agent.

#### Config file

```toml
[security]
mode = "allowlist"                    # "allowlist" | "open"
deny_message = "Sorry, you are not authorized to use this service."
rate_limit = 10                       # max messages per window per sender
rate_window = 60                      # window in seconds
session_isolation = true              # per-sender sessions (false = shared)
default_role = "member"               # role for unlisted senders in "open" mode

[security.roles]
admin = ["+1234567890"]
member = ["+0987654321", "+1122334455"]
```

Each role maps to a list of phone numbers. The agent receives a `[role: <role>]` tag in every forwarded message, enabling role-based capability enforcement in SKILL.md.

#### Environment variables

For simple setups without roles:

| Variable | Description |
|---|---|
| `KAPSO_SECURITY_MODE` | `"allowlist"` or `"open"` |
| `KAPSO_ALLOWED_NUMBERS` | Comma-separated phone numbers (all get `default_role`) |
| `KAPSO_DENY_MESSAGE` | Message sent to unauthorized senders |
| `KAPSO_RATE_LIMIT` | Max messages per window per sender |
| `KAPSO_RATE_WINDOW` | Rate limit window in seconds |
| `KAPSO_SESSION_ISOLATION` | `"true"` or `"false"` |
| `KAPSO_DEFAULT_ROLE` | Role for senders not in the roles map |

If a number appears in both `KAPSO_ALLOWED_NUMBERS` and the TOML `[security.roles]`, the TOML role wins.

#### Behavior

- **Allowlist mode** (default): Only numbers in `[security.roles]` can send messages. Unauthorized senders receive the deny message.
- **Open mode**: Anyone can send. Senders not in the roles map get `default_role`.
- **Rate limiting**: Fixed-window token bucket per sender. Excess messages are silently dropped (no response to avoid amplification).
- **Session isolation** (default on): Each sender gets their own OpenClaw session (`main-wa-<number>`), preventing cross-sender context leakage.

### Delivery modes

#### Polling (default)

Works out of the box — no public endpoint, no domain, no tunnel. Polls every 30 seconds with up to 30s latency on incoming messages.

```
kapso-whatsapp-poller  --poll-->  Kapso REST API
       |                               |
       |  (new messages)               |
       v                               |
  OpenClaw Gateway  --reply-->  kapso-whatsapp-cli  -->  Kapso API  -->  WhatsApp
```

No extra config needed — just set the two required env vars.

#### Tailscale Funnel (zero-config tunnel)

Real-time delivery (< 1s latency) without owning a domain. The poller starts [Tailscale Funnel](https://tailscale.com/kb/1223/funnel) automatically and prints the webhook URL to register in Kapso. Tailscale Funnel works on the free plan.

```
Kapso webhook POST  -->  https://<machine>.<tailnet>.ts.net/webhook
                               |  (tailscale funnel, auto-started)
                          Webhook server (:18790)
                               |
                          OpenClaw Gateway  --reply-->  Kapso API  -->  WhatsApp
```

**Prerequisites:** Tailscale installed and running (`tailscale up`), HTTPS certs enabled (`tailscale cert`).

**Config diff from defaults:**

```toml
[delivery]
mode = "tailscale"
```

Plus set `KAPSO_WEBHOOK_VERIFY_TOKEN`.

#### Your own domain

If you have a domain with HTTPS (behind nginx, Caddy, or Cloudflare), point your reverse proxy at the webhook server.

```
Kapso webhook POST  -->  https://yourdomain.com/webhook
                               |  (reverse proxy -> :18790)
                          Webhook server (:18790)
                               |
                          OpenClaw Gateway  --reply-->  Kapso API  -->  WhatsApp
```

**Config diff from defaults:**

```toml
[delivery]
mode = "domain"
```

Plus set `KAPSO_WEBHOOK_VERIFY_TOKEN` (and optionally `KAPSO_WEBHOOK_SECRET` for HMAC validation).

Register `https://yourdomain.com/webhook` as the webhook URL in Kapso.

> **Polling fallback:** Any webhook mode can also run polling as a safety net by setting `poll_fallback = true`. Messages are deduplicated by ID.

## Project structure

```
cmd/
  kapso-whatsapp-cli/       CLI for sending messages
  kapso-whatsapp-poller/    Receives inbound messages (polling, tailscale, or domain)
internal/
  config/                   TOML config loading with env var overrides
  kapso/                    Kapso API client, message types, list endpoint
  gateway/                  WebSocket client to OpenClaw gateway
  delivery/                 Source abstraction, fan-in merge, dedup, extraction
    poller/                 Polling source
    webhook/                HTTP webhook source
  relay/                    Relay agent replies back to WhatsApp
  security/                 Allowlist, rate limiting, role tagging, session isolation
  tailscale/                Tailscale Funnel automation (auto-start, URL discovery)
nix/
  module.nix                Home-manager module with typed options + sops-nix support
skills/
  whatsapp/                 SKILL.md — agent instructions
```

## Development

### Prerequisites

- Go 1.22+
- (Optional) [just](https://github.com/casey/just) command runner

### Building and testing

```bash
just build          # Build both binaries
just test           # Run tests
just lint           # Run golangci-lint
just check          # Run tests + vet + format check
just install        # Install to $GOPATH/bin
```

Or without `just`:

```bash
go build ./...
go test ./...
go vet ./...
```

### Nix dev shell

If you use Nix, `direnv allow` or `nix develop` gives you Go, gopls, golangci-lint, goreleaser, and just.

## Contributing

Issues and PRs welcome. Run `just check` before submitting. The Nix dev shell provides all tooling: `direnv allow` or `nix develop`.

## License

MIT
