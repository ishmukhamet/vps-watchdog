# ADR 0009: Detect reboots by boot id and report how the host went down

## Status

Accepted.

## Context

An unexpected reboot is the main visible trace of several things the agent
cannot observe directly: a boot into the provider's rescue system, a forced
reset from the control panel, a kernel crash, or host maintenance. See the
limits in [SECURITY.md](../SECURITY.md#purpose-and-limits).

The agent announces every start with the same message. An update, a crash
restart, and a reboot behind the administrator's back look identical, and the
message says nothing about how long the host was away. The announcement is also
sent once, at boot, when the network is least likely to be ready, and is lost if
that attempt fails ([ADR 0005](0005-collection-independent-of-delivery.md)).

## Decision

On start the agent compares the kernel boot id from
`/proc/sys/kernel/random/boot_id` with the one saved in its state directory.

- Same boot id: an agent restart, reported as before.
- Different boot id: a reboot, reported as a separate warning-level event with
  two facts:
  - whether the previous run ended cleanly. The agent writes a marker on
    graceful stop and removes it on start; a missing marker means the agent was
    killed with the host (forced reset, power loss, crash);
  - approximate downtime, from a "last alive" timestamp the agent refreshes in
    its state directory while it runs.
- No saved boot id (first install, wiped state): the value is recorded and no
  reboot is reported, in line with
  [ADR 0007](0007-journal-cursor-start-at-now.md).
- The reboot event is kept in the state directory as pending and retried until
  it is delivered. It is not subject to the drop-on-failure rule of the event
  queue, because it is produced exactly when delivery is least reliable.

The agent reports facts and does not guess the cause: the administrator knows
whether a reboot was planned.

Alternatives considered:

- *Uptime from `/proc/uptime`.* Needs a threshold to tell a reboot from an
  agent restart shortly after boot, and gives no information about the previous
  run.
- *Boot list from journald or `wtmp`.* Depends on log retention and formats that
  differ between images; the boot id is one stable kernel value.
- *Hooking systemd shutdown targets.* More installer surface for the same
  information the clean-stop marker gives.

## Consequences

- A reboot nobody planned stands out, and "unclean, away for 40 minutes" reads
  differently from "clean, away for 30 seconds".
- Every planned reboot produces an alert too. This is intended.
- Downtime is approximate: it depends on the refresh interval and on the wall
  clock, which can be wrong early in boot.
- The state directory gets periodic small writes.
- Not detected: live migration, snapshots, and memory inspection, which do not
  reboot the guest. An attacker with offline disk access can edit the saved
  state to hide a boot; liveness monitoring
  ([ADR 0008](0008-external-liveness-monitoring.md)) covers the downtime itself.
- Whatever ran while another system was booted stays invisible; only the gap is
  reported.
