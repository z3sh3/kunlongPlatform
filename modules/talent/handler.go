package talent

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modules/shared"
	"github.com/gin-gonic/gin"
)

type courseRequest struct {
	Title       string `json:"title" binding:"required"`
	Category    string `json:"category" binding:"required"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Schedule    string `json:"schedule"`
	Capacity    int    `json:"capacity"`
	PriceMode   int    `json:"price_mode"`
	Price       int64  `json:"price"`
}

type courseStatusRequest struct {
	Status int `json:"status" binding:"required"`
}

type reviewEnrollmentRequest struct {
	Status int    `json:"status" binding:"required"`
	Remark string `json:"remark"`
}

// ListCourses returns published courses, optionally filtered by category.
func ListCourses(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&TrainingCourse{}).Where("status = ?", shared.PublishStatusPublished)
	if category := c.Query("category"); category != "" {
		query = query.Where("category = ?", category)
	}
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("title LIKE ?", keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var courses []TrainingCourse
	if err := query.Order("id desc").Offset(pageInfo.GetStartIdx()).
		Limit(pageInfo.GetPageSize()).Find(&courses).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(courses)
	common.ApiSuccess(c, pageInfo)
}

// GetCourse returns one published course.
func GetCourse(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid course id")
		return
	}
	var course TrainingCourse
	if err := model.DB.First(&course, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if course.Status != shared.PublishStatusPublished {
		common.ApiErrorMsg(c, "course is not available")
		return
	}
	common.ApiSuccess(c, course)
}

// ListMyEnrollments returns the current user's enrollment records.
func ListMyEnrollments(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&TrainingEnrollment{}).Where("user_id = ?", c.GetInt("id"))
	if courseId, err := strconv.Atoi(c.Query("course_id")); err == nil && courseId > 0 {
		query = query.Where("course_id = ?", courseId)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var enrollments []TrainingEnrollment
	if err := query.Preload("Course").Order("id desc").
		Offset(pageInfo.GetStartIdx()).Limit(pageInfo.GetPageSize()).
		Find(&enrollments).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(enrollments)
	common.ApiSuccess(c, pageInfo)
}

// Enroll requests a seat in a published course. Capacity counts pending
// and approved enrollments.
func Enroll(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid course id")
		return
	}
	var course TrainingCourse
	if err := model.DB.First(&course, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if course.Status != shared.PublishStatusPublished {
		common.ApiErrorMsg(c, "course is not available")
		return
	}
	userId := c.GetInt("id")
	var count int64
	if err := model.DB.Model(&TrainingEnrollment{}).
		Where("course_id = ? AND user_id = ? AND status IN ?", course.Id, userId,
			[]int{shared.ReviewStatusPending, shared.ReviewStatusApproved, EnrollmentStatusCompleted}).
		Count(&count).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if count > 0 {
		common.ApiErrorMsg(c, "already enrolled in this course")
		return
	}
	if course.Capacity > 0 {
		if err := model.DB.Model(&TrainingEnrollment{}).
			Where("course_id = ? AND status IN ?", course.Id,
				[]int{shared.ReviewStatusPending, shared.ReviewStatusApproved}).
			Count(&count).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		if count >= int64(course.Capacity) {
			common.ApiErrorMsg(c, "course is fully booked")
			return
		}
	}
	enrollment := TrainingEnrollment{
		CourseId: course.Id,
		UserId:   userId,
		Status:   shared.ReviewStatusPending,
	}
	if err := model.DB.Create(&enrollment).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, enrollment)
}

// AdminListCourses returns every course regardless of status.
func AdminListCourses(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&TrainingCourse{})
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("title LIKE ?", keyword+"%")
	}
	if status, err := strconv.Atoi(c.Query("status")); err == nil && status >= 0 {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var courses []TrainingCourse
	if err := query.Order("id desc").Offset(pageInfo.GetStartIdx()).
		Limit(pageInfo.GetPageSize()).Find(&courses).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(courses)
	common.ApiSuccess(c, pageInfo)
}

// AdminCreateCourse creates a draft course.
func AdminCreateCourse(c *gin.Context) {
	var req courseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateCourseRequest(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	course := TrainingCourse{
		Title:       req.Title,
		Category:    req.Category,
		Description: req.Description,
		Content:     req.Content,
		Schedule:    req.Schedule,
		Capacity:    req.Capacity,
		PriceMode:   req.PriceMode,
		Price:       req.Price,
		Status:      shared.PublishStatusDraft,
	}
	if err := model.DB.Create(&course).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, course)
}

// AdminUpdateCourse edits a course.
func AdminUpdateCourse(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid course id")
		return
	}
	var course TrainingCourse
	if err := model.DB.First(&course, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var req courseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateCourseRequest(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	updates := map[string]any{
		"title":       req.Title,
		"category":    req.Category,
		"description": req.Description,
		"content":     req.Content,
		"schedule":    req.Schedule,
		"capacity":    req.Capacity,
		"price_mode":  req.PriceMode,
		"price":       req.Price,
	}
	if err := model.DB.Model(&course).Updates(updates).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, course)
}

// AdminDeleteCourse deletes a course.
func AdminDeleteCourse(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid course id")
		return
	}
	if err := model.DB.Delete(&TrainingCourse{}, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminUpdateCourseStatus publishes or takes a course offline.
func AdminUpdateCourseStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid course id")
		return
	}
	var req courseStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Status != shared.PublishStatusDraft && req.Status != shared.PublishStatusPublished && req.Status != shared.PublishStatusOffline {
		common.ApiErrorMsg(c, "invalid status")
		return
	}
	result := model.DB.Model(&TrainingCourse{}).Where("id = ?", id).Update("status", req.Status)
	if result.Error != nil {
		common.ApiError(c, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		common.ApiErrorMsg(c, "course not found")
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminListEnrollments returns enrollment records with course info.
func AdminListEnrollments(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&TrainingEnrollment{})
	if courseId, err := strconv.Atoi(c.Query("course_id")); err == nil && courseId > 0 {
		query = query.Where("course_id = ?", courseId)
	}
	if status, err := strconv.Atoi(c.Query("status")); err == nil && status >= 0 {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var enrollments []TrainingEnrollment
	if err := query.Preload("Course").Order("id desc").
		Offset(pageInfo.GetStartIdx()).Limit(pageInfo.GetPageSize()).
		Find(&enrollments).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(enrollments)
	common.ApiSuccess(c, pageInfo)
}

// AdminReviewEnrollment approves, rejects or completes an enrollment.
func AdminReviewEnrollment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid enrollment id")
		return
	}
	var req reviewEnrollmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Status != shared.ReviewStatusApproved && req.Status != shared.ReviewStatusRejected && req.Status != EnrollmentStatusCompleted {
		common.ApiErrorMsg(c, "invalid enrollment status")
		return
	}
	var enrollment TrainingEnrollment
	if err := model.DB.First(&enrollment, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if enrollment.Status == shared.ReviewStatusRejected || enrollment.Status == EnrollmentStatusCompleted {
		common.ApiErrorMsg(c, "enrollment is already closed")
		return
	}
	if req.Status == EnrollmentStatusCompleted && enrollment.Status != shared.ReviewStatusApproved {
		common.ApiErrorMsg(c, "only approved enrollments can be completed")
		return
	}
	updates := map[string]any{
		"status": req.Status,
		"remark": req.Remark,
	}
	if err := model.DB.Model(&enrollment).Updates(updates).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func validateCourseRequest(req *courseRequest) error {
	if req.Category != CategoryCertification && req.Category != CategoryConsultation {
		return errors.New("invalid course category")
	}
	if req.PriceMode != shared.PriceModeFree && req.PriceMode != shared.PriceModePaid {
		return errors.New("invalid price mode")
	}
	if req.PriceMode == shared.PriceModePaid && req.Price <= 0 {
		return errors.New("paid courses require a positive price")
	}
	if req.Capacity < 0 {
		return errors.New("capacity cannot be negative")
	}
	return nil
}
