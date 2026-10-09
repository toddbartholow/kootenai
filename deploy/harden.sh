#!/usr/bin/env bash
set -euo pipefail

echo "=== Kootenai VM Hardening ==="

# File permissions
echo "Fixing file permissions..."
chmod 600 ~/kootenai/deploy/.env
chmod 600 ~/kootenai/deploy/ssl/server.key 2>/dev/null || echo "  (server.key owned by root — run with sudo to fix)"

# Verify demo auth is safe.
#
# .env.example carries AUTH_DEMO_MODE=false but no AUTH_DEMO_ROLE line at all,
# so the role is normally absent and compose supplies the default (student).
# Report absence as that default rather than calling an empty result "OK".
ENV_FILE=~/kootenai/deploy/.env

# read_env NAME — last assignment wins, matching dotenv semantics. Normalises
# what the parsers normalise so this agrees with runtime rather than with the
# literal text: an optional `export ` prefix, surrounding quotes, a trailing
# CR from a CRLF file, and case.
#
# `|| true` is load-bearing: this script runs under `set -euo pipefail`, and
# grep exits 1 when a key is absent — which is the normal case for
# AUTH_DEMO_ROLE. Without it the script dies here and nothing below runs.
read_env() {
  key="$1"
  grep -E "^[[:space:]]*(export[[:space:]]+)?${key}=" "$ENV_FILE" 2>/dev/null \
    | tail -1 \
    | sed -E "s|^[[:space:]]*(export[[:space:]]+)?${key}=||" \
    | tr -d '\r' \
    | sed -E 's|[[:space:]]+#.*$||; s|[[:space:]]+$||' \
    | sed -E 's|^"(.*)"$|\1|' \
    | sed -E "s|^'(.*)'\$|\1|" \
    | tr '[:upper:]' '[:lower:]' \
    || true
}

demo_mode=$(read_env AUTH_DEMO_MODE)
demo_role=$(read_env AUTH_DEMO_ROLE)
demo_confirm=$(read_env AUTH_DEMO_MODE_CONFIRM)

if [ "$demo_mode" = "true" ]; then
  echo "WARNING: AUTH_DEMO_MODE=true — every request is treated as an authenticated"
  echo "         user, CSRF is skipped and WebSocket auth is bypassed. Unset it."
  # The API refuses to boot in demo mode without this, so an unset confirm is
  # what is actually keeping the door shut.
  if [ "$demo_confirm" = "i_understand_the_risks" ]; then
    echo "WARNING: AUTH_DEMO_MODE_CONFIRM is set — the API will start in this state."
  else
    echo "         AUTH_DEMO_MODE_CONFIRM is unset, so the API will refuse to start."
  fi
else
  echo "Demo mode OK: ${demo_mode:-unset (defaults to false)}"
fi

if [ "$demo_role" = "admin" ]; then
  echo "WARNING: AUTH_DEMO_ROLE is set to 'admin' — changing to 'student'"
  # Replace the whole value rather than matching admin's exact spelling —
  # read_env has already normalised quoting and case, so the line is known bad.
  sed -i.bak -E "s|^([[:space:]]*(export[[:space:]]+)?AUTH_DEMO_ROLE=).*|\\1student|" "$ENV_FILE"
  rm -f "${ENV_FILE}.bak"
else
  echo "Demo role OK: ${demo_role:-unset (defaults to student)}"
fi

# Clean up stale artifacts
echo "Removing stale artifacts..."
rm -f ~/kootenai/Taskfile.yml ~/kootenai/TODO.md
rm -rf ~/kootenai/d/ ~/kootenai/mockups/ ~/kootenai/configs/
rm -f ~/kootenai/migration/export/*.sql

# Prune Docker
echo "Pruning unused Docker images..."
docker image prune -f 2>/dev/null || true

echo "=== Hardening complete ==="
