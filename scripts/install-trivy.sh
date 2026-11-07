#!/usr/bin/env bash
# Installs a pinned Trivy release after checking its SHA-256, so CI never
# runs a scanner binary that differs from the one we reviewed.
# Usage: scripts/install-trivy.sh <version> <sha256 of the Linux-64bit tarball>
set -euo pipefail
version="$1"
expected_sha256="$2"
archive="trivy_${version}_Linux-64bit.tar.gz"
directory="$(mktemp -d)"
curl -fsSL -o "$directory/$archive" "https://github.com/aquasecurity/trivy/releases/download/v${version}/${archive}"
echo "${expected_sha256}  $directory/$archive" | sha256sum -c -
tar -xzf "$directory/$archive" -C "$directory" trivy
sudo install -m 0755 "$directory/trivy" /usr/local/bin/trivy
trivy --version
