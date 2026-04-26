#!/usr/bin/env bash
set -euo pipefail

RUN_USER="vpswatchdog"
SERVICE="vps-watchdog.service"
SERVICE_FILE="/etc/systemd/system/vps-watchdog.service"
BIN_DST="/usr/local/bin/vps-watchdog"
CONFIG_DIR="/etc/vps-watchdog"
STATE_DIR="/var/lib/vps-watchdog"
LIBEXEC_DIR="/usr/local/libexec/vps-watchdog"
PAM_HOOK="$LIBEXEC_DIR/pam-login-event"
REMOVE_USER="false"
PURGE_CONFIG="false"

usage() {
  cat <<EOF
Usage:
  sudo ./packaging/uninstall.sh [--remove-user] [--purge-config]

Options:
  --remove-user   Remove the vpswatchdog system user when possible.
  --purge-config  Remove /etc/vps-watchdog. Without this, config and secrets stay.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --remove-user)
      REMOVE_USER="true"
      shift
      ;;
    --purge-config)
      PURGE_CONFIG="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Run as root" >&2
  exit 1
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl disable --now "$SERVICE" >/dev/null 2>&1 || true
fi

rm -f "$SERVICE_FILE"
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi

PAM_LINE="session optional pam_exec.so quiet type=open_session $PAM_HOOK"
for pam_file in /etc/pam.d/sshd /etc/pam.d/login; do
  if [[ -f "$pam_file" ]] && grep -Fq "$PAM_LINE" "$pam_file"; then
    tmp="$(mktemp)"
    { grep -Fv "$PAM_LINE" "$pam_file" || true; } | sed '/# vps-watchdog login alert/d' > "$tmp"
    cat "$tmp" > "$pam_file"
    rm -f "$tmp"
  fi
done

rm -f "$BIN_DST"
rm -rf "$LIBEXEC_DIR"
rm -rf "$STATE_DIR"

if [[ "$PURGE_CONFIG" == "true" ]]; then
  rm -rf "$CONFIG_DIR"
else
  echo "Preserved config directory: $CONFIG_DIR"
fi

if [[ "$REMOVE_USER" == "true" ]] && id "$RUN_USER" >/dev/null 2>&1; then
  userdel "$RUN_USER" || true
fi

echo "Uninstalled vps-watchdog."
