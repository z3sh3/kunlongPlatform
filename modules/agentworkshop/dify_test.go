package agentworkshop

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/modules/shared/testauth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDify stands in for the Dify console API. It records what the client sent
// so the assertions stay on the test goroutine.
type fakeDify struct {
	loginCount int
	appsCount  int
	rejectNext bool

	// appsPayload answers mode=all (and any classic mode) requests;
	// agentAppsPayload answers mode=agent requests.
	appsPayload      string
	agentAppsPayload string

	// detailPayload answers GET /console/api/apps/<id>; detailMissing makes
	// every detail request answer 404.
	detailPayload string
	detailMissing bool
	detailCount   int

	lastAuthHeader  string
	lastCSRFHeader  string
	lastCSRFCookie  string
	lastPasswordRaw string
	queries         []string
}

func (f *fakeDify) start(t *testing.T) string {
	t.Helper()
	if f.agentAppsPayload == "" {
		f.agentAppsPayload = `{"page":1,"limit":1,"total":0,"data":[]}`
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/console/api/login", func(w http.ResponseWriter, r *http.Request) {
		f.loginCount++
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		_ = common.DecodeJson(r.Body, &body)
		f.lastPasswordRaw = body.Password
		http.SetCookie(w, &http.Cookie{Name: "access_token", Value: "access-1"})
		http.SetCookie(w, &http.Cookie{Name: "csrf_token", Value: "csrf-1"})
		_, _ = w.Write([]byte(`{"result":"success"}`))
	})
	mux.HandleFunc("/console/api/apps", func(w http.ResponseWriter, r *http.Request) {
		f.appsCount++
		if f.rejectNext && f.appsCount == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.lastAuthHeader = r.Header.Get("Authorization")
		f.lastCSRFHeader = r.Header.Get("X-CSRF-Token")
		if cookie, err := r.Cookie("csrf_token"); err == nil {
			f.lastCSRFCookie = cookie.Value
		}
		f.queries = append(f.queries, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		payload := f.appsPayload
		if r.URL.Query().Get("mode") == "agent" {
			payload = f.agentAppsPayload
		}
		_, _ = w.Write([]byte(payload))
	})
	// /console/api/setup is the unauthenticated console endpoint the
	// connection test probes to verify the console address.
	mux.HandleFunc("/console/api/setup", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"step":"finished","setup_at":"2024-01-01T00:00:00Z"}`))
	})
	mux.HandleFunc("/console/api/apps/", func(w http.ResponseWriter, r *http.Request) {
		f.detailCount++
		if f.detailMissing {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		f.lastAuthHeader = r.Header.Get("Authorization")
		f.lastCSRFHeader = r.Header.Get("X-CSRF-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(f.detailPayload))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func setDifyOptions(t *testing.T, values map[string]string) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	previous := make(map[string]string, len(values))
	for key, value := range values {
		previous[key] = common.OptionMap[key]
		common.OptionMap[key] = value
	}
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		for key, value := range previous {
			if value == "" {
				delete(common.OptionMap, key)
				continue
			}
			common.OptionMap[key] = value
		}
		common.OptionMapRWMutex.Unlock()
	})
}

func difyTestOptions(baseURL, consoleURL string) map[string]string {
	return map[string]string{
		OptionDifyAPIBaseURL:    baseURL,
		OptionDifyConsoleURL:    consoleURL,
		OptionDifyAccountEmail:  "bot@example.com",
		OptionDifyAccountSecret: "s3cret",
	}
}

func TestListDifyAgents(t *testing.T) {
	dify := &fakeDify{appsPayload: `{"page":1,"limit":10,"total":2,"has_more":false,"data":[
		{"id":"app-chat","name":"Support bot","description":"Answers questions","mode":"chat",
		 "icon_type":"emoji","icon":"robot","icon_background":"#FFEAD5",
		 "created_at":1700000000,"updated_at":1700003600},
		{"id":"app-flow","name":"Intake flow","mode":"advanced-chat","bound_agent_id":"agent-9"}
	]}`}
	setDifyOptions(t, difyTestOptions(dify.start(t), "http://console.example:8081/"))

	r := setupRouter(t)
	_, resp, err := testauth.DoJSON(r, http.MethodGet,
		"/api/agent-workshop/dify/agents?p=1&page_size=10&keyword=support&mode=chat",
		1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)

	// The password travels base64 encoded and the CSRF token is sent twice.
	decoded, err := base64.StdEncoding.DecodeString(dify.lastPasswordRaw)
	require.NoError(t, err)
	assert.Equal(t, "s3cret", string(decoded))
	assert.Equal(t, "Bearer access-1", dify.lastAuthHeader)
	assert.Equal(t, "csrf-1", dify.lastCSRFHeader)
	assert.Equal(t, "csrf-1", dify.lastCSRFCookie)
	// The chat filter goes straight to that one Dify source.
	require.Len(t, dify.queries, 1)
	assert.Contains(t, dify.queries[0], "name=support")
	assert.Contains(t, dify.queries[0], "mode=chat")

	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "expected paginated data envelope: %v", resp)
	assert.Equal(t, float64(2), data["total"])
	assert.Equal(t, "http://console.example:8081", data["console_base_url"])

	items := listItems(t, resp)
	require.Len(t, items, 2)
	assert.Equal(t, "Support bot", items[0]["name"])
	assert.Equal(t, "Answers questions", items[0]["description"])
	assert.Equal(t, "/app/app-chat/configuration", items[0]["console_path"])
	assert.Equal(t, "2023-11-14T22:13:20Z", items[0]["created_at"])
	assert.Equal(t, "/app/app-flow/workflow", items[1]["console_path"])

	// A second request reuses the cached console session, and the unfiltered
	// listing queries both Dify sources: classic apps and the Agent product.
	_, resp, err = testauth.DoJSON(r, http.MethodGet,
		"/api/agent-workshop/dify/agents?mine=1", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	assert.Equal(t, 1, dify.loginCount)
	require.Len(t, dify.queries, 3)
	assert.Contains(t, dify.queries[1], "mode=all")
	assert.Contains(t, dify.queries[2], "mode=agent")
	assert.Contains(t, dify.queries[2], "is_created_by_me=true")
}

func TestListDifyAgentsMergesAgentProduct(t *testing.T) {
	dify := &fakeDify{
		appsPayload: `{"page":1,"limit":10,"total":2,"has_more":false,"data":[
			{"id":"app-chat","name":"Support bot","mode":"chat","updated_at":1700003600},
			{"id":"app-flow","name":"Intake flow","mode":"advanced-chat"}
		]}`,
		agentAppsPayload: `{"page":1,"limit":10,"total":1,"has_more":false,"data":[
			{"id":"app-agent","name":"Roster helper","mode":"agent","bound_agent_id":"agent-9","updated_at":1700001000}
		]}`,
	}
	setDifyOptions(t, difyTestOptions(dify.start(t), ""))

	r := setupRouter(t)
	_, resp, err := testauth.DoJSON(r, http.MethodGet,
		"/api/agent-workshop/dify/agents?p=1&page_size=2", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)

	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "expected paginated data envelope: %v", resp)
	// The total counts both sources even when the page slice does not.
	assert.Equal(t, float64(3), data["total"])

	items := listItems(t, resp)
	require.Len(t, items, 2)
	// Merged order is last modification first; the agent deep link points
	// at the roster configure page of its bound agent.
	assert.Equal(t, "Support bot", items[0]["name"])
	assert.Equal(t, "Roster helper", items[1]["name"])
	assert.Equal(t, "/agents/agent-9/configure", items[1]["console_path"])

	_, resp, err = testauth.DoJSON(r, http.MethodGet,
		"/api/agent-workshop/dify/agents?p=2&page_size=2", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	items = listItems(t, resp)
	require.Len(t, items, 1)
	assert.Equal(t, "Intake flow", items[0]["name"])
}

func TestListDifyAgentsRelogin(t *testing.T) {
	dify := &fakeDify{
		rejectNext:  true,
		appsPayload: `{"page":1,"limit":10,"total":0,"has_more":false,"data":[]}`,
	}
	setDifyOptions(t, difyTestOptions(dify.start(t), ""))

	r := setupRouter(t)
	_, resp, err := testauth.DoJSON(r, http.MethodGet,
		"/api/agent-workshop/dify/agents", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)

	// The rejected classic-source call triggers one new login and a fresh
	// crawl; the agent source then reuses the refreshed session.
	assert.Equal(t, 2, dify.loginCount)
	assert.Equal(t, 3, dify.appsCount)
	assert.Empty(t, listItems(t, resp))
}

func TestListDifyAgentsNotConfigured(t *testing.T) {
	setDifyOptions(t, difyTestOptions("", ""))

	r := setupRouter(t)
	_, resp, err := testauth.DoJSON(r, http.MethodGet,
		"/api/agent-workshop/dify/agents", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)
	assert.Equal(t, difyCodeNotConfigured, resp["code"])
}

func TestListDifyAgentsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	baseURL := srv.URL
	srv.Close()
	setDifyOptions(t, difyTestOptions(baseURL, ""))

	r := setupRouter(t)
	_, resp, err := testauth.DoJSON(r, http.MethodGet,
		"/api/agent-workshop/dify/agents", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)
	assert.Equal(t, difyCodeUnreachable, resp["code"])
}

func TestListDifyAgentsRejectsUnsupportedMode(t *testing.T) {
	dify := &fakeDify{appsPayload: `{"total":0,"data":[]}`}
	setDifyOptions(t, difyTestOptions(dify.start(t), ""))

	r := setupRouter(t)
	_, resp, err := testauth.DoJSON(r, http.MethodGet,
		"/api/agent-workshop/dify/agents?mode=rag-pipeline", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)
	assert.Equal(t, 0, dify.appsCount)
}

func TestCheckDifyConnection(t *testing.T) {
	dify := &fakeDify{appsPayload: `{"page":1,"limit":1,"total":7,"has_more":true,"data":[]}`}
	// The console address is probed by the test, so it must be the fake
	// server, which serves /console/api/setup.
	url := dify.start(t)
	setDifyOptions(t, difyTestOptions(url, url))

	r := setupRouter(t)

	w, _, err := testauth.DoJSON(r, http.MethodPost,
		"/api/agent-workshop/admin/dify/test", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, w.Code)

	_, resp, err := testauth.DoJSON(r, http.MethodPost,
		"/api/agent-workshop/admin/dify/test", 9, common.RoleAdminUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	// The count covers both sources: classic apps plus the Agent product.
	assert.Equal(t, 2, dify.appsCount)
	assert.Contains(t, dify.queries[0], "mode=all")
	assert.Contains(t, dify.queries[1], "mode=agent")
	assert.Equal(t, float64(7), data["total"])
	assert.Equal(t, "bot@example.com", data["email"])
	assert.Equal(t, true, data["console_url_ok"])
}

func TestCheckDifyConnectionReportsDeadConsoleURL(t *testing.T) {
	// A console address that does not serve the Dify console API (here: a
	// plain 404 server, like the new-api gateway) must be reported so the
	// admin fixes the setting before the workshop shows broken links.
	dify := &fakeDify{appsPayload: `{"page":1,"limit":1,"total":7,"data":[]}`}
	notDify := httptest.NewServer(http.NotFoundHandler())
	defer notDify.Close()
	setDifyOptions(t, difyTestOptions(dify.start(t), notDify.URL))

	r := setupRouter(t)
	_, resp, err := testauth.DoJSON(r, http.MethodPost,
		"/api/agent-workshop/admin/dify/test", 9, common.RoleAdminUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, data["console_url_ok"])
}

func TestCheckDifyConnectionUsesOverridesWhenNothingSaved(t *testing.T) {
	dify := &fakeDify{appsPayload: `{"page":1,"limit":1,"total":3,"data":[]}`}
	srvURL := dify.start(t)
	// Nothing is saved, so the unsaved form values must carry the whole config.
	setDifyOptions(t, difyTestOptions("", ""))

	r := setupRouter(t)
	_, resp, err := testauth.DoJSON(r, http.MethodPost,
		"/api/agent-workshop/admin/dify/test", 9, common.RoleAdminUser,
		map[string]any{
			"base_url": srvURL,
			"email":    "typed@example.com",
			"password": "typed-pass",
		})
	require.NoError(t, err)
	requireSuccess(t, resp)

	decoded, err := base64.StdEncoding.DecodeString(dify.lastPasswordRaw)
	require.NoError(t, err)
	assert.Equal(t, "typed-pass", string(decoded))
	assert.Equal(t, "typed@example.com", resp["data"].(map[string]any)["email"])

	// A partial override against an empty saved config still reads as not
	// configured instead of half-connecting.
	_, resp, err = testauth.DoJSON(r, http.MethodPost,
		"/api/agent-workshop/admin/dify/test", 9, common.RoleAdminUser,
		map[string]any{"base_url": srvURL})
	require.NoError(t, err)
	requireFailure(t, resp)
	assert.Equal(t, difyCodeNotConfigured, resp["code"])
}

func TestCheckDifyConnectionKeepsSavedValuesForBlankOverrides(t *testing.T) {
	dify := &fakeDify{appsPayload: `{"page":1,"limit":1,"total":1,"data":[]}`}
	setDifyOptions(t, difyTestOptions(dify.start(t), ""))

	r := setupRouter(t)
	// The settings page sends the whole form; a blank password falls back to
	// the saved one while the typed password wins when provided.
	_, resp, err := testauth.DoJSON(r, http.MethodPost,
		"/api/agent-workshop/admin/dify/test", 9, common.RoleAdminUser,
		map[string]any{"base_url": "", "email": "", "password": "retyped-pass"})
	require.NoError(t, err)
	requireSuccess(t, resp)
	assert.Equal(t, "bot@example.com", resp["data"].(map[string]any)["email"])

	decoded, err := base64.StdEncoding.DecodeString(dify.lastPasswordRaw)
	require.NoError(t, err)
	assert.Equal(t, "retyped-pass", string(decoded))
}

func TestGetDifyAgentAccess(t *testing.T) {
	dify := &fakeDify{
		appsPayload: `{"total":0,"data":[]}`,
		detailPayload: `{"id":"app-agent","name":"Roster helper","mode":"agent","bound_agent_id":"agent-9",
		 "enable_site":true,"enable_api":true,"api_base_url":"",
		 "site":{"code":"tok123"}}`,
	}
	setDifyOptions(t, difyTestOptions(dify.start(t), "http://console.example:8081"))

	r := setupRouter(t)
	_, resp, err := testauth.DoJSON(r, http.MethodGet,
		"/api/agent-workshop/dify/agents/app-agent/access", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)

	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "expected access data: %v", resp)
	// Agent-product apps share at /agent/<site code> next to the console.
	assert.Equal(t, "http://console.example:8081/agent/tok123", data["webapp_url"])
	// An empty APP_API_URL falls back to the console origin's /v1.
	assert.Equal(t, "http://console.example:8081/v1", data["api_base_url"])
	assert.Equal(t, "/agents/agent-9/configure", data["console_path"])
	assert.Equal(t, "Roster helper", data["name"])
	// The detail request carries the full console session.
	assert.Equal(t, "Bearer access-1", dify.lastAuthHeader)
	assert.Equal(t, "csrf-1", dify.lastCSRFHeader)
}

func TestGetDifyAgentAccessDisabledSurfaces(t *testing.T) {
	dify := &fakeDify{
		appsPayload: `{"total":0,"data":[]}`,
		detailPayload: `{"id":"app-chat","name":"Support bot","mode":"chat",
		 "enable_site":false,"enable_api":true,"api_base_url":"http://api.example/v1",
		 "site":{"code":"abc"}}`,
	}
	setDifyOptions(t, difyTestOptions(dify.start(t), "http://console.example:8081"))

	r := setupRouter(t)
	_, resp, err := testauth.DoJSON(r, http.MethodGet,
		"/api/agent-workshop/dify/agents/app-chat/access", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireSuccess(t, resp)

	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "expected access data: %v", resp)
	// Site access is off, so no web app URL is offered even with a code.
	assert.Empty(t, data["webapp_url"])
	// The configured console origin wins over the api_base_url Dify derives
	// from the request Host header (which loses the port behind the proxy).
	assert.Equal(t, "http://console.example:8081/v1", data["api_base_url"])
	assert.Equal(t, "/app/app-chat/configuration", data["console_path"])
}

func TestGetDifyAgentAccessNotFound(t *testing.T) {
	dify := &fakeDify{appsPayload: `{"total":0,"data":[]}`, detailMissing: true}
	setDifyOptions(t, difyTestOptions(dify.start(t), ""))

	r := setupRouter(t)
	_, resp, err := testauth.DoJSON(r, http.MethodGet,
		"/api/agent-workshop/dify/agents/app-gone/access", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)
	assert.Equal(t, difyCodeNotFound, resp["code"])
}

func TestGetDifyAgentAccessRejectsBadID(t *testing.T) {
	dify := &fakeDify{appsPayload: `{"total":0,"data":[]}`}
	setDifyOptions(t, difyTestOptions(dify.start(t), ""))

	r := setupRouter(t)
	_, resp, err := testauth.DoJSON(r, http.MethodGet,
		"/api/agent-workshop/dify/agents/bad_id/access", 1, common.RoleCommonUser, nil)
	require.NoError(t, err)
	requireFailure(t, resp)
	// The id never reaches Dify.
	assert.Equal(t, 0, dify.detailCount)
}
