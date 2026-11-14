# AGENTS.md

Rules for coding agents and human contributors. Start with
[README.md](README.md) for what the project is; read
[docs/architecture.md](docs/architecture.md) and
[docs/SECURITY.md](docs/SECURITY.md) before changing runtime, installer, or
anything on the login path.

The repository is public: everything committed — code, docs, examples, commit
messages — must be safe to publish. No secrets, real hostnames, IP addresses,
chat ids, or personal data.

## Invariants

Breaking any of these needs an ADR in `docs/adr/`, not just a code change.

- Login and sudo monitoring never wait on the network. The PAM hook never does
  network I/O and never fails closed.
- Telegram tokens and full Telegram API URLs never appear in logs, errors, CLI
  flags, tests, or docs.
- Log, PAM, and journal input is untrusted text.
- The runtime runs as an unprivileged user; root is limited to the installer.
- Installer and uninstaller are idempotent and touch only what they own.
- Standard library first; a dependency needs a documented reason.
- One self-contained `linux/amd64` binary.

## Making a change

1. Add tests for parsing, threshold logic, formatting, and error paths. A
   repeated mistake becomes a test or a check in `scripts/`, not a new paragraph
   of guidance.
2. Update the owning document in the same change:
   - commands, paths, config keys → `docs/install.md`, `docs/configuration.md`;
   - a new failure mode seen in practice → `docs/troubleshooting.md`;
   - structure, boundaries, failure handling → `docs/architecture.md`;
   - threat model or secret handling → `docs/SECURITY.md`;
   - a decision with lasting trade-offs → a new ADR;
   - contributor rules → this file.
3. Run `make check` (fmt, vet, test, docs check, build). `make help` lists
   focused targets.

## Writing docs

- Each fact has one home; elsewhere, link to it.
- Entry points (`README.md`, this file) stay short and link to depth.
- Describe principles, rules, and reasons, not an inventory of what the code
  currently contains. User-facing paths, commands, and config keys are the
  exception: they are a contract.
- Progress, known gaps, and plans belong in GitHub issues, not in docs.
- Prefer concrete statements ("the hook does no network I/O") to adjectives
  ("the hook is lightweight").
