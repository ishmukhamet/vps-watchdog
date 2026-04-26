# Security policy and threat model

## Security posture

`vps-watchdog` is a personal monitoring agent. It is intended to create fast,
off-host visibility into suspicious VPS activity. It is not a prevention tool and
not a tamper-proof audit system.

## What the project tries to detect

- Successful PAM-covered logins.
- Sudo usage and sudo failures visible in journald.
- Sustained abnormal CPU, memory, or I/O pressure.
- Agent startup and basic notification health.

## What the project does not detect

- Cloud provider panel access before the provider opens a console.
- Hypervisor-level access.
- Kernel-level rootkits.
- Direct tampering by an attacker who already has root.
- Activity that bypasses PAM and journald visibility.
- Network compromise that does not produce local host signals.

## Trust assumptions

The MVP assumes:

- systemd is the init system;
- journald is available;
- `/proc` is readable;
- outbound HTTPS to Telegram is available;
- the installer is run as root by the legitimate administrator;
- the runtime user `vpswatchdog` is not used for interactive login.

## Secret handling

Telegram secrets are stored as files under `/etc/vps-watchdog`:

```text
/etc/vps-watchdog/telegram.token
/etc/vps-watchdog/telegram.chat_id
```

Expected permissions:

```text
owner: root
group: vpswatchdog
mode:  0640
```

Rules:

- Never print the bot token.
- Never log a full Telegram API URL containing the token.
- Never pass the token as a CLI flag.
- Sanitize Telegram send errors before logging or returning them; Go network
  errors can include the request URL.
- Do not include real secrets in examples, tests, docs, issues, or ExecPlans.

## PAM safety

The PAM hook must remain small and non-blocking. It must not perform network I/O.
It should only emit a local log event and exit successfully.

The systemd service, agent startup path, and local event handlers must not wait
on network availability before login and sudo monitoring continues. Telegram
delivery is best-effort and must not gate local event collection.

PAM-provided values are untrusted. The installed hook hex-encodes PAM values
before writing its local journal message so spaces, quotes, and key-like content
cannot reshape parsed alert fields.

The PAM line uses `session optional` so monitoring failure does not lock out the
administrator.

## Runtime user

The service runs as `vpswatchdog`, a locked system user. It is granted only the
minimum practical read access needed for journald/auth logs and state files.

## Reporting vulnerabilities

Please do not include secrets, private hostnames, or exploit details in public
issues. If GitHub private vulnerability reporting is enabled for the repository,
use that channel. Otherwise, open a minimal public issue asking for a private
security contact without disclosing sensitive details.

## Security review checklist

Before a stable release:

- Verify installer idempotency on Debian and Ubuntu LTS.
- Verify no token leakage in logs or errors.
- Verify systemd sandbox settings do not break runtime behavior.
- Verify PAM hook is non-blocking under Telegram/network failures.
- Verify `uninstall.sh` removes only files it owns.
- Add integration tests or VM smoke tests for install/uninstall.
