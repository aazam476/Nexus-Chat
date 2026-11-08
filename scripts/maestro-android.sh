#!/usr/bin/env bash
# Runs the Maestro flows (apps/client/.maestro) against a running Android
# emulator and an instance on this machine, which the emulator reaches at
# 10.0.2.2 (spec §7.2, mobile end-to-end tests).
#
# Usage: scripts/maestro-android.sh APK SETUP_TOKEN
# Needs: adb, maestro on PATH, an instance on port 8080.
set -euo pipefail

apk="$1"
setup_token="$2"
repository_root="$(cd "$(dirname "$0")/.." && pwd)"

debug_output="$repository_root/maestro-debug"

adb wait-for-device
# The emulator reports booted before its package manager is ready.
until adb shell pm path android >/dev/null 2>&1; do sleep 2; done
adb install -r "$apk"
if ! maestro test "$repository_root/apps/client/.maestro" \
  --format junit --output "$repository_root/maestro-report.xml" \
  --debug-output "$debug_output" \
  -e INSTANCE=http://10.0.2.2:8080 \
  -e SETUP_TOKEN="$setup_token" \
  -e PASSWORD="maestro admin password"; then
  # Maestro prints only which flows failed; its log says why.
  echo "== Maestro report"
  cat "$repository_root/maestro-report.xml" 2>/dev/null || true
  echo "== Maestro log (last 200 lines)"
  find "$debug_output" -name 'maestro.log' -exec tail -n 200 {} \; 2>/dev/null || true
  exit 1
fi
