#!/usr/bin/env bash
# Downloads the MLS interoperability test vectors (spec §5.1) that the
# crypto package's tests run, from the mlswg/mls-implementations
# repository at a pinned commit, and checks each file's SHA-256 so a
# changed upstream file cannot change what the tests check.
#
# Usage: scripts/fetch-mls-vectors.sh
set -euo pipefail

commit="2034fe72e0c805a683ba76165660c834f2a8142a"
target="$(cd "$(dirname "$0")/.." && pwd)/packages/crypto/vectors"
mkdir -p "$target"

while read -r checksum file; do
  if ! echo "${checksum}  ${target}/${file}" | sha256sum -c --status - 2>/dev/null; then
    curl -fsSL -o "${target}/${file}" \
      "https://raw.githubusercontent.com/mlswg/mls-implementations/${commit}/test-vectors/${file}"
    echo "${checksum}  ${target}/${file}" | sha256sum -c -
  fi
done <<'LIST'
92ebb04b67b1aca4290965ae363650921842ceebe275bd79a2733414078d849c passive-client-welcome.json
b24949fdf857bf79d511f57504a8bdc7ce3247eb199684745e439c11b1a1da21 passive-client-handling-commit.json
0095d863e9d316872e237fc708debcd20ba2687ee905a3e831f97dd3e578d3c1 passive-client-random.json
LIST
echo "MLS test vectors are in ${target}"
