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
  never downloads anything.

## Consequences

- The root step is auditable and works offline or from a source build.
- Release download and verification can change without touching root code.
- Users who skip the bootstrap must verify checksums themselves.
