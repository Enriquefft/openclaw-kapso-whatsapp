# CLAUDE.md

## Project

Go daemon bridging WhatsApp (Kapso Cloud API) → OpenClaw AI gateway. Two binaries: `kapso-whatsapp-poller` (main daemon) and `kapso-whatsapp-cli` (send messages/health checks).

## Commands

```bash
just build                            # Build both binaries
just test                             # Run tests
just check                            # Run tests + vet + fmt check
just lint                             # Run golangci-lint
direnv reload                         # After flake.nix changes (Nix devs only)
```

## Structure

```
cmd/kapso-whatsapp-poller/main.go     # Daemon entrypoint
cmd/kapso-whatsapp-cli/main.go        # CLI entrypoint
internal/config/                      # TOML + env config (3-tier: defaults < file < env)
internal/kapso/                       # Kapso HTTP client + webhook types
internal/gateway/                     # WebSocket client to OpenClaw
internal/delivery/                    # Message source abstraction (poller + webhook)
internal/relay/                       # Polls agent session JSONL, sends replies back
internal/security/                    # Allowlist, rate limiting, roles, session isolation
internal/tailscale/                   # Auto Tailscale Funnel for webhooks
nix/module.nix                        # Home-manager module (NixOS users)
skills/whatsapp/SKILL.md              # Agent skill definition
```

## Conventions

- **Go 1.22**, minimal deps (gorilla/websocket, BurntSushi/toml)
- Standard `log` package, no frameworks
- Table-driven tests with dependency injection (e.g., mockable `now()`)
- Errors wrapped with `fmt.Errorf` for context
- Context-based cancellation for all goroutines
- Interfaces for extensibility (`delivery.Source`)
- No globals — all state in structs
- CGO disabled in builds

## CI/CD

- GitHub Actions: CI on push/PR (build, test, vet, fmt check)
- Releases: tag `v*` triggers GoReleaser → prebuilt binaries on GitHub Releases

## Config

Config file: `~/.config/kapso-whatsapp/config.toml`
Required env vars: `KAPSO_API_KEY`, `KAPSO_PHONE_NUMBER_ID`
Delivery modes: `polling` (default), `tailscale`, `domain`
