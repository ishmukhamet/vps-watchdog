#!/usr/bin/env bash
set -euo pipefail

required=(
  "README.md"
  "LICENSE"
  "AGENTS.md"
  "docs/index.md"
  "docs/architecture.md"
  "docs/SECURITY.md"
  "docs/sdd.md"
  "docs/QUALITY.md"
  "docs/product-specs/mvp.md"
  "docs/exec-plans/PLANS.md"
  "docs/exec-plans/active/0001-mvp-agent.md"
  "docs/design-docs/core-beliefs.md"
  "docs/references/harness-engineering-notes.md"
)

for path in "${required[@]}"; do
  if [[ ! -f "$path" ]]; then
    echo "missing required file: $path" >&2
    exit 1
  fi
  if [[ ! -s "$path" && "$path" != *.gitkeep ]]; then
    echo "required file is empty: $path" >&2
    exit 1
  fi
done

if ! grep -q "docs/" AGENTS.md; then
  echo "AGENTS.md must point to docs/" >&2
  exit 1
fi

if ! grep -q "No warranty" README.md; then
  echo "README.md must contain a no-warranty notice" >&2
  exit 1
fi

mapfile -t scan_files < <(git ls-files -co --exclude-standard)
secret_scan="$(mktemp)"
placeholder_scan="$(mktemp)"
trap 'rm -f "$secret_scan" "$placeholder_scan"' EXIT

if [[ "${#scan_files[@]}" -gt 0 ]] && grep -I -l -E '([0-9]{8,10}:[A-Za-z0-9_-]{20,}|bot[0-9]{8,10}:[A-Za-z0-9_-]{20,})' "${scan_files[@]}" >"$secret_scan"; then
  echo "possible Telegram token found in:" >&2
  sed 's/^/  /' "$secret_scan" >&2
  exit 1
fi

if [[ "${#scan_files[@]}" -gt 0 ]] && grep -I -l -E 'TODO:[[:space:]]ask user' "${scan_files[@]}" >"$placeholder_scan"; then
  echo "found agent placeholder that depends on chat history" >&2
  sed 's/^/  /' "$placeholder_scan" >&2
  exit 1
fi

exit 0
