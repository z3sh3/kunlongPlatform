package modelsquare

import (
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// Migrate creates or upgrades the modelsquare tables.
func Migrate() error {
	return model.DB.AutoMigrate(&FinetuneJob{}, &TrainingDataset{}, &TrainedModel{})
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
		user := group.Group("")
		user.Use(userAuth)
		{
			// Service capability discovery for the job and dataset forms.
			user.GET("/capabilities", TrainingCapabilities)

			// Bundled demonstration datasets shipped with the binary.
			user.GET("/demo-datasets", ListDemoDatasets)
			user.POST("/demo-datasets/install", InstallDemoDatasets)

			// Datasets: intake, DataFlow preparation and preparation logs.
			user.GET("/datasets", ListDatasets)
			user.POST("/datasets", UploadDataset)
			user.POST("/datasets/from-url", CreateDatasetFromURL)
			user.DELETE("/datasets/:id", DeleteDataset)
			user.POST("/datasets/:id/prepare", StartDatasetPrepare)
			user.GET("/datasets/:id/prepare", GetDatasetPrepare)
			user.POST("/datasets/:id/prepare/cancel", CancelDatasetPrepare)
			user.GET("/datasets/:id/prepare/logs", DatasetPrepareLogs)

			// Fine-tuning jobs.
			user.GET("/jobs", ListJobs)
			user.GET("/jobs/:id", GetJob)
			user.POST("/jobs", CreateJob)
			user.PUT("/jobs/:id", UpdateJob)
			user.DELETE("/jobs/:id", DeleteJob)
			user.POST("/jobs/:id/submit", SubmitJob)
			user.POST("/jobs/:id/cancel", CancelJob)
			user.GET("/jobs/:id/logs", JobLogs)
			user.POST("/jobs/:id/artifact", RegisterArtifact)

			// Trained model assets owned by the caller.
			user.GET("/artifacts", ListArtifacts)
		}

		admin := group.Group("/admin")
		admin.Use(adminAuth)
		{
			admin.GET("/jobs", AdminListJobs)
			admin.PUT("/jobs/:id/status", AdminUpdateJobStatus)
			admin.POST("/artifacts/:id/model", AdminPromoteArtifactToModel)
			admin.POST("/artifacts/:id/channel", AdminRegisterChannelForArtifact)
			admin.POST("/training-service/test", AdminTestTrainingService)
		}
	}
}
