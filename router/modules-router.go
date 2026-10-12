package router

import (
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/modules/agentworkshop"
	"github.com/QuantumNous/new-api/modules/datasetmarket"
	"github.com/QuantumNous/new-api/modules/enterprise"
	"github.com/QuantumNous/new-api/modules/modelsquare"
	"github.com/QuantumNous/new-api/modules/talent"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// SetModulesRouter registers the independent console business modules
// (data marketplace, model training, agent workshop, talent training,
// enterprise incubation). Each module owns its own sub-routes; the modules
// never depend on each other.
func SetModulesRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.RouteTag("api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.AccessTokenAudit())
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	{
		datasetmarket.RegisterRoutes(apiRouter)
		modelsquare.RegisterRoutes(apiRouter)
		agentworkshop.RegisterRoutes(apiRouter)
		talent.RegisterRoutes(apiRouter)
		enterprise.RegisterRoutes(apiRouter)
	}
}
