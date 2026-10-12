package talent

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modules/shared"
	"github.com/QuantumNous/new-api/modules/shared/testauth"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRouter(t *testing.T) *gin.Engine {
	t.Helper()
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&TrainingCourse{}, &TrainingEnrollment{}))
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerRoutes(r.Group("/api"), testauth.UserAuth, testauth.AdminAuth)
	return r
}

func requireSuccess(t *testing.T, resp map[string]any) {
	t.Helper()
	require.Equal(t, true, resp["success"], "unexpected failure: %v", resp["message"])
}

func requireFailure(t *testing.T, resp map[string]any) {
	t.Helper()
	require.Equal(t, false, resp["success"], "expected the request to fail")
}

// publishCourse creates a course as admin and publishes it.
func publishCourse(t *testing.T, r *gin.Engine, title string, capacity int) {
	t.Helper()
	_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/talent-training/admin/courses", 9, common.RoleAdminUser,
		map[string]any{"title": title, "category": CategoryCertification, "capacity": capacity, "price_mode": shared.PriceModeFree})
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/talent-training/admin/courses/1/status", 9, common.RoleAdminUser,
		map[string]any{"status": shared.PublishStatusPublished})
	require.NoError(t, err)
	requireSuccess(t, resp)
}

func TestTrainingEnrollmentFlow(t *testing.T) {
	r := setupRouter(t)
	publishCourse(t, r, "LLM basics", 1)

	// Invalid categories are rejected.
	_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/talent-training/admin/courses", 9, common.RoleAdminUser,
		map[string]any{"title": "bad", "category": "yoga", "price_mode": shared.PriceModeFree})
	require.NoError(t, err)
	requireFailure(t, resp)

	// A user enrolls in the published course.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/talent-training/courses/1/enroll", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)

	// Duplicate enrollment is rejected.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/talent-training/courses/1/enroll", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)

	// Capacity 1: a second user cannot enroll.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/talent-training/courses/1/enroll", 3, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)

	// The admin approves the enrollment and later marks it completed.
	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/talent-training/admin/enrollments/1/review", 9, common.RoleAdminUser,
		map[string]any{"status": shared.ReviewStatusApproved})
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/talent-training/admin/enrollments/1/review", 9, common.RoleAdminUser,
		map[string]any{"status": EnrollmentStatusCompleted})
	require.NoError(t, err)
	requireSuccess(t, resp)

	// Completed enrollments are closed for further review.
	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/talent-training/admin/enrollments/1/review", 9, common.RoleAdminUser,
		map[string]any{"status": shared.ReviewStatusRejected})
	require.NoError(t, err)
	requireFailure(t, resp)

	// The user's enrollment list preloads the course.
	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/talent-training/enrollments/my", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	items, _ := data["items"].([]any)
	require.Len(t, items, 1)
	enrollment, _ := items[0].(map[string]any)
	course, hasCourse := enrollment["course"].(map[string]any)
	require.True(t, hasCourse, "enrollment should preload course info")
	assert.Equal(t, "LLM basics", course["title"])
}

func TestTrainingCourseAdminGating(t *testing.T) {
	r := setupRouter(t)

	// Non-admin users cannot create courses.
	w, _, err := testauth.DoJSON(r, http.MethodPost, "/api/talent-training/admin/courses", 1, common.RoleCommonUser,
		map[string]any{"title": "x", "category": CategoryCertification, "price_mode": shared.PriceModeFree})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Unpublished courses are invisible to users.
	_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/talent-training/admin/courses", 9, common.RoleAdminUser,
		map[string]any{"title": "draft course", "category": CategoryConsultation, "price_mode": shared.PriceModeFree})
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/talent-training/courses", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	items, _ := data["items"].([]any)
	assert.Empty(t, items)
}
