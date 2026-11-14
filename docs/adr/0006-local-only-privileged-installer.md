# ADR 0006: The privileged installer works only on local files

## Status

Accepted.

## Context

A common pattern is `curl … | sudo sh`, where code fetched from the network runs
as root immediately. That makes the network path part of the root trust
boundary and makes the root step hard to audit or repeat offline.

## Decision

Split installation in two:

- a non-root bootstrap script downloads a release archive and its checksums,
  verifies them, and extracts the archive;
- the root installer (`packaging/install.sh`) takes a local binary path and
  never downloads anything. Its only network activity is the best-effort test
  notification, which runs as the runtime user, not as root.

## Consequences

- The root step is auditable and works offline or from a source build.
- Release download and verification can change without touching root code.
- Users who skip the bootstrap must verify checksums themselves.
- The bootstrap does not refuse to run as root. Started as root, it downloads,
  extracts, and executes the release binary with root privileges, and the
  split gives no protection.
- The checksums are downloaded from the same release as the archive. They
  detect a corrupted or truncated download, not a replaced release: whoever can
  publish or substitute a release controls both files.
- The installer that runs as root comes from that archive. The split keeps
  network code out of the root step; it does not remove the release from the
  root trust boundary.
