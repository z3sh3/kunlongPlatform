package datasetmarket

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modules/shared"
	"github.com/gin-gonic/gin"
)

type createDatasetRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Format      string `json:"format"`
	Tags        string `json:"tags"`
	FileUrl     string `json:"file_url"`
	CoverUrl    string `json:"cover_url"`
	SizeBytes   int64  `json:"size_bytes"`
	PriceMode   int    `json:"price_mode"`
	UnitPrice   int64  `json:"unit_price"`
}

type updateDatasetStatusRequest struct {
	Status int `json:"status" binding:"required"`
}

type reviewGrantRequest struct {
	Status    int    `json:"status" binding:"required"`
	ExpiresAt int64  `json:"expires_at"`
	Remark    string `json:"remark"`
}

// ListDatasets returns published datasets for browsing, or the current
// user's own datasets of any status when mine=1.
func ListDatasets(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&Dataset{})
	if c.Query("mine") == "1" {
		query = query.Where("user_id = ?", c.GetInt("id"))
	} else {
		query = query.Where("status = ?", shared.PublishStatusPublished)
		if keyword := c.Query("keyword"); keyword != "" {
			query = query.Where("name LIKE ?", keyword+"%")
		}
		if category := c.Query("category"); category != "" {
			query = query.Where("category = ?", category)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var datasets []Dataset
	if err := query.Order("id desc").Offset(pageInfo.GetStartIdx()).Limit(pageInfo.GetPageSize()).Find(&datasets).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(datasets)
	common.ApiSuccess(c, pageInfo)
}

// GetDataset returns one dataset. Published datasets are visible to every
// logged-in user; drafts are limited to the owner and administrators.
func GetDataset(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid dataset id")
		return
	}
	var dataset Dataset
	if err := model.DB.First(&dataset, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	userId := c.GetInt("id")
	if dataset.Status != shared.PublishStatusPublished && dataset.UserId != userId && !isAdmin(c) {
		common.ApiErrorMsg(c, "no permission to view this dataset")
		return
	}
	common.ApiSuccess(c, dataset)
}

// CreateDataset lets a user submit a dataset as a draft for later review.
func CreateDataset(c *gin.Context) {
	var req createDatasetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validatePrice(req.PriceMode, req.UnitPrice); err != nil {
		common.ApiError(c, err)
		return
	}
	dataset := Dataset{
		UserId:      c.GetInt("id"),
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		Format:      req.Format,
		Tags:        req.Tags,
		FileUrl:     req.FileUrl,
		CoverUrl:    req.CoverUrl,
		SizeBytes:   req.SizeBytes,
		PriceMode:   req.PriceMode,
		UnitPrice:   req.UnitPrice,
		Status:      shared.PublishStatusDraft,
	}
	if err := model.DB.Create(&dataset).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, dataset)
}

// UpdateDataset lets the owner edit a draft, or an administrator edit any
// dataset.
func UpdateDataset(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid dataset id")
		return
	}
	var dataset Dataset
	if err := model.DB.First(&dataset, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if !canManage(c, dataset) {
		common.ApiErrorMsg(c, "no permission to edit this dataset")
		return
	}
	var req createDatasetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validatePrice(req.PriceMode, req.UnitPrice); err != nil {
		common.ApiError(c, err)
		return
	}
	updates := map[string]any{
		"name":        req.Name,
		"description": req.Description,
		"category":    req.Category,
		"format":      req.Format,
		"tags":        req.Tags,
		"file_url":    req.FileUrl,
		"cover_url":   req.CoverUrl,
		"size_bytes":  req.SizeBytes,
		"price_mode":  req.PriceMode,
		"unit_price":  req.UnitPrice,
	}
	if err := model.DB.Model(&dataset).Updates(updates).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, dataset)
}

// DeleteDataset removes a dataset. Owners may delete their own unpublished
// datasets; administrators may delete any dataset.
func DeleteDataset(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid dataset id")
		return
	}
	var dataset Dataset
	if err := model.DB.First(&dataset, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	userId := c.GetInt("id")
	isAdminUser := isAdmin(c)
	if dataset.UserId != userId && !isAdminUser {
		common.ApiErrorMsg(c, "no permission to delete this dataset")
		return
	}
	if dataset.UserId == userId && !isAdminUser && dataset.Status == shared.PublishStatusPublished {
		common.ApiErrorMsg(c, "published datasets can only be deleted by an administrator")
		return
	}
	if err := model.DB.Delete(&dataset).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// ApplyDataset requests authorized use of a published dataset. Free
// datasets are granted immediately; paid ones stay pending for
// administrator review (billing settlement is a follow-up).
func ApplyDataset(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid dataset id")
		return
	}
	var dataset Dataset
	if err := model.DB.First(&dataset, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if dataset.Status != shared.PublishStatusPublished {
		common.ApiErrorMsg(c, "dataset is not available")
		return
	}
	userId := c.GetInt("id")
	if dataset.UserId == userId {
		common.ApiErrorMsg(c, "no need to apply for your own dataset")
		return
	}
	var count int64
	if err := model.DB.Model(&DatasetGrant{}).
		Where("dataset_id = ? AND user_id = ? AND status IN ?", dataset.Id, userId,
			[]int{shared.ReviewStatusPending, shared.ReviewStatusApproved}).
		Count(&count).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if count > 0 {
		common.ApiErrorMsg(c, "already applied for this dataset")
		return
	}
	grant := DatasetGrant{DatasetId: dataset.Id, UserId: userId}
	if dataset.PriceMode == shared.PriceModeFree {
		grant.Status = shared.ReviewStatusApproved
	} else {
		grant.Status = shared.ReviewStatusPending
	}
	if err := model.DB.Create(&grant).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, grant)
}

// ListMyGrants returns the current user's dataset grants.
func ListMyGrants(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&DatasetGrant{}).Where("user_id = ?", c.GetInt("id"))
	if datasetId, err := strconv.Atoi(c.Query("dataset_id")); err == nil && datasetId > 0 {
		query = query.Where("dataset_id = ?", datasetId)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var grants []DatasetGrant
	if err := query.Preload("Dataset").Order("id desc").
		Offset(pageInfo.GetStartIdx()).Limit(pageInfo.GetPageSize()).
		Find(&grants).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(grants)
	common.ApiSuccess(c, pageInfo)
}

// AdminListDatasets returns all datasets regardless of status.
func AdminListDatasets(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&Dataset{})
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
	var datasets []Dataset
	if err := query.Order("id desc").Offset(pageInfo.GetStartIdx()).
		Limit(pageInfo.GetPageSize()).Find(&datasets).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(datasets)
	common.ApiSuccess(c, pageInfo)
}

// AdminUpdateDatasetStatus publishes or takes a dataset offline.
func AdminUpdateDatasetStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid dataset id")
		return
	}
	var req updateDatasetStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Status != shared.PublishStatusDraft && req.Status != shared.PublishStatusPublished && req.Status != shared.PublishStatusOffline {
		common.ApiErrorMsg(c, "invalid status")
		return
	}
	result := model.DB.Model(&Dataset{}).Where("id = ?", id).Update("status", req.Status)
	if result.Error != nil {
		common.ApiError(c, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		common.ApiErrorMsg(c, "dataset not found")
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminListGrants returns every dataset grant regardless of status.
func AdminListGrants(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&DatasetGrant{})
	if status, err := strconv.Atoi(c.Query("status")); err == nil && status >= 0 {
		query = query.Where("status = ?", status)
	}
	if datasetId, err := strconv.Atoi(c.Query("dataset_id")); err == nil && datasetId > 0 {
		query = query.Where("dataset_id = ?", datasetId)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var grants []DatasetGrant
	if err := query.Preload("Dataset").Order("id desc").
		Offset(pageInfo.GetStartIdx()).Limit(pageInfo.GetPageSize()).
		Find(&grants).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(grants)
	common.ApiSuccess(c, pageInfo)
}

// AdminReviewGrant approves or rejects a pending dataset grant.
func AdminReviewGrant(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid grant id")
		return
	}
	var req reviewGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Status != shared.ReviewStatusApproved && req.Status != shared.ReviewStatusRejected {
		common.ApiErrorMsg(c, "invalid review status")
		return
	}
	var grant DatasetGrant
	if err := model.DB.First(&grant, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if grant.Status != shared.ReviewStatusPending {
		common.ApiErrorMsg(c, "grant has already been reviewed")
		return
	}
	updates := map[string]any{
		"status":     req.Status,
		"expires_at": req.ExpiresAt,
		"remark":     req.Remark,
	}
	if err := model.DB.Model(&grant).Updates(updates).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func isAdmin(c *gin.Context) bool {
	return c.GetInt("role") >= common.RoleAdminUser
}

func canManage(c *gin.Context, dataset Dataset) bool {
	if isAdmin(c) {
		return true
	}
	return dataset.UserId == c.GetInt("id") && dataset.Status == shared.PublishStatusDraft
}

func validatePrice(priceMode int, unitPrice int64) error {
	if priceMode != shared.PriceModeFree && priceMode != shared.PriceModePaid {
		return errors.New("invalid price mode")
	}
	if priceMode == shared.PriceModePaid && unitPrice <= 0 {
		return errors.New("paid datasets require a positive unit price")
	}
	return nil
}
