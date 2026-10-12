package modelsquare

import (
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// Migrate creates or upgrades the modelsquare tables.
func Migrate() error {
	return model.DB.AutoMigrate(&FinetuneJob{})
}

// RegisterRoutes mounts the fine-tuning endpoints on the shared /api
// router group.
func RegisterRoutes(rg *gin.RouterGroup) {
	registerRoutes(rg, middleware.UserAuth(), middleware.AdminAuth())
}

// registerRoutes is split from RegisterRoutes so tests can inject fake
// authentication middleware.
func registerRoutes(rg *gin.RouterGroup, userAuth, adminAuth gin.HandlerFunc) {
	group := rg.Group("/model-training")
	{
		user := group.Group("/jobs")
		user.Use(userAuth)
		{
			user.GET("", ListJobs)
			user.GET("/:id", GetJob)
			user.POST("", CreateJob)
			user.PUT("/:id", UpdateJob)
			user.DELETE("/:id", DeleteJob)
			user.POST("/:id/submit", SubmitJob)
			user.POST("/:id/cancel", CancelJob)
		}
		admin := group.Group("/admin/jobs")
		admin.Use(adminAuth)
		{
			admin.GET("", AdminListJobs)
			admin.PUT("/:id/status", AdminUpdateJobStatus)
		}
	}
}
