# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- README overhaul for launch (badges, architecture diagram, full config reference)
- Security controls: sender allowlist, per-sender rate limiting, role tagging, session isolation
- Cross-platform dev tooling: justfile, GitHub Actions CI, GoReleaser config, Nix devShell
- TOML config file support with 3-tier loading (defaults < file < env vars)
- Home-manager NixOS module with sops-nix support
- Three delivery modes: polling (default), Tailscale Funnel (auto-tunnel), domain (reverse proxy)
- Webhook support with automatic Tailscale Funnel setup
- Media message handling (images, audio, video, documents, stickers, location, contacts)
- Relay: auto-read agent session JSONL and send replies back to WhatsApp
- Transparent WhatsApp transport: strip prefix, Markdown-to-WhatsApp formatter, smart message splitter
- Delivery source abstraction with fan-in merge and message deduplication
- CLI tool for sending messages and health checks
- MIT license
- CONTRIBUTING.md and SECURITY.md community files

### Fixed

- Duplicate relay responses from concurrent goroutines
- OpenClaw WebSocket auth (challenge-response, protocol version 3, X-API-Key header)
- Timestamp parsing for Kapso message format
- Module path aligned with GitHub repository
