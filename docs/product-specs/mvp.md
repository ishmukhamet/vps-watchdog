# MVP product specification

## Problem

A personal VPS owner wants fast Telegram alerts when someone logs into a server,
uses sudo, or causes sustained resource pressure. The primary goal is to notice
possible unauthorized access quickly enough to investigate or isolate the host.

## Users

Primary user: one technical VPS owner who administers their own Linux servers.

## MVP features

### Login alerts

The agent should notify when a successful PAM-covered session opens.

Minimum fields:

- host id;
- service, for example `sshd` or `login`;
- user;
- remote host when provided by PAM;
- tty when provided;
- timestamp.

### Sudo alerts

The agent should notify on sudo events visible in journald.

Minimum fields:

- host id;
- raw sudo message;
- journal timestamp when available.

Structured command extraction is desirable but not required for the first MVP.

### Load alerts

The agent should notify when resource pressure remains above threshold for the
configured duration.

Signals:

- CPU busy percent;
- memory available percent;
- PSI I/O `some avg60` when available.

### Telegram notification

The MVP supports only Telegram Bot API `sendMessage`.

### Installation

The MVP installer should:

- require root;
- install one `linux/amd64` binary;
- create a dedicated system user;
- install systemd service;
- install PAM hook;
- create config and secret files;
- start and enable the service.

## Acceptance criteria

- `make check` passes.
- `vps-watchdog validate-config` accepts the generated config.
- `vps-watchdog test-notify` sends a Telegram message when credentials are valid.
- The service starts automatically after reboot.
- SSH login produces a login alert on a supported host.
- Sudo usage produces a sudo alert on a supported host.
- Sustained CPU pressure above threshold produces a load alert.

## Out of scope

- Web UI.
- Non-Telegram notifiers.
- ARM builds.
- Cloud provider API monitoring.
- Windows/macOS support.
- Tamper-proof audit logging.
