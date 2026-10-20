# 智能体工坊「智能体」模块接入 Dify 现存 Agent

## Context（背景）

控制台「智能体工坊」(`/agent-workshop`) 的「智能体」模块目前展示 new-api 本地 `agents` 表
（`modules/agentworkshop/`），`config` 字段只是低代码编排占位，页面上没有任何真实的智能体。

实际创建智能体的地方是已部署的 Dify 控制台（独立端口 `http://<host>:8081`，见
`plans/integrate-dify-agent-dev.md`）。用户要求：**在「智能体」模块里直接查询并展示 Dify
中已存在的 agent（Dify App），不再维护第二套数据**。

已与用户确认的决策：

| 决策点 | 结论 |
| --- | --- |
| 展示形态 | 数据源直接改为 Dify；不做来源徽标；不存在两套数据 |
| Tab | 「浏览智能体」= 全部 Dify 应用；「我的智能体」= 按 Dify 集成账号的创建者过滤；**移除「管理」Tab** |
| 新建入口 | 不加新建入口（应用在 Dify 里创建） |
| 每项能力 | 只读列表 + 跳转 Dify 控制台对应应用页 |
| 认证方式 | 系统设置里填 Dify 账号密码，后端登录并缓存 token |
| 展示范围 | 全部应用，标注应用类型（chat / agent / workflow …） |
| 配置位置 | 系统设置新增「Dify 智能体服务」section |

## 已核实的技术事实（Dify 1.17.1 源码，`~/github/dify`）

### 接口
- 控制台 API 前缀 `/console/api`（`api/controllers/console/__init__.py`），内网端口 5001。
- 列表：`GET /console/api/apps?page&limit(≤100)&mode&name&is_created_by_me&sort_by`
  → `{page, limit, total, has_more, data: AppPartial[]}`（`api/fields/app_fields.py` `AppPagination`）。
- `AppPartial` 可用字段：`id, name, description, mode, icon_type, icon, icon_background,
  created_at(unixtime), updated_at(unixtime), created_by, create_user_name, access_mode`。
- `mode` 取值：`chat / advanced-chat / agent-chat / agent / workflow / completion /
  channel / rag-pipeline`（`AppMode`）。
- `mode` **查询参数**只接受 `completion, chat, advanced-chat, workflow, agent-chat,
  agent, channel, all`（`AppListMode`）——**`rag-pipeline` 不能作为筛选值**（会 400）。
- 官方“打开应用”跳转规则（`web/utils/app-redirection.ts`）：
  `agent` → `/agents/<bound_agent_id>/configure`；
  `workflow` / `advanced-chat` → `/app/<id>/workflow`；
  其余 → `/app/<id>/configuration`。

### 认证（关键坑）
- `GET /console/api/apps` 同样走 CSRF 校验（`api/libs/login.py` `login_required`
  对非 OPTIONS/HEAD 一律 `check_csrf_token`），所以**光带 Bearer 不行**。
- `POST /console/api/login`：密码只需 **base64**（`libs/encryption.py` `decrypt_field`
  只做 base64 decode，1.17 已无 RSA）；响应体只有 `{"result":"success"}`，
  token 全部通过 **Set-Cookie** 下发：`access_token`、`refresh_token`、`csrf_token`。
- 因此后续请求需要三件套：
  `Authorization: Bearer <access_token>` + `X-CSRF-Token: <csrf_token>` +
  `Cookie: csrf_token=<csrf_token>`（header 与 cookie 必须相等，见 `check_csrf_token`）。
- `access_token` 生命周期默认 60 分钟（`ACCESS_TOKEN_EXPIRE_MINUTES`）；登录有频率限制
  （`LoginRateLimitError`），必须缓存 token、避免每次请求登录。
- 未初始化时登录被 `@setup_required` 拒绝；邮箱密码登录被关闭时被
  `@email_password_login_enabled` 拒绝——两种情况都要给出可读错误。
- 应用列表的 `@console_account_admission(require_valid_enterprise_license=True)`
  在社区版返回 `LicenseStatus.NONE`，不拦截（已核实 `get_license_status`）。

### 网络
- dify 的 `api` 服务已挂到 `newapi_default`（`docker/dify/docker-compose.yaml` 补丁记录 #3），
  newapi 容器可直连；但服务别名只是通用的 `api`，需补一个显式别名 `dify-api`。
- `nginx`（8081 宿主端口）**没有**挂 `newapi_default`，所以后端走 `http://dify-api:5001`，
  前端跳转走公开地址 `http://<host>:8081`。

## Approach

### 数据流

```text
浏览器 ──GET /api/agent-workshop/dify/agents──> newapi
                                                │  读 OptionMap 配置
                                                │  Redis 缓存 access/csrf token
                                                └─GET /console/api/apps──> dify-api:5001
浏览器 ──点击「打开」──> http://<host>:8081/app/<id>/workflow  (新标签页)
```

Dify 是唯一数据源：不落库、不同步、不做来源标记；列表分页/搜索/类型筛选全部透传给 Dify。

### 后端

**配置项（存 `common.OptionMap`，走现有 `PUT /api/option/`）**

| key | 说明 | 备注 |
| --- | --- | --- |
| `DifyAPIBaseURL` | 内网 API 基址，如 `http://dify-api:5001` | 空 = 未接入 |
| `DifyConsoleURL` | 公开控制台地址，如 `http://192.168.1.10:8081` | 空则前端按 `getAgentDevUrl()` 推导 |
| `DifyAccountEmail` | Dify 控制台账号 | |
| `DifyAccountSecret` | Dify 控制台密码 | 后缀 `Secret` 会被 `controller/option.go:GetOptions` 过滤，按 SMTPToken 的“只写不可读”模式处理 |

**`modules/agentworkshop/dify.go`（新）**
- 配置读取（`common.OptionMapRWMutex` 保护读）。
- 会话缓存：登录后把 `{access_token, csrf_token, expires_at}` 写 Redis
  （`common.RedisSet/RedisGet`，key 带 `baseURL+email` 短哈希后缀，改配置自然失效，
  TTL 50 分钟）；`!common.RedisEnabled` 时退化为进程内 mutex 保护的内存缓存（单节点/测试）。
- `login()`：`POST {base}/console/api/login`，body
  `{"email":..., "password": base64(pwd), "remember_me": true}`，从 `Set-Cookie`
  解析 `access_token` / `csrf_token`。
- `listApps()`：`GET {base}/console/api/apps?...`，带三件套头；`401/403` 时清缓存、
  重登一次并重试一次；其余非 2xx 转成可读错误。
- 归一化输出（每项）：
  `id, name, description, mode, icon_type, icon, icon_background,
  created_at/updated_at (RFC3339 string), console_path`；
  `console_path` 按上面官方规则由后端算好（`agent` → `/agents`，
  `workflow|advanced-chat` → `/app/<id>/workflow`，其余 → `/app/<id>/configuration`）。
- 使用 `http.Client{Timeout: 10s}`；不关闭 TLS 校验。

**`modules/agentworkshop/handler.go`（改）**
- `ListDifyAgents`：`UserAuth`。`common.GetPageQuery(c)`（`p`/`page_size`）→ Dify 的
  `page`/`limit`（`page_size` 夹到 1..100）；`keyword` → `name`；`mode` 白名单校验
  （不含 `rag-pipeline`），默认 `all`；`mine=1` → `is_created_by_me=true`。
  返回 `common.PageInfo`（`SetTotal`/`SetItems`）+ `console_base_url`。
  错误用 `common.ApiErrorStatus(c, 200, common.NewMessage(key), gin.H{"code": ...})`：
  `dify_not_configured` / `dify_unreachable` / `dify_auth_failed`。
- `TestDifyConnection`：`AdminAuth`。登录 + 拉 1 条，返回 `{total, email}` 或具体失败原因，
  供设置页「测试连接」使用。

**`modules/agentworkshop/router.go`（改）**
- `GET  /api/agent-workshop/dify/agents`（userAuth）
- `POST /api/agent-workshop/admin/dify/test`（adminAuth）

**本地 `agents` 模块（不动）**：现有 CRUD/审核接口、`Agent` 模型与 `AutoMigrate` 保持原样，
只是页面上不再有入口；后续单独决定是否清理。

### 前端

**`web/src/features/agent-workshop/`**
- `index.tsx`：只保留两个 Tab（`browse` / `mine`），移除 admin Tab 分支与 `ROLE` 判断。
- `types.ts`：`DifyAgent`、`DifyAgentListParams`；保留 `ApiResponse` / `PagedData`。
- `api.ts`：`listDifyAgents()`、`testDifyConnection()`；删除本地 CRUD 函数。
- `components/dify-agents-list.tsx`（新）：
  - 列：图标+名称+描述、类型（Badge，`mode` 映射文案）、更新时间、操作（「打开」外链
    `console_base_url + console_path`，`target='_blank'` + `relnoopener`）。
  - 顶部：关键词搜索（防抖）+ 类型下拉（`Select`），均走服务端参数。
  - 状态：加载 / 空（Dify 里没有应用）/ 未接入（`code=dify_not_configured`，提示联系管理员）/
    连接失败（可重试）。
  - 复用 `StaticDataTable` + `ServerPagination`。
- 删除（页面已无使用、避免死代码）：`components/agents-list.tsx`、
  `components/agent-form-dialog.tsx`、`lib/schema.ts`、`constants.ts`、
  `__tests__/agents-list.test.tsx`。
- 新增测试 `__tests__/dify-agents-list.test.tsx`（沿用现有 mock 方式）：渲染行、mode→文案映射、
  未接入与空列表两种状态、外链地址拼接。

**`web/src/features/system-settings/`**
- `integrations/dify-settings-section.tsx`（新）：`DifyAPIBaseURL`、`DifyConsoleURL`、
  `DifyAccountEmail`、`DifyAccountSecret`（password 输入，占位「留空保持不变」）、
  「测试连接」按钮（调 `POST /admin/dify/test`，toast 结果），底部说明默认地址
  `http://dify-api:5001` 与端口 8081。
- `operations/section-registry.tsx`：新增 `dify` section（`titleKey: 'Dify Agent Service'`）。
- `operations/index.tsx` + `types.ts`（`OperationsSettings`）：补 4 个 key 的默认值 `''`。
- 复用 `SettingsSection` / `SettingsForm` / `useUpdateOption` / `handleServerError`。

### 部署

- `docker/dify/docker-compose.yaml`：`api` 服务的 `newapi_default` 改为映射形式并加别名
  `dify-api`，同步在文件头补丁清单里新增一条（#5），便于升级时重放。
- `start.sh`：dify 栈启动后多打印一行提示（内网 API 基址 `http://dify-api:5001`、
  控制台地址、以及「在 系统设置 → 运营 → Dify 智能体服务 里填账号密码」）。
- 不需要新增环境变量、不改根 `docker-compose.yml`。

### i18n
按 `i18n-translate` skill：**只能用 `web/scripts/add-missing-keys.mjs` 写入 7 个 locale**，
然后 `bun run i18n:sync`。新增 key 预计：

- 列表：`Type`、`All types`、`Open`、`Open in Dify`、`Updated`、`Search agents...`、
  `No agents found in Dify`、`Dify is not connected`、`Contact an administrator to connect Dify`、
  `Failed to load agents from Dify`、`Retry`
- 类型文案：`Chatbot`、`Agent`、`Agent Chat`、`Chatflow`、`Workflow`、`Text Completion`、
  `Channel`、`RAG Pipeline`
- 设置：`Dify Agent Service`、`Dify API Base URL`、`Dify Console URL`、`Dify account email`、
  `Dify account password`、`Leave blank to keep the current password`、`Test connection`、
  `Connection succeeded`、`View agents`
- 后端消息 key（同样进 locale）：`Dify agent service is not configured`、
  `Failed to connect to Dify`、`Dify rejected the configured account`

## Files to modify

### 后端（新）
- `modules/agentworkshop/dify.go`
- `modules/agentworkshop/dify_test.go`

### 后端（改）
- `modules/agentworkshop/handler.go`、`modules/agentworkshop/router.go`

### 前端（新）
- `web/src/features/agent-workshop/components/dify-agents-list.tsx`
- `web/src/features/agent-workshop/__tests__/dify-agents-list.test.tsx`
- `web/src/features/system-settings/integrations/dify-settings-section.tsx`

### 前端（改）
- `web/src/features/agent-workshop/{index.tsx,api.ts,types.ts}`
- `web/src/features/system-settings/operations/{index.tsx,section-registry.tsx}`
- `web/src/features/system-settings/types.ts`
- `web/src/i18n/locales/*.json`（经脚本写入）、`web/src/i18n/static-keys.ts`（如新增导航文案需要）

### 前端（删）
- `web/src/features/agent-workshop/components/{agents-list.tsx,agent-form-dialog.tsx}`
- `web/src/features/agent-workshop/lib/schema.ts`、`constants.ts`
- `web/src/features/agent-workshop/__tests__/agents-list.test.tsx`

### 部署（改）
- `docker/dify/docker-compose.yaml`（`dify-api` 网络别名 + 补丁清单）、`start.sh`（提示信息）

## Reuse（复用现有实现）

- 列表 UI：`@/components/data-table` 的 `StaticDataTable`、`ServerPagination`（现有 `agents-list.tsx` 同款用法）
- 分页与响应：`common.GetPageQuery` / `common.PageInfo` / `common.ApiSuccess` /
  `common.ApiErrorStatus` / `common.NewMessage`
- token 缓存：`common.RedisSet` / `common.RedisGet` / `common.RedisDel`（`common/redis.go`）
- 密钥写入式设置字段：`SMTPToken` 的既有模式（`GetOptions` 后缀过滤 + 前端默认 `''`）
- 设置页骨架：`SettingsSection` / `SettingsForm` / `useUpdateOption` / `handleServerError`
- 公开控制台地址推导：`web/src/lib/constants.ts:getAgentDevUrl()`（与侧边栏外链共用）
- 已有 `ApiResponse` / `PagedData` 类型与 `requireServerSuccess` 包装

## Steps

### 阶段 1 — 后端 Dify 客户端
- [x] 1. `dify.go`：配置读取 + 会话缓存（Redis / 内存双实现）+ `login` + `listApps`（含 401 重登一次）
- [x] 2. `handler.go`：`ListDifyAgents`（分页/搜索/类型/`mine` 透传、page_size 夹到 100、
      `rag-pipeline` 拒绝作为筛选值）+ `TestDifyConnection`
- [x] 3. `router.go` 注册两条路由
- [x] 4. `dify_test.go`：`httptest` 模拟 Dify（校验三件套请求头、401 后重登、模式→`console_path`、
      未配置/不可达的错误码），SQLite 无关、无需临时数据库

### 阶段 2 — 前端页面
- [x] 5. `types.ts` / `api.ts` 换成 Dify 数据源
- [x] 6. `dify-agents-list.tsx`（表格 + 搜索 + 类型筛选 + 四种状态）
- [x] 7. `index.tsx` 改为两个 Tab，移除 admin Tab；删除无用的本地 agents UI 文件
- [x] 8. `dify-agents-list.test.tsx`

### 阶段 3 — 管理端配置
- [x] 9. `dify-settings-section.tsx` + 注册到 `operations/section-registry.tsx` + 类型/默认值
- [x] 10. 「测试连接」按钮接 `POST /api/agent-workshop/admin/dify/test`

### 阶段 4 — 部署与文案
- [x] 11. `docker/dify/docker-compose.yaml` 加 `dify-api` 别名并更新补丁清单；`start.sh` 输出配置提示
- [x] 12. i18n：`add-missing-keys.mjs` 写入 7 语言 + `bun run i18n:sync`

## Verification

### 后端

1. `go build ./...`、`go vet ./modules/agentworkshop/...`
2. `go test ./modules/agentworkshop/...`（新增的 `dify_test.go`：登录三件套、401 重登、
   mode→path、未配置/不可达错误码）。本次不动表结构、不新增 `AutoMigrate`，
   option 走既有 `options` 表写入路径，因此不触发三库矩阵要求（如后续改 schema 需补做）。

### 前端

3. `cd web && bun run typecheck`、`bun run lint`、`bun run build`
4. `bun run test`（含新增 `dify-agents-list.test.tsx`）
5. `bun run i18n:sync` 无缺失 key

### 端到端（用户在服务器上执行；按项目约定不自动起服务）

6. `docker compose -p dify --project-directory docker/dify -f docker/dify/docker-compose.yaml config`
   —— 确认 `api` 服务 `newapi_default` 带 `dify-api` 别名
7. `./start.sh` 后 dify 栈 healthy；`docker exec newapi wget -qO- http://dify-api:5001/health` 通
8. 系统设置 → 运营 → Dify 智能体服务：填 `http://dify-api:5001` + `http://<host>:8081` +
   Dify 账号密码 →「测试连接」成功
9. 在 Dify 控制台创建 1 个 chatflow + 1 个 workflow 应用 → 智能体工坊「浏览智能体」可见两条，
   类型徽标正确，「打开」跳到 `/app/<id>/workflow` 等正确页面
10. 「我的智能体」按集成账号创建者过滤；用另一个 Dify 账号创建的应用只在「浏览」出现
11. 故障降级：停掉 dify `api` 容器 → 页面显示连接失败并可重试，不白屏；
    清空 `DifyAPIBaseURL` → 显示「未接入，请联系管理员」

## 风险与备注

- **Dify 账号密码以明文存 `options` 表**（与 `SMTPToken`、`TelegramBotToken` 一致）。
  建议用一个专门的 Dify 工作区成员/所有者账号，不要用个人账号；`DifyAccountSecret`
  因后缀命中过滤规则不会随 `GET /api/option/` 返回，前端按只写字段处理。
- Dify 登录有频率限制：必须依赖缓存，401 才重登，避免每次请求都登录。
- Dify 升级后需复核：登录字段编码方式（当前 base64）、CSRF 三件套、`AppPartial` 字段、
  `AppListMode` 白名单、`/app/<id>/{workflow,configuration}` 路径。
- 列表完全依赖 Dify 可用性；Dify 停机时「智能体」页不可用（不做本地缓存，因为不存在两套数据）。
- 应用页以新标签页打开后仍需在 Dify 侧已登录（两套账号体系，无 SSO），交付说明需写明。

## Out of scope

- 站内对话 / 走 new-api 计费调用 Dify 应用（需 Service API key 与计费设计）
- Dify 应用与 new-api 用户的身份映射 / SSO
- 本地 `agents` 表、`modules/agentworkshop` 现有 CRUD 与审核接口的清理
- 应用排序自定义、标签(tag)筛选、收藏
