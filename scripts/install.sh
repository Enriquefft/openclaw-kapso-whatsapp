#!/usr/bin/env bash
set -euo pipefail

REPO="juanrestrepot/openclaw-kapso-whatsapp"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
TAG="${TAG:-}"

TMPDIR=""
cleanup() { [ -n "$TMPDIR" ] && rm -rf "$TMPDIR"; }
trap cleanup EXIT

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

need_cmd curl
need_cmd tar
need_cmd install
need_cmd uname
need_cmd mktemp

# --- Detect OS ---
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  darwin|linux) ;;
  *)
    echo "error: unsupported OS: $(uname -s). Supported: darwin, linux" >&2
    exit 1
    ;;
esac

# --- Detect arch ---
arch_raw="$(uname -m)"
case "$arch_raw" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "error: unsupported architecture: $arch_raw. Supported: x86_64, arm64" >&2
    exit 1
    ;;
esac

# --- Resolve release tag ---
if [ -z "$TAG" ]; then
  api_response="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")"
  if command -v jq >/dev/null 2>&1; then
    TAG="$(echo "$api_response" | jq -r '.tag_name')"
  else
    TAG="$(echo "$api_response" | grep '"tag_name"' | sed 's/.*"\(v[^"]*\)".*/\1/' | head -n 1)"
  fi
fi

if [ -z "$TAG" ]; then
  echo "error: failed to resolve release tag (set TAG=vX.Y.Z and retry)" >&2
  exit 1
fi

# --- Build download URLs ---
version="${TAG#v}"
asset="openclaw-kapso-whatsapp_${version}_${os}_${arch}.tar.gz"
base_url="https://github.com/${REPO}/releases/download/${TAG}"

# --- Download to temp dir ---
TMPDIR="$(mktemp -d)"
echo "Downloading ${asset}..."
curl -fSL -o "$TMPDIR/$asset" "$base_url/$asset"
curl -fsSL -o "$TMPDIR/checksums.txt" "$base_url/checksums.txt"

# --- Verify checksum ---
verify_checksum() {
  local expected actual
  expected="$(grep "$asset" "$TMPDIR/checksums.txt" | awk '{print $1}')"
  if [ -z "$expected" ]; then
    echo "error: checksum not found for $asset in checksums.txt" >&2
    exit 1
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$TMPDIR/$asset" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$TMPDIR/$asset" | awk '{print $1}')"
  else
    echo "error: no sha256sum or shasum found; cannot verify checksum" >&2
    exit 1
  fi
  if [ "$actual" != "$expected" ]; then
    echo "error: checksum mismatch for $asset" >&2
    echo "  expected: $expected" >&2
    echo "  actual:   $actual" >&2
    exit 1
  fi
  echo "Checksum verified."
}
verify_checksum

# --- Extract and install ---
tar -xzf "$TMPDIR/$asset" -C "$TMPDIR"

for bin in kapso-whatsapp-bridge kapso-whatsapp-cli; do
  if [ ! -f "$TMPDIR/$bin" ]; then
    echo "error: release archive is missing expected binary: $bin" >&2
    exit 1
  fi
done

mkdir -p "$INSTALL_DIR"
install -m 0755 "$TMPDIR/kapso-whatsapp-bridge" "$INSTALL_DIR/kapso-whatsapp-bridge"
install -m 0755 "$TMPDIR/kapso-whatsapp-cli" "$INSTALL_DIR/kapso-whatsapp-cli"

echo "Installed:"
echo "  $INSTALL_DIR/kapso-whatsapp-bridge"
echo "  $INSTALL_DIR/kapso-whatsapp-cli"

# --- PATH check ---
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo
    echo "warning: $INSTALL_DIR is not in PATH for this shell."
    echo "Add this line to your shell profile:"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac

echo
echo "Run 'kapso-whatsapp-cli preflight' to verify your setup."
