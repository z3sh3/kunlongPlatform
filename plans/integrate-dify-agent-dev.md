# 集成 Dify（智能体开发）到智能体工坊

## Context（背景）

new-api 前端已有「智能体工坊」（Agent Workshop：`web/src/features/agent-workshop/`，侧边栏 `agent-workshop` 分区，后端 `modules/agentworkshop/`）。用户希望把 `~/github/dify/`（Dify **v1.17.1**，镜像均已发布）这个大模型应用开发平台集成进来，入口命名「智能体开发」：

- 以 Docker Compose 方式并入现有部署流程（`docker-compose.yml` + `start.sh`/`deploy.sh`）
- 复用现有 newapi 栈的 PostgreSQL / Redis 实例（不另起 dify 专属 db/redis）
- 模型供应商指向 new-api 网关（`http://newapi:3000/v1`），让 Dify 应用消耗平台额度
- 访问方式：**独立端口 `http://<主机>:8081`**（端口可配，不走子域名、不经 Caddy）
- 前端在侧边栏「智能体工坊」分区新增「智能体开发」入口（新标签页打开）
- 初始化密码与全部安全密钥**由部署脚本随机生成后直接写入 `docker/dify/.env`**，绝不使用上游公开默认值

**硬约束（已核实 Dify 源码）**：Dify 官方对控制台页面强制 `X-Frame-Options: DENY` + `frame-ancestors 'none'`（`web/proxy.ts`），控制台**无法 iframe 内嵌**，只能新标签页打开；Dify 不支持子路径（subpath）部署。

**INIT_PASSWORD 机制（已核实源码 `api/services/init_validation_service.py`）**：`INIT_PASSWORD` 是**初始化门槛密码**（防止部署完成到管理员创建之间被陌生人抢占）；验证通过后，首个访问者在控制台设置页创建真正的**管理员账号**（自选邮箱 + 密码，创建者即 workspace owner）。

## Dify 技术栈调研结论

| 组件 | 技术 | 镜像 |
|---|---|---|
| API | Python 3.13 / Flask + Gunicorn(gevent) | `langgenius/dify-api:1.17.1` |
| 异步任务 | Celery worker + worker_beat（复用 API 镜像） | 同上 |
| 协作 WebSocket | `api_websocket`（compose profile `collaboration`） | 同上 |
| Web 控制台 | Next.js | `langgenius/dify-web:1.17.1` |
| 插件运行时 | Go daemon | `langgenius/dify-plugin-daemon:0.6.10-local` |
| 代码沙箱 | Go + SSRF Squid 代理（`ssrf_proxy` / `agent_ssrf_proxy`） | `langgenius/dify-sandbox:0.2.15`、`ubuntu/squid` |
| Agent v2 后端 | `agent_backend` + `local_sandbox` | `langgenius/dify-agent-backend:1.17.1` 等 |
| 数据库 | PostgreSQL（库 `dify` + 插件库 `dify_plugin`） | —— 复用 newapi 的 postgres 18.6 |
| 缓存/队列 | Redis（db0/1/2） | —— 复用 newapi 的 redis 8 |
| 向量库 | 默认 Weaviate（compose profile `weaviate`） | `cr.weaviate.io/semitechnologies/weaviate:1.39.2` |
| 入口 | 官方 nginx 容器聚合 web/api 路由（模板渲染） | `nginx:latest` |

官方 compose 由模板自动生成，引用同目录的 `nginx/`、`ssrf_proxy/` 模板与 `envs/*.env`（均 `required: false`），卷挂在 `./volumes/` 下。

## 集成方案

### 总体架构

```
浏览器 ──> Caddy (:80/:443) ──> newapi:3000        （现状完全不变，Caddy 零改动）
浏览器 ──> http://<主机>:8081 ──> dify 栈的 nginx 容器（直接发布，不经 Caddy）
                                    └─ dify-api / dify-web（独立 compose project "dify"）

dify 栈 ──复用──> newapi 栈的 postgres（新建库 dify、dify_plugin）
                 └──────────> redis（db3=缓存、db1=celery、db2=agent，前缀 dify_）
dify 模型供应商 ──> http://newapi:3000/v1（SSRF 域名白名单放行 newapi）
```

- **独立 compose project**：`docker compose -p dify --project-directory docker/dify -f docker/dify/docker-compose.yaml up -d`。相对路径（`./volumes`、`./envs`、`./nginx`）全部落在 `docker/dify/` 内，与官方布局一致，升级时直接覆盖 vendored 文件。
- **共享网络**：newapi project 的 default network 实际名为 `newapi_default`（`-p newapi` 固定）。dify 栈中需要访问 postgres/redis 的服务（api、api_websocket、worker、worker_beat、plugin_daemon、agent_backend）和需要被 Caddy 反代的 nginx 加入该 external 网络。
- **跨 project 无 depends_on**：由 `start.sh` 保证顺序——先起 newapi 栈并等 postgres 健康，再建 dify/dify_plugin 库，最后起 dify 栈。

### 需人工确认的环境变量（用户重点关切）

**A. 人工必须确认/提供的：**

| 变量 | 说明 |
|---|---|
| `DIFY_ENABLED` | 是否启用 dify 栈（默认 `false`，在根 `.env` 里显式开启） |
| `AGENT_DEV_PORT` | Dify 对外端口（默认 `8081`），防火墙需放行 |
| `SSRF_PROXY_ALLOW_PRIVATE_DOMAINS=newapi` | 允许 Dify 经 SSRF 代理访问内网 new-api 网关（模型接入方案必需） |
| SMTP 邮件配置 | `MAIL_TYPE=smtp` + `SMTP_*`（`envs/core-services/shared.env`）；邀请成员/找回密码需要，不配则相关功能不可用 |
| `NEXT_PUBLIC_SOCKET_URL`（可选） | 工作流实时协作（collaboration）需要 `ws://<主机>:8081`；端口模式下主机名部署时不可知，**默认关闭协作服务**（`ENABLE_COLLABORATION_MODE=false`），需要时人工填写后再开启 |
| `TURNSTILE_SITE_KEY` / `PIP_MIRROR_URL` / `MARKETPLACE_ENABLED` | 可选：人机验证 / 国内 pip 镜像 / 国内网络下插件市场可达性 |

**B. 全部密钥由 `start.sh` 随机生成、直接写入 `docker/dify/.env`（不使用任何上游默认值；已存在则不覆盖）：**

`SECRET_KEY`、`INIT_PASSWORD`（初始化门槛密码）、`WEAVIATE_API_KEY`、`SANDBOX_API_KEY`、`PLUGIN_DAEMON_KEY`、`PLUGIN_DIFY_INNER_API_KEY`、`DIFY_AGENT_API_TOKEN`、`DIFY_AGENT_SERVER_SECRET_KEY`、`DIFY_AGENT_LOCAL_SANDBOX_AUTH_TOKEN`

生成方式：`openssl rand -base64 42`（与 Dify 官方文档建议一致）。

**管理员账号备注（随交付说明/PR 描述提供）：**

- `INIT_PASSWORD`（生成后记录在 `docker/dify/.env`，交付说明中告知管理员）：首次访问 `http://<主机>:8081` 时输入此密码通过初始化验证
- 真正的**管理员账号在首次初始化时创建**：验证后控制台要求设置邮箱 + 密码，创建者即 workspace owner（Dify 源码 `api/services/init_validation_service.py` 已核实此流程）

**C. 派生自现有根 `.env`（复用实例，start.sh 生成时注入，不重复存明文）：**

- `DB_HOST=postgres`、`DB_PORT=5432`、`DB_USERNAME/DB_PASSWORD` = 根 `.env` 的 `POSTGRES_USER/POSTGRES_PASSWORD`、`DB_DATABASE=dify`、`DB_PLUGIN_DATABASE=dify_plugin`
- `REDIS_HOST=redis`、`REDIS_PASSWORD` = 根 `.env` 的 `REDIS_PASSWORD`、`REDIS_DB=3`、`REDIS_KEY_PREFIX=dify_`、`CELERY_BROKER_URL=redis://:<pw>@redis:6379/1`、`DIFY_AGENT_REDIS_URL=...db/2`
- ⚠️ newapi 占用 redis db0，dify 全部错开到 db1/2/3；postgres 中 `dify`/`dify_plugin` 为独立数据库，与 `newapi` 库互不影响

**D. 端口部署模式下同源推导（无需人工填写，`start.sh` 渲染为空即可）：**

- `CONSOLE_API_URL` / `CONSOLE_WEB_URL` / `APP_API_URL` / `APP_WEB_URL` / `SERVICE_API_URL` / `FILES_URL` / `COOKIE_DOMAIN` —— 留空时 Dify Web 按同源相对路径调用 API（与上游 compose 开箱即用行为一致）

**安全备注**：Dify OSS 控制台账号体系独立（无 SSO/OIDC 对接 new-api），注册默认开放——初始化完成后应由管理员在 Dify 控制台设置里关闭开放注册，成员账号按需邀请（配 SMTP）。

## Files to modify

**新增（vendored，来自 `~/github/dify/docker/`，版本钉死 1.17.1）：**
- `docker/dify/docker-compose.yaml` — 官方文件 + 最小补丁（见步骤 2）
- `docker/dify/nginx/`（4 个模板/脚本）、`docker/dify/ssrf_proxy/`（模板/脚本）、`docker/dify/envs/`（全部 `*.env.example`）
- `docker/dify/.env.example` — **本项目模板**（占位符由 start.sh 渲染）
- `docker/dify/README.md` 不创建（遵守项目 docs 约定；变更说明写在本计划/PR 描述）

**修改：**
- `start.sh` — dify 启用门控 + 建库 + 随机生成密钥并渲染 `docker/dify/.env` + 起 dify 栈
- `.env.example`（根）— 新增 `DIFY_ENABLED`、`AGENT_DEV_PORT=8081` 等说明
- `.gitignore` / `deploy.sh` — 排除 `docker/dify/volumes/`、`docker/dify/.env`
- `web/src/components/layout/components/nav-group.tsx` — `SidebarMenuLink` 支持 http(s) 绝对 URL 渲染为 `<a target='_blank' rel='noopener noreferrer'>`
- `web/src/hooks/use-sidebar-data.ts` — `agent-workshop` 分区新增「智能体开发」项
- `web/src/hooks/use-sidebar-config.ts` — `DEFAULT_SIDEBAR_MODULES` 与 `URL_TO_CONFIG_MAP` 新增 `agentdev` 键（沿用现有合并逻辑，后端 `SidebarModulesAdmin` 是透传 JSON，无需 Go 侧枚举改动）
- `web/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json` — 新 key（实施时按 i18n-translate skill）

（Caddy、`docker-compose.yml` 均不需要改动——Dify 走独立端口直连，不经过 Caddy）

## Reuse（可复用的现有实现）

- `start.sh` 的 `env_value()` / `compose()` 辅助函数与 postgres 就绪等待循环（`pg_isready`）→ 建库逻辑直接复用此模式（`controller/misc.go` 之外的唯一后端触点为零：**本次不改 Go 代码**）
- `deploy.sh` 的 rsync 镜像同步天然覆盖 `docker/dify/`，远端仍调用 `start.sh`，部署链路自动生效
- 侧边栏外链渲染模式：`web/src/components/layout/components/nav-link-item.tsx` 中已有 `link.external` → `<a target='_blank'>` 的写法，nav-group 照搬
- 前端 URL 推导：`http://${window.location.hostname}:${AGENT_DEV_PORT}`（`AGENT_DEV_PORT=8081` 常量与部署侧同名变量保持一致，双向注释标注），无需后端下发
- SSRF 域名白名单：Dify 官方 `ssrf_proxy/docker-entrypoint.sh` 原生支持 `SSRF_PROXY_ALLOW_PRIVATE_DOMAINS`（dstdomain acl），零改动

## Steps

### 阶段 1 — 部署物料准备（docker/dify/）
- [x] 1. 从 `~/github/dify/docker/` vendor 必需文件到 `docker/dify/`：`docker-compose.yaml`、`nginx/`、`ssrf_proxy/`、`envs/`（不拷贝 certbot/、tidb/、elasticsearch/、couchbase-server/、iris/ 等未启用组件）
- [x] 2. 给 vendored `docker-compose.yaml` 打最小补丁并逐条记录（便于升级时重放）：
  - 删除 `db_postgres`、`db_mysql`、`redis` 三个服务及其在 `depends_on` 中的引用（`required: false` 依赖随服务删除一并清理）
  - `api`/`api_websocket`/`worker`/`worker_beat`/`plugin_daemon`/`agent_backend`/`nginx` 的 `networks` 追加 `newapi_default`；顶层声明 `newapi_default: { external: true, name: newapi_default }`
  - nginx 的 `ports` 改为 `"${AGENT_DEV_PORT:-8081}:80"`（发布到 `0.0.0.0`，与前端推导端口一致）
  - `COMPOSE_PROFILES` 固定为 `weaviate`（协作模式默认关闭；开启时加回 `collaboration` profile 并需人工配置 `NEXT_PUBLIC_SOCKET_URL`）
- [x] 3. 编写 `docker/dify/.env.example` 模板：B 类密钥留 `__GENERATED__` 占位（start.sh 用 `openssl rand -base64 42` 生成）、C 类留根 `.env` 占位、D 类留空（同源）、A 类留空并附中文注释
- [x] 4. `.gitignore` 与 `deploy.sh` rsync 排除项补充运行时产物

### 阶段 2 — 部署流程接入（start.sh）
- [x] 5. `start.sh`：新增 `DIFY_ENABLED` 门控块（newapi 栈健康之后执行）——
  - a. 等待 postgres 就绪后用 `compose exec -T postgres psql` 幂等建库：`SELECT 1 FROM pg_database WHERE datname='dify'` 不存在则 `CREATE DATABASE dify`，`dify_plugin` 同理
  - b. 由模板渲染 `docker/dify/.env`：`__GENERATED__` 占位符用 `openssl rand -base64 42` 生成随机值（含 `INIT_PASSWORD`）**直接写入文件**；已存在则只更新派生类变量，**绝不覆盖已生成的密钥**，也绝不回落到上游默认值
  - c. `docker compose -p dify --project-directory docker/dify -f docker/dify/docker-compose.yaml up -d`
- [x] 6. 根 `.env.example` 增加 `DIFY_ENABLED` / `AGENT_DEV_PORT` / Dify 相关说明段（含 INIT_PASSWORD 机制、SMTP、防火墙放行 8081 提醒）

### 阶段 3 — 前端「智能体开发」入口
- [x] 7. `nav-group.tsx`：`SidebarMenuLink`（及折叠子项如有需要）识别绝对 http(s) URL → 渲染外链
- [x] 8. `use-sidebar-data.ts`：`agent-workshop` 分区新增 `{ title: t('Agent Development'), url: <computed>, icon: ... }`，URL 由小工具函数推导（8081 与后端 `AGENT_DEV_PORT` 保持一致，常量注释双向标注）
- [x] 9. `use-sidebar-config.ts`：默认配置与 URL 映射新增 `agentdev`；验证管理员侧边栏开关可隐藏入口
- [x] 10. 按 i18n-translate skill 补全 7 种语言翻译（"Agent Development" → zh「智能体开发」等）

### 阶段 4 — 模型接入与联调
- [x] 11. 确认 `docker/dify/.env` 中 `SSRF_PROXY_ALLOW_PRIVATE_DOMAINS=newapi` 生效（squid 白名单），并在交付说明中写清控制台操作：创建平台令牌 → Dify「设置→模型供应商→OpenAI-API-compatible」→ Base URL `http://newapi:3000/v1` + 令牌
- [ ] 12. 在 Dify 控制台初始化管理员（输入生成的 `INIT_PASSWORD` → 创建管理员邮箱+密码账号）、关闭开放注册、验证一个最小 Chat 应用的编排与发布；交付说明中备注管理员账号与 `docker/dify/.env` 位置

## Verification

**部署侧（在有 docker 的服务器上，用户执行）：**
1. `docker compose -p dify --project-directory docker/dify -f docker/dify/docker-compose.yaml config` — 渲染检查：无 db/redis 服务、网络引用正确、密钥无上游默认值
2. `./start.sh` 后：`docker compose -p dify ps` 全部 healthy/running；`psql -l` 确认 `dify`/`dify_plugin` 库存在；重复执行 `./start.sh` 验证幂等（库不重建、密钥不重置）
3. 端口访问验证：`http://<主机或IP>:8081` 打开 Dify 控制台；首次访问输入 `docker/dify/.env` 中生成的 `INIT_PASSWORD` 通过初始化验证，再创建管理员账号（邮箱+密码）并能登录；侧边栏入口推导出相同地址
4. 密钥验证：`docker/dify/.env` 中不存在任何上游默认值（无 `difyai123456`、`WVF5YTha...`、`lYkiYYT6...` 等）；重复执行 `./start.sh` 后密钥不变
5. 数据隔离回归：`docker compose -p newapi logs newapi` 正常；newapi 的 `/api/status` 正常；redis `redis-cli -n 0/1/2/3 keys` 抽查 db0 无 dify 前缀键

**模型链路（联调，核心验收）：**
6. Dify 应用选 newapi 网关的模型发起对话 → new-api「日志」页出现对应消耗记录（经平台计费）——同时验证 SSRF 白名单放行

**前端：**
7. `cd web && bun run build` 通过；`bun run i18n:sync` 无缺失 key
8. 侧边栏「智能体工坊 → 智能体开发」新标签页打开 `http://<主机>:8081`；管理后台关闭 `agentdev` 模块后入口隐藏

## 风险与备注

- Dify 1.17.1 的 compose 为上游自动生成文件，升级 = 重新 vendor + 重放步骤 2 的补丁清单（补丁记录要写全）
- dify 栈约 9 个常驻容器（api/worker×2/web/nginx/sandbox/plugin_daemon/agent_backend/local_sandbox/weaviate + 2 个 squid；协作模式关闭后无 api_websocket），首次部署需评估服务器内存（约 2~3GB 增量）
- 8081 端口为明文 HTTP（登录凭据不加密传输），需提醒用户防火墙策略与部署环境风险
- 跨 project `docker compose down` 只下本 project；`start.sh` 不改动现有 newapi 的 up/down 语义，dify 栈仅由 `DIFY_ENABLED` 控制
