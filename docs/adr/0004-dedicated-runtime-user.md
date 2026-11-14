# ADR 0004: Create a dedicated runtime user

## Status

Accepted.

## Context

Running as root is unnecessary for normal monitoring. Running as a personal admin
account couples the agent to a human identity and weakens process isolation.

## Decision

The installer creates a locked system user named `vpswatchdog` and runs the
systemd service under that account.

## Consequences

- The runtime UID is not an interactive login account.
- The service does not depend on the runtime user's login session.
- File permissions can be scoped to the runtime group.
- Some distributions may need group adjustments for journal access.
