package datasetmarket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func setupDemoMarketRouter(t *testing.T) *gin.Engine {
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

func decodeData(t *testing.T, resp map[string]any) any {
	t.Helper()
	require.Equal(t, true, resp["success"], "unexpected failure: %v", resp["message"])
	return resp["data"]
}

func TestDemoMarketSeedsThreeDimensions(t *testing.T) {
	r := setupDemoMarketRouter(t)

	// The bundled catalogue covers three distinct capability dimensions.
	_, resp, err := testauth.DoJSON(r, http.MethodGet, "/api/data-market/admin/demo-datasets", 9, common.RoleAdminUser, nil)
	require.NoError(t, err)
	catalogue, ok := decodeData(t, resp).([]any)
	require.True(t, ok)
	require.Len(t, catalogue, 3)

	categories := map[string]bool{}
	for _, item := range catalogue {
		entry := item.(map[string]any)
		assert.Equal(t, false, entry["exists"], "nothing is published yet")
		categories[entry["category"].(string)] = true
		assert.NotZero(t, entry["rows"])
	}
	assert.Equal(t, map[string]bool{"domain-knowledge": true, "code": true, "reasoning": true}, categories,
		"each listing must cover a different dimension")

	// Seeding publishes all three.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/data-market/admin/demo-datasets", 9, common.RoleAdminUser, nil)
	require.NoError(t, err)
	result := decodeData(t, resp).(map[string]any)
	assert.Len(t, result["created"].([]any), 3)
	assert.Empty(t, result["existing"].([]any))

	// They are immediately visible in the public browse list.
	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/data-market/datasets", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	page := decodeData(t, resp).(map[string]any)
	assert.Equal(t, float64(3), page["total"])

	var published []Dataset
	require.NoError(t, model.DB.Where("status = ?", shared.PublishStatusPublished).Find(&published).Error)
	require.Len(t, published, 3)
	for _, dataset := range published {
		assert.Equal(t, "jsonl", dataset.Format)
		assert.Equal(t, shared.PriceModeFree, dataset.PriceMode)
		assert.NotEmpty(t, dataset.Description)
		assert.Contains(t, dataset.FileUrl, "/api/data-market/demo-datasets/",
			"the listing must point at the downloadable file")
		assert.NotContains(t, dataset.FileUrl, " ", "the file URL must be usable as-is")
	}

	// Re-running is idempotent and never duplicates a listing.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, "/api/data-market/admin/demo-datasets", 9, common.RoleAdminUser, nil)
	require.NoError(t, err)
	second := decodeData(t, resp).(map[string]any)
	assert.Empty(t, second["created"].([]any))
	assert.Len(t, second["existing"].([]any), 3)

	var count int64
	require.NoError(t, model.DB.Model(&Dataset{}).Count(&count).Error)
	assert.Equal(t, int64(3), count)
}

func TestDemoMarketFileDownloadIsKeyedAndPublic(t *testing.T) {
	r := setupDemoMarketRouter(t)

	// The file endpoint is reachable without a session: the training service
	// downloads listings by URL.
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/data-market/demo-datasets/math-reasoning/file", nil)
	r.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	lines := strings.Split(strings.TrimSpace(recorder.Body.String()), "\n")
	assert.Len(t, lines, 24, "the served file must be the full bundled dataset")

	var row map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &row))
	assert.Contains(t, row, "instruction")
	assert.Contains(t, row, "output")

	// An unknown key is refused, so no path can escape the embedded manifest.
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/data-market/demo-datasets/../../etc/passwd/file", nil)
	r.ServeHTTP(recorder, request)
	assert.NotEqual(t, http.StatusOK, recorder.Code)
}

func TestDemoMarketSeedingRequiresAdmin(t *testing.T) {
	r := setupDemoMarketRouter(t)

	recorder, _, err := testauth.DoJSON(r, http.MethodPost, "/api/data-market/admin/demo-datasets", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, recorder.Code)

	var count int64
	require.NoError(t, model.DB.Model(&Dataset{}).Count(&count).Error)
	assert.Zero(t, count, "a non-admin must not create listings")
}

func TestDatasetPreviewAndAccessKey(t *testing.T) {
	r := setupDemoMarketRouter(t)

	_, resp, err := testauth.DoJSON(r, http.MethodPost, "/api/data-market/admin/demo-datasets", 9, common.RoleAdminUser, nil)
	require.NoError(t, err)
	require.Equal(t, true, resp["success"], "%v", resp["message"])

	var dataset Dataset
	require.NoError(t, model.DB.Where("category = ?", "reasoning").First(&dataset).Error)

	// 1) Preview exposes the real rows, capped and flagged as truncated.
	_, resp, err = testauth.DoJSON(r, http.MethodGet, fmt.Sprintf("/api/data-market/datasets/%d/preview", dataset.Id), 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	preview := decodeData(t, resp).(map[string]any)
	assert.Equal(t, true, preview["available"])
	assert.Equal(t, float64(24), preview["total_rows"])
	assert.Equal(t, true, preview["truncated"])
	assert.NotEmpty(t, preview["columns"])

	rows := preview["rows"].([]any)
	require.Len(t, rows, demoPreviewRowLimit)
	first := rows[0].(map[string]any)
	assert.Contains(t, first, "instruction")
	assert.Contains(t, first, "output")

	// A draft dataset is invisible to other users.
	draft := Dataset{UserId: 9, Name: "draft only", Status: shared.PublishStatusDraft}
	require.NoError(t, model.DB.Create(&draft).Error)
	recorder, _, err := testauth.DoJSON(r, http.MethodGet, fmt.Sprintf("/api/data-market/datasets/%d/preview", draft.Id), 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, recorder.Code)
	var denied map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &denied))
	assert.Equal(t, false, denied["success"])

	// 2) Applying issues an access key, and the grant list exposes its URL.
	_, resp, err = testauth.DoJSON(r, http.MethodPost, fmt.Sprintf("/api/data-market/datasets/%d/apply", dataset.Id), 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	grant := decodeData(t, resp).(map[string]any)
	accessKey, ok := grant["access_key"].(string)
	require.True(t, ok)
	require.NotEmpty(t, accessKey)

	_, resp, err = testauth.DoJSON(r, http.MethodGet, "/api/data-market/grants/my", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	page := decodeData(t, resp).(map[string]any)
	items := page["items"].([]any)
	require.Len(t, items, 1)
	listed := items[0].(map[string]any)
	assert.Equal(t, accessKey, listed["access_key"])
	assert.Contains(t, listed["access_url"], "/api/data-market/access/"+accessKey+"/file")

	// 3) The key alone downloads the data, without a console session.
	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/data-market/access/"+accessKey+"/file", nil)
	r.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Len(t, strings.Split(strings.TrimSpace(recorder.Body.String()), "\n"), 24)

	// An unknown key is refused.
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/data-market/access/unknown-key-xxxxxxxx/file", nil)
	r.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusNotFound, recorder.Code)

	// 4) An unapproved key is refused.
	require.NoError(t, model.DB.Model(&DatasetGrant{}).Where("access_key = ?", accessKey).Update("status", shared.ReviewStatusPending).Error)
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/data-market/access/"+accessKey+"/file", nil)
	r.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusForbidden, recorder.Code)

	// 5) An expired key is refused.
	require.NoError(t, model.DB.Model(&DatasetGrant{}).Where("access_key = ?", accessKey).Updates(map[string]any{
		"status":     shared.ReviewStatusApproved,
		"expires_at": time.Now().Add(-time.Hour).Unix(),
	}).Error)
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/data-market/access/"+accessKey+"/file", nil)
	r.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "expired")
}
