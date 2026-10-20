package agentworkshop

import (
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// Migrate creates or upgrades the agentworkshop tables.
func Migrate() error {
	return model.DB.AutoMigrate(&Agent{})
}

// RegisterRoutes mounts the agent workshop endpoints on the shared /api
// router group.
func RegisterRoutes(rg *gin.RouterGroup) {
	registerRoutes(rg, middleware.UserAuth(), middleware.AdminAuth())
}

// registerRoutes is split from RegisterRoutes so tests can inject fake
// authentication middleware.
func registerRoutes(rg *gin.RouterGroup, userAuth, adminAuth gin.HandlerFunc) {
	group := rg.Group("/agent-workshop")
	{
		user := group.Group("/agents")
		user.Use(userAuth)
		{
			user.GET("", ListAgents)
			user.GET("/:id", GetAgent)
			user.POST("", CreateAgent)
			user.PUT("/:id", UpdateAgent)
			user.DELETE("/:id", DeleteAgent)
			user.POST("/:id/submit", SubmitAgent)
		}
		dify := group.Group("/dify")
		dify.Use(userAuth)
		{
			dify.GET("/agents", ListDifyAgents)
			dify.GET("/agents/:id/access", GetDifyAgentAccess)
		}
		admin := group.Group("/admin/agents")
		admin.Use(adminAuth)
		{
			admin.GET("", AdminListAgents)
			admin.PUT("/:id/review", AdminReviewAgent)
			admin.PUT("/:id/offline", AdminSetAgentOffline)
		}
		difyAdmin := group.Group("/admin/dify")
		difyAdmin.Use(adminAuth)
		{
			difyAdmin.POST("/test", CheckDifyConnection)
		}
	}
}
