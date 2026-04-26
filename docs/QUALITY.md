# Quality rubric

This file tracks the quality bar for future public release.

## Current score: foundation / pre-alpha

The repository has public-quality scaffolding, but the runtime behavior still
needs integration testing on real VPS images before public release.

## Rubric

### Correctness

- Config validation catches bad shape, missing required values, thresholds, and
  disabled sections. Credential readability and Telegram reachability are tested
  by `test-notify`.
- Journal parsing tolerates malformed and distro-specific input.
- Metric sampling is tested against fixture data.
- Alert thresholds use duration and cooldown logic.

### Security

- The service runs as a dedicated non-login user.
- Telegram tokens are never logged.
- PAM hooks are non-blocking.
- Installer operations are idempotent and auditable, including permission repair
  for existing config, secret files, and the runtime account.

### Operability

- `systemctl status vps-watchdog` provides useful logs.
- `test-notify` validates Telegram configuration.
- Installer checks verify runtime-user readability for config and secret files.
- `validate-config` catches common setup errors.
- Runbooks cover install, restart, failure, and uninstall.

### Agent legibility

- `AGENTS.md` remains short.
- Docs are indexed from `docs/index.md`.
- ExecPlans are self-contained.
- Repeated mistakes become tests or repository checks.

### Public release readiness

- CI is green.
- README has realistic guarantees and limitations.
- License is present.
- No private hostnames, tokens, or personal data exist in examples.
- Release artifacts are reproducible from `make package`.

## Known gaps

- No VM-level install smoke test yet.
- No durable notification queue yet.
- The runtime notification queue is in-memory and best-effort; events may be
  dropped during prolonged Telegram/network outages.
- Sudo parsing classifies common command, auth failure, and denial messages;
  distro-specific sudo formats still need broader coverage.
- GitHub release workflow exists but needs a first public tag/release dry run.
- No distro compatibility matrix yet.
- Installer behavior still needs VM-level idempotency checks across supported
  distro versions.
