# Security

## Reporting a vulnerability

Do not put exploit details, secrets, or private hostnames in public issues. Use
GitHub private vulnerability reporting if it is enabled for the repository;
otherwise open a minimal public issue asking for a private contact.

## Purpose and limits

`vps-watchdog` gives fast off-host visibility into suspicious activity on a VPS.
It detects; it does not prevent, and it is not an audit log.

It can see what passes through PAM, journald, and `/proc` on the host while the
agent is intact. It cannot see:

- access through the cloud provider's panel or hypervisor;
- kernel-level rootkits or anything else that hides from PAM and journald;
- activity after an attacker with root has disabled or altered the agent;
- network-level compromise that leaves no local trace.

## Trust boundary

Local root is trusted. An attacker who reaches root can stop the service, edit
PAM, replace the binary, and read the bot token. The design goal is to get
evidence off-host before that point, not to survive it.

Assumptions: systemd and journald are present, `/proc` is readable, the
installer is run by the legitimate administrator, and nobody logs in as the
runtime user.

## Principles

**Never lock out the administrator.** The PAM hook is attached as
`session optional`, does no network I/O, only writes a local journal entry, and
always exits successfully. A monitoring failure must never block a login.

**Never let the network gate detection.** Service startup, watchers, and event
handling do not wait on network availability. Delivery is best-effort; see the
failure model in [architecture.md](architecture.md#failure-model).

**Least privilege.** The agent runs as a locked system user with no login shell.
It gets read access to journals and its config, write access to its state
directory, and nothing else; the systemd unit is sandboxed accordingly. Only the
installer runs as root. See
[ADR 0004](adr/0004-dedicated-runtime-user.md).

**Secrets live in files, not in config, flags, or logs.**

- The bot token and chat id are separate files readable only by root and the
  runtime group. The installer enforces this on every run.
- The token is never printed, passed as a CLI flag, or logged — including inside
  a full Telegram API URL. Delivery errors are sanitized because Go network
  errors embed the request URL.
- No real secrets, hostnames, IP addresses, or chat ids in examples, tests,
  docs, or issues. `make check-docs` scans for token-shaped strings.

**Host input is untrusted.** PAM values and journal entries can be influenced by
an attacker. They are encoded at the source where possible, bounded in size,
parsed defensively, and HTML-escaped before rendering into Telegram messages.

**Installers are auditable.** Install and uninstall are idempotent, repair
ownership and permissions of what they own, keep a backup before editing PAM
files, and remove only what they created.

## Changes that need extra care

Describe the security impact in the pull request for any change to:

- anything executed from PAM or with root;
- systemd unit sandboxing and supplementary groups;
- how secrets are read, stored, or reported in errors;
- parsing of journal or PAM input;
- message rendering.
