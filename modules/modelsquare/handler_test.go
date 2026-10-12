package modelsquare

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
	require.NoError(t, db.AutoMigrate(&FinetuneJob{}))
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

func createDraftJob(t *testing.T, r *gin.Engine, userId int) map[string]any {
	t.Helper()
	_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/model-training/jobs", userId, common.RoleCommonUser,
		map[string]any{"name": "my job", "base_model": "gpt-x", "method": MethodLora})
	require.NoError(t, err)
	requireSuccess(t, resp)
	job, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	return job
}

func TestFinetuneJobLifecycle(t *testing.T) {
	r := setupRouter(t)
	createDraftJob(t, r, 1)

	// Drafts are editable and deletable by the owner.
	_, resp, err := testauth.DoJSON(r, http.MethodPut, "/api/model-training/jobs/1", 1, common.RoleCommonUser,
		map[string]any{"name": "renamed", "base_model": "gpt-y", "method": MethodSft})
	require.NoError(t, err)
	requireSuccess(t, resp)

	// Another user cannot touch the job.
	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/model-training/jobs/1", 2, common.RoleCommonUser,
		map[string]any{"name": "hijack", "base_model": "gpt-y", "method": MethodSft})
	require.NoError(t, err)
	requireFailure(t, resp)

	// Unsupported methods are rejected.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/model-training/jobs", 1, common.RoleCommonUser,
		map[string]any{"name": "bad", "base_model": "gpt-x", "method": "quantum"})
	require.NoError(t, err)
	requireFailure(t, resp)

	// Submit moves the job into the admin queue; drafts can then no longer
	// be edited.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/model-training/jobs/1/submit", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/model-training/jobs/1", 1, common.RoleCommonUser,
		map[string]any{"name": "renamed again", "base_model": "gpt-y", "method": MethodSft})
	require.NoError(t, err)
	requireFailure(t, resp)

	// The admin drives the training lifecycle.
	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/model-training/admin/jobs/1/status", 9, common.RoleAdminUser,
		map[string]any{"status": JobStatusRunning})
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/model-training/admin/jobs/1/status", 9, common.RoleAdminUser,
		map[string]any{"status": JobStatusSucceeded, "result_model": "ft:gpt-y:abc"})
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/model-training/jobs/1", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	job, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(JobStatusSucceeded), job["status"])
	assert.Equal(t, "ft:gpt-y:abc", job["result_model"])

	// Succeeded jobs can no longer be canceled or deleted.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/model-training/jobs/1/cancel", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodDelete, "/api/model-training/jobs/1", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)
}

func TestFinetuneJobAdminGatingAndCancel(t *testing.T) {
	r := setupRouter(t)
	createDraftJob(t, r, 1)

	// Non-admin users cannot reach the admin endpoints.
	w, _, err := testauth.DoJSON(r, http.MethodGet, "/api/model-training/admin/jobs", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// A submitted job can be canceled by its owner.
	_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/model-training/jobs/1/submit", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/model-training/jobs/1/cancel", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/model-training/jobs/1", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	job, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(JobStatusCanceled), job["status"])

	// The admin sees every job in the admin listing.
	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/model-training/admin/jobs", 9, common.RoleAdminUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
}
