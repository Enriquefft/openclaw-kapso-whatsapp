#!/usr/bin/env bash
set -euo pipefail

failures=0
warnings=0

ok() {
  echo "[OK]   $1"
}

warn() {
  echo "[WARN] $1"
  warnings=$((warnings + 1))
}

fail() {
  echo "[FAIL] $1"
  failures=$((failures + 1))
}

has_cmd() {
  command -v "$1" >/dev/null 2>&1
}

echo "kapso-whatsapp preflight"
echo "------------------------"

if has_cmd kapso-whatsapp-bridge; then
  ok "kapso-whatsapp-bridge found in PATH"
else
  fail "kapso-whatsapp-bridge not found in PATH (run ./scripts/install-binaries.sh)"
fi

if has_cmd kapso-whatsapp-cli; then
  ok "kapso-whatsapp-cli found in PATH"
else
  fail "kapso-whatsapp-cli not found in PATH (run ./scripts/install-binaries.sh)"
fi

if has_cmd openclaw; then
  ok "openclaw found in PATH"
else
  fail "openclaw CLI not found in PATH (install OpenClaw first)"
fi

if [[ -n "${KAPSO_API_KEY:-}" ]]; then
  ok "KAPSO_API_KEY is set"
else
  fail "KAPSO_API_KEY is missing"
fi

if [[ -n "${KAPSO_PHONE_NUMBER_ID:-}" ]]; then
  ok "KAPSO_PHONE_NUMBER_ID is set"
else
  fail "KAPSO_PHONE_NUMBER_ID is missing"
fi

auth_mode=""
if has_cmd openclaw; then
  if auth_mode="$(openclaw config get gateway.auth.mode 2>/dev/null || true)"; then
    auth_mode="$(echo "$auth_mode" | tr -d '\r\n' | tr '[:upper:]' '[:lower:]')"
  fi
  if [[ -n "$auth_mode" ]]; then
    ok "OpenClaw gateway auth mode: $auth_mode"
  else
    warn "Could not determine OpenClaw gateway auth mode"
  fi
fi

if [[ "$auth_mode" == "token" && -z "${OPENCLAW_TOKEN:-}" ]]; then
  fail "OPENCLAW_TOKEN is required when gateway auth mode is token"
fi

gateway_url="${OPENCLAW_GATEWAY_URL:-ws://127.0.0.1:18789}"
if has_cmd openclaw; then
  cmd=(openclaw gateway health --timeout 5000 --json --url "$gateway_url")
  if [[ -n "${OPENCLAW_TOKEN:-}" ]]; then
    cmd+=(--token "$OPENCLAW_TOKEN")
  fi

  if "${cmd[@]}" >/dev/null 2>&1; then
    ok "OpenClaw gateway is reachable at $gateway_url"
  else
    fail "Cannot reach OpenClaw gateway at $gateway_url (start gateway or fix OPENCLAW_GATEWAY_URL/OPENCLAW_TOKEN)"
  fi
fi

if has_cmd curl; then
  if [[ -n "${KAPSO_API_KEY:-}" && -n "${KAPSO_PHONE_NUMBER_ID:-}" ]]; then
    tmp="$(mktemp)"
    status="$(curl -sS -o "$tmp" -w '%{http_code}' \
      -H "X-API-Key: ${KAPSO_API_KEY}" \
      "https://api.kapso.ai/meta/whatsapp/v24.0/${KAPSO_PHONE_NUMBER_ID}/messages?direction=inbound&limit=1" || true)"

    if [[ "$status" == "200" ]]; then
      ok "Kapso credentials are valid (list messages returned 200)"
    else
      body="$(head -c 300 "$tmp" | tr '\n' ' ')"
      fail "Kapso API check failed (HTTP $status). Response: $body"
    fi
    rm -f "$tmp"
  fi
else
  fail "curl is required for Kapso API preflight checks"
fi

echo
if ((failures > 0)); then
  echo "Preflight failed: $failures issue(s), $warnings warning(s)."
  exit 1
fi

echo "Preflight passed: no blocking issues, $warnings warning(s)."

