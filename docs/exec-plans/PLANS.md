# ExecPlans

An ExecPlan is a self-contained implementation plan for a coding agent. It must
be detailed enough that a future agent can continue from the plan and the current
working tree without prior chat history.

## When to use an ExecPlan

Use one for:

- new monitoring signals;
- security-sensitive behavior;
- installer/systemd/PAM changes;
- state and cursor behavior;
- refactors spanning multiple packages;
- tasks expected to take more than one focused implementation session.

## Required sections

Each ExecPlan must contain:

1. Title.
2. Status.
3. Context.
4. Goal.
5. Non-goals.
6. Current state.
7. Proposed design.
8. Milestones.
9. Validation plan.
10. Progress log.
11. Decision log.
12. Risks and mitigations.
13. Completion criteria.

## Rules

- Keep the plan self-contained.
- Update the plan as discoveries happen.
- Record decisions with dates.
- Keep validation commands copy/pasteable.
- Do not store secrets or private host details.
- Move completed plans to `docs/exec-plans/completed/`.

## Template

```markdown
# <title>

## Status

Draft | Active | Completed | Abandoned

## Context

What problem exists and what repository state matters?

## Goal

What should be true when this is complete?

## Non-goals

What is explicitly out of scope?

## Current state

What files, packages, and behavior exist now?

## Proposed design

How will the solution work?

## Milestones

- [ ] Milestone 1
- [ ] Milestone 2

## Validation plan

Commands and manual checks.

## Progress log

- YYYY-MM-DD: Initial plan created.

## Decision log

- YYYY-MM-DD: Decision and rationale.

## Risks and mitigations

Known risks and handling.

## Completion criteria

Concrete conditions for completion.
```
