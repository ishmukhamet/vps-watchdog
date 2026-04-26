# Harness engineering notes

This repository follows a small subset of the engineering harness ideas described
in OpenAI's "Harness engineering: leveraging Codex in an agent-first world" and
OpenAI's Codex ExecPlans guidance.

Sources:

- https://openai.com/index/harness-engineering/
- https://developers.openai.com/cookbook/articles/codex_exec_plans

## Applied locally

### AGENTS.md is a map

The root `AGENTS.md` should stay short and point agents to the relevant source of
truth. It should not become a complete manual.

### docs/ is the source of truth

Product specs, architecture, security model, operations, references, and
execution plans live under `docs/`.

### ExecPlans are first-class artifacts

Complex work should create or update a self-contained ExecPlan under
`docs/exec-plans/active/`. The plan should include progress, decisions,
validation, and recovery context.

### Mechanical checks beat prose

Use `make check`, tests, and repository scripts to enforce structure and quality.
When a repeated failure appears, add a check or test rather than another long
instruction.

### Agent legibility matters

Prefer code and docs that a future agent can inspect, validate, and modify
without relying on hidden chat context.
