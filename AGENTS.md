# AGENTS.md

This file is the entry point for coding agents working in this repository. Keep
it short. Treat it as a map, not as the full instruction manual.

## Project

`vps-watchdog` is a Go-based Linux VPS security monitoring agent. It runs as a
systemd system service, reads local host signals, and sends Telegram alerts.

The project is personal-first but public-quality: code, docs, scripts, commit
history, and tests should remain suitable for a future public release.

## Start here

Read these files before non-trivial work:

1. `README.md` — user-facing overview.
2. `docs/index.md` — documentation map.
3. `docs/product-specs/mvp.md` — current product scope.
4. `docs/architecture.md` — runtime architecture and boundaries.
5. `docs/SECURITY.md` — threat model and security constraints.
6. `docs/exec-plans/PLANS.md` — required format for complex tasks.

## Development rules

- Use Go standard library unless a dependency has clear, documented value.
- Keep the binary self-contained and easy to cross-compile for `linux/amd64`.
- Prefer explicit boundaries over clever abstractions.
- Do not make login or sudo paths depend on network availability.
- Never log Telegram bot tokens or full Telegram API URLs.
- Keep installer actions idempotent.
- Keep runtime code non-root where possible.
- Treat all log/PAM/journal input as untrusted text.
- Include tests for parsing, threshold logic, formatting, and error paths.

## Commands

Run before proposing changes:

```bash
make check
```

Useful focused commands:

```bash
make fmt
make test
make vet
make build
make check-docs
```

## ExecPlans

Use an ExecPlan for:

- new monitoring signals;
- installer changes;
- systemd/PAM behavior changes;
- notification semantics;
- state/cursor changes;
- security-sensitive refactors;
- changes spanning more than two packages.

ExecPlans live in:

```text
docs/exec-plans/active/
docs/exec-plans/completed/
```

Follow `docs/exec-plans/PLANS.md`. ExecPlans are living documents: update the
progress, decisions, validation, and discoveries as work proceeds.

## Documentation updates

If behavior changes, update docs in the same change:

- User behavior: `README.md`, `docs/operations/*.md`.
- Architecture: `docs/architecture.md` and, when needed, an ADR.
- Security behavior: `docs/SECURITY.md`.
- Development workflow: `AGENTS.md` or `docs/sdd.md`.
- Planned work: active ExecPlan and `docs/QUALITY.md`.

## Repository boundaries

Expected source layout:

```text
cmd/vps-watchdog/          CLI entrypoint
internal/agent/            process orchestration
internal/config/           config loading and validation
internal/event/            event model and formatting
internal/journal/          journald integration
internal/metrics/          /proc and PSI collectors
internal/notifier/         notification interfaces
internal/notifier/telegram Telegram implementation
internal/state/            persistent runtime state helpers
packaging/                 installer, uninstaller, systemd/PAM assets
scripts/                   repository checks and release helpers
docs/                      source-of-truth documentation
```

Do not add new top-level directories without documenting why in `docs/index.md`.
