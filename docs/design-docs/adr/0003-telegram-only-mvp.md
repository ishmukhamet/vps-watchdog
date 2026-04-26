# ADR 0003: Telegram-only notifications for MVP

## Status

Accepted.

## Context

The owner currently wants only Telegram alerts. Adding a generic notification
framework too early would increase complexity.

## Decision

Support Telegram Bot API `sendMessage` only for the MVP, behind a small internal
notifier interface.

## Consequences

- The MVP remains small.
- The notifier interface can later support email, webhook, Slack, or Matrix.
- Telegram token handling is security-sensitive and must stay out of logs.
