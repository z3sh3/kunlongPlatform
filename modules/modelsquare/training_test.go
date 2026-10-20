package modelsquare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modules/shared/testauth"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTrainingService is a minimal stand-in for the Python training service. It
// records the requests the module sends and replies with the same contract, so
// the tests exercise the real HTTP client, status mapping and SSE proxy.
type fakeTrainingService struct {
	mu sync.Mutex

	server *httptest.Server
	secret string

	// jobStatus is what GET /v1/jobs/{id} reports for jobs it created.
	jobStatus string
	// prepareStatus is what GET .../prepare/{run} reports as the run status.
	prepareStatus string
	// prepareOutput is returned once prepareStatus reaches "succeeded".
	prepareOutput *servicePrepareOutput
	// failStatus, when non-zero, makes every request fail with that HTTP code.
	failStatus int
	// unreachable short-circuits every request by refusing the connection.
	unreachable bool

	requests    []string
	lastPrepare servicePrepareRequest
	lastJob     serviceJobRequest
	jobCancels  int
	datasetPost int
}

func newFakeTrainingService(t *testing.T) *fakeTrainingService {
	t.Helper()
	fake := &fakeTrainingService{
		secret:        "test-secret",
		jobStatus:     "running",
		prepareStatus: "running",
	}
	fake.server = httptest.NewServer(http.HandlerFunc(fake.handle))
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *fakeTrainingService) url() string {
	if f.unreachable {
		// A port that was just closed fails with a connection error.
		return "http://127.0.0.1:1"
	}
	return f.server.URL
}

func (f *fakeTrainingService) datasetPostCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.datasetPost
}

func (f *fakeTrainingService) recordedRequests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

func (f *fakeTrainingService) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (f *fakeTrainingService) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	failStatus := f.failStatus
	jobStatus := f.jobStatus
	prepareStatus := f.prepareStatus
	prepareOutput := f.prepareOutput
	secret := f.secret
	f.mu.Unlock()

	if r.Header.Get("Authorization") != "Bearer "+secret {
		f.writeJSON(w, http.StatusUnauthorized, map[string]any{"code": "unauthorized", "message": "bad token"})
		return
	}
	if failStatus != 0 {
		f.writeJSON(w, failStatus, map[string]any{"code": "internal_error", "message": "service exploded"})
		return
	}

	path := r.URL.Path
	switch {
	case path == "/v1/health":
		f.writeJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"versions": map[string]string{"dataflow": "1.0", "llamafactory": "2.0"},
			"gpus":     []map[string]any{{"name": "fake-gpu", "memory_mb": "81920"}},
			"limits":   map[string]any{"dataset_max_mb": 200},
			"active":   map[string]int{"jobs": 0, "prepares": 0},
		})
	case path == "/v1/capabilities":
		f.writeJSON(w, http.StatusOK, map[string]any{
			"pipelines": []map[string]any{{
				"id": "kb_pdf_to_qa", "name": "Documents to Q&A",
				"params": []map[string]any{{"key": "chunk_size", "type": "int", "default": 512}},
			}},
			"methods":      []string{"lora", "sft", "full"},
			"hyper_params": []map[string]any{{"key": "lora_rank", "type": "int", "default": 8}},
		})
	case path == "/v1/datasets" && r.Method == http.MethodPost:
		rows := int64(7)
		f.mu.Lock()
		f.datasetPost++
		f.mu.Unlock()
		f.writeJSON(w, http.StatusOK, map[string]any{
			"id": "ds-remote", "name": "uploaded", "file_name": "train.jsonl",
			"format": "jsonl", "size_bytes": 128, "rows": rows,
		})
	case path == "/v1/datasets/from-url":
		f.writeJSON(w, http.StatusOK, map[string]any{
			"id": "ds-url", "name": "from url", "file_name": "data.jsonl",
			"format": "jsonl", "size_bytes": 64, "rows": 3,
		})
	case strings.HasSuffix(path, "/prepare") && r.Method == http.MethodPost:
		var payload servicePrepareRequest
		_ = json.NewDecoder(r.Body).Decode(&payload)
		f.mu.Lock()
		f.lastPrepare = payload
		f.mu.Unlock()
		f.writeJSON(w, http.StatusOK, map[string]any{
			"id": "run-1", "dataset_id": "ds-remote", "status": "running",
		})
	case strings.Contains(path, "/prepare/") && r.Method == http.MethodGet:
		payload := map[string]any{
			"id": "run-1", "dataset_id": "ds-remote", "status": prepareStatus, "progress": 50,
		}
		if prepareOutput != nil && prepareStatus == "succeeded" {
			payload["output"] = prepareOutput
		}
		f.writeJSON(w, http.StatusOK, payload)
	case strings.HasSuffix(path, "/prepare/run-1/cancel"):
		f.writeJSON(w, http.StatusOK, map[string]any{"canceled": true})
	case path == "/v1/jobs" && r.Method == http.MethodPost:
		var payload serviceJobRequest
		_ = json.NewDecoder(r.Body).Decode(&payload)
		f.mu.Lock()
		f.lastJob = payload
		f.mu.Unlock()
		f.writeJSON(w, http.StatusOK, map[string]any{"id": "job-remote", "status": "running"})
	case strings.HasPrefix(path, "/v1/jobs/") && strings.HasSuffix(path, "/cancel"):
		f.mu.Lock()
		f.jobCancels++
		f.mu.Unlock()
		f.writeJSON(w, http.StatusOK, map[string]any{"canceled": true})
	case strings.HasPrefix(path, "/v1/jobs/") && strings.HasSuffix(path, "/logs"):
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "data: {\"offset\":1,\"line\":\"epoch 1 loss=0.5\"}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	case strings.HasPrefix(path, "/v1/jobs/") && r.Method == http.MethodGet:
		f.writeJSON(w, http.StatusOK, map[string]any{
			"id": "job-remote", "status": jobStatus, "progress": 42,
			"error":      statusError(jobStatus),
			"output_dir": "/data/jobs/job-remote/output",
			"artifact":   map[string]any{"output_dir": "/data/jobs/job-remote/output", "kind": "lora", "size_bytes": 4096, "file_count": 3},
			"usage":      map[string]any{"gpu_seconds": 120, "rows": 7},
		})
	case strings.HasPrefix(path, "/v1/jobs/") && r.Method == http.MethodDelete:
		f.writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	case strings.HasPrefix(path, "/v1/datasets/") && r.Method == http.MethodDelete:
		f.writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	default:
		f.writeJSON(w, http.StatusNotFound, map[string]any{"code": "not_found", "message": "unknown route " + path})
	}
}

func statusError(status string) string {
	if status == "failed" {
		return "llamafactory-cli exited with code 1"
	}
	return ""
}

// configureTrainingService points the module at the fake service and clears the
// cached refresh timestamps so each test starts from a clean slate.
func configureTrainingService(t *testing.T, baseURL string) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMap[OptionTrainingServiceURL] = baseURL
	common.OptionMap[OptionTrainingServiceSecret] = "test-secret"
	common.OptionMap[OptionTrainingServiceLLMBaseURL] = "http://gateway.test/v1"
	common.OptionMapRWMutex.Unlock()

	remoteRefreshTimes.Range(func(key, _ any) bool {
		remoteRefreshTimes.Delete(key)
		return true
	})
}

// clearTrainingService leaves the integration unconfigured.
func clearTrainingService(t *testing.T) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	delete(common.OptionMap, OptionTrainingServiceURL)
	delete(common.OptionMap, OptionTrainingServiceSecret)
	common.OptionMapRWMutex.Unlock()
	remoteRefreshTimes.Range(func(key, _ any) bool {
		remoteRefreshTimes.Delete(key)
		return true
	})
}

// uploadDatasetFile posts a multipart dataset as the given user.
func uploadDatasetFile(t *testing.T, r *gin.Engine, userId int, filename string, content []byte) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/model-training/datasets", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set(testauth.HeaderUserId, fmt.Sprintf("%d", userId))
	req.Header.Set(testauth.HeaderRole, fmt.Sprintf("%d", common.RoleCommonUser))

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	return recorder, resp
}

// doStreamRequest performs a request whose body is not JSON (an SSE stream) and
// returns the raw recorder, so the frames can be asserted verbatim.
func doStreamRequest(t *testing.T, r *gin.Engine, target string, userId int) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set(testauth.HeaderUserId, fmt.Sprintf("%d", userId))
	req.Header.Set(testauth.HeaderRole, fmt.Sprintf("%d", common.RoleCommonUser))
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)
	return recorder
}

// createReadyDataset inserts a dataset row owned by userId.
func createReadyDataset(t *testing.T, userId int) TrainingDataset {
	t.Helper()
	dataset := TrainingDataset{
		UserId: userId, Name: "train", Source: DatasetSourceUpload,
		Format: "jsonl", Status: DatasetStatusReady, RemoteId: "ds-remote", Rows: 5,
	}
	require.NoError(t, model.DB.Create(&dataset).Error)
	return dataset
}

func TestTrainingServiceClientErrors(t *testing.T) {
	fake := newFakeTrainingService(t)
	setupRouter(t)

	// Unconfigured: the console must be told to contact an administrator.
	clearTrainingService(t)
	_, resp, err := testauth.DoJSON(setupRouter(t), http.MethodGet, "/api/model-training/capabilities", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)
	assert.Equal(t, trainingCodeNotConfigured, resp["code"])

	// Unreachable service.
	configureTrainingService(t, "http://127.0.0.1:1")
	_, resp, err = testauth.DoJSON(setupRouter(t), http.MethodGet, "/api/model-training/capabilities", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)
	assert.Equal(t, trainingCodeUnreachable, resp["code"])

	// Wrong shared secret.
	fake.secret = "a-different-secret"
	configureTrainingService(t, fake.url())
	_, resp, err = testauth.DoJSON(setupRouter(t), http.MethodGet, "/api/model-training/capabilities", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)
	assert.Equal(t, trainingCodeAuthFailed, resp["code"])

	// A service-side failure surfaces its own message.
	fake.secret = "test-secret"
	fake.failStatus = http.StatusInternalServerError
	configureTrainingService(t, fake.url())
	_, resp, err = testauth.DoJSON(setupRouter(t), http.MethodGet, "/api/model-training/capabilities", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)
	assert.Equal(t, trainingCodeError, resp["code"])
	assert.Equal(t, "service exploded", resp["message"])
}

func TestTrainingCapabilitiesAndHealth(t *testing.T) {
	fake := newFakeTrainingService(t)
	r := setupRouter(t)
	configureTrainingService(t, fake.url())

	_, resp, err := testauth.DoJSON(r, http.MethodGet, "/api/model-training/capabilities", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	caps, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	pipelines, ok := caps["pipelines"].([]any)
	require.True(t, ok)
	require.Len(t, pipelines, 1)
	assert.Equal(t, "kb_pdf_to_qa", pipelines[0].(map[string]any)["id"])

	// The health probe is administrator-only.
	w, _, err := testauth.DoJSON(r, http.MethodPost, "/api/model-training/admin/training-service/test", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, w.Code)

	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/model-training/admin/training-service/test", 9, common.RoleAdminUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, data["configured"])
	health, ok := data["health"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1.0", health["versions"].(map[string]any)["dataflow"])
}

func TestDatasetIntake(t *testing.T) {
	fake := newFakeTrainingService(t)
	r := setupRouter(t)
	configureTrainingService(t, fake.url())

	_, resp := uploadDatasetFile(t, r, 1, "train.jsonl", []byte("{\"instruction\":\"a\"}\n"))
	requireSuccess(t, resp)
	dataset, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ds-remote", dataset["remote_id"])
	assert.Equal(t, float64(7), dataset["rows"])

	// Unauthenticated uploads never reach the service.
	recorder, _ := uploadDatasetFile(t, r, 0, "train.jsonl", []byte("x"))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)

	// A market pick is recorded with its own source but no cross-module import.
	_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/model-training/datasets/from-url", 1, common.RoleCommonUser,
		map[string]any{"url": "https://example.com/data.jsonl", "name": "market", "market_dataset_id": 5})
	require.NoError(t, err)
	requireSuccess(t, resp)
	fromURL, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, DatasetSourceMarket, fromURL["source"])
	assert.Equal(t, "https://example.com/data.jsonl", fromURL["source_ref"])

	// Loopback URLs are rejected before they reach the service.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/model-training/datasets/from-url", 1, common.RoleCommonUser,
		map[string]any{"url": "http://127.0.0.1:3000/internal"})
	require.NoError(t, err)
	requireFailure(t, resp)

	// Another user cannot read the dataset list of someone else.
	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/model-training/datasets", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	page := resp["data"].(map[string]any)
	assert.Equal(t, float64(0), page["total"])
}

func TestDatasetPrepareMintsAndRevokesToken(t *testing.T) {
	fake := newFakeTrainingService(t)
	r := setupRouter(t)
	configureTrainingService(t, fake.url())
	dataset := createReadyDataset(t, 1)

	_, resp, err := testauth.DoJSON(r, http.MethodPost, fmt.Sprintf("/api/model-training/datasets/%d/prepare", dataset.Id),
		1, common.RoleCommonUser, map[string]any{
			"pipeline": "kb_pdf_to_qa",
			"params":   map[string]any{"chunk_size": 256},
			"model":    "gpt-4o-mini",
		})
	require.NoError(t, err)
	requireSuccess(t, resp)

	// The service received the gateway URL and a token, never the platform secret.
	fake.mu.Lock()
	lastPrepare := fake.lastPrepare
	fake.mu.Unlock()
	assert.Equal(t, "kb_pdf_to_qa", lastPrepare.Pipeline)
	assert.Equal(t, "http://gateway.test/v1", lastPrepare.LLM.BaseURL)
	assert.Equal(t, "gpt-4o-mini", lastPrepare.LLM.Model)
	require.NotEmpty(t, lastPrepare.LLM.ApiKey)

	// The token is limited to the chosen cleaning model and belongs to the user.
	var token model.Token
	require.NoError(t, model.DB.Where("user_id = ?", 1).First(&token).Error)
	assert.True(t, token.ModelLimitsEnabled)
	assert.Equal(t, "gpt-4o-mini", token.ModelLimits)
	assert.True(t, token.UnlimitedQuota)

	// While the run is in flight the token stays valid.
	_, resp, err = testauth.DoJSON(r, http.MethodGet, fmt.Sprintf("/api/model-training/datasets/%d/prepare", dataset.Id),
		1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	assert.Equal(t, float64(JobStatusRunning), resp["data"].(map[string]any)["prepare_status"])

	// Once the run succeeds the produced dataset is recorded and the token is revoked.
	fake.mu.Lock()
	fake.prepareStatus = "succeeded"
	fake.prepareOutput = &servicePrepareOutput{DatasetId: "prepared-1", DatasetName: "clean", Rows: 12, SizeBytes: 900}
	fake.mu.Unlock()

	_, resp, err = testauth.DoJSON(r, http.MethodGet, fmt.Sprintf("/api/model-training/datasets/%d/prepare", dataset.Id),
		1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	prepared := resp["data"].(map[string]any)
	assert.Equal(t, true, prepared["prepared"])
	assert.Equal(t, "prepared-1", prepared["prepared_ref"])
	assert.Equal(t, float64(12), prepared["rows"])

	var remaining int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).Count(&remaining).Error)
	assert.Zero(t, remaining, "the data cleaning token must be revoked once the run ends")
}

func TestSubmitJobDispatchesToService(t *testing.T) {
	fake := newFakeTrainingService(t)
	r := setupRouter(t)
	configureTrainingService(t, fake.url())
	dataset := createReadyDataset(t, 1)

	_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/model-training/jobs", 1, common.RoleCommonUser,
		map[string]any{
			"name": "my job", "base_model": "Qwen/Qwen3-4B", "method": MethodLora,
			"dataset_id": dataset.Id, "hyper_params": `{"lora_rank":16}`,
		})
	require.NoError(t, err)
	requireSuccess(t, resp)
	jobId := int(resp["data"].(map[string]any)["id"].(float64))

	_, resp, err = testauth.DoJSON(r, http.MethodPost, fmt.Sprintf("/api/model-training/jobs/%d/submit", jobId),
		1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)

	// The service got the prepared dataset reference and typed hyper-parameters.
	fake.mu.Lock()
	lastJob := fake.lastJob
	fake.mu.Unlock()
	assert.Equal(t, "ds-remote", lastJob.DatasetId)
	assert.Equal(t, "Qwen/Qwen3-4B", lastJob.BaseModel)
	assert.Equal(t, float64(16), lastJob.HyperParams["lora_rank"])

	var job FinetuneJob
	require.NoError(t, model.DB.First(&job, jobId).Error)
	assert.Equal(t, JobStatusQueued, job.Status)
	assert.Equal(t, "job-remote", job.ServiceJobId)
	assert.NotZero(t, job.StartedAt)

	// Submitting a job without a dataset is rejected before any HTTP call.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/model-training/jobs", 1, common.RoleCommonUser,
		map[string]any{"name": "no data", "base_model": "Qwen/Qwen3-4B", "method": MethodLora})
	require.NoError(t, err)
	requireSuccess(t, resp)
	noDataId := int(resp["data"].(map[string]any)["id"].(float64))
	_, resp, err = testauth.DoJSON(r, http.MethodPost, fmt.Sprintf("/api/model-training/jobs/%d/submit", noDataId),
		1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)

	// Malformed hyper-parameters are rejected.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/model-training/jobs", 1, common.RoleCommonUser,
		map[string]any{"name": "bad hp", "base_model": "m", "method": MethodLora, "dataset_id": dataset.Id, "hyper_params": `[1,2]`})
	require.NoError(t, err)
	requireSuccess(t, resp)
	badHpId := int(resp["data"].(map[string]any)["id"].(float64))
	_, resp, err = testauth.DoJSON(r, http.MethodPost, fmt.Sprintf("/api/model-training/jobs/%d/submit", badHpId),
		1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)
}

func TestJobRemoteRefreshAndCancel(t *testing.T) {
	fake := newFakeTrainingService(t)
	r := setupRouter(t)
	configureTrainingService(t, fake.url())
	dataset := createReadyDataset(t, 1)

	job := FinetuneJob{
		UserId: 1, Name: "running job", BaseModel: "m", Method: MethodLora,
		DatasetId: dataset.Id, Status: JobStatusQueued, ServiceJobId: "job-remote",
	}
	require.NoError(t, model.DB.Create(&job).Error)

	// Reading the job pulls the remote state: the fake reports running.
	_, resp, err := testauth.DoJSON(r, http.MethodGet, fmt.Sprintf("/api/model-training/jobs/%d", job.Id), 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	refreshed := resp["data"].(map[string]any)
	assert.Equal(t, float64(JobStatusRunning), refreshed["status"])
	assert.Equal(t, float64(42), refreshed["progress"])

	// The refresh is rate limited: a second read inside the TTL makes no call.
	before := len(fake.recordedRequests())
	_, resp, err = testauth.DoJSON(r, http.MethodGet, fmt.Sprintf("/api/model-training/jobs/%d", job.Id), 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	assert.Equal(t, before, len(fake.recordedRequests()))

	// A terminal remote status finishes the job.
	fake.mu.Lock()
	fake.jobStatus = "succeeded"
	fake.mu.Unlock()
	remoteRefreshTimes.Range(func(key, _ any) bool {
		remoteRefreshTimes.Delete(key)
		return true
	})
	_, resp, err = testauth.DoJSON(r, http.MethodGet, fmt.Sprintf("/api/model-training/jobs/%d", job.Id), 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	finished := resp["data"].(map[string]any)
	assert.Equal(t, float64(JobStatusSucceeded), finished["status"])
	assert.NotEmpty(t, finished["output_dir"])
	assert.NotZero(t, finished["finished_at"])
	assert.Equal(t, "running job", finished["result_model"])

	// Cancel reaches the service and records the local state.
	cancelJob := FinetuneJob{
		UserId: 1, Name: "cancel me", BaseModel: "m", Method: MethodLora,
		DatasetId: dataset.Id, Status: JobStatusRunning, ServiceJobId: "job-remote",
	}
	require.NoError(t, model.DB.Create(&cancelJob).Error)
	_, resp, err = testauth.DoJSON(r, http.MethodPost, fmt.Sprintf("/api/model-training/jobs/%d/cancel", cancelJob.Id),
		1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	fake.mu.Lock()
	cancelCount := fake.jobCancels
	fake.mu.Unlock()
	assert.Equal(t, 1, cancelCount)

	var canceled FinetuneJob
	require.NoError(t, model.DB.First(&canceled, cancelJob.Id).Error)
	assert.Equal(t, JobStatusCanceled, canceled.Status)
}

func TestJobLogStreamIsProxied(t *testing.T) {
	fake := newFakeTrainingService(t)
	r := setupRouter(t)
	configureTrainingService(t, fake.url())

	job := FinetuneJob{
		UserId: 1, Name: "logs", BaseModel: "m", Method: MethodLora,
		Status: JobStatusRunning, ServiceJobId: "job-remote",
	}
	require.NoError(t, model.DB.Create(&job).Error)

	recorder := doStreamRequest(t, r, fmt.Sprintf("/api/model-training/jobs/%d/logs", job.Id), 1)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	body := recorder.Body.String()
	assert.Contains(t, body, "loss=0.5")
	assert.Contains(t, body, "data: [DONE]")

	// A job that was never submitted has no remote log.
	draft := FinetuneJob{UserId: 1, Name: "draft", BaseModel: "m", Method: MethodLora, Status: JobStatusDraft}
	require.NoError(t, model.DB.Create(&draft).Error)
	_, resp, err := testauth.DoJSON(r, http.MethodGet, fmt.Sprintf("/api/model-training/jobs/%d/logs", draft.Id), 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)
}

func TestArtifactRegistrationAndPromotion(t *testing.T) {
	fake := newFakeTrainingService(t)
	r := setupRouter(t)
	configureTrainingService(t, fake.url())

	job := FinetuneJob{
		UserId: 1, Name: "trained-lora", BaseModel: "Qwen/Qwen3-4B", Method: MethodLora,
		Status: JobStatusSucceeded, ResultModel: "trained-lora",
		OutputDir: "/data/jobs/job-remote/output",
		Usage:     `{"gpu_seconds":120,"artifact":{"kind":"lora","size_bytes":4096}}`,
	}
	require.NoError(t, model.DB.Create(&job).Error)

	_, resp, err := testauth.DoJSON(r, http.MethodPost, fmt.Sprintf("/api/model-training/jobs/%d/artifact", job.Id),
		1, common.RoleCommonUser, map[string]any{"name": "my-lora"})
	require.NoError(t, err)
	requireSuccess(t, resp)
	artifact := resp["data"].(map[string]any)
	assert.Equal(t, "my-lora", artifact["name"])
	assert.Equal(t, "lora", artifact["artifact_kind"])
	assert.Equal(t, float64(4096), artifact["size_bytes"])
	artifactId := int(artifact["id"].(float64))

	// Duplicate names are rejected for the same owner.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, fmt.Sprintf("/api/model-training/jobs/%d/artifact", job.Id),
		1, common.RoleCommonUser, map[string]any{"name": "my-lora"})
	require.NoError(t, err)
	requireFailure(t, resp)

	// Only succeeded jobs produce artifacts.
	draft := FinetuneJob{UserId: 1, Name: "d", BaseModel: "m", Method: MethodLora, Status: JobStatusRunning}
	require.NoError(t, model.DB.Create(&draft).Error)
	_, resp, err = testauth.DoJSON(r, http.MethodPost, fmt.Sprintf("/api/model-training/jobs/%d/artifact", draft.Id),
		1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)

	// Promotion to the platform catalog and to a channel is administrator-only.
	w, _, err := testauth.DoJSON(r, http.MethodPost, fmt.Sprintf("/api/model-training/admin/artifacts/%d/model", artifactId),
		1, common.RoleCommonUser, map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// The handler refuses to overwrite an existing catalog entry. The catalog row
	// is created directly: model.Model.Insert() needs the SQL dialect helpers that
	// only model.InitDB initialises, which are out of reach for a package test.
	require.NoError(t, model.DB.Create(&model.Model{
		ModelName: "my-lora", Status: 1, CreatedTime: common.GetTimestamp(),
	}).Error)
	_, resp, err = testauth.DoJSON(r, http.MethodPost, fmt.Sprintf("/api/model-training/admin/artifacts/%d/model", artifactId),
		9, common.RoleAdminUser, map[string]any{"description": "duplicate"})
	require.NoError(t, err)
	requireFailure(t, resp)

	// Registering an inference channel wires the artifact to a real channel.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, fmt.Sprintf("/api/model-training/admin/artifacts/%d/channel", artifactId),
		9, common.RoleAdminUser, map[string]any{"base_url": "http://gpu-host:8000/v1", "key": "sk-local", "group": "default"})
	require.NoError(t, err)
	requireSuccess(t, resp)
	channelData, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	require.NotZero(t, channelData["channel_id"])

	var linked TrainedModel
	require.NoError(t, model.DB.First(&linked, artifactId).Error)
	require.NotZero(t, linked.ChannelId)

	var channel model.Channel
	require.NoError(t, model.DB.First(&channel, linked.ChannelId).Error)
	assert.Equal(t, "my-lora", channel.Models)
	assert.Equal(t, "default", channel.Group)

	// A second channel for the same artifact is refused.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, fmt.Sprintf("/api/model-training/admin/artifacts/%d/channel", artifactId),
		9, common.RoleAdminUser, map[string]any{"base_url": "http://gpu-host:8000/v1", "key": "sk-local"})
	require.NoError(t, err)
	requireFailure(t, resp)

	// The artifact list is owner scoped.
	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/model-training/artifacts", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	page := resp["data"].(map[string]any)
	assert.Equal(t, float64(1), page["total"])
}

// demoCatalogueData unwraps the {configured, datasets} demo catalogue envelope.
func demoCatalogueData(t *testing.T, resp map[string]any) (bool, []any) {
	t.Helper()
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	datasets, ok := data["datasets"].([]any)
	require.True(t, ok)
	configured, ok := data["configured"].(bool)
	require.True(t, ok)
	return configured, datasets
}

func TestDemoDatasetCatalogueAndInstall(t *testing.T) {
	fake := newFakeTrainingService(t)
	r := setupRouter(t)
	configureTrainingService(t, fake.url())

	// The catalogue is served from the binary, without contacting the service.
	_, resp, err := testauth.DoJSON(r, http.MethodGet, "/api/model-training/demo-datasets", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	configured, catalogue := demoCatalogueData(t, resp)
	assert.True(t, configured, "the fake service is configured")
	require.Len(t, catalogue, 6, "the bundled catalogue is part of the demo contract")

	keys := make(map[string]bool, len(catalogue))
	trainable := 0
	for _, item := range catalogue {
		entry, isMap := item.(map[string]any)
		require.True(t, isMap)
		keys[entry["key"].(string)] = true
		assert.Equal(t, false, entry["installed"], "nothing is installed yet")
		if entry["suggested_pipeline"] == "" {
			trainable++
		}
	}
	assert.True(t, keys["customer-support-alpaca"])
	assert.True(t, keys["product-manual"])
	assert.Equal(t, 2, trainable, "two bundled sets train without data preparation")

	// Installing everything uploads each bundled file exactly once.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/model-training/demo-datasets/install", 1, common.RoleCommonUser, map[string]any{})
	require.NoError(t, err)
	requireSuccess(t, resp)
	installResult := resp["data"].(map[string]any)
	assert.Len(t, installResult["installed"].([]any), 6)
	assert.Empty(t, installResult["skipped"].([]any))
	assert.Empty(t, installResult["failed"].([]any))

	var stored int64
	require.NoError(t, model.DB.Model(&TrainingDataset{}).Where("user_id = ?", 1).Count(&stored).Error)
	assert.Equal(t, int64(6), stored)

	// Provenance records which demo set produced the row.
	var demoRow TrainingDataset
	require.NoError(t, model.DB.Where("source_ref = ?", "demo:product-manual").First(&demoRow).Error)
	assert.Equal(t, "Demo raw: product manual (document)", demoRow.Name)
	assert.Equal(t, DatasetStatusReady, demoRow.Status)

	// Re-installing skips everything and does not call the service again.
	uploadsBefore := fake.datasetPostCount()
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/model-training/demo-datasets/install", 1, common.RoleCommonUser, map[string]any{})
	require.NoError(t, err)
	requireSuccess(t, resp)
	second := resp["data"].(map[string]any)
	assert.Empty(t, second["installed"].([]any))
	assert.Len(t, second["skipped"].([]any), 6)
	assert.Equal(t, uploadsBefore, fake.datasetPostCount(), "skipped installs must not re-upload")

	// The catalogue now reports the installed state per user.
	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/model-training/demo-datasets", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	_, reloaded := demoCatalogueData(t, resp)
	for _, item := range reloaded {
		assert.Equal(t, true, item.(map[string]any)["installed"])
	}

	// Another user starts from an empty catalogue state.
	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/model-training/demo-datasets", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	_, otherUser := demoCatalogueData(t, resp)
	for _, item := range otherUser {
		assert.Equal(t, false, item.(map[string]any)["installed"], "install state is per user")
	}

	// Unknown keys are rejected instead of silently ignored.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/model-training/demo-datasets/install", 1, common.RoleCommonUser,
		map[string]any{"keys": []string{"not-a-demo"}})
	require.NoError(t, err)
	requireFailure(t, resp)
}

func TestDemoDatasetsRequireConfiguredService(t *testing.T) {
	fake := newFakeTrainingService(t)
	r := setupRouter(t)
	configureTrainingService(t, fake.url())
	clearTrainingService(t)

	// The catalogue stays readable and reports the missing configuration so the
	// console can explain what to set up instead of showing a bare error.
	_, resp, err := testauth.DoJSON(r, http.MethodGet, "/api/model-training/demo-datasets", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	configured, catalogue := demoCatalogueData(t, resp)
	assert.False(t, configured, "an unconfigured service must be reported")
	assert.Len(t, catalogue, 6)

	// Installing is what needs the service.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/model-training/demo-datasets/install", 1, common.RoleCommonUser, map[string]any{})
	require.NoError(t, err)
	requireFailure(t, resp)
	assert.Equal(t, trainingCodeNotConfigured, resp["code"])
}

// TestDemoModeRunsWholeFlowInProcess proves the console flow works with no GPU
// and no external training host: the gateway serves the v1 contract itself.
func TestDemoModeRunsWholeFlowInProcess(t *testing.T) {
	r := setupRouter(t)
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMap[OptionTrainingServiceDemoMode] = "true"
	common.OptionMap[OptionTrainingServiceDemoDir] = t.TempDir()
	common.OptionMapRWMutex.Unlock()
	clearTrainingService(t)
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		delete(common.OptionMap, OptionTrainingServiceDemoMode)
		delete(common.OptionMap, OptionTrainingServiceDemoDir)
		common.OptionMapRWMutex.Unlock()
		closeDemoTrainingServer()
	})

	// The catalogue reports itself installable without any remote configuration.
	_, resp, err := testauth.DoJSON(r, http.MethodGet, "/api/model-training/demo-datasets", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	configured, _ := demoCatalogueData(t, resp)
	assert.True(t, configured, "demo mode must count as configured")

	// 1) Install the bundled datasets through the in-process service.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/model-training/demo-datasets/install", 1, common.RoleCommonUser, map[string]any{})
	require.NoError(t, err)
	requireSuccess(t, resp)
	installed := resp["data"].(map[string]any)["installed"].([]any)
	require.Len(t, installed, 6)
	datasetId := int(installed[0].(map[string]any)["id"].(float64))

	// 2) Run a pipeline: no cleaning model and no gateway address are required.
	preparePath := fmt.Sprintf("/api/model-training/datasets/%d/prepare", datasetId)
	_, resp, err = testauth.DoJSON(r, http.MethodPost, preparePath, 1, common.RoleCommonUser, map[string]any{
		"pipeline": "text_clean_sft",
		"params":   map[string]any{"max_samples": 8},
		"model":    "",
	})
	require.NoError(t, err)
	requireSuccess(t, resp)

	deadline := time.Now().Add(30 * time.Second)
	var prepared map[string]any
	for time.Now().Before(deadline) {
		_, resp, err = testauth.DoJSON(r, http.MethodGet, preparePath, 1, common.RoleCommonUser, nil)
		require.NoError(t, err)
		requireSuccess(t, resp)
		prepared = resp["data"].(map[string]any)
		if prepared["prepare_status"].(float64) == float64(JobStatusSucceeded) {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	require.Equal(t, float64(JobStatusSucceeded), prepared["prepare_status"], "prepare error: %v", prepared["prepare_error"])
	assert.Equal(t, true, prepared["prepared"])
	assert.NotEmpty(t, prepared["prepared_ref"], "the produced dataset reference must be recorded")

	// 3) Create and submit a job against the prepared dataset.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/model-training/jobs", 1, common.RoleCommonUser, map[string]any{
		"name":         "demo-lora",
		"base_model":   "Qwen/Qwen3-4B-Instruct-2507",
		"method":       MethodLora,
		"dataset_id":   datasetId,
		"hyper_params": `{"lora_rank":8,"num_train_epochs":1,"cutoff_len":1024,"learning_rate":0.0001,"per_device_train_batch_size":1,"gradient_accumulation_steps":8,"bf16":false}`,
	})
	require.NoError(t, err)
	requireSuccess(t, resp)
	jobId := int(resp["data"].(map[string]any)["id"].(float64))

	_, resp, err = testauth.DoJSON(r, http.MethodPost, fmt.Sprintf("/api/model-training/jobs/%d/submit", jobId), 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)

	var finished map[string]any
	for time.Now().Before(deadline) {
		// Bypass the 5s refresh TTL so the test does not wait on it.
		remoteRefreshTimes.Delete(jobId)
		_, resp, err = testauth.DoJSON(r, http.MethodGet, fmt.Sprintf("/api/model-training/jobs/%d", jobId), 1, common.RoleCommonUser, nil)
		require.NoError(t, err)
		requireSuccess(t, resp)
		finished = resp["data"].(map[string]any)
		if finished["status"].(float64) == float64(JobStatusSucceeded) {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	require.Equal(t, float64(JobStatusSucceeded), finished["status"], "job error: %v", finished["error_msg"])
	assert.NotEmpty(t, finished["output_dir"])
	assert.Contains(t, finished["usage"], "rows")
	assert.NotZero(t, finished["finished_at"])

	// 4) Register the artifact, which the demo service produced on disk.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, fmt.Sprintf("/api/model-training/jobs/%d/artifact", jobId), 1, common.RoleCommonUser, map[string]any{"name": "demo-lora"})
	require.NoError(t, err)
	requireSuccess(t, resp)
	artifact := resp["data"].(map[string]any)
	assert.Equal(t, "demo-lora", artifact["name"])
}
