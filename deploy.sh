#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# Deploy new-api to the production server.
#
#   1. Compile & test locally (Go build for the root module and relaykit; the
#      frontend is built inside the docker image on the server).
#   2. rsync the source tree to the server (mirror, excluding runtime data and
#      the root .env). Pass --env <file> to also transfer a specific .env file.
#   3. Trigger start.sh on the server to rebuild images and restart docker compose
#      (start.sh pins the compose project name to `newapi` via `-p newapi`).
#
# Usage:
#   DEPLOY_HOST=deploy@203.0.113.10 ./deploy.sh
#   ./deploy.sh --env .env.production   # also transfer .env.production as .env
#   ./deploy.sh --force                 # rebuild + force-recreate + remove orphans
#   ./deploy.sh --dry-run               # preview the rsync plan + remote command
#
# Flags:
#   --env FILE   rsync FILE to the server as /opt/newapi/.env (secrets); off by default
#   --force      rebuild images + force-recreate containers and remove orphans
#   --dry-run    preview: rsync -n and echo the remote command without running it
#
# DEPLOY_PATH is fixed to /opt/newapi on the server.
# ---------------------------------------------------------------------------
set -euo pipefail

# Timestamped logging: every deploy message is prefixed with [YYYY-MM-DD HH:MM:SS].
log() {
  printf '[%s] [deploy] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}
log_err() {
  printf '[%s] [deploy] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >&2
}

DEPLOY_HOST="${DEPLOY_HOST:-}"
DEPLOY_PATH="/opt/newapi"
ENV_FILE=""   # empty = do not sync any .env (default); --env <file> sets it
DRY_RUN=0
FORCE=0

usage() {
  cat <<'EOF'
Usage: ./deploy.sh [--env FILE] [--force] [--dry-run]

Required environment:
  DEPLOY_HOST   SSH target, e.g. deploy@203.0.113.10

Fixed:
  DEPLOY_PATH   remote project root (always /opt/newapi)

Flags:
  --env FILE    rsync FILE to the server as /opt/newapi/.env (secrets); off by default
  --force       rebuild images + force-recreate containers and remove orphans
  --dry-run     preview: rsync -n and echo the remote command without running it
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --env)
      [ $# -ge 2 ] || { log_err "--env requires a file path"; usage >&2; exit 2; }
      ENV_FILE="$2"
      shift 2
      ;;
    --dry-run)  DRY_RUN=1; shift ;;
    --force)    FORCE=1; shift ;;
    -h|--help)  usage; exit 0 ;;
    *) log_err "unknown argument: $1"; usage >&2; exit 2 ;;
  esac
done

[ -n "$DEPLOY_HOST" ] || { log_err "error: DEPLOY_HOST is required"; usage >&2; exit 1; }

for tool in go rsync ssh; do
  command -v "$tool" >/dev/null 2>&1 || { log_err "error: '$tool' not found on PATH"; exit 1; }
done

# --- 1. compile & test locally ------------------------------------------------
log "build Go root module"
go build ./...

log "build relaykit module (independent build check)"
# relaykit must stay independently buildable (see AGENTS.md); a successful
# root-module build does not prove that, so build it with the workspace off.
(cd relaykit && GOWORK=off go build ./...)

# --- 2. rsync source tree to server (mirror) -----------------------------------
# Excludes mirror .dockerignore/.gitignore so runtime data and secrets survive the
# --delete (rsync never touches excluded paths). The web/ frontend sources are
# REQUIRED: the docker image builds the frontend on the server.
RSYNC_EXCLUDES=(
  # vcs / toolchains / caches
  --exclude='.git'
  --exclude='.venv'
  --exclude='.worktrees'
  --exclude='node_modules'
  --exclude='**/node_modules'
  --exclude='web/dist'
  --exclude='.cache'
  --exclude='.gocache'
  --exclude='.gomodcache'
  --exclude='.gopath'
  --exclude='tiktoken_cache'
  --exclude='electron/node_modules'
  --exclude='electron/dist'

  # runtime data (survive --delete; live under /srv/newapi on the host)
  --exclude='logs/'
  --exclude='data/'
  --exclude='*.db'
  --exclude='*.db-journal'
  --exclude='*.db-shm'
  --exclude='*.db-wal'
  --exclude='upload/'

  # secrets — .env is transferred only via --env <file>; the .env.example
  # template is harmless and kept for reference on the server.
  --include='.env.example'
  --exclude='.env'
  --exclude='.env.*'

  # test code & dev tooling
  --exclude='e2e/'
  --exclude='.local-tests/'
  --exclude='.agents/'
  --exclude='.claude/'
  --exclude='.cursor/'
  --exclude='.review'
  --exclude='.vscode'
  --exclude='.zed'
  --exclude='.history'
  --exclude='.playwright-mcp'
  --exclude='skills-lock.json'
)

RSYNC_OPTS=(-az --delete)
if [ "$DRY_RUN" -eq 1 ]; then
  RSYNC_OPTS+=(-n)
fi

log "rsync -> ${DEPLOY_HOST}:${DEPLOY_PATH}"
rsync "${RSYNC_OPTS[@]}" "${RSYNC_EXCLUDES[@]}" ./ "${DEPLOY_HOST}:${DEPLOY_PATH}/"

# Opt-in .env transfer: only when --env <file> was given.
if [ -n "$ENV_FILE" ]; then
  [ -f "$ENV_FILE" ] || { log_err "env file not found: $ENV_FILE"; exit 1; }
  if [ "$DRY_RUN" -eq 1 ]; then
    log "would transfer $ENV_FILE -> ${DEPLOY_HOST}:${DEPLOY_PATH}/.env"
  else
    log "transferring $ENV_FILE -> ${DEPLOY_PATH}/.env"
    rsync "$ENV_FILE" "${DEPLOY_HOST}:${DEPLOY_PATH}/.env"
  fi
fi

# --- 3. rebuild & restart the remote stack --------------------------------------
REMOTE_CMD="cd ${DEPLOY_PATH} && ./start.sh --build"
[ "$FORCE" -eq 1 ] && REMOTE_CMD="cd ${DEPLOY_PATH} && ./start.sh --build --force"
if [ "$DRY_RUN" -eq 1 ]; then
  log "would run: ssh ${DEPLOY_HOST} \"$REMOTE_CMD\""
else
  log "restarting remote stack"
  ssh "$DEPLOY_HOST" "$REMOTE_CMD"
fi

log "done"
