# ADR 0008: External liveness monitoring through a dead man's switch

## Status

Accepted.

## Context

Every alert is pushed from the host. When the host is powered off, cut off from
the network, or the agent is stopped, nothing is sent, and silence looks the
same as a quiet day. This is the cheapest way to blind the agent: it needs no
root in the guest for someone who controls the hypervisor or the network, and
one command for someone who has root.

Absence of a signal can only be noticed from outside the host. Telegram cannot
do that: a bot cannot raise an alert because a message did not arrive.

## Decision

The agent periodically sends an HTTPS request to a URL of an external dead
man's switch service chosen by the administrator. That service, not the agent,
raises the alarm when pings stop, through its own notification channel.

- The feature is opt-in: without a configured URL the agent sends no pings.
- The contract is one plain HTTPS request to a fixed URL per interval, which
  hosted and self-hosted services of this kind accept. The agent is not tied to
  a specific vendor. The grace period and the alarm channel are configured in
  that service.
- The reference service is Healthchecks, hosted (healthchecks.io) or
  self-hosted: the feature is tested against its ping endpoint. Its optional
  start and fail signals are not used.
- The request carries no host data: no events, host id, or metrics.
- The ping URL identifies the check and is a secret. It is stored in its own
  file under the same rules as the bot token and never appears in the config,
  CLI flags, logs, or errors (see [SECURITY.md](../SECURITY.md)).
- Pings run in their own loop, outside the event queue. A slow or failing ping
  never delays collection or alert delivery
  ([ADR 0005](0005-collection-independent-of-delivery.md)).
- A ping is sent only while the enabled journal watchers are running, so that
  it means "the agent is watching", not "the process exists".
- A graceful stop simply stops the pings. There is no "going down" call: an
  attacker could use it to silence the alarm.

Telegram stays the only alert channel
([ADR 0003](0003-telegram-only-mvp.md)): the ping carries no alerts and does not
go through the notifier interface.

Alternatives considered:

- *Periodic "still alive" messages in Telegram.* A person has to notice that a
  message is missing; in practice nobody does, and the chat fills with noise.
- *External probe that polls the host.* Needs an open port on the host, and a
  TCP or ICMP probe shows that the host is up, not that the agent is watching.
- *Own receiving server.* Adds infrastructure to deploy and monitor, which is
  what ADR 0003 avoids.

## Consequences

- A stopped agent, a watcher that cannot read the journal, a host that is down,
  loss of outbound network, and a long boot into a rescue system all become
  visible within the grace period.
- Planned maintenance and provider network outages also raise the alarm. The
  grace period trades detection delay against false alarms.
- Not detected: an attacker who leaves the agent running, and an attacker with
  root or disk access who reads the URL and keeps pinging from elsewhere.
- A third party learns the host's address and its uptime pattern, and becomes a
  dependency: if it is down, liveness monitoring is silently off.
- One more secret file for the installer to own and protect.
