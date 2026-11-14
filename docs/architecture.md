# Architecture

`vps-watchdog` is one static Go binary run by systemd as a system service under
a dedicated unprivileged user. It turns local host signals into alerts and
pushes them off-host to Telegram.

Paths, commands, and config keys live in [install.md](install.md) and
[configuration.md](configuration.md). Reasons for individual choices are in the
[ADRs](adr/).

## Design principles

1. **Boring, inspectable interfaces.** PAM, journald, `/proc`, and systemd are
   used directly so that every input can be reproduced by hand with standard
   tools. See [ADR 0002](adr/0002-journalctl-subprocess.md).
2. **Collection is independent of delivery.** Detecting an event must never wait
   on the network. Delivery is best-effort and decoupled from collection. See
   [ADR 0005](adr/0005-collection-independent-of-delivery.md).
3. **Monitoring must not break access.** Anything on the login path does no
   network I/O and cannot fail closed.
4. **Least privilege.** Only the installer runs as root. The agent gets read
   access to what it watches and write access to its own state, nothing more.
   See [ADR 0004](adr/0004-dedicated-runtime-user.md).
5. **Signal over volume.** Every alert should be worth reading on a phone: short
   spikes, replayed history, and duplicate noise are suppressed, and each alert
   carries enough context (host, who, from where, when) to act without logging
   in. When input cannot be parsed reliably, the raw text is sent rather than
   the event dropped.
6. **All host input is untrusted.** PAM values and journal entries are attacker
   influenced. They are encoded at the source where possible, bounded in size,
   and rendered as data, never as markup or commands.

## Data flow

```text
SSH / console ─▶ PAM ─▶ hook ─▶ journald ─┐
sudo ──────────────────────────▶ journald ─┤
/proc (CPU, memory, PSI) ──────────────────┤
                                           ▼
                                 vps-watchdog agent
                       watchers ─▶ normalized events ─▶ bounded queue
                                                            │ HTTPS
                                                            ▼
                                                    Telegram Bot API
```

## Signals

**Logins.** A PAM `session optional` hook writes one local journal entry per
opened session and exits. PAM values are hex-encoded by the hook and decoded by
the agent, so untrusted fields cannot inject extra key/value pairs. The agent
follows those entries in journald.

**Sudo.** Sudo already logs to syslog/journald, so no hook is installed. Where
sudo entries land differs between images, so the watcher matches by several
journal fields across all journals the runtime user can read, not only the
system journal. Messages are classified into broad kinds (command, auth
failure, denial) while the raw message is always kept, because formats vary
across distributions and sudo versions. Per-command PAM session noise is
dropped.

**Load.** CPU, available memory, and PSI I/O pressure are sampled periodically
from `/proc`. Alerts are duration-based to ignore short spikes:

```text
breached continuously for N seconds  -> alert
still breached                       -> repeat no more often than the cooldown
recovered after an alert             -> recovery event
```

PSI is optional; hosts without it simply have no I/O signal.

## Journal cursors

Journal watchers save the cursor of every handled entry in the state
directory, so restarts, including after a forced reset, resume where they
stopped. Without a cursor (first install, wiped state), a watcher starts at the
current end of the journal: replaying retained history would flood the chat
with old events. See [ADR 0007](adr/0007-journal-cursor-start-at-now.md).

## Failure model

The agent prefers degraded operation over stopping:

- systemd restarts the process; the agent itself restarts journal watchers that
  exit.
- Watchers start before any network activity, including the startup alert.
- Events go through a bounded in-memory queue. If delivery stalls long enough
  for the queue to fill, new events are dropped and logged locally; collection
  continues.
- A failing watcher terminates its `journalctl` subprocess rather than leaving it
  orphaned, and includes bounded subprocess stderr in its error so permission
  problems are visible in service logs.
- Oversized or malformed journal entries are skipped, so hostile input cannot
  force a restart loop.
- Delivery errors are sanitized before logging, because Go network errors can
  include the request URL, which contains the bot token.

## Package boundaries

```text
cmd/vps-watchdog/           CLI wiring only, no business logic
internal/agent/             orchestration; may depend on any internal package
internal/config/            loading and validation; no runtime dependencies
internal/event/             normalized events and rendering; no Telegram dependency
internal/journal/           journalctl subprocesses and entry parsing
internal/metrics/           /proc collectors behind injectable readers
internal/notifier/          delivery interface
internal/notifier/telegram/ Telegram client; no dependency on agent internals
internal/state/             state directory helpers
packaging/                  installer, uninstaller, systemd unit, PAM hook
scripts/                    repository checks and release helpers
```

Dependencies point inward: `agent` composes the others, and leaf packages stay
testable without a real host.

## Trust boundary

The boundary is local root. An attacker with root can stop the service, edit PAM,
replace the binary, or read the Telegram credentials. The design aims to get
the evidence off-host before that point; it does not try to resist tampering
after it. Details: [SECURITY.md](SECURITY.md).
