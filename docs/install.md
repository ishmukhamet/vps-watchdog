# Installation

Requirements: Linux `amd64` with systemd and journald, root access, and the
tools the installer checks for (`systemctl`, `journalctl`, `logger`, `runuser`,
`od`, `tr`; all part of standard Debian/Ubuntu base images). You also need a
Telegram bot token and the chat id to send alerts to.

## From a release (recommended)

```bash
curl -fsSLO https://raw.githubusercontent.com/ishmukhamet/vps-watchdog/master/scripts/install-release.sh
sh install-release.sh latest   # or a tag such as v0.1.0
```

Run it as a regular user. It downloads the release archive and `checksums.txt`,
verifies the checksum, extracts the archive, and only then calls the bundled
installer through `sudo`. See
[ADR 0006](adr/0006-local-only-privileged-installer.md) for why the
steps are split.

Manual equivalent:

```bash
VERSION=v0.1.0
curl -fsSLO "https://github.com/ishmukhamet/vps-watchdog/releases/download/${VERSION}/vps-watchdog-linux-amd64.tar.gz"
curl -fsSLO "https://github.com/ishmukhamet/vps-watchdog/releases/download/${VERSION}/checksums.txt"
sha256sum -c --ignore-missing checksums.txt
mkdir -p vps-watchdog-install
tar -xzf vps-watchdog-linux-amd64.tar.gz -C vps-watchdog-install
cd vps-watchdog-install
sudo ./packaging/install.sh --bin ./vps-watchdog
```

## From source

```bash
make build    # produces dist/vps-watchdog-linux-amd64
sudo ./packaging/install.sh --bin ./dist/vps-watchdog-linux-amd64
```

## What the installer does

The installer is idempotent: rerunning it upgrades the binary and repairs the
installation without losing configuration.

- Creates or repairs the locked system user and group `vpswatchdog` (no home,
  no login shell, locked password).
- Writes a default config if none exists; existing config is kept.
- Prompts for the Telegram token and chat id if their files are missing or
  empty.
- Enforces `root:vpswatchdog` and mode `0640` on config and credential files,
  including the liveness ping URL file if it exists. Liveness pings are off by
  default; see [configuration.md](configuration.md#liveness-pings).
- Adds a `session optional` PAM line for the login hook to `sshd` and `login`
  PAM services, keeping a `.vps-watchdog.bak` copy of each file before the first
  edit.
- Installs the systemd unit, adding journal-reading groups that exist on the
  host.
- Validates the config, checks that the runtime user can read it and the
  credentials, and sends a test alert. A failed test alert is reported but does
  not abort installation.
- Enables the service and starts or restarts it.

Installed paths:

```text
/usr/local/bin/vps-watchdog
/usr/local/libexec/vps-watchdog/pam-login-event
/etc/vps-watchdog/config.yaml
/etc/vps-watchdog/telegram.token
/etc/vps-watchdog/telegram.chat_id
/etc/vps-watchdog/liveness.url        # optional, created by the administrator
/etc/systemd/system/vps-watchdog.service
/var/lib/vps-watchdog/
```

After the first start, alerts cover only new events; older journal history is
not replayed
([ADR 0007](adr/0007-journal-cursor-start-at-now.md)).

## Uninstall

```bash
sudo ./packaging/uninstall.sh [--purge-config] [--remove-user]
```

Stops and removes the service, the PAM lines, the binary, the hook, and the state
directory. Config and credentials in `/etc/vps-watchdog` are kept unless
`--purge-config` is given; the runtime user is kept unless `--remove-user` is
given.
