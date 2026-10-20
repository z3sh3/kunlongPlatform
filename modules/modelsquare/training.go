package modelsquare

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// Option keys of the training service integration. They live in the shared
// option map so an administrator can change them from the console without a
// restart. The secret key ends with "Secret" on purpose: controller.GetOptions
// filters such keys out, which keeps it write-only.
const (
	OptionTrainingServiceURL              = "TrainingServiceURL"
	OptionTrainingServiceSecret           = "TrainingServiceSecret"
	OptionTrainingServiceTimeoutSeconds   = "TrainingServiceTimeoutSeconds"
	OptionTrainingServiceTokenTTLHours    = "TrainingServiceTokenTTLHours"
	defaultTrainingServiceTimeoutSeconds  = 60
	defaultTrainingServiceTokenTTLHours   = 168
	trainingServiceMaxJSONResponseBytes   = 8 << 20
	trainingServiceMaxLogFrameBytes       = 256 << 10
	trainingServiceLogStreamIdleTimeoutMs = 10 * 60 * 1000
)

// Stable error codes returned to the console so the UI can tell "not connected"
// from "connected but failing", mirroring the Dify integration.
const (
	trainingCodeNotConfigured = "training_service_not_configured"
	trainingCodeUnreachable   = "training_service_unreachable"
	trainingCodeAuthFailed    = "training_service_auth_failed"
	trainingCodeError         = "training_service_error"
)

// trainingError carries a stable code plus the English console message key.
type trainingError struct {
	code string
	key  string
	err  error
}

func (e *trainingError) Error() string { return e.key }

func (e *trainingError) Unwrap() error { return e.err }

func newTrainingError(code, key string, err error) *trainingError {
	return &trainingError{code: code, key: key, err: err}
}

// trainingErrorCode extracts the stable code of a training service failure.
func trainingErrorCode(err error) string {
	var target *trainingError
	if errors.As(err, &target) {
		return target.code
	}
	return trainingCodeError
}

// trainingConfig is the connection settings shared by every service request.
type trainingConfig struct {
	BaseURL   string
	Secret    string
	Timeout   time.Duration
	TokenTTL  time.Duration
	IsEnabled bool
	// Demo runs the in-process demo service instead of a remote host.
	Demo bool
}

// loadTrainingConfig reads the integration settings from the option map. The
// integration is enabled when both the URL and the secret are present, which
// avoids a separate flag that could disagree with the configured values.
func loadTrainingConfig() (trainingConfig, error) {
	common.OptionMapRWMutex.RLock()
	baseURL := strings.TrimRight(strings.TrimSpace(common.OptionMap[OptionTrainingServiceURL]), "/")
	secret := strings.TrimSpace(common.OptionMap[OptionTrainingServiceSecret])
	timeout := trainingOptionInt(OptionTrainingServiceTimeoutSeconds, defaultTrainingServiceTimeoutSeconds)
	tokenTTL := trainingOptionInt(OptionTrainingServiceTokenTTLHours, defaultTrainingServiceTokenTTLHours)
	common.OptionMapRWMutex.RUnlock()

	cfg := trainingConfig{
		BaseURL:  baseURL,
		Secret:   secret,
		Timeout:  time.Duration(timeout) * time.Second,
		TokenTTL: time.Duration(tokenTTL) * time.Hour,
	}
	// Demo mode needs neither a remote host nor a secret: the gateway serves the
	// same contract itself, so a demonstration works on a machine without a GPU.
	if demoModeEnabled() {
		server, demoSecret, err := demoTrainingServer()
		if err != nil {
			return cfg, newTrainingError(trainingCodeError,
				"Failed to start the demo training service", err)
		}
		cfg.BaseURL = server.URL
		cfg.Secret = demoSecret
		cfg.Demo = true
		cfg.IsEnabled = true
		return cfg, nil
	}

	if cfg.BaseURL == "" || cfg.Secret == "" {
		return cfg, newTrainingError(trainingCodeNotConfigured,
			"Model training service is not configured", nil)
	}
	cfg.IsEnabled = true
	return cfg, nil
}

func trainingOptionInt(key string, fallback int) int {
	raw := strings.TrimSpace(common.OptionMap[key])
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

// Wire shapes of the training service API (v1).

type serviceCapabilities struct {
	Pipelines   []servicePipeline `json:"pipelines"`
	Methods     []string          `json:"methods"`
	Stages      []string          `json:"stages"`
	HyperParams []serviceParam    `json:"hyper_params"`
	// DemoMode reports that the service simulates preparation and training, so
	// no cleaning model is needed to run a pipeline.
	DemoMode bool `json:"demo_mode"`
}

type servicePipeline struct {
	Id            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	InputFormats  []string       `json:"input_formats"`
	Params        []serviceParam `json:"params"`
	ParamsByField []serviceParam `json:"-"`
}

type serviceParam struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Default  any      `json:"default"`
	Minimum  *float64 `json:"minimum"`
	Maximum  *float64 `json:"maximum"`
	Options  []string `json:"options"`
	Unit     string   `json:"unit"`
	Help     string   `json:"help"`
}

type serviceDataset struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	FileName  string `json:"file_name"`
	Format    string `json:"format"`
	SizeBytes int64  `json:"size_bytes"`
	Rows      *int64 `json:"rows"`
}

type servicePrepareOutput struct {
	DatasetId   string `json:"dataset_id"`
	DatasetName string `json:"dataset_name"`
	Rows        int64  `json:"rows"`
	SizeBytes   int64  `json:"size_bytes"`
}

type servicePrepare struct {
	Id           string                `json:"id"`
	DatasetId    string                `json:"dataset_id"`
	Template     string                `json:"template"`
	TemplateName string                `json:"template_name"`
	Status       string                `json:"status"`
	Progress     int                   `json:"progress"`
	Error        string                `json:"error"`
	Output       *servicePrepareOutput `json:"output"`
}

type serviceArtifact struct {
	OutputDir string `json:"output_dir"`
	Kind      string `json:"kind"`
	FileCount int    `json:"file_count"`
	SizeBytes int64  `json:"size_bytes"`
}

type serviceJob struct {
	Id         string           `json:"id"`
	Name       string           `json:"name"`
	BaseModel  string           `json:"base_model"`
	Method     string           `json:"method"`
	Status     string           `json:"status"`
	Progress   int              `json:"progress"`
	Error      string           `json:"error"`
	OutputDir  string           `json:"output_dir"`
	Artifact   *serviceArtifact `json:"artifact"`
	Usage      map[string]any   `json:"usage"`
	DatasetRef string           `json:"dataset_id"`
}

type serviceHealth struct {
	Ok       bool              `json:"ok"`
	DemoMode bool              `json:"demo_mode"`
	Versions map[string]string `json:"versions"`
	Gpus     []map[string]any  `json:"gpus"`
	Limits   map[string]any    `json:"limits"`
	Active   map[string]int    `json:"active"`
}

type serviceDatasetLLM struct {
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	ApiKey  string `json:"api_key"`
}

type servicePrepareRequest struct {
	Pipeline string            `json:"pipeline"`
	Params   map[string]any    `json:"params"`
	LLM      serviceDatasetLLM `json:"llm"`
}

type serviceJobRequest struct {
	Name        string         `json:"name"`
	BaseModel   string         `json:"base_model"`
	Method      string         `json:"method"`
	DatasetId   string         `json:"dataset_id"`
	Template    string         `json:"template"`
	HyperParams map[string]any `json:"hyper_params"`
}

// trainingServiceStatus maps a service status string onto the console status.
// An unknown status is reported as queued rather than guessed as terminal, so a
// job can never be marked finished by a typo.
func trainingServiceStatus(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "succeeded":
		return JobStatusSucceeded
	case "failed":
		return JobStatusFailed
	case "canceled":
		return JobStatusCanceled
	case "running":
		return JobStatusRunning
	default:
		return JobStatusQueued
	}
}

func isTerminalJobStatus(status int) bool {
	return status == JobStatusSucceeded || status == JobStatusFailed || status == JobStatusCanceled
}

// trainingClient performs authenticated JSON calls against the service.
type trainingClient struct {
	cfg  trainingConfig
	http *http.Client
}

func newTrainingClient(cfg trainingConfig) *trainingClient {
	return &trainingClient{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}
}

func (c *trainingClient) do(ctx context.Context, method, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := common.Marshal(payload)
		if err != nil {
			return newTrainingError(trainingCodeError, "Failed to encode the training request", err)
		}
		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, body)
	if err != nil {
		return newTrainingError(trainingCodeError, "Failed to build the training request", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.cfg.Secret)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.http.Do(request)
	if err != nil {
		return newTrainingError(trainingCodeUnreachable, "Model training service is unreachable", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return newTrainingError(trainingCodeAuthFailed, "Model training service rejected the access token", nil)
	}

	limited := io.LimitReader(response.Body, trainingServiceMaxJSONResponseBytes)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := trainingServiceErrorMessage(limited)
		if message == "" {
			message = fmt.Sprintf("Model training service returned HTTP %d", response.StatusCode)
		}
		return newTrainingError(trainingCodeError, message, nil)
	}
	if out == nil {
		return nil
	}
	data, err := io.ReadAll(limited)
	if err != nil {
		return newTrainingError(trainingCodeUnreachable, "Failed to read the training service response", err)
	}
	if err := common.Unmarshal(data, out); err != nil {
		return newTrainingError(trainingCodeError, "Model training service returned an unreadable response", err)
	}
	return nil
}

// trainingServiceErrorMessage extracts the service's {"code","message"} error
// message so the operator sees the real reason instead of a bare status code.
func trainingServiceErrorMessage(body io.Reader) string {
	data, err := io.ReadAll(body)
	if err != nil || len(data) == 0 {
		return ""
	}
	var envelope struct {
		Message string `json:"message"`
	}
	if err := common.Unmarshal(data, &envelope); err != nil {
		return ""
	}
	return strings.TrimSpace(envelope.Message)
}

func (c *trainingClient) health(ctx context.Context) (serviceHealth, error) {
	var health serviceHealth
	err := c.do(ctx, http.MethodGet, "/v1/health", nil, &health)
	return health, err
}

func (c *trainingClient) capabilities(ctx context.Context) (serviceCapabilities, error) {
	var caps serviceCapabilities
	err := c.do(ctx, http.MethodGet, "/v1/capabilities", nil, &caps)
	return caps, err
}

// uploadDataset streams one dataset file to the service.
func (c *trainingClient) uploadDataset(ctx context.Context, name, filename string, file io.Reader) (serviceDataset, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	if name != "" {
		if err := writer.WriteField("name", name); err != nil {
			return serviceDataset{}, newTrainingError(trainingCodeError, "Failed to build the dataset upload", err)
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return serviceDataset{}, newTrainingError(trainingCodeError, "Failed to build the dataset upload", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return serviceDataset{}, newTrainingError(trainingCodeError, "Failed to read the dataset file", err)
	}
	if err := writer.Close(); err != nil {
		return serviceDataset{}, newTrainingError(trainingCodeError, "Failed to build the dataset upload", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/v1/datasets", &buffer)
	if err != nil {
		return serviceDataset{}, newTrainingError(trainingCodeError, "Failed to build the dataset upload", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.cfg.Secret)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	response, err := c.http.Do(request)
	if err != nil {
		return serviceDataset{}, newTrainingError(trainingCodeUnreachable, "Model training service is unreachable", err)
	}
	defer response.Body.Close()

	limited := io.LimitReader(response.Body, trainingServiceMaxJSONResponseBytes)
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return serviceDataset{}, newTrainingError(trainingCodeAuthFailed, "Model training service rejected the access token", nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := trainingServiceErrorMessage(limited)
		if message == "" {
			message = fmt.Sprintf("Model training service returned HTTP %d", response.StatusCode)
		}
		return serviceDataset{}, newTrainingError(trainingCodeError, message, nil)
	}
	data, err := io.ReadAll(limited)
	if err != nil {
		return serviceDataset{}, newTrainingError(trainingCodeUnreachable, "Failed to read the training service response", err)
	}
	var dataset serviceDataset
	if err := common.Unmarshal(data, &dataset); err != nil {
		return serviceDataset{}, newTrainingError(trainingCodeError, "Model training service returned an unreadable response", err)
	}
	return dataset, nil
}

func (c *trainingClient) datasetFromURL(ctx context.Context, rawURL, name string) (serviceDataset, error) {
	payload := map[string]string{"url": rawURL, "name": name}
	var dataset serviceDataset
	err := c.do(ctx, http.MethodPost, "/v1/datasets/from-url", payload, &dataset)
	return dataset, err
}

func (c *trainingClient) deleteDataset(ctx context.Context, remoteId string) error {
	return c.do(ctx, http.MethodDelete, "/v1/datasets/"+url.PathEscape(remoteId), nil, nil)
}

func (c *trainingClient) startPrepare(ctx context.Context, remoteId string, request servicePrepareRequest) (servicePrepare, error) {
	var prepare servicePrepare
	err := c.do(ctx, http.MethodPost, "/v1/datasets/"+url.PathEscape(remoteId)+"/prepare", request, &prepare)
	return prepare, err
}

func (c *trainingClient) getPrepare(ctx context.Context, remoteId, runId string) (servicePrepare, error) {
	var prepare servicePrepare
	path := "/v1/datasets/" + url.PathEscape(remoteId) + "/prepare/" + url.PathEscape(runId)
	err := c.do(ctx, http.MethodGet, path, nil, &prepare)
	return prepare, err
}

func (c *trainingClient) cancelPrepare(ctx context.Context, remoteId, runId string) error {
	path := "/v1/datasets/" + url.PathEscape(remoteId) + "/prepare/" + url.PathEscape(runId) + "/cancel"
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

func (c *trainingClient) createJob(ctx context.Context, request serviceJobRequest) (serviceJob, error) {
	var job serviceJob
	err := c.do(ctx, http.MethodPost, "/v1/jobs", request, &job)
	return job, err
}

func (c *trainingClient) getJob(ctx context.Context, jobId string) (serviceJob, error) {
	var job serviceJob
	err := c.do(ctx, http.MethodGet, "/v1/jobs/"+url.PathEscape(jobId), nil, &job)
	return job, err
}

func (c *trainingClient) cancelJob(ctx context.Context, jobId string) error {
	return c.do(ctx, http.MethodPost, "/v1/jobs/"+url.PathEscape(jobId)+"/cancel", nil, nil)
}

func (c *trainingClient) deleteJob(ctx context.Context, jobId string) error {
	return c.do(ctx, http.MethodDelete, "/v1/jobs/"+url.PathEscape(jobId), nil, nil)
}

// openLogStream starts a server-sent events request against the service. The
// caller owns the response body and must close it.
func (c *trainingClient) openLogStream(ctx context.Context, path string, offset int64) (*http.Response, error) {
	full := fmt.Sprintf("%s%s?offset=%d", c.cfg.BaseURL, path, offset)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, newTrainingError(trainingCodeError, "Failed to build the log request", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.cfg.Secret)
	request.Header.Set("Accept", "text/event-stream")

	// A dedicated client without the JSON timeout: a training log stream is
	// long-lived by design.
	streamClient := &http.Client{Timeout: time.Duration(trainingServiceLogStreamIdleTimeoutMs) * time.Millisecond}
	response, err := streamClient.Do(request)
	if err != nil {
		return nil, newTrainingError(trainingCodeUnreachable, "Model training service is unreachable", err)
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		response.Body.Close()
		return nil, newTrainingError(trainingCodeAuthFailed, "Model training service rejected the access token", nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := trainingServiceErrorMessage(io.LimitReader(response.Body, trainingServiceMaxLogFrameBytes))
		response.Body.Close()
		if message == "" {
			message = fmt.Sprintf("Model training service returned HTTP %d", response.StatusCode)
		}
		return nil, newTrainingError(trainingCodeError, message, nil)
	}
	return response, nil
}
