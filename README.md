# openclaw-kapso-whatsapp

An [OpenClaw](https://openclaw.dev) plugin that bridges [Kapso](https://kapso.ai) WhatsApp Cloud API to your OpenClaw gateway. Your agent can send and receive WhatsApp messages without persistent connections, phone emulation, or ban risk.

## How it works

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

The plugin ships two binaries:

- **`kapso-whatsapp-poller`** — Polls the Kapso messages API for inbound messages and forwards them to the OpenClaw gateway over WebSocket. Tracks state to avoid duplicates.
- **`kapso-whatsapp-cli`** — CLI tool added to the agent's PATH so it can send messages on demand.

Both are statically compiled Go binaries with no runtime dependencies. No tunnels, domains, or public endpoints needed.

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
  kapso-whatsapp-poller/    Polls Kapso API for inbound messages
internal/
  kapso/                    Kapso API client, message types, list endpoint
  gateway/                  WebSocket client to OpenClaw gateway
skills/
  whatsapp/                 SKILL.md — agent instructions
```

## Why polling?

- **No public endpoint** — outbound-only, nothing to expose
- **No tunnel or domain** — works behind any NAT or firewall
- **Near-zero resources** — one HTTP request every 30 seconds
- **Simple** — no webhook verification, no signature checking, no TLS

The trade-off is up to 30s latency on incoming messages, which is fine for personal use.

## Why Kapso instead of Baileys/direct WhatsApp?

- **No persistent connections** — stateless API calls, near-zero idle CPU
- **No phone emulation** — uses official Cloud API through Kapso, no ban risk
- **No session management** — nothing to keep alive
- **Lower power footprint** — ideal for home servers and laptops

## License

MIT
