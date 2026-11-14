# ADR 0010: Alert on low disk space

## Status

Accepted.

## Context

A full disk is one of the most common ways a small VPS breaks, and it is easy
to see coming. It is an operational signal, not a security one, but it sits
next to the existing load alerts and it affects the agent itself: with no free
space journald stops accepting entries and the agent cannot save its state.

## Decision

The agent periodically checks free space with `statfs` and alerts when it falls
below a threshold.

- Both free blocks and free inodes are checked: running out of inodes fails the
  same way and `df` without flags does not show it.
- Free space is measured as what is available to unprivileged processes, the
  same number `df` reports, so the alert matches what the administrator sees.
- The root filesystem is watched by default. More mount points can be listed in
  the config as one comma-separated value, because the config format has no
  lists ([configuration.md](../configuration.md#format)). Paths on the same
  filesystem are reported once.
- Alerting follows the load alerts: a breach must hold for a duration, repeats
  are limited by the cooldown, and recovery is reported
  ([architecture.md](../architecture.md#signals)).
- Thresholds are percentages. Events show absolute free space as well.
- The systemd sandbox is not relaxed for this signal. A path the sandboxed
  agent cannot see, such as a separate filesystem under `/home`, is rejected at
  config validation instead of being measured wrongly.

Alternatives considered:

- *Discover filesystems from `/proc/self/mountinfo`.* No configuration, but it
  needs rules for pseudo, read-only, and container filesystems, and those rules
  go wrong silently. An explicit list is predictable.
- *Predict time until full from the growth rate.* More useful in theory; in
  practice noisy on small disks with log rotation. A threshold is enough to act
  on.
- *Leave it to a general monitoring system.* The target host has none; that is
  why this agent exists.

## Consequences

- The administrator learns about a filling disk before services fail, including
  the agent's own dependencies.
- The agent takes on a signal unrelated to suspicious activity. New operational
  signals still need their own justification; this ADR does not make the agent
  a general monitoring tool.
- Filesystems hidden by the sandbox cannot be watched without a separate
  decision about the unit's hardening.
- A percentage threshold is coarse on very large or very small disks; the
  absolute value in the event compensates.
