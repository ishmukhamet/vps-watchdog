# Spec-driven development workflow

This repository is optimized for LLM-assisted development. The goal is not to
make one giant prompt. The goal is to make the repository itself legible enough
that an agent can recover context from versioned files and validate its work with
standard commands.

## Principles

1. Keep repository knowledge local and versioned.
2. Use small entry points that point to deeper source-of-truth documents.
3. Convert repeated review comments into docs, tests, or checks.
4. Prefer mechanical validation over prose-only rules.
5. Keep complex plans self-contained so work can resume without chat history.

## Workflow

For small changes:

1. Read `AGENTS.md` and the relevant docs.
2. Make the change.
3. Update docs if behavior changed.
4. Run `make check`.

For complex changes:

1. Create an ExecPlan in `docs/exec-plans/active/`.
2. Make the plan self-contained.
3. Record progress and decisions while implementing.
4. Validate with tests and, when relevant, manual commands.
5. Move the plan to `docs/exec-plans/completed/` when done.

## What needs an ExecPlan

Use an ExecPlan for:

- monitoring signal additions;
- state/cursor changes;
- installer or systemd behavior changes;
- security-sensitive behavior;
- significant refactors;
- any task likely to take more than one focused agent run.

## Review expectations

A good change includes:

- a clear problem statement;
- code changes with tests;
- docs updates;
- validation output or a reproducible validation path;
- no secret material;
- no hidden dependency on chat history.
