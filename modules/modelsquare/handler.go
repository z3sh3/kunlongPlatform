package modelsquare

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// Supported fine-tuning methods.
const (
	MethodLora = "lora"
	MethodSft  = "sft"
	MethodFull = "full"
)

type jobRequest struct {
	Name        string `json:"name" binding:"required"`
	BaseModel   string `json:"base_model" binding:"required"`
	Method      string `json:"method" binding:"required"`
	DatasetId   int    `json:"dataset_id"`
	DatasetRefs string `json:"dataset_refs"`
	HyperParams string `json:"hyper_params"`
}

type adminJobStatusRequest struct {
	Status      int    `json:"status" binding:"required"`
	ResultModel string `json:"result_model"`
	ErrorMsg    string `json:"error_msg"`
}

// ListJobs returns the current user's fine-tuning jobs.
func ListJobs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&FinetuneJob{}).Where("user_id = ?", c.GetInt("id"))
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("name LIKE ?", keyword+"%")
	}
	if status, err := strconv.Atoi(c.Query("status")); err == nil && status >= 0 {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var jobs []FinetuneJob
	if err := query.Order("id desc").Offset(pageInfo.GetStartIdx()).
		Limit(pageInfo.GetPageSize()).Find(&jobs).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	refreshJobs(jobs)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(jobs)
	common.ApiSuccess(c, pageInfo)
}

// GetJob returns one job. Jobs are visible to their owner and to
// administrators.
func GetJob(c *gin.Context) {
	job, ok := loadOwnedJob(c)
	if !ok {
		return
	}
	refreshJob(&job)
	common.ApiSuccess(c, job)
}

// CreateJob creates a draft fine-tuning job.
func CreateJob(c *gin.Context) {
	var req jobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateJobRequest(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	job := FinetuneJob{
		UserId:      c.GetInt("id"),
		Name:        req.Name,
		BaseModel:   req.BaseModel,
		Method:      req.Method,
		DatasetId:   req.DatasetId,
		DatasetRefs: req.DatasetRefs,
		HyperParams: req.HyperParams,
		Status:      JobStatusDraft,
	}
	if err := model.DB.Create(&job).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, job)
}

// UpdateJob edits a draft job owned by the current user.
func UpdateJob(c *gin.Context) {
	job, ok := loadOwnedJob(c)
	if !ok {
		return
	}
	if job.Status != JobStatusDraft {
		common.ApiErrorMsg(c, "only draft jobs can be edited")
		return
	}
	var req jobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateJobRequest(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	updates := map[string]any{
		"name":         req.Name,
		"base_model":   req.BaseModel,
		"method":       req.Method,
		"dataset_refs": req.DatasetRefs,
		"hyper_params": req.HyperParams,
	}
	// An omitted dataset id means "keep the current dataset": a job cannot be
	// submitted without one, so clearing it would only lose the association.
	if req.DatasetId > 0 {
		updates["dataset_id"] = req.DatasetId
	}
	if err := model.DB.Model(&job).Updates(updates).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, job)
}

// DeleteJob deletes a draft or terminal job owned by the current user and asks
// the training service to drop its working directory.
func DeleteJob(c *gin.Context) {
	job, ok := loadOwnedJob(c)
	if !ok {
		return
	}
	if job.Status != JobStatusDraft && job.Status != JobStatusCanceled && job.Status != JobStatusFailed {
		common.ApiErrorMsg(c, "only draft, canceled or failed jobs can be deleted")
		return
	}
	if job.ServiceJobId != "" {
		if cfg, err := loadTrainingConfig(); err == nil {
			if err := newTrainingClient(cfg).deleteJob(c.Request.Context(), job.ServiceJobId); err != nil {
				// The console row is authoritative; a service that already forgot
				// the job must not block the user from cleaning up the list.
				common.SysLog(common.LogText("failed to delete the remote training job: %s", err.Error()))
			}
		}
	}
	if err := model.DB.Delete(&job).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// SubmitJob dispatches a draft job to the training service. The service owns the
// execution state; the platform keeps the ownership row and mirrors progress.
func SubmitJob(c *gin.Context) {
	job, ok := loadOwnedJob(c)
	if !ok {
		return
	}
	if job.Status != JobStatusDraft {
		common.ApiErrorMsg(c, "only draft jobs can be submitted")
		return
	}
	cfg, err := loadTrainingConfig()
	if err != nil {
		respondTrainingError(c, err)
		return
	}
	hyperParams, err := parseHyperParams(job.HyperParams)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	dataset, datasetRef, err := resolveJobDataset(job)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	serviceJob, err := newTrainingClient(cfg).createJob(c.Request.Context(), serviceJobRequest{
		Name:        job.Name,
		BaseModel:   job.BaseModel,
		Method:      job.Method,
		DatasetId:   datasetRef,
		Template:    dataset.Pipeline,
		HyperParams: hyperParams,
	})
	if err != nil {
		respondTrainingError(c, err)
		return
	}

	updates := map[string]any{
		"status":            JobStatusQueued,
		"service_job_id":    serviceJob.Id,
		"progress":          0,
		"error_msg":         "",
		"started_at":        common.GetTimestamp(),
		"dataflow_template": dataset.Pipeline,
		"dataflow_params":   dataset.PipelineParams,
	}
	if err := model.DB.Model(&job).Updates(updates).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, job)
}

// CancelJob cancels a queued or in-flight job owned by the current user.
func CancelJob(c *gin.Context) {
	job, ok := loadOwnedJob(c)
	if !ok {
		return
	}
	if job.Status != JobStatusSubmitted && job.Status != JobStatusQueued && job.Status != JobStatusRunning {
		common.ApiErrorMsg(c, "job cannot be canceled in its current state")
		return
	}
	if job.ServiceJobId != "" {
		if cfg, err := loadTrainingConfig(); err == nil {
			if err := newTrainingClient(cfg).cancelJob(c.Request.Context(), job.ServiceJobId); err != nil {
				// A cancel that cannot reach the service is still recorded locally:
				// the user asked for the job to stop and must not be blocked by an
				// unreachable trainer.
				common.SysLog(common.LogText("failed to cancel the remote training job: %s", err.Error()))
			}
		}
	}
	if err := model.DB.Model(&job).Updates(map[string]any{
		"status":      JobStatusCanceled,
		"finished_at": common.GetTimestamp(),
	}).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminListJobs returns every fine-tuning job.
func AdminListJobs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&FinetuneJob{})
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("name LIKE ?", keyword+"%")
	}
	if status, err := strconv.Atoi(c.Query("status")); err == nil && status >= 0 {
		query = query.Where("status = ?", status)
	}
	if userId, err := strconv.Atoi(c.Query("user_id")); err == nil && userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var jobs []FinetuneJob
	if err := query.Order("id desc").Offset(pageInfo.GetStartIdx()).
		Limit(pageInfo.GetPageSize()).Find(&jobs).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	refreshJobs(jobs)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(jobs)
	common.ApiSuccess(c, pageInfo)
}

// AdminUpdateJobStatus drives the queued→running→succeeded/failed
// lifecycle on behalf of the (future) training scheduler.
func AdminUpdateJobStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid job id")
		return
	}
	var req adminJobStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Status != JobStatusQueued && req.Status != JobStatusRunning &&
		req.Status != JobStatusSucceeded && req.Status != JobStatusFailed {
		common.ApiErrorMsg(c, "invalid job status")
		return
	}
	var job FinetuneJob
	if err := model.DB.First(&job, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if job.Status != JobStatusSubmitted && job.Status != JobStatusQueued && job.Status != JobStatusRunning {
		common.ApiErrorMsg(c, "job cannot transition to the requested status")
		return
	}
	updates := map[string]any{"status": req.Status}
	if req.Status == JobStatusSucceeded {
		updates["result_model"] = req.ResultModel
	}
	if req.Status == JobStatusFailed {
		updates["error_msg"] = req.ErrorMsg
	}
	if err := model.DB.Model(&job).Updates(updates).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func loadOwnedJob(c *gin.Context) (FinetuneJob, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid job id")
		return FinetuneJob{}, false
	}
	var job FinetuneJob
	if err := model.DB.First(&job, id).Error; err != nil {
		common.ApiError(c, err)
		return FinetuneJob{}, false
	}
	if job.UserId != c.GetInt("id") && c.GetInt("role") < common.RoleAdminUser {
		common.ApiErrorMsg(c, "no permission to access this job")
		return FinetuneJob{}, false
	}
	return job, true
}

func validateJobRequest(req *jobRequest) error {
	switch strings.ToLower(strings.TrimSpace(req.Method)) {
	case MethodLora, MethodSft, MethodFull:
		req.Method = strings.ToLower(strings.TrimSpace(req.Method))
	default:
		return errors.New("unsupported fine-tuning method")
	}
	return nil
}

// refreshJobs brings a page of jobs up to date with the training service before
// the console renders them.
func refreshJobs(jobs []FinetuneJob) {
	cfg, err := loadTrainingConfig()
	if err != nil {
		// Not configured: the console keeps showing the recorded state.
		return
	}
	client := newTrainingClient(cfg)
	// Deliberately detached from the request context: a client that navigates
	// away must not abort the refresh half-way and leave the row inconsistent.
	ctx, cancel := context.WithTimeout(context.Background(), remoteRefreshTimeout)
	defer cancel()

	refreshed := 0
	for i := range jobs {
		if refreshed >= remoteRefreshBatchLimit {
			return
		}
		if !jobNeedsRemoteRefresh(jobs[i]) || !claimRemoteRefresh(jobs[i].Id) {
			continue
		}
		refreshed++
		applyRemoteJobState(ctx, client, &jobs[i])
	}
}

// refreshJob brings a single job up to date with the training service.
func refreshJob(job *FinetuneJob) {
	if !jobNeedsRemoteRefresh(*job) || !claimRemoteRefresh(job.Id) {
		return
	}
	cfg, err := loadTrainingConfig()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), remoteRefreshTimeout)
	defer cancel()
	applyRemoteJobState(ctx, newTrainingClient(cfg), job)
}

// parseHyperParams turns the stored hyper-parameter JSON into the object the
// training service expects. Ranges are validated by the service, which owns the
// training framework's real limits; this only rejects malformed input.
func parseHyperParams(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	params := make(map[string]any)
	if err := common.UnmarshalJsonStr(raw, &params); err != nil {
		return nil, errors.New("hyper parameters must be a JSON object")
	}
	if params == nil {
		// A literal JSON `null` clears the map instead of leaving it empty.
		return map[string]any{}, nil
	}
	return params, nil
}

// resolveJobDataset returns the dataset a job trains on together with the id the
// training service knows it by. A prepared dataset is preferred: its DataFlow
// output is what the job should consume, not the raw upload.
func resolveJobDataset(job FinetuneJob) (TrainingDataset, string, error) {
	if job.DatasetId == 0 {
		return TrainingDataset{}, "", errors.New("select a dataset before submitting this job")
	}
	var dataset TrainingDataset
	if err := model.DB.First(&dataset, job.DatasetId).Error; err != nil {
		return TrainingDataset{}, "", errors.New("the selected dataset no longer exists")
	}
	if dataset.UserId != job.UserId {
		return TrainingDataset{}, "", errors.New("the selected dataset belongs to another user")
	}
	if dataset.Status != DatasetStatusReady {
		return TrainingDataset{}, "", errors.New("the selected dataset is not ready")
	}
	datasetRef := dataset.PreparedRef
	if datasetRef == "" {
		datasetRef = dataset.RemoteId
	}
	if datasetRef == "" {
		return TrainingDataset{}, "", errors.New("the selected dataset has no file on the training service")
	}
	return dataset, datasetRef, nil
}
