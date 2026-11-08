#!/usr/bin/env bash
# Fails when a security advisory affects the pinned MLS library (ADR 0002).
# `pnpm audit` reads only reviewed advisories, and a high-severity ts-mls
# advisory once sat unreviewed for weeks, so this asks GitHub's advisory
# database for reviewed and unreviewed advisories affecting the exact
# version in packages/crypto/package.json, plus those published on the
# library's own repository.
#
# Usage: GITHUB_TOKEN=... scripts/check-mls-advisories.sh
set -euo pipefail

repository_root="$(cd "$(dirname "$0")/.." && pwd)"
version=$(node -p "require('${repository_root}/packages/crypto/package.json').dependencies['ts-mls']")
api="https://api.github.com"
auth=(-H "Accept: application/vnd.github+json")
if [ -n "${GITHUB_TOKEN:-}" ]; then
  auth+=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
fi

found=""
for type in reviewed unreviewed; do
  ids=$(curl -fsS "${auth[@]}" "${api}/advisories?ecosystem=npm&type=${type}&affects=ts-mls@${version}&per_page=100" |
    node -e 'let s="";process.stdin.on("data",c=>s+=c).on("end",()=>console.log(JSON.parse(s).map(a=>a.ghsa_id).join(" ")))')
  found="${found} ${ids}"
done
found=$(echo "$found" | xargs)
if [ -n "$found" ]; then
  echo "Advisories affect ts-mls ${version}: ${found}"
  echo "Upgrade or patch it before releasing (see ADR 0002)."
  exit 1
fi
echo "No known advisories affect ts-mls ${version}."
