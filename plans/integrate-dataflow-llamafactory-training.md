# 集成 DataFlow + LLaMA Factory 实现真实大模型训练（模型训练模块）

## Context（背景）

平台「模型训练」模块目前是**纯占位**：

- 前端 `web/src/features/model-training/`（路由 `/model-training`，侧边栏 `model-square` 分区）只有任务列表 + 一个超参 JSON 文本框 + 管理员手动改状态。
- 后端 `/api/model-training/jobs` 的 CRUD 在 `modules/modelsquare/`，`handler.go` 注释明确写着 "Actual training dispatch is a follow-up"：`SubmitJob` 只把 `draft → submitted`，`queued → running → succeeded/failed` 全靠管理员在「All jobs」页手填。**没有任何真实训练发生。**

目标：把 `~/github/DataFlow`（PKU-DCAI，数据准备）与 `~/github/LlamaFactory`（LLaMA Factory，SFT/DPO/LoRA/全参训练）接进「模型训练」模块，让用户在控制台完成「数据集准备 → 配置微调 → 运行训练 → 查看日志 → 产出模型资产」全链路。

### 已确认的决策（用户选定）

| 议题 | 决策 |
|---|---|
| 训练服务形态 | **只连外部服务**：平台侧只做 HTTP 客户端 + 协议 + 设置项，不写进 `docker-compose.yml`、不发布镜像。由用户在 GPU 主机自行部署 |
| 服务实现归属 | **随仓交付参考实现源码** `training-service/`（Python/FastAPI），因为两个上游项目都没有训练用 REST API |
| DataFlow 暴露深度 | **精选 pipeline 模板 + 少量参数**（模板与参数 schema 由服务端声明，前端动态渲染表单） |
| 训练日志 | **SSE 实时流**（平台透传服务的日志流） |
| 产出模型 | **登记为模型资产 + 可选注册渠道**：本人可登记为模块内模型资产；推进全局模型目录 + 注册 channel 仅管理员 |
| 数据集来源 | **上传文件 + URL + 从「数据市场」选** |
| DataFlow 的 LLM 凭证 | **用提交用户的 token**（平台为任务临时签发专用 token） |

### ⚠️ 需要用户知晓的两个后果（不阻塞实施）

1. **DataFlow 清洗会真实消耗额度。** 用提交用户 token 调 `http://newapi:3000/v1`，走的是既有 relay 计费链路，token 消耗会计到该用户名下。这与「本轮不计费」中的**训练部分**并不冲突：本方案**不新增任何计费代码**，只对 GPU 训练时长做数值记录（`usage` JSON），不扣额度。若希望清洗也完全不扣额度，把凭证改成「管理员共用 Key」即可（协议不变，仅换签发逻辑）。
2. **Token 需交给外部服务。** 明文 token 会出现在平台 → 服务的请求体里，因此设置项中要求 URL 为 `https://` 或位于可信内网；服务端不得将 key 写入日志。

## 现状事实（已核实源码）

### 平台侧（new-api）

- 模块约定：`modules/<name>/`，每包只暴露 `RegisterRoutes(*gin.RouterGroup)` + `Migrate() error`；**模块间禁止互相 import**，公共代码在 `modules/shared/`。
- 注册点：`router/modules-router.go` → `SetModulesRouter`；迁移点：`modules/modules.go` → `MigrateAll()`。
- 认证：`middleware.UserAuth()` / `middleware.AdminAuth()`；`c.GetInt("id")` / `c.GetInt("role")`。
- 响应：`common.ApiSuccess` / `ApiError` / `ApiErrorMsg`（`{success,message,data}`）；分页 `common.GetPageQuery(c)`（`*PageInfo`，`SetItems/SetTotal`）。
- JSON 必须走 `common.Marshal/Unmarshal`（禁止直连 `encoding/json`）。
- 模块测试脚手架：`modules/shared/testauth`（`DoJSON(r, method, path, userId, role, body)`）。
- **外部 Python 服务集成范式**：`modules/agentworkshop/dify.go`（712 行）——配置读 `common.OptionMap`，`*Secret` 后缀被 `controller/option.go:93` 过滤成只写，错误用稳定 code（`dify_not_configured` 等），可选 Redis 缓存。
- Option 默认值注册在 `model/option.go` 的 `InitOptionMap()`（`DifyAPIBaseURL` 等就在第 89-92 行）。
- 设置 UI：`web/src/features/system-settings/operations/section-registry.tsx` → `integrations/dify-settings-section.tsx`、`integrations/ionet-deployment-settings-section.tsx`（含「测试连接」按钮范式）。
- Token 签发基础：`model.Token{...}.Insert()`、`common.GenerateKey()`、`Token.ExpiredTime/UnlimitedQuota/ModelLimits`。
- 模型目录：`model.Model`（`model/model_meta.go:35`，含 `Insert()/Update()`）；渠道：`model.Channel{...}.Insert()` 内部会 `AddAbilities()`（`model/channel.go:548`、`model/ability.go:215`）。
- 前端鉴权用 **header**（`getFreshAuthHeaders()`）而非纯 cookie → 原生 `EventSource` 不可用。
- 已有 SSE 逐行读流范式：`web/src/features/channels/components/dialogs/ollama-models-dialog.tsx:253`（`fetch` + `credentials:'include'` + `Accept: text/event-stream` + `AbortController` + `data: [DONE]`）。
- 后端 SSE 范式：`controller/channel.go:2183`（`text/event-stream` + `fmt.Fprintf(c.Writer, "data: %s\n\n")` + `Flush()`）。
- **无通用文件上传端点**（`controller/` 无 upload）；`datasetmarket.Dataset.FileUrl` 只是文本字段。multipart 解析范式：`common.ParseMultipartFormReusable(c)`。
- 部署：`docker-compose.yml`（project `newapi`）/ `start.sh`（`env_value()`、`__GEN_*__` 密钥渲染）/ `docker/Caddyfile`（全量反代 `newapi:3000`）。
- **本机无 GPU**（`nvidia-smi` 不存在）→ 训练必须在外部 GPU 主机。

### DataFlow 侧（`~/github/DataFlow`）

- 包名 `open-dataflow`，CLI `dataflow`；`requires-python >=3.7,<4`；依赖很重（torch、transformers、gradio、fastapi、uvicorn 已在 requirements 中）。
- Pipeline 组合方式（`test/test_general_text.py` 是最小范例）：
  ```python
  self.storage = FileStorage(first_entry_file_name=..., cache_path="./cache",
                             file_name_prefix="dataflow_cache_step", cache_type="jsonl")
  self.processor.run(storage=self.storage.step(), input_key='raw_content')
  ```
- 现成 pipeline：`dataflow/example/{KBCleaningPipeline,GeneralTextPipeline,CodePipeline,ReasoningPipeline,Text2SQLPipeline,PDF2VQAPipeline,AgenticRAGPipeline,SpeechTranscription,...}`。
- 算子分类：`dataflow/operators/{core_text,core_vision,core_speech,general_text,knowledge_cleaning,pdf2vqa,reasoning,text2sql,text_sft,text_pt,code,conversations,agentic_rag,chemistry}`。
- LLM 服务可直接指向本平台网关：`dataflow/serving/api_vlm_serving_openai.py` 等接受 `api_url` + `key_name_of_api_key`（环境变量名）+ `model_name`。
- DataFlow-WebUI 需另下载 zip（`dataflow webui`），**不在本地仓库内**，不纳入本次。

### LLaMA Factory 侧（`~/github/LlamaFactory`）

- CLI：`llamafactory-cli train <yaml>`（`src/llamafactory/cli.py` → `launcher.py`；`USE_V1=1` 走 `v1`）。
- 其他子命令：`export`（合并 LoRA）、`api`（OpenAI 风格推理）、`webui`、`chat`、`env`、`version`。
- **没有训练用 REST API**：`src/llamafactory/api/` 只是推理服务。
- 数据集注册：`data/dataset_info.json`；可用 `dataset_dir` 指向自定义目录（`hparams/data_args.py:38`）。
- SFT YAML 关键字段（`examples/train_lora/qwen3_lora_sft.yaml`）：`model_name_or_path / stage / finetuning_type / lora_rank / lora_target / dataset / template / cutoff_len / max_samples / output_dir / per_device_train_batch_size / gradient_accumulation_steps / learning_rate / num_train_epochs / lr_scheduler_type / warmup_ratio / bf16 / overwrite_output_dir / report_to`。
- `finetuning_type` 合法值：`lora | oft | freeze | full`（`hparams/finetuning_args.py:464`）；`output_dir` 非空且未设 `overwrite_output_dir` 会报错（`hparams/parser.py:615`）。

## Approach

### 总体架构

```
浏览器 ──> new-api (gin, :3000) ──> modules/modelsquare ──HTTP Bearer──> training-service (GPU 主机, :8083)
                                        │                                    ├─ DataFlow pipeline 模板执行
                                        ├─ DB: finetune_jobs (扩展)           │    └─ LLM 调用 → https://<平台>/v1（用户 token）
                                        │      training_datasets              └─ llamafactory-cli train <yaml> 子进程
                                        │      trained_models                        └─ 日志文件 ──SSE──> 平台 ──SSE──> 前端
                                        └─ 产出：model.Model（全局目录）+ model.Channel（管理员）
```

- 平台是唯一鉴权/授权边界；训练服务只在可信网络内被平台后端访问，**对浏览器不直接暴露**。
- 平台 DB 是业务真相（谁的数据集、谁的作业、谁的资产）；服务只持有执行状态与产物（`remote_id` 映射）。
- 作业状态**惰性同步**：仅在读取非终态作业时向服务拉一次（每作业 5s 内存 TTL），不新增后台 goroutine（模块层目前没有后台任务生命周期，`main.go` 也不为业务模块起 goroutine）。

### 模块归属

**扩展现有 `modules/modelsquare`**（`FinetuneJob` 结构名不变 → 表名 `finetune_jobs` 不变，`AutoMigrate` 只加列，无数据迁移）。理由：该包当前唯一内容就是 finetune job，路由前缀已是 `/model-training`，与既有计划的「模型广场 → 模型训练」归属一致；新增包会与它以同一路径冲突。

内部按职责分文件（不新增包）：

```
modules/modelsquare/
  model.go        # FinetuneJob(扩展) + TrainingDataset + TrainedModel
  handler.go      # 既有 jobs CRUD + 扩展校验
  training.go     # 训练服务 HTTP 客户端（仿 dify.go：OptionMap 配置、错误码、超时）
  prepare.go      # DataFlow：capabilities 代理、数据集上传/URL/市场、prepare 运行与查询
  streaming.go    # SSE 透传（作业日志、prepare 日志）
  artifact.go     # trained_models 登记 + 管理员注册 model.Model / channel
  sync.go         # 惰性状态同步（TTL 缓存）
  router.go       # 路由注册（扩展）
  handler_test.go # 既有测试 + 扩展
  training_test.go# 新：假训练服务（httptest）+ 客户端/协议/状态映射/SSE 测试
```

### 训练服务 HTTP 协议（v1，平台 → 服务）

鉴权：`Authorization: Bearer <TrainingServiceSecret>`，所有请求；错误统一 `{"code","message"}`。

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/v1/health` | 设置页「测试连接」：`{ok, versions:{dataflow,llamafactory}, gpus:[{name,memory_mb}]}` |
| GET | `/v1/capabilities` | `{pipelines:[{id,name,description,params:[{key,label,type,required,default,min,max,options,unit}]}], methods:[...], templates:[...]}` — **模板与参数 schema 的唯一来源**，Go 侧不硬编码模板 |
| POST | `/v1/datasets` | multipart `file` → `{id, name, size_bytes, format, rows}` |
| POST | `/v1/datasets/from-url` | `{url, name}` → 同上 |
| DELETE | `/v1/datasets/{id}` | 删除（best-effort） |
| POST | `/v1/datasets/{id}/prepare` | `{pipeline, params, llm:{base_url, model, api_key}}` → `{run_id}` |
| GET | `/v1/datasets/{id}/prepare/{run_id}` | `{status, progress, error, output:{ref, rows, size_bytes, preview:[...]}}` |
| GET | `/v1/datasets/{id}/prepare/{run_id}/logs?offset=N` | SSE，`data: {"offset","line"}`，结束 `data: [DONE]` |
| POST | `/v1/jobs` | `{name, base_model, method, hyper_params, dataset:{ref}, template?}` → `{id, status}` |
| GET | `/v1/jobs/{id}` | `{id, status, progress, error, output_dir, result_model, usage:{gpu_seconds,steps,tokens}}` |
| POST | `/v1/jobs/{id}/cancel` | 取消（SIGTERM → SIGKILL） |
| GET | `/v1/jobs/{id}/logs?offset=N` | SSE，同 prepare 日志 |
| DELETE | `/v1/jobs/{id}` | 清理（best-effort） |

状态字符串 → 平台 int：`pending|queued→2`、`running→3`、`succeeded→4`、`failed→5`、`canceled→6`。

### 参考实现 `training-service/`（随仓交付，不写 compose）

```
training-service/
  requirements.txt          # open-dataflow, llamafactory, fastapi, uvicorn
  app/main.py               # FastAPI 路由（上表），Bearer 校验
  app/settings.py           # 环境变量：端口、Bearer token、数据目录、LLaMA Factory 目录
  app/capabilities.py       # 精选 pipeline 模板注册表 + 参数校验
  app/pipelines.py          # DataFlow 模板执行器（子进程，隔离 cwd/cache）
  app/datasets.py           # 上传/URL 落盘、格式识别、行数统计
  app/training.py           # 生成 YAML + llamafactory-cli train 子进程、状态机、取消
  app/logs.py               # 每作业日志文件 + offset 读取（SSE 用）
  app/store.py              # 作业/数据集元数据 JSON 落盘（服务侧状态，重启可恢复）
```

- DataFlow 执行器以**子进程**方式跑（`python -m training_service.pipelines --template X --config yaml`），保证一次 DataFlow 崩溃不影响训练服务主进程，也便于 `rayorch` 之外的最简实现。
- 精选模板（首批 4 个，全部可由数据说话）：`kb_pdf_to_qa`（PDF/文档 → QA）、`text_clean_sft`（嘈杂文本/低质 QA → 清洗+改写 → SFT）、`code_sft`（代码仓库/代码题）、`reasoning_sft`（数学/推理增强）。每个模板只暴露 3-6 个参数（输入源、输出条数、LLM 模型、并发、语言/清洗强度）。
- 训练：由 `hyper_params` 渲染 YAML 写到 `<jobs>/<id>/train.yaml`，`output_dir=<jobs>/<id>/output`，`dataset_dir=<jobs>/<id>/data` + 生成的 `dataset_info.json`，`report_to: none`、`overwrite_output_dir: true`、`logging_steps` 固定小值以便日志密集。
- 运行说明（安装、启动、GPU/端口要求）写在**本计划文件与 PR 描述**里，不在 `training-service/` 下新建 README（遵守「不新增 docs 类文件」的项目规则）。

### 平台侧关键实现点

**1. 设置项（`model/option.go` `InitOptionMap()`，仿 Dify 四行）**

```go
common.OptionMap["TrainingServiceURL"] = ""              // 例 https://gpu-host:8083
common.OptionMap["TrainingServiceSecret"] = ""           // 只写（*Secret 被 controller 过滤）
common.OptionMap["TrainingServiceTimeoutSeconds"] = "60"
common.OptionMap["TrainingServiceTokenTTLHours"] = "168" // 临时 token 有效期
```
启用判定：`URL` 与 `Secret` 均非空（无独立 enabled 开关，减少状态不一致）。
设置 UI：`operations/section-registry.tsx` 新增「Model Training Service」段（`integrations/training-service-settings-section.tsx`，照 `ionet-deployment-settings-section.tsx` 的「测试连接」范式）。

**2. `training.go`：客户端**

- `trainingConfig{BaseURL, Secret, Timeout}` 从 OptionMap 读取（`common.OptionMapRWMutex`）。
- 稳定错误码：`training_service_not_configured` / `training_service_unreachable` / `training_service_auth_failed` / `training_service_error`，前端据此区分「未配置（联系管理员）」与「服务故障」。
- `http.Client` 复用 + 显式超时；带 `Authorization: Bearer`；响应体大小上限（防超大 JSON）。

**3. `prepare.go`：数据集与 DataFlow**

- `POST /datasets`：媒体类型白名单（`.jsonl/.json/.csv/.parquet/.txt/.zip/.pdf`）、单文件上限（`TrainingDatasetMaxMB`，默认 200）、流式转发（不整体读入内存），先建平台记录（`status=pending`），服务返回 `remote_id` 后置 `ready`；转发失败置 `failed` + `error_msg`。
- `POST /datasets/from-url`：URL 校验（`http/https`，拒绝内网地址以免 SSRF 打到平台自身），由服务下载。
- `POST /datasets/:id/prepare`：见下方 token 签发；把 `llm.api_key` 只放进请求体，**不落库、不写日志**；`token_id` 落库用于结束后回收。
- `GET /capabilities`：直接代理服务响应（失败时返回空 list + 错误码，前端显示「训练服务未配置」，不阻塞作业列表页）。

**4. 临时 token 签发（用提交用户的 token）**

```go
token := model.Token{
    UserId: userId, Name: fmt.Sprintf("model-training:%d", datasetId),
    Key: common.GenerateKey(), Status: common.TokenStatusEnabled,
    CreatedTime: now, ExpiredTime: now + TTL,
    UnlimitedQuota: true,            // 走用户额度，由既有 relay 计费链路扣减
    ModelLimitsEnabled: true, ModelLimits: cleaningModel, // 最小权限
    Group: userGroup,
}
```
prepare 终结（成功/失败/取消）时 best-effort `DeleteTokenById`（软删）。不新增任何计费代码，不读写 billing 字段。

**5. `sync.go`：惰性状态同步**

- `GetJob` / `ListJobs` / `AdminListJobs` 中，对 `status ∈ {submitted,queued,running}` 的作业调用服务刷新；每作业 5s TTL（`sync.Map` + 时间戳），单次列表最多刷新 20 个；服务不可达时忽略并保留库内状态（不把作业误标失败）。
- `status` 变更与 `output_dir/result_model/usage/finished_at` 一并回写；终态时回收 token。

**6. `artifact.go`：产出登记**

- 作业成功 → 用户在作业详情点「登记为模型资产」→ 写 `trained_models`（`name, base_model, method, artifact_ref, size_bytes, job_id, user_id`），`name` 唯一性在用户内校验。
- 管理员「注册到模型目录」：`model.Model{ModelName, Description, Tags}.Insert()/Update()`。
- 管理员「注册为渠道」：表单填 `base_url` + `key` + `group` + `models`（默认取资产 `name`）→ `model.Channel{Type: constant.ChannelTypeOpenAI, ...}.Insert()`（内部会建 abilities）。**仅管理员**，因为渠道是平台级资源。

**7. `streaming.go`：SSE 透传**

- 平台端点 `GET /jobs/:id/logs?offset=N` 与 `GET /datasets/:id/prepare/logs?offset=N`：校验归属 → 用 `http.Client` 向服务发起流式请求 → 逐行读取并 `fmt.Fprintf(c.Writer, "data: %s\n\n", line)` + `Flush()`；`c.Request.Context()` 取消时中止上游请求。
- 头：`text/event-stream` / `no-cache` / `X-Accel-Buffering: no`。
- 权限：作业/数据集 owner 或管理员；未授权直接 403 JSON（不进入流模式）。

**8. 路由（扩展 `router.go`，前缀 `/api/model-training`）**

```
GET    /capabilities                                user
GET    /datasets                                    user
POST   /datasets                                    user (multipart)
POST   /datasets/from-url                           user
DELETE /datasets/:id                                user
POST   /datasets/:id/prepare                        user
GET    /datasets/:id/prepare                        user
GET    /datasets/:id/prepare/logs                   user (SSE)

GET    /jobs  /jobs/:id                             user
POST   /jobs  PUT /jobs/:id  DELETE /jobs/:id        user
POST   /jobs/:id/submit  POST /jobs/:id/cancel       user
GET    /jobs/:id/logs                               user (SSE)
POST   /jobs/:id/artifacts                          user (登记资产)

GET    /admin/jobs                                  admin
PUT    /admin/jobs/:id/status                       admin (保留手动兜底)
POST   /admin/artifacts/:id/model                   admin (注册模型目录)
POST   /admin/artifacts/:id/channel                 admin (注册渠道)
POST   /admin/training-service/test                 admin (测试连接)
```

**9. 数据表变更（全部跨库安全：int/bigint/varchar/text，无方言特性、无布尔 default tag）**

`finetune_jobs` 新增列（`AutoMigrate` 加列）：
`dataflow_template varchar(64)`、`dataflow_params text`、`dataset_id int index`、`service_job_id varchar(64) index`、`progress int`、`output_dir varchar(512)`、`usage text`、`started_at bigint`、`finished_at bigint`。

新表：
- `training_datasets`：`user_id, name, source(upload|url|market), source_ref, format, size_bytes, rows, status, error_msg, remote_id, prepared(bool), prepared_ref, pipeline_id, pipeline_params, token_id`
- `trained_models`：`user_id, job_id, name, base_model, method, artifact_ref, size_bytes, status, channel_id, model_catalog_id`

**10. 前端改造（`web/src/features/model-training/`）**

页面结构（`index.tsx`，`SectionPageLayout` + `Tabs` 沿用现有写法）：

- `我的任务`：列表（复用 `StaticDataTable` + `ServerPagination`），列增加数据准备模板、进度条、耗时；行操作：编辑/提交/取消/查看日志/登记资产。
- `数据准备`：我的数据集列表 + 「新建数据集」流程（上传 / URL / 从数据市场选）→ 选模板（来自 `/capabilities` 动态表单）→ 选清洗模型 → 启动 → SSE 日志抽屉。
- `模型资产`：`trained_models` 列表。
- 管理员：`全部任务`、`服务状态`（健康检查 + 模型目录/渠道注册入口）。

作业详情用 Drawer（`@/components/drawer-layout`）承载「概览 / 配置 / 日志」三块；日志用 fetch+reader 流（照 `ollama-models-dialog.tsx`），失败可从 `offset` 续读。

`lib/schema.ts` 从「超参 JSON 文本框」改为**结构化字段**（`base_model / method / lora_rank / cutoff_len / epochs / learning_rate / batch_size / gradient_accumulation`），仍写回 `hyper_params`（`common.Marshal` 后的 JSON 字符串），与后端 opaque 字段兼容。

i18n：新增 key 全部 `t('English key')`，7 个 locale + `static-keys.ts`；按 `i18n-translate` skill 执行。

## Files to modify

**新增 — 参考服务**
- `training-service/requirements.txt`
- `training-service/app/{main,settings,capabilities,pipelines,datasets,training,logs,store}.py`

**新增 — 后端模块文件**
- `modules/modelsquare/{training,prepare,streaming,artifact,sync}.go`
- `modules/modelsquare/training_test.go`

**修改 — 后端**
- `modules/modelsquare/{model,handler,router}.go`、`modules/modelsquare/handler_test.go`
- `model/option.go`（`InitOptionMap()` 追加 4 个 TrainingService 键，紧邻 Dify 块）

**修改 — 前端**
- `web/src/features/model-training/{api,types,constants,index}.tsx|ts`
- `web/src/features/model-training/lib/schema.ts`
- `web/src/features/model-training/components/{jobs-list,job-form-dialog}.tsx`
- `web/src/features/model-training/components/` 新增：`datasets-panel.tsx`、`dataset-form-dialog.tsx`、`pipeline-param-form.tsx`、`job-detail-drawer.tsx`、`log-stream-viewer.tsx`、`artifacts-panel.tsx`、`admin-service-panel.tsx`（复用 `@/components/ui/*`、`@/components/data-table`、`@/components/empty-state`、`@/components/confirm-dialog`、`@/components/dialog`）
- `web/src/features/system-settings/operations/{index.tsx,types.ts,section-registry.tsx}`
- `web/src/features/system-settings/integrations/training-service-settings-section.tsx`（新）
- `web/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`、`web/src/i18n/static-keys.ts`

**不改动 & 为什么**
- `docker-compose.yml` / `docker/training/` / `start.sh`：用户选择「只连外部服务」。
- `modules/modules.go`、`router/modules-router.go`：沿用既有 `modelsquare` 注册点。
- `.env.example`：服务地址/密钥走控制台设置，不走 env。
- 不新增 `docs/` 下任何文件、不新增 `training-service/README.md`。

## Reuse（复用清单）

| 复用对象 | 路径 | 用途 |
|---|---|---|
| Dify 客户端范式 | `modules/agentworkshop/dify.go` | OptionMap 配置读取、`*Secret` 只写、稳定错误码、HTTP 超时 |
| Dify 设置段 | `web/src/features/system-settings/integrations/dify-settings-section.tsx` | 表单 + 字段校验结构 |
| IoNet 设置段 | `web/src/features/system-settings/integrations/ionet-deployment-settings-section.tsx` | 「测试连接」按钮 + 结果反馈 |
| Option 注册位置 | `model/option.go:89-92` | 追加 TrainingService 键 |
| Token 签发 | `model.Token{}.Insert()`、`common.GenerateKey()`、`model.DeleteTokenById` | 任务级临时 token |
| 模型目录 / 渠道 | `model.Model.Insert()`、`model.Channel.Insert()`（含 `AddAbilities`） | 产出模型注册 |
| 后端 SSE 写法 | `controller/channel.go:2183` | `text/event-stream` + `data: ...\n\n` + `Flush` |
| 前端 SSE 读流 | `web/src/features/channels/components/dialogs/ollama-models-dialog.tsx:253` | `fetch` + `getFreshAuthHeaders` + `AbortController` + `[DONE]` |
| 列表/分页 | `@/components/data-table`（`StaticDataTable`、`ServerPagination`） | 任务/数据集/资产列表 |
| 弹窗/抽屉/确认 | `@/components/dialog`、`@/components/drawer-layout`、`@/components/confirm-dialog` | 表单与销毁确认 |
| 空/加载/错误态 | `@/components/empty-state`、`@/components/loading-state`、`@/components/error-state` | 服务未配置/未就绪提示 |
| 模块测试脚手架 | `modules/shared/testauth` | handler 测试 |
| multipart 解析 | `common.ParseMultipartFormReusable(c)` | 上传端点 |
| 分页/响应 | `common.GetPageQuery`、`common.ApiSuccess/ApiError/ApiErrorMsg` | 全模块 |
| DataFlow 组合范式 | `~/github/DataFlow/test/test_general_text.py` | 服务端模板实现 |
| LLaMA Factory YAML 范式 | `~/github/LlamaFactory/examples/train_lora/qwen3_lora_sft.yaml` | 服务端 YAML 渲染 |

## Steps

### 阶段 1 — 参考训练服务（`training-service/`）
- [x] 1. `settings.py`：端口、Bearer token、`DATA_ROOT`、`LLAMAFACTORY_HOME`、`DATASET_MAX_MB`、并发上限；启动时校验目录可写
- [x] 2. `store.py`：作业/数据集元数据 JSON 落盘 + 进程内锁（重启恢复）
- [x] 3. `capabilities.py`：4 个精选 pipeline 模板 + 参数 schema + 参数校验（类型/范围/枚举）
- [x] 4. `datasets.py`：上传落盘（流式、大小上限、扩展名白名单）、URL 下载（超时/大小上限）、行数统计、`dataset_info.json` 生成
- [x] 5. `pipelines.py`：模板 → DataFlow pipeline 子进程执行；LLM 服务参数（`api_url`/`key_name_of_api_key`/`model_name`）从 prepare 请求注入环境变量（不落盘、不进日志）；输出 SFT jsonl + 统计
- [x] 6. `training.py`：YAML 渲染、`llamafactory-cli train` 子进程、状态机、进度解析（从日志 `loss` / `%`）、取消（SIGTERM→SIGKILL）、`output_dir` 产物扫描
- [x] 7. `logs.py`：每作业追加写日志文件 + `offset` 读取（SSE 用）
- [x] 8. `main.py`：上表全部端点 + Bearer 中间件 + SSE 生成器 + 错误 envelope

### 阶段 2 — 平台后端（`modules/modelsquare`）
- [x] 9. `model.go`：`FinetuneJob` 加列；新增 `TrainingDataset`、`TrainedModel`；`router.go` 的 `Migrate()` 纳入新表
- [x] 10. `training.go`：`trainingConfig` 加载、`doTrainingRequest`、错误码、`health()`、`capabilities()`
- [x] 11. `prepare.go`：数据集 CRUD + 上传/URL/market 三条入口 + prepare 启动/查询 + 临时 token 签发/回收
- [x] 12. `handler.go`：`SubmitJob` 改为真实提交（校验数据集 ready、超参合法性 → 调服务 → 回写 `service_job_id`/`queued`）；`CancelJob` 调服务
- [x] 13. `sync.go`：惰性同步（TTL + 上限），接入 `GetJob`/`ListJobs`/`AdminListJobs`
- [x] 14. `streaming.go`：两个 SSE 透传端点 + 归属校验 + 上游取消
- [x] 15. `artifact.go`：资产登记、管理员注册 `model.Model` / `model.Channel`
- [x] 16. `router.go`：新增全部路由（user / admin 分组）
- [x] 17. `model/option.go`：追加 TrainingService 四项默认值
- [x] 18. `gofmt` + `go build ./...` + `go vet ./modules/... ./router/...`
- [x] 19. 测试（`handler_test.go` 扩展 + `training_test.go` 新增，同一包内两个文件，不跨层建测试文件）：
  - 假训练服务（`httptest.Server`）覆盖：未配置/不可达/鉴权失败的错误码、提交成功回写 `service_job_id`、状态映射、取消、惰性同步 TTL
  - 归属与权限：非 owner 403、admin 路由 gate、他人数据集不可用于自己作业
  - 上传：类型白名单与大小上限拒绝
  - SSE：`data: ...` 行格式与 `[DONE]`、未授权不进流模式
  - 临时 token：prepare 时签发且限制 `ModelLimits`，终结后删除
  - 全部用 `testify require/assert`，显式初始化 DB/context
- [x] 20. 依赖边界检查：`go list -deps ./modules/...` 证明无模块间 import；模块不 import `controller`

### 阶段 3 — 前端
- [x] 21. 读 `web/AGENTS.md` + `shadcn-ui` skill，检索 `web/src/components/` 与 `features/model-training/`，确认复用候选（已在 Reuse 表列出，实施前复核）
- [x] 22. `api.ts` / `types.ts` / `constants.ts`：新增 capabilities、datasets、prepare、logs、artifacts、admin 端点；SSE 用 fetch+reader 封装
- [x] 23. `lib/schema.ts`：结构化超参 schema（zod），保持写回 `hyper_params` JSON 字符串
- [x] 24. 组件：`datasets-panel`、`dataset-form-dialog`（上传/URL/市场三选一）、`pipeline-param-form`（按 capabilities 动态渲染）、`job-detail-drawer`、`log-stream-viewer`、`artifacts-panel`、`admin-service-panel`
- [x] 25. `jobs-list.tsx` 扩展（进度、耗时、日志入口、登记资产）；`job-form-dialog.tsx` 改为结构化表单 + 数据集选择
- [x] 26. 设置段 `training-service-settings-section.tsx` + `section-registry.tsx` + `operations/{index,types}.ts` 接线
- [x] 27. i18n：加载 `i18n-translate` skill，7 locale 全量补齐 + `static-keys.ts` + `bun run i18n:sync`
- [x] 28. `bun run typecheck` / `bun run lint`（改动文件零 error）/ `bun run build`

### 阶段 4 — 收尾
- [x] 29. 按 `.agents/rules/billing.md` 复核结论记录：本次不读写 `PriceData`/配额预扣/结算/消费日志字段，不新增计费代码（DataFlow 的额度消耗完全由既有 relay 链路产生）
- [x] 30. 数据库矩阵验证（SQLite / MySQL ≥5.7.8 / PostgreSQL ≥9.6）：全新库迁移 + 由上一版本库升级 + 连续启动两次证明幂等，`finetune_jobs` 既有数据保留、新表与索引正确
- [x] 31. 按 `git-commit-message` skill 生成提交信息

## Verification

**自动化（本仓可执行）**
1. `go build ./... && go vet ./modules/... ./router/...`
2. `go test ./modules/... ./router/...`
3. `go list -deps ./modules/... | grep 'modules/'` — 仅 `modules/shared` 与自身
4. `cd web && bun run typecheck && bun run lint && bun run build`
5. `bun run i18n:sync` 无 diff
6. 数据库矩阵（强制，新增 AutoMigrate 表 + 加列）：用一次性容器起 MySQL/PostgreSQL + 本地 SQLite，分别验证「全新库」与「旧版库升级」，各启动两次；记录引擎版本、命令与结果到交付说明。任一引擎无法执行则显式报告阻塞，不声称兼容

**端到端（按项目规则不自动启动服务/数据库/浏览器，作为人工步骤交付）**
7. GPU 主机：`pip install -r training-service/requirements.txt` → 设 `TRAINING_SERVICE_TOKEN`/`DATA_ROOT`/`LLAMAFACTORY_HOME` → `uvicorn app.main:app --host 0.0.0.0 --port 8083`
8. 控制台 → 系统设置 → 运维 → Model Training Service：填 URL + Secret → 「测试连接」返回 DataFlow/LLaMA Factory 版本与 GPU 信息
9. 用普通用户走通：上传一份 PDF 或选数据市场数据集 → 选 `kb_pdf_to_qa` → 选清洗模型 → 启动，日志实时滚动 → 产出数据集 `ready`
10. 新建作业引用该数据集（lora，rank 8，1 epoch）→ 提交 → 状态自动 `queued→running→succeeded`（无需管理员手改）→ 日志页可见 loss → 产出 `output_dir` 与资产
11. 取消路径：running 中取消 → 服务进程终止 → 状态 `canceled`
12. 负向：停掉训练服务 → 提交报 `training_service_unreachable` 且既有作业状态不被误改；管理员无配置时前端显示「训练服务未配置」
13. 管理员：资产 → 注册到模型目录 → 渠道页可见；注册渠道后可对该渠道做连通性测试

## Risks / 已知取舍

- **产物搬运**：LoRA/全参权重可能达 GB 级，本方案只登记**服务侧路径**，不做下载/上传。若需平台托管权重，是独立的存储与配额议题（后续项）。
- **`finetune_jobs.dataset_refs` 保留**：老字段继续存在（前端不再作为主入口），避免破坏已有数据与 API 消费者。
- **惰性同步的可见延迟**：仅在读取作业时刷新（≤5s TTL）；不使用后台轮询，因此不刷新就看不到更新——这是刻意的（模块层无后台生命周期）。
- **模板在服务端**：`/capabilities` 不可用时前端无法新建数据准备/作业，只能查看历史记录；这是「服务为唯一模板来源」的代价，换取平台与服务的模板永不漂移。
- **训练服务是单点**：不做多 GPU 主机调度/队列分发，队列语义由服务内部串行或并发上限决定。
