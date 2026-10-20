package talent

import (
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// Migrate creates or upgrades the talent module tables.
func Migrate() error {
	return model.DB.AutoMigrate(&TrainingCourse{}, &TrainingEnrollment{})
}

// RegisterRoutes mounts the talent training endpoints on the shared /api
// router group.
func RegisterRoutes(rg *gin.RouterGroup) {
	registerRoutes(rg, middleware.UserAuth(), middleware.AdminAuth())
}

// registerRoutes is split from RegisterRoutes so tests can inject fake
// authentication middleware.
func registerRoutes(rg *gin.RouterGroup, userAuth, adminAuth gin.HandlerFunc) {
	group := rg.Group("/talent-training")
	{
		user := group.Group("")
		user.Use(userAuth)
		{
			user.GET("/courses", ListCourses)
			user.GET("/courses/:id", GetCourse)
			user.GET("/enrollments/my", ListMyEnrollments)
			user.POST("/courses/:id/enroll", Enroll)
		}
		admin := group.Group("/admin")
		admin.Use(adminAuth)
		{
			admin.GET("/courses", AdminListCourses)
			admin.POST("/courses", AdminCreateCourse)
			admin.POST("/demo-courses", SeedDemoCourses)
			admin.PUT("/courses/:id", AdminUpdateCourse)
			admin.DELETE("/courses/:id", AdminDeleteCourse)
			admin.PUT("/courses/:id/status", AdminUpdateCourseStatus)
			admin.GET("/enrollments", AdminListEnrollments)
			admin.PUT("/enrollments/:id/review", AdminReviewEnrollment)
		}
	}
}
