package agentworkshop

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
	require.NoError(t, db.AutoMigrate(&Agent{}))
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

func TestAgentReviewFlow(t *testing.T) {
	r := setupRouter(t)

	// The author creates a draft agent.
	_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/agent-workshop/agents", 1, common.RoleCommonUser,
		map[string]any{"name": "helper", "category": "assistant", "price_mode": 0})
	require.NoError(t, err)
	requireSuccess(t, resp)

	// Another user cannot edit it.
	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/agent-workshop/agents/1", 2, common.RoleCommonUser,
		map[string]any{"name": "hijack", "price_mode": 0})
	require.NoError(t, err)
	requireFailure(t, resp)

	// Submitting puts the agent under review.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/agent-workshop/agents/1/submit", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)

	// The admin rejects it with a comment; the agent returns to draft.
	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/agent-workshop/admin/agents/1/review", 9, common.RoleAdminUser,
		map[string]any{"approve": false, "comment": "needs work"})
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/agent-workshop/agents/1", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	agent, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(StatusDraft), agent["status"])
	assert.Equal(t, "needs work", agent["review_comment"])

	// Re-submit and approve: the agent appears in the public listing.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/agent-workshop/agents/1/submit", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/agent-workshop/admin/agents/1/review", 9, common.RoleAdminUser,
		map[string]any{"approve": true})
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/agent-workshop/agents", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	require.Len(t, listItems(t, resp), 1)

	// The owner can no longer edit a published agent.
	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/agent-workshop/agents/1", 1, common.RoleCommonUser,
		map[string]any{"name": "renamed", "price_mode": 0})
	require.NoError(t, err)
	requireFailure(t, resp)

	// The admin takes it offline, then restores it.
	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/agent-workshop/admin/agents/1/offline", 9, common.RoleAdminUser,
		map[string]any{"offline": true})
	require.NoError(t, err)
	requireSuccess(t, resp)

	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/agent-workshop/agents", 2, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	assert.Empty(t, listItems(t, resp))

	_, resp, err = testauth.DoJSON(r, http.MethodPut, "/api/agent-workshop/admin/agents/1/offline", 9, common.RoleAdminUser,
		map[string]any{"offline": false})
	require.NoError(t, err)
	requireSuccess(t, resp)
}

func TestAgentAdminGating(t *testing.T) {
	r := setupRouter(t)

	_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/agent-workshop/agents", 1, common.RoleCommonUser,
		map[string]any{"name": "helper", "price_mode": 0})
	require.NoError(t, err)
	requireSuccess(t, resp)

	w, _, err := testauth.DoJSON(r, http.MethodGet, "/api/agent-workshop/admin/agents", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Paid agents require a positive price.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/agent-workshop/agents", 1, common.RoleCommonUser,
		map[string]any{"name": "paid", "price_mode": 1, "unit_price": 0})
	require.NoError(t, err)
	requireFailure(t, resp)
}
