package enterprise

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modules/shared"
	"github.com/gin-gonic/gin"
)

type programRequest struct {
	Title        string `json:"title" binding:"required"`
	Type         string `json:"type" binding:"required"`
	Summary      string `json:"summary"`
	CoverUrl     string `json:"cover_url"`
	Description  string `json:"description"`
	Requirements string `json:"requirements"`
}

type programStatusRequest struct {
	Status int `json:"status" binding:"required"`
}

type applicationRequest struct {
	ProgramId    int    `json:"program_id" binding:"required"`
	CompanyName  string `json:"company_name" binding:"required"`
	ContactName  string `json:"contact_name" binding:"required"`
	ContactPhone string `json:"contact_phone" binding:"required"`
	Description  string `json:"description" binding:"required"`
}

type reviewApplicationRequest struct {
	Status  int    `json:"status" binding:"required"`
	Comment string `json:"comment"`
}

// ListPrograms returns published incubation programs, optionally filtered
// by type.
func ListPrograms(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&IncubationProgram{}).Where("status = ?", shared.PublishStatusPublished)
	if programType := c.Query("type"); programType != "" {
		query = query.Where("type = ?", programType)
	}
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("title LIKE ?", keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var programs []IncubationProgram
	if err := query.Order("id desc").Offset(pageInfo.GetStartIdx()).
		Limit(pageInfo.GetPageSize()).Find(&programs).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(programs)
	common.ApiSuccess(c, pageInfo)
}

// GetProgram returns one published program.
func GetProgram(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid program id")
		return
	}
	var program IncubationProgram
	if err := model.DB.First(&program, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if program.Status != shared.PublishStatusPublished {
		common.ApiErrorMsg(c, "program is not available")
		return
	}
	common.ApiSuccess(c, program)
}

// ListMyApplications returns the current user's applications.
func ListMyApplications(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&IncubationApplication{}).Where("user_id = ?", c.GetInt("id"))
	if programId, err := strconv.Atoi(c.Query("program_id")); err == nil && programId > 0 {
		query = query.Where("program_id = ?", programId)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var applications []IncubationApplication
	if err := query.Preload("Program").Order("id desc").
		Offset(pageInfo.GetStartIdx()).Limit(pageInfo.GetPageSize()).
		Find(&applications).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(applications)
	common.ApiSuccess(c, pageInfo)
}

// CreateApplication submits an application to a published program.
func CreateApplication(c *gin.Context) {
	var req applicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	var program IncubationProgram
	if err := model.DB.First(&program, req.ProgramId).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if program.Status != shared.PublishStatusPublished {
		common.ApiErrorMsg(c, "program is not available")
		return
	}
	application := IncubationApplication{
		ProgramId:    program.Id,
		UserId:       c.GetInt("id"),
		CompanyName:  req.CompanyName,
		ContactName:  req.ContactName,
		ContactPhone: req.ContactPhone,
		Description:  req.Description,
		Status:       shared.ReviewStatusPending,
	}
	if err := model.DB.Create(&application).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, application)
}

// AdminListPrograms returns every program regardless of status.
func AdminListPrograms(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&IncubationProgram{})
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("title LIKE ?", keyword+"%")
	}
	if programType := c.Query("type"); programType != "" {
		query = query.Where("type = ?", programType)
	}
	if status, err := strconv.Atoi(c.Query("status")); err == nil && status >= 0 {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var programs []IncubationProgram
	if err := query.Order("id desc").Offset(pageInfo.GetStartIdx()).
		Limit(pageInfo.GetPageSize()).Find(&programs).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(programs)
	common.ApiSuccess(c, pageInfo)
}

// AdminCreateProgram creates a draft program.
func AdminCreateProgram(c *gin.Context) {
	var req programRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateProgramRequest(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	program := IncubationProgram{
		Title:        req.Title,
		Type:         req.Type,
		Summary:      shared.NormalizeSummary(req.Summary),
		CoverUrl:     shared.NormalizeCoverUrl(req.CoverUrl),
		Description:  req.Description,
		Requirements: req.Requirements,
		Status:       shared.PublishStatusDraft,
	}
	if err := model.DB.Create(&program).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, program)
}

// AdminUpdateProgram edits a program.
func AdminUpdateProgram(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid program id")
		return
	}
	var program IncubationProgram
	if err := model.DB.First(&program, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var req programRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateProgramRequest(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	updates := map[string]any{
		"title":        req.Title,
		"type":         req.Type,
		"description":  req.Description,
		"summary":      shared.NormalizeSummary(req.Summary),
		"cover_url":    shared.NormalizeCoverUrl(req.CoverUrl),
		"requirements": req.Requirements,
	}
	if err := model.DB.Model(&program).Updates(updates).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, program)
}

// AdminDeleteProgram deletes a program.
func AdminDeleteProgram(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid program id")
		return
	}
	if err := model.DB.Delete(&IncubationProgram{}, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminUpdateProgramStatus publishes or takes a program offline.
func AdminUpdateProgramStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid program id")
		return
	}
	var req programStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Status != shared.PublishStatusDraft && req.Status != shared.PublishStatusPublished && req.Status != shared.PublishStatusOffline {
		common.ApiErrorMsg(c, "invalid status")
		return
	}
	result := model.DB.Model(&IncubationProgram{}).Where("id = ?", id).Update("status", req.Status)
	if result.Error != nil {
		common.ApiError(c, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		common.ApiErrorMsg(c, "program not found")
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminListApplications returns applications with program info.
func AdminListApplications(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&IncubationApplication{})
	if programId, err := strconv.Atoi(c.Query("program_id")); err == nil && programId > 0 {
		query = query.Where("program_id = ?", programId)
	}
	if status, err := strconv.Atoi(c.Query("status")); err == nil && status >= 0 {
		query = query.Where("status = ?", status)
	}
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("company_name LIKE ?", keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var applications []IncubationApplication
	if err := query.Preload("Program").Order("id desc").
		Offset(pageInfo.GetStartIdx()).Limit(pageInfo.GetPageSize()).
		Find(&applications).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(applications)
	common.ApiSuccess(c, pageInfo)
}

// AdminReviewApplication moves an application through the review flow:
// pending → reviewing → approved/rejected.
func AdminReviewApplication(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid application id")
		return
	}
	var req reviewApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Status != shared.ReviewStatusReviewing && req.Status != shared.ReviewStatusApproved && req.Status != shared.ReviewStatusRejected {
		common.ApiErrorMsg(c, "invalid review status")
		return
	}
	var application IncubationApplication
	if err := model.DB.First(&application, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if application.Status != shared.ReviewStatusPending && application.Status != shared.ReviewStatusReviewing {
		common.ApiErrorMsg(c, "application review is already closed")
		return
	}
	updates := map[string]any{
		"status":         req.Status,
		"review_comment": req.Comment,
	}
	if err := model.DB.Model(&application).Updates(updates).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func validateProgramRequest(req *programRequest) error {
	switch req.Type {
	case TypeIncubation, TypeSMESupport, TypeGovPark:
		return nil
	default:
		return errors.New("invalid program type")
	}
}
