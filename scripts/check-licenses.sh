#!/usr/bin/env bash
# Fails if any dependency uses a license outside the allowlist (spec §0.3:
# Apache-2.0-compatible only, no GPL/AGPL). Every allowed license and why it
# is allowed is listed in docs/dependencies.md.
#
# Usage:
#   scripts/check-licenses.sh go <module directory>   # e.g. apps/server
#   scripts/check-licenses.sh js <directory>          # e.g. . (whole workspace)
set -euo pipefail

go_allowed="Apache-2.0,MIT,BSD-2-Clause,BSD-3-Clause,ISC,MPL-2.0,0BSD,Unlicense,CC0-1.0"
js_allowed="Apache-2.0;MIT;BSD-2-Clause;BSD-3-Clause;ISC;MPL-2.0;0BSD;Unlicense;CC0-1.0;BlueOak-1.0.0;Python-2.0;CC-BY-3.0;CC-BY-4.0;(MIT OR CC0-1.0)"

kind="$1"
directory="$2"
case "$kind" in
  go)
    # Only packages that end up in the shipped binaries (./cmd/...) count;
    # test-only and build-only tools are not distributed. Our own packages
    # are covered by the repository's LICENSE file.
    (cd "$directory" && go-licenses check ./cmd/... \
      --allowed_licenses="$go_allowed" \
      --ignore github.com/aazam476/nexus-chat)
    ;;
  js)
    # All dependencies, including development tools, are checked: it is
    # simpler than proving which ones end up in an app bundle.
    license-checker-rseidelsohn --start "$directory" --excludePrivatePackages --onlyAllow "$js_allowed" >/dev/null
    ;;
  *)
    echo "usage: $0 go|js <directory>" >&2
    exit 2
    ;;
esac
echo "License check passed for $kind dependencies in $directory"
