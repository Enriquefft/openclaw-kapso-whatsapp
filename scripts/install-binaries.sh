#!/usr/bin/env bash
set -euo pipefail

REPO="Enriquefft/openclaw-kapso-whatsapp"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
TAG="${TAG:-}"

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

if [[ -z "$TAG" ]]; then
  TAG="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' \
    | head -n 1)"
fi

if [[ -z "$TAG" ]]; then
  echo "error: failed to resolve release tag (set TAG=vX.Y.Z and retry)" >&2
  exit 1
fi

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  darwin|linux) ;;
  *)
    echo "error: unsupported OS: $(uname -s). Supported: darwin, linux" >&2
    exit 1
    ;;
esac

arch_raw="$(uname -m)"
case "$arch_raw" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "error: unsupported architecture: $arch_raw. Supported: x86_64, arm64" >&2
    exit 1
    ;;
esac

version="${TAG#v}"
asset="openclaw-kapso-whatsapp_${version}_${os}_${arch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${TAG}/${asset}"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

echo "Downloading ${asset}..."
curl -fL "$url" -o "$tmpdir/$asset"

tar -xzf "$tmpdir/$asset" -C "$tmpdir"

for bin in kapso-whatsapp-bridge kapso-whatsapp-cli; do
  if [[ ! -f "$tmpdir/$bin" ]]; then
    echo "error: release archive is missing expected binary: $bin" >&2
    exit 1
  fi
done

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmpdir/kapso-whatsapp-bridge" "$INSTALL_DIR/kapso-whatsapp-bridge"
install -m 0755 "$tmpdir/kapso-whatsapp-cli" "$INSTALL_DIR/kapso-whatsapp-cli"

echo "Installed:"
echo "  $INSTALL_DIR/kapso-whatsapp-bridge"
echo "  $INSTALL_DIR/kapso-whatsapp-cli"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo
    echo "warning: $INSTALL_DIR is not in PATH for this shell."
    echo "add this line to your shell profile:"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac

