# Core beliefs

## Product beliefs

- Fast, simple alerts beat complicated dashboards for the initial use case.
- A local host agent can detect useful signals, but it cannot defend itself from
  root compromise.
- Login and sudo events are more urgent than resource alerts.
- The monitoring path must not be able to lock out the administrator.
- Telegram-only is acceptable for the personal MVP.

## Engineering beliefs

- Boring Linux interfaces are better than opaque dependencies for this project.
- Small packages with explicit boundaries are easier for agents and humans to
  reason about.
- Public-quality private code reduces cleanup work before release.
- Docs should be short, indexed, and close to the behavior they describe.
- Every repeated mistake should become a test, check, ADR, or documented rule.

## Agentic development beliefs

- The repository is the durable memory.
- `AGENTS.md` is a map, not the encyclopedia.
- ExecPlans should make long tasks restartable.
- Mechanical checks are more reliable than style guidance.
- If an agent needs hidden context to succeed, the repository is underspecified.
