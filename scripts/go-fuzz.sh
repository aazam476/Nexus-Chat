#!/usr/bin/env bash
# Runs every Go fuzz target in a module for a short time, so CI keeps
# exploring the parsers that handle untrusted input (code standards §7.1).
# A failing input is saved under testdata/fuzz and fails the build; commit
# it with the fix so it is replayed by every later test run.
#
# Usage: scripts/go-fuzz.sh apps/server [seconds-per-target]
set -euo pipefail

module_directory="$1"
seconds="${2:-30}"
cd "$module_directory"

found=0
# Each line of the listing is "package/path FuzzName".
while read -r package target; do
  found=$((found + 1))
  echo "== ${target} in ${package} (${seconds}s)"
  go test -run='^$' -fuzz="^${target}\$" -fuzztime="${seconds}s" "${package}"
done < <(grep -rl --include='*_test.go' '^func Fuzz' . | while read -r file; do
  package="./$(dirname "${file#./}")"
  grep -o '^func Fuzz[A-Za-z0-9_]*' "$file" | sed 's/^func //' | while read -r name; do
    echo "${package} ${name}"
  done
done)
echo "Fuzzed ${found} targets."
