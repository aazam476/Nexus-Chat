#!/usr/bin/env bash
# Mutation testing (spec §7.1): changes the code of the critical packages
# in small ways (a flipped condition, a changed boundary) and checks that
# the tests notice. Each package must catch at least 70% of the changes
# its tests cover ("test efficacy").
#
# Usage: scripts/go-mutation.sh apps/server
# The packages are the ones listed in <module>/coverage-critical.txt, the
# same list that needs 100% branch coverage. A module without the file
# has nothing to check.
set -euo pipefail

module_directory="$1"
minimum_efficacy=70
cd "$module_directory"
if [ ! -f coverage-critical.txt ]; then
  echo "No coverage-critical.txt in ${module_directory}; nothing to mutate."
  exit 0
fi

failed=0
while read -r package; do
  [ -z "$package" ] && continue
  echo "== ${package}"
  # A generous timeout: the default is derived from one quick test run and
  # marks mutants as timed out on a busy machine.
  if ! gremlins unleash "$package" --threshold-efficacy "$minimum_efficacy" --timeout-coefficient 20 \
      --workers 2 --exclude-files '_test\.go$' --exclude-files '\.gen\.go$' --output-statuses lt; then
    failed=1
  fi
done < coverage-critical.txt
if [ "$failed" -ne 0 ]; then
  echo "Mutation testing: a package caught fewer than ${minimum_efficacy}% of mutants (see LIVED above)."
  exit 1
fi
echo "Mutation testing passed."
