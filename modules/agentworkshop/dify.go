package agentworkshop

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// Option keys of the Dify (agent development platform) integration. They live
// in the shared option map, so they can be edited from the console settings
// page without a restart. The password key ends with "Secret" on purpose:
// controller.GetOptions filters such keys out, which keeps it write-only.
const (
	OptionDifyAPIBaseURL    = "DifyAPIBaseURL"
	OptionDifyConsoleURL    = "DifyConsoleURL"
	OptionDifyAccountEmail  = "DifyAccountEmail"
	OptionDifyAccountSecret = "DifyAccountSecret"
)

const (
	difyAppModeFilters = "completion,chat,advanced-chat,workflow,agent-chat,agent,channel"

	difyMaxPageSize     = 100
	difySessionTTL      = 50 * time.Minute
	difyRequestTimeout  = 10 * time.Second
	difySessionCacheKey = "new-api:dify:console:session:"
	// difyMergePageCap bounds how many pages the merged "all types"
	// listing pulls from each Dify source, so one console request cannot
	// fan out into an unbounded crawl of a huge workspace.
	difyMergePageCap = 5
)

// Error codes returned to the console so the UI can tell "not connected" from
// "connected but failing".
const (
	difyCodeNotConfigured = "dify_not_configured"
	difyCodeUnreachable   = "dify_unreachable"
	difyCodeAuthFailed    = "dify_auth_failed"
	difyCodeNotFound      = "dify_not_found"
)

// difyError carries a stable code plus the English console message key.
type difyError struct {
	code string
	key  string
	err  error
}

func (e *difyError) Error() string { return e.key }

func (e *difyError) Unwrap() error { return e.err }

func newDifyError(code, key string, err error) *difyError {
	return &difyError{code: code, key: key, err: err}
}

// difyConfig is the connection settings shared by every Dify request.
type difyConfig struct {
	BaseURL    string
	ConsoleURL string
	Email      string
	Password   string
}

// loadDifyConfig reads the integration settings from the option map.
func loadDifyConfig() (difyConfig, error) {
	common.OptionMapRWMutex.RLock()
	cfg := difyConfig{
		BaseURL:    strings.TrimRight(strings.TrimSpace(common.OptionMap[OptionDifyAPIBaseURL]), "/"),
		ConsoleURL: strings.TrimRight(strings.TrimSpace(common.OptionMap[OptionDifyConsoleURL]), "/"),
		Email:      strings.TrimSpace(common.OptionMap[OptionDifyAccountEmail]),
		Password:   common.OptionMap[OptionDifyAccountSecret],
	}
	common.OptionMapRWMutex.RUnlock()

	if cfg.BaseURL == "" || cfg.Email == "" || cfg.Password == "" {
		return difyConfig{}, newDifyError(difyCodeNotConfigured,
			"Dify agent service is not configured", nil)
	}
	return cfg, nil
}

// difySession is the console credential pair Dify hands out on login. Dify only
// returns them as cookies, and its CSRF check requires the token as both an
// X-CSRF-Token header and a csrf_token cookie, so both are cached together.
type difySession struct {
	AccessToken string `json:"access_token"`
	CSRFToken   string `json:"csrf_token"`
	ExpiresAt   int64  `json:"expires_at"`
}

func (s difySession) valid(now time.Time) bool {
	return s.AccessToken != "" && s.CSRFToken != "" && s.ExpiresAt > now.Unix()
}

// difyAppQuery mirrors the subset of Dify's app list filters the console uses.
type difyAppQuery struct {
	Page  int
	Limit int
	Mode  string
	Name  string
	Mine  bool
}

func (q difyAppQuery) values() url.Values {
	values := url.Values{}
	values.Set("page", strconv.Itoa(q.Page))
	values.Set("limit", strconv.Itoa(q.Limit))
	values.Set("sort_by", "last_modified")
	if q.Mode != "" && q.Mode != "all" {
		values.Set("mode", q.Mode)
	} else {
		values.Set("mode", "all")
	}
	if q.Name != "" {
		values.Set("name", q.Name)
	}
	if q.Mine {
		values.Set("is_created_by_me", "true")
	}
	return values
}

// difyApp is the subset of Dify's app payload the console displays.
type difyApp struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Mode           string `json:"mode"`
	IconType       string `json:"icon_type"`
	Icon           string `json:"icon"`
	IconBackground string `json:"icon_background"`
	BoundAgentID   string `json:"bound_agent_id"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	ConsolePath    string `json:"console_path"`
}

type difyAppPage struct {
	Total int
	Items []difyApp
}

// difyClient talks to the Dify console API on behalf of the whole console.
type difyClient struct {
	cfg  difyConfig
	http *http.Client
}

// difyHTTPClient is shared by every Dify call.
func difyHTTPClient() *http.Client {
	return &http.Client{
		Timeout: difyRequestTimeout,
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
}

func newDifyClient() (*difyClient, error) {
	cfg, err := loadDifyConfig()
	if err != nil {
		return nil, err
	}
	return &difyClient{cfg: cfg, http: difyHTTPClient()}, nil
}

// newDifyTestClient builds the client used by the settings-page connection
// test. Non-empty override fields replace the saved values, so the test can
// run against unsaved form input; a blank password keeps the saved one.
func newDifyTestClient(baseURL, email, password string) (*difyClient, error) {
	cfg, err := loadDifyConfig()
	if err != nil {
		// Nothing usable is saved, so the overrides must carry the whole
		// connection config for the test to mean anything.
		if baseURL == "" || email == "" || password == "" {
			return nil, err
		}
		cfg = difyConfig{}
	}
	if baseURL != "" {
		cfg.BaseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	}
	if email != "" {
		cfg.Email = strings.TrimSpace(email)
	}
	if password != "" {
		cfg.Password = password
	}
	if cfg.BaseURL == "" || cfg.Email == "" || cfg.Password == "" {
		return nil, newDifyError(difyCodeNotConfigured,
			"Dify agent service is not configured", nil)
	}
	return &difyClient{cfg: cfg, http: difyHTTPClient()}, nil
}

// listApps returns one page of Dify applications. Dify's mode=all list
// deliberately excludes the Agent product (its query builder is
// App.mode != 'agent'), so the unfiltered listing merges the classic app
// list with the agent list; a specific mode goes to that source directly.
func (c *difyClient) listApps(ctx context.Context, q difyAppQuery) (difyAppPage, error) {
	if q.Mode != "" && q.Mode != "all" {
		return c.fetchAppsPage(ctx, q)
	}
	return c.listMergedApps(ctx, q)
}

// fetchAppsPage fetches one Dify-side page.
func (c *difyClient) fetchAppsPage(ctx context.Context, q difyAppQuery) (difyAppPage, error) {
	var page difyAppPage
	err := c.withSession(ctx, func(session difySession) error {
		result, err := c.fetchApps(ctx, q, session)
		if err != nil {
			return err
		}
		page = result
		return nil
	})
	return page, err
}

// withSession runs fn with the cached console session and retries exactly
// once with a fresh login when the session is rejected.
func (c *difyClient) withSession(ctx context.Context, fn func(difySession) error) error {
	session, err := c.session(ctx)
	if err != nil {
		return err
	}
	err = fn(session)
	if err == nil || !isDifyAuthFailure(err) {
		return err
	}
	// The cached token expired or was revoked: log in again and retry once.
	session, err = c.refreshSession(ctx)
	if err != nil {
		return err
	}
	return fn(session)
}

// listMergedApps merges the classic apps (mode=all) and the Agent product
// apps (mode=agent) into one list ordered by last modification, then applies
// the requested page in memory.
func (c *difyClient) listMergedApps(ctx context.Context, q difyAppQuery) (difyAppPage, error) {
	classicQuery := q
	classicQuery.Mode = "all"
	classic, classicTotal, err := c.fetchAllAppPages(ctx, classicQuery)
	if err != nil {
		return difyAppPage{}, err
	}

	agentQuery := q
	agentQuery.Mode = "agent"
	agents, agentTotal, err := c.fetchAllAppPages(ctx, agentQuery)
	if err != nil {
		return difyAppPage{}, err
	}

	items := classic
	items = append(items, agents...)
	// RFC3339 UTC timestamps order chronologically as plain strings; apps
	// without a timestamp sink to the end.
	sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt > items[j].UpdatedAt })

	start := (q.Page - 1) * q.Limit
	if start >= len(items) {
		items = nil
	} else {
		end := min(start+q.Limit, len(items))
		items = items[start:end]
	}
	return difyAppPage{Total: classicTotal + agentTotal, Items: items}, nil
}

// fetchAllAppPages walks every page of one Dify source for the merged
// listing. The total still comes from Dify, so pagination stays correct even
// when the page cap leaves items unfetched.
func (c *difyClient) fetchAllAppPages(ctx context.Context, q difyAppQuery) ([]difyApp, int, error) {
	var items []difyApp
	total := 0
	err := c.withSession(ctx, func(session difySession) error {
		items = nil
		total = 0
		for page := 1; page <= difyMergePageCap; page++ {
			pageQuery := q
			pageQuery.Page = page
			pageQuery.Limit = difyMaxPageSize
			result, err := c.fetchApps(ctx, pageQuery, session)
			if err != nil {
				return err
			}
			total = result.Total
			items = append(items, result.Items...)
			if len(items) >= total || len(result.Items) < difyMaxPageSize {
				break
			}
		}
		if total > len(items) {
			common.SysLog(common.LogText("dify workspace has more apps than the merge page cap; listing is truncated"))
		}
		return nil
	})
	return items, total, err
}

// countApps returns the size of each source with a single small request per
// source; it backs the settings-page connection test.
func (c *difyClient) countApps(ctx context.Context) (classicTotal, agentTotal int, err error) {
	classicPage, err := c.fetchAppsPage(ctx, difyAppQuery{Page: 1, Limit: 1, Mode: "all"})
	if err != nil {
		return 0, 0, err
	}
	agentPage, err := c.fetchAppsPage(ctx, difyAppQuery{Page: 1, Limit: 1, Mode: "agent"})
	if err != nil {
		return 0, 0, err
	}
	return classicPage.Total, agentPage.Total, nil
}

// probeConsoleURL checks that the public console address really serves the
// Dify console API. A wrong port or an origin pointing at another service
// would otherwise surface only as broken links in the workshop. An empty
// console URL is fine: the frontend derives the origin from the Dify port.
func (c *difyClient) probeConsoleURL(ctx context.Context) bool {
	if c.cfg.ConsoleURL == "" {
		return true
	}
	// /console/api/setup is a Dify console endpoint that answers without
	// authentication, so it identifies the console reliably.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.cfg.ConsoleURL+"/console/api/setup", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func isDifyAuthFailure(err error) bool {
	var requestErr *difyError
	return errors.As(err, &requestErr) && requestErr.code == difyCodeAuthFailed
}

// difyAppAccess describes the ways one agent can be reached from outside
// Dify: the published web app, the service API, and the console entry.
type difyAppAccess struct {
	Name        string `json:"name"`
	Mode        string `json:"mode"`
	WebAppURL   string `json:"webapp_url"`
	APIBaseURL  string `json:"api_base_url"`
	ConsolePath string `json:"console_path"`
}

// fetchAppAccess reads the Dify app detail and normalizes its access points.
func (c *difyClient) fetchAppAccess(ctx context.Context, appID string) (difyAppAccess, error) {
	var access difyAppAccess
	err := c.withSession(ctx, func(session difySession) error {
		result, err := c.fetchAppAccessOnce(ctx, appID, session)
		if err != nil {
			return err
		}
		access = result
		return nil
	})
	return access, err
}

func (c *difyClient) fetchAppAccessOnce(ctx context.Context, appID string, session difySession) (difyAppAccess, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.cfg.BaseURL+"/console/api/apps/"+appID, nil)
	if err != nil {
		return difyAppAccess{}, newDifyError(difyCodeUnreachable, "Failed to connect to Dify", err)
	}
	req.Header.Set("Authorization", "Bearer "+session.AccessToken)
	req.Header.Set("X-CSRF-Token", session.CSRFToken)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: session.CSRFToken})

	resp, err := c.http.Do(req)
	if err != nil {
		return difyAppAccess{}, newDifyError(difyCodeUnreachable, "Failed to connect to Dify", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return difyAppAccess{}, newDifyError(difyCodeNotFound, "Agent not found in Dify", nil)
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return difyAppAccess{}, newDifyError(difyCodeAuthFailed,
			"Dify rejected the configured account", nil)
	case resp.StatusCode != http.StatusOK:
		return difyAppAccess{}, newDifyError(difyCodeUnreachable, "Failed to connect to Dify",
			fmt.Errorf("dify app detail returned status %d", resp.StatusCode))
	}

	var payload struct {
		Name         string  `json:"name"`
		Mode         string  `json:"mode"`
		BoundAgentID *string `json:"bound_agent_id"`
		EnableSite   bool    `json:"enable_site"`
		EnableAPI    bool    `json:"enable_api"`
		APIBaseURL   string  `json:"api_base_url"`
		Site         *struct {
			Code *string `json:"code"`
		} `json:"site"`
	}
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return difyAppAccess{}, newDifyError(difyCodeUnreachable, "Failed to connect to Dify", err)
	}

	access := difyAppAccess{
		Name:        payload.Name,
		Mode:        payload.Mode,
		ConsolePath: difyConsolePath(appID, payload.Mode, derefString(payload.BoundAgentID)),
	}
	// The web app link only works once site access is enabled and the app
	// has a share token for its mode.
	if payload.EnableSite && payload.Site != nil && c.cfg.ConsoleURL != "" {
		if code := derefString(payload.Site.Code); code != "" {
			if path := difyWebAppPath(payload.Mode, code); path != "" {
				access.WebAppURL = c.cfg.ConsoleURL + path
			}
		}
	}
	if payload.EnableAPI {
		// Prefer the admin-configured console origin: this deployment's
		// SERVICE_API_URL is empty, so Dify derives api_base_url from the
		// request Host header, which loses the port behind the nginx proxy
		// (or leaks the internal hostname on direct calls). The console URL
		// is the public origin and nginx serves /v1 right next to it.
		if c.cfg.ConsoleURL != "" {
			access.APIBaseURL = c.cfg.ConsoleURL + "/v1"
		} else {
			access.APIBaseURL = strings.TrimSpace(payload.APIBaseURL)
		}
	}
	return access, nil
}

func (c *difyClient) fetchApps(ctx context.Context, q difyAppQuery, session difySession) (difyAppPage, error) {
	endpoint := c.cfg.BaseURL + "/console/api/apps?" + q.values().Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return difyAppPage{}, newDifyError(difyCodeUnreachable, "Failed to connect to Dify", err)
	}
	req.Header.Set("Authorization", "Bearer "+session.AccessToken)
	req.Header.Set("X-CSRF-Token", session.CSRFToken)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: session.CSRFToken})

	resp, err := c.http.Do(req)
	if err != nil {
		return difyAppPage{}, newDifyError(difyCodeUnreachable, "Failed to connect to Dify", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return difyAppPage{}, newDifyError(difyCodeAuthFailed,
			"Dify rejected the configured account", nil)
	}
	if resp.StatusCode != http.StatusOK {
		return difyAppPage{}, newDifyError(difyCodeUnreachable, "Failed to connect to Dify",
			fmt.Errorf("dify apps returned status %d", resp.StatusCode))
	}

	var payload struct {
		Total int `json:"total"`
		Data  []struct {
			ID             string  `json:"id"`
			Name           string  `json:"name"`
			Description    *string `json:"description"`
			Mode           string  `json:"mode"`
			IconType       *string `json:"icon_type"`
			Icon           *string `json:"icon"`
			IconBackground *string `json:"icon_background"`
			BoundAgentID   *string `json:"bound_agent_id"`
			CreatedAt      *int64  `json:"created_at"`
			UpdatedAt      *int64  `json:"updated_at"`
		} `json:"data"`
	}
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return difyAppPage{}, newDifyError(difyCodeUnreachable, "Failed to connect to Dify", err)
	}

	apps := make([]difyApp, 0, len(payload.Data))
	for _, item := range payload.Data {
		apps = append(apps, difyApp{
			ID:             item.ID,
			Name:           item.Name,
			Description:    derefString(item.Description),
			Mode:           item.Mode,
			IconType:       derefString(item.IconType),
			Icon:           derefString(item.Icon),
			IconBackground: derefString(item.IconBackground),
			CreatedAt:      formatDifyTime(item.CreatedAt),
			UpdatedAt:      formatDifyTime(item.UpdatedAt),
			ConsolePath:    difyConsolePath(item.ID, item.Mode, derefString(item.BoundAgentID)),
		})
	}
	return difyAppPage{Total: payload.Total, Items: apps}, nil
}

// session returns the cached console session, logging in when there is none.
func (c *difyClient) session(ctx context.Context) (difySession, error) {
	key := c.sessionCacheKey()
	if cached, ok := readDifySession(key, time.Now()); ok {
		return cached, nil
	}
	return c.loginAndCache(ctx, key)
}

// refreshSession replaces the cached session with a fresh login.
func (c *difyClient) refreshSession(ctx context.Context) (difySession, error) {
	return c.loginAndCache(ctx, c.sessionCacheKey())
}

func (c *difyClient) loginAndCache(ctx context.Context, key string) (difySession, error) {
	session, err := c.login(ctx)
	if err != nil {
		dropDifySession(key)
		return difySession{}, err
	}
	storeDifySession(key, session)
	return session, nil
}

// login exchanges the configured account for a console session.
func (c *difyClient) login(ctx context.Context) (difySession, error) {
	body, err := common.Marshal(map[string]any{
		"email": c.cfg.Email,
		// Dify base64-decodes the password field on the wire.
		"password":    base64.StdEncoding.EncodeToString([]byte(c.cfg.Password)),
		"remember_me": true,
	})
	if err != nil {
		return difySession{}, newDifyError(difyCodeAuthFailed, "Dify rejected the configured account", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/console/api/login", strings.NewReader(string(body)))
	if err != nil {
		return difySession{}, newDifyError(difyCodeUnreachable, "Failed to connect to Dify", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return difySession{}, newDifyError(difyCodeUnreachable, "Failed to connect to Dify", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return difySession{}, newDifyError(difyCodeAuthFailed,
			"Dify rejected the configured account", fmt.Errorf("dify login returned status %d", resp.StatusCode))
	}

	session := difySession{ExpiresAt: time.Now().Add(difySessionTTL).Unix()}
	for _, cookie := range resp.Cookies() {
		switch strings.TrimPrefix(cookie.Name, "__Host-") {
		case "access_token":
			session.AccessToken = cookie.Value
		case "csrf_token":
			session.CSRFToken = cookie.Value
		}
	}
	if !session.valid(time.Now()) {
		return difySession{}, newDifyError(difyCodeAuthFailed,
			"Dify rejected the configured account", errors.New("dify login returned no console tokens"))
	}
	return session, nil
}

// sessionCacheKey keeps cached sessions apart per endpoint and account, so
// editing the settings takes effect immediately without an explicit flush.
func (c *difyClient) sessionCacheKey() string {
	sum := sha256.Sum256([]byte(c.cfg.BaseURL + "|" + c.cfg.Email))
	return difySessionCacheKey + hex.EncodeToString(sum[:8])
}

func readDifySession(key string, now time.Time) (difySession, bool) {
	if common.RedisEnabled && common.RDB != nil {
		raw, err := common.RedisGet(key)
		if err != nil {
			return difySession{}, false
		}
		var session difySession
		if common.UnmarshalJsonStr(raw, &session) != nil || !session.valid(now) {
			return difySession{}, false
		}
		return session, true
	}

	difySessionMu.Lock()
	defer difySessionMu.Unlock()
	session, ok := difySessionMemory[key]
	if !ok || !session.valid(now) {
		return difySession{}, false
	}
	return session, true
}

func storeDifySession(key string, session difySession) {
	if common.RedisEnabled && common.RDB != nil {
		if raw, err := common.Marshal(session); err == nil {
			_ = common.RedisSet(key, string(raw), difySessionTTL)
		}
		return
	}

	difySessionMu.Lock()
	defer difySessionMu.Unlock()
	if difySessionMemory == nil {
		difySessionMemory = map[string]difySession{}
	}
	difySessionMemory[key] = session
}

func dropDifySession(key string) {
	if common.RedisEnabled && common.RDB != nil {
		_ = common.RedisDel(key)
		return
	}

	difySessionMu.Lock()
	defer difySessionMu.Unlock()
	delete(difySessionMemory, key)
}

// difySessionMemory backs the session cache when Redis is not configured (single
// node deployments and tests).
var (
	difySessionMu     sync.Mutex
	difySessionMemory = map[string]difySession{}
)

// difyConsolePath is the console deep link Dify itself uses for an app.
func difyConsolePath(appID, mode, boundAgentID string) string {
	switch mode {
	case "agent":
		if boundAgentID != "" {
			return "/agents/" + boundAgentID + "/configure"
		}
		return "/agents"
	case "workflow", "advanced-chat":
		return "/app/" + appID + "/workflow"
	default:
		return "/app/" + appID + "/configuration"
	}
}

// difyWebAppPath is the public share path of an app according to its mode;
// the token is the app site's access code. Modes without a share page
// (channel, rag-pipeline) return an empty path.
func difyWebAppPath(mode, code string) string {
	switch mode {
	case "agent":
		return "/agent/" + code
	case "workflow":
		return "/workflow/" + code
	case "completion":
		return "/completion/" + code
	case "chat", "advanced-chat", "agent-chat":
		return "/chat/" + code
	default:
		return ""
	}
}

func difyModeFilterAllowed(mode string) bool {
	if mode == "" || mode == "all" {
		return true
	}
	for allowed := range strings.SplitSeq(difyAppModeFilters, ",") {
		if mode == allowed {
			return true
		}
	}
	return false
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func formatDifyTime(value *int64) string {
	if value == nil || *value <= 0 {
		return ""
	}
	return time.Unix(*value, 0).UTC().Format(time.RFC3339)
}
