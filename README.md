# vps-watchdog

`vps-watchdog` is a small Linux agent that sends Telegram alerts when someone
logs into a VPS, uses `sudo`, or keeps the host under sustained CPU, memory, or
I/O pressure. The goal is to notice possible unauthorized access quickly enough
to investigate or isolate the host.

> **No warranty.** This is a personal security monitoring project. It does not
> provide intrusion prevention, forensic integrity, tamper-proof logging, or any
> guarantee that compromise will be detected. If an attacker obtains root, they
> can disable or modify this agent. Use it as one defense-in-depth signal, not as
> your only control.

**Status: pre-alpha.** Not yet validated across real VPS images; expect rough
edges.

## Scope

Alerts on:

- successful PAM-covered logins (SSH, console);
- `sudo` commands, authentication failures, and denials;
- sustained high CPU, low available memory, and Linux PSI I/O pressure;
- agent start and stop.

Not a goal: blocking access, EDR, tamper resistance after root compromise,
cloud control-panel monitoring, notifiers other than Telegram, or replacing
hardening, backups, and centralized logging.

Requirements: Linux with systemd and journald, `linux/amd64`, outbound HTTPS to
the Telegram Bot API, and a Telegram bot token plus chat id.

## Install

```bash
curl -fsSLO https://raw.githubusercontent.com/ishmukhamet/vps-watchdog/master/scripts/install-release.sh
sh install-release.sh latest   # or a pinned tag, e.g. v0.1.0
```

The script downloads and verifies the release without root, then runs the
bundled installer through `sudo`. The installer prompts for Telegram credentials,
installs a systemd service and a PAM hook, and sends a test alert.

Manual install, building from source, and uninstall:
[docs/install.md](docs/install.md).

## Use

```bash
vps-watchdog validate-config --config /etc/vps-watchdog/config.yaml
vps-watchdog test-notify --config /etc/vps-watchdog/config.yaml
```

- Configuration and thresholds: [docs/configuration.md](docs/configuration.md)
- Troubleshooting: [docs/troubleshooting.md](docs/troubleshooting.md)

## How it works and security model

The agent runs as a locked system user, reads journald and `/proc`, and pushes
alerts off-host. It cannot protect itself from an attacker who already has root.

- [docs/architecture.md](docs/architecture.md) — design principles, data flow,
  failure model, package boundaries
- [docs/SECURITY.md](docs/SECURITY.md) — threat model, security rules, reporting
  vulnerabilities
- [docs/adr/](docs/adr/) — why individual decisions were made

## Contributing

Start with [AGENTS.md](AGENTS.md); it applies to human contributors as well.
`make check` must pass before a change is proposed.

## License

MIT. See [LICENSE](LICENSE).
