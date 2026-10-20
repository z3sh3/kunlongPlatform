package modelsquare

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type registerArtifactRequest struct {
	Name string `json:"name"`
}

type registerModelRequest struct {
	Description string `json:"description"`
	Tags        string `json:"tags"`
}

type registerChannelRequest struct {
	BaseURL string `json:"base_url" binding:"required"`
	Key     string `json:"key" binding:"required"`
	Group   string `json:"group"`
	Name    string `json:"name"`
}

type trainingServiceTestResponse struct {
	Configured bool          `json:"configured"`
	Health     serviceHealth `json:"health"`
}

// ListArtifacts returns the current user's trained model assets.
func ListArtifacts(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&TrainedModel{}).Where("user_id = ?", c.GetInt("id"))
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		query = query.Where("name LIKE ?", keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var artifacts []TrainedModel
	if err := query.Order("id desc").Offset(pageInfo.GetStartIdx()).
		Limit(pageInfo.GetPageSize()).Find(&artifacts).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(artifacts)
	common.ApiSuccess(c, pageInfo)
}

// RegisterArtifact records a succeeded job's output as a reusable model asset.
// This is owner-scoped: publishing to the platform model catalog and creating an
// inference channel stay administrator actions.
func RegisterArtifact(c *gin.Context) {
	job, ok := loadOwnedJob(c)
	if !ok {
		return
	}
	if job.Status != JobStatusSucceeded {
		common.ApiErrorMsg(c, "only succeeded jobs can produce a model asset")
		return
	}

	var req registerArtifactRequest
	// The request body is optional: an empty body means "use the job name".
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		common.ApiError(c, err)
		return
	}
	name := firstNonEmpty(req.Name, job.ResultModel, job.Name)
	if name == "" {
		common.ApiErrorMsg(c, "a model asset name is required")
		return
	}

	var existing int64
	if err := model.DB.Model(&TrainedModel{}).
		Where("user_id = ? AND name = ?", job.UserId, name).
		Count(&existing).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if existing > 0 {
		common.ApiErrorMsg(c, "a model asset with this name already exists")
		return
	}

	artifactKind := ""
	sizeBytes := int64(0)
	if job.OutputDir == "" {
		common.ApiErrorMsg(c, "this job has no recorded output directory")
		return
	}
	// The artifact kind and size come from the service and were mirrored into the
	// job's usage payload when the job finished.
	var usage struct {
		Artifact struct {
			Kind      string `json:"kind"`
			SizeBytes int64  `json:"size_bytes"`
		} `json:"artifact"`
	}
	if job.Usage != "" {
		if err := common.UnmarshalJsonStr(job.Usage, &usage); err == nil {
			artifactKind = usage.Artifact.Kind
			sizeBytes = usage.Artifact.SizeBytes
		}
	}

	artifact := TrainedModel{
		UserId:       job.UserId,
		JobId:        job.Id,
		Name:         name,
		BaseModel:    job.BaseModel,
		Method:       job.Method,
		ArtifactKind: artifactKind,
		ArtifactRef:  job.OutputDir,
		SizeBytes:    sizeBytes,
		Status:       ArtifactStatusAvailable,
	}
	if err := model.DB.Create(&artifact).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, artifact)
}

// AdminPromoteArtifactToModel publishes an artifact into the platform model
// catalog so it appears in the model square alongside upstream models.
func AdminPromoteArtifactToModel(c *gin.Context) {
	artifact, ok := loadArtifact(c)
	if !ok {
		return
	}
	var req registerModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	var catalog model.Model
	err := model.DB.Where("model_name = ?", artifact.Name).First(&catalog).Error
	if err == nil {
		common.ApiErrorMsg(c, "a model with this name already exists in the catalog")
		return
	}

	catalog = model.Model{
		ModelName:   artifact.Name,
		Description: req.Description,
		Tags:        req.Tags,
		Status:      1,
		CreatedTime: common.GetTimestamp(),
		UpdatedTime: common.GetTimestamp(),
	}
	if err := catalog.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DB.Model(&artifact).Update("model_catalog_id", catalog.Id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, catalog)
}

// AdminRegisterChannelForArtifact creates an OpenAI-compatible channel that
// serves the trained model, so the artifact becomes usable inside the platform.
func AdminRegisterChannelForArtifact(c *gin.Context) {
	artifact, ok := loadArtifact(c)
	if !ok {
		return
	}
	if artifact.ChannelId > 0 {
		common.ApiErrorMsg(c, "this model asset already has a channel")
		return
	}
	var req registerChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	group := firstNonEmpty(req.Group, "default")
	channel := model.Channel{
		Type:        constant.ChannelTypeOpenAI,
		Key:         req.Key,
		Status:      common.ChannelStatusEnabled,
		Name:        firstNonEmpty(req.Name, artifact.Name),
		CreatedTime: common.GetTimestamp(),
		BaseURL:     &req.BaseURL,
		Models:      artifact.Name,
		Group:       group,
	}
	if err := channel.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DB.Model(&artifact).Update("channel_id", channel.Id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"channel_id": channel.Id})
}

// loadArtifact loads an artifact by id for an administrator.
func loadArtifact(c *gin.Context) (TrainedModel, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid model asset id")
		return TrainedModel{}, false
	}
	var artifact TrainedModel
	if err := model.DB.First(&artifact, id).Error; err != nil {
		common.ApiError(c, err)
		return TrainedModel{}, false
	}
	return artifact, true
}

// TrainingCapabilities proxies the service's pipeline and hyper-parameter
// schema so the console can render its forms without hard-coding templates.
func TrainingCapabilities(c *gin.Context) {
	cfg, err := loadTrainingConfig()
	if err != nil {
		respondTrainingError(c, err)
		return
	}
	caps, err := newTrainingClient(cfg).capabilities(c.Request.Context())
	if err != nil {
		respondTrainingError(c, err)
		return
	}
	common.ApiSuccess(c, caps)
}

// AdminTestTrainingService reports connectivity, framework versions and GPUs for
// the console's "test connection" button.
func AdminTestTrainingService(c *gin.Context) {
	cfg, err := loadTrainingConfig()
	if err != nil {
		respondTrainingError(c, err)
		return
	}
	health, err := newTrainingClient(cfg).health(c.Request.Context())
	if err != nil {
		respondTrainingError(c, err)
		return
	}
	common.ApiSuccess(c, trainingServiceTestResponse{
		Configured: true,
		Health:     health,
	})
}
