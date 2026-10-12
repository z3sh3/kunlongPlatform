# Plan: 控制台五大新模块（数据市场 / 模型广场 / 智能体工坊 / 人才培养与认证 / 企业培育中心）

## Context

The deployment is being repositioned as a regional AI public-service platform. This iteration adds 5 new business modules to the console (控制台), each implemented as an **independent Go package** on the backend with **no cross-module dependencies** (shared code extracted into a common package), and as new console sidebar groups on the frontend, at the same level as the existing 常规 / 个人 / 管理员 groups.

Decisions confirmed with the user:
- **现有 API 功能名称改为「算力超市」：本轮先保持原样（deferred）**，不在本次改动范围内。
- Implementation depth: **scaffold + core CRUD** (data tables, list/create/update/delete, publish/review state machines runnable end-to-end). Fine-tuning job scheduling, dataset billing settlement, low-code agent orchestration editor, certificate issuance, etc. are interface/placeholder level only — documented as follow-up.
- Backend layout: **top-level `modules/` directory**, each package exposing only `RegisterRoutes()` + `Migrate()`; still uses existing infra (`model.DB`, `middleware.UserAuth/AdminAuth`, root `common`).
- Visibility: **all logged-in users can browse/apply; admins manage/publish/review** (same pattern as wallet/redemption codes).

## Architecture

### Backend — top-level `modules/`

```
modules/
  shared/            # 公共 package：分页、状态常量、通用响应（被 5 个模块依赖，不依赖任何模块）
    page.go          #   PageQuery + ParsePageQuery(c) + Paginate
    status.go        #   PublishStatus(draft/published/offline), ReviewStatus(pending/approved/rejected)
    respond.go       #   Ok(c, data) / Fail(c, code, msg) — gin.H {"success", "message", "data"} 约定
  datasetmarket/     # 数据市场
    model.go         #   Dataset, DatasetGrant
    handler.go       #   gin handlers（用户浏览/申请 + 管理员上架/审核）
    router.go        #   RegisterRoutes(rg), Migrate()
    handler_test.go  #   焦点回归测试（CRUD + 权限）
  modelsquare/       # 模型广场（后训练/微调）
  agentworkshop/     # 智能体工坊
  talent/            # 人才培养与认证
  enterprise/        # 企业培育中心
```

Each module package (except `shared`) imports ONLY: root `model` (DB), root `middleware` (auth), root `common` (if needed), `modules/shared`, and stdlib/gin. **Module packages never import each other.** Import-cycle safety: `model` never imports `modules/`, so `modules/* → model` is safe.

### Entities (all GORM, cross-DB safe: no dialect-specific types, no boolean default tags)

| Module | Tables | Key fields |
|---|---|---|
| datasetmarket | `datasets`, `dataset_grants` | Dataset: name, description, category, tags, format, size, file_url, cover_url, price_mode(free/paid), unit_price, status(0 draft/1 published/2 offline), user_id; Grant: dataset_id, user_id, status(pending/approved/rejected), expires_at, remark |
| modelsquare | `finetune_jobs` | name, base_model, method(lora/sft/full), dataset_refs(JSON text), hyper_params(JSON text), status(draft/submitted/queued/running/succeeded/failed/canceled as int), result_model, error_msg, user_id |
| agentworkshop | `agents` | name, description, icon, category, config(JSON text — 低代码编排占位), price_mode, unit_price, status(0 draft/1 published/2 offline), user_id |
| talent | `training_courses`, `training_enrollments` | Course: title, category(certification/consultation), description, content, schedule, capacity, price_mode, price, status; Enrollment: course_id, user_id, status(pending/approved/rejected/completed) |
| enterprise | `incubation_programs`, `incubation_applications` | Program: title, type(incubation/sme_support/gov_park), description, requirements, status; Application: program_id, user_id, company_name, contact_name, contact_phone, description, status(submitted/reviewing/approved/rejected), review_comment |

Follow existing model conventions in `model/` (int Id primary key, `gorm.DeletedAt` only if peers use it, timestamps as peers do).

### API surface (registered under `/api`)

Pattern mirrors `router/api-router.go`; each module defines its own sub-group in its `router.go`:

- `datasetmarket`: `GET /api/data-market/datasets` (published list, UserAuth), `GET /:id`, `POST /apply/:id` (申请授权使用), `GET /my-grants`, `POST /`, `PUT /:id`, `DELETE /:id` (owner or admin), admin: `GET /admin/list`, `PUT /admin/:id/status` (上架/下架), `PUT /admin/grants/:id/review`
- `modelsquare`: user CRUD on own `finetune/jobs` + `POST /:id/submit` / `/:id/cancel`; admin `GET /admin/list`, `PUT /admin/:id/status` (实际训练调度为占位，状态机先行)
- `agentworkshop`: `agents` CRUD (own), `POST /:id/publish-request`; admin publish/offline + list all
- `talent`: `courses` list/detail (UserAuth), `POST /courses/:id/enroll`, `GET /my-enrollments`; admin course CRUD + enrollment review
- `enterprise`: `programs` list/detail, `POST /applications` (user submit), `GET /my-applications`; admin program CRUD + application review

### Wiring

- **Routes**: new file `router/modules-router.go` — `SetModulesRouter(router *gin.Engine)` creates `/api`-level group (RouteTag, gzip already applied per-route-group; attach same base middlewares as `SetApiRouter` group: `middleware.RouteTag("api")`, `middleware.AccessTokenAudit()`, `middleware.GlobalAPIRateLimit()`), then calls each module's `RegisterRoutes`. Called from `router/main.go` next to `SetApiRouter`.
- **Migration**: `main.go` after `model.InitDB()` (~line 314) — call each module's `Migrate()` (each does `model.DB.AutoMigrate(&Entity{}, ...)`), abort on error, same as existing startup error handling. Do NOT add these models to `model/main.go`'s `migrateDB` (would create `model → modules` import cycle).
- **User id** after `middleware.UserAuth()`: `c.GetInt("id")`; admin gate: `middleware.AdminAuth()` (see `middleware/auth.go:100-112`).
- **JSON**: use `common.Marshal/Unmarshal` wrappers for any JSON marshal/unmarshal in business code (project rule).

### Frontend

- **Sidebar groups** (`web/src/hooks/use-sidebar-data.ts`): add 5 groups between `personal` and `admin`, each with its main entry (same level as 常规/个人/管理员):
  - `data-market` → title `t('Data Market')`, item `t('Datasets')` → `/data-market`, icon `Database`
  - `model-square` → `t('Model Training')` … item → `/model-training`, icon `BrainCircuit` (避开现有「模型广场 Model Square」/pricing 的命名冲突)
  - `agent-workshop` → item `/agent-workshop`, icon `Bot`
  - `talent-training` → item `/talent-training`, icon `GraduationCap`
  - `enterprise-incubation` → item `/enterprise-incubation`, icon `Building2`
- **Visibility config**: extend `URL_TO_CONFIG_MAP` + `DEFAULT_SIDEBAR_MODULES` in `web/src/hooks/use-sidebar-config.ts` so admins can toggle each new section (unmapped URLs default visible, but adding the mapping matches the existing pattern for `系统设置 → 维护` UI).
- **Routes**: `web/src/routes/_authenticated/{data-market,model-training,agent-workshop,talent-training,enterprise-incubation}/index.tsx` — `beforeLoad` requires login (redirect to `/403` like `redemption-codes/index.tsx`, but role gate = any logged-in user; admin tabs gated inside the page). `routeTree.gen.ts` regenerates automatically.
- **Features**: `web/src/features/{data-market,model-training,agent-workshop,talent-training,enterprise-incubation}/` each with `api.ts`, `types.ts`, `components/`, `index.tsx`. Single page per module with internal tabs (浏览列表 / 我的机会 / 管理[admin-only]). Copy the shape of `web/src/features/redemption-codes/` + its route file.
  - **Reuse (mandatory per web/AGENTS.md)**: `@/components/data-table` for lists, existing shared business components (ConfirmDialog, form components) — read `web/AGENTS.md` + `shadcn-ui` skill before implementation and search `web/src/components/` first.
  - Frontend API client: same axios instance pattern as `features/redemption-codes/api.ts` (`api.get/post/put` returning `{success, message, data}`).
- **i18n**: all new UI strings as English keys via `t()`; add keys + translations to all 7 locales (`web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json`); update `web/src/i18n/static-keys.ts` for nav titles; load the `i18n-translate` skill and run `bun run i18n:sync`. Chinese labels: 数据市场、模型广场(训练) shown as 「模型训练」in nav item but module group title 「模型广场」— final labels: group titles 数据市场 / 模型广场 / 智能体工坊 / 人才培养与认证 / 企业培育中心.

## Files to modify

**Backend (new)**: `modules/shared/{page.go,status.go,respond.go}`, `modules/{datasetmarket,modelsquare,agentworkshop,talent,enterprise}/{model.go,handler.go,router.go}` + one small `handler_test.go` per module package.
**Backend (modified)**: `router/main.go` (add `SetModulesRouter` call), `router/modules-router.go` (new), `main.go` (module migrations after `InitDB`).
**Frontend (new)**: 5 route files under `web/src/routes/_authenticated/`, 5 feature dirs under `web/src/features/`.
**Frontend (modified)**: `web/src/hooks/use-sidebar-data.ts`, `web/src/hooks/use-sidebar-config.ts`, `web/src/i18n/static-keys.ts`, `web/src/i18n/locales/*.json` (7 files).

## Steps

### Backend
- [x] 1. Create `modules/shared` (pagination parse + paginate, PublishStatus/ReviewStatus constants, Ok/Fail response helpers)
- [x] 2. `modules/datasetmarket`: entities + handlers (user browse/apply, admin publish/review) + `RegisterRoutes` + `Migrate`
- [x] 3. `modules/modelsquare` (finetune jobs CRUD + submit/cancel state machine)
- [x] 4. `modules/agentworkshop` (agents CRUD + publish request/admin review)
- [x] 5. `modules/talent` (courses + enrollments + admin review)
- [ ] 6. `modules/enterprise` (programs + applications + admin review)
- [x] 7. `router/modules-router.go` + call in `router/main.go`; migrations in `main.go`
- [x] 8. `gofmt` all new files; `go build ./...`; small focused tests per package (SQLite fixture following existing test conventions, `testify require/assert`) covering: auth gating (user vs admin routes), owner-scope CRUD, publish/review transitions
- [x] 9. Dependency-boundary check: `go list -deps ./modules/...` shows no `modules/<a>` depending on `modules/<b>` (a≠b), and no root `model`/`router` deps on module internals beyond the explicit registration points

### Frontend
- [x] 10. Read `web/AGENTS.md` + `shadcn-ui` skill; survey existing components before writing UI
- [x] 11. 5 feature dirs (api.ts / types.ts / index.tsx with tabs / components)
- [x] 12. 5 route files with login-guard `beforeLoad`
- [x] 13. Sidebar groups in `use-sidebar-data.ts` + config mapping in `use-sidebar-config.ts`
- [x] 14. i18n: load `i18n-translate` skill, add keys to 7 locales, update `static-keys.ts`, run `bun run i18n:sync`
- [x] 15. `bun run build` + lint + existing test suites for touched areas

## Verification

- **Build**: `go build ./...` at repo root; `go vet ./modules/... ./router/...`
- **Tests**: `go test ./modules/... ./router/...`
- **Database matrix (mandatory — new AutoMigrate tables)**: run startup against real SQLite (default), MySQL ≥5.7.8, and PostgreSQL ≥9.6 (e.g. `docker run` mysql:8 + postgres:16, `SQL_DSN=...`), each **twice** to prove idempotency; verify the 9 new tables exist with expected columns and restart preserves data. If any engine cannot be run in this environment, report the blocker explicitly in the handoff — do not claim DB compatibility.
- **Module independence**: grep/`go list` proof that no module imports another module.
- **Frontend**: `cd web && bun run build`, `bun run i18n:sync` clean, login and confirm 5 new sidebar groups appear for a normal user (browse + apply flows) and admin tabs/publish/review work; route guards redirect anonymous users.
- Per project rules the agent will not start servers/databases/browser automatically — DB matrix uses throwaway docker containers created specifically for verification, and final UI walkthrough is a documented manual step for the user.

## Out of scope (documented placeholders)

- Rename of existing API feature to 「算力超市」 (deferred per user decision)
- Dataset billing settlement (quota deduction on grant approval), fine-tuning scheduler integration, low-code orchestration editor, certificate generation, incubation case management
- Public (anonymous) marketplace pages — console modules are login-visible only for now
