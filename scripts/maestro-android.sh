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

adb wait-for-device
adb install -r "$apk"
maestro test "$repository_root/apps/client/.maestro" \
  --format junit --output "$repository_root/maestro-report.xml" \
  -e INSTANCE=http://10.0.2.2:8080 \
  -e SETUP_TOKEN="$setup_token" \
  -e PASSWORD="maestro admin password"
