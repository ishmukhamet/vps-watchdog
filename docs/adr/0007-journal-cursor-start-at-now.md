# ADR 0007: Journal watchers start at "now" without a cursor

## Status

Accepted.

## Context

Journal watchers persist a cursor to resume after restarts. On first install
there is no cursor, and journald often retains weeks of login and sudo history.
Replaying it would flood the chat with stale alerts and bury anything current.

## Decision

When a watcher has no stored cursor, it starts at the current time. Once a
cursor exists, it always resumes from the cursor.

## Consequences

- First install is quiet.
- Events between losing state and the next start are not alerted. Deleting the
  state directory is therefore an explicit "forget history" action.
- A cursor copied from another host or journal can cause skipped or replayed
  events; the runbook covers this case.
