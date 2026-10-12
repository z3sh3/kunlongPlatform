package enterprise

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
	require.NoError(t, db.AutoMigrate(&IncubationProgram{}, &IncubationApplication{}))
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

// publishProgram creates a program as admin and publishes it.
func publishProgram(t *testing.T, r *gin.Engine, title, programType string) {
	t.Helper()
	_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/enterprise-incubation/admin/programs", 9, common.RoleAdminUser,
		map[string]any{"title": title, "type": programType})
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/enterprise-incubation/admin/programs/1/status", 9, common.RoleAdminUser,
		map[string]any{"status": shared.PublishStatusPublished})
	require.NoError(t, err)
	requireSuccess(t, resp)
}

func TestIncubationApplicationFlow(t *testing.T) {
	r := setupRouter(t)
	publishProgram(t, r, "AI startup camp", TypeIncubation)

	// Invalid program types are rejected.
	_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/enterprise-incubation/admin/programs", 9, common.RoleAdminUser,
		map[string]any{"title": "bad", "type": "yoga"})
	require.NoError(t, err)
	requireFailure(t, resp)

	// A user applies to the published program.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/enterprise-incubation/applications", 2, common.RoleCommonUser,
		map[string]any{
			"program_id":    1,
			"company_name":  "ACME",
			"contact_name":  "Wang",
			"contact_phone": "13800000000",
			"description":   "AI startup",
		})
	require.NoError(t, err)
	requireSuccess(t, resp)

	// Applications to unpublished programs are rejected.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/enterprise-incubation/admin/programs", 9, common.RoleAdminUser,
		map[string]any{"title": "draft program", "type": TypeSMESupport})
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/enterprise-incubation/applications", 2, common.RoleCommonUser,
		map[string]any{
			"program_id":    2,
			"company_name":  "ACME",
			"contact_name":  "Wang",
			"contact_phone": "13800000000",
			"description":   "AI startup",
		})
	require.NoError(t, err)
	requireFailure(t, resp)

	// The admin moves the application through the review flow.
	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/enterprise-incubation/admin/applications/1/review", 9, common.RoleAdminUser,
		map[string]any{"status": shared.ReviewStatusReviewing})
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/enterprise-incubation/admin/applications/1/review", 9, common.RoleAdminUser,
		map[string]any{"status": shared.ReviewStatusApproved, "comment": "welcome"})
	require.NoError(t, err)
	requireSuccess(t, resp)

	// Closed applications cannot be re-reviewed.
	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/enterprise-incubation/admin/applications/1/review", 9, common.RoleAdminUser,
		map[string]any{"status": shared.ReviewStatusRejected})
	require.NoError(t, err)
	requireFailure(t, resp)

	// The user's application list preloads the program.
	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/enterprise-incubation/applications/my", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	items, _ := data["items"].([]any)
	require.Len(t, items, 1)
	application, _ := items[0].(map[string]any)
	program, hasProgram := application["program"].(map[string]any)
	require.True(t, hasProgram, "application should preload program info")
	assert.Equal(t, "AI startup camp", program["title"])
}

func TestIncubationAdminGating(t *testing.T) {
	r := setupRouter(t)

	// Non-admin users cannot manage programs.
	w, _, err := testauth.DoJSON(r, http.MethodPost, "/api/enterprise-incubation/admin/programs", 1, common.RoleCommonUser,
		map[string]any{"title": "x", "type": TypeGovPark})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Unpublished programs are invisible in the public listing.
	_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/enterprise-incubation/admin/programs", 9, common.RoleAdminUser,
		map[string]any{"title": "draft program", "type": TypeGovPark})
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/enterprise-incubation/programs", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	items, _ := data["items"].([]any)
	assert.Empty(t, items)
}
