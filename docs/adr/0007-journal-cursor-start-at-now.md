# ADR 0007: Journal watchers start at "now" without a cursor

## Status

Accepted. Amended: the agent stores the cursor itself and seeds it at the end
of the journal; see Decision.

## Context

Journal watchers persist a cursor to resume after restarts. On first install
there is no cursor, and journald often retains weeks of login and sudo history.
Replaying it would flood the chat with stale alerts and bury anything current.

## Decision

When a watcher has no stored cursor, it records the current end of the journal
as its cursor and starts there. Once a cursor exists, it always resumes from
the cursor.

The agent owns the cursor file:

- It saves the cursor after each handled entry, atomically and with `fsync`.
- `journalctl --cursor-file` is not used. journalctl saves that file only on a
  clean exit; the agent stops it with `SIGKILL`, and on systemd 252
  `SIGTERM`, `SIGINT`, and a closed pipe also skip the save. With it, every
  restart looked like a first install.
- The watcher resumes with `--cursor` and skips the entry at the cursor itself.
  On systemd 252, `--after-cursor` drops the first matching entry when the
  cursor points at an entry that does not match, which is the usual case for a
  seeded cursor.
- A cursor that is not in journalctl's format, or that journalctl rejects, is
  renamed to `*.cursor.rejected` and logged; the watcher then starts as if it
  had none.

## Consequences

- First install is quiet.
- Events that happen while the agent is stopped or restarting are alerted on
  the next start, including on a host where no login or sudo has been handled
  yet.
- Events between losing state and the next start are not alerted. Deleting the
  state directory is therefore an explicit "forget history" action.
- If seeding fails, the watcher starts at the current time and seeds again on
  the next start.
- An entry counts as handled once its event is queued, not once it is
  delivered ([ADR 0005](0005-collection-independent-of-delivery.md)). After a
  forced reset at most the last entry can be alerted twice.
- One small synced write per handled entry. Login and sudo entries are rare, so
  the cost is negligible.
- A cursor copied from another host or journal can cause skipped or replayed
  events; see
  [troubleshooting.md](../troubleshooting.md#old-events-after-restoring-state).
