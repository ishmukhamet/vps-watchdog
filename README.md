# vps-watchdog

`vps-watchdog` is a small Linux security monitoring agent for personal VPS hosts.
It sends Telegram alerts for successful logins, sudo activity, and sustained CPU,
memory, or I/O pressure.

The project is personal-first, but the repository is maintained to a public
quality bar: documentation, tests, installer behavior, and agentic development
conventions should remain safe to show.

> **No warranty.** This is a personal security monitoring project. It does not
> provide intrusion prevention, forensic integrity, tamper-proof logging, or any
> guarantee that compromise will be detected. If an attacker obtains root, they
> can disable or modify this agent. Use it as one defense-in-depth signal, not as
> your only control.

## Project status

Pre-alpha. The repository contains the initial production-shaped scaffold:

- Go command skeleton with `agent`, `test-notify`, `validate-config`, and
  `version` subcommands.
- Telegram notifier implementation.
- `journalctl`-based login and sudo watchers.
- `/proc`-based CPU, RAM, and Linux PSI I/O sampling.
- systemd + PAM installer scripts.
- Spec-driven documentation structure for LLM-based development.
- CI, Makefile targets, tests, and repository hygiene checks.

The initial agent code is intentionally conservative. It is a foundation, not a
finished host security product. It still needs real VPS image smoke testing
before it should be treated as broadly installable.

## Goals

`vps-watchdog` is optimized for one primary use case: quickly noticing suspicious
access to a VPS and receiving enough context to decide whether to investigate or
isolate the host.

Planned MVP signals:

- Successful interactive logins through PAM-covered services such as SSH and
  local console login.
- `sudo` usage and sudo failures as seen in journald/syslog.
- Sustained high CPU utilization.
- Low available memory.
- Sustained Linux PSI I/O pressure.
- Agent start/stop and installation health checks.

## Non-goals

- It is not an EDR.
- It is not tamper-proof after root compromise.
- It does not monitor cloud provider control panels.
- It does not block logins or sudo commands.
- It does not replace hardening, backups, SSH key hygiene, MFA, or centralized
  logging.

## Supported platform

The project targets:

- Linux VPS hosts.
- systemd-based distributions.
- `linux/amd64` binaries.
- Telegram Bot API notifications.

ARM builds are intentionally out of scope for now.

## Quick start from release

For public releases, download the small bootstrap script and run it with a pinned
version or `latest`:

```bash
curl -fsSLO https://raw.githubusercontent.com/ishmukhamet/vps-watchdog/master/scripts/install-release.sh
sh install-release.sh latest
```

The bootstrap script runs without root while it downloads the release tarball,
downloads `checksums.txt`, verifies the archive, extracts it, and then delegates
the privileged part to:

```bash
sudo ./packaging/install.sh --bin ./vps-watchdog
```

To install a specific version:

```bash
sh install-release.sh v0.1.0
```

Manual release install:

```bash
VERSION=v0.1.0
curl -fsSLO "https://github.com/ishmukhamet/vps-watchdog/releases/download/${VERSION}/vps-watchdog-linux-amd64.tar.gz"
curl -fsSLO "https://github.com/ishmukhamet/vps-watchdog/releases/download/${VERSION}/checksums.txt"
sha256sum -c --ignore-missing checksums.txt
mkdir -p vps-watchdog-install
tar -xzf vps-watchdog-linux-amd64.tar.gz -C vps-watchdog-install
cd vps-watchdog-install
sudo ./packaging/install.sh --bin ./vps-watchdog
```

## Quick start from source

```bash
git clone https://github.com/ishmukhamet/vps-watchdog.git
cd vps-watchdog
make check
make build
```

This creates:

```text
dist/vps-watchdog-linux-amd64
```

Install on a VPS:

```bash
sudo ./packaging/install.sh --bin ./dist/vps-watchdog-linux-amd64
```

The installer:

- copies the binary to `/usr/local/bin/vps-watchdog`;
- creates or repairs a locked system user named `vpswatchdog`;
- creates `/etc/vps-watchdog` and `/var/lib/vps-watchdog`;
- writes a default config when missing;
- stores Telegram credentials in root-owned files readable by the runtime group
  and repairs those permissions on reinstall;
- installs a small PAM hook for login events;
- installs and enables `/etc/systemd/system/vps-watchdog.service`;
- rejects empty Telegram credential files, validates runtime-user readability,
  sends a best-effort test notification, then starts or restarts the service and
  enables it for boot.

The daemon is a systemd **system service**. It does not depend on anyone logging
in as `vpswatchdog`.

## Configuration

Default config path:

```text
/etc/vps-watchdog/config.yaml
```

Example:

```yaml
host_id: "web-01"

telegram:
  token_file: /etc/vps-watchdog/telegram.token
  chat_id_file: /etc/vps-watchdog/telegram.chat_id

login:
  enabled: true

sudo:
  enabled: true

load:
  enabled: true
  sample_interval_sec: 15
  cooldown_sec: 900

  cpu_busy_percent: 90
  cpu_duration_sec: 180

  mem_available_percent: 10
  mem_duration_sec: 180

  psi_io_some_avg60: 20
  io_duration_sec: 180
```

Validate config locally:

```bash
vps-watchdog validate-config --config /etc/vps-watchdog/config.yaml
```

This validates config shape and threshold ranges. It does not contact Telegram.

Send a test alert:

```bash
vps-watchdog test-notify --config /etc/vps-watchdog/config.yaml
```

## Development commands

```bash
make fmt            # gofmt
make test           # go test ./...
make vet            # go vet ./...
make build          # build linux/amd64 binary
make check-docs     # validate repository documentation shape
make check          # fmt + vet + test + check-docs + build
make package        # create dist/vps-watchdog-linux-amd64.tar.gz
```

## Repository knowledge model

This repository is intended to be developed primarily through LLM coding agents
using spec-driven development.

- `AGENTS.md` is the short map for agents.
- `docs/` is the source of truth.
- `docs/exec-plans/PLANS.md` defines the execution plan format.
- Active work should live under `docs/exec-plans/active/`.
- Completed plans should move to `docs/exec-plans/completed/`.
- Architectural decisions belong under `docs/design-docs/adr/`.

Start here:

- [AGENTS.md](AGENTS.md)
- [docs/index.md](docs/index.md)
- [docs/architecture.md](docs/architecture.md)
- [docs/product-specs/mvp.md](docs/product-specs/mvp.md)
- [docs/exec-plans/PLANS.md](docs/exec-plans/PLANS.md)

## Security model

See [docs/SECURITY.md](docs/SECURITY.md).

Key point: this agent aims to get security-relevant events off-host quickly. It
cannot protect itself from a fully privileged attacker on the same host.

## License

MIT. See [LICENSE](LICENSE).
