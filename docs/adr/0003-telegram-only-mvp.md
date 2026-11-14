# ADR 0003: Telegram as the only notifier

## Status

Accepted.

## Context

The target user reads alerts on a phone, and Telegram bots need no server-side
infrastructure. A generic multi-channel notification framework would add
configuration and failure modes before there is a second real use case.

## Decision

Deliver alerts only through the Telegram Bot API, behind a small internal
notifier interface.

## Consequences

- The project stays small; rendering can be tuned for one client.
- The notifier interface takes a message already rendered as Telegram HTML.
  A second channel needs rendering moved behind the interface first; the
  interface alone does not make channels interchangeable.
- The bot token is embedded in API URLs, so URL- and error-handling must keep it
  out of logs (see [SECURITY.md](../SECURITY.md)).
