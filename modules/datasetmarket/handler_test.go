package datasetmarket

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
	require.NoError(t, db.AutoMigrate(&Dataset{}, &DatasetGrant{}))
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

func listItems(t *testing.T, resp map[string]any) []map[string]any {
	t.Helper()
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "expected paginated data envelope: %v", resp)
	items, _ := data["items"].([]any)
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		require.True(t, ok)
		result = append(result, m)
	}
	return result
}

func TestDatasetPublishFlow(t *testing.T) {
	r := setupRouter(t)

	// A user submits a draft dataset.
	_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/data-market/datasets", 1, common.RoleCommonUser,
		map[string]any{"name": "open images", "category": "cv", "price_mode": shared.PriceModeFree})
	require.NoError(t, err)
	requireSuccess(t, resp)

	// Drafts are invisible in the public listing and for other users.
	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/data-market/datasets", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	assert.Empty(t, listItems(t, resp))

	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/data-market/datasets/1", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)

	// Non-admin users cannot reach the admin publish endpoint.
	w, _, err := testauth.DoJSON(r, http.MethodPut, "/api/data-market/admin/datasets/1/status", 2, common.RoleCommonUser,
		map[string]any{"status": shared.PublishStatusPublished})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// An admin publishes the dataset and it becomes publicly visible.
	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/data-market/admin/datasets/1/status", 9, common.RoleAdminUser,
		map[string]any{"status": shared.PublishStatusPublished})
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/data-market/datasets", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	require.Len(t, listItems(t, resp), 1)

	// The owner can no longer edit the published dataset.
	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/data-market/datasets/1", 1, common.RoleCommonUser,
		map[string]any{"name": "renamed", "price_mode": shared.PriceModeFree})
	require.NoError(t, err)
	requireFailure(t, resp)

	// Paid datasets must declare a positive unit price.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/data-market/datasets", 1, common.RoleCommonUser,
		map[string]any{"name": "paid set", "price_mode": shared.PriceModePaid, "unit_price": 0})
	require.NoError(t, err)
	requireFailure(t, resp)
}

func TestDatasetGrantFlow(t *testing.T) {
	r := setupRouter(t)

	// User 1 publishes a free dataset and a paid dataset.
	for i, priceMode := range []int{shared.PriceModeFree, shared.PriceModePaid} {
		_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/data-market/datasets", 1, common.RoleCommonUser,
			map[string]any{"name": "dataset-" + string(rune('a'+i)), "price_mode": priceMode, "unit_price": 100})
		require.NoError(t, err)
		requireSuccess(t, resp)
	}
	for i := 1; i <= 2; i++ {
		_, resp, err := testauth.DoJSON(r, http.MethodPut, "/api/data-market/admin/datasets/"+string(rune('0'+i))+"/status",
			9, common.RoleAdminUser, map[string]any{"status": shared.PublishStatusPublished})
		require.NoError(t, err)
		requireSuccess(t, resp)
	}

	// Applying for a free dataset grants access immediately.
	_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/data-market/datasets/1/apply", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	grant, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(shared.ReviewStatusApproved), grant["status"])

	// Applying for a paid dataset stays pending for admin review.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/data-market/datasets/2/apply", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	grant, ok = resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(shared.ReviewStatusPending), grant["status"])

	// Duplicate applications are rejected.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/data-market/datasets/2/apply", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)

	// The admin approves the pending grant; re-review is rejected.
	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/data-market/admin/grants/2/review", 9, common.RoleAdminUser,
		map[string]any{"status": shared.ReviewStatusApproved, "expires_at": 4102444800})
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/data-market/admin/grants/2/review", 9, common.RoleAdminUser,
		map[string]any{"status": shared.ReviewStatusRejected})
	require.NoError(t, err)
	requireFailure(t, resp)

	// The user sees both grants with dataset info preloaded.
	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/data-market/grants/my", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	items := listItems(t, resp)
	require.Len(t, items, 2)
	_, hasDataset := items[0]["dataset"].(map[string]any)
	assert.True(t, hasDataset, "grant should preload dataset info")

	// The admin can list every grant for review; non-admins cannot.
	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/data-market/admin/grants", 9, common.RoleAdminUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	assert.Len(t, listItems(t, resp), 2)

	w, _, err := testauth.DoJSON(r, http.MethodGet, "/api/data-market/admin/grants", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, w.Code)
}
