package router

import (
	v2 "github.com/1Panel-dev/1Panel/core/app/api/v2"
	"github.com/1Panel-dev/1Panel/core/extensions/enhance"
	"github.com/gin-gonic/gin"
)

type SimpleNodeRouter struct{}

func (s *SimpleNodeRouter) InitRouter(Router *gin.RouterGroup) {
	baseApi := v2.ApiGroupApp.BaseApi
	enhance.RegisterSimpleNodeRoutes(Router, enhance.SimpleNodeHandlers{
		ListAll:       baseApi.ListAllSimpleNodes,
		Create:        baseApi.CreateSimpleNode,
		Update:        baseApi.UpdateSimpleNode,
		Delete:        baseApi.DeleteSimpleNode,
		Check:         baseApi.CheckSimpleNode,
		BuildVisitURL: baseApi.BuildSimpleNodeVisitURL,
		Refresh:       baseApi.RefreshSimpleNode,
		RefreshAll:    baseApi.RefreshAllSimpleNodes,
	})
}
