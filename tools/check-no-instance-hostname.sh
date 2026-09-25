#!/usr/bin/env bash
# tools/check-no-instance-hostname.sh
#
# Operator-security guard: scan tracked source for the operator's production
# hostname and fail if found, so a future PR can't reintroduce it.
#
# The pattern is built from a fragment join so this script itself doesn't
# contain the literal token it's scanning for (otherwise the guard would
# fail-self). Anyone adding more secret-shaped tokens should follow the same
# pattern.

set -euo pipefail

# Build the forbidden pattern via fragment join — keep the literal out of the
# file that's looking for it.
forbidden="bnu""uy"

if grep -rn --exclude-dir=.git --exclude-dir=vendor --exclude-dir=node_modules --exclude-dir=tmp --exclude-dir=bin --exclude="$(basename "$0")" "$forbidden" .; then
  echo
  echo "ERROR: operator's production hostname must not appear in tracked source."
  echo "See cordinator/dispatches/chronicle/C-SCRUB-INSTANCE-URLS.md for context."
  exit 1
fi
echo "OK: no instance hostname references in tracked source."
