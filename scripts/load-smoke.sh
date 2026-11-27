#!/usr/bin/env bash
# The load smoke test (spec §7.2, tests/load/smoke.js): seeds a running
# test instance's database with USERS users (500 by default) in one server,
# then runs k6 in its official container against the instance. k6 fails
# the run when the 95th percentile delivery time is 300 ms or more, or when
# messages go missing.
#
# Usage: scripts/load-smoke.sh BASE_URL DATABASE_URL
# Needs: go and docker. Run it only against a throwaway instance: it adds
# accounts with sessions directly to the database.
set -euo pipefail

base_url="$1"
database_url="$2"
users="${USERS:-500}"
repository_root="$(cd "$(dirname "$0")/.." && pwd)"
# grafana/k6:1.3.0
k6_image="grafana/k6@sha256:3ddc8b1a33a2c3d8edc6e99b6a762ae36cba08788463458f5e6a7703e14eb77d"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
(cd "$repository_root/apps/server" && DATABASE_URL="$database_url" go run ./test/load/seed -users "$users") > "$work/seed.json"
# k6 runs as an unprivileged user in its container.
chmod a+rx "$work" && chmod a+r "$work/seed.json"
docker run --rm --network host \
  -v "$repository_root/tests/load:/load:ro" -v "$work:/seed:ro" \
  "$k6_image" run -e SEED=/seed/seed.json -e BASE_URL="$base_url" /load/smoke.js
