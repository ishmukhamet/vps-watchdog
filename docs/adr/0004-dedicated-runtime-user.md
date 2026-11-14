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
- Reading the journal requires membership in a journal-reading group
  (`systemd-journal` or `adm`, depending on the distribution). That grants read
  access to the whole system journal, not only to login and sudo entries, so a
  compromised agent can read everything other services log. journald has no
  narrower grant.
