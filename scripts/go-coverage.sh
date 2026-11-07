#!/usr/bin/env bash
# Runs a Go module's tests with the race detector and enforces the coverage
# gates from docs/contributing/code-standards.md:
#   - statement (line) coverage >= 85% across the module
#   - branch coverage >= 75% across the module (measured with gobco)
#   - 100% branch coverage for every package listed in the module's
#     coverage-critical.txt (auth, permissions, crypto), if that file exists
#
# Generated code (*.gen.go), sqlc store packages (generated only) and test
# helper packages (named like databasetest) are excluded: they are either
# not ours or not shipped.
#
# Usage: scripts/go-coverage.sh apps/server
set -euo pipefail

module_directory="$1"
minimum_statement_coverage=85
minimum_branch_coverage=75
cd "$module_directory"

echo "== Tests with race detector and statement coverage"
go test -race -count=1 -covermode=atomic -coverpkg=./... -coverprofile=coverage.raw.out ./...
# Keep the mode line, drop generated files and test helpers.
grep -v -E '\.gen\.go:|/[a-z]+test/' coverage.raw.out > coverage.out
statement_total=$(go tool cover -func=coverage.out | awk '/^total:/ { sub("%", "", $3); print $3 }')
echo "Statement coverage: ${statement_total}% (minimum ${minimum_statement_coverage}%)"

echo "== Branch coverage"
# Packages that contain only generated code or test helpers are skipped.
# gobco wants relative directories such as ./internal/config.
packages=$(go list -f '{{.Dir}}' ./... | sed "s|^$PWD|.|" | grep -v -E '/internal/api$|/store$|/[a-z]+test$|/test/')
covered_total=0
branches_total=0
for package in $packages; do
  result=$(gobco -branch "$package" | awk '/^Branch coverage:/ { print $3 }')
  covered=${result%/*}
  branches=${result#*/}
  echo "  ${package}: ${covered}/${branches}"
  covered_total=$((covered_total + covered))
  branches_total=$((branches_total + branches))
done

critical_failures=0
if [ -f coverage-critical.txt ]; then
  while read -r critical_package; do
    [ -z "$critical_package" ] && continue
    result=$(gobco -branch "$critical_package" | awk '/^Branch coverage:/ { print $3 }')
    if [ "${result%/*}" != "${result#*/}" ]; then
      echo "FAIL: $critical_package must have 100% branch coverage, has $result"
      critical_failures=1
    fi
  done < coverage-critical.txt
fi

branch_percent=100
if [ "$branches_total" -gt 0 ]; then
  branch_percent=$(awk -v c="$covered_total" -v b="$branches_total" 'BEGIN { printf "%.1f", 100 * c / b }')
fi
echo "Branch coverage: ${branch_percent}% (${covered_total}/${branches_total}, minimum ${minimum_branch_coverage}%)"

failed=$critical_failures
if awk -v v="$statement_total" -v m="$minimum_statement_coverage" 'BEGIN { exit !(v < m) }'; then
  echo "FAIL: statement coverage ${statement_total}% is below ${minimum_statement_coverage}%"
  failed=1
fi
if awk -v v="$branch_percent" -v m="$minimum_branch_coverage" 'BEGIN { exit !(v < m) }'; then
  echo "FAIL: branch coverage ${branch_percent}% is below ${minimum_branch_coverage}%"
  failed=1
fi
exit "$failed"
