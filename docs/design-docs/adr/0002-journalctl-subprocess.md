# ADR 0002: Use journalctl subprocess for the MVP

## Status

Accepted.

## Context

Native journald integration often requires cgo or external bindings. The project
wants a simple static `linux/amd64` Go binary for the first release.

## Decision

Use `journalctl` as a subprocess with JSON output and cursor files.

## Consequences

- No cgo or libsystemd dependency is needed.
- The implementation is easy to debug manually.
- Hosts must have `journalctl` available.
- The agent must handle subprocess exit and restart watchers.
