# Installation

## Install From Release

For a public release, download and run the release bootstrap script:

```bash
curl -fsSLO https://raw.githubusercontent.com/ishmukhamet/vps-watchdog/master/scripts/install-release.sh
sh install-release.sh latest
```

For a pinned version:

```bash
sh install-release.sh v0.1.0
```

The bootstrap script downloads `vps-watchdog-linux-amd64.tar.gz` and
`checksums.txt` from GitHub Releases, verifies the checksum, extracts the archive,
prints the bundled binary version, and then runs the local installer through
`sudo`.

Manual release install:

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

## Install From Source

From the repository root:

```bash
make build
```

This creates:

```text
dist/vps-watchdog-linux-amd64
```

## Install Built Binary

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
