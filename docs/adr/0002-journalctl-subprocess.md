# ADR 0002: Read journald through a journalctl subprocess

## Status

Accepted.

## Context

Native journald integration often requires cgo or external bindings. The project
wants a simple static `linux/amd64` Go binary for the first release.

## Decision

Use `journalctl` as a subprocess with JSON output. The agent stores the
journal cursor itself and passes it back on the next start
([ADR 0007](0007-journal-cursor-start-at-now.md)).

## Consequences

- No cgo or libsystemd dependency is needed.
- The implementation is easy to debug manually.
- Hosts must have `journalctl` available.
- The agent must handle subprocess exit and restart watchers.
