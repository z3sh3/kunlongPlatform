package modelsquare

import (
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
	if err := model.DB.Model(&job).Updates(updates).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, job)
}

// DeleteJob deletes a draft or terminal job owned by the current user.
func DeleteJob(c *gin.Context) {
	job, ok := loadOwnedJob(c)
	if !ok {
		return
	}
	if job.Status != JobStatusDraft && job.Status != JobStatusCanceled && job.Status != JobStatusFailed {
		common.ApiErrorMsg(c, "only draft, canceled or failed jobs can be deleted")
		return
	}
	if err := model.DB.Delete(&job).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// SubmitJob moves a draft job to submitted, entering the administrator
// queue. Actual training dispatch is a follow-up.
func SubmitJob(c *gin.Context) {
	job, ok := loadOwnedJob(c)
	if !ok {
		return
	}
	if job.Status != JobStatusDraft {
		common.ApiErrorMsg(c, "only draft jobs can be submitted")
		return
	}
	if err := model.DB.Model(&job).Update("status", JobStatusSubmitted).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
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
	if err := model.DB.Model(&job).Update("status", JobStatusCanceled).Error; err != nil {
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
