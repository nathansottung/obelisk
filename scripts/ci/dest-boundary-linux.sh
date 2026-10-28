#!/usr/bin/env bash
# Runs the destination-boundary tests on Linux and fails if any of them skipped.
# The full suite already runs them; this step exists because a skip there (no
# symlink support, an unexpected filesystem) would otherwise pass silently, and
# these are the tests that show Obelisk never writes into a registered source.
set -euo pipefail
cd "$(dirname "$0")/../.."
selector='^TestDestAuth_'
tests=$(go test -list "$selector" . | sed -n '/^Test/p')
if [ -z "$tests" ]; then echo "no destination-boundary tests found" >&2; exit 1; fi
echo "$tests"
echo "temp folder: ${TMPDIR:-/tmp}"
stat -f -c 'filesystem=%T path=%n' "${TMPDIR:-/tmp}" /
log=$(mktemp)
go test -count=1 -v -run "$selector" . | tee "$log"
if grep -Eq '^[[:space:]]*--- SKIP:' "$log"; then
  echo "destination-boundary tests skipped on Linux; they must run here" >&2
  grep -E '^[[:space:]]*--- SKIP:' "$log" >&2
  exit 1
fi
