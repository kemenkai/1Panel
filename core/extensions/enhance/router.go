package enhance

import (
	"github.com/1Panel-dev/1Panel/core/middleware"
	"github.com/gin-gonic/gin"
)

type SimpleNodeHandlers struct {
	ListAll       gin.HandlerFunc
	Create        gin.HandlerFunc
	Update        gin.HandlerFunc
	Delete        gin.HandlerFunc
	Check         gin.HandlerFunc
	BuildVisitURL gin.HandlerFunc
	Refresh       gin.HandlerFunc
	RefreshAll    gin.HandlerFunc
}

func RegisterAuthRoutes(baseRouter *gin.RouterGroup, simpleNodeLogin gin.HandlerFunc) {
	baseRouter.GET("/simple-node/login", simpleNodeLogin)
}

func RegisterSimpleNodeRoutes(router *gin.RouterGroup, handlers SimpleNodeHandlers) {
	nodeRouter := router.Group("nodes/simple").
		Use(middleware.SessionAuth()).
		Use(middleware.PasswordExpired())
	xpackRouter := router.Group("xpack/nodes/simple").
		Use(middleware.SessionAuth()).
		Use(middleware.PasswordExpired())

	nodeRouter.GET("/all", handlers.ListAll)

	xpackRouter.POST("", handlers.Create)
	xpackRouter.POST("/update/base", handlers.Update)
	xpackRouter.POST("/del", handlers.Delete)
	xpackRouter.POST("/check", handlers.Check)
	xpackRouter.POST("/visit", handlers.BuildVisitURL)
	xpackRouter.POST("/refresh", handlers.Refresh)
	xpackRouter.POST("/refresh/all", handlers.RefreshAll)
}
