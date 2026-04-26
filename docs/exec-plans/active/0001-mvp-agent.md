# MVP agent foundation

## Status

Active.

## Context

The repository needs a working foundation for a Go-based VPS monitoring agent.
The first version should be installable as a systemd service and should establish
clear package boundaries for future LLM-driven development.

## Goal

Create a public-quality initial scaffold that can:

- build one `linux/amd64` binary;
- validate a simple config file;
- send a Telegram test notification;
- start a long-running agent loop;
- follow login and sudo journal streams;
- sample basic load metrics;
- install via Bash as a systemd system service.

## Non-goals

- Perfect distro compatibility.
- Native journald bindings.
- Durable notification queue.
- Full sudo command parser.
- ARM builds.

## Current state

Initial scaffold includes docs, code packages, installer scripts, Makefile, and CI.

## Proposed design

Use one Go binary with subcommands:

```text
vps-watchdog agent
vps-watchdog test-notify
vps-watchdog validate-config
vps-watchdog version
```

Use `journalctl` subprocesses for journald streams and `/proc` readers for
metrics. Use a small notifier interface with Telegram as the only implementation.

## Milestones

- [x] Create repository documentation scaffold.
- [x] Create Go package skeleton.
- [x] Implement config validation and Telegram test command.
- [x] Implement journal watcher foundation.
- [x] Implement `/proc` metrics foundation.
- [x] Implement installer and uninstaller scripts.
- [ ] Smoke test on Debian/Ubuntu VPS.
- [ ] Tune alert threshold behavior after real-host testing.
- [ ] Add durable notification queue.

## Validation plan

```bash
make check
./dist/vps-watchdog-linux-amd64 version
```

On a VPS:

```bash
sudo ./packaging/install.sh --bin ./dist/vps-watchdog-linux-amd64
sudo systemctl status vps-watchdog.service
sudo /usr/local/bin/vps-watchdog test-notify --config /etc/vps-watchdog/config.yaml
```

## Progress log

- 2026-04-26: Initial plan created with repository scaffold.
- 2026-04-26: Hardened Telegram error sanitization, journal watcher shutdown,
  first-start cursor behavior, and metric edge cases discovered during repo
  review.
- 2026-04-26: Changed installer to render the checked-in systemd service
  template instead of carrying a second inline unit definition.
- 2026-04-26: Replaced scaffold module path with
  `github.com/ishmukhamet/vps-watchdog`, repaired installer idempotency gaps,
  hardened PAM value encoding, and bounded long alert fields.
- 2026-04-26: Hardened repository checks to avoid scanning ignored artifacts or
  printing secret contents, and made reinstall restart an active service.
- 2026-04-26: Added empty-secret rejection, runtime-user readability checks,
  oversized journal line skipping, and HTML-safe custom test notifications.
- 2026-04-26: Made prompted secret writes restrictive from creation time and
  reject malformed Telegram success responses.
- 2026-04-26: Removed `network-online.target` ordering and made startup
  notification asynchronous after local watchers are launched.
- 2026-04-26: Added bounded in-memory notification queue so Telegram delivery
  does not block login/sudo journal scanning.
- 2026-04-26: Discovered on a real VPS that sudo entries may be visible only
  without `journalctl --system`; planned sudo watcher matching across accessible
  journals and structured sudo alert classification.
- 2026-04-26: Changed Telegram rendering to compact alerts with
  `[emoji] [hh:mm:ss] [host]` headers and event details in the message body.

## Decision log

- 2026-04-26: Build only `linux/amd64`; ARM is out of scope.
- 2026-04-26: Runtime user is `vpswatchdog`, created by installer.
- 2026-04-26: Use systemd system service to avoid login-session dependency.
- 2026-04-26: Use Telegram only for MVP notifications.
- 2026-04-26: When a journal cursor file is missing, start watchers with
  `--since=now` to avoid replaying old retained login/sudo history after first
  install.
- 2026-04-26: Run config validation before enabling the systemd service; keep
  `test-notify` best-effort so transient Telegram/network failures do not block
  installation.
- 2026-04-26: Treat empty Telegram credential files as installer errors because
  they prevent the agent from starting successfully.
- 2026-04-26: Do not let Telegram/network availability gate login and sudo
  monitoring startup.
- 2026-04-26: Follow sudo via `_COMM=sudo` / `_EXE=/usr/bin/sudo` / strong
  identifier fallback across accessible journals instead of system-journal-only
  matching, because sudo syslog entries can live outside `journalctl --system`.
- 2026-04-26: Use compact Telegram alert headers to keep mobile notifications
  scannable while retaining structured fields below the event title.

## Risks and mitigations

- PAM changes can lock out users if done incorrectly. Mitigation: use `session
  optional`, make hook non-networking, and back up PAM files before editing.
- Sudo logs vary by distro. Mitigation: raw-message-first alerts.
- journal access may need distro-specific groups. Mitigation: installer adds
  existing `systemd-journal` and `adm` groups.

## Completion criteria

- CI passes.
- Installer is smoke-tested on at least one Debian or Ubuntu VPS.
- README install flow matches actual behavior.
- Security limitations are documented.
