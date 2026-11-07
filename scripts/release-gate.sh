#!/usr/bin/env bash
# Release gate (spec §7.4): install Nexus Chat exactly the way README.md
# tells a self hoster to, then run a smoke test against it.
#
# Usage: scripts/release-gate.sh plain|caddy|nginx|traefik <server image tag>
#   plain    reach the instance on http://localhost:8080
#   caddy    reach it through Caddy, using Caddy's internal test CA
#   nginx    reach it through Nginx, using a throwaway certificate
#   traefik  reach it through Traefik, using a throwaway certificate
#
# The image must already exist locally as
# ghcr.io/aazam476/nexus-chat-server:<tag>.
#
# The smoke test grows with each milestone. Today it covers: containers come
# up healthy, setup token appears in the logs, setup creates the admin, and
# LiveKit is running. Second user, message and voice join are added in M2,
# M4 and M9.
set -euo pipefail

mode="$1"
image_tag="$2"
repository_root="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
start_time=$(date +%s)

step() { echo "== $*"; }
fail() { echo "FAIL: $*"; (cd "$work" && docker compose logs --no-color) || true; exit 1; }

# Only these two files are given to the "new user".
cp "$repository_root/infra/compose/docker-compose.yml" "$repository_root/infra/compose/.env.example" "$work/"
cd "$work"
compose_files=(-f docker-compose.yml)
base_url="http://localhost:8080"
curl_options=()

step "Follow README: copy .env.example to .env"
cp .env.example .env
echo "NEXUS_VERSION=$image_tag" >> .env

if [ "$mode" != "plain" ]; then
  cp -r "$repository_root/scripts/release-gate/$mode/." .
  compose_files+=(-f compose.proxy.yml)
  if [ "$mode" != "caddy" ]; then
    step "Create a throwaway certificate for nexus.test"
    mkdir -p certs
    openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -days 2 \
      -subj /CN=nexus.test -addext subjectAltName=DNS:nexus.test \
      -keyout certs/key.pem -out certs/cert.pem 2>/dev/null
    chmod 0644 certs/key.pem # read by the proxy container's own user
  fi
  # Docker's default bridge networks are in 172.16.0.0/12.
  sed -i 's|^TRUSTED_PROXIES=.*|TRUSTED_PROXIES=172.16.0.0/12|; s|^DOMAIN=.*|DOMAIN=nexus.test|' .env
  base_url="https://nexus.test"
  curl_options=(--resolve nexus.test:443:127.0.0.1)
fi

step "docker compose up -d"
docker compose "${compose_files[@]}" up -d --wait --wait-timeout 180 || fail "containers did not become healthy"

if [ "$mode" = "caddy" ]; then
  step "Trust Caddy's test CA"
  # Caddy creates its CA shortly after starting, so retry the copy.
  for _ in $(seq 1 30); do
    docker compose "${compose_files[@]}" cp proxy:/data/caddy/pki/authorities/local/root.crt ./test-ca.crt 2>/dev/null && break
    sleep 1
  done
  [ -f test-ca.crt ] || fail "Caddy did not create its test CA"
  curl_options+=(--cacert ./test-ca.crt)
elif [ "$mode" != "plain" ]; then
  curl_options+=(--cacert ./certs/cert.pem)
fi
if [ "$mode" != "plain" ]; then
  # A proxy counts as started before it has loaded its configuration.
  for _ in $(seq 1 30); do curl -fsS "${curl_options[@]}" "$base_url/healthz" >/dev/null 2>&1 && break; sleep 1; done
fi

step "Read the setup token from the logs"
token=""
for _ in $(seq 1 30); do
  token=$(docker compose logs server 2>/dev/null | grep -o '"setup_token":"[A-Z2-7-]*"' | tail -1 | cut -d'"' -f4 || true)
  [ -n "$token" ] && break
  sleep 1
done
[ -n "$token" ] || fail "no setup token in the server logs"

step "Setup status"
status=$(curl -fsS "${curl_options[@]}" "$base_url/api/v1/setup") || fail "GET /api/v1/setup failed"
echo "$status"
echo "$status" | grep -q '"setupRequired":true' || fail "setup should be required on a fresh instance"
echo "$status" | grep -q '"secureConnection":true' || fail "connection should count as secure (localhost or HTTPS via trusted proxy)"

step "Complete setup (create the admin)"
code=$(curl -sS "${curl_options[@]}" -o response.json -w '%{http_code}' -X POST "$base_url/api/v1/setup" \
  -H 'Content-Type: application/json' \
  -d "{\"setupToken\":\"$token\",\"admin\":{\"username\":\"admin\",\"displayName\":\"Admin\",\"email\":\"admin@example.com\",\"password\":\"release gate password\"},\"instance\":{\"name\":\"Release Gate\",\"registrationMode\":\"invite_only\"}}")
[ "$code" = "201" ] || { cat response.json; fail "setup returned $code"; }

step "LiveKit is running"
docker compose "${compose_files[@]}" ps --status running --services | grep -qx livekit || fail "livekit is not running"

step "Clean up"
docker compose "${compose_files[@]}" down -v
elapsed=$(( $(date +%s) - start_time ))
echo "Release gate ($mode) passed in ${elapsed}s"
[ "$elapsed" -lt 600 ] || fail "took longer than the 10-minute limit"
