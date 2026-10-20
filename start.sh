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
#   4. Optionally start the Dify (agent development) stack as a separate compose
#      project `dify` when DIFY_ENABLED=true in the env file: create the shared
#      dify/dify_plugin databases, render docker/dify/.env from the vendored
#      template (random secrets generated once, derived values always fresh),
#      then `docker compose -p dify ... up -d`.
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

# --- 4. Dify (agent development) stack (optional) -----------------------------
# Runs as its own compose project `dify` (see docker/dify/) and reuses the
# postgres/redis containers of the `newapi` project over the shared
# newapi_default network. Enabled via DIFY_ENABLED=true in the env file.
DIFY_ENABLED="$(env_value DIFY_ENABLED false)"
if [ "$DIFY_ENABLED" != "true" ]; then
  echo "[start] dify stack disabled (set DIFY_ENABLED=true to enable); skipping"
  exit 0
fi

DIFY_DIR="docker/dify"
DIFY_ENV="$DIFY_DIR/.env"
DIFY_TEMPLATE="$DIFY_DIR/.env.example"
if [ ! -f "$DIFY_TEMPLATE" ]; then
  echo "[start] ERROR: $DIFY_TEMPLATE is missing; cannot start the dify stack" >&2
  exit 1
fi

# 4a. idempotently create the dify databases in the shared postgres cluster.
#     Local psql inside the container authenticates over the trust socket,
#     same as the role/database check above.
for dify_db in dify dify_plugin; do
  if ! compose exec -T postgres psql -U "$POSTGRES_USER" -d postgres -tAc \
      "SELECT 1 FROM pg_database WHERE datname='$dify_db'" | grep -q 1; then
    echo "[start] creating database: $dify_db"
    compose exec -T postgres psql -U "$POSTGRES_USER" -d postgres -c "CREATE DATABASE \"$dify_db\"" >/dev/null
  fi
done

# 4b. (re)render docker/dify/.env from the vendored template. Secrets marked
#     with __GEN_*__ placeholders are generated once (openssl rand -base64 42)
#     and then carried over from an existing .env on later runs; derived values
#     (shared credentials, port) are refreshed from the root env file on every
#     run so credential rotations propagate. Upstream sample defaults are
#     never used.
POSTGRES_PASSWORD_VAL="$(env_value POSTGRES_PASSWORD '')"
REDIS_PASSWORD_VAL="$(env_value REDIS_PASSWORD '')"
if [ -z "$POSTGRES_PASSWORD_VAL" ] || [ -z "$REDIS_PASSWORD_VAL" ]; then
  echo "[start] ERROR: POSTGRES_PASSWORD/REDIS_PASSWORD are required to render $DIFY_ENV" >&2
  exit 1
fi
AGENT_DEV_PORT_VAL="$(env_value AGENT_DEV_PORT 8081)"

# Seed each __GEN_*__ secret from an existing docker/dify/.env when present,
# otherwise generate a fresh random value. DIFY_INIT_PASSWORD_GENERATED flags a
# first-boot bootstrap-gate password so we can tell the operator where to find
# it (INIT_PASSWORD is consumed once during console initialization but is
# persisted like the rest).
DIFY_INIT_PASSWORD_GENERATED=0
DIFF_TMP="$DIFY_DIR/.env.tmp.render"
cp "$DIFY_TEMPLATE" "$DIFF_TMP"

# Escape sed replacement metacharacters (&, backslash, the | delimiter) so
# operator-chosen passwords from the root env file cannot break substitution.
sed_escape() { printf '%s' "$1" | sed -e 's/[\\|&]/\\&/g'; }
for pair in \
  'SECRET_KEY|__GEN_SECRET_KEY__' \
  'INIT_PASSWORD|__GEN_INIT_PASSWORD__' \
  'WEAVIATE_API_KEY|__GEN_WEAVIATE_API_KEY__' \
  'SANDBOX_API_KEY|__GEN_SANDBOX_API_KEY__' \
  'PLUGIN_DAEMON_KEY|__GEN_PLUGIN_DAEMON_KEY__' \
  'PLUGIN_DIFY_INNER_API_KEY|__GEN_PLUGIN_INNER_KEY__' \
  'DIFY_AGENT_API_TOKEN|__GEN_AGENT_API_TOKEN__' \
  'DIFY_AGENT_SERVER_SECRET_KEY|__GEN_AGENT_SERVER_KEY__' \
  'DIFY_AGENT_LOCAL_SANDBOX_AUTH_TOKEN|__GEN_AGENT_SANDBOX_TOKEN__'; do
  dify_key="${pair%%|*}"; placeholder="${pair#*|}"
  val=""
  if [ -f "$DIFY_ENV" ]; then
    val="$(sed -n "s/^${dify_key}=//p" "$DIFY_ENV" | tail -n 1 || true)"
  fi
  case "$dify_key" in
    INIT_PASSWORD)
      # Dify's console validates the bootstrap password with
      # max_length=30 (api/controllers/console/init_validate.py).
      # Regenerate legacy values that exceed the limit instead of
      # carrying them over (harmless: the gate is single-use).
      if [ "${#val}" -gt 30 ]; then val=""; fi
      ;;
    DIFY_AGENT_SERVER_SECRET_KEY)
      # The agent backend derives its JWE root key from this value and its
      # settings only accept unpadded base64url (dify_agent server settings,
      # decode_server_secret_key). Values rendered by older versions of
      # this script used standard base64 and crash-loop agent_backend, so
      # regenerate them. Rotation only invalidates in-flight agent-stub
      # bearer tokens.
      if ! printf '%s' "$val" | grep -qE '^[A-Za-z0-9_-]+$'; then
        if [ -n "$val" ]; then
          echo "[start] regenerating invalid DIFY_AGENT_SERVER_SECRET_KEY (must be unpadded base64url)"
        fi
        val=""
      fi
      ;;
  esac
  if [ -z "$val" ]; then
    if [ "$dify_key" = "INIT_PASSWORD" ]; then
      # 28 base64 chars (21 bytes, no padding) fits the 30-char API limit.
      val="$(openssl rand -base64 21 | tr -d '\n')"
      DIFY_INIT_PASSWORD_GENERATED=1
    elif [ "$dify_key" = "DIFY_AGENT_SERVER_SECRET_KEY" ]; then
      # Exactly 32 bytes as unpadded base64url (43 chars): the agent
      # backend rejects the +, / and = of standard base64 and any decoded
      # length other than 32.
      val="$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=\n')"
    else
      val="$(openssl rand -base64 42 | tr -d '\n')"
    fi
  fi
  sed -i "s|${placeholder}|${val}|g" "$DIFF_TMP"
done
sed -i \
  -e "s|__POSTGRES_USER__|$(sed_escape "$POSTGRES_USER")|g" \
  -e "s|__POSTGRES_PASSWORD__|$(sed_escape "$POSTGRES_PASSWORD_VAL")|g" \
  -e "s|__REDIS_PASSWORD__|$(sed_escape "$REDIS_PASSWORD_VAL")|g" \
  -e "s|__AGENT_DEV_PORT__|$(sed_escape "$AGENT_DEV_PORT_VAL")|g" \
  "$DIFF_TMP"
if grep -v '^[[:space:]]*#' "$DIFF_TMP" | grep -q -e '__GEN_' -e '__POSTGRES_USER__' -e '__POSTGRES_PASSWORD__' \
        -e '__REDIS_PASSWORD__' -e '__AGENT_DEV_PORT__'; then
  echo "[start] ERROR: unresolved placeholders in $DIFF_TMP; refusing to start the dify stack" >&2
  rm -f "$DIFF_TMP"
  exit 1
fi
mv "$DIFF_TMP" "$DIFY_ENV"
if [ "$DIFY_INIT_PASSWORD_GENERATED" = "1" ]; then
  echo "[start] dify INIT_PASSWORD generated (one-time bootstrap gate); see INIT_PASSWORD in $DIFY_ENV"
fi

# 4c. start the dify stack (own compose project; interpolation and container
#     env both come from docker/dify/.env, which doubles as the project env file).
docker compose -p dify --project-directory "$DIFY_DIR" -f "$DIFY_DIR/docker-compose.yaml" up -d

echo "[start] dify stack is up; console reachable at http://<host>:$AGENT_DEV_PORT_VAL (plain HTTP; open the firewall as needed)"
# The console reads Dify agents through the Dify console API, so it needs an
# account that can see the workspace apps. Both addresses are console options
# (System settings -> Operations -> Dify Agent Service), not env vars.
echo "[start] to show Dify agents in the Agent Workshop, set in System settings -> Operations -> Dify Agent Service:"
echo "[start]   Dify API Base URL = http://dify-api:5001"
echo "[start]   Dify Console URL  = http://<host>:$AGENT_DEV_PORT_VAL"
echo "[start]   plus the email and password of a Dify console account"
