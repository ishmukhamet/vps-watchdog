# Architecture

## Summary

`vps-watchdog` is a single static Go binary installed on a Linux VPS and run by
systemd as a system service under a dedicated unprivileged system user.

The agent observes local host signals and sends Telegram alerts. The first
implementation intentionally uses boring, inspectable Linux interfaces:

- PAM hook + journald for login events.
- journald/syslog for sudo events.
- `/proc/stat`, `/proc/meminfo`, and `/proc/pressure/io` for load signals.
- systemd for lifecycle management.
- Telegram Bot API for outbound notification delivery.

## Runtime diagram

```text
                  ┌──────────────────────────┐
                  │ Linux host / VPS          │
                  │                          │
SSH / console ───▶│ PAM                       │
                  │  └─ pam_exec hook         │
                  │       └─ logger           │
                  │            └─ journald    │
                  │                          │
sudo ────────────▶│ sudo/syslog ──▶ journald  │
                  │                          │
/proc ───────────▶│ metrics collector         │
                  │                          │
                  │ systemd                  │
                  │  └─ vps-watchdog.service │
                  │       └─ vps-watchdog     │
                  └────────────┬─────────────┘
                               │ HTTPS
                               ▼
                     Telegram Bot API
```

## Process model

The installer creates a locked runtime user:

```text
user:  vpswatchdog
group: vpswatchdog
home:  /nonexistent
shell: /usr/sbin/nologin or /sbin/nologin
```

If the user already exists, the installer repairs its primary group, home, shell,
and locked-password state instead of assuming the existing account is safe.

The service is installed as:

```text
/etc/systemd/system/vps-watchdog.service
```

It is enabled with:

```bash
systemctl enable --now vps-watchdog.service
```

This is a system service. It does not depend on an interactive login session.
It also does not wait for `network-online.target`; local login and sudo watchers
must start without depending on Telegram or outbound network availability.

## Package boundaries

```text
cmd/vps-watchdog/          CLI entrypoint and process wiring
internal/agent/            long-running agent orchestration
internal/config/           config loading, defaults, validation
internal/event/            normalized events and Telegram rendering
internal/journal/          journalctl process management and JSON parsing
internal/metrics/          /proc collectors and threshold sampling primitives
internal/notifier/         notification interface
internal/notifier/telegram Telegram Bot API client
internal/state/            state directory and file helpers
```

Boundary rules:

- `cmd/` wires packages together but should not contain business logic.
- `internal/agent` may depend on all internal packages.
- `internal/config` must not depend on runtime packages.
- `internal/event` must not depend on Telegram.
- `internal/notifier/telegram` must not depend on agent internals.
- `internal/metrics` must be testable with injected readers.
- `internal/journal` must treat journal entries as untrusted input.

## Login detection

The installer appends this PAM line to supported PAM service files, currently
`/etc/pam.d/sshd` and `/etc/pam.d/login`:

```text
session optional pam_exec.so quiet type=open_session /usr/local/libexec/vps-watchdog/pam-login-event
```

The hook is deliberately tiny. It uses `logger` to write a local event with the
identifier `vps-watchdog-login`. It must not call Telegram or perform network
I/O, because monitoring failures must not block logins.

The installed hook uses the `logger` path detected by the installer and
hex-encodes PAM values in the log message. The Go agent decodes those values
after reading journald so untrusted PAM fields cannot inject additional parsed
keys by containing spaces or `key=value` text.

The Go agent follows journald for `SYSLOG_IDENTIFIER=vps-watchdog-login` and
normalizes each line into a login alert.

## Sudo detection

The Go agent follows journald for sudo entries across journals visible to the
runtime user. It matches `_COMM=sudo`, `_EXE=/usr/bin/sudo`, and
`SYSLOG_IDENTIFIER=sudo` because some VPS images expose sudo syslog entries only
outside `journalctl --system`.

Sudo log formats vary across distributions and sudo versions. The MVP classifies
common command execution, authentication failure, and not-in-sudoers denial
messages, while preserving the raw message in the alert fields. PAM session
opened/closed messages are ignored to avoid duplicate noise around one sudo
command.

## Load detection

The load monitor samples:

- CPU busy percentage from deltas in `/proc/stat`.
- memory availability from `MemAvailable` and `MemTotal` in `/proc/meminfo`.
- I/O pressure from `some avg60` in `/proc/pressure/io` when PSI is available.

The alerting model is duration-based:

```text
threshold breached continuously for N seconds -> alert
condition remains breached -> suppress until cooldown
condition recovers -> optional recovery event
```

The MVP contains the sampling primitives and initial alert loop. Fine-tuning is
tracked in `docs/exec-plans/active/0001-mvp-agent.md`.

## State

State lives under:

```text
/var/lib/vps-watchdog
```

Current state files:

- `login.cursor` — journald cursor for login watcher.
- `sudo.cursor` — journald cursor for sudo watcher.

If a cursor file does not exist yet, the watcher starts with `--since=now` to
avoid replaying old retained journal history during first install. Once
`journalctl` writes the cursor file, subsequent starts resume from the stored
cursor.

Future state may include notification queues and durable rate-limit windows.

## Failure handling

- systemd restarts the service with `Restart=always`.
- local watchers are started before the best-effort startup notification so
  Telegram/network delays do not block login or sudo monitoring.
- event handlers enqueue notifications into a bounded in-memory queue instead of
  calling Telegram synchronously; if the queue is full, the event is dropped and
  logged because local collection must continue.
- journal watchers are restarted by the agent if `journalctl` exits.
- journal watchers cancel their `journalctl` subprocess when entry handling fails
  so handler errors do not leave a hanging `journalctl --follow` process.
- bounded `journalctl` stderr is returned with watcher errors so permission and
  filter problems are visible in service logs.
- oversized journald JSON lines are skipped instead of causing a permanent
  watcher restart loop on untrusted input.
- Telegram failures are returned to the caller and logged without token leakage,
  including network errors whose Go error text may contain the request URL.
- Login hooks never fail closed.

## Security boundary

The primary trust boundary is the local root account. If an attacker obtains root,
they can disable the service, alter PAM, modify the binary, or read Telegram
credentials. The design goal is fast off-host alerting before that happens, not
post-root tamper resistance.
