<p align="center">
  <img src=".github/icon.jpg" alt="openclaw-kapso-whatsapp" width="120">
</p>

# openclaw-kapso-whatsapp

Give your [OpenClaw](https://openclaw.ai) AI agent a WhatsApp number.
Official Meta Cloud API via [Kapso](https://kapso.ai) — a unified API for WhatsApp Cloud. No ban risk.
Stateless. Two Go binaries. Near-zero idle CPU.

[![Hypercommit](https://img.shields.io/badge/Hypercommit-DB2475)](https://hypercommit.com/openclaw-kapso-whatsapp)
[![CI](https://github.com/Enriquefft/openclaw-kapso-whatsapp/actions/workflows/ci.yml/badge.svg)](https://github.com/Enriquefft/openclaw-kapso-whatsapp/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/Enriquefft/openclaw-kapso-whatsapp)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/Enriquefft/openclaw-kapso-whatsapp)](https://github.com/Enriquefft/openclaw-kapso-whatsapp/releases)

## Architecture

```
WhatsApp --> Kapso API --> kapso-whatsapp-bridge --> OpenClaw Gateway --> AI Agent
   ^                                |
   +--------------------------------+
          relay: reads session JSONL, sends reply back
```

Libraries like Baileys and whatsapp-web.js reverse-engineer WhatsApp Web — Meta actively detects and bans these connections. This bridge uses the **official Cloud API** through Kapso, so your number stays safe. Stateless API calls, no session management, near-zero idle CPU.

<details>
<summary>Table of contents</summary>

- [Installation](#installation)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Security](#security)
- [Delivery modes](#delivery-modes)
- [Voice transcription](#voice-transcription)
- [Development](#development)
- [Contributing](#contributing)

</details>

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/Enriquefft/openclaw-kapso-whatsapp/main/scripts/install.sh | bash
```

Downloads the latest release, verifies SHA256 checksums, and installs to `~/.local/bin`. Override the install directory or version:

```bash
INSTALL_DIR=/usr/local/bin TAG=v0.2.1 curl -fsSL https://raw.githubusercontent.com/Enriquefft/openclaw-kapso-whatsapp/main/scripts/install.sh | bash
```

<details>
<summary>NixOS / Home Manager</summary>

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
  package = inputs.kapso-whatsapp.packages.${pkgs.system}.bridge;
  cliPackage = inputs.kapso-whatsapp.packages.${pkgs.system}.cli;
  secrets.apiKeyFile = config.sops.secrets.kapso-api-key.path;          # or any file path
  secrets.phoneNumberIdFile = config.sops.secrets.kapso-phone-number-id.path;
};
```

The module generates `~/.config/kapso-whatsapp/config.toml`, installs the CLI, and creates a systemd user service. Secrets are read from files at startup — works with sops-nix, agenix, or plain files.

</details>

## Quick start

No config file. No database. No reverse proxy.

```bash
export KAPSO_API_KEY="your-key"
export KAPSO_PHONE_NUMBER_ID="your-phone-number-id"

# Verify everything is configured correctly
kapso-whatsapp-cli preflight

# Start the bridge
kapso-whatsapp-bridge
```

That's it — polling mode works with zero configuration. To cut latency to under 1 second, see [Tailscale Funnel mode](#tailscale-funnel-zero-config-tunnel).

## Configuration

The [Quick start](#quick-start) covers the minimum: just two env vars. Everything else has sensible defaults.

**Loading order:** built-in defaults → config file → env vars. Environment variables always win.

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

| Variable | When needed |
|---|---|
| `KAPSO_API_KEY` | Always |
| `KAPSO_PHONE_NUMBER_ID` | Always |
| `KAPSO_WEBHOOK_VERIFY_TOKEN` | Tailscale or domain mode |
| `KAPSO_WEBHOOK_SECRET` | Domain mode (optional, HMAC validation) |
| `OPENCLAW_TOKEN` | If gateway auth is enabled |

</details>

## Security

The bridge enforces sender allowlisting, per-sender rate limiting, role tagging, and session isolation. By default, security mode is `allowlist` — only phone numbers listed in `[security.roles]` can interact with the agent.

<details>
<summary>Security configuration</summary>

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

For simple setups without roles, use env vars:

| Variable | Description |
|---|---|
| `KAPSO_SECURITY_MODE` | `"allowlist"` or `"open"` |
| `KAPSO_ALLOWED_NUMBERS` | Comma-separated phone numbers (all get `default_role`) |
| `KAPSO_DENY_MESSAGE` | Message sent to unauthorized senders |
| `KAPSO_RATE_LIMIT` / `KAPSO_RATE_WINDOW` | Rate limit settings |
| `KAPSO_SESSION_ISOLATION` | `"true"` or `"false"` |

</details>

**Behavior:**

- **Allowlist mode** (default): Only numbers in `[security.roles]` can send messages. Unauthorized senders receive the deny message.
- **Open mode**: Anyone can send. Senders not in the roles map get `default_role`.
- **Rate limiting**: Fixed-window token bucket per sender. Excess messages are silently dropped.
- **Session isolation** (default on): Each sender gets their own OpenClaw session, preventing cross-sender context leakage.

## Delivery modes

#### Polling (default)

Works out of the box — no public endpoint, no domain, no tunnel. Polls every 30 seconds.

#### Tailscale Funnel (zero-config tunnel)

Real-time delivery (< 1s latency) without owning a domain. The bridge starts [Tailscale Funnel](https://tailscale.com/kb/1223/funnel) automatically. Works on the free plan.

```toml
[delivery]
mode = "tailscale"
```

Plus set `KAPSO_WEBHOOK_VERIFY_TOKEN`. Prerequisites: Tailscale installed and running.

#### Your own domain

Point your reverse proxy at the webhook server (`:18790`).

```toml
[delivery]
mode = "domain"
```

Plus set `KAPSO_WEBHOOK_VERIFY_TOKEN` (and optionally `KAPSO_WEBHOOK_SECRET` for HMAC validation). Register `https://yourdomain.com/webhook` in Kapso.

> **Polling fallback:** Any webhook mode can also run polling as a safety net by setting `poll_fallback = true`. Messages are deduplicated by ID.

## Voice transcription

Incoming voice notes are automatically transcribed and forwarded as `[voice] <transcript>`. If transcription is not configured or fails, the message is forwarded as `[audio] (audio/ogg)` instead. No messages are ever lost.

<details>
<summary>Transcription configuration</summary>

### Cloud providers

```toml
[transcribe]
provider = "groq"           # "groq" | "openai" | "deepgram"
api_key = ""                # prefer KAPSO_TRANSCRIBE_API_KEY env var
```

| Provider | Default model | Base URL |
|----------|--------------|----------|
| `groq` | `whisper-large-v3` | `api.groq.com/openai/v1` |
| `openai` | `whisper-1` | `api.openai.com/v1` |
| `deepgram` | `nova-3` | `api.deepgram.com/v1` |

All cloud providers include automatic retry on 429/5xx (3 attempts, exponential backoff).

### Local provider (whisper.cpp)

No API costs. Requires [whisper.cpp](https://github.com/ggerganov/whisper.cpp) and ffmpeg installed.

```toml
[transcribe]
provider = "local"
binary_path = "whisper-cli"          # path to whisper-cli binary
model_path = "/path/to/ggml-base.bin"
```

### Full reference

```toml
[transcribe]
provider = ""               # "groq" | "openai" | "deepgram" | "local" (empty = disabled)
api_key = ""                # API key for cloud providers
model = ""                  # override default model per provider
language = ""               # language hint (empty = auto-detect)
max_audio_size = 26214400   # max download size in bytes (default 25MB)
binary_path = "whisper-cli" # local provider only
model_path = ""             # local provider only
timeout = 30                # per-call timeout in seconds (cloud providers)
no_speech_threshold = 0.85  # silence detection — above this, falls back to [audio]
cache_ttl = 3600            # transcript cache lifetime in seconds (SHA-256 keyed)
debug = false               # log avg_logprob, no_speech_prob, detected language
```

Minimal cloud setup with env vars only:

```bash
export KAPSO_TRANSCRIBE_PROVIDER="groq"
export KAPSO_TRANSCRIBE_API_KEY="your-key"
```

</details>

## Development

### Prerequisites

- Go 1.22+
- (Optional) [just](https://github.com/casey/just) command runner

### Building and testing

```bash
# From source
go install github.com/Enriquefft/openclaw-kapso-whatsapp/cmd/kapso-whatsapp-cli@latest
go install github.com/Enriquefft/openclaw-kapso-whatsapp/cmd/kapso-whatsapp-bridge@latest

# With just
just build          # Build both binaries
just test           # Run tests
just lint           # Run golangci-lint
just check          # Run tests + vet + format check
```

If you use Nix, `direnv allow` or `nix develop` gives you Go, gopls, golangci-lint, goreleaser, and just.

### Project structure

```
cmd/
  kapso-whatsapp-cli/       CLI for sending messages and preflight checks
  kapso-whatsapp-bridge/    Receives inbound messages (polling, tailscale, or domain)
internal/
  config/                   TOML config loading with env var overrides
  kapso/                    Kapso API client, message types, list endpoint
  gateway/                  WebSocket client to OpenClaw gateway
  delivery/                 Source abstraction, fan-in merge, dedup, extraction
    poller/                 Polling source
    webhook/                HTTP webhook source
  relay/                    Relay agent replies back to WhatsApp
  security/                 Allowlist, rate limiting, role tagging, session isolation
  transcribe/               Voice transcription providers and caching
  preflight/                Setup verification checks
  tailscale/                Tailscale Funnel automation (auto-start, URL discovery)
scripts/
  install.sh                Curl-pipe-bash installer with checksum verification
nix/
  module.nix                Home-manager module with typed options + sops-nix support
skills/
  whatsapp/                 SKILL.md — agent instructions
```

## Contributing

Issues and PRs welcome. Run `just check` before submitting.

## License

MIT
