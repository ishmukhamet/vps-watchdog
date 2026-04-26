# ADR 0001: Use a systemd system service

## Status

Accepted.

## Context

The agent must run automatically when the VPS boots and must not depend on an
interactive user session.

## Decision

Install `vps-watchdog` as a systemd system service at:

```text
/etc/systemd/system/vps-watchdog.service
```

Enable it with:

```bash
systemctl enable --now vps-watchdog.service
```

## Consequences

- The agent starts at boot.
- The agent is managed with standard systemd tools.
- The installer requires root.
- Non-systemd platforms are out of scope for the MVP.
