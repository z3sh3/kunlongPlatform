package datasetmarket

import (
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// Migrate creates or upgrades the datasetmarket tables.
func Migrate() error {
	return model.DB.AutoMigrate(&Dataset{}, &DatasetGrant{})
}

// RegisterRoutes mounts the data marketplace endpoints on the shared /api
// router group.
func RegisterRoutes(rg *gin.RouterGroup) {
	registerRoutes(rg, middleware.UserAuth(), middleware.AdminAuth())
}

// registerRoutes is split from RegisterRoutes so tests can inject fake
// authentication middleware.
func registerRoutes(rg *gin.RouterGroup, userAuth, adminAuth gin.HandlerFunc) {
	group := rg.Group("/data-market")
	{
		user := group.Group("")
		user.Use(userAuth)
		{
			user.GET("/datasets", ListDatasets)
			user.GET("/datasets/:id", GetDataset)
			user.POST("/datasets", CreateDataset)
			user.PUT("/datasets/:id", UpdateDataset)
			user.DELETE("/datasets/:id", DeleteDataset)
			user.POST("/datasets/:id/apply", ApplyDataset)
			user.GET("/grants/my", ListMyGrants)
		}
		admin := group.Group("/admin")
		admin.Use(adminAuth)
		{
			admin.GET("/datasets", AdminListDatasets)
			admin.GET("/grants", AdminListGrants)
			admin.PUT("/datasets/:id/status", AdminUpdateDatasetStatus)
			admin.PUT("/grants/:id/review", AdminReviewGrant)
		}
	}
}
