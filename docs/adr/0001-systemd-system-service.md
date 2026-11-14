# ADR 0001: Run as a systemd system service

## Status

Accepted.

## Context

The agent must start at boot and keep running without any interactive session.
Target hosts are mainstream VPS images, which ship systemd.

## Decision

Run `vps-watchdog` as a systemd system service (not a user service, cron job, or
self-daemonizing process). Use systemd for restarts and sandboxing.

## Consequences

- Starts at boot and is managed with standard tooling.
- Restart policy and sandboxing are declared in the unit, not reimplemented.
- Installation requires root.
- Non-systemd hosts are unsupported.
