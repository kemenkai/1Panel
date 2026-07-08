package router

import (
	v2 "github.com/1Panel-dev/1Panel/agent/app/api/v2"
	"github.com/gin-gonic/gin"
)

type WindowsServiceRouter struct{}

func (w *WindowsServiceRouter) InitRouter(Router *gin.RouterGroup) {
	windowsRouter := Router.Group("windows/services")
	baseApi := v2.ApiGroupApp.BaseApi
	{
		windowsRouter.GET("", baseApi.ListWindowsServices)
		windowsRouter.GET("/config-template", baseApi.GetWindowsServiceConfigTemplate)
		windowsRouter.GET("/:id/config-file", baseApi.GetWindowsServiceConfigFile)
		windowsRouter.POST("/:id/config-file", baseApi.UpdateWindowsServiceConfigFile)
		windowsRouter.GET("/:id/log", baseApi.GetWindowsServiceLogFile)
		windowsRouter.POST("", baseApi.CreateWindowsService)
		windowsRouter.POST("/upload-package", baseApi.UploadWindowsServicePackage)
		windowsRouter.POST("/upload-jar", baseApi.UploadWindowsServiceJar)
		windowsRouter.POST("/update", baseApi.UpdateWindowsService)
		windowsRouter.POST("/operate", baseApi.OperateWindowsService)
		windowsRouter.POST("/del", baseApi.DeleteWindowsService)
	}
}
