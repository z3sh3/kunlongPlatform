# new-api Docker 部署改造计划（参照 musera）

## Context

参照 `~/Work/musera/` 的 docker compose 配置，改造 new-api 的容器化部署：

1. 加入 `cloudflared` 服务，通过 Cloudflare Tunnel 暴露服务
2. 持久化数据统一 bind-mount 到宿主机 `/srv/new-api/...` 目录（替代 named volume 与 `./data`、`./logs`）
3. 参照 musera 的 `deploy.sh` 模式编写部署脚本（deploy.sh + start.sh）
4. PostgreSQL 从 15 升级到 `postgres:18.6`（musera 同版本）——全新部署，无存量数据迁移
5. Dockerfile 构建流程加入国内镜像加速（Go module / bun npm / Debian apt）
6. 生成随机密码替换 `123456` 默认值，存入 `.env`（gitignored），提交 `.env.example` 模板

已确认决策：compose 镜像改为**本地 Dockerfile 构建**（服务器上 compose build）；**全新部署**无需数据迁移；采用 **deploy.sh + start.sh** 结构；**生成 .env + 提交 .env.example**。

## musera 参照要点（已调研）

- compose：`name:` 固定项目名；`postgres:18.6` 且需 `PGDATA: /var/lib/postgresql/data`（18 的默认 PGDATA 移到了 `/var/lib/postgresql/<major>/docker`，pin 回 bind-mount 路径）；redis 不暴露宿主端口；cloudflared 服务：`TUNNEL_TOKEN` + `command: tunnel --no-autoupdate --protocol http2 run`，ingress 在 CF Zero Trust 控制台配置
- `start.sh`：确保 PGDATA bind 目录存在且属主为 999:999（postgres:18 容器内用户）；`docker compose --env-file .env -p <name> up -d`；验证 DB role/database 存在（POSTGRES_* 只在首次 initdb 生效）
- `deploy.sh`：本地编译验证 → rsync 源码 mirror 到服务器（排除运行时数据/密钥，`--env` 传 .env）→ ssh 触发远程 start.sh
- 镜像加速：清华 Debian deb822 源、清华 PyPI。new-api 对应：`GOPROXY=https://goproxy.cn,direct`、bun 用 `bunfig.toml` 设 npmmirror registry、运行时 debian:bookworm-slim 换清华 bookworm 源

## Approach

### 1. 重写 `docker-compose.yml`

- `name: new-api` 固定项目名；去掉过时的 `version:` 行；使用默认网络（去掉自定义 network，如 musera）
- **postgres**：`postgres:18.6`，`POSTGRES_USER/PASSWORD/DB` 由 `.env` 插值，`PGDATA: /var/lib/postgresql/data`；bind mount `${PGDATA_DIR:-/srv/new-api/postgres}:/var/lib/postgresql/data`；healthcheck `pg_isready`；无宿主端口
- **redis**：`redis:8-alpine`（与 musera 一致，替代 `redis:latest`），`--requirepass ${REDIS_PASSWORD}`，healthcheck `redis-cli ping`（带 `-a`），无宿主端口
- **new-api**：改为 `build: context: . dockerfile: Dockerfile`（去掉 `image: calciumion/new-api:latest`），`restart: unless-stopped`；`SQL_DSN=postgresql://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}`、`REDIS_CONN_STRING=redis://:${REDIS_PASSWORD}@redis:6379`、`SESSION_SECRET`（多实例安全）、`TZ=Asia/Shanghai` 等从 `.env` 插值；volumes：
  - `${DATA_DIR:-/srv/new-api/data}:/data`
  - `${LOG_DIR:-/srv/new-api/log}:/app/logs`
  - ports 改为 `127.0.0.1:3000:3000`（仅本机调试用，外部流量走 tunnel）；healthcheck 沿用 `wget /api/status`
- **cloudflared**：`cloudflare/cloudflared:latest`，`depends_on: new-api`，`TUNNEL_TOKEN: ${TUNNEL_TOKEN:?required}`，`command: tunnel --no-autoupdate --protocol http2 run`；注释说明 ingress 在 CF 控制台指向 `http://new-api:3000`
- 保留 mysql / clickhouse 注释块（密码改为 `${VAR:?...}` 插值，路径注释说明对应 `/srv/new-api/...`），保留 LOG_SQL_DSN 等可选项注释
- 去掉 `volumes:` 顶级段（无 named volume 了）

### 2. 改 `Dockerfile` 加国内镜像

- bun builder stage（oven/bun）：`COPY web/package.json web/bun.lock ./` 后、`bun install` 前用 `RUN printf` 生成 `bunfig.toml`，设 `[install] registry = "https://registry.npmmirror.com"`（不改仓库内 web/，仅构建期生效）
- golang builder stage（golang:1.26.1-alpine）：`ENV GOPROXY=https://goproxy.cn,direct`
- runtime stage（debian:bookworm-slim）：新增 `docker/debian.sources`（bookworm 版 deb822 清华源，参照 musera 的 trixie 版改 suites 为 bookworm），`COPY` 到 `/etc/apt/sources.list.d/debian.sources` 后再 `apt-get update`

### 3. 新增 `start.sh`（服务器端，参照 musera）

- `--build` / `--force` 参数；`ENV_FILE`/`COMPOSE_FILE` 可覆盖；`compose()` 固定 `-p new-api`
- `POSTGRES_UID/GID=999`；确保 `${PGDATA_DIR:-/srv/new-api/postgres}` 存在且属主 999:999；确保 `${DATA_DIR:-/srv/new-api/data}`、`${LOG_DIR:-/srv/new-api/log}` 存在
- `compose up -d`；等待 pg_isready（30s）；`psql` 验证 role/database 存在，失败给出可操作的修复提示（同 musera）

### 4. 新增 `deploy.sh`（本地端，参照 musera）

- `DEPLOY_HOST` 必填环境变量；`DEPLOY_PATH=/opt/new-api` 固定；`--env FILE` / `--force` / `--dry-run` 参数
- 本地验证门槛：检查 go/rsync/ssh 在 PATH；`go build ./...` + `cd relaykit && GOWORK=off go build ./...`（前端在服务器 docker build 中构建，本地不重复）
- rsync mirror 排除：`.git`、`node_modules`（含 web/）、`web/dist`、`logs/`、`data/`、`*.db`、`.env*`（仅经 `--env` 传输）、`tiktoken_cache`、`.cache`、`.gomodcache`、`.gopath`、`docs/`、`e2e/`、`.agents/`、`.claude/` 等开发产物
- 远程执行 `cd /opt/new-api && ./start.sh --build`（`--force` 时加 force-recreate）

### 5. 新增 `.env` + `.env.example`

- `.env`（gitignored，已确认 .gitignore 含 `.env`）：
  - `POSTGRES_PASSWORD`、`REDIS_PASSWORD`：`openssl rand -hex 24` 生成（hex 字符，避免 DSN 转义问题）
  - `SESSION_SECRET`：`openssl rand -hex 32`
  - `POSTGRES_USER=newapi`、`POSTGRES_DB=new-api`
  - `TUNNEL_TOKEN=`（留空待填，compose 用 `:?required` 强制）
  - `PGDATA_DIR=/srv/new-api/postgres`、`DATA_DIR=/srv/new-api/data`、`LOG_DIR=/srv/new-api/log`
- `.env.example`（提交）：同结构，敏感值留空并附注释说明生成方式

## Files to modify

- `docker-compose.yml` — 重写（postgres 18.6、bind mounts、cloudflared、随机密码插值、本地构建）
- `Dockerfile` — 加 GOPROXY / bunfig npmmirror / 清华 apt 源
- `docker/debian.sources` — 新增（bookworm 清华 deb822 源）
- `start.sh` — 新增
- `deploy.sh` — 新增
- `.env` — 新增（随机密码，gitignored）
- `.env.example` — 新增

不改动：`docker-compose.dev.yml`（开发用，保持原样）。

## Reuse

- 直接参照并适配 musera 的 `docker-compose.yml` / `start.sh` / `deploy.sh` / `docker/debian.sources`（`~/Work/musera/`）
- new-api 现有 healthcheck（`wget /api/status`）与 `--log-dir /app/logs` 启动参数沿用

## Steps

- [x] 1. 生成随机密码（`openssl rand -hex`），写入 `.env`；编写 `.env.example`
- [x] 2. 重写 `docker-compose.yml`（含 cloudflared 服务、postgres 18.6、/srv bind mounts、密码插值、本地 build）
- [x] 3. 新增 `docker/debian.sources`（bookworm 清华源），修改 `Dockerfile` 三个构建阶段加国内镜像
- [x] 4. 编写 `start.sh`（/srv 目录准备 + compose up + DB 校验）
- [x] 5. 编写 `deploy.sh`（本地 go build 验证 + rsync + 远程 start.sh）
- [x] 6. 校验（见 Verification）

## Verification

- `docker compose --env-file .env -f docker-compose.yml config` 校验插值与语法（不启动容器）
- `bash -n deploy.sh start.sh` 语法检查
- `TUNNEL_TOKEN=x docker compose ... config` 确认 cloudflared 必填变量生效
- 实际部署由用户在服务器执行：`DEPLOY_HOST=... ./deploy.sh`，验证 postgres/new-api/cloudflared 三容器健康、tunnel 域名可达 `/api/status`
- 遵守项目规则：不自动启动服务器/数据库/浏览器
