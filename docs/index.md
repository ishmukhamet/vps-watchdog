# Documentation index

This directory is the source of truth for the repository. `AGENTS.md` is only a
map; deeper project knowledge belongs here.

## Product and scope

- `product-specs/mvp.md` — current MVP product specification.
- `design-docs/core-beliefs.md` — product and engineering principles.
- `QUALITY.md` — quality rubric and known gaps.

## Architecture and decisions

- `architecture.md` — current system architecture.
- `design-docs/adr/` — architecture decision records.

Current ADRs:

- `design-docs/adr/0001-systemd-system-service.md`
- `design-docs/adr/0002-journalctl-subprocess.md`
- `design-docs/adr/0003-telegram-only-mvp.md`
- `design-docs/adr/0004-dedicated-runtime-user.md`

## Operations

- `operations/install.md` — installation model.
- `operations/configuration.md` — config file and secrets.
- `operations/runbook.md` — service checks and troubleshooting.

## Security

- `SECURITY.md` — threat model, guarantees, and non-guarantees.

## Spec-driven development

- `sdd.md` — repository workflow for LLM-driven development.
- `exec-plans/PLANS.md` — ExecPlan contract.
- `exec-plans/active/` — active plans.
- `exec-plans/completed/` — completed plans.

## References

- `references/harness-engineering-notes.md` — local summary of the engineering
  harness principles used to shape this repository.

## Documentation rules

- Prefer short documents with clear ownership over one long manual.
- Update docs in the same change as behavior.
- Every non-trivial behavior should be discoverable from this directory.
- Every ExecPlan must be self-contained enough to restart work without chat
  history.
