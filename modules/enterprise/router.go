package enterprise

import (
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// Migrate creates or upgrades the enterprise module tables.
func Migrate() error {
	return model.DB.AutoMigrate(&IncubationProgram{}, &IncubationApplication{})
}

// RegisterRoutes mounts the enterprise incubation endpoints on the shared
// /api router group.
func RegisterRoutes(rg *gin.RouterGroup) {
	registerRoutes(rg, middleware.UserAuth(), middleware.AdminAuth())
}

// registerRoutes is split from RegisterRoutes so tests can inject fake
// authentication middleware.
func registerRoutes(rg *gin.RouterGroup, userAuth, adminAuth gin.HandlerFunc) {
	group := rg.Group("/enterprise-incubation")
	{
		user := group.Group("")
		user.Use(userAuth)
		{
			user.GET("/programs", ListPrograms)
			user.GET("/programs/:id", GetProgram)
			user.GET("/applications/my", ListMyApplications)
			user.POST("/applications", CreateApplication)
		}
		admin := group.Group("/admin")
		admin.Use(adminAuth)
		{
			admin.GET("/programs", AdminListPrograms)
			admin.POST("/programs", AdminCreateProgram)
			admin.PUT("/programs/:id", AdminUpdateProgram)
			admin.DELETE("/programs/:id", AdminDeleteProgram)
			admin.PUT("/programs/:id/status", AdminUpdateProgramStatus)
			admin.GET("/applications", AdminListApplications)
			admin.PUT("/applications/:id/review", AdminReviewApplication)
		}
	}
}
