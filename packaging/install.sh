#!/usr/bin/env bash
set -euo pipefail

APP="vps-watchdog"
RUN_USER="vpswatchdog"
RUN_GROUP="vpswatchdog"

BIN_SRC=""
BIN_DST="/usr/local/bin/vps-watchdog"
CONFIG_DIR="/etc/vps-watchdog"
STATE_DIR="/var/lib/vps-watchdog"
SERVICE_FILE="/etc/systemd/system/vps-watchdog.service"
SERVICE_TEMPLATE="$(dirname "$0")/vps-watchdog.service.tmpl"
LIBEXEC_DIR="/usr/local/libexec/vps-watchdog"
PAM_HOOK="$LIBEXEC_DIR/pam-login-event"

usage() {
  cat <<EOF
Usage:
  sudo ./packaging/install.sh --bin ./dist/vps-watchdog-linux-amd64

Options:
  --bin PATH     Path to a built linux/amd64 vps-watchdog binary.
  -h, --help     Show help.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bin)
      BIN_SRC="${2:-}"
      shift 2
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
  echo "Run as root: sudo ./packaging/install.sh --bin ./dist/vps-watchdog-linux-amd64" >&2
  exit 1
fi

if [[ -z "$BIN_SRC" || ! -f "$BIN_SRC" ]]; then
  echo "--bin is required and must point to an existing binary" >&2
  exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
  echo "systemctl not found; vps-watchdog currently requires systemd" >&2
  exit 1
fi

LOGGER_BIN="$(command -v logger || true)"
if [[ -z "$LOGGER_BIN" ]]; then
  echo "logger not found; install bsdutils/util-linux equivalent first" >&2
  exit 1
fi

for tool in od tr; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "$tool not found; vps-watchdog PAM hook requires POSIX text utilities" >&2
    exit 1
  fi
done

if ! command -v runuser >/dev/null 2>&1; then
  echo "runuser not found; vps-watchdog installer requires util-linux" >&2
  exit 1
fi

file_has_value() {
  [[ -f "$1" ]] && grep -q '[^[:space:]]' "$1"
}

prompt_secret_file() {
  local path="$1"
  local prompt="$2"
  local hide_input="$3"
  local value=""
  local tmp=""

  printf "%s" "$prompt"
  if [[ "$hide_input" == "true" ]]; then
    trap 'stty echo 2>/dev/null || true' EXIT INT TERM
    stty -echo || true
  fi
  if ! read -r value; then
    if [[ "$hide_input" == "true" ]]; then
      stty echo || true
      printf '\n'
      trap - EXIT INT TERM
    fi
    echo "failed to read secret value: $path" >&2
    exit 1
  fi
  if [[ "$hide_input" == "true" ]]; then
    stty echo || true
    printf '\n'
    trap - EXIT INT TERM
  fi
  if [[ -z "${value//[[:space:]]/}" ]]; then
    echo "secret value must not be empty: $path" >&2
    exit 1
  fi
  tmp="$(mktemp)"
  printf "%s" "$value" > "$tmp"
  install -m0640 -o root -g "$RUN_GROUP" "$tmp" "$path"
  rm -f "$tmp"
}

if ! command -v journalctl >/dev/null 2>&1; then
  echo "journalctl not found; vps-watchdog currently requires systemd-journald" >&2
  exit 1
fi

if [[ ! -f "$SERVICE_TEMPLATE" ]]; then
  echo "service template not found: $SERVICE_TEMPLATE" >&2
  exit 1
fi

nologin_shell="/usr/sbin/nologin"
if [[ ! -x "$nologin_shell" && -x "/sbin/nologin" ]]; then
  nologin_shell="/sbin/nologin"
fi

if ! getent group "$RUN_GROUP" >/dev/null 2>&1; then
  groupadd --system "$RUN_GROUP"
fi

if ! id "$RUN_USER" >/dev/null 2>&1; then
  useradd \
    --system \
    --no-create-home \
    --home-dir /nonexistent \
    --shell "$nologin_shell" \
    --gid "$RUN_GROUP" \
    "$RUN_USER"
else
  usermod \
    --home /nonexistent \
    --shell "$nologin_shell" \
    --gid "$RUN_GROUP" \
    "$RUN_USER"
fi
passwd -l "$RUN_USER" >/dev/null 2>&1 || true

install -Dm0755 "$BIN_SRC" "$BIN_DST"

install -d -m0750 "$CONFIG_DIR"
install -d -m0750 "$STATE_DIR"
install -d -m0755 "$LIBEXEC_DIR"

chown root:"$RUN_GROUP" "$CONFIG_DIR"
chown "$RUN_USER":"$RUN_GROUP" "$STATE_DIR"

if [[ ! -f "$CONFIG_DIR/config.yaml" ]]; then
  cat > "$CONFIG_DIR/config.yaml" <<EOF
host_id: "$(hostname)"

telegram:
  token_file: /etc/vps-watchdog/telegram.token
  chat_id_file: /etc/vps-watchdog/telegram.chat_id

login:
  enabled: true

sudo:
  enabled: true

load:
  enabled: true
  sample_interval_sec: 15
  cooldown_sec: 900

  cpu_busy_percent: 90
  cpu_duration_sec: 180

  mem_available_percent: 10
  mem_duration_sec: 180

  psi_io_some_avg60: 20
  io_duration_sec: 180
EOF
fi
chown root:"$RUN_GROUP" "$CONFIG_DIR/config.yaml"
chmod 0640 "$CONFIG_DIR/config.yaml"

if [[ -f "$CONFIG_DIR/telegram.token" ]]; then
  chown root:"$RUN_GROUP" "$CONFIG_DIR/telegram.token"
  chmod 0640 "$CONFIG_DIR/telegram.token"
fi

if [[ -f "$CONFIG_DIR/telegram.chat_id" ]]; then
  chown root:"$RUN_GROUP" "$CONFIG_DIR/telegram.chat_id"
  chmod 0640 "$CONFIG_DIR/telegram.chat_id"
fi

if ! file_has_value "$CONFIG_DIR/telegram.token"; then
  prompt_secret_file "$CONFIG_DIR/telegram.token" "Telegram bot token: " "true"
fi

if ! file_has_value "$CONFIG_DIR/telegram.chat_id"; then
  prompt_secret_file "$CONFIG_DIR/telegram.chat_id" "Telegram chat_id: " "false"
fi

chown root:"$RUN_GROUP" "$CONFIG_DIR/config.yaml" "$CONFIG_DIR/telegram.token" "$CONFIG_DIR/telegram.chat_id"
chmod 0640 "$CONFIG_DIR/config.yaml" "$CONFIG_DIR/telegram.token" "$CONFIG_DIR/telegram.chat_id"

tmp_hook="$(mktemp)"
sed "s#__LOGGER__#$LOGGER_BIN#" "$(dirname "$0")/pam-login-event" > "$tmp_hook"
install -m0755 "$tmp_hook" "$PAM_HOOK"
rm -f "$tmp_hook"
chown root:root "$PAM_HOOK"

"$BIN_DST" validate-config --config "$CONFIG_DIR/config.yaml"
runuser -u "$RUN_USER" -- test -r "$CONFIG_DIR/config.yaml"
runuser -u "$RUN_USER" -- test -r "$CONFIG_DIR/telegram.token"
runuser -u "$RUN_USER" -- test -r "$CONFIG_DIR/telegram.chat_id"
runuser -u "$RUN_USER" -- "$BIN_DST" validate-config --config "$CONFIG_DIR/config.yaml"
runuser -u "$RUN_USER" -- "$BIN_DST" test-notify --config "$CONFIG_DIR/config.yaml" || true

PAM_LINE="session optional pam_exec.so quiet type=open_session $PAM_HOOK"
for pam_file in /etc/pam.d/sshd /etc/pam.d/login; do
  if [[ -f "$pam_file" ]] && ! grep -Fq "$PAM_LINE" "$pam_file"; then
    backup="$pam_file.vps-watchdog.bak"
    if [[ ! -f "$backup" ]]; then
      cp "$pam_file" "$backup"
    fi
    printf '\n# vps-watchdog login alert\n%s\n' "$PAM_LINE" >> "$pam_file"
  fi
done

supp_groups=()
getent group systemd-journal >/dev/null 2>&1 && supp_groups+=("systemd-journal")
getent group adm >/dev/null 2>&1 && supp_groups+=("adm")

supp_line=""
if [[ "${#supp_groups[@]}" -gt 0 ]]; then
  supp_line="SupplementaryGroups=${supp_groups[*]}"
fi

tmp_service="$(mktemp)"
sed "s#__SUPPLEMENTARY_GROUPS__#$supp_line#" "$SERVICE_TEMPLATE" > "$tmp_service"
install -m0644 "$tmp_service" "$SERVICE_FILE"
rm -f "$tmp_service"

systemctl daemon-reload
if systemctl is-active --quiet vps-watchdog.service; then
  systemctl enable vps-watchdog.service
  systemctl restart vps-watchdog.service
else
  systemctl enable --now vps-watchdog.service
fi

echo "Installed vps-watchdog."
echo "Status: systemctl status vps-watchdog.service"
