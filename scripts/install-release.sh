#!/bin/sh
set -eu

REPO="${VPS_WATCHDOG_REPO:-ishmukhamet/vps-watchdog}"
VERSION="${1:-latest}"
ARCHIVE="vps-watchdog-linux-amd64.tar.gz"
CHECKSUMS="checksums.txt"

usage() {
  cat <<EOF
Usage:
  sh install-release.sh [latest|vX.Y.Z]

Environment:
  VPS_WATCHDOG_REPO  GitHub repository, default: ishmukhamet/vps-watchdog
EOF
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "required command not found: $1" >&2
    exit 1
  fi
}

case "$(uname -s)" in
  Linux) ;;
  *)
    echo "unsupported OS: $(uname -s); vps-watchdog release artifacts target Linux" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64) ;;
  *)
    echo "unsupported architecture: $(uname -m); only linux/amd64 is published" >&2
    exit 1
    ;;
esac

need curl
need grep
need mktemp
need sha256sum
need tar

if [ "$(id -u)" -eq 0 ]; then
  SUDO=""
else
  need sudo
  SUDO="sudo"
fi

case "$VERSION" in
  latest)
    BASE_URL="https://github.com/$REPO/releases/latest/download"
    ;;
  v*)
    BASE_URL="https://github.com/$REPO/releases/download/$VERSION"
    ;;
  *)
    echo "version must be 'latest' or start with 'v', got: $VERSION" >&2
    exit 1
    ;;
esac

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

echo "Downloading $REPO $VERSION release artifacts..."
curl -fLSo "$TMP_DIR/$ARCHIVE" "$BASE_URL/$ARCHIVE"
curl -fLSo "$TMP_DIR/$CHECKSUMS" "$BASE_URL/$CHECKSUMS"

cd "$TMP_DIR"
if ! grep -E "[ *]$ARCHIVE$" "$CHECKSUMS" >/dev/null 2>&1; then
  echo "$CHECKSUMS does not contain $ARCHIVE" >&2
  exit 1
fi
sha256sum -c --ignore-missing "$CHECKSUMS"

mkdir install
tar -xzf "$ARCHIVE" -C install
cd install

if [ ! -f ./vps-watchdog ]; then
  echo "release archive did not contain ./vps-watchdog" >&2
  exit 1
fi
if [ ! -x ./packaging/install.sh ]; then
  echo "release archive did not contain executable ./packaging/install.sh" >&2
  exit 1
fi

chmod +x ./vps-watchdog
./vps-watchdog version

if [ -n "$SUDO" ]; then
  exec "$SUDO" ./packaging/install.sh --bin ./vps-watchdog
fi
exec ./packaging/install.sh --bin ./vps-watchdog
