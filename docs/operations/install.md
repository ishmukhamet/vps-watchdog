# Installation

## Build

From the repository root:

```bash
make build
```

This creates:

```text
dist/vps-watchdog-linux-amd64
```

## Install

On the target VPS:

```bash
sudo ./packaging/install.sh --bin ./dist/vps-watchdog-linux-amd64
```

The installer expects `systemctl`, `journalctl`, `logger`, `runuser`, `od`, and
`tr` to be available on the host. On Debian/Ubuntu, `runuser`, `logger`, `od`,
and `tr` are provided by standard base utilities.

## Installed files

```text
/usr/local/bin/vps-watchdog
/usr/local/libexec/vps-watchdog/pam-login-event
/etc/vps-watchdog/config.yaml
/etc/vps-watchdog/telegram.token
/etc/vps-watchdog/telegram.chat_id
/etc/systemd/system/vps-watchdog.service
/var/lib/vps-watchdog/
```

On every install or reinstall, the installer enforces `root:vpswatchdog` and
`0640` on config and Telegram secret files while preserving existing contents.
Empty credential files are rejected and re-prompted before service installation
continues. Newly prompted credential files are written through a restrictive
temporary file and installed with `0640` permissions.

## Runtime user

The installer creates:

```text
vpswatchdog
```

This is a locked system user, not an account for interactive login.
If the user or group already exists, the installer repairs the primary group,
home, shell, and locked-password state to match the runtime model.

## Service lifecycle

```bash
sudo systemctl status vps-watchdog.service
sudo systemctl restart vps-watchdog.service
sudo systemctl enable vps-watchdog.service
sudo systemctl disable vps-watchdog.service
```

On first start, login and sudo journal watchers begin at the current time when no
cursor file exists yet. This avoids sending alerts for old retained journal
entries during installation. After that, cursor files under
`/var/lib/vps-watchdog` are used to resume watching across restarts.

The installer validates the config, verifies that the `vpswatchdog` runtime user
can read the config and Telegram secret files, and sends a best-effort Telegram
test message before starting the systemd service. Test notification failure is
reported but does not abort installation, because the host may temporarily lack
outbound network access or Telegram credentials may need bot-side correction.

On reinstall, an already-running service is restarted so the copied binary and
rendered service file take effect immediately.

## Uninstall

```bash
sudo ./packaging/uninstall.sh
```

The uninstaller removes service files and owned runtime files. It preserves the
runtime user by default unless called with `--remove-user`.
