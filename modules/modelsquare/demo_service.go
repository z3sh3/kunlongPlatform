package modelsquare

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// In-process demo training service.
//
// When TrainingServiceDemoMode is enabled the gateway serves the same v1 contract
// itself, so the whole console flow (dataset intake, DataFlow preparation, job
// dispatch, live logs, artifacts) can be demonstrated without a GPU, without the
// two upstream frameworks, and without deploying anything.
//
// The demo server speaks real HTTP and real server-sent events on purpose: every
// handler, client and SSE proxy path is then exercised exactly as in production,
// instead of branching the platform code on "is this a demo".
const (
	OptionTrainingServiceDemoMode = "TrainingServiceDemoMode"
	OptionTrainingServiceDemoDir  = "TrainingServiceDemoDir"
	defaultTrainingServiceDemoDir = "data/training-demo"
)

// demoStoredDataset is one dataset held by the demo service on local disk.
type demoStoredDataset struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	FileName  string `json:"file_name"`
	Format    string `json:"format"`
	SizeBytes int64  `json:"size_bytes"`
	Rows      *int64 `json:"rows"`
	Path      string `json:"-"`
	CreatedAt int64  `json:"created_at"`
}

type demoPrepare struct {
	Id        string                `json:"id"`
	DatasetId string                `json:"dataset_id"`
	Template  string                `json:"template"`
	Status    string                `json:"status"`
	Progress  int                   `json:"progress"`
	Error     string                `json:"error"`
	Output    *servicePrepareOutput `json:"output"`
	Logs      []string              `json:"-"`
	Canceled  bool                  `json:"-"`
}

type demoJob struct {
	Id        string           `json:"id"`
	Name      string           `json:"name"`
	BaseModel string           `json:"base_model"`
	Method    string           `json:"method"`
	Status    string           `json:"status"`
	Progress  int              `json:"progress"`
	Error     string           `json:"error"`
	OutputDir string           `json:"output_dir"`
	Artifact  *serviceArtifact `json:"artifact"`
	Usage     map[string]any   `json:"usage"`
	DatasetId string           `json:"dataset_id"`
	Rows      int64            `json:"dataset_rows"`
	Logs      []string         `json:"-"`
	Canceled  bool             `json:"-"`
}

// demoService is the mutable state of one in-process demo instance.
type demoService struct {
	mu       sync.Mutex
	baseDir  string
	secret   string
	datasets map[string]*demoStoredDataset
	prepares map[string]*demoPrepare
	jobs     map[string]*demoJob
	sequence int
}

func newDemoService(baseDir, secret string) *demoService {
	return &demoService{
		baseDir:  baseDir,
		secret:   secret,
		datasets: map[string]*demoStoredDataset{},
		prepares: map[string]*demoPrepare{},
		jobs:     map[string]*demoJob{},
	}
}

func (s *demoService) nextId(prefix string) string {
	s.sequence++
	return fmt.Sprintf("%s-demo-%d", prefix, s.sequence)
}

// ---- request plumbing -------------------------------------------------------

func (s *demoService) writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := common.Marshal(payload)
	if err != nil {
		http.Error(w, `{"message":"encode failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (s *demoService) writeError(w http.ResponseWriter, status int, code, message string) {
	s.writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func (s *demoService) authorized(r *http.Request) bool {
	return r.Header.Get("Authorization") == "Bearer "+s.secret
}

// ServeHTTP implements the subset of the v1 contract the console uses.
func (s *demoService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		s.writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid bearer token")
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")

	switch {
	case path == "/v1/health":
		s.writeJSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"demo_mode": true,
			"versions":  map[string]string{},
			"gpus":      []any{},
			"limits":    map[string]any{"dataset_max_mb": 200},
			"active":    map[string]int{"jobs": 0, "prepares": 0},
		})
	case path == "/v1/capabilities":
		s.writeJSON(w, http.StatusOK, demoCapabilities())
	case path == "/v1/datasets" && r.Method == http.MethodPost:
		s.handleUpload(w, r)
	case path == "/v1/datasets/from-url" && r.Method == http.MethodPost:
		s.handleFromURL(w, r)
	case strings.HasPrefix(path, "/v1/datasets/"):
		s.routeDataset(w, r, strings.TrimPrefix(path, "/v1/datasets/"))
	case path == "/v1/jobs" && r.Method == http.MethodPost:
		s.handleCreateJob(w, r)
	case strings.HasPrefix(path, "/v1/jobs/"):
		s.routeJob(w, r, strings.TrimPrefix(path, "/v1/jobs/"))
	default:
		s.writeError(w, http.StatusNotFound, "not_found", "unknown route "+path)
	}
}

// demoCapabilities advertises the same curated templates as the reference
// service, so the console renders a realistic form in demo mode.
func demoCapabilities() map[string]any {
	return map[string]any{
		"demo_mode": true,
		"methods":   []string{"lora", "sft", "full"},
		"stages":    []string{"sft"},
		"pipelines": []map[string]any{
			{
				"id":            "kb_pdf_to_qa",
				"name":          "Documents to Q&A",
				"description":   "Split documents into chunks and generate question-answer pairs.",
				"input_formats": []string{".pdf", ".txt", ".jsonl"},
				"params": []map[string]any{
					{"key": "chunk_size", "label": "Chunk size", "type": "int", "default": 512, "minimum": 128, "maximum": 4096, "unit": "tokens"},
					{"key": "questions_per_chunk", "label": "Questions per chunk", "type": "int", "default": 3, "minimum": 1, "maximum": 10},
					{"key": "max_samples", "label": "Maximum output samples", "type": "int", "default": 1000, "minimum": 1, "maximum": 200000, "unit": "rows"},
				},
			},
			{
				"id":            "text_clean_sft",
				"name":          "Noisy text to clean SFT",
				"description":   "Score, filter and rewrite noisy question-answer pairs.",
				"input_formats": []string{".jsonl", ".json", ".csv"},
				"params": []map[string]any{
					{"key": "min_score", "label": "Minimum quality score", "type": "float", "default": 4, "minimum": 1, "maximum": 5},
					{"key": "rewrite", "label": "Rewrite filtered rows", "type": "bool", "default": true},
					{"key": "max_samples", "label": "Maximum output samples", "type": "int", "default": 1000, "minimum": 1, "maximum": 200000, "unit": "rows"},
				},
			},
			{
				"id":            "code_sft",
				"name":          "Code corpus to SFT",
				"description":   "Deduplicate and lint a code corpus into instruction/code pairs.",
				"input_formats": []string{".jsonl", ".json"},
				"params": []map[string]any{
					{"key": "min_lines", "label": "Minimum solution length", "type": "int", "default": 3, "minimum": 1, "maximum": 500, "unit": "lines"},
					{"key": "max_samples", "label": "Maximum output samples", "type": "int", "default": 1000, "minimum": 1, "maximum": 200000, "unit": "rows"},
				},
			},
			{
				"id":            "reasoning_sft",
				"name":          "Reasoning augmentation",
				"description":   "Expand math samples with verified step-by-step solutions.",
				"input_formats": []string{".jsonl", ".json"},
				"params": []map[string]any{
					{"key": "verify_answers", "label": "Verify answers", "type": "bool", "default": true},
					{"key": "max_samples", "label": "Maximum output samples", "type": "int", "default": 1000, "minimum": 1, "maximum": 200000, "unit": "rows"},
				},
			},
		},
		"hyper_params": []map[string]any{
			{"key": "lora_rank", "label": "LoRA rank", "type": "int", "default": 8, "minimum": 1, "maximum": 256},
			{"key": "cutoff_len", "label": "Cutoff length", "type": "int", "default": 2048, "minimum": 128, "maximum": 32768, "unit": "tokens"},
			{"key": "num_train_epochs", "label": "Epochs", "type": "float", "default": 3, "minimum": 0.1, "maximum": 20},
			{"key": "learning_rate", "label": "Learning rate", "type": "float", "default": 0.0001, "minimum": 0.0000001, "maximum": 1},
			{"key": "per_device_train_batch_size", "label": "Batch size per device", "type": "int", "default": 1, "minimum": 1, "maximum": 128},
			{"key": "gradient_accumulation_steps", "label": "Gradient accumulation steps", "type": "int", "default": 8, "minimum": 1, "maximum": 256},
			{"key": "lr_scheduler_type", "label": "Learning rate scheduler", "type": "enum", "default": "cosine", "options": []string{"cosine", "linear", "constant", "constant_with_warmup"}},
			{"key": "bf16", "label": "BF16", "type": "bool", "default": true},
		},
	}
}

// ---- datasets ---------------------------------------------------------------

func (s *demoService) handleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_dataset", "cannot parse the upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_dataset", "a dataset file is required")
		return
	}
	defer file.Close()

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = header.Filename
	}
	dataset, err := s.storeDataset(name, header.Filename, func(path string) error {
		out, createErr := os.Create(path)
		if createErr != nil {
			return createErr
		}
		defer out.Close()
		_, copyErr := io.Copy(out, file)
		return copyErr
	})
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_dataset", err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, dataset)
}

func (s *demoService) handleFromURL(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := common.DecodeJson(r.Body, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_dataset", "cannot parse the request")
		return
	}
	filename := filepath.Base(strings.TrimSpace(payload.URL))
	if filename == "." || filename == "/" || filename == "" {
		filename = "dataset.jsonl"
	}
	name := payload.Name
	if name == "" {
		name = filename
	}
	// A demo never downloads the remote file: it records the reference and
	// writes a small synthetic source so preparation still has input.
	dataset, err := s.storeDataset(name, filename, func(path string) error {
		return os.WriteFile(path, []byte(`{"text":"demo dataset downloaded from a URL"}`+"\n"), 0o644)
	})
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_dataset", err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, dataset)
}

// storeDataset writes one dataset file and registers it.
func (s *demoService) storeDataset(name, filename string, write func(string) error) (demoStoredDataset, error) {
	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return demoStoredDataset{}, fmt.Errorf("cannot create the demo directory: %w", err)
	}
	id := s.nextId("ds")
	dir := filepath.Join(s.baseDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return demoStoredDataset{}, fmt.Errorf("cannot create the dataset directory: %w", err)
	}
	safeName := filepath.Base(filename)
	if safeName == "" || safeName == "." {
		safeName = "dataset.jsonl"
	}
	path := filepath.Join(dir, safeName)
	if err := write(path); err != nil {
		return demoStoredDataset{}, fmt.Errorf("cannot store the dataset: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return demoStoredDataset{}, fmt.Errorf("cannot read the dataset: %w", err)
	}
	rows := countDemoRows(path)
	dataset := newDemoStoredDataset(id, name, safeName, path, info.Size(), rows)

	s.mu.Lock()
	s.datasets[id] = &dataset
	s.mu.Unlock()
	return dataset, nil
}

func newDemoStoredDataset(id, name, fileName, path string, size int64, rows int64) demoStoredDataset {
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	return demoStoredDataset{
		Id:        id,
		Name:      name,
		FileName:  fileName,
		Format:    format,
		SizeBytes: size,
		Rows:      &rows,
		Path:      path,
		CreatedAt: common.GetTimestamp(),
	}
}

// countDemoRows counts JSONL lines, which is all the console displays.
func countDemoRows(path string) int64 {
	if strings.ToLower(filepath.Ext(path)) != ".jsonl" {
		return 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var count int64
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func (s *demoService) routeDataset(w http.ResponseWriter, r *http.Request, rest string) {
	parts := strings.Split(rest, "/")
	id := parts[0]

	if len(parts) == 1 && r.Method == http.MethodDelete {
		s.mu.Lock()
		dataset, ok := s.datasets[id]
		delete(s.datasets, id)
		s.mu.Unlock()
		if ok {
			_ = os.RemoveAll(filepath.Dir(dataset.Path))
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
		return
	}

	if len(parts) >= 2 && parts[1] == "prepare" {
		s.mu.Lock()
		dataset, ok := s.datasets[id]
		s.mu.Unlock()
		if !ok {
			s.writeError(w, http.StatusNotFound, "not_found", "dataset not found")
			return
		}
		// POST /prepare creates a run; GET /prepare/{run} and /prepare/{run}/logs read it.
		if len(parts) == 2 && r.Method == http.MethodPost {
			s.startPrepare(w, r, dataset)
			return
		}
		if len(parts) >= 3 {
			s.handlePrepareRun(w, r, parts[2], len(parts) >= 4 && parts[3] == "logs")
			return
		}
	}
	s.writeError(w, http.StatusNotFound, "not_found", "unknown dataset route")
}

func (s *demoService) startPrepare(w http.ResponseWriter, r *http.Request, dataset *demoStoredDataset) {
	var payload servicePrepareRequest
	if err := common.DecodeJson(r.Body, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_params", "cannot parse the request")
		return
	}
	run := &demoPrepare{
		Id:        s.nextId("run"),
		DatasetId: dataset.Id,
		Template:  payload.Pipeline,
		Status:    "running",
		Progress:  0,
	}
	s.mu.Lock()
	s.prepares[run.Id] = run
	s.mu.Unlock()

	go s.simulatePrepare(run, dataset, payload.Params)
	s.writeJSON(w, http.StatusOK, run)
}

// simulatePrepare produces a synthetic SFT dataset and streams progress.
func (s *demoService) simulatePrepare(run *demoPrepare, dataset *demoStoredDataset, params map[string]any) {
	appendLog := func(line string) {
		s.mu.Lock()
		run.Logs = append(run.Logs, line)
		s.mu.Unlock()
	}
	setProgress := func(value int) {
		s.mu.Lock()
		run.Progress = value
		s.mu.Unlock()
	}

	appendLog("[demo] simulated DataFlow run: no GPU and no model calls")
	appendLog("[demo] template=" + run.Template + " input=" + dataset.FileName)
	time.Sleep(300 * time.Millisecond)
	setProgress(30)

	rows := demoPreparedRows(dataset, params)
	for index := 0; index < 4; index++ {
		s.mu.Lock()
		canceled := run.Canceled
		s.mu.Unlock()
		if canceled {
			return
		}
		setProgress(30 + (index+1)*15)
		appendLog(fmt.Sprintf("[demo] generated %d/%d rows", (index+1)*len(rows)/4, len(rows)))
		time.Sleep(250 * time.Millisecond)
	}
	if len(rows) == 0 {
		s.mu.Lock()
		run.Status = "failed"
		run.Error = "the pipeline produced no rows"
		s.mu.Unlock()
		return
	}

	outId := s.nextId("ds")
	dir := filepath.Join(s.baseDir, outId)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.mu.Lock()
		run.Status = "failed"
		run.Error = err.Error()
		s.mu.Unlock()
		return
	}
	path := filepath.Join(dir, "output.jsonl")
	var builder strings.Builder
	for _, row := range rows {
		encoded, err := common.Marshal(row)
		if err != nil {
			continue
		}
		builder.Write(encoded)
		builder.WriteString("\n")
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		s.mu.Lock()
		run.Status = "failed"
		run.Error = err.Error()
		s.mu.Unlock()
		return
	}
	info, _ := os.Stat(path)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	outputRows := int64(len(rows))
	outDataset := newDemoStoredDataset(outId, dataset.Name+" (prepared)", "output.jsonl", path, size, outputRows)
	s.mu.Lock()
	s.datasets[outId] = &outDataset
	run.Status = "succeeded"
	run.Progress = 100
	run.Output = &servicePrepareOutput{
		DatasetId:   outId,
		DatasetName: outDataset.Name,
		Rows:        outputRows,
		SizeBytes:   size,
	}
	run.Logs = append(run.Logs, fmt.Sprintf("[demo] done: %d rows -> dataset %s", outputRows, outId))
	s.mu.Unlock()
}

// demoPreparedRows derives instruction/output pairs from the source file.
func demoPreparedRows(dataset *demoStoredDataset, params map[string]any) []map[string]string {
	limit := 24
	if raw, ok := params["max_samples"]; ok {
		if value, err := strconv.Atoi(fmt.Sprintf("%v", raw)); err == nil && value > 0 && value < limit {
			limit = value
		}
	}

	var seeds []string
	if data, err := os.ReadFile(dataset.Path); err == nil {
		if strings.ToLower(dataset.Format) == "jsonl" {
			for _, line := range strings.Split(string(data), "\n") {
				var row map[string]any
				if err := common.UnmarshalJsonStr(strings.TrimSpace(line), &row); err != nil {
					continue
				}
				for _, key := range []string{"answer", "output", "text", "code", "response"} {
					if value, ok := row[key].(string); ok && strings.TrimSpace(value) != "" {
						seeds = append(seeds, value)
						break
					}
				}
			}
		} else {
			seeds = append(seeds, string(data))
		}
	}
	if len(seeds) == 0 {
		seeds = []string{"demo source content"}
	}

	rows := make([]map[string]string, 0, limit)
	for index := 0; index < limit; index++ {
		seed := seeds[index%len(seeds)]
		if len(seed) > 400 {
			seed = seed[:400]
		}
		rows = append(rows, map[string]string{
			"instruction": fmt.Sprintf("演示问题 %d：请根据给定内容作答。", index+1),
			"output":      seed,
		})
	}
	return rows
}

func (s *demoService) handlePrepareRun(w http.ResponseWriter, r *http.Request, runId string, logs bool) {
	s.mu.Lock()
	run, ok := s.prepares[runId]
	s.mu.Unlock()
	if !ok {
		s.writeError(w, http.StatusNotFound, "not_found", "prepare run not found")
		return
	}
	if logs {
		s.streamLogs(w, r, func() ([]string, bool) {
			s.mu.Lock()
			defer s.mu.Unlock()
			return append([]string(nil), run.Logs...), isTerminalDemoStatus(run.Status)
		})
		return
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cancel") {
		s.mu.Lock()
		run.Canceled = true
		run.Status = "canceled"
		run.Error = "canceled by the operator"
		s.mu.Unlock()
		s.writeJSON(w, http.StatusOK, map[string]any{"canceled": true})
		return
	}
	s.writeJSON(w, http.StatusOK, run)
}

// ---- jobs -------------------------------------------------------------------

func (s *demoService) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var payload serviceJobRequest
	if err := common.DecodeJson(r.Body, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, "job_failed", "cannot parse the request")
		return
	}
	s.mu.Lock()
	dataset, ok := s.datasets[payload.DatasetId]
	s.mu.Unlock()
	if !ok {
		s.writeError(w, http.StatusBadRequest, "job_failed", "dataset not found")
		return
	}

	job := &demoJob{
		Id:        s.nextId("job"),
		Name:      payload.Name,
		BaseModel: payload.BaseModel,
		Method:    payload.Method,
		Status:    "running",
		DatasetId: payload.DatasetId,
	}
	if dataset.Rows != nil {
		job.Rows = *dataset.Rows
	}
	s.mu.Lock()
	s.jobs[job.Id] = job
	s.mu.Unlock()

	go s.simulateTraining(job, payload.HyperParams)
	s.writeJSON(w, http.StatusOK, job)
}

func isTerminalDemoStatus(status string) bool {
	return status == "succeeded" || status == "failed" || status == "canceled"
}

func (s *demoService) simulateTraining(job *demoJob, hyper map[string]any) {
	appendLog := func(line string) {
		s.mu.Lock()
		job.Logs = append(job.Logs, line)
		s.mu.Unlock()
	}
	epochs := 1.0
	if raw, ok := hyper["num_train_epochs"]; ok {
		if parsed, err := strconv.ParseFloat(fmt.Sprintf("%v", raw), 64); err == nil && parsed > 0 {
			epochs = parsed
		}
	}

	appendLog("[demo] simulated LLaMA Factory training: no GPU")
	appendLog("***** Running training *****")
	appendLog(fmt.Sprintf("  Num examples = %d", job.Rows))
	totalSteps := 12
	for step := 1; step <= totalSteps; step++ {
		s.mu.Lock()
		canceled := job.Canceled
		s.mu.Unlock()
		if canceled {
			return
		}
		loss := 2.4 - float64(step)*0.17
		if loss < 0.12 {
			loss = 0.12
		}
		epoch := float64(step) / float64(totalSteps) * epochs
		appendLog(fmt.Sprintf("{'loss': %.4f, 'epoch': %.2f, 'step': %d}", loss, epoch, step))
		appendLog(fmt.Sprintf("%d%%|%s| %d/%d", step*100/totalSteps,
			strings.Repeat("#", step*20/totalSteps), step, totalSteps))
		s.mu.Lock()
		job.Progress = step * 100 / totalSteps
		s.mu.Unlock()
		time.Sleep(200 * time.Millisecond)
	}

	outputDir := filepath.Join(s.baseDir, job.Id, "output")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		s.mu.Lock()
		job.Status = "failed"
		job.Error = err.Error()
		s.mu.Unlock()
		return
	}
	rank := 8
	if raw, ok := hyper["lora_rank"]; ok {
		if parsed, err := strconv.Atoi(fmt.Sprintf("%v", raw)); err == nil && parsed > 0 {
			rank = parsed
		}
	}
	adapterConfig := fmt.Sprintf("{\n  \"peft_type\": \"LORA\",\n  \"r\": %d,\n  \"demo\": true\n}\n", rank)
	_ = os.WriteFile(filepath.Join(outputDir, "adapter_config.json"), []byte(adapterConfig), 0o644)
	_ = os.WriteFile(filepath.Join(outputDir, "adapter_model.safetensors"), []byte("new-api training demo artifact\n"), 0o644)
	_ = os.WriteFile(filepath.Join(outputDir, "trainer_state.json"), []byte("{\n  \"demo\": true\n}\n"), 0o644)

	artifact := scanDemoArtifact(outputDir)
	appendLog("[demo] finished; placeholder artifact in " + outputDir)

	s.mu.Lock()
	job.Status = "succeeded"
	job.Progress = 100
	job.OutputDir = outputDir
	job.Artifact = &artifact
	job.Usage = map[string]any{
		"rows":        job.Rows,
		"epochs":      epochs,
		"gpu_seconds": 0,
		"demo":        true,
	}
	s.mu.Unlock()
}

// scanDemoArtifact mirrors the reference service's artifact detection.
func scanDemoArtifact(dir string) serviceArtifact {
	artifact := serviceArtifact{OutputDir: dir, Kind: "unknown"}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return artifact
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, statErr := entry.Info()
		if statErr == nil {
			artifact.SizeBytes += info.Size()
		}
		artifact.FileCount++
		if entry.Name() == "adapter_config.json" {
			artifact.Kind = "lora"
		}
	}
	return artifact
}

func (s *demoService) routeJob(w http.ResponseWriter, r *http.Request, rest string) {
	parts := strings.Split(rest, "/")
	id := parts[0]
	s.mu.Lock()
	job, ok := s.jobs[id]
	s.mu.Unlock()
	if !ok {
		s.writeError(w, http.StatusNotFound, "not_found", "job not found")
		return
	}
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		s.writeJSON(w, http.StatusOK, job)
	case len(parts) == 1 && r.Method == http.MethodDelete:
		s.mu.Lock()
		delete(s.jobs, id)
		s.mu.Unlock()
		_ = os.RemoveAll(filepath.Join(s.baseDir, id))
		s.writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
	case len(parts) >= 2 && parts[1] == "cancel":
		s.mu.Lock()
		if !isTerminalDemoStatus(job.Status) {
			job.Canceled = true
			job.Status = "canceled"
			job.Error = "canceled by the operator"
		}
		s.mu.Unlock()
		s.writeJSON(w, http.StatusOK, map[string]any{"canceled": true})
	case len(parts) >= 2 && parts[1] == "logs":
		s.streamLogs(w, r, func() ([]string, bool) {
			s.mu.Lock()
			defer s.mu.Unlock()
			return append([]string(nil), job.Logs...), isTerminalDemoStatus(job.Status)
		})
	default:
		s.writeError(w, http.StatusNotFound, "not_found", "unknown job route")
	}
}

// streamLogs emits the accumulated lines as server-sent events, following new
// lines until the owning run reaches a terminal status.
func (s *demoService) streamLogs(w http.ResponseWriter, r *http.Request, snapshot func() ([]string, bool)) {
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	for {
		lines, terminal := snapshot()
		for offset < len(lines) {
			offset++
			frame, err := common.Marshal(map[string]any{"offset": offset, "line": lines[offset-1]})
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", frame)
		}
		if flusher != nil {
			flusher.Flush()
		}
		if terminal && offset >= len(lines) {
			fmt.Fprint(w, "data: [DONE]\n\n")
			if flusher != nil {
				flusher.Flush()
			}
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(150 * time.Millisecond):
		}
	}
}

// ---- lifecycle --------------------------------------------------------------

var (
	demoServerOnce   sync.Once
	demoServer       *httptest.Server
	demoServerSecret string
)

// demoTrainingServer returns the process-wide demo server, starting it on first
// use. It listens on a loopback port so the normal HTTP client path is used.
func demoTrainingServer() (*httptest.Server, string, error) {
	var startErr error
	demoServerOnce.Do(func() {
		secret, err := common.GenerateKey()
		if err != nil {
			startErr = err
			return
		}
		common.OptionMapRWMutex.RLock()
		baseDir := strings.TrimSpace(common.OptionMap[OptionTrainingServiceDemoDir])
		common.OptionMapRWMutex.RUnlock()
		if baseDir == "" {
			baseDir = defaultTrainingServiceDemoDir
		}
		service := newDemoService(baseDir, secret)
		demoServerSecret = secret
		demoServer = httptest.NewServer(service)
	})
	return demoServer, demoServerSecret, startErr
}

// closeDemoTrainingServer stops the demo server; used by tests.
func closeDemoTrainingServer() {
	if demoServer != nil {
		demoServer.Close()
		demoServer = nil
		demoServerSecret = ""
		demoServerOnce = sync.Once{}
	}
}

// demoModeEnabled reports whether the console should use the in-process service.
func demoModeEnabled() bool {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return strings.EqualFold(strings.TrimSpace(common.OptionMap[OptionTrainingServiceDemoMode]), "true")
}
