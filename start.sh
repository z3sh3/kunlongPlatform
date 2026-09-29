#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# Boot helper for the new-api stack (server side, called by deploy.sh or manually).
#
# Responsibilities:
#   1. Ensure the bind-mount data directories exist on the host; the PostgreSQL
#      directory must be owned by the postgres user inside the container
#      (UID:GID 999 in postgres:18), otherwise `initdb` aborts on the first boot.
#   2. Start the stack via docker compose with explicit env/project/file paths.
#   3. Verify the configured DB role/database exist; fail fast (not crash-loop)
#      when a pre-existing cluster was initialized with different credentials.
#
# Usage:
#   ./start.sh [--build] [--force]
#
# Flags:
#   --build   rebuild images before starting (used by deploy.sh on code changes)
#   --force   rebuild images + force-recreate containers and remove orphans
#             (implies --build)
#
# Overridable via environment (or the env file for the *_DIR and POSTGRES_* vars):
#   ENV_FILE         env file passed to `docker compose --env-file`  (default: .env)
#   COMPOSE_FILE     compose file path                               (default: docker-compose.yml)
#   PGDATA_DIR       host path of the PostgreSQL bind mount          (default: /srv/newapi/postgres)
#   DATA_DIR         host path of the new-api /data bind mount      (default: /srv/newapi/data)
#   LOG_DIR          host path of the new-api log bind mount        (default: /srv/newapi/log)
#   CADDY_DATA_DIR   host path of the Caddy certificates bind mount  (default: /srv/newapi/caddy/data)
#   CADDY_CONFIG_DIR host path of the Caddy autosave bind mount     (default: /srv/newapi/caddy/config)
#
# The compose project name is pinned to `newapi` (see `compose()` below); it is
# never read from the environment so every invocation targets the same project.
# ---------------------------------------------------------------------------
set -euo pipefail

BUILD_FLAG=""
FORCE_FLAGS=""
while [ $# -gt 0 ]; do
  case "$1" in
    --build) BUILD_FLAG="--build"; shift ;;
    --force) BUILD_FLAG="--build"; FORCE_FLAGS="--force-recreate --remove-orphans"; shift ;;
    *) echo "[start] unknown argument: $1" >&2; exit 2 ;;
  esac
done

ENV_FILE="${ENV_FILE:-.env}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"

# Read a single KEY=VALUE out of $ENV_FILE without exporting the rest of the
# file into the shell (avoids `set -a` polluting the environment).
# Usage: env_value KEY [default]
env_value() {
  local key="$1" default="${2:-}" val=""
  if [ -f "$ENV_FILE" ]; then
    val="$(sed -n "s/^[[:space:]]*${key}=//p" "$ENV_FILE" | tail -n 1 || true)"
  fi
  printf '%s' "${val:-$default}"
}

# Run docker compose with explicit env/project/file paths. The project name is
# hardcoded to `newapi` so it never depends on the directory name or the
# COMPOSE_PROJECT_NAME environment variable.
compose() {
  docker compose --env-file "$ENV_FILE" -p newapi -f "$COMPOSE_FILE" "$@"
}

# The official postgres:18 image runs the database as UID:GID 999. The bind
# mount must be writable by that uid/gid or the container fails on first boot.
POSTGRES_UID="${POSTGRES_UID:-999}"
POSTGRES_GID="${POSTGRES_GID:-999}"

# Resolve the host directories with the same precedence docker compose uses
# (shell env > env file > built-in default), so the directories we prepare are
# exactly the ones compose will bind-mount.
PGDATA_DIR="${PGDATA_DIR:-$(env_value PGDATA_DIR /srv/newapi/postgres)}"
DATA_DIR="${DATA_DIR:-$(env_value DATA_DIR /srv/newapi/data)}"
LOG_DIR="${LOG_DIR:-$(env_value LOG_DIR /srv/newapi/log)}"
CADDY_DATA_DIR="${CADDY_DATA_DIR:-$(env_value CADDY_DATA_DIR /srv/newapi/caddy/data)}"
CADDY_CONFIG_DIR="${CADDY_CONFIG_DIR:-$(env_value CADDY_CONFIG_DIR /srv/newapi/caddy/config)}"

# --- 1. ensure the bind-mount directories exist with correct ownership ---------
if [ ! -d "$PGDATA_DIR" ]; then
  echo "[start] creating PostgreSQL data directory: $PGDATA_DIR"
  mkdir -p "$PGDATA_DIR"
fi

uid="$(stat -c '%u' "$PGDATA_DIR" 2>/dev/null || true)"
gid="$(stat -c '%g' "$PGDATA_DIR" 2>/dev/null || true)"
if [ "$uid" != "$POSTGRES_UID" ] || [ "$gid" != "$POSTGRES_GID" ]; then
  echo "[start] setting ownership of $PGDATA_DIR to ${POSTGRES_UID}:${POSTGRES_GID}"
  chown "$POSTGRES_UID":"$POSTGRES_GID" "$PGDATA_DIR"
fi

for dir in "$DATA_DIR" "$LOG_DIR" "$CADDY_DATA_DIR" "$CADDY_CONFIG_DIR"; do
  if [ ! -d "$dir" ]; then
    echo "[start] creating directory: $dir"
    mkdir -p "$dir"
  fi
done

export PGDATA_DIR DATA_DIR LOG_DIR CADDY_DATA_DIR CADDY_CONFIG_DIR

# --- 2. start the stack -------------------------------------------------------
compose up -d $BUILD_FLAG $FORCE_FLAGS

# --- 3. verify the configured DB role/database exist ---------------------------
# POSTGRES_USER/PASSWORD/DB take effect only on FIRST initdb; a volume that was
# initialized with different credentials keeps them. Detect the mismatch here
# and fail with an actionable message instead of a new-api crash-loop.
POSTGRES_USER="$(env_value POSTGRES_USER newapi)"
POSTGRES_DB="$(env_value POSTGRES_DB newapi)"

# Wait for the server to accept connections (pg_isready ignores the role).
for _ in {1..30}; do
  compose exec -T postgres pg_isready >/dev/null 2>&1 && break
  sleep 1
done
if ! compose exec -T postgres pg_isready >/dev/null 2>&1; then
  echo "[start] ERROR: PostgreSQL did not become ready within 30s (check 'docker compose logs postgres')" >&2
  exit 1
fi

if ! compose exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc 'SELECT 1' >/dev/null 2>&1; then
  cat >&2 <<EOF
[start] ERROR: role "$POSTGRES_USER" or database "$POSTGRES_DB" does not exist in
        the existing PostgreSQL cluster at $PGDATA_DIR.
        POSTGRES_USER/PASSWORD/DB apply only on first initdb; a volume initialized
        with different credentials keeps them.
        Fix:
          * data is disposable:
              docker compose -p newapi down && sudo rm -rf $PGDATA_DIR && ./start.sh
          * keep the data: restore POSTGRES_USER/DB to the original values, or
            CREATE ROLE/DATABASE against the old cluster manually.
EOF
  exit 1
fi

echo "[start] stack is up; new-api reachable at https://<domain> (automatic HTTPS when DOMAIN is set) or http://<public-ip> on :80, via caddy"
