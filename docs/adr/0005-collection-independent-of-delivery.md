# ADR 0005: Collection is independent of delivery

## Status

Accepted.

## Context

On a real VPS, outbound network and Telegram can be slow or unavailable,
especially at boot. Early versions ordered the service after
`network-online.target` and sent notifications synchronously from event
handlers, so a network problem could delay or stall login and sudo watching —
exactly when it matters most.

## Decision

- The service does not wait for network availability.
- Local watchers start before any network activity, including the startup alert.
- Watchers hand events to a bounded in-memory queue; a separate sender delivers
  them. When the queue is full, new events are dropped and logged locally.
- The installer's test notification is best-effort and does not abort
  installation.

## Consequences

- A network outage cannot block detection.
- Alerts can be lost during a long outage or after a restart while events are
  queued. A durable queue would close this gap.
- Local service logs remain the fallback record of dropped events.
