package modelsquare

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// Option key of the gateway base URL the training service must call for data
// cleaning. The service runs on another host, so it cannot derive this address
// from the request; the administrator sets it once.
const OptionTrainingServiceLLMBaseURL = "TrainingServiceLLMBaseURL"

// maxDatasetUploadBytes bounds a single dataset upload. The training service
// enforces its own limit too; this one protects the gateway.
const maxDatasetUploadBytes = 200 << 20

type datasetURLRequest struct {
	URL             string `json:"url" binding:"required"`
	Name            string `json:"name"`
	MarketDatasetId int    `json:"market_dataset_id"`
}

type prepareRequest struct {
	Pipeline string         `json:"pipeline" binding:"required"`
	Params   map[string]any `json:"params"`
	// Model is optional: a service running in demo mode never calls a model, and
	// it rejects a missing model itself when it needs one.
	Model string `json:"model"`
}

// ListDatasets returns the current user's datasets.
func ListDatasets(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&TrainingDataset{}).Where("user_id = ?", c.GetInt("id"))
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		query = query.Where("name LIKE ?", keyword+"%")
	}
	if source := strings.TrimSpace(c.Query("source")); source != "" {
		query = query.Where("source = ?", source)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var datasets []TrainingDataset
	if err := query.Order("id desc").Offset(pageInfo.GetStartIdx()).
		Limit(pageInfo.GetPageSize()).Find(&datasets).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(datasets)
	common.ApiSuccess(c, pageInfo)
}

// UploadDataset stores a dataset with the training service. The multipart body
// is forwarded without buffering the whole file in memory.
func UploadDataset(c *gin.Context) {
	cfg, err := loadTrainingConfig()
	if err != nil {
		respondTrainingError(c, err)
		return
	}
	header, err := c.FormFile("file")
	if err != nil {
		common.ApiErrorMsg(c, "a dataset file is required")
		return
	}
	if header.Size > maxDatasetUploadBytes {
		common.ApiErrorMsg(c, fmt.Sprintf("dataset exceeds the %d MB limit", maxDatasetUploadBytes>>20))
		return
	}
	file, err := header.Open()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer file.Close()

	dataset, err := newTrainingClient(cfg).uploadDataset(
		c.Request.Context(), strings.TrimSpace(c.PostForm("name")), header.Filename, file,
	)
	if err != nil {
		respondTrainingError(c, err)
		return
	}
	record := persistDataset(c, dataset, DatasetSourceUpload, "")
	common.ApiSuccess(c, record)
}

// CreateDatasetFromURL registers a dataset the training service downloads.
// Data market picks arrive here too: the console forwards the listing's file URL
// and its id, so this module never imports the data market package.
func CreateDatasetFromURL(c *gin.Context) {
	cfg, err := loadTrainingConfig()
	if err != nil {
		respondTrainingError(c, err)
		return
	}
	var req datasetURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateDatasetURL(req.URL); err != nil {
		common.ApiError(c, err)
		return
	}

	dataset, err := newTrainingClient(cfg).datasetFromURL(c.Request.Context(), req.URL, req.Name)
	if err != nil {
		respondTrainingError(c, err)
		return
	}
	source := DatasetSourceURL
	if req.MarketDatasetId > 0 {
		source = DatasetSourceMarket
	}
	record := persistDataset(c, dataset, source, req.URL)
	common.ApiSuccess(c, record)
}

// DeleteDataset removes a dataset from the platform and, best effort, from the
// training service.
func DeleteDataset(c *gin.Context) {
	dataset, ok := loadOwnedDataset(c)
	if !ok {
		return
	}
	if cfg, err := loadTrainingConfig(); err == nil && dataset.RemoteId != "" {
		if err := newTrainingClient(cfg).deleteDataset(c.Request.Context(), dataset.RemoteId); err != nil {
			// The platform record is authoritative; a stale remote file is not a
			// reason to keep the dataset visible in the console.
			common.SysLog(common.LogText("failed to delete the remote dataset: %s", err.Error()))
		}
	}
	revokeDatasetToken(&dataset)
	if err := model.DB.Delete(&dataset).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// StartDatasetPrepare launches a DataFlow pipeline over one dataset.
func StartDatasetPrepare(c *gin.Context) {
	dataset, ok := loadOwnedDataset(c)
	if !ok {
		return
	}
	if dataset.Prepared && dataset.PreparedRef != "" {
		common.ApiErrorMsg(c, "this dataset has already been prepared")
		return
	}
	if dataset.PrepareStatus == JobStatusQueued || dataset.PrepareStatus == JobStatusRunning {
		common.ApiErrorMsg(c, "this dataset is already being prepared")
		return
	}
	cfg, err := loadTrainingConfig()
	if err != nil {
		respondTrainingError(c, err)
		return
	}
	// The gateway address is only needed when the service really calls a model;
	// demo mode accepts an empty value, so it is passed through rather than
	// enforced here.
	llmBaseURL := trainingGatewayBaseURL()

	var req prepareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	token, plaintext, err := mintTrainingToken(c.GetInt("id"), req.Model, cfg.TokenTTL)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	params := map[string]any{}
	for key, value := range req.Params {
		params[key] = value
	}
	encodedParams, err := common.Marshal(params)
	if err != nil {
		revokeToken(token)
		common.ApiError(c, err)
		return
	}

	prepare, err := newTrainingClient(cfg).startPrepare(c.Request.Context(), dataset.RemoteId, servicePrepareRequest{
		Pipeline: req.Pipeline,
		Params:   params,
		LLM: serviceDatasetLLM{
			BaseURL: llmBaseURL,
			Model:   req.Model,
			ApiKey:  plaintext,
		},
	})
	if err != nil {
		revokeToken(token)
		respondTrainingError(c, err)
		return
	}

	updates := map[string]any{
		"pipeline":         req.Pipeline,
		"pipeline_params":  string(encodedParams),
		"prepare_run_id":   prepare.Id,
		"prepare_status":   JobStatusQueued,
		"prepare_error":    "",
		"service_token_id": token.Id,
	}
	if err := model.DB.Model(&dataset).Updates(updates).Error; err != nil {
		revokeToken(token)
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, map[string]any{"prepare_run_id": prepare.Id})
}

// GetDatasetPrepare reports a DataFlow run and settles it into the dataset row
// once the service reaches a terminal status.
func GetDatasetPrepare(c *gin.Context) {
	dataset, ok := loadOwnedDataset(c)
	if !ok {
		return
	}
	if dataset.PrepareRunId == "" {
		common.ApiErrorMsg(c, "this dataset has no preparation run")
		return
	}
	cfg, err := loadTrainingConfig()
	if err != nil {
		respondTrainingError(c, err)
		return
	}
	prepare, err := newTrainingClient(cfg).getPrepare(c.Request.Context(), dataset.RemoteId, dataset.PrepareRunId)
	if err != nil {
		respondTrainingError(c, err)
		return
	}
	applyPrepareResult(&dataset, prepare)
	common.ApiSuccess(c, dataset)
}

// CancelDatasetPrepare stops a running DataFlow pipeline.
func CancelDatasetPrepare(c *gin.Context) {
	dataset, ok := loadOwnedDataset(c)
	if !ok {
		return
	}
	if dataset.PrepareRunId == "" || isTerminalJobStatus(dataset.PrepareStatus) {
		common.ApiErrorMsg(c, "this preparation run is not running")
		return
	}
	cfg, err := loadTrainingConfig()
	if err != nil {
		respondTrainingError(c, err)
		return
	}
	if err := newTrainingClient(cfg).cancelPrepare(c.Request.Context(), dataset.RemoteId, dataset.PrepareRunId); err != nil {
		respondTrainingError(c, err)
		return
	}
	applyPrepareResult(&dataset, servicePrepare{Status: "canceled", Error: "canceled by the operator"})
	common.ApiSuccess(c, nil)
}

// applyPrepareResult mirrors the service status onto the dataset row and revokes
// the data cleaning token once the run can no longer call the gateway.
func applyPrepareResult(dataset *TrainingDataset, prepare servicePrepare) {
	status := trainingServiceStatus(prepare.Status)
	dataset.PrepareStatus = status
	dataset.PrepareError = prepare.Error
	updates := map[string]any{
		"prepare_status": status,
		"prepare_error":  prepare.Error,
	}
	if prepare.Output != nil && status == JobStatusSucceeded {
		dataset.Prepared = true
		dataset.PreparedRef = prepare.Output.DatasetId
		dataset.Rows = prepare.Output.Rows
		dataset.SizeBytes = prepare.Output.SizeBytes
		updates["prepared"] = true
		updates["prepared_ref"] = prepare.Output.DatasetId
		updates["rows"] = prepare.Output.Rows
		updates["size_bytes"] = prepare.Output.SizeBytes
	}
	if isTerminalJobStatus(status) {
		revokeDatasetToken(dataset)
	}
	if err := model.DB.Model(dataset).Updates(updates).Error; err != nil {
		common.SysLog(common.LogText("failed to store the prepare result: %s", err.Error()))
	}
}

// storeDatasetRecord builds and stores the platform row for a dataset the
// training service has accepted. Manual uploads and the bundled demo installer
// both go through it, so ownership and provenance are recorded in one place.
func storeDatasetRecord(userId int, dataset serviceDataset, source, sourceRef string) (TrainingDataset, error) {
	rows := int64(0)
	if dataset.Rows != nil {
		rows = *dataset.Rows
	}
	record := TrainingDataset{
		UserId:    userId,
		Name:      firstNonEmpty(dataset.Name, dataset.FileName, "dataset"),
		Source:    source,
		SourceRef: sourceRef,
		Format:    dataset.Format,
		SizeBytes: dataset.SizeBytes,
		Rows:      rows,
		Status:    DatasetStatusReady,
		RemoteId:  dataset.Id,
	}
	if err := model.DB.Create(&record).Error; err != nil {
		return TrainingDataset{}, err
	}
	return record, nil
}

// persistDataset records a dataset the service accepted for the caller.
func persistDataset(c *gin.Context, dataset serviceDataset, source, sourceRef string) TrainingDataset {
	record, err := storeDatasetRecord(c.GetInt("id"), dataset, source, sourceRef)
	if err != nil {
		common.ApiError(c, err)
		return TrainingDataset{}
	}
	return record
}

// loadOwnedDataset loads a dataset by id for its owner or an administrator.
func loadOwnedDataset(c *gin.Context) (TrainingDataset, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid dataset id")
		return TrainingDataset{}, false
	}
	var dataset TrainingDataset
	if err := model.DB.First(&dataset, id).Error; err != nil {
		common.ApiError(c, err)
		return TrainingDataset{}, false
	}
	if dataset.UserId != c.GetInt("id") && c.GetInt("role") < common.RoleAdminUser {
		common.ApiErrorMsg(c, "no permission to access this dataset")
		return TrainingDataset{}, false
	}
	return dataset, true
}

// mintTrainingToken issues a short-lived token owned by the requesting user so
// the training service can call the gateway for data cleaning. Consumption is
// billed by the ordinary relay path; the token is limited to the chosen model
// and revoked as soon as the run finishes.
func mintTrainingToken(userId int, modelName string, ttl time.Duration) (*model.Token, string, error) {
	key, err := common.GenerateKey()
	if err != nil {
		return nil, "", err
	}
	if ttl <= 0 {
		ttl = time.Duration(defaultTrainingServiceTokenTTLHours) * time.Hour
	}
	token := &model.Token{
		UserId:         userId,
		Key:            key,
		Status:         common.TokenStatusEnabled,
		Name:           fmt.Sprintf("model-training-dataflow-%d", time.Now().Unix()),
		CreatedTime:    common.GetTimestamp(),
		ExpiredTime:    time.Now().Add(ttl).Unix(),
		UnlimitedQuota: true,
	}
	if strings.TrimSpace(modelName) != "" {
		token.ModelLimitsEnabled = true
		token.ModelLimits = modelName
	}
	if err := token.Insert(); err != nil {
		return nil, "", err
	}
	return token, key, nil
}

func revokeToken(token *model.Token) {
	if token == nil || token.Id == 0 {
		return
	}
	if err := model.DeleteTokenById(token.Id, token.UserId); err != nil {
		common.SysLog(common.LogText("failed to revoke the data cleaning token: %s", err.Error()))
	}
}

// revokeDatasetToken revokes and clears the token attached to a dataset.
func revokeDatasetToken(dataset *TrainingDataset) {
	if dataset.ServiceTokenId == 0 {
		return
	}
	revokeToken(&model.Token{Id: dataset.ServiceTokenId, UserId: dataset.UserId})
	if err := model.DB.Model(dataset).Update("service_token_id", 0).Error; err != nil {
		common.SysLog(common.LogText("failed to clear the data cleaning token: %s", err.Error()))
	}
	dataset.ServiceTokenId = 0
}

// trainingGatewayBaseURL is the gateway address the service must call. The
// service runs on another host, so this cannot be derived from the request.
func trainingGatewayBaseURL() string {
	common.OptionMapRWMutex.RLock()
	baseURL := strings.TrimRight(strings.TrimSpace(common.OptionMap[OptionTrainingServiceLLMBaseURL]), "/")
	common.OptionMapRWMutex.RUnlock()
	return baseURL
}

// validateDatasetURL rejects non-HTTP URLs and loopback addresses, so a dataset
// URL cannot be turned into a request against the gateway's own internals.
func validateDatasetURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return errors.New("dataset URL is not a valid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("dataset URL must be an absolute http(s) URL")
	}
	host := parsed.Hostname()
	if host == "" {
		return errors.New("dataset URL must include a host")
	}
	if host == "localhost" || strings.HasPrefix(host, "127.") || host == "::1" {
		return errors.New("dataset URL must not point at a loopback address")
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// respondTrainingError maps a service failure onto the console error envelope.
// The stable `code` is included next to the English message so the UI can send
// the operator to the settings page instead of only showing raw text.
func respondTrainingError(c *gin.Context, err error) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"code":    trainingErrorCode(err),
		"message": err.Error(),
	})
}
