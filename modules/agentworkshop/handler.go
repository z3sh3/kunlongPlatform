package agentworkshop

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modules/shared"
	"github.com/gin-gonic/gin"
)

type agentRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
	Config      string `json:"config"`
	PriceMode   int    `json:"price_mode"`
	UnitPrice   int64  `json:"unit_price"`
}

type reviewAgentRequest struct {
	Approve bool   `json:"approve"`
	Comment string `json:"comment"`
}

type offlineAgentRequest struct {
	Offline bool `json:"offline"`
}

// ListAgents returns published agents for browsing, or the current user's
// own agents of any status when mine=1.
func ListAgents(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&Agent{})
	if c.Query("mine") == "1" {
		query = query.Where("user_id = ?", c.GetInt("id"))
	} else {
		query = query.Where("status = ?", StatusPublished)
		if keyword := c.Query("keyword"); keyword != "" {
			query = query.Where("name LIKE ?", keyword+"%")
		}
		if category := c.Query("category"); category != "" {
			query = query.Where("category = ?", category)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var agents []Agent
	if err := query.Order("id desc").Offset(pageInfo.GetStartIdx()).
		Limit(pageInfo.GetPageSize()).Find(&agents).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(agents)
	common.ApiSuccess(c, pageInfo)
}

// GetAgent returns one agent. Published agents are visible to every
// logged-in user; other statuses are limited to the owner and
// administrators.
func GetAgent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid agent id")
		return
	}
	var agent Agent
	if err := model.DB.First(&agent, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if agent.Status != StatusPublished && agent.UserId != c.GetInt("id") && !isAdmin(c) {
		common.ApiErrorMsg(c, "no permission to view this agent")
		return
	}
	common.ApiSuccess(c, agent)
}

// CreateAgent creates a draft agent.
func CreateAgent(c *gin.Context) {
	var req agentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validatePrice(req.PriceMode, req.UnitPrice); err != nil {
		common.ApiError(c, err)
		return
	}
	agent := Agent{
		UserId:      c.GetInt("id"),
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		Category:    req.Category,
		Config:      req.Config,
		PriceMode:   req.PriceMode,
		UnitPrice:   req.UnitPrice,
		Status:      StatusDraft,
	}
	if err := model.DB.Create(&agent).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, agent)
}

// UpdateAgent edits a draft agent owned by the current user.
func UpdateAgent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid agent id")
		return
	}
	var agent Agent
	if err := model.DB.First(&agent, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if agent.UserId != c.GetInt("id") {
		common.ApiErrorMsg(c, "no permission to edit this agent")
		return
	}
	if agent.Status != StatusDraft {
		common.ApiErrorMsg(c, "only draft agents can be edited")
		return
	}
	var req agentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validatePrice(req.PriceMode, req.UnitPrice); err != nil {
		common.ApiError(c, err)
		return
	}
	updates := map[string]any{
		"name":        req.Name,
		"description": req.Description,
		"icon":        req.Icon,
		"category":    req.Category,
		"config":      req.Config,
		"price_mode":  req.PriceMode,
		"unit_price":  req.UnitPrice,
	}
	if err := model.DB.Model(&agent).Updates(updates).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, agent)
}

// DeleteAgent deletes a draft or offline agent owned by the current user.
func DeleteAgent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid agent id")
		return
	}
	var agent Agent
	if err := model.DB.First(&agent, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if agent.UserId != c.GetInt("id") && !isAdmin(c) {
		common.ApiErrorMsg(c, "no permission to delete this agent")
		return
	}
	if agent.Status != StatusDraft && agent.Status != StatusOffline && !isAdmin(c) {
		common.ApiErrorMsg(c, "only draft or offline agents can be deleted")
		return
	}
	if err := model.DB.Delete(&agent).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// SubmitAgent sends a draft agent to review.
func SubmitAgent(c *gin.Context) {
	agent, ok := loadOwnedAgent(c)
	if !ok {
		return
	}
	if agent.Status != StatusDraft {
		common.ApiErrorMsg(c, "only draft agents can be submitted for review")
		return
	}
	if err := model.DB.Model(&agent).Updates(map[string]any{
		"status":         StatusReviewing,
		"review_comment": "",
	}).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminListAgents returns every agent regardless of status.
func AdminListAgents(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&Agent{})
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("name LIKE ?", keyword+"%")
	}
	if status, err := strconv.Atoi(c.Query("status")); err == nil && status >= 0 {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var agents []Agent
	if err := query.Order("id desc").Offset(pageInfo.GetStartIdx()).
		Limit(pageInfo.GetPageSize()).Find(&agents).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(agents)
	common.ApiSuccess(c, pageInfo)
}

// AdminReviewAgent approves or rejects an agent under review. Rejection
// returns it to draft together with the review comment.
func AdminReviewAgent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid agent id")
		return
	}
	var req reviewAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	var agent Agent
	if err := model.DB.First(&agent, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if agent.Status != StatusReviewing {
		common.ApiErrorMsg(c, "agent is not under review")
		return
	}
	nextStatus := StatusDraft
	if req.Approve {
		nextStatus = StatusPublished
	}
	if err := model.DB.Model(&agent).Updates(map[string]any{
		"status":         nextStatus,
		"review_comment": req.Comment,
	}).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminSetAgentOffline takes a published agent offline, or brings an
// offline agent back to published.
func AdminSetAgentOffline(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid agent id")
		return
	}
	var req offlineAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	var agent Agent
	if err := model.DB.First(&agent, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	nextStatus := StatusPublished
	if req.Offline {
		nextStatus = StatusOffline
	}
	if agent.Status != StatusPublished && agent.Status != StatusOffline {
		common.ApiErrorMsg(c, "only published or offline agents can toggle availability")
		return
	}
	if err := model.DB.Model(&agent).Update("status", nextStatus).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// difyAgentsResponse is the paginated app list plus the public console origin
// the UI needs to build its "open in Dify" links.
type difyAgentsResponse struct {
	*common.PageInfo
	ConsoleBaseURL string `json:"console_base_url"`
}

// ListDifyAgents returns the Dify applications of the configured workspace.
// Dify is the single source of truth for agents, so nothing is stored locally.
func ListDifyAgents(c *gin.Context) {
	client, err := newDifyClient()
	if err != nil {
		writeDifyError(c, err)
		return
	}

	mode := c.Query("mode")
	if !difyModeFilterAllowed(mode) {
		common.ApiErrorT(c, "Unsupported agent type filter")
		return
	}

	pageInfo := common.GetPageQuery(c)
	pageInfo.Page = max(pageInfo.Page, 1)
	pageInfo.PageSize = min(max(pageInfo.PageSize, 1), difyMaxPageSize)

	page, err := client.listApps(c.Request.Context(), difyAppQuery{
		Page:  pageInfo.Page,
		Limit: pageInfo.PageSize,
		Mode:  mode,
		Name:  c.Query("keyword"),
		Mine:  c.Query("mine") == "1",
	})
	if err != nil {
		writeDifyError(c, err)
		return
	}

	pageInfo.SetTotal(page.Total)
	pageInfo.SetItems(page.Items)
	common.ApiSuccess(c, difyAgentsResponse{
		PageInfo:       pageInfo,
		ConsoleBaseURL: client.cfg.ConsoleURL,
	})
}

// difyTestRequest carries optional unsaved form values from the settings
// page. Every field is optional; blank fields keep the saved value.
type difyTestRequest struct {
	BaseURL  string `json:"base_url"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// CheckDifyConnection verifies the configured account can list applications.
// The optional body lets the settings page test the form values before
// saving them, so a freshly typed configuration works immediately.
func CheckDifyConnection(c *gin.Context) {
	var req difyTestRequest
	if c.Request != nil && c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	client, err := newDifyTestClient(req.BaseURL, req.Email, req.Password)
	if err != nil {
		writeDifyError(c, err)
		return
	}
	classicTotal, agentTotal, err := client.countApps(c.Request.Context())
	if err != nil {
		writeDifyError(c, err)
		return
	}
	// The count covers both Dify sources: classic apps plus the Agent product.
	common.ApiSuccess(c, gin.H{
		"total":          classicTotal + agentTotal,
		"email":          client.cfg.Email,
		"console_url_ok": client.probeConsoleURL(c.Request.Context()),
	})
}

// GetDifyAgentAccess explains how one agent can be reached: the published
// web app, the service API, and the Dify console entry.
func GetDifyAgentAccess(c *gin.Context) {
	appID := c.Param("id")
	if !difyAppIDAllowed(appID) {
		common.ApiErrorT(c, "Invalid agent id")
		return
	}
	client, err := newDifyClient()
	if err != nil {
		writeDifyError(c, err)
		return
	}
	access, err := client.fetchAppAccess(c.Request.Context(), appID)
	if err != nil {
		writeDifyError(c, err)
		return
	}
	common.ApiSuccess(c, access)
}

// difyAppIDAllowed guards the Dify app id before it is embedded in the
// upstream request path: Dify ids are UUID-shaped.
func difyAppIDAllowed(appID string) bool {
	if appID == "" || len(appID) > 64 {
		return false
	}
	// The id must contain no rune outside the UUID alphabet.
	return !strings.ContainsFunc(appID, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			return false
		default:
			return true
		}
	})
}

func writeDifyError(c *gin.Context, err error) {
	var difyErr *difyError
	if errors.As(err, &difyErr) {
		common.ApiErrorStatus(c, http.StatusOK, common.NewMessage(difyErr.Error()), gin.H{"code": difyErr.code})
		return
	}
	common.ApiError(c, err)
}

func loadOwnedAgent(c *gin.Context) (Agent, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid agent id")
		return Agent{}, false
	}
	var agent Agent
	if err := model.DB.First(&agent, id).Error; err != nil {
		common.ApiError(c, err)
		return Agent{}, false
	}
	if agent.UserId != c.GetInt("id") {
		common.ApiErrorMsg(c, "no permission to access this agent")
		return Agent{}, false
	}
	return agent, true
}

func isAdmin(c *gin.Context) bool {
	return c.GetInt("role") >= common.RoleAdminUser
}

func validatePrice(priceMode int, unitPrice int64) error {
	if priceMode != shared.PriceModeFree && priceMode != shared.PriceModePaid {
		return errors.New("invalid price mode")
	}
	if priceMode == shared.PriceModePaid && unitPrice <= 0 {
		return errors.New("paid agents require a positive unit price")
	}
	return nil
}
