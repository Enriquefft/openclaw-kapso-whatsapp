# Contributing

Thanks for your interest in contributing to openclaw-kapso-whatsapp!

## Dev environment

### Option A: Nix (recommended)

```bash
direnv allow   # or: nix develop
```

This gives you Go, gopls, golangci-lint, goreleaser, and just — nothing else to install.

### Option B: Manual

- Go 1.22+
- [just](https://github.com/casey/just) (optional but recommended)

## Workflow

1. **Small fixes** — open a PR directly.
2. **Large changes** — open an issue first so we can discuss the approach before you invest time.

## Before submitting

```bash
just check   # runs tests + vet + format check
```

All CI checks must pass.

## Security issues

If you find a security vulnerability, **do not open a public issue**. See [SECURITY.md](SECURITY.md) for responsible disclosure instructions.
